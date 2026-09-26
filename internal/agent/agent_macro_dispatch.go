package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"tasks/internal/sddfiles"
)

// localSpecRepo is the folder carrying a project's specifications on this
// workstation: the workstation's own override when there is one, else the
// project's checkout (#484). The server holds no specifications path: it would
// name a directory on another machine.
func localSpecRepo(overrides agentconfig.Settings, projectID, root string) (string, error) {
	mapped := strings.TrimSpace(overrides.ProjectSettings[projectID].SpecPath)
	if mapped == "" {
		return root, nil
	}
	if !filepath.IsAbs(mapped) {
		mapped = filepath.Join(root, mapped)
	}
	if _, err := os.Stat(mapped); err != nil {
		return "", fmt.Errorf("le dossier des spécifications %s déclaré sur ce poste est introuvable", mapped)
	}
	return mapped, nil
}

// handleMacroDispatch runs a macro-scoped skill. It follows a task dispatch
// where it can (admission, queue slot, skills, MCP, supervised console) and
// leaves out everything a task carries: no task to read, no task worktree, no
// branch to record. The skill runs in the project's checkout, where its skills
// and AGENTS.md are, and writes in the macro worktree the environment names.
func (d *agentDaemon) handleMacroDispatch(ctx context.Context, conn *websocket.Conn, msg agentprotocol.Message, payload agentconfig.Dispatch) {
	macroKey := strings.ToUpper(strings.TrimSpace(payload.MacroKey))
	if payload.RunID == "" {
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", "A run ID is required for supervised execution")
		return
	}
	if strings.TrimSpace(payload.ProjectID) == "" || msg.TaskID != "" || payload.TaskID != "" {
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", "A macro dispatch names its project and no task")
		return
	}
	config, err := d.fetchConfig(ctx, payload.ProjectID, "")
	if err != nil {
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", err.Error())
		return
	}
	// A macro skill is a conversation about a specification: it always runs
	// in a terminal the user answers.
	payload.Mode = models.SkillModeInteractive
	payload.TaskKey = macroKey
	payload.MacroKey = macroKey
	run, err := d.admitProjectRun(ctx, "", payload, config)
	if err != nil {
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", err.Error())
		return
	}
	d.sendStatus(conn, msg.MsgID, msg.TaskID, "completed", "Execution accepted into the local queue")

	launched := false
	var launchFailure error
	defer func() {
		if launched {
			return
		}
		d.queue.mu.Lock()
		status := "failed"
		if run.canceled {
			status = "canceled"
		}
		run.desktop.Status = status
		run.once.Do(func() { close(run.exited) })
		d.queue.mu.Unlock()
		note := ""
		if launchFailure != nil {
			note = "Execution never started: " + launchFailure.Error()
		}
		_ = d.finishDesktopRun(context.Background(), "", payload.RunID, status, note)
	}()
	if err := d.awaitRunSlot(ctx, run); err != nil {
		return
	}

	config, root, workspace, err := d.prepareMacroWorkspace(ctx, config, macroKey, payload.MacroTitle)
	if err != nil {
		launchFailure = err
		return
	}
	prompt := strings.TrimSpace(payload.Prompt)
	if workspace.Warning != "" {
		prompt += "\nMacro workspace notice: " + workspace.Warning + "."
	}
	prompt += fmt.Sprintf("\nRemote execution runId: %s. Reuse this ID with start_run and finish it using finish_run (projectId %s, macroKey %s) when the entire skill ends.", payload.RunID, config.ProjectID, macroKey)
	fullLine, err := dispatchCommand(config, macroKey, payload.SkillID, payload.Action, strings.TrimSpace(prompt), payload.Command, payload.Mode, payload.Model,
		agentCommandContext{Branch: workspace.Branch, Directory: root, Tracker: config.IssueTracker, Repo: config.GithubRepo})
	if err != nil {
		launchFailure = err
		return
	}
	runProvider, runModel := launchEngine(config, payload.SkillID, payload.Model, payload.Mode)
	go d.postRunEngine(payload.RunID, runProvider, runModel)
	fullLine, err = d.wrapRun("", payload.RunID, fullLine)
	if err != nil {
		launchFailure = err
		return
	}
	envVars := map[string]string{
		"SECTILE_TASK_KEY": "", "SECTILE_TASK_ID": "", "SECTILE_TASK_BRANCH": "", "SECTILE_TASK_WORKTREE": "",
		"SECTILE_MACRO_KEY":        macroKey,
		"SECTILE_MACRO_PROJECT_ID": config.ProjectID,
		"SECTILE_SPEC_REPO":        workspace.Path,
		"SECTILE_SPEC_BRANCH":      workspace.Branch,
		"SECTILE_SPEC_WORKTREE":    fmt.Sprint(workspace.Worktree),
		"SECTILE_PROJECT_ID":       config.ProjectID,
		"SECTILE_RUN_ID":           payload.RunID,
		"SECTILE_REMOTE_MODE":      "true",
		"SECTILE_AGENT_URL":        d.link.serverURL,
		"SECTILE_SERVER_URL":       d.link.serverURL,
		"SECTILE_AGENT_TOKEN":      d.link.token,
		"SECTILE_LOOPBACK_URL":     d.loopback.url,
	}
	sessionID := payload.RunID
	if d.terminal.manager == nil {
		launchFailure = fmt.Errorf("terminal manager unavailable")
		return
	}
	if _, err := d.terminal.manager.GetOrCreateSession(sessionID, root, envVars); err != nil {
		launchFailure = err
		return
	}
	d.queue.read(payload.RunID, func(run *controlledRun) {
		run.desktop = desktopRun{CreatedAt: run.desktop.CreatedAt, Prompt: run.desktop.Prompt, ID: payload.RunID, TaskKey: macroKey, MacroKey: macroKey,
			ProjectID: config.ProjectID, Skill: payload.SkillID, SessionID: sessionID, Directory: root, Branch: workspace.Branch, Status: "running",
			Provider: runProvider, Model: runModel}
	})
	if err := d.runInPty(sessionID, root, envVars, fullLine); err != nil {
		launchFailure = err
		return
	}
	launched = true
	log.Printf("[Agent] Macro skill %s launched for %s in %s (spec checkout %s on %s)", payload.SkillID, macroKey, root, workspace.Path, workspace.Branch)
}

// prepareMacroWorkspace installs the project's skills and MCP registration, as
// a task dispatch does, then prepares the macro worktree in the specifications
// repository. It returns the project checkout the skill runs in.
func (d *agentDaemon) prepareMacroWorkspace(ctx context.Context, config agentconfig.Config, macroKey, title string) (agentconfig.Config, string, macroWorkspace, error) {
	config, root, spec, err := d.prepareMacroSkills(ctx, config)
	if err != nil {
		return config, "", macroWorkspace{}, err
	}
	// The worktree is prepared outside prepareMu: it fetches, and a slow remote
	// must not hold every other launch of the agent behind it. The worktree has
	// its own per-repository lock.
	workspace, err := ensureMacroWorktree(ctx, spec, macroKey, title, config.UseWorktrees)
	return config, root, workspace, err
}

// prepareMacroSkills is the part of a macro launch that runs under prepareMu:
// the checkout mapping, the skills and the MCP registration.
func (d *agentDaemon) prepareMacroSkills(ctx context.Context, config agentconfig.Config) (agentconfig.Config, string, string, error) {
	d.prepareMu.Lock()
	defer d.prepareMu.Unlock()
	root, overrides, err := d.localProjectRoot(ctx, config)
	if err != nil {
		return config, "", "", err
	}
	config = agentconfig.Resolve(config, overrides)
	if err := config.Validate(); err != nil {
		return config, "", "", err
	}
	preserved, err := agentconfig.Scaffold(root, config)
	for _, path := range preserved {
		log.Printf("[Agent] Saved previous skill content: %s", path)
	}
	if err != nil {
		return config, "", "", err
	}
	if err := d.bootstrapLocalMCP(&config); err != nil {
		return config, "", "", err
	}
	spec, err := localSpecRepo(overrides, config.ProjectID, root)
	return config, root, spec, err
}

// macroWorkspaceFor answers the macro_worktree operation: the same preparation
// as a launch, for a skill somebody invoked by hand in their own session.
func (d *agentDaemon) macroWorkspaceFor(ctx context.Context, projectID, macroKey, title string) (macroWorkspace, error) {
	config, err := d.fetchConfig(ctx, projectID, "")
	if err != nil {
		return macroWorkspace{}, err
	}
	d.prepareMu.Lock()
	root, overrides, err := d.localProjectRoot(ctx, config)
	d.prepareMu.Unlock()
	if err != nil {
		return macroWorkspace{}, err
	}
	config = agentconfig.Resolve(config, overrides)
	spec, err := localSpecRepo(overrides, config.ProjectID, root)
	if err != nil {
		return macroWorkspace{}, err
	}
	return ensureMacroWorktree(ctx, spec, macroKey, title, config.UseWorktrees)
}

// macroSpecFileFor answers the macro_spec_file operation: the server's
// slicing import reads the macro's specification in this workstation's
// specifications folder, since the server's disk holds none.
func (d *agentDaemon) macroSpecFileFor(ctx context.Context, projectID, macroKey, framework, fileName string) (agentprotocol.MacroSpecFile, error) {
	config, err := d.fetchConfig(ctx, projectID, "")
	if err != nil {
		return agentprotocol.MacroSpecFile{}, err
	}
	d.prepareMu.Lock()
	root, overrides, err := d.localProjectRoot(ctx, config)
	d.prepareMu.Unlock()
	if err != nil {
		return agentprotocol.MacroSpecFile{}, err
	}
	folder, err := localSpecRepo(overrides, config.ProjectID, root)
	if err != nil {
		return agentprotocol.MacroSpecFile{}, err
	}
	if strings.TrimSpace(framework) == "" {
		framework = config.SpecFramework
	}
	content, origin, err := sddfiles.Read(ctx, folder, framework, macroKey, fileName)
	if err != nil {
		return agentprotocol.MacroSpecFile{}, err
	}
	return agentprotocol.MacroSpecFile{Content: content, Origin: origin}, nil
}

// macroRunIdentity is what reporting a macro run's end needs.
type macroRunIdentity struct{ projectID, macroKey string }

// macroRunIdentities remembers the project and macro of every macro run from
// admission until its end is reported. The queue alone is not enough: clearing
// finished executions from the desktop removes a run from it, possibly before
// the goroutine reporting its exit has read it, and the macro would then stay
// busy on the server for good.
var macroRunIdentities sync.Map

func (d *agentDaemon) rememberMacroRun(runID, projectID, macroKey string) {
	macroRunIdentities.Store(runID, macroRunIdentity{projectID, macroKey})
}

func (d *agentDaemon) forgetMacroRun(runID string) { macroRunIdentities.Delete(runID) }

// macroOfRun returns the project and macro of a macro run, "" for any other.
func (d *agentDaemon) macroOfRun(runID string) (projectID, macroKey string) {
	if value, ok := macroRunIdentities.Load(runID); ok {
		identity := value.(macroRunIdentity)
		return identity.projectID, identity.macroKey
	}
	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()
	if run := d.queue.runs[runID]; run != nil {
		return run.desktop.ProjectID, run.desktop.MacroKey
	}
	return "", ""
}
