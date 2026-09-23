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
// slot (a user and a project), so another instance can forward agent work to
// it instead of answering that no agent is connected. Only live instances
// count: the row of an instance that stopped answering is ignored, and removed
// by the reaper.

// agentProjectFallbacks are the project slots a lookup falls back to, in
// order, after the exact one: an agent that registered for every project.
var agentProjectFallbacks = []string{"default", "all", ""}

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
func (d *DB) AgentConnected(userID, projectID, deviceID string) (AgentLocation, bool, error) {
	previous, had := d.agentHolder(userID, projectID)
	now := time.Now().UTC()
	if _, err := d.conn.Exec(`INSERT INTO agent_presence (user_id, project_id, instance_id, device_id, connected_at, disconnected_at)
		VALUES (?, ?, ?, ?, ?, NULL)
		ON CONFLICT (user_id, project_id) DO UPDATE SET instance_id = excluded.instance_id, device_id = excluded.device_id,
			connected_at = excluded.connected_at, disconnected_at = NULL`,
		userID, projectID, d.instanceID, deviceID, now); err != nil {
		return AgentLocation{}, false, fmt.Errorf("recording the agent of %s on %s: %w", userID, projectID, err)
	}
	if !had || previous.InstanceID == d.instanceID {
		return AgentLocation{}, false, nil
	}
	return previous, true, nil
}

// AgentDisconnected records that this instance no longer holds an agent slot.
// A slot another instance or device has taken since is left alone.
func (d *DB) AgentDisconnected(userID, projectID, deviceID string) {
	_, _ = d.conn.Exec(`UPDATE agent_presence SET disconnected_at = ?
		WHERE user_id = ? AND project_id = ? AND instance_id = ? AND device_id = ? AND disconnected_at IS NULL`,
		time.Now().UTC(), userID, projectID, d.instanceID, deviceID)
}

// AgentOwner returns the live instance holding the agent for a user and
// project, falling back to an agent registered for every project.
func (d *DB) AgentOwner(userID, projectID string) (AgentLocation, bool) {
	for _, slot := range append([]string{projectID}, agentProjectFallbacks...) {
		if location, ok := d.agentHolder(userID, slot); ok {
			return location, true
		}
	}
	return AgentLocation{}, false
}

// agentHolder is the live instance holding exactly this slot.
func (d *DB) agentHolder(userID, projectID string) (AgentLocation, bool) {
	cutoff := time.Now().UTC().Add(-instanceDeadAfter)
	location := AgentLocation{UserID: userID, ProjectID: projectID}
	err := d.conn.QueryRow(`SELECT p.instance_id, i.address, p.device_id, p.connected_at
		FROM agent_presence p JOIN server_instances i ON i.id = p.instance_id
		WHERE p.user_id = ? AND p.project_id = ? AND p.disconnected_at IS NULL AND i.last_seen >= ?`,
		userID, projectID, cutoff).Scan(&location.InstanceID, &location.Address, &location.DeviceID, &location.ConnectedAt)
	if err != nil {
		return AgentLocation{}, false
	}
	return location, true
}

// AgentRecentlyConnected reports whether a slot, or its fallbacks, held an
// agent on any instance since the given instant, so an absence can be read as
// a reconnection in progress rather than as no agent at all.
func (d *DB) AgentRecentlyConnected(userID, projectID string, since time.Time) bool {
	for _, slot := range append([]string{projectID}, agentProjectFallbacks...) {
		var connected time.Time
		var disconnected sql.NullTime
		err := d.conn.QueryRow(`SELECT connected_at, disconnected_at FROM agent_presence WHERE user_id = ? AND project_id = ?`,
			userID, slot).Scan(&connected, &disconnected)
		if errors.Is(err, sql.ErrNoRows) || err != nil {
			continue
		}
		if !disconnected.Valid || connected.After(since) || disconnected.Time.After(since) {
			return true
		}
	}
	return false
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
