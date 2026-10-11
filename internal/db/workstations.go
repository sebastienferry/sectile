package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// WorkstationConnected atomically claims only the authenticated device/scope slot.
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
 VALUES (?,?,?,?,?,?,?) ON CONFLICT(user_id,device_id,project_scope)
 DO UPDATE SET connection_id = agent_workstation_presence.connection_id
 RETURNING connection_id,instance_id,connected_at`, l.ConnectionID, l.UserID, l.DeviceID, l.ProjectID, d.instanceID, l.ConnectedAt, l.ConnectedAt).Scan(&old.ConnectionID, &old.InstanceID, &old.ConnectedAt)
	if err != nil {
		return old, false, err
	}
	had := old.ConnectionID != l.ConnectionID
	if had {
		if _, err = tx.Exec(`UPDATE agent_workstation_presence SET connection_id=?,instance_id=?,connected_at=?,last_ping_at=?,inventory_state='unknown',inventory_updated_at=NULL,inventory_json='[]' WHERE connection_id=?`, l.ConnectionID, d.instanceID, l.ConnectedAt, l.ConnectedAt, old.ConnectionID); err != nil {
			return old, false, err
		}
		_ = tx.QueryRow(`SELECT address FROM server_instances WHERE id=?`, old.InstanceID).Scan(&old.Address)
	}
	if _, err = tx.Exec(`INSERT INTO agent_presence(user_id,project_id,instance_id,device_id,connected_at,disconnected_at)
 VALUES(?,?,?,?,?,NULL) ON CONFLICT(user_id,project_id) DO UPDATE SET instance_id=excluded.instance_id,device_id=excluded.device_id,connected_at=excluded.connected_at,disconnected_at=NULL
 WHERE agent_presence.connected_at <= excluded.connected_at`, l.UserID, l.ProjectID, d.instanceID, l.DeviceID, l.ConnectedAt); err != nil {
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
	err = tx.QueryRow(`DELETE FROM agent_workstation_presence WHERE connection_id=? AND instance_id=? RETURNING user_id,project_scope,device_id`, id, d.instanceID).Scan(&user, &scope, &device)
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
	// Lock the compatibility row before selecting its newest remaining owner.
	if _, err = tx.Exec(`UPDATE agent_presence SET disconnected_at=disconnected_at WHERE user_id=? AND project_id=?`, user, scope); err != nil {
		return err
	}
	var instance string
	var connected time.Time
	err = tx.QueryRow(`SELECT p.instance_id,p.device_id,p.connected_at FROM agent_workstation_presence p JOIN server_instances i ON i.id=p.instance_id WHERE p.user_id=? AND p.project_scope=? AND i.last_seen>=? ORDER BY p.connected_at DESC LIMIT 1`, user, scope, now.Add(-instanceDeadAfter)).Scan(&instance, &device, &connected)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.Exec(`UPDATE agent_presence SET disconnected_at=? WHERE user_id=? AND project_id=?`, now, user, scope)
	} else if err == nil {
		_, err = tx.Exec(`UPDATE agent_presence SET instance_id=?,device_id=?,connected_at=?,disconnected_at=NULL WHERE user_id=? AND project_id=?`, instance, device, connected, user, scope)
	}
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

// workstationHolder resolves a compatibility route without consulting inventory.
func (d *DB) workstationHolder(userID, scope string) (AgentLocation, bool) {
	l := AgentLocation{UserID: userID, ProjectID: scope}
	err := d.conn.QueryRow(`SELECT p.connection_id,p.instance_id,i.address,p.device_id,p.connected_at,p.last_ping_at
 FROM agent_workstation_presence p JOIN server_instances i ON i.id=p.instance_id
 WHERE p.user_id=? AND p.project_scope=? AND i.last_seen>=?
 ORDER BY p.connected_at DESC LIMIT 1`, userID, scope, time.Now().UTC().Add(-instanceDeadAfter)).Scan(&l.ConnectionID, &l.InstanceID, &l.Address, &l.DeviceID, &l.ConnectedAt, &l.LastPingAt)
	return l, err == nil
}
