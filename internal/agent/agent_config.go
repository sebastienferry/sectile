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
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	raw, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(string(raw)))
	}
	return strings.TrimSpace(string(raw)), nil
}

func repositoryIdentity(remote string) string {
	remote = strings.TrimSuffix(strings.TrimSpace(remote), ".git")
	if parsed, err := url.Parse(remote); err == nil && parsed.Host != "" {
		return strings.ToLower(parsed.Host) + strings.TrimRight(parsed.Path, "/")
	}
	remote = strings.TrimPrefix(remote, "git@")
	return strings.Replace(remote, ":", "/", 1)
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
		if err != nil || c.GitRemoteURL == "" || repositoryIdentity(remote) != repositoryIdentity(c.GitRemoteURL) {
			return "", overrides, fmt.Errorf("no local repository mapping for project %s; configure ~/.config/taskflow/settings.json projects", c.ProjectID)
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
	if overrides.AIProvider != "" {
		local.AIProvider = overrides.AIProvider
	}
	if overrides.AICommandTemplate != "" {
		local.AICommandTemplate = overrides.AICommandTemplate
	}
	if overrides.Terminal != "" {
		local.Terminal = overrides.Terminal
	}
	if local.Skills == nil {
		local.Skills = map[string]string{}
	}
	for id, content := range overrides.Skills {
		local.Skills[id] = content
	}
	return root, local, nil
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
	target := filepath.Join(root, ".tasks", "worktrees", task.Key)
	if _, err := os.Stat(target); err == nil {
		top, err := gitLocal(ctx, target, "rev-parse", "--show-toplevel")
		if err != nil || !sameDirectory(top, target) {
			return "", "", fmt.Errorf("existing task path is not a worktree: %s", target)
		}
		current, err := gitLocal(ctx, target, "branch", "--show-current")
		if err != nil || current != branch {
			return "", "", fmt.Errorf("existing worktree does not use assigned branch %s", branch)
		}
		return target, branch, nil
	} else if !os.IsNotExist(err) {
		return "", "", err
	}
	// A task may already own the main checkout, including its uncommitted work.
	// Git cannot check out that branch again in a new worktree.
	current, err := gitLocal(ctx, root, "branch", "--show-current")
	if err != nil {
		return "", "", err
	}
	if task.BranchName != nil && strings.TrimSpace(*task.BranchName) != "" && current == branch {
		return root, branch, nil
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

func (d *agentDaemon) prepareDispatch(ctx context.Context, taskKey string, useWorktrees ...bool) (agentconfig.Config, string, string, models.Task, error) {
	d.prepareMu.Lock()
	defer d.prepareMu.Unlock()
	var task models.Task
	config, err := d.fetchConfig(ctx, "", taskKey)
	if err != nil {
		return config, "", "", task, err
	}
	root, overrides, err := d.localProjectRoot(ctx, config)
	if err != nil {
		return config, "", "", task, err
	}
	if err := d.readAPI(ctx, "/api/tasks/"+url.PathEscape(taskKey), &task); err != nil {
		return config, "", "", task, err
	}
	if task.ProjectID != config.ProjectID {
		return config, "", "", task, fmt.Errorf("task project changed during configuration sync")
	}
	config = agentconfig.ApplyOverrides(config, overrides)
	if len(useWorktrees) > 0 {
		config.UseWorktrees = useWorktrees[0]
	}
	if err := config.Validate(); err != nil {
		return config, "", "", task, err
	}
	workDir, branch, err := ensureLocalWorktree(ctx, root, task, config.UseWorktrees)
	if err != nil {
		return config, "", "", task, err
	}
	preserved, err := agentconfig.Scaffold(workDir, config)
	for _, path := range preserved {
		log.Printf("[Agent] Saved previous skill content: %s", path)
	}
	if err != nil {
		return config, workDir, branch, task, err
	}
	err = d.bootstrapLocalMCP(&config)
	return config, workDir, branch, task, err
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
	for _, provider := range providers {
		path, err := agentconfig.BootstrapMCP(provider, executable, d.link.serverURL, d.link.token)
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
func headlessCommandLine(provider, model, prompt string) (string, error) {
	modelFlag := strings.Join(agentconfig.ModelArgs(provider, model), " ")
	switch provider {
	case "claude":
		return words("claude", "-p", "--permission-mode", "bypassPermissions", modelFlag, quoteShell(prompt)), nil
	case "codex":
		// codex exec is non-interactive, but its approval bypass flag is not
		// attested here: it is left to a custom template until it is verified.
		return words("codex", "exec", modelFlag, quoteShell(prompt)), nil
	case "vibe":
		// vibe takes no model flag, so ModelArgs returns nothing for it.
		return "vibe -p --auto-approve " + quoteShell(prompt), nil
	default:
		return "", fmt.Errorf("provider %q has no headless mode: run this skill interactively, or configure an AI command template carrying a {mode:AUTONOMOUS|INTERACTIVE} placeholder", provider)
	}
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
	if autonomous {
		return headlessCommandLine(provider, model, prompt)
	}
	modelFlag := strings.Join(agentconfig.ModelArgs(provider, model), " ")
	switch provider {
	case "agy":
		return words("agy", "-i", quoteShell(prompt)), nil
	case "claude", "codex", "gemini":
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
