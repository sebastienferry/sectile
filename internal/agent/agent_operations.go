package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"tasks/internal/runner"
	"tasks/internal/workspace"
)

// operationRegistry tracks the in-flight workspace operations that the server
// may still cancel, keyed by the message that started them. It owns its mutex.
type operationRegistry struct {
	mu      sync.Mutex
	pending map[string]context.CancelFunc
}

// claim registers cancel under id and reports whether the claim was free. A
// duplicate id means the server re-sent a message that is already running, so
// the caller drops the new one rather than shadowing the live operation.
func (o *operationRegistry) claim(id string, cancel context.CancelFunc) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.pending == nil {
		o.pending = map[string]context.CancelFunc{}
	}
	if _, exists := o.pending[id]; exists {
		return false
	}
	o.pending[id] = cancel
	return true
}

// release forgets an operation once it has finished, cancelled or not.
func (o *operationRegistry) release(id string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.pending, id)
}

// cancelFunc returns the canceller for a live operation, nil when the operation
// already finished or was never claimed.
func (o *operationRegistry) cancelFunc(id string) context.CancelFunc {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.pending[id]
}

func (d *agentDaemon) startOperation(ctx context.Context, conn *websocket.Conn, msg agentprotocol.Message) {
	ctx, cancel := context.WithCancel(ctx)
	if !d.operations.claim(msg.MsgID, cancel) {
		cancel()
		return
	}
	go func() {
		defer cancel()
		defer d.operations.release(msg.MsgID)
		d.handleOperation(ctx, conn, msg)
	}()
}

func (d *agentDaemon) handleOperation(ctx context.Context, conn *websocket.Conn, msg agentprotocol.Message) {
	var op agentprotocol.Operation
	res := agentprotocol.Result{}
	if err := json.Unmarshal(msg.Payload, &op); err != nil {
		res.Error = "invalid operation request"
	} else {
		value, err := d.executeOperation(ctx, op)
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Value, err = json.Marshal(value)
			if err != nil {
				res.Error = err.Error()
			}
		}
	}
	raw, _ := json.Marshal(res)
	_ = d.link.write(conn, agentprotocol.Message{MsgID: msg.MsgID, TaskID: msg.TaskID, Type: "workspace_result", Payload: raw})
}
func (d *agentDaemon) executeOperation(ctx context.Context, op agentprotocol.Operation) (any, error) {
	if op.ProjectID == "" {
		return nil, fmt.Errorf("project primary key is required")
	}
	switch op.Action {
	case "git_status", "git_branches", "git_checkout", "git_clean", "git_delete", "open_editor", "cli_status", "prepare_workspace", "remove_workspace", "workspace_info", "git_diff", "git_evidence", "run_prompt", "skills_status", "skill_files", "sync_config", "read_skill", "spec_status", "spec_install", "init_git":
	default:
		return nil, fmt.Errorf("unknown local operation %q", op.Action)
	}
	config, err := d.fetchConfig(ctx, op.ProjectID, op.TaskID, op.Framework)
	if err != nil {
		return nil, err
	}
	if config.ProjectID != op.ProjectID {
		return nil, fmt.Errorf("project mismatch")
	}
	root, overrides, err := d.localProjectRoot(ctx, config, op.Action == "init_git")
	if err != nil {
		return nil, err
	}
	config = agentconfig.ApplyOverrides(config, overrides)
	if err := config.Validate(); err != nil {
		return nil, err
	}
	var task models.Task
	target := root
	if op.TaskID != "" {
		if err := d.readAPI(ctx, "/api/tasks/"+op.TaskID, &task); err != nil {
			return nil, err
		}
		if task.ID != op.TaskID || task.ProjectID != op.ProjectID {
			return nil, fmt.Errorf("task identity mismatch")
		}
		if config.UseWorktrees {
			if !filepath.IsLocal(task.Key) || strings.ContainsAny(task.Key, "/\\") {
				return nil, fmt.Errorf("invalid task key")
			}
			target, err = localTaskPath(ctx, root, task)
			if err != nil && op.Action != "prepare_workspace" {
				return nil, err
			}
		}
	}
	local := workspace.New(ctx)
	r := runner.NewRunner()
	switch op.Action {
	case "sync_config":
		if op.Framework != "" {
			config.SpecFramework = op.Framework
		}
		if op.Provider != "" {
			config.AIProvider = op.Provider
		}
		if op.AICommandTemplate != "" {
			config.AICommandTemplate = op.AICommandTemplate
		}
		if err := config.Validate(); err != nil {
			return nil, err
		}
		d.prepareMu.Lock()
		defer d.prepareMu.Unlock()
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		paths, err := localWorktreePaths(ctx, root)
		if err != nil {
			return nil, err
		}
		// The installation is user-level, so it is written once; every checkout is
		// still visited to retire the copies earlier releases left in it.
		for _, path := range paths {
			if _, err = agentconfig.Scaffold(path, config); err != nil {
				return nil, err
			}
		}
		if err = d.bootstrapLocalMCP(&config); err != nil {
			return nil, err
		}
		loc, err := agentconfig.ResolveLocations(agentconfig.EffectiveProvider(config.AIProvider, config.AICommandTemplate))
		if err != nil {
			return nil, err
		}
		written := 0
		if loc.InstallsSkills() {
			written = len(config.Skills)
		}
		return map[string]any{"written": written}, nil
	case "skill_files", "read_skill":
		files, err := localSkillFiles(config)
		if err != nil {
			return nil, err
		}
		if op.Action == "skill_files" {
			return files, nil
		}
		file := files[models.NormalizeSkillID(op.SkillID)]
		if len(file.Paths) == 0 || strings.TrimSpace(file.Content) == "" {
			return nil, fmt.Errorf("local skill file not found")
		}
		return map[string]string{"content": file.Content}, nil
	case "skills_status":
		loc, err := agentconfig.ResolveLocations(agentconfig.EffectiveProvider(config.AIProvider, config.AICommandTemplate))
		if err != nil {
			return nil, err
		}
		status := models.ProjectSkillsStatus{ProjectID: config.ProjectID, ProjectName: config.ProjectName, RepoPath: root, PathExists: true, IsGitRepo: true, InstalledAll: true, SpecFramework: config.SpecFramework, WorktreePaths: []string{root}, WorktreesCount: 1}
		paths, err := localWorktreePaths(ctx, root)
		if err != nil {
			return nil, err
		}
		status.WorktreePaths = paths
		status.WorktreesCount = len(paths)
		branch, _ := gitLocal(ctx, root, "branch", "--show-current")
		status.GitBranch = branch
		for _, skill := range config.Skills {
			info := models.InstalledSkillInfo{ID: skill.ID, Name: skill.Directory}
			if loc.InstallsSkills() {
				path := filepath.Join(loc.Home, loc.SkillDir, skill.Directory, "SKILL.md")
				if fi, err := os.Stat(path); err == nil && fi.Mode().IsRegular() {
					info.Installed = true
					info.Path = path
				}
			}
			status.InstalledAll = status.InstalledAll && info.Installed
			status.Skills = append(status.Skills, info)
		}
		// Nothing is missing when the provider has no skill convention.
		if !loc.InstallsSkills() {
			status.InstalledAll = true
		}
		return status, nil
	case "spec_status":
		frameworks := []string{"speckit", "openspec"}
		if op.Framework != "" {
			frameworks = []string{op.Framework}
		}
		var result []models.SpecFrameworkStatus
		for _, framework := range frameworks {
			result = append(result, *r.GetSpecFrameworkStatus(framework, root))
		}
		return result, nil
	case "spec_install":
		framework := op.Framework
		if framework == "" {
			framework = config.SpecFramework
		}
		provider := op.Provider
		if provider == "" {
			provider = config.AIProvider
		}
		return r.InstallSpecFrameworkContext(ctx, models.SpecFrameworkInstallRequest{Framework: framework, RepoPath: root, AIAgent: provider, Force: op.Force}), nil
	case "init_git":
		_, err := gitLocal(ctx, root, "init")
		if err != nil {
			return nil, err
		}
		branch, err := gitLocal(ctx, root, "branch", "--show-current")
		return models.ProjectGitInitResult{RepoPath: root, IsGitRepo: true, Branch: branch, Initialized: err == nil, Message: "Repository initialized by the local agent"}, err
	case "prepare_workspace":
		if op.TaskID == "" {
			return nil, fmt.Errorf("task is required")
		}
		// prepareWorkspace takes prepareMu itself; locking it here as well made
		// the operation wait on its own lock forever.
		_, projectRoot, dir, branch, t, err := d.prepareWorkspace(ctx, op.TaskID)
		if err != nil {
			return nil, err
		}
		// The server sends this operation for a branch checkout from the board,
		// which waits on it synchronously. The answer goes back as soon as the
		// worktree exists, and the install runs behind it: the per-worktree lock
		// makes a launch that follows wait for it instead of installing twice.
		// The operation's context is cancelled once it answers, so the install
		// keeps only its own timeouts.
		go provisionWorktree(context.WithoutCancel(ctx), projectRoot, dir)
		return models.WorktreeInfo{TaskKey: t.Key, Branch: branch, WorktreePath: dir, MainRepoPath: root, Exists: config.UseWorktrees}, nil
	case "workspace_info":
		branch := ""
		if task.BranchName != nil {
			branch = *task.BranchName
		}
		fi, err := os.Stat(target)
		return models.WorktreeInfo{TaskKey: task.Key, Branch: branch, WorktreePath: target, MainRepoPath: root, Exists: config.UseWorktrees && err == nil && fi.IsDir()}, nil
	case "remove_workspace":
		if op.TaskID == "" || !config.UseWorktrees || filepath.Clean(target) == filepath.Clean(root) {
			return nil, fmt.Errorf("task has no isolated worktree")
		}
		_, err := gitLocal(ctx, root, "worktree", "remove", target)
		return nil, err
	case "git_diff":
		branch := ""
		if task.BranchName != nil {
			branch = *task.BranchName
		}
		return r.GetGitDiff(target, branch, task.Key, task.PrURL)
	case "git_evidence":
		sha, err := gitLocal(ctx, target, "rev-parse", "HEAD")
		if err != nil {
			return nil, err
		}
		status, err := gitLocal(ctx, target, "status", "--porcelain")
		if err != nil {
			return nil, err
		}
		branch, err := gitLocal(ctx, target, "branch", "--show-current")
		return map[string]any{"sha": strings.TrimSpace(sha), "branch": strings.TrimSpace(branch), "clean": strings.TrimSpace(status) == ""}, err
	case "run_prompt":
		output, steps, err := r.RunAgentPrompt(ctx, &models.Settings{RepoPath: root, AIProvider: config.AIProvider, AICommandTemplate: config.AICommandTemplate, AIModel: agentconfig.ResolveModel(config, "")}, op.Prompt)
		return map[string]any{"output": output, "steps": steps}, err
	case "git_status":
		return r.GetCwdGitStatus(root)
	case "git_branches":
		return local.GetGitBranches(root)
	case "git_checkout":
		return local.SwitchGitBranch(root, op.Branch, op.Create)
	case "git_clean":
		return local.CleanAllLocalBranches(root)
	case "git_delete":
		return nil, local.DeleteGitBranch(root, op.Branch, op.DeleteRemote)
	case "open_editor":
		return nil, r.OpenInEditor(op.Editor, target)
	case "cli_status":
		return r.CheckCliTools(root), nil
	}
	return nil, fmt.Errorf("unsupported operation")
}

// Resolve the checkout from local Git metadata. A server path is never trusted,
// and an assigned branch already checked out elsewhere must be reused.
func localTaskPath(ctx context.Context, root string, task models.Task) (string, error) {
	target := filepath.Join(root, ".tasks", "worktrees", task.Key)
	branch := ""
	if task.BranchName != nil {
		branch = strings.TrimSpace(*task.BranchName)
	}
	if branch != "" {
		out, err := gitLocal(ctx, root, "worktree", "list", "--porcelain")
		if err != nil {
			return "", err
		}
		current := ""
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, "worktree ") {
				current = strings.TrimPrefix(line, "worktree ")
			}
			if line == "branch refs/heads/"+branch && current != "" {
				return current, nil
			}
		}
	}
	if fi, err := os.Stat(target); err == nil && fi.IsDir() && branch != "" {
		actual, err := gitLocal(ctx, target, "branch", "--show-current")
		if err != nil || actual != branch {
			return "", fmt.Errorf("task checkout does not match assigned branch %s", branch)
		}
	}
	return target, nil
}
