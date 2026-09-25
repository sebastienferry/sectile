package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"tasks/internal/models"
)

// A remote run's action names its owner, which is what decides who may close
// it. The distinction outlives the server process, so it is the only thing a
// restart can rely on to tell an abandoned run from one still being watched.
const (
	// RunActionClient marks a run a connected client created. Its owner is that
	// client's MCP session, which lives and dies inside the server process.
	RunActionClient = "Remote skill execution"
	// RunActionAgent marks a run an agent dispatched. Its owner is the agent's
	// supervisor, which outlives the server and reports the real process exit.
	RunActionAgent = "Agent-owned remote execution"
)

// RunLaunch is what the launcher knows about a run that the run itself cannot
// report: the mode it was resolved to, the stage the task sat on when it
// started, and, for a step of a full chain, the stage that chain stops at. It is
// recorded at launch because that is the only moment those values are true, and
// read when the run ends to tell a run that handed the workflow back from one
// that merely exited.
type RunLaunch struct {
	// Mode is the resolved execution mode, autonomous or interactive. Empty when
	// the launcher did not resolve one, as for a run a standalone CLI declares.
	Mode string
	// Provider and Model are the engine the launcher resolved for this run. The
	// agent corrects them later through SetRemoteRunEngine, since it alone sees
	// the workstation override.
	Provider string
	Model    string
	// Stage is the workflow stage of the task at launch.
	Stage string
	// ChainStop is set only on a step of a full chain run, to the stage that
	// chain stops at. Empty means this run chains nothing.
	ChainStop string
	// UserID is the owner of the run: the person who launched it, or the user
	// the launching agent's key is bound to. Only the owner or an admin may
	// stop it.
	UserID string
	// Force marks a run started with "Launch anyway", next to another active
	// run on the task. It is recorded as concurrent, out of the one-run rule.
	Force bool
}

// StartRemoteRun creates an independent execution record with no owner. It
// never locks stages. Callers that know who is reporting use StartRemoteRunBy.
func (d *DB) StartRemoteRun(taskKey, skill, runID string) (*models.TaskActivity, error) {
	return d.StartRemoteRunBy("", taskKey, skill, runID)
}

// StartRemoteRunBy is StartRemoteRun for a caller whose identity is known: the
// user the MCP client's key resolves to becomes the owner of the run.
func (d *DB) StartRemoteRunBy(userID, taskKey, skill, runID string) (*models.TaskActivity, error) {
	return d.startRemoteRun(taskKey, skill, runID, false, RunLaunch{UserID: userID})
}
func (d *DB) StartAgentRemoteRun(taskKey, skill string) (*models.TaskActivity, error) {
	return d.startRemoteRun(taskKey, skill, "", true, RunLaunch{})
}

// StartAgentRun is StartAgentRemoteRun for a launcher that knows how the run was
// resolved. The stage is read here rather than taken from the caller: the run
// record must carry the stage the task is really on at the instant it starts.
func (d *DB) StartAgentRun(taskKey, skill string, launch RunLaunch) (*models.TaskActivity, error) {
	return d.startRemoteRun(taskKey, skill, "", true, launch)
}

func (d *DB) startRemoteRun(taskKey, skill, runID string, agentOwned bool, launch RunLaunch) (*models.TaskActivity, error) {
	task, err := d.GetTaskByID(taskKey)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, fmt.Errorf("task not found")
	}
	if runID != "" {
		activity, err := d.GetActivityByID(runID)
		if err != nil {
			return nil, err
		}
		if activity == nil || activity.TaskID != task.ID || activity.SkillID != "remote_run" || activity.Status != "running" {
			return nil, fmt.Errorf("remote run does not match an active execution on this task")
		}
		return activity, nil
	}
	if strings.TrimSpace(skill) == "" {
		return nil, fmt.Errorf("skill is required")
	}
	now := time.Now()
	// No project_id: the activity is attached to the task, and the task is what
	// carries the project. Writing both would be two sources of truth for one
	// attachment, and the schema refuses it outright.
	activity := &models.TaskActivity{ID: uuid.NewString(), TaskID: task.ID,
		SkillID: "remote_run", SkillName: skill, Action: RunActionClient,
		Status: "running", Summary: "Execution reported by a local agent or native client",
		CreatedAt: now, StartedAt: &now, Steps: []string{}, UserID: strings.TrimSpace(launch.UserID),
		// What the launcher resolved. The agent corrects it through
		// SetRemoteRunEngine once it has built the real command line.
		Provider: strings.TrimSpace(launch.Provider), Model: strings.TrimSpace(launch.Model)}
	if agentOwned {
		activity.Action = RunActionAgent
		activity.Concurrent = launch.Force
	} else {
		// A client declaring its own run is never refused (#308): the session
		// is already running, and refusing would only lose track of it.
		activity.Concurrent = true
	}
	if err := d.AddTaskActivity(*activity); err != nil {
		return nil, d.taskBusy(task.ID, err)
	}
	launch.Stage = d.StageOfTask(task)
	if launch.Mode != "" || launch.Stage != "" || launch.ChainStop != "" {
		d.mu.Lock()
		_, _ = d.conn.Exec("UPDATE task_activities SET run_mode=?, launch_stage=?, chain_stop_stage=? WHERE id=?",
			models.NormalizeSkillMode(launch.Mode), launch.Stage, launch.ChainStop, activity.ID)
		d.mu.Unlock()
	}
	d.notifyPostBackListeners(task, activity, nil)
	return activity, nil
}

// NoteRemoteRun appends one sentence to a remote run that is still running. It
// is how an observation about a run (a silence, so far) reaches the board
// without pretending to be an outcome: a finished run is never annotated after
// the fact, and no other field is touched.
func (d *DB) NoteRemoteRun(runID, note string) error {
	runID, note = strings.TrimSpace(runID), strings.TrimSpace(note)
	if runID == "" || note == "" {
		return fmt.Errorf("runId and note are required")
	}
	return d.appendToRunSummary(runID, note, " AND skill_id='remote_run' AND status='running'")
}

// ErrRunNotYours refuses closing an execution that belongs to somebody else.
// Reporting a run's outcome is the run's own business: a third party closing it
// looks exactly like the run ending, hands the workflow back, and leaves the
// real process running on its owner's machine. The name differs from the
// dispatcher's ErrRunNotOwned, which says an agent does not have a run at all.
var ErrRunNotYours = errors.New("this execution belongs to another user")

// FinishRemoteRun completes only the specified execution, preserving concurrent runs.
// It checks no identity; callers that have one use FinishRemoteRunAs.
func (d *DB) FinishRemoteRun(taskKey, runID, status, note string) (*models.TaskActivity, error) {
	// A macro skill run has no task key to name. The session that adopted one
	// closes it through here on a disconnection, without an identity, as the
	// server closes a task run it adopted.
	if strings.TrimSpace(taskKey) == "" {
		if key := d.macroKeyOfActivity(runID); key != "" {
			if activity, err := d.GetActivityByID(runID); err == nil && activity != nil {
				return d.FinishMacroRunAs(Actor{}, true, activity.ProjectID, key, runID, status, note)
			}
		}
	}
	return d.finishRemoteRun(taskKey, runID, status, note, nil)
}

// FinishRemoteRunAs is FinishRemoteRun for a caller whose identity is known.
// The owner closes their own run; an admin closes anyone's; a run with no
// recorded owner predates ownership and stays closable by anyone, as before.
func (d *DB) FinishRemoteRunAs(caller Actor, admin bool, taskKey, runID, status, note string) (*models.TaskActivity, error) {
	check := func(owner string) error {
		if admin || owner == "" || owner == caller.ID {
			return nil
		}
		return ErrRunNotYours
	}
	return d.finishRemoteRun(taskKey, runID, status, note, check)
}

func (d *DB) finishRemoteRun(taskKey, runID, status, note string, authorize func(ownerID string) error) (*models.TaskActivity, error) {
	if status != "completed" && status != "failed" && status != "canceled" {
		return nil, fmt.Errorf("status must be completed, failed or canceled")
	}
	if authorize != nil {
		existing, err := d.GetActivityByID(runID)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			if err := authorize(existing.UserID); err != nil {
				return nil, err
			}
		}
	}
	task, err := d.GetTaskByID(taskKey)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, fmt.Errorf("task not found")
	}
	// A silence was an observation, not an outcome: the report joins it rather
	// than erasing the only trace of why the run looked quiet. The join happens
	// in the closing statement, on the summary as it is then, so a note another
	// instance added a moment before is not lost. The prefix holds neither % nor
	// _, so LIKE matches it literally.
	summaryExpr := "CASE WHEN summary LIKE ? THEN summary || ? || ? ELSE ? END"
	d.mu.Lock()
	// A run canceled by a disconnection is still its owner's to report on: the
	// server decided that outcome in the client's absence, so the client may
	// correct it. Only an identified caller may, since the server's own closure
	// path has no identity and must never rewrite an outcome it just recorded.
	// The note is matched anywhere in the summary: the hand-back appends its own
	// sentence after it, and a silence sentence comes before it on a run that
	// fell quiet before it was closed (#319). A cancellation someone typed never
	// carries that sentence and stays final.
	closable := "status='running'"
	args := []any{status, "%" + models.RunSilencePrefix + "%", " \u2014 ", note, note, time.Now(), runID, task.ID}
	if authorize != nil {
		closable = "(status='running' OR (status='canceled' AND summary LIKE ?))"
		args = append(args, "%"+models.RunDisconnectNote+"%")
	}
	result, err := d.conn.Exec("UPDATE task_activities SET status=?, summary="+summaryExpr+", completed_at=?, waiting_since=NULL, waiting_session='' WHERE id=? AND task_id=? AND skill_id='remote_run' AND "+closable, args...)
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	// Only the call that actually closed the run hands it back: finish_run is
	// idempotent, and a second report must not enqueue a second chain step.
	if count == 1 {
		d.handBackRun(task.ID, runID, status)
	}
	activity, err := d.GetActivityByID(runID)
	if err != nil {
		return nil, err
	}
	if count == 0 && (activity == nil || activity.TaskID != task.ID || activity.SkillID != "remote_run" || activity.Status != status) {
		return nil, fmt.Errorf("remote run not found or already finished with another status")
	}
	d.notifyPostBackListeners(task, activity, nil)
	return activity, nil
}

// SyncRemoteRunStatus reconciles a remote run's execution status reported by an agent.
// It updates an existing record or inserts a new one. A run that already ended
// stays ended: a late "running" from the agent never reopens a run the server,
// its owner or another instance has closed (#407). A terminal run may still move
// to another terminal status, since an owner may report the real outcome of a
// run the server closed (ADR 0007).
func (d *DB) SyncRemoteRunStatus(activityID, taskID, projectID, taskKey, skillName, status, summary string, startedAt *time.Time) (*models.TaskActivity, error) {
	return d.SyncRemoteRunStatusFor("", activityID, taskID, projectID, taskKey, skillName, status, summary, startedAt)
}

// SyncRemoteRunStatusFor is SyncRemoteRunStatus with the reporting agent's
// user. A run the agent reports is the agent's user's: a record inserted here
// takes that owner, and a record that has none yet adopts it, so a run started
// before ownership existed becomes stoppable by the person actually running it.
func (d *DB) SyncRemoteRunStatusFor(ownerID, activityID, taskID, projectID, taskKey, skillName, status, summary string, startedAt *time.Time) (*models.TaskActivity, error) {
	ownerID = strings.TrimSpace(ownerID)
	if status != "queued" && status != "running" && status != "completed" && status != "failed" && status != "canceled" {
		return nil, fmt.Errorf("invalid status: %s", status)
	}
	task, err := d.GetTaskByID(taskID)
	if err != nil {
		return nil, err
	}
	if task == nil && taskKey != "" {
		task, _ = d.GetTaskByID(taskKey)
	}
	realTaskID := taskID
	if task != nil {
		realTaskID = task.ID
	}

	d.mu.Lock()
	var existingID, currentStatus string
	err = d.conn.QueryRow("SELECT id, status FROM task_activities WHERE id = ?", activityID).Scan(&existingID, &currentStatus)
	now := time.Now()

	if err == nil && existingID != "" {
		if status == "running" {
			sAt := now
			if startedAt != nil && !startedAt.IsZero() {
				sAt = *startedAt
			}
			_, err = d.conn.Exec(`UPDATE task_activities SET status = ?, summary = ?, error = '', completed_at = NULL, started_at = COALESCE(started_at, ?)
				WHERE id = ? AND status NOT IN ('completed', 'failed', 'canceled')`,
				status, summary, sAt, activityID)
		} else if status == "queued" {
			_, err = d.conn.Exec(`UPDATE task_activities SET status = ?, summary = ?, error = '', completed_at = NULL, started_at = NULL
				WHERE id = ? AND status NOT IN ('completed', 'failed', 'canceled')`,
				status, summary, activityID)
		} else {
			// A terminal status ends the run, and a terminal run is never waiting.
			_, err = d.conn.Exec(`UPDATE task_activities SET status = ?, summary = ?, waiting_since = NULL, waiting_session = '' WHERE id = ?`,
				status, summary, activityID)
		}
		if err == nil && ownerID != "" {
			_, err = d.conn.Exec(`UPDATE task_activities SET user_id = ? WHERE id = ? AND user_id = ''`, ownerID, activityID)
		}
		if err != nil {
			d.mu.Unlock()
			return nil, err
		}
	} else {
		action := RunActionAgent
		if summary == "" {
			if status == "queued" {
				summary = "Execution queued on local agent"
			} else {
				summary = "Execution running on local agent"
			}
		}
		var sAt any = nil
		if startedAt != nil && !startedAt.IsZero() {
			sAt = *startedAt
		} else if status == "running" {
			sAt = now
		}
		// A run the server did not create is already executing on the agent:
		// refusing it would only lose track of it, so it is concurrent, out of
		// the one-run rule. Another instance inserting the same report first is
		// not an error.
		_, err = d.conn.Exec(`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action, status, summary, output, steps, prompt, started_at, completed_at, error, created_at, user_id, concurrent)
			VALUES (?, ?, 'remote_run', ?, ?, ?, ?, '', '[]', '', ?, NULL, '', ?, ?, 1)
			ON CONFLICT (id) DO NOTHING`,
			activityID, realTaskID, skillName, action, status, summary, sAt, now, ownerID)
		if err != nil {
			d.mu.Unlock()
			return nil, err
		}
	}
	d.mu.Unlock()

	activity, err := d.GetActivityByID(activityID)
	if err != nil {
		return nil, err
	}
	if task != nil {
		d.notifyPostBackListeners(task, activity, nil)
	}
	return activity, nil
}

// SetRemoteRunWaiting marks a running remote execution as blocked on the user,
// or clears that mark when it resumes, by hand: the mark belongs to no session,
// so no session's call ends it. A session declares its own wait through
// ReportSessionRunWaitingAs.
func (d *DB) SetRemoteRunWaiting(runID string, waiting bool) error {
	return d.setRemoteRunWaiting(runID, "", waiting)
}

// setRemoteRunWaiting marks a running remote execution as blocked on the user,
// on behalf of a session, or clears that mark. Only a run still `running` is
// touched: a report arriving late, after the session already ended, must not
// resurrect a waiting state on a closed run. The first mark wins: a second
// waiting report keeps the original instant, so the elapsed time the board
// shows is the real one, while the latest declaring session becomes the one
// whose next call ends the wait. Like its neighbours it ends on
// notifyPostBackListeners, which carries the change to the UI, and a change
// of the mark itself also reaches the wait listeners.
func (d *DB) setRemoteRunWaiting(runID, sessionID string, waiting bool) error {
	if strings.TrimSpace(runID) == "" {
		return fmt.Errorf("run id is required")
	}
	const live = " WHERE id=? AND skill_id='remote_run' AND status='running'"
	changed := "UPDATE task_activities SET waiting_since=NULL, waiting_session='', waiting_reason=''" + live + " AND waiting_since IS NOT NULL"
	changedArgs := []any{runID}
	// A clear of a run that was not waiting changes nothing, and is not an error.
	unchanged := "UPDATE task_activities SET waiting_reason=''" + live
	unchangedArgs := []any{runID}
	if waiting {
		changed = "UPDATE task_activities SET waiting_since=?, waiting_session=?, waiting_reason=''" + live + " AND waiting_since IS NULL"
		changedArgs = []any{time.Now(), sessionID, runID}
		unchanged = "UPDATE task_activities SET waiting_session=?, waiting_reason=''" + live
		unchangedArgs = []any{sessionID, runID}
	}
	d.mu.Lock()
	count, err := d.execCount(changed, changedArgs...)
	mark := count == 1
	if err == nil && !mark {
		count, err = d.execCount(unchanged, unchangedArgs...)
	}
	d.mu.Unlock()
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("remote run not found or no longer running")
	}
	d.notifyWaitChange(runID, mark)
	return nil
}

// execCount runs one statement and reports how many rows it changed.
func (d *DB) execCount(statement string, args ...any) (int64, error) {
	result, err := d.conn.Exec(statement, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// notifyWaitChange tells the listeners about a run whose waiting state was
// written: the postback listeners always, as every write of a run does, and
// the wait listeners when the mark itself changed.
func (d *DB) notifyWaitChange(runID string, changed bool) {
	activity, err := d.GetActivityByID(runID)
	if err != nil || activity == nil {
		return
	}
	task, err := d.GetTaskByID(activity.TaskID)
	if err != nil || task == nil {
		return
	}
	d.notifyPostBackListeners(task, activity, nil)
	if changed {
		d.notifyWaitListeners(task, activity)
	}
}

// WaitListener is told that a run's waiting mark was set or cleared. It is not
// told about the other writes of a run, so it can relay every one it hears.
type WaitListener func(task *models.Task, activity *models.TaskActivity)

// RegisterWaitListener registers a listener for the changes of a waiting mark.
func (d *DB) RegisterWaitListener(listener WaitListener) {
	d.postBackMu.Lock()
	defer d.postBackMu.Unlock()
	d.waitListeners = append(d.waitListeners, listener)
}

func (d *DB) notifyWaitListeners(task *models.Task, activity *models.TaskActivity) {
	d.postBackMu.RLock()
	defer d.postBackMu.RUnlock()
	for _, l := range d.waitListeners {
		fn := l
		go fn(task, activity)
	}
}

// ResumeWaits ends every wait a session declared, on a run still running. A
// session that makes a call is no longer blocked on its owner, whatever it
// forgot to report. The session is read from the run rather than from the
// memory of the instance that served the declaration, so the call ends the
// wait on any instance and after a restart (#475). It returns the runs it
// cleared; an empty session id clears nothing.
func (d *DB) ResumeWaits(sessionID string) ([]string, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, nil
	}
	rows, err := d.conn.Query(`SELECT id FROM task_activities
		WHERE waiting_session = ? AND waiting_since IS NOT NULL AND status = 'running' AND skill_id = 'remote_run'`, sessionID)
	if err != nil {
		return nil, err
	}
	var candidates []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		candidates = append(candidates, id)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var cleared []string
	for _, id := range candidates {
		// The session is matched again: another session may have taken the
		// wait over between the read and this write.
		d.mu.Lock()
		count, err := d.execCount(`UPDATE task_activities SET waiting_since=NULL, waiting_session='', waiting_reason=''
			WHERE id = ? AND waiting_session = ? AND waiting_since IS NOT NULL AND status = 'running' AND skill_id = 'remote_run'`, id, sessionID)
		d.mu.Unlock()
		if err != nil {
			return cleared, err
		}
		if count == 1 {
			cleared = append(cleared, id)
			d.notifyWaitChange(id, true)
		}
	}
	return cleared, nil
}

// AnswerRemoteRunWait ends a wait its owner answered in the run's console
// (#475). The agent names the mark it saw, and only that mark is cleared: a
// question asked after the answer was typed carries a newer instant and stays.
// Only a running run the agent dispatched for that same user qualifies, and
// only a question asked in the session: a launch parked on a repository is
// answered by a pin on the ticket, not by a key in the console. It reports
// whether a wait was cleared.
func (d *DB) AnswerRemoteRunWait(userID, runID string, waitingSince time.Time) (bool, error) {
	userID, runID = strings.TrimSpace(userID), strings.TrimSpace(runID)
	if userID == "" || runID == "" || waitingSince.IsZero() {
		return false, fmt.Errorf("user, run and answered wait are required")
	}
	var current sql.NullTime
	var reason string
	d.mu.Lock()
	err := d.conn.QueryRow(`SELECT waiting_since, waiting_reason FROM task_activities
		WHERE id = ? AND user_id = ? AND action = ? AND status = 'running' AND skill_id = 'remote_run'`,
		runID, userID, RunActionAgent).Scan(&current, &reason)
	if errors.Is(err, sql.ErrNoRows) {
		d.mu.Unlock()
		return false, nil
	}
	if err != nil {
		d.mu.Unlock()
		return false, err
	}
	if !current.Valid || reason != "" || !sameInstant(current.Time, waitingSince) {
		d.mu.Unlock()
		return false, nil
	}
	count, err := d.execCount(`UPDATE task_activities SET waiting_since=NULL, waiting_session='', waiting_reason=''
		WHERE id = ? AND user_id = ? AND waiting_since IS NOT NULL AND waiting_reason = '' AND status = 'running' AND skill_id = 'remote_run'`,
		runID, userID)
	d.mu.Unlock()
	if err != nil || count == 0 {
		return false, err
	}
	d.notifyWaitChange(runID, true)
	return true, nil
}

// sameInstant compares two waiting marks at the precision every engine keeps,
// since the agent sends back the instant it was pushed after a JSON round trip.
func sameInstant(a, b time.Time) bool {
	return a.Truncate(time.Microsecond).Equal(b.Truncate(time.Microsecond))
}

// ReportRemoteRunWaitingAs is ReportSessionRunWaitingAs for a caller with no
// session, whose wait no later call ends.
func (d *DB) ReportRemoteRunWaitingAs(caller Actor, admin bool, taskKey, runID string, waiting bool) (*models.TaskActivity, bool, error) {
	return d.ReportSessionRunWaitingAs(caller, admin, "", taskKey, runID, waiting)
}

// ReportSessionRunWaitingAs is setRemoteRunWaiting for a session reporting on
// a run, under the ownership rule of FinishRemoteRunAs: the owner, an admin, or
// anyone on a run with no recorded owner. It reports whether the mark was
// applied. A headless run has nobody to answer it, so a wait declared on one is
// accepted and ignored rather than shown to an owner who cannot act on it.
func (d *DB) ReportSessionRunWaitingAs(caller Actor, admin bool, sessionID, taskKey, runID string, waiting bool) (*models.TaskActivity, bool, error) {
	runID = strings.TrimSpace(runID)
	if strings.TrimSpace(taskKey) == "" || runID == "" {
		return nil, false, fmt.Errorf("taskKey and runId are required")
	}
	task, err := d.GetTaskByID(taskKey)
	if err != nil {
		return nil, false, err
	}
	if task == nil {
		return nil, false, fmt.Errorf("task not found")
	}
	existing, err := d.GetActivityByID(runID)
	if err != nil {
		return nil, false, err
	}
	if existing == nil || existing.TaskID != task.ID || existing.SkillID != "remote_run" || existing.Status != "running" {
		return nil, false, fmt.Errorf("remote run not found or no longer running")
	}
	if !admin && existing.UserID != "" && existing.UserID != caller.ID {
		return nil, false, ErrRunNotYours
	}
	if waiting && models.NormalizeSkillMode(d.runOutcomeOf(runID).Mode) == models.SkillModeAutonomous {
		return existing, false, nil
	}
	if err := d.setRemoteRunWaiting(runID, sessionID, waiting); err != nil {
		return nil, false, err
	}
	activity, err := d.GetActivityByID(runID)
	if err != nil {
		return nil, false, err
	}
	return activity, true, nil
}

// RemoteRunOutputLimit bounds what one autonomous run can record. A headless CLI
// streams everything it prints into a single activity, and an unbounded record
// grows with the run. Past the limit the output keeps its head, which is where
// the launch and the first errors are, and says it was cut. The limit counts
// characters, as SQL's LENGTH does on both engines.
const RemoteRunOutputLimit = 256 * 1024

const remoteRunOutputTruncated = "\n\n[output truncated: the run printed more than the recorded limit]"

// SetRemoteRunEngine records the engine a run is really running against, as the
// agent reports it once the command line is built. The launcher wrote its own
// resolution at launch, but only the agent sees the workstation override, so
// this is the value that ends up displayed.
//
// A run that is no longer running is left alone: a late report must not rewrite
// a finished record, which is the same rule SetRemoteRunWaiting follows.
func (d *DB) SetRemoteRunEngine(runID, provider, model string) error {
	if strings.TrimSpace(runID) == "" {
		return fmt.Errorf("run id is required")
	}
	provider, model = strings.TrimSpace(provider), strings.TrimSpace(model)
	d.mu.Lock()
	result, err := d.conn.Exec("UPDATE task_activities SET run_provider=?, run_model=? WHERE id=? AND skill_id='remote_run' AND status='running'",
		provider, model, runID)
	d.mu.Unlock()
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("remote run not found or no longer running")
	}
	activity, err := d.GetActivityByID(runID)
	if err != nil || activity == nil {
		return err
	}
	task, err := d.GetTaskByID(activity.TaskID)
	if err != nil || task == nil {
		return nil
	}
	d.notifyPostBackListeners(task, activity, nil)
	return nil
}

// AppendRemoteRunOutput adds captured CLI output to a running remote activity.
// It is the only channel an autonomous run has: nobody is watching a terminal,
// so what the CLI printed has to survive on the activity itself.
func (d *DB) AppendRemoteRunOutput(taskKey, runID, chunk string) error {
	if strings.TrimSpace(chunk) == "" {
		return nil
	}
	task, err := d.GetTaskByID(taskKey)
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("task not found")
	}
	// One statement, so two instances appending to the same run at once both
	// land: the row lock of the UPDATE orders them, and each appends to what the
	// other committed. LENGTH and SUBSTR count characters on both engines, so a
	// cut never splits a multi-byte character, which PostgreSQL would refuse.
	// The marker holds neither % nor _, so LIKE matches it literally.
	d.mu.Lock()
	defer d.mu.Unlock()
	res, err := d.conn.Exec(`
		UPDATE task_activities SET output = CASE
			WHEN output LIKE '%' || ? THEN output
			WHEN LENGTH(output) + LENGTH(CAST(? AS TEXT)) > ? THEN SUBSTR(output || CAST(? AS TEXT), 1, ?) || ?
			ELSE output || CAST(? AS TEXT)
		END
		WHERE id = ? AND task_id = ? AND skill_id = 'remote_run'`,
		remoteRunOutputTruncated, chunk, RemoteRunOutputLimit, chunk, RemoteRunOutputLimit, remoteRunOutputTruncated, chunk, runID, task.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
