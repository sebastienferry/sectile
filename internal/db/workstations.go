package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// WorkstationConnected atomically claims only the authenticated device slot.
// The no-op upsert locks that identity until the replacement and route commit.
func (d *DB) WorkstationConnected(l AgentLocation) (AgentLocation, bool, error) {
	tx, err := d.conn.Begin()
	if err != nil {
		return AgentLocation{}, false, err
	}
	defer tx.Rollback()
	old := AgentLocation{UserID: l.UserID, ProjectID: l.ProjectID, DeviceID: l.DeviceID}
	err = tx.QueryRow(`INSERT INTO agent_workstation_presence
 (connection_id,user_id,device_id,project_scope,instance_id,connected_at,last_ping_at)
 VALUES (?,?,?,?,?,?,?) ON CONFLICT(user_id,device_id)
 DO UPDATE SET connection_id = agent_workstation_presence.connection_id
 RETURNING connection_id,instance_id,connected_at,project_scope`, l.ConnectionID, l.UserID, l.DeviceID, l.ProjectID, d.instanceID, l.ConnectedAt, l.ConnectedAt).Scan(&old.ConnectionID, &old.InstanceID, &old.ConnectedAt, &old.ProjectID)
	if err != nil {
		return old, false, err
	}
	had := old.ConnectionID != l.ConnectionID
	if had {
		if _, err = tx.Exec(`UPDATE agent_workstation_presence SET connection_id=?,instance_id=?,connected_at=?,last_ping_at=?,project_scope=?,inventory_state='unknown',inventory_updated_at=NULL,inventory_json='[]' WHERE connection_id=?`, l.ConnectionID, d.instanceID, l.ConnectedAt, l.ConnectedAt, l.ProjectID, old.ConnectionID); err != nil {
			return old, false, err
		}
		_ = tx.QueryRow(`SELECT address FROM server_instances WHERE id=?`, old.InstanceID).Scan(&old.Address)
	}
	if _, err = tx.Exec(`INSERT INTO agent_presence(user_id,project_id,instance_id,device_id,connected_at,disconnected_at)
 VALUES(?,?,?,?,?,NULL) ON CONFLICT(user_id,device_id) DO UPDATE SET project_id=excluded.project_id,instance_id=excluded.instance_id,connected_at=excluded.connected_at,disconnected_at=NULL`, l.UserID, l.ProjectID, d.instanceID, l.DeviceID, l.ConnectedAt); err != nil {
		return old, false, err
	}
	return old, had, tx.Commit()
}

func (d *DB) WorkstationInventory(id string, projects []string, now time.Time) error {
	raw, err := json.Marshal(projects)
	if err != nil {
		return err
	}
	result, err := d.conn.Exec(`UPDATE agent_workstation_presence SET inventory_state='reported',inventory_json=?,inventory_updated_at=? WHERE connection_id=? AND instance_id=?`, string(raw), now, id, d.instanceID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count != 1 {
		return errors.New("inactive workstation session")
	}
	return err
}
func (d *DB) WorkstationPing(id string, now time.Time) error {
	_, err := d.conn.Exec(`UPDATE agent_workstation_presence SET last_ping_at=? WHERE connection_id=? AND instance_id=?`, now, id, d.instanceID)
	return err
}

func (d *DB) WorkstationDisconnected(id string) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var user, scope, device string
	var connected time.Time
	err = tx.QueryRow(`DELETE FROM agent_workstation_presence WHERE connection_id=? AND instance_id=? RETURNING user_id,project_scope,device_id,connected_at`, id, d.instanceID).Scan(&user, &scope, &device, &connected)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	// Retain the departure for credential idle expiry even when another device owns the route.
	now := time.Now().UTC()
	if _, err = tx.Exec(`UPDATE user_credential_unlocks SET agent_seen_at=? WHERE user_id=? AND (agent_seen_at IS NULL OR agent_seen_at < ?)`, now, user, now); err != nil {
		return err
	}
	// Retire only this exact device generation; another device remains untouched.
	_, err = tx.Exec(`UPDATE agent_presence SET disconnected_at=? WHERE user_id=? AND device_id=? AND instance_id=? AND disconnected_at IS NULL AND connected_at=?`, now, user, device, d.instanceID, connected)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (d *DB) ConnectedWorkstations() []AgentLocation {
	rows, err := d.conn.Query(`SELECT p.connection_id,p.user_id,p.project_scope,p.instance_id,i.address,p.device_id,p.connected_at,p.last_ping_at,p.inventory_state,p.inventory_updated_at,p.inventory_json FROM agent_workstation_presence p JOIN server_instances i ON i.id=p.instance_id WHERE i.last_seen>=? ORDER BY p.connected_at`, time.Now().UTC().Add(-instanceDeadAfter))
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []AgentLocation{}
	for rows.Next() {
		var l AgentLocation
		var updated sql.NullTime
		var raw string
		if rows.Scan(&l.ConnectionID, &l.UserID, &l.ProjectID, &l.InstanceID, &l.Address, &l.DeviceID, &l.ConnectedAt, &l.LastPingAt, &l.InventoryState, &updated, &raw) != nil {
			continue
		}
		if json.Unmarshal([]byte(raw), &l.ProjectIDs) != nil {
			continue
		}
		if updated.Valid {
			l.InventoryUpdatedAt = &updated.Time
		}
		out = append(out, l)
	}
	return out
}
