package db

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// A local agent keeps one WebSocket to whichever server instance the load
// balancer gave it. agent_presence records which instance that is, per agent
// slot (a user and a device), so another instance can forward agent work to
// it instead of answering that no agent is connected. Only live instances
// count: the row of an instance that stopped answering is ignored, and removed
// by the reaper.

// AgentLocation is where an agent slot is held.
type AgentLocation struct {
	UserID      string
	ProjectID   string
	InstanceID  string
	Address     string
	DeviceID    string
	ConnectedAt time.Time
}

// internalTokenLabel is what the internal bearer is derived from.
const internalTokenLabel = "sectile-internal-v1"

// Shared reports whether other server instances may share this store, which is
// when agent work may have to be forwarded between them.
func (d *DB) Shared() bool { return !d.dialect.ServesOneProcess() }

// SetInstanceAddress records where the other instances reach this one. It is
// set before StartInstance, which writes it with the instance row.
func (d *DB) SetInstanceAddress(address string) { d.instanceAddress = address }

// InternalToken is the bearer the instances present to each other. It is
// derived from the server key every instance of a shared store already holds,
// so no second secret has to be distributed.
func (d *DB) InternalToken() (string, error) {
	if d.serverKeyErr != nil {
		return "", fmt.Errorf("no server key to authenticate the internal calls: %w", d.serverKeyErr)
	}
	mac := hmac.New(sha256.New, d.serverKey[:])
	mac.Write([]byte(internalTokenLabel))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// AgentConnected records that this instance now holds an agent slot, and
// returns the live instance that held it before, if it was another one.
func (d *DB) AgentConnected(userID, projectID, deviceID string, generation ...time.Time) (AgentLocation, bool, error) {
	if deviceID == "" {
		deviceID = "unknown"
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if len(generation) > 0 {
		now = generation[0].UTC()
	}
	var previous AgentLocation
	had := false
	err := d.conn.WithTx(func(tx *sqlTx) error {
		// The insert locks even an absent slot, serializing concurrent first
		// connections before reading and replacing its holder on PostgreSQL.
		_, err := tx.Exec(`INSERT INTO agent_presence (user_id, project_id, instance_id, device_id, connected_at, disconnected_at)
   VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT (user_id, device_id) DO NOTHING`, userID, projectID, d.instanceID, deviceID, now, now)
		if err != nil {
			return err
		}
		var disconnected sql.NullTime
		previous.UserID, previous.DeviceID = userID, deviceID
		err = tx.QueryRow(`SELECT project_id, instance_id, connected_at, disconnected_at FROM agent_presence
   WHERE user_id = ? AND device_id = ?`+d.dialect.ForUpdate(), userID, deviceID).Scan(&previous.ProjectID, &previous.InstanceID, &previous.ConnectedAt, &disconnected)
		if err != nil {
			return err
		}
		if !disconnected.Valid && previous.InstanceID != d.instanceID {
			var seen time.Time
			err = tx.QueryRow(`SELECT address, last_seen FROM server_instances WHERE id = ?`, previous.InstanceID).Scan(&previous.Address, &seen)
			had = err == nil && seen.After(time.Now().UTC().Add(-instanceDeadAfter))
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
		_, err = tx.Exec(`UPDATE agent_presence SET project_id = ?, instance_id = ?, connected_at = ?, disconnected_at = NULL
   WHERE user_id = ? AND device_id = ?`, projectID, d.instanceID, now, userID, deviceID)
		return err
	})
	return previous, had, err
}

// AgentDisconnected matches the connection generation as well as its holder.
func (d *DB) AgentDisconnected(userID, projectID, deviceID string, generation ...time.Time) {
	query := `UPDATE agent_presence SET disconnected_at = ? WHERE user_id = ? AND device_id = ? AND instance_id = ? AND disconnected_at IS NULL`
	args := []any{time.Now().UTC(), userID, deviceID, d.instanceID}
	if len(generation) > 0 {
		query += ` AND connected_at = ?`
		args = append(args, generation[0].UTC())
	}
	_, _ = d.conn.Exec(query, args...)
}

// AgentOwner returns the live instance holding the agent for a user and
// project, falling back to an agent registered for every project.
func (d *DB) AgentOwner(userID, projectID string) (AgentLocation, bool) {
	var selected AgentLocation
	count := 0
	for _, location := range d.ConnectedAgentLocations() {
		if location.UserID != userID {
			continue
		}
		count++
		selected = location
	}
	eligible := selected.ProjectID == projectID || selected.ProjectID == "default" || selected.ProjectID == "all" || selected.ProjectID == ""
	return selected, count == 1 && eligible
}

// AgentRecentlyConnected reports whether a slot, or its fallbacks, held an
// agent on any instance since the given instant, so an absence can be read as
// a reconnection in progress rather than as no agent at all.
func (d *DB) AgentRecentlyConnected(userID, projectID string, since time.Time) bool {
	var count int
	err := d.conn.QueryRow(`SELECT COUNT(*) FROM agent_presence WHERE user_id = ?
  AND (project_id = ? OR project_id IN ('default', 'all', ''))
  AND (disconnected_at IS NULL OR connected_at > ? OR disconnected_at > ?)`, userID, projectID, since, since).Scan(&count)
	return err == nil && count > 0
}

// ConnectedAgentLocations lists the agents held by live instances.
func (d *DB) ConnectedAgentLocations() []AgentLocation {
	cutoff := time.Now().UTC().Add(-instanceDeadAfter)
	rows, err := d.conn.Query(`SELECT p.user_id, p.project_id, p.instance_id, i.address, p.device_id, p.connected_at
		FROM agent_presence p JOIN server_instances i ON i.id = p.instance_id
		WHERE p.disconnected_at IS NULL AND i.last_seen >= ?
		ORDER BY p.connected_at`, cutoff)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []AgentLocation
	for rows.Next() {
		var l AgentLocation
		if rows.Scan(&l.UserID, &l.ProjectID, &l.InstanceID, &l.Address, &l.DeviceID, &l.ConnectedAt) == nil {
			out = append(out, l)
		}
	}
	return out
}

// RecentAgentDevices lists eligible device slots whose absence may be a
// reconnect, including a slot last held on another replica.
func (d *DB) RecentAgentDevices(userID, projectID string, since time.Time) []string {
	rows, err := d.conn.Query(`SELECT device_id FROM agent_presence WHERE user_id = ?
  AND (project_id = ? OR project_id IN ('default','all',''))
  AND (disconnected_at IS NULL OR connected_at > ? OR disconnected_at > ?) ORDER BY device_id`, userID, projectID, since, since)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var devices []string
	for rows.Next() {
		var device string
		if rows.Scan(&device) == nil {
			devices = append(devices, device)
		}
	}
	return devices
}
