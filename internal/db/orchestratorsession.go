package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
)

// The orchestrator is one agent session per project (ADR 0064). It reads the
// whole backlog, decides which tickets to take, and launches their chains
// through launch_task_chain, which applies every hard guard. The session is a
// project activity with no task, marked run_scope 'orchestrator', and a partial
// unique index allows one active at a time per project across instances.
//
// Starting orchestration arms the project for one user, one workstation and one
// profile. While armed, the end of an orchestrated chain wakes a new session.
// A chain that ends while a session runs sets wake_pending instead, and the end
// of that session wakes the next one: a freed slot is never lost, and at most
// one session follows the running one. A session that launches nothing frees
// no slot, so an exhausted backlog stays quiet.

// OrchestratorSkillID is the skill an orchestrator session runs.
const OrchestratorSkillID = "orchestrate_backlog"

// orchestratorRunScope marks the orchestrator session among task activities.
const orchestratorRunScope = "orchestrator"

// orchestratorActiveIndex is the partial unique index of migration 57.
const orchestratorActiveIndex = "idx_task_activities_orchestrator_active"

func init() {
	uniqueIndexColumns[orchestratorActiveIndex] = "task_activities.project_id"
}

var (
	// ErrOrchestratorSessionActive refuses a second session on a project.
	ErrOrchestratorSessionActive = errors.New("an orchestrator session is already active on this project")
	// ErrOrchestratorArmedElsewhere refuses to start a profile while another
	// profile, or the same one on another workstation, is armed.
	ErrOrchestratorArmedElsewhere = errors.New("orchestration is already started")
	// ErrOrchestratorNoWorkstation refuses to start orchestration when no
	// workstation of the caller can run the session.
	ErrOrchestratorNoWorkstation = errors.New("no workstation can run the orchestrator session")
	// ErrNotOrchestratorSession refuses an orchestrator tool to any caller but
	// the project's running orchestrator session.
	ErrNotOrchestratorSession = errors.New("only the project's running orchestrator session may call this tool")
)

// orchestratorBlockerLabels are the labels that keep a ticket out of reach of
// the orchestrator, whatever its policy says.
var orchestratorBlockerLabels = []string{"blocked", "hold", "on-hold", "needs-info", "manual"}

// Decisions the orchestrator records.
const (
	OrchestratorDecisionLaunched = "launched"
	OrchestratorDecisionDeferred = "deferred"
)

// OrchestrationStart is who starts orchestration, from where, with what policy.
// An empty ProfileID runs the project's shared policy (orchestrator_prompt).
type OrchestrationStart struct {
	UserID      string
	DeviceID    string
	ProfileID   string
	ProfileName string
}

// StartOrchestration arms the project and starts a session. Starting the armed
// profile again on the same workstation starts a session when none is active,
// and otherwise returns the active one with ErrOrchestratorSessionActive.
func (d *DB) StartOrchestration(projectID string, start OrchestrationStart) (*models.TaskActivity, error) {
	project, err := d.GetProjectByID(projectID)
	if err != nil || project == nil {
		return nil, fmt.Errorf("%w: %s", ErrOrchestratorProjectNotFound, projectID)
	}
	if !project.OrchestratorEnabled {
		return nil, fmt.Errorf("%w: %q", ErrOrchestratorDisabled, project.Name)
	}
	start.ProfileID, start.ProfileName = strings.TrimSpace(start.ProfileID), strings.TrimSpace(start.ProfileName)
	// The workstation is chosen once, as a queued launch chooses it, and
	// stays the project's until orchestration stops: no failover (ADR 0064).
	if location, ok := d.AgentOwner(start.UserID, project.ID); ok && start.DeviceID == "" {
		start.DeviceID = location.DeviceID
	}
	if d.agentSelection != nil {
		if start.DeviceID, err = d.agentSelection(start.UserID, project.ID, start.DeviceID); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrOrchestratorNoWorkstation, err)
		}
	}
	// The first start wins across instances: the insert is a no-op when the
	// project is already armed, and the row read back says by whom. Starting
	// the armed profile again from its workstation is allowed.
	d.mu.Lock()
	_, err = d.conn.Exec(`INSERT INTO orchestrator_arming (project_id, user_id, device_id, profile_id, profile_name, wake_pending, armed_at) VALUES (?, ?, ?, ?, ?, 0, ?) ON CONFLICT (project_id) DO NOTHING`,
		project.ID, start.UserID, start.DeviceID, start.ProfileID, start.ProfileName, time.Now().UTC())
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}
	armed, err := d.OrchestratorArming(project.ID)
	if err != nil {
		return nil, err
	}
	if armed == nil {
		return nil, fmt.Errorf("%w: orchestration was stopped while starting", ErrOrchestratorArmedElsewhere)
	}
	if armed.ProfileID != start.ProfileID || armed.DeviceID != start.DeviceID {
		return nil, fmt.Errorf("%w with profile %q on another workstation or profile: stop it first", ErrOrchestratorArmedElsewhere, nonEmpty(armed.ProfileName, armed.ProfileID, "default")[0])
	}
	// The session runs as whoever armed the project, whichever caller restarted it.
	start.UserID = armed.UserID
	run, err := d.startOrchestratorSession(project, start)
	if errors.Is(err, ErrOrchestratorSessionActive) {
		active, _ := d.ActiveOrchestratorSession(project.ID)
		return active, err
	}
	return run, err
}

// StopOrchestration disarms the project. Running chains and a running session
// finish on their own; nothing they end wakes another session. A session still
// waiting for its workstation never runs.
func (d *DB) StopOrchestration(projectID string) error {
	project, err := d.GetProjectByID(projectID)
	if err != nil || project == nil {
		return fmt.Errorf("%w: %s", ErrOrchestratorProjectNotFound, projectID)
	}
	d.mu.Lock()
	_, err = d.conn.Exec(`DELETE FROM orchestrator_arming WHERE project_id = ?`, project.ID)
	d.mu.Unlock()
	if err != nil {
		return err
	}
	if session, err := d.ActiveOrchestratorSession(project.ID); err == nil && session != nil {
		if state, _ := d.RunDispatchState(session.ID); state == orchestratorWaitingDispatch {
			_, _ = d.FinishOrchestratorSession(Actor{}, true, project.ID, session.ID, "canceled", "Orchestration was stopped before the workstation connected")
		}
	}
	return nil
}

// OrchestratorArming returns who armed the project, or nil when it is not armed.
func (d *DB) OrchestratorArming(projectID string) (*models.OrchestratorArming, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return orchestratorArmingOn(d.conn, projectID)
}

func orchestratorArmingOn(q rowQuerier, projectID string) (*models.OrchestratorArming, error) {
	armed := &models.OrchestratorArming{}
	var pending int
	err := q.QueryRow(`SELECT project_id, user_id, device_id, profile_id, profile_name, wake_pending, armed_at FROM orchestrator_arming WHERE project_id = ?`, projectID).
		Scan(&armed.ProjectID, &armed.UserID, &armed.DeviceID, &armed.ProfileID, &armed.ProfileName, &pending, &armed.ArmedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	armed.WakePending = pending != 0
	return armed, nil
}

// ActiveOrchestratorSession returns the project's queued or running session.
func (d *DB) ActiveOrchestratorSession(projectID string) (*models.TaskActivity, error) {
	d.mu.RLock()
	var id string
	err := d.conn.QueryRow(`SELECT id FROM task_activities WHERE project_id = ? AND run_scope = ? AND status IN ('queued', 'pending', 'running')`,
		projectID, orchestratorRunScope).Scan(&id)
	d.mu.RUnlock()
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return d.orchestratorSession(id)
}

// orchestratorSession reads an activity and marks it as an orchestrator session
// when it is one.
func (d *DB) orchestratorSession(runID string) (*models.TaskActivity, error) {
	activity, err := d.GetActivityByID(runID)
	if err != nil || activity == nil {
		return activity, err
	}
	d.mu.RLock()
	var scope string
	_ = d.conn.QueryRow(`SELECT run_scope FROM task_activities WHERE id = ?`, runID).Scan(&scope)
	d.mu.RUnlock()
	activity.RunScope = scope
	return activity, nil
}

// IsOrchestratorSession says whether a run is an orchestrator session.
func (d *DB) IsOrchestratorSession(runID string) bool {
	if strings.TrimSpace(runID) == "" {
		return false
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	var scope string
	_ = d.conn.QueryRow(`SELECT run_scope FROM task_activities WHERE id = ?`, runID).Scan(&scope)
	return scope == orchestratorRunScope
}

// orchestratorSessionPrompt is what the server tells the session. The skill
// carries the method; the policy is the profile's file, which the workstation
// adds, or the project's shared policy when no profile was named.
func orchestratorSessionPrompt(project *models.Project, start OrchestrationStart) string {
	prompt := fmt.Sprintf("Orchestrate the backlog of project %s (projectId %s).", project.Name, project.ID)
	if start.ProfileID == "" {
		if policy := strings.TrimSpace(project.OrchestratorPrompt); policy != "" {
			prompt += "\nOrchestration policy (mandatory):\n" + policy
		}
	}
	return prompt
}

// startOrchestratorSession records a session and hands it to the armed
// workstation in the background: a wake-up runs inside a hand-back, which must
// not wait on a workstation.
func (d *DB) startOrchestratorSession(project *models.Project, start OrchestrationStart) (*models.TaskActivity, error) {
	now := time.Now()
	prompt := orchestratorSessionPrompt(project, start)
	activity := models.TaskActivity{ID: uuid.NewString(), ProjectID: project.ID, RunProjectID: project.ID,
		SkillID: "remote_run", SkillName: OrchestratorSkillID, Action: RunActionAgent, DispatchState: "admitted",
		Status: "running", Summary: "Orchestrator session dispatched to the workstation",
		CreatedAt: now, StartedAt: &now, Steps: []string{}, Prompt: prompt, UserID: start.UserID, DeviceID: start.DeviceID,
		RunScope: orchestratorRunScope}
	d.mu.Lock()
	err := d.conn.WithTx(func(tx *sqlTx) error {
		if err := insertTaskActivity(tx, d.instanceID, activity); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE task_activities SET run_scope = ?, run_mode = ? WHERE id = ?`, orchestratorRunScope, models.SkillModeAutonomous, activity.ID)
		return err
	})
	d.mu.Unlock()
	if isUniqueViolation(err, orchestratorActiveIndex) {
		return nil, ErrOrchestratorSessionActive
	}
	if err != nil {
		return nil, err
	}
	d.jobs.begin()
	go func() {
		defer d.jobs.end()
		d.dispatchOrchestratorSession(project.ID, activity.ID, prompt, start)
	}()
	return &activity, nil
}

// orchestratorWaitingDispatch is the dispatch state of a session whose armed
// workstation is offline. No restart reclaims it, as it does an admitted run:
// the session waits, queued, until that workstation connects again.
const orchestratorWaitingDispatch = "waiting"

// dispatchOrchestratorSession sends the session to the armed workstation. An
// offline workstation leaves it queued until it connects (ADR 0064: no
// failover); a workstation that refuses it fails the session, and the project
// stays armed.
func (d *DB) dispatchOrchestratorSession(projectID, runID, prompt string, start OrchestrationStart) {
	if d.agentSelection != nil {
		if _, err := d.agentSelection(start.UserID, projectID, start.DeviceID); err != nil {
			d.mu.Lock()
			_, _ = d.conn.Exec(`UPDATE task_activities SET dispatch_state = ?, status = 'queued', summary = ?
				WHERE id = ? AND dispatch_state = 'admitted' AND status IN ('queued', 'pending', 'running')`,
				orchestratorWaitingDispatch, "Waiting for the workstation to connect", runID)
			d.mu.Unlock()
			log.Printf("[orchestrator] session %s of project %s waits for its workstation: %v", runID, projectID, err)
			return
		}
	}
	if err := d.ClaimRunDispatch(runID); err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	err := d.callAgentContext(ctx, agentprotocol.Operation{UserID: start.UserID, DeviceID: start.DeviceID, ProjectID: projectID,
		Action: "execute_skill", SkillID: OrchestratorSkillID, Prompt: prompt, RunID: runID, Mode: models.SkillModeAutonomous,
		Scope: agentprotocol.ScopeOrchestrator, ProfileID: start.ProfileID, ProfileName: start.ProfileName}, nil)
	_ = d.CompleteRunDispatch(runID)
	if err != nil && !errors.Is(err, agentprotocol.ErrLaunchUnconfirmed) {
		log.Printf("[orchestrator] session %s of project %s could not reach its workstation: %v", runID, projectID, err)
		_, _ = d.FinishOrchestratorSession(Actor{}, true, projectID, runID, "failed", "The orchestrator session could not reach its workstation: "+err.Error())
	}
}

// ResumeOrchestratorSessions sends the sessions waiting for a workstation that
// has just connected. Each is taken back atomically, so of several instances
// seeing the connection one sends it.
func (d *DB) ResumeOrchestratorSessions(userID, deviceID string) {
	d.mu.RLock()
	rows, err := d.conn.Query(`SELECT id, project_id, prompt FROM task_activities
		WHERE run_scope = ? AND dispatch_state = ? AND status = 'queued' AND user_id = ? AND device_id = ?`,
		orchestratorRunScope, orchestratorWaitingDispatch, userID, deviceID)
	if err != nil {
		d.mu.RUnlock()
		return
	}
	type waiting struct{ runID, projectID, prompt string }
	var sessions []waiting
	for rows.Next() {
		var item waiting
		if rows.Scan(&item.runID, &item.projectID, &item.prompt) == nil {
			sessions = append(sessions, item)
		}
	}
	rows.Close()
	d.mu.RUnlock()
	for _, session := range sessions {
		armed, err := d.OrchestratorArming(session.projectID)
		if err != nil || armed == nil {
			// Stopped while it waited: it never runs.
			_, _ = d.FinishOrchestratorSession(Actor{}, true, session.projectID, session.runID, "canceled", "Orchestration was stopped before the workstation connected")
			continue
		}
		d.mu.Lock()
		result, err := d.conn.Exec(`UPDATE task_activities SET dispatch_state = 'admitted', status = 'running', summary = ?
			WHERE id = ? AND dispatch_state = ? AND status = 'queued'`, "Orchestrator session dispatched to the workstation", session.runID, orchestratorWaitingDispatch)
		d.mu.Unlock()
		if err != nil {
			continue
		}
		if count, _ := result.RowsAffected(); count != 1 {
			continue
		}
		start := OrchestrationStart{UserID: armed.UserID, DeviceID: armed.DeviceID, ProfileID: armed.ProfileID, ProfileName: armed.ProfileName}
		d.jobs.begin()
		go func(projectID, runID, prompt string) {
			defer d.jobs.end()
			d.dispatchOrchestratorSession(projectID, runID, prompt, start)
		}(session.projectID, session.runID, session.prompt)
	}
}

// FinishOrchestratorSession closes a session and, when a slot freed while it
// ran, wakes the next one.
func (d *DB) FinishOrchestratorSession(caller Actor, admin bool, projectID, runID, status, note string) (*models.TaskActivity, error) {
	if status != "completed" && status != "failed" && status != "canceled" {
		return nil, fmt.Errorf("status must be completed, failed or canceled")
	}
	existing, err := d.orchestratorSession(runID)
	if err != nil {
		return nil, err
	}
	if existing == nil || existing.RunScope != orchestratorRunScope || (strings.TrimSpace(projectID) != "" && existing.ProjectID != projectID) {
		return nil, fmt.Errorf("orchestrator session not found on this project")
	}
	if !admin && existing.UserID != "" && existing.UserID != caller.ID {
		return nil, ErrRunNotYours
	}
	d.mu.Lock()
	result, err := d.conn.Exec(`UPDATE task_activities SET status=?, summary=?, completed_at=?, waiting_since=NULL, waiting_session=''
		WHERE id=? AND run_scope=? AND status IN ('queued', 'pending', 'running')`, status, note, time.Now(), runID, orchestratorRunScope)
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}
	activity, err := d.orchestratorSession(runID)
	if err != nil {
		return nil, err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		if activity != nil && activity.Status == status {
			return activity, nil
		}
		return nil, fmt.Errorf("orchestrator session not found or already finished with another status")
	}
	d.notifyPostBackListeners(nil, activity, nil)
	d.wakeOrchestrator(existing.ProjectID, true)
	return activity, nil
}

// wakeOrchestrator starts a session for an armed project when a slot freed.
// afterSession is set when the trigger is the end of a session, which only
// wakes the next one if a slot freed meanwhile.
func (d *DB) wakeOrchestrator(projectID string, afterSession bool) {
	project, err := d.GetProjectByID(projectID)
	if err != nil || project == nil {
		return
	}
	armed, err := d.OrchestratorArming(project.ID)
	if err != nil || armed == nil {
		return
	}
	if !project.OrchestratorEnabled {
		_ = d.StopOrchestration(project.ID)
		return
	}
	if afterSession && !armed.WakePending {
		return
	}
	start := OrchestrationStart{UserID: armed.UserID, DeviceID: armed.DeviceID, ProfileID: armed.ProfileID, ProfileName: armed.ProfileName}
	d.mu.Lock()
	_, _ = d.conn.Exec(`UPDATE orchestrator_arming SET wake_pending = 0 WHERE project_id = ?`, project.ID)
	d.mu.Unlock()
	if _, err := d.startOrchestratorSession(project, start); errors.Is(err, ErrOrchestratorSessionActive) {
		// The running session read the state before this slot freed: the
		// next one, after it, will see it.
		d.mu.Lock()
		_, _ = d.conn.Exec(`UPDATE orchestrator_arming SET wake_pending = 1 WHERE project_id = ?`, project.ID)
		d.mu.Unlock()
	} else if err != nil {
		log.Printf("[orchestrator] cannot wake the orchestrator of project %s: %v", project.ID, err)
	}
}

// afterOrchestratedRun is called when an orchestrated run has ended and its
// hand-back is done: when the chain did not continue, its slot is free.
func (d *DB) afterOrchestratedRun(taskID, runID string) {
	if active, batch, err := d.ActiveBusyCause(taskID); err != nil || active != nil || batch != nil {
		return
	}
	if projectID := d.runProjectOf(runID); projectID != "" {
		d.wakeOrchestrator(projectID, false)
	}
}

// orchestratorSessionOfCall returns the running orchestrator session a tool
// call comes from, refusing any other caller.
func (d *DB) orchestratorSessionOfCall(runID string) (*models.TaskActivity, error) {
	session, err := d.orchestratorSession(strings.TrimSpace(runID))
	if err != nil {
		return nil, err
	}
	if session == nil || session.RunScope != orchestratorRunScope || session.Status != "running" {
		return nil, ErrNotOrchestratorSession
	}
	return session, nil
}

// orchestratorBlocker names the blocker label a ticket carries, "" for none.
func orchestratorBlocker(task *models.Task) string {
	for _, label := range task.Labels {
		norm := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(label), "#")))
		for _, blocker := range orchestratorBlockerLabels {
			if norm == blocker {
				return label
			}
		}
	}
	return ""
}

// orchestratedChainPrompt opens with the orchestrator prefix, which ADR 0063
// counts slots by and enqueueChainStep repeats on every step, and carries the
// orchestrator's reason as context, not as a gate.
func orchestratedChainPrompt(profileName, profileID, rationale string) string {
	who := "the project's orchestrator"
	if profileID != "" {
		who = fmt.Sprintf("orchestration agent %q (%s)", nonEmpty(profileName, profileID)[0], profileID)
	}
	prompt := orchestratorPromptPrefix + " launched by " + who + "."
	if rationale = strings.TrimSpace(rationale); rationale != "" {
		prompt += "\nWhy this ticket now: " + rationale
	}
	return prompt + "\nThe chain stops at implemented; review and merge require a human."
}

// LaunchTaskChain is launch_task_chain: the orchestrator session asks for a
// ticket's chain, and every hard guard is applied here, not left to the agent.
func (d *DB) LaunchTaskChain(ctx context.Context, sessionRunID, taskRef, rationale string) (*models.TaskActivity, error) {
	session, err := d.orchestratorSessionOfCall(sessionRunID)
	if err != nil {
		return nil, err
	}
	project, err := d.GetProjectByID(session.ProjectID)
	if err != nil || project == nil {
		return nil, fmt.Errorf("%w: %s", ErrOrchestratorProjectNotFound, session.ProjectID)
	}
	if !project.OrchestratorEnabled {
		return nil, fmt.Errorf("%w: %q", ErrOrchestratorDisabled, project.Name)
	}
	armed, err := d.OrchestratorArming(project.ID)
	if err != nil {
		return nil, err
	}
	if armed == nil {
		return nil, fmt.Errorf("orchestration was stopped on project %q", project.Name)
	}
	task, err := d.GetTaskByIDIn(project.ID, strings.TrimSpace(taskRef))
	if err != nil || task == nil {
		return nil, fmt.Errorf("task %s not found in project %q", taskRef, project.Name)
	}
	if label := orchestratorBlocker(task); label != "" {
		return nil, fmt.Errorf("task %s carries the blocker label %q", task.Key, label)
	}
	stage := d.StageOfTask(task)
	if stage == "finished" || stageAtOrPast(stage, models.FullChainStopImplemented) {
		return nil, fmt.Errorf("task %s is already at stage %q: the orchestrator stops at implemented", task.Key, stage)
	}
	step, ok := NextStep(stage)
	if !ok {
		return nil, fmt.Errorf("no workflow step follows stage %q of task %s", stage, task.Key)
	}
	_, act, err := d.enqueueSkillWithOptions(task.ID, project.ID, step.SkillID, orchestratedChainPrompt(armed.ProfileName, armed.ProfileID, rationale), true, models.SkillModeAutonomous, "", queuedSkillOptions{
		UserID: armed.UserID, DeviceID: armed.DeviceID, ChainStop: models.FullChainStopImplemented, Orchestrated: true,
	})
	if errors.Is(err, errOrchestratorCapacity) {
		return nil, fmt.Errorf("no orchestrator slot is free on project %q: %w", project.Name, err)
	}
	if err != nil {
		return nil, err
	}
	_ = d.recordOrchestratorDecision(project.ID, task, session.ID, OrchestratorDecisionLaunched, rationale)
	return act, nil
}

// DeferTask is defer_task: the orchestrator session records why it leaves a
// ticket for now.
func (d *DB) DeferTask(sessionRunID, taskRef, reason string) error {
	session, err := d.orchestratorSessionOfCall(sessionRunID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("a reason is required")
	}
	task, err := d.GetTaskByIDIn(session.ProjectID, strings.TrimSpace(taskRef))
	if err != nil || task == nil {
		return fmt.Errorf("task %s not found in this project", taskRef)
	}
	return d.recordOrchestratorDecision(session.ProjectID, task, session.ID, OrchestratorDecisionDeferred, reason)
}

// orchestratorDecisionRetention bounds the decisions kept per project.
const orchestratorDecisionRetention = 30 * 24 * time.Hour

func (d *DB) recordOrchestratorDecision(projectID string, task *models.Task, runID, decision, reason string) error {
	now := time.Now().UTC()
	d.mu.Lock()
	defer d.mu.Unlock()
	_, _ = d.conn.Exec(`DELETE FROM orchestrator_decisions WHERE project_id = ? AND created_at < ?`, projectID, now.Add(-orchestratorDecisionRetention))
	_, err := d.conn.Exec(`INSERT INTO orchestrator_decisions (id, project_id, task_id, run_id, decision, reason, task_updated_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), projectID, task.ID, runID, decision, strings.TrimSpace(reason), task.UpdatedAt.UTC(), now)
	return err
}

// OrchestratorDecisions returns the latest decision on each ticket of the
// project, most recent first, flagging those whose ticket has not changed.
func (d *DB) OrchestratorDecisions(projectID string) ([]models.OrchestratorDecision, error) {
	d.mu.RLock()
	rows, err := d.conn.Query(`SELECT o.task_id, COALESCE(t.key, ''), COALESCE(t.title, ''), o.run_id, o.decision, o.reason, o.task_updated_at, t.updated_at, o.created_at
		FROM orchestrator_decisions o LEFT JOIN tasks t ON t.id = o.task_id
		WHERE o.project_id = ? ORDER BY o.created_at DESC`, projectID)
	if err != nil {
		d.mu.RUnlock()
		return nil, err
	}
	seen := map[string]bool{}
	out := []models.OrchestratorDecision{}
	for rows.Next() {
		var item models.OrchestratorDecision
		var then, now sql.NullTime
		if err := rows.Scan(&item.TaskID, &item.TaskKey, &item.Title, &item.RunID, &item.Decision, &item.Reason, &then, &now, &item.DecidedAt); err != nil {
			rows.Close()
			d.mu.RUnlock()
			return nil, err
		}
		if seen[item.TaskID] {
			continue
		}
		seen[item.TaskID] = true
		item.UnchangedSinceDecision = then.Valid && now.Valid && !now.Time.After(then.Time)
		out = append(out, item)
	}
	err = rows.Err()
	rows.Close()
	d.mu.RUnlock()
	return out, err
}

// OrchestrationState is get_orchestration_state, and what the desktop shows.
func (d *DB) OrchestrationState(projectID string) (*models.OrchestrationState, error) {
	project, err := d.GetProjectByID(projectID)
	if err != nil || project == nil {
		return nil, fmt.Errorf("%w: %s", ErrOrchestratorProjectNotFound, projectID)
	}
	maxConcurrent := models.NormalizeOrchestratorMaxConcurrent(project.OrchestratorMaxConcurrent)
	active, err := d.ActiveRunsCountForProject(project.ID)
	if err != nil {
		return nil, err
	}
	state := &models.OrchestrationState{ProjectID: project.ID, Enabled: project.OrchestratorEnabled, MaxConcurrent: maxConcurrent,
		ActiveRuns: active, AvailableSlots: max(0, maxConcurrent-active), BlockerLabels: orchestratorBlockerLabels}
	if state.Armed, err = d.OrchestratorArming(project.ID); err != nil {
		return nil, err
	}
	if state.Session, err = d.ActiveOrchestratorSession(project.ID); err != nil {
		return nil, err
	}
	if state.Decisions, err = d.OrchestratorDecisions(project.ID); err != nil {
		return nil, err
	}
	return state, nil
}
