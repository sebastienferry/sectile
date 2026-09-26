package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"tasks/internal/agentexec"
	"tasks/internal/agenthttp"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
	"tasks/internal/runner"
)

// contractPrefix is the path every versioned agent route shares. A server that
// serves the contract serves all of them; one that serves none of them predates
// the contract entirely.
const contractPrefix = "/api/v1/agent/"

// contractState remembers the last contract mismatch seen on any server call,
// so the desktop can say the server is incompatible instead of showing a bare
// disconnection. It owns its mutex: nothing else is read or written under it.
type contractState struct {
	mu       sync.Mutex
	mismatch string
}

// note records a mismatch and announces it once per episode, then returns it
// unchanged. Every server call funnels through here, so a mismatch surfacing
// during a dispatch or a desktop request is reported as loudly as one surfacing
// while connecting. Announcing on the message rather than the episode would
// repeat the banner every time the failing route alternates between the
// connection loop and a desktop call.
func (c *contractState) note(mismatch *agentconfig.Mismatch) error {
	c.mu.Lock()
	first := c.mismatch == ""
	c.mismatch = mismatch.Error()
	c.mu.Unlock()
	if first {
		log.Printf("⛔ [Agent] Server contract mismatch: %v", mismatch)
		fmt.Printf("\n⛔ ========================================================\n")
		fmt.Printf("⛔ [Agent] Server contract mismatch\n")
		fmt.Printf("   %s\n", mismatch)
		fmt.Printf("========================================================\n\n")
	}
	return mismatch
}

// clear forgets a mismatch once a contract route answers correctly, which is
// what an updated and restarted server produces.
func (c *contractState) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mismatch = ""
}

// current reports the standing mismatch, empty when there is none.
func (c *contractState) current() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.mismatch
}

func (d *agentDaemon) readAPI(ctx context.Context, path string, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.link.serverURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := agenthttp.Client(d.link.token).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// No contract handler answers 404: an unknown project is a 400 and a
	// rejected credential a 401. A 404 here therefore means the route is not
	// registered at all, which the server's catch-all reports as a missing API
	// route. Read as a plain HTTP failure it looks transient and the agent
	// retries forever; named for what it is, it points at the build to update.
	if resp.StatusCode == http.StatusNotFound && strings.HasPrefix(path, contractPrefix) {
		route, _, _ := strings.Cut(path, "?")
		return d.contract.note(&agentconfig.Mismatch{Server: d.link.serverURL, Route: route, Status: resp.StatusCode})
	}
	if resp.StatusCode != http.StatusOK {
		var detail struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&detail)
		if detail.Error != "" {
			return fmt.Errorf("Sectile API returned HTTP %d: %s", resp.StatusCode, detail.Error)
		}
		return fmt.Errorf("Sectile API returned HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(result)
}

func (d *agentDaemon) fetchConfig(ctx context.Context, projectID, taskKey string, framework ...string) (agentconfig.Config, error) {
	var c agentconfig.Config
	q := url.Values{}
	if taskKey != "" {
		q.Set("taskKey", taskKey)
	} else {
		q.Set("projectId", projectID)
	}
	if len(framework) > 0 && framework[0] != "" {
		q.Set("framework", framework[0])
	}
	err := d.readAPI(ctx, "/api/v1/agent/config?"+q.Encode(), &c)
	if err == nil {
		if c.SchemaVersion != agentconfig.Version {
			// Validate rejects this too, but its message serves local files as
			// well. A payload from the server is a build disagreement, and
			// saying so keeps every contract failure reading alike.
			err = d.contract.note(&agentconfig.Mismatch{Server: d.link.serverURL, Route: "/api/v1/agent/config", Served: c.SchemaVersion})
		} else {
			d.contract.clear()
		}
	}
	if err == nil {
		err = c.Validate()
	}
	if err == nil && projectID != "" && c.ProjectID != projectID {
		err = fmt.Errorf("configuration projectId mismatch: requested %s, received %s", projectID, c.ProjectID)
	}
	return c, err
}

func gitLocal(ctx context.Context, root string, args ...string) (string, error) {
	cmd := agentexec.Hidden(exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...))
	raw, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(string(raw)))
	}
	return strings.TrimSpace(string(raw)), nil
}

// localProjectRoot resolves workstation mappings. Remote filesystem paths are
// deliberately absent from the contract and never used as local working dirs.
func (d *agentDaemon) localProjectRoot(ctx context.Context, c agentconfig.Config, allowUninitialized ...bool) (string, agentconfig.Overrides, error) {
	root := d.repoRoot
	if root == "" {
		root, _ = os.Getwd()
		root = findRepoRoot(root)
	}
	overrides, err := agentconfig.ReadSettings(root)
	if err != nil {
		return "", overrides, err
	}
	if overrides.DisconnectedProjects[c.ProjectID] {
		return "", overrides, fmt.Errorf("project %s is disconnected; add it again in the desktop before launching", c.ProjectID)
	}
	if mapped := overrides.Projects[c.ProjectID]; mapped != "" {
		if !filepath.IsAbs(mapped) {
			mapped = filepath.Join(root, mapped)
		}
		root = mapped
	} else if d.link.projectID != c.ProjectID {
		remote, err := gitLocal(ctx, root, "remote", "get-url", "origin")
		if err != nil || c.GitRemoteURL == "" || models.RepositoryIdentity(remote) != models.RepositoryIdentity(c.GitRemoteURL) {
			return "", overrides, fmt.Errorf("no local repository mapping for project %s; configure ~/.config/sectile/settings.json projects", c.ProjectID)
		}
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", overrides, err
	}
	if _, err := gitLocal(ctx, root, "rev-parse", "--show-toplevel"); err != nil && !(len(allowUninitialized) > 0 && allowUninitialized[0]) {
		return "", overrides, err
	}
	local, err := agentconfig.ReadOverrides(root)
	if err != nil {
		return "", overrides, err
	}
	if value, ok := overrides.Worktrees[c.ProjectID]; ok {
		if local.Worktrees == nil {
			local.Worktrees = map[string]bool{}
		}
		local.Worktrees[c.ProjectID] = value
	}
	if value, ok := overrides.SpecArtifacts[c.ProjectID]; ok {
		if local.SpecArtifacts == nil {
			local.SpecArtifacts = map[string]string{}
		}
		local.SpecArtifacts[c.ProjectID] = value
	}
	if value, ok := overrides.Parallelism[c.ProjectID]; ok {
		if local.Parallelism == nil {
			local.Parallelism = map[string]int{}
		}
		local.Parallelism[c.ProjectID] = value
	}
	if local.Commands == nil {
		local.Commands = map[string]string{}
	}
	for id, command := range overrides.Commands {
		local.Commands[id] = command
	}
	if local.CommandsAutonomous == nil {
		local.CommandsAutonomous = map[string]string{}
	}
	for id, command := range overrides.CommandsAutonomous {
		local.CommandsAutonomous[id] = command
	}
	if local.AIProviders == nil {
		local.AIProviders = map[string]string{}
	}
	for id, provider := range overrides.AIProviders {
		local.AIProviders[id] = provider
	}
	if local.AIModels == nil {
		local.AIModels = map[string]string{}
	}
	for id, model := range overrides.AIModels {
		local.AIModels[id] = model
	}
	if overrides.AIProvider != "" {
		local.AIProvider = overrides.AIProvider
	}
	if overrides.AICommandTemplate != "" {
		local.AICommandTemplate = overrides.AICommandTemplate
	}
	if overrides.AICommandTemplateAutonomous != "" {
		local.AICommandTemplateAutonomous = overrides.AICommandTemplateAutonomous
	}
	if overrides.AIModel != "" {
		local.AIModel = overrides.AIModel
	}
	if local.AISkillModels == nil {
		local.AISkillModels = map[string]string{}
	}
	for id, model := range overrides.AISkillModels {
		local.AISkillModels[id] = model
	}
	if overrides.Terminal != "" {
		local.Terminal = overrides.Terminal
	}
	if local.Terminals == nil {
		local.Terminals = map[string]string{}
	}
	for id, terminal := range overrides.Terminals {
		local.Terminals[id] = terminal
	}
	if local.Skills == nil {
		local.Skills = map[string]string{}
	}
	for id, content := range overrides.Skills {
		local.Skills[id] = content
	}
	// The specifications folder is saved in the workstation settings only:
	// leaving it out here made the desktop setting invisible to every macro
	// operation.
	if local.SpecRepos == nil {
		local.SpecRepos = map[string]string{}
	}
	for id, folder := range overrides.SpecRepos {
		local.SpecRepos[id] = folder
	}
	// So are the repository folders (#456), which every multi-repo launch
	// resolves through the value returned here.
	if local.Repositories == nil {
		local.Repositories = map[string]string{}
	}
	for identity, folder := range overrides.Repositories {
		local.Repositories[identity] = folder
	}
	return root, local, nil
}

// worktreeForBranch returns the path of the worktree checked out on branch,
// among every worktree of the repository at root, including the main checkout.
// It returns "" when no worktree carries it. A detached or bare worktree
// carries no branch and never matches.
func worktreeForBranch(ctx context.Context, root, branch string) (string, error) {
	out, err := gitLocal(ctx, root, "worktree", "list", "--porcelain")
	if err != nil {
		return "", err
	}
	// The porcelain format is blank-line separated records, each opening with
	// "worktree <path>" and carrying at most one of "branch refs/heads/<name>",
	// "detached" or "bare".
	path := ""
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "worktree "):
			path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "branch refs/heads/"):
			if path != "" && strings.TrimPrefix(line, "branch refs/heads/") == branch {
				return path, nil
			}
		case line == "":
			path = ""
		}
	}
	return "", nil
}

func ensureLocalWorktree(ctx context.Context, root string, task models.Task, useWorktrees bool) (string, string, error) {
	branch := ""
	if task.BranchName != nil {
		branch = strings.TrimSpace(*task.BranchName)
	}
	if !useWorktrees {
		current, err := gitLocal(ctx, root, "branch", "--show-current")
		if branch != "" && current != branch {
			return "", "", fmt.Errorf("checkout branch %s does not match assigned branch %s", current, branch)
		}
		return root, current, err
	}
	if task.Key == "" || task.Key == "." || task.Key == ".." || strings.ContainsAny(task.Key, "/\\") {
		return "", "", fmt.Errorf("invalid task key for worktree")
	}
	if branch == "" {
		slug := strings.Trim(strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
				return r
			}
			return '-'
		}, strings.ToLower(task.Key)), "-")
		if slug == "" {
			return "", "", fmt.Errorf("task key cannot produce a branch name")
		}
		branch = "feat/" + slug
	}
	if _, err := gitLocal(ctx, root, "check-ref-format", "--branch", branch); err != nil {
		return "", "", err
	}
	// The worktree is resolved by branch, not by path. git reports every linked
	// worktree and the main checkout in one call, so the tree that carries the
	// assigned branch is reused wherever it sits - under another key, or in the
	// main checkout with its uncommitted work. A path nobody thought to probe is
	// precisely what made a launch fail while the branch was alive next door.
	existing, err := worktreeForBranch(ctx, root, branch)
	if err != nil {
		return "", "", err
	}
	if existing != "" {
		// git reports fully resolved paths; on macOS the main checkout comes
		// back through /private, so the caller's own root is preferred when the
		// two name the same directory.
		if sameDirectory(existing, root) {
			return root, branch, nil
		}
		return existing, branch, nil
	}

	// The branch is checked out nowhere, so a worktree has to be created. The
	// key path is the natural home; when it is taken by an unrelated branch the
	// launch still proceeds, on a sibling path, and the stale path is named in
	// the log rather than turned into a refusal.
	target := filepath.Join(root, ".tasks", "worktrees", task.Key)
	if _, err := os.Stat(target); err == nil {
		occupant, occErr := gitLocal(ctx, target, "branch", "--show-current")
		if occErr != nil {
			occupant = "an unknown branch"
		}
		suffix := strings.ReplaceAll(models.SanitizeBranchName(branch), "/", "-")
		if suffix == "" {
			suffix = "branch"
		}
		log.Printf("[Agent] Stale worktree path %s carries %s, not the assigned branch %s; creating the worktree beside it", target, occupant, branch)
		target = filepath.Join(root, ".tasks", "worktrees", task.Key+"-"+suffix)
	} else if !os.IsNotExist(err) {
		return "", "", err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return "", "", err
	}
	args := []string{"worktree", "add", target, branch}
	if _, err := gitLocal(ctx, root, "show-ref", "--verify", "refs/heads/"+branch); err != nil {
		base := "HEAD"
		if _, err := gitLocal(ctx, root, "show-ref", "--verify", "refs/remotes/origin/"+branch); err == nil {
			base = "refs/remotes/origin/" + branch
		}
		args = []string{"worktree", "add", "-b", branch, target, base}
	}
	if _, err := gitLocal(ctx, root, args...); err != nil {
		return "", "", err
	}
	return target, branch, nil
}

// prepareDispatch resolves the task's workspace, then provisions its
// dependencies. The install can take minutes, so it runs once prepareMu is
// released: holding the mutex through it would stall every other preparation on
// the agent behind one npm ci. The launch path waits for the install, so the
// session starts with its dependencies in place.
func (d *agentDaemon) prepareDispatch(ctx context.Context, taskKey string, useWorktrees ...bool) (agentconfig.Config, string, string, models.Task, error) {
	config, root, workDir, branch, task, err := d.prepareWorkspace(ctx, taskKey, useWorktrees...)
	if err == nil {
		provisionWorktree(ctx, root, workDir)
		// A multi-repo ticket may work in another checkout than the one it
		// was admitted against: the queue compares checkouts through this.
		d.queue.mu.Lock()
		for _, run := range d.queue.runs {
			if run.taskID == taskKey && run.desktop.Status == "preparing" {
				run.root = root
			}
		}
		d.queue.mu.Unlock()
	}
	return config, workDir, branch, task, err
}

// prepareWorkspace resolves the task's workspace under prepareMu without
// provisioning it, and returns the project root beside the working directory.
func (d *agentDaemon) prepareWorkspace(ctx context.Context, taskKey string, useWorktrees ...bool) (agentconfig.Config, string, string, string, models.Task, error) {
	d.prepareMu.Lock()
	defer d.prepareMu.Unlock()
	return d.prepareDispatchLocked(ctx, taskKey, useWorktrees...)
}

// prepareDispatchLocked is the part of prepareDispatch that runs under
// prepareMu. It also returns the project root, so the caller can tell a linked
// worktree from the main checkout.
func (d *agentDaemon) prepareDispatchLocked(ctx context.Context, taskKey string, useWorktrees ...bool) (agentconfig.Config, string, string, string, models.Task, error) {
	var task models.Task
	config, err := d.fetchConfig(ctx, "", taskKey)
	if err != nil {
		return config, "", "", "", task, err
	}
	root, overrides, err := d.localProjectRoot(ctx, config)
	if err != nil {
		return config, "", "", "", task, err
	}
	if err := d.readAPI(ctx, "/api/tasks/"+url.PathEscape(taskKey), &task); err != nil {
		return config, "", "", "", task, err
	}
	if task.ProjectID != config.ProjectID {
		return config, "", "", "", task, fmt.Errorf("task project changed during configuration sync")
	}
	config = agentconfig.ApplyOverrides(config, overrides)
	if len(useWorktrees) > 0 {
		config.UseWorktrees = useWorktrees[0]
	}
	if err := config.Validate(); err != nil {
		return config, "", "", "", task, err
	}
	// The worktree lives in the ticket's own repository, which on a multi-repo
	// project is not necessarily the project root (#456).
	primary, _, pin, err := primaryRoot(ctx, config, overrides, root, task)
	if err != nil {
		return config, "", "", "", task, err
	}
	if pin != "" {
		// The only repository mapped here: later stages must stay in it.
		if patchErr := d.patchTask(ctx, taskKey, map[string]string{"repository": pin}); patchErr != nil {
			log.Printf("[Agent] Could not pin task %s to %s: %v", taskKey, pin, patchErr)
		}
		task.Repository = pin
	}
	root = primary
	workDir, branch, err := ensureLocalWorktree(ctx, root, task, config.UseWorktrees)
	if err != nil {
		return config, "", "", "", task, err
	}
	if err := applySpecArtifacts(ctx, &config, root, task.Key); err != nil {
		return config, "", "", "", task, err
	}
	preserved, err := agentconfig.Scaffold(workDir, config)
	for _, path := range preserved {
		log.Printf("[Agent] Saved previous skill content: %s", path)
	}
	if err != nil {
		return config, root, workDir, branch, task, err
	}
	err = d.bootstrapLocalMCP(&config)
	return config, root, workDir, branch, task, err
}

// bootstrapLocalMCP registers the Sectile MCP server for every agent the project
// sets up. A registration that cannot be written aborts the dispatch: an agent
// without MCP cannot transition stages or finish its run.
func (d *agentDaemon) bootstrapLocalMCP(config *agentconfig.Config) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	// The path is written into the registration native clients read later, so
	// a throwaway binary registers a command that stops resolving as soon as
	// the build cache is pruned. The client then starts, finds no MCP server
	// and exits at once, which reads as a failed run and nothing else.
	// A test binary is temporary by nature and registers into a directory the
	// test owns, so the guard would only break the suite. What it protects
	// against is a real agent leaving a registration behind that stops
	// resolving; temporaryExecutable itself is covered by its own test.
	if temporaryExecutable(executable) && !runningUnderTest() {
		log.Printf("[Agent] Refusing to register MCP with the temporary binary %s. "+
			"Run a built agent (make start) rather than `go run`.", executable)
		return fmt.Errorf("agent is running from a temporary build at %s; run a built binary so native clients keep resolving it", executable)
	}
	if strings.TrimSpace(config.AIProvider) == "" {
		config.AIProvider = "agy"
	}
	providers, err := agentconfig.SetupProviders(*config)
	if err != nil {
		return err
	}
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		return err
	}
	for _, provider := range providers {
		choice, selected := settings.MCPConnections[provider]
		var path string
		if selected {
			server := d.link.serverURL
			if choice.Target == "local" {
				server = d.loopback.url
			}
			path, err = agentconfig.ConfigureMCP(provider, executable, server, d.link.token, choice.Transport, choice.Target == "local")
		} else {
			path, err = agentconfig.BootstrapMCP(provider, executable, d.link.serverURL, d.link.token)
		}
		if err != nil {
			return fmt.Errorf("register the Sectile MCP server for provider %q: %w", provider, err)
		}
		log.Printf("[Agent] MCP registered for %s: %s", provider, path)
	}
	return nil
}

// quoteShell protects task text when it is passed through an interactive shell.
func quoteShell(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// words joins the parts of a provider invocation, eliding the ones that resolve
// to nothing so an absent model leaves the command line exactly as it was.
func words(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, " ")
}

func agentCommandLine(provider, template, model, prompt string, contexts ...agentCommandContext) (string, error) {
	return modeCommandLine(provider, template, model, prompt, models.SkillModeInteractive, contexts...)
}

// headlessCommandLine is the autonomous form of agentCommandLine. It covers only
// the providers whose headless invocation this repository attests: claude -p,
// codex exec, and vibe, which is already headless today. Guessing a flag for the
// others is worse than refusing: an unsupported flag either fails opaquely or is
// swallowed as prompt text. Adding a provider here is a one-line change once its
// headless mode is verified.
//
// A headless run also carries the provider's non-interactive approval mode. There
// is nobody to answer a permission prompt in a run with no terminal: without it
// the CLI is denied every tool it asks for, including the Sectile MCP tools that
// report the run and move the stage, so the run ends having only printed why it
// could not work and the board never moves. Only an attested flag is passed, for
// the same reason the provider list itself is attested.
func headlessCommandLine(provider, model, prompt string, addDirs ...string) (string, error) {
	modelFlag := strings.Join(agentconfig.ModelArgs(provider, model), " ")
	dirFlags := addDirArgs(provider, addDirs)
	reasoning := ""
	if engineStreamsReasoning(provider) {
		reasoning = strings.Join(reasoningOptions, " ")
	}
	switch provider {
	case "claude":
		return words("claude", "-p", "--permission-mode", "bypassPermissions", reasoning, modelFlag, quoteShell(prompt), dirFlags), nil
	case "codex":
		// codex exec is non-interactive, but its approval bypass flag is not
		// attested here: it is left to a custom template until it is verified.
		return words("codex", "exec", reasoning, modelFlag, quoteShell(prompt)), nil
	case "vibe":
		// vibe takes no model flag, so ModelArgs returns nothing for it.
		return words("vibe", "-p", "--auto-approve", reasoning, quoteShell(prompt)), nil
	default:
		return "", fmt.Errorf("provider %q has no headless mode: run this skill interactively, or configure an AI command template carrying a {mode:AUTONOMOUS|INTERACTIVE} placeholder", provider)
	}
}

// addDirArgs passes the task's other folders to a provider whose option for it
// is attested: Claude Code's --add-dir. Every other provider gets nothing,
// which is what headlessCommandLine does for any unattested flag; the folder
// map in the prompt still names the folders.
//
// --add-dir takes several values: written "--add-dir <path>", it goes on
// swallowing every argument that follows, the prompt included. The
// "--add-dir=<path>" form takes exactly one, wherever a template places it,
// and the built-in lines also put the options after the prompt.
func addDirArgs(provider string, dirs []string) string {
	if !strings.EqualFold(strings.TrimSpace(provider), "claude") {
		return ""
	}
	var args []string
	for _, dir := range dirs {
		if dir = strings.TrimSpace(dir); dir != "" {
			args = append(args, "--add-dir="+quoteShell(dir))
		}
	}
	return strings.Join(args, " ")
}

// templateProvider is the CLI a command template starts: its first word,
// without a directory.
func templateProvider(template string) string {
	fields := strings.Fields(template)
	if len(fields) == 0 {
		return ""
	}
	return filepath.Base(fields[0])
}

// reasoningOptions make an engine print what it is doing as it does it: one
// JSON object per line (the prose, the tool calls, then a final result message
// carrying the answer) instead of the answer alone. They are only ever added to
// a headless launch: an interactive session already shows all of this to the
// human watching it.
var reasoningOptions = []string{"--output-format", "stream-json", "--verbose"}

// engineStreamsReasoning says whether an engine can be asked for that stream.
// Only an engine that can is ever handed the options, so nothing is passed a
// flag it does not have, and adding an engine here is a one-line change once its
// stream format is attested: the reader in internal/runner is Claude's shape.
func engineStreamsReasoning(provider string) bool {
	return strings.EqualFold(strings.TrimSpace(provider), "claude")
}

// commandReadsReasoning says whether a command line asks its engine for the
// reasoning stream, which is the same question as "is this run's standard output
// a stream of JSON objects rather than an answer".
//
// It is read off the line that will really run rather than re-derived from the
// provider: the decision was made while building that line, through branches a
// configured template and a dedicated autonomous command leave early, and asking
// the provider again at the far end would answer for a branch that was not
// taken. A template asking for the stream itself is read as one, which is
// exactly right: its output is that stream.
func commandReadsReasoning(commandLine string) bool {
	return strings.Contains(commandLine, strings.Join(reasoningOptions, " "))
}

// liveSessionMode pins the mode of a launch that opens a live provider session.
// A discussion and a bare terminal are interactive by construction: the command
// they build opens the provider with no prompt to run, so forking it headless
// gives a CLI with no input at all, which exits at once on "Input must be
// provided". A project defaulting to autonomous must not turn those two launches
// into a run that cannot work.
func liveSessionMode(skillID, action, mode string) string {
	if models.NormalizeSkillID(skillID) == "discuss" || models.NormalizeSkillID(action) == "open_terminal" {
		return models.SkillModeInteractive
	}
	return mode
}

// modeCommandLine builds the command line for one resolved mode. A configured
// template still wins over the provider defaults, as it does today, but it owns
// the mode: without a {mode:...|...} placeholder it can only run what its author
// wrote, so an autonomous launch is refused rather than silently running the
// template's own mode.
func modeCommandLine(provider, template, model, prompt, mode string, contexts ...agentCommandContext) (string, error) {
	autonomous := models.NormalizeSkillMode(mode) == models.SkillModeAutonomous
	if strings.TrimSpace(template) != "" {
		if autonomous && !templateCarriesMode(template) {
			return "", fmt.Errorf("the configured AI command template decides the execution mode: add a {mode:AUTONOMOUS|INTERACTIVE} placeholder to it, or run this skill interactively")
		}
		return expandConfiguredTemplate(template, model, prompt, autonomous, contexts...)
	}
	var addDirs []string
	if len(contexts) > 0 {
		addDirs = contexts[0].AddDirs
	}
	if autonomous {
		return headlessCommandLine(provider, model, prompt, addDirs...)
	}
	modelFlag := strings.Join(agentconfig.ModelArgs(provider, model), " ")
	switch provider {
	case "agy":
		return words("agy", "-i", quoteShell(prompt)), nil
	case "claude":
		return words(provider, modelFlag, quoteShell(prompt), addDirArgs(provider, addDirs)), nil
	case "codex", "gemini":
		return words(provider, modelFlag, quoteShell(prompt)), nil
	case "vibe":
		return words("vibe", "-p", quoteShell(prompt)), nil
	case "cursor":
		return words("cursor", "agent", modelFlag, quoteShell(prompt)), nil
	default:
		return "", fmt.Errorf("unsupported AI provider %q; configure an AI command template", provider)
	}
}

// expandConfiguredTemplate builds the command line a configured template asks
// for. A template owns its command line: the model reaches it through its own
// {model} slot, never as a flag spliced in beside the template's words.
func expandConfiguredTemplate(template, model, prompt string, autonomous bool, contexts ...agentCommandContext) (string, error) {
	var launch agentCommandContext
	if len(contexts) > 0 {
		launch = contexts[0]
	}
	values := launch.values(prompt)
	resolved := resolveTemplateMode(template, autonomous)
	// {addDirs} is several options, already quoted, rather than one value: it
	// is spliced in before the quoting pass, and is empty for a provider whose
	// flag is not attested. It needs the provider the template runs, which is
	// the first word of the command it starts.
	resolved = strings.ReplaceAll(resolved, "{addDirs}", addDirArgs(templateProvider(resolved), launch.AddDirs))
	if configured := strings.TrimSpace(model); configured != "" {
		values["model"] = configured
	} else {
		// Nothing to quote: the slot leaves with the option it belongs to
		// rather than reaching the quoting pass and becoming an empty ''.
		resolved = agentconfig.ExpandModel(resolved, "")
	}
	return expandAgentTemplate(resolved, values)
}

// launchCommandLine builds the command line for one launch, from whichever of
// the two configured commands serves its mode.
//
// A command written for headless use answers for itself: it needs no
// {mode:...} marker, because its author wrote it for that mode and nothing has
// to be reinterpreted. Only the general command, asked to serve a mode it may
// not have been written for, has to declare that it can.
func launchCommandLine(config agentconfig.Config, model, prompt, mode string, contexts ...agentCommandContext) (string, error) {
	if models.NormalizeSkillMode(mode) == models.SkillModeAutonomous {
		if dedicated := strings.TrimSpace(config.AICommandTemplateAutonomous); dedicated != "" {
			return expandConfiguredTemplate(dedicated, model, prompt, true, contexts...)
		}
	}
	return modeCommandLine(config.AIProvider, config.AICommandTemplate, model, prompt, mode, contexts...)
}

func sameDirectory(a, b string) bool {
	first, err := os.Stat(a)
	if err != nil {
		return false
	}
	second, err := os.Stat(b)
	return err == nil && os.SameFile(first, second)
}

// LaunchModel is the model one launch runs against: the model chosen for that
// launch when there is one, otherwise the model the configured levels resolve
// for the skill. A launch names a single run, which is more specific than any
// per-skill entry, so it wins outright instead of being merged as a bare model.
//
// An identifier the shape rule rejects is refused here rather than resolved:
// the value is about to be placed on a command line this process runs through
// sh -c, so the agent checks it even though the server already did.
func LaunchModel(config agentconfig.Config, skillID, override string) (string, error) {
	if override = strings.TrimSpace(override); override != "" {
		if err := agentconfig.ValidModel(override); err != nil {
			return "", err
		}
		return override, nil
	}
	return agentconfig.ResolveModel(config, models.NormalizeSkillID(skillID)), nil
}

// launchEngine names the provider and the model a launch really runs against,
// once the rules that govern the command line have had their say: a template
// carries the model only through its {model} slot, and a provider without a
// model flag runs without one. It is what the run record ends up displaying.
func launchEngine(config agentconfig.Config, skillID, modelOverride, mode string) (string, string) {
	provider := strings.ToLower(strings.TrimSpace(config.AIProvider))
	if provider == "" {
		provider = "agy"
	}
	model, err := LaunchModel(config, skillID, modelOverride)
	if err != nil {
		return provider, ""
	}
	template := config.AICommandTemplate
	if models.NormalizeSkillMode(mode) == models.SkillModeAutonomous {
		if dedicated := strings.TrimSpace(config.AICommandTemplateAutonomous); dedicated != "" {
			template = dedicated
		}
	}
	return provider, agentconfig.EffectiveModel(provider, template, model)
}

// dispatchCommand distinguishes opening an interactive agent from running a skill.
// mode is the execution mode the server resolved for this launch; an empty value
// reads as interactive, which keeps an older server working. modelOverride is the
// model picked for this launch, empty when the user kept the configured one.
func dispatchCommand(config agentconfig.Config, taskKey, skillID, action, prompt, command, mode, modelOverride string, contexts ...agentCommandContext) (string, error) {
	skillID = models.NormalizeSkillID(skillID)
	action = models.NormalizeSkillID(action)
	// A discussion or a bare terminal has no skill, so it runs against the model
	// the project resolves rather than a per-skill one.
	live := func() (string, error) {
		return runner.InteractiveAgentLaunch(&models.Settings{
			AIProvider: config.AIProvider, AICommandTemplate: config.AICommandTemplate,
			AIModel: agentconfig.ResolveModel(config, ""),
		})
	}
	model, err := LaunchModel(config, skillID, modelOverride)
	if err != nil {
		return "", err
	}
	if action == "open_terminal" {
		if strings.TrimSpace(command) != "" {
			return command, nil
		}
		if skillID == "" {
			return live()
		}
	}
	// A discussion opens the provider as a live session and nothing else: no
	// skill command, and none of the prompt the dispatch carries for skills.
	if skillID == "discuss" {
		return live()
	}
	if skillID == "custom" {
		if strings.TrimSpace(prompt) == "" {
			return "", fmt.Errorf("custom instructions required")
		}
		return launchCommandLine(config, model, "Sectile task: "+taskKey+"\n\n"+prompt, mode, contexts...)
	}
	skillCmd := ""
	for _, skill := range config.Skills {
		if skillID == skill.ID || skillID == skill.Directory || action == skill.ID {
			if skill.RequiresReconciliation {
				return "", fmt.Errorf("legacy customization requires reconciliation in Skills before adjustment")
			}
			skillCmd = skill.Command
			break
		}
	}
	if skillCmd == "" {
		return "", fmt.Errorf("unknown configured skill %q", skillID)
	}
	if !strings.HasPrefix(skillCmd, "/") {
		skillCmd = "/" + skillCmd
	}
	promptArg := skillCmd + " " + taskKey
	if strings.HasPrefix(strings.TrimSpace(prompt), "/") {
		promptArg = prompt
	} else if strings.TrimSpace(prompt) != "" {
		promptArg += "\n\n" + prompt
	}
	if skillID == "adjust" {
		promptArg += "\n\n" + runner.AdjustmentContract
	}
	return launchCommandLine(config, model, promptArg, mode, contexts...)
}

func (d *agentDaemon) discoverProjects(ctx context.Context) (agentconfig.Projects, error) {
	var projects agentconfig.Projects
	if err := d.readAPI(ctx, "/api/v1/agent/projects", &projects); err != nil {
		return projects, err
	}
	if projects.SchemaVersion != agentconfig.Version {
		return projects, d.contract.note(&agentconfig.Mismatch{Server: d.link.serverURL, Route: "/api/v1/agent/projects", Served: projects.SchemaVersion})
	}
	d.contract.clear()
	return projects, nil
}

// syncLocalProject does not redeploy a disconnected project during reconnection.
func (d *agentDaemon) syncLocalProject(ctx context.Context, config agentconfig.Config) error {
	d.prepareMu.Lock()
	defer d.prepareMu.Unlock()
	root, overrides, err := d.localProjectRoot(ctx, config)
	if overrides.DisconnectedProjects[config.ProjectID] {
		return nil
	}
	if err != nil {
		return err
	}
	config = agentconfig.ApplyOverrides(config, overrides)
	if err := config.Validate(); err != nil {
		return err
	}
	if _, err := agentconfig.Scaffold(root, config); err != nil {
		return err
	}
	return d.bootstrapLocalMCP(&config)
}

// temporaryExecutable reports a binary the toolchain may delete, such as what
// `go run` builds into the module cache.
func temporaryExecutable(path string) bool {
	for _, marker := range []string{"/go-build", string(os.PathSeparator) + "T" + string(os.PathSeparator), os.TempDir()} {
		if marker != "" && marker != "/" && strings.Contains(path, marker) {
			return true
		}
	}
	return false
}
