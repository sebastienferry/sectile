package db

import "tasks/internal/models"

// FinishAgentLaunch records process-launch acknowledgement, not workflow completion.
func (d *DB) FinishAgentLaunch(activity models.TaskActivity) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.conn.Exec(`UPDATE task_activities SET status=?, summary=?, error=?, completed_at=? WHERE id=? AND skill_id='agent_launch'`, activity.Status, activity.Summary, activity.Error, activity.CompletedAt, activity.ID)
	return err
}
