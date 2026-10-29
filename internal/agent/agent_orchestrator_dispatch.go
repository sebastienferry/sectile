package agent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
)

// handleOrchestratorDispatch runs a project's orchestrator session (ADR 0064).
// It follows a macro dispatch (admission, queue slot, skills, MCP, supervised
// console) without a macro worktree: the session reads the backlog through MCP
// and writes nothing in a repository. It always runs headless. Its policy is
// the named orchestration agent's Markdown file, read here at each launch so an
// edit applies to the next session; the server never holds the file.
func (d *agentDaemon) handleOrchestratorDispatch(ctx context.Context, conn *websocket.Conn, msg agentprotocol.Message, payload agentconfig.Dispatch) {
	if payload.RunID == "" {
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", "A run ID is required for supervised execution")
		return
	}
	if strings.TrimSpace(payload.ProjectID) == "" || msg.TaskID != "" || payload.TaskID != "" || payload.MacroKey != "" {
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", "An orchestrator dispatch names its project and neither a task nor a macro")
		return
	}
	config, err := d.fetchConfig(ctx, payload.ProjectID, "")
	if err != nil {
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", err.Error())
		return
	}
	payload.Mode = models.SkillModeAutonomous
	payload.TaskKey = ""
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

	config, root, err := d.prepareOrchestratorSession(ctx, config)
	if err != nil {
		launchFailure = err
		return
	}
	prompt := strings.TrimSpace(payload.Prompt)
	if profileID := strings.TrimSpace(payload.ProfileID); profileID != "" {
		policy, err := d.orchestrationProfilePolicy(config.ProjectID, root, profileID)
		if err != nil {
			launchFailure = err
			return
		}
		prompt += "\nOrchestration policy (mandatory):\n" + policy
	}
	prompt += d.sandboxNotice(config)
	prompt += fmt.Sprintf("\nRemote execution runId: %s. Finish it using finish_run (projectId %s, runId %s, no taskKey) when the entire skill ends.", payload.RunID, config.ProjectID, payload.RunID)
	choice, err := d.prepareSkill(config, payload.SkillID, payload.Action, strings.TrimSpace(prompt), payload.RunID, run.exited, root, "")
	if err != nil {
		launchFailure = err
		return
	}
	claudeSettings, err := d.launchClaudeSettings(config)
	if err != nil {
		launchFailure = err
		return
	}
	fullLine, err := dispatchCommand(config, config.ProjectID, payload.SkillID, payload.Action, strings.TrimSpace(prompt), payload.Command, payload.Mode, payload.Model,
		agentCommandContext{Directory: root, Tracker: config.IssueTracker, Repo: config.GithubRepo, ClaudeSettings: claudeSettings, Skill: choice})
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
		"SECTILE_MACRO_KEY": "", "SECTILE_MACRO_PROJECT_ID": "",
		"SECTILE_PROJECT_ID":   config.ProjectID,
		"SECTILE_RUN_ID":       payload.RunID,
		"SECTILE_REMOTE_MODE":  "true",
		"SECTILE_AGENT_URL":    d.link.serverURL,
		"SECTILE_SERVER_URL":   d.link.serverURL,
		"SECTILE_DEVICE_ID":    d.link.deviceID,
		"SECTILE_AGENT_TOKEN":  d.link.token,
		"SECTILE_LOOPBACK_URL": d.loopback.url,
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
		run.desktop = desktopRun{CreatedAt: run.desktop.CreatedAt, Prompt: run.desktop.Prompt, ID: payload.RunID, ProjectID: config.ProjectID,
			Scope: agentprotocol.ScopeOrchestrator, ProfileID: payload.ProfileID, ProfileName: payload.ProfileName,
			Skill: payload.SkillID, SessionID: sessionID, Directory: root, Status: "running", Provider: runProvider, Model: runModel}
	})
	if err := d.runInPty(sessionID, root, envVars, fullLine); err != nil {
		launchFailure = err
		return
	}
	launched = true
	d.recordCustomSkillUse(config, choice, payload.RunID)
	log.Printf("[Agent] Orchestrator session launched for project %s in %s (profile %q)", config.ProjectID, root, payload.ProfileID)
}

// prepareOrchestratorSession resolves the project's configuration and its
// checkout, where the skills and AGENTS.md are, under prepareMu.
func (d *agentDaemon) prepareOrchestratorSession(ctx context.Context, config agentconfig.Config) (agentconfig.Config, string, error) {
	d.prepareMu.Lock()
	defer d.prepareMu.Unlock()
	root, overrides, err := d.localProjectRoot(ctx, config)
	if err != nil {
		return config, "", err
	}
	config = agentconfig.Resolve(config, overrides)
	if err := config.Validate(); err != nil {
		return config, "", err
	}
	return config, root, nil
}

// orchestrationProfilePolicy reads the named orchestration agent's file. A
// profile removed or made unreadable since orchestration started fails the
// session with the reason, rather than running without its policy.
func (d *agentDaemon) orchestrationProfilePolicy(projectID, root, profileID string) (string, error) {
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		return "", err
	}
	for _, agent := range settings.Project(projectID).OrchestrationAgentDefinitions() {
		if agent.ID != profileID {
			continue
		}
		_, snapshot, err := readOrchestrationAgentGuidelines(root, agent)
		if err != nil {
			return "", fmt.Errorf("orchestration agent %s: %w", agent.Name, err)
		}
		return snapshot, nil
	}
	return "", fmt.Errorf("orchestration agent %s is no longer configured on this workstation: stop orchestration or start another agent", profileID)
}

// orchestratorRunProjects remembers the project of every orchestrator session
// from admission until its end is reported, for the same reason as
// macroRunIdentities: clearing the desktop list may drop the run first.
var orchestratorRunProjects sync.Map

func (d *agentDaemon) rememberOrchestratorRun(runID, projectID string) {
	orchestratorRunProjects.Store(runID, projectID)
}

func (d *agentDaemon) forgetOrchestratorRun(runID string) { orchestratorRunProjects.Delete(runID) }

// orchestratorProjectOfRun returns the project of an orchestrator session, ""
// for any other run.
func (d *agentDaemon) orchestratorProjectOfRun(runID string) string {
	if value, ok := orchestratorRunProjects.Load(runID); ok {
		return value.(string)
	}
	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()
	if run := d.queue.runs[runID]; run != nil && run.desktop.Scope == agentprotocol.ScopeOrchestrator {
		return run.desktop.ProjectID
	}
	return ""
}
