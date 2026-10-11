package db

import "strings"

// WorkstationLabel resolves only the owner's paired devices, with an ID fallback.
func (d *DB) WorkstationLabel(userID, deviceID string) string {
	devices, err := d.ListDeviceCredentials(userID)
	if err == nil {
		for _, device := range devices {
			if device.ID == deviceID && strings.TrimSpace(device.Label) != "" {
				return strings.TrimSpace(device.Label)
			}
		}
	}
	return deviceID
}

// FillRunWorkstation records authenticated legacy attribution once. A different
// workstation or owner cannot replace the recorded assignment.
func (d *DB) FillRunWorkstation(runID, ownerID, deviceID string) error {
	if ownerID == "" || deviceID == "" {
		return nil
	}
	label := d.WorkstationLabel(ownerID, deviceID)
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.conn.Exec(`UPDATE task_activities SET device_id=?, device_label=?
 WHERE id=? AND user_id=? AND device_id='' AND skill_id='remote_run'`, deviceID, label, runID, ownerID)
	return err
}
