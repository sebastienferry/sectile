package db

import (
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
	// Stage is the workflow stage of the task at launch.
	Stage string
	// ChainStop is set only on a step of a full chain run, to the stage that
	// chain stops at. Empty means this run chains nothing.
	ChainStop string
}

// StartRemoteRun creates an independent execution record. It never locks stages.
func (d *DB) StartRemoteRun(taskKey, skill, runID string) (*models.TaskActivity, error) {
	return d.startRemoteRun(taskKey, skill, runID, false, RunLaunch{})
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
	activity := &models.TaskActivity{ID: uuid.NewString(), TaskID: task.ID, ProjectID: task.ProjectID,
		SkillID: "remote_run", SkillName: skill, Action: RunActionClient,
		Status: "running", Summary: "Execution reported by a local agent or native client",
		CreatedAt: now, StartedAt: &now, Steps: []string{}}
	if agentOwned {
		activity.Action = RunActionAgent
	}
	if err := d.AddTaskActivity(*activity); err != nil {
		return nil, err
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

// FinishRemoteRun completes only the specified execution, preserving concurrent runs.
func (d *DB) FinishRemoteRun(taskKey, runID, status, note string) (*models.TaskActivity, error) {
	if status != "completed" && status != "failed" && status != "canceled" {
		return nil, fmt.Errorf("status must be completed, failed or canceled")
	}
	task, err := d.GetTaskByID(taskKey)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, fmt.Errorf("task not found")
	}
	d.mu.Lock()
	result, err := d.conn.Exec("UPDATE task_activities SET status=?, summary=?, completed_at=?, waiting_since=NULL WHERE id=? AND task_id=? AND skill_id='remote_run' AND status='running'", status, note, time.Now(), runID, task.ID)
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
// It updates existing records (clearing previous restart/failure errors) or inserts a new record.
func (d *DB) SyncRemoteRunStatus(activityID, taskID, projectID, taskKey, skillName, status, summary string, startedAt *time.Time) (*models.TaskActivity, error) {
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
			_, err = d.conn.Exec(`UPDATE task_activities SET status = ?, summary = ?, error = '', completed_at = NULL, started_at = COALESCE(started_at, ?) WHERE id = ?`,
				status, summary, sAt, activityID)
		} else if status == "queued" {
			_, err = d.conn.Exec(`UPDATE task_activities SET status = ?, summary = ?, error = '', completed_at = NULL, started_at = NULL WHERE id = ?`,
				status, summary, activityID)
		} else {
			// A terminal status ends the run, and a terminal run is never waiting.
			_, err = d.conn.Exec(`UPDATE task_activities SET status = ?, summary = ?, waiting_since = NULL WHERE id = ?`,
				status, summary, activityID)
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
		_, err = d.conn.Exec(`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action, status, summary, output, steps, prompt, started_at, completed_at, error, created_at)
			VALUES (?, ?, 'remote_run', ?, ?, ?, ?, '', '[]', '', ?, NULL, '', ?)`,
			activityID, realTaskID, skillName, action, status, summary, sAt, now)
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
// or clears that mark when it resumes. Only a run still `running` is touched: a
// hook reporting late, after the session already ended, must not resurrect a
// waiting state on a closed run. Like its neighbours it ends on
// notifyPostBackListeners, which is what carries the change to the UI.
func (d *DB) SetRemoteRunWaiting(runID string, waiting bool) error {
	if strings.TrimSpace(runID) == "" {
		return fmt.Errorf("run id is required")
	}
	var waitingSince any
	if waiting {
		waitingSince = time.Now()
	}
	d.mu.Lock()
	result, err := d.conn.Exec("UPDATE task_activities SET waiting_since=? WHERE id=? AND skill_id='remote_run' AND status='running'", waitingSince, runID)
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

// RemoteRunOutputLimit bounds what one autonomous run can record. A headless CLI
// streams everything it prints into a single activity, and an unbounded record
// grows with the run. Past the limit the output keeps its head, which is where
// the launch and the first errors are, and says it was cut.
const RemoteRunOutputLimit = 256 * 1024

const remoteRunOutputTruncated = "\n\n[output truncated: the run printed more than the recorded limit]"

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
	d.mu.Lock()
	defer d.mu.Unlock()
	var current string
	if err := d.conn.QueryRow("SELECT output FROM task_activities WHERE id=? AND task_id=? AND skill_id='remote_run'", runID, task.ID).Scan(&current); err != nil {
		return err
	}
	if strings.HasSuffix(current, remoteRunOutputTruncated) {
		return nil
	}
	combined := current + chunk
	if len(combined) > RemoteRunOutputLimit {
		combined = combined[:RemoteRunOutputLimit] + remoteRunOutputTruncated
	}
	_, err = d.conn.Exec("UPDATE task_activities SET output=? WHERE id=? AND task_id=?", combined, runID, task.ID)
	return err
}
