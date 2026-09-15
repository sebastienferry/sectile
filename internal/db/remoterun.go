package db

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"tasks/internal/models"
)

// StartRemoteRun creates an independent execution record. It never locks stages.
func (d *DB) StartRemoteRun(taskKey, skill, runID string) (*models.TaskActivity, error) {
	return d.startRemoteRun(taskKey, skill, runID, false)
}
func (d *DB) StartAgentRemoteRun(taskKey, skill string) (*models.TaskActivity, error) {
	return d.startRemoteRun(taskKey, skill, "", true)
}
func (d *DB) startRemoteRun(taskKey, skill, runID string, agentOwned bool) (*models.TaskActivity, error) {
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
		SkillID: "remote_run", SkillName: skill, Action: "Remote skill execution",
		Status: "running", Summary: "Execution reported by a local agent or native client",
		CreatedAt: now, StartedAt: &now, Steps: []string{}}
	if agentOwned {
		activity.Action = "Agent-owned remote execution"
	}
	if err := d.AddTaskActivity(*activity); err != nil {
		return nil, err
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
	result, err := d.conn.Exec("UPDATE task_activities SET status=?, summary=?, completed_at=? WHERE id=? AND task_id=? AND skill_id='remote_run' AND status='running'", status, note, time.Now(), runID, task.ID)
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return nil, err
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
			_, err = d.conn.Exec(`UPDATE task_activities SET status = ?, summary = ? WHERE id = ?`,
				status, summary, activityID)
		}
		if err != nil {
			d.mu.Unlock()
			return nil, err
		}
	} else {
		action := "Agent-owned remote execution"
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

