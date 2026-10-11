package db

import (
	"database/sql"
	"errors"
)

// TaskLaunchDevice returns only acknowledged launches in this owner's project.
// Execution failure after a successful launch does not erase local work affinity.
func (d *DB) TaskLaunchDevice(taskID, projectID, userID string) (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var device string
	err := d.conn.QueryRow(`SELECT device_id FROM task_activities
 WHERE task_id=? AND run_project_id=? AND user_id=? AND skill_id='agent_launch'
 AND status='completed' AND device_id<>'' ORDER BY created_at DESC, id DESC LIMIT 1`, taskID, projectID, userID).Scan(&device)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return device, err
}
