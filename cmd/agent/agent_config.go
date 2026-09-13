package main

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

	"tasks/internal/agentconfig"
	"tasks/internal/models"
	"tasks/internal/runner"
)

func (d *agentDaemon) readAPI(ctx context.Context, path string, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.serverURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := agentHTTPClient(d.token).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var detail struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&detail)
		if detail.Error != "" {
			return fmt.Errorf("TaskFlow API returned HTTP %d: %s", resp.StatusCode, detail.Error)
		}
		return fmt.Errorf("TaskFlow API returned HTTP %d", resp.StatusCode)
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
	} else if d.projectID != c.ProjectID {
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
	err = d.bootstrapLocalMCP(workDir, &config)
	return config, workDir, branch, task, err
}

func (d *agentDaemon) bootstrapLocalMCP(workDir string, config *agentconfig.Config) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	provider := strings.ToLower(strings.TrimSpace(config.AIProvider))
	if provider == "" {
		provider = "agy"
		config.AIProvider = provider
	}
	if provider == "custom" {
		words := strings.Fields(config.AICommandTemplate)
		if len(words) > 0 {
			provider = filepath.Base(strings.Trim(words[0], "\"'"))
		}
	}
	_, err = agentconfig.BootstrapMCP(workDir, provider, executable, d.agentURL)
	return err
}

// quoteShell protects task text when it is passed through an interactive shell.
func quoteShell(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func agentCommandLine(provider, template, prompt string, contexts ...agentCommandContext) (string, error) {
	if strings.TrimSpace(template) != "" {
		var launch agentCommandContext
		if len(contexts) > 0 {
			launch = contexts[0]
		}
		return expandAgentTemplate(template, launch.values(prompt))
	}
	switch provider {
	case "agy":
		return "agy -i " + quoteShell(prompt), nil
	case "claude", "codex", "gemini":
		return provider + " " + quoteShell(prompt), nil
	case "vibe":
		return "vibe -p " + quoteShell(prompt), nil
	case "cursor":
		return "cursor agent " + quoteShell(prompt), nil
	default:
		return "", fmt.Errorf("unsupported AI provider %q; configure an AI command template", provider)
	}
}

func sameDirectory(a, b string) bool {
	first, err := os.Stat(a)
	if err != nil {
		return false
	}
	second, err := os.Stat(b)
	return err == nil && os.SameFile(first, second)
}

// dispatchCommand distinguishes opening an interactive agent from running a skill.
func dispatchCommand(config agentconfig.Config, taskKey, skillID, action, prompt, command string, contexts ...agentCommandContext) (string, error) {
	skillID = models.NormalizeSkillID(skillID)
	action = models.NormalizeSkillID(action)
	if action == "open_terminal" {
		if strings.TrimSpace(command) != "" {
			return command, nil
		}
		if skillID == "" {
			return runner.InteractiveAgentLaunch(&models.Settings{AIProvider: config.AIProvider, AICommandTemplate: config.AICommandTemplate})
		}
	}
	if skillID == "custom" {
		if strings.TrimSpace(prompt) == "" {
			return "", fmt.Errorf("custom instructions required")
		}
		return agentCommandLine(config.AIProvider, config.AICommandTemplate, "TaskFlow task: "+taskKey+"\n\n"+prompt, contexts...)
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
	return agentCommandLine(config.AIProvider, config.AICommandTemplate, promptArg, contexts...)
}

func (d *agentDaemon) discoverProjects(ctx context.Context) (agentconfig.Projects, error) {
	var projects agentconfig.Projects
	if err := d.readAPI(ctx, "/api/v1/agent/projects", &projects); err != nil {
		return projects, err
	}
	if projects.SchemaVersion != agentconfig.Version {
		return projects, fmt.Errorf("unsupported project discovery schemaVersion %d", projects.SchemaVersion)
	}
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
	return d.bootstrapLocalMCP(root, &config)
}
