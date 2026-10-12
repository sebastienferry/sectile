package db

import "tasks/internal/models"

// RetryTaskRequest preserves the old launch's prompt, mode, model and batch
// membership while allowing a new explicit workstation assignment.
func (d *DB) RetryTaskRequest(run *models.TaskActivity, projectID, deviceID string) (models.RunSkillRequest, error) {
	request := models.RunSkillRequest{SkillID: run.SkillID, Prompt: run.Prompt, ProjectID: projectID, DeviceID: deviceID, Model: run.Model}
	if request.SkillID == "remote_run" || request.SkillID == "agent_launch" {
		request.SkillID = models.NormalizeSkillID(run.SkillName)
	}
	d.mu.RLock()
	err := d.conn.QueryRow("SELECT COALESCE(run_mode, '') FROM task_activities WHERE id=?", run.ID).Scan(&request.Mode)
	d.mu.RUnlock()
	if err != nil {
		return request, err
	}
	request.BatchTaskIDs = d.batchMemberIDs(run.ID)
	return request, nil
}
