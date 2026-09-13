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
