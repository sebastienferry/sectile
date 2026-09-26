package db

import (
	"log"
	"strings"
	"time"
)

// An unlock of a sealed credential lasts while its owner is connected, then the
// idle window (#501, ADR 0031). Connected is read from what the store already
// records, so every instance reaches the same answer without keeping a clock of
// its own: a browser session that is open and was seen recently, or a local
// agent held by a live instance.
//
// The sweep, not the resolver, enforces it: an unlock past its window stays
// usable for at most one sweep period, and the resolver keeps its single query.

// unlockIdleWindow is how long an unlock outlives its owner's last presence.
// It is fixed on purpose: no setting.
const unlockIdleWindow = 30 * time.Minute

// unlockSweepEvery is how often each instance forgets idle unlocks. A variable
// so tests can shorten it, like instanceHeartbeatEvery.
var unlockSweepEvery = time.Minute

// presentBrowser is true when the user named by the outer row has an open
// browser session seen at or after the first parameter; the second is now.
const presentBrowser = `EXISTS (SELECT 1 FROM web_sessions s
	WHERE s.user_id = user_credential_unlocks.user_id
	  AND s.revoked_at IS NULL AND s.expires_at > ?
	  AND COALESCE(s.last_seen_at, s.created_at) >= ?)`

// presentAgent is true when the user named by the outer row has an agent whose
// last sign of life is at or after the parameter: now while it is held by a
// live instance (which heartbeats every few seconds), its instance's last
// heartbeat when that instance died holding it, its disconnection otherwise.
const presentAgent = `EXISTS (SELECT 1 FROM agent_presence p
	LEFT JOIN server_instances i ON i.id = p.instance_id
	WHERE p.user_id = user_credential_unlocks.user_id
	  AND COALESCE(p.disconnected_at, i.last_seen, p.connected_at) >= ?)`

// connectedAgent is true when the user named by the outer row has an agent
// connected to a live instance right now. Parameter: the liveness cutoff.
const connectedAgent = `EXISTS (SELECT 1 FROM agent_presence p
	JOIN server_instances i ON i.id = p.instance_id
	WHERE p.user_id = user_credential_unlocks.user_id
	  AND p.disconnected_at IS NULL AND i.last_seen >= ?)`

// ForgetIdleUnlocks deletes every unlock whose owner's last presence is older
// than the idle window at now, and says how many it forgot. It is one
// statement, so several instances running it at once forget each unlock once.
func (d *DB) ForgetIdleUnlocks(now time.Time) (int64, error) {
	now = now.UTC()
	cutoff := now.Add(-unlockIdleWindow)
	d.mu.Lock()
	defer d.mu.Unlock()
	result, err := d.conn.Exec(`DELETE FROM user_credential_unlocks
		WHERE unlocked_at < ?
		  AND (agent_seen_at IS NULL OR agent_seen_at < ?)
		  AND NOT `+presentBrowser+`
		  AND NOT `+presentAgent,
		cutoff, cutoff, now, cutoff, cutoff)
	if err != nil {
		return 0, err
	}
	forgotten, _ := result.RowsAffected()
	return forgotten, nil
}

// sweepIdleUnlocks is one pass of the instance loop over ForgetIdleUnlocks.
func (d *DB) sweepIdleUnlocks(now time.Time) {
	forgotten, err := d.ForgetIdleUnlocks(now)
	if err != nil {
		log.Printf("⚠️  Oubli des jetons descellés inactifs : %v", err)
	} else if forgotten > 0 {
		log.Printf("%d jeton(s) descellé(s) reverrouillé(s) : leur propriétaire est absent depuis plus de %s", forgotten, unlockIdleWindow)
	}
}

// ForgetUnlocksIfAbsent deletes one person's unlocks when, at now, they are not
// connected: no open browser session seen within the idle window, no agent
// held by a live instance. It is what signing out does, so the time of the
// unlock does not keep it alive, and neither does an agent that has already
// disconnected.
func (d *DB) ForgetUnlocksIfAbsent(userID string, now time.Time) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}
	now = now.UTC()
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.conn.Exec(`DELETE FROM user_credential_unlocks
		WHERE user_id = ?
		  AND NOT `+presentBrowser+`
		  AND NOT `+connectedAgent,
		userID, now, now.Add(-unlockIdleWindow), now.Add(-instanceDeadAfter))
	return err
}

// ForgetUserUnlocks deletes one person's unlocks whatever their presence, which
// is what blocking the account does.
func (d *DB) ForgetUserUnlocks(userID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.conn.Exec(`DELETE FROM user_credential_unlocks WHERE user_id = ?`, strings.TrimSpace(userID))
	return err
}

// keepAgentPresenceInUnlocks copies into agent_seen_at the last sign of life
// of the agents about to be forgotten with their instance, before their
// agent_presence rows go. Without it, an unlock kept alive by an agent alone
// would be forgotten the moment its instance is reclaimed, or the server
// restarts, instead of 30 minutes after the agent was last seen.
//
// liveSince is the liveness cutoff: only the agents of instances last seen
// before it, or already gone, are copied. A single-process store restarting
// passes now, since every instance row was written by an earlier process.
func (d *DB) keepAgentPresenceInUnlocks(liveSince time.Time) error {
	const lastSign = `COALESCE(p.disconnected_at, i.last_seen, p.connected_at)`
	const forgotten = `FROM agent_presence p LEFT JOIN server_instances i ON i.id = p.instance_id
		WHERE p.user_id = user_credential_unlocks.user_id
		  AND (i.id IS NULL OR i.last_seen < ?)`
	_, err := d.conn.Exec(`UPDATE user_credential_unlocks
		SET agent_seen_at = (SELECT MAX(`+lastSign+`) `+forgotten+`)
		WHERE EXISTS (SELECT 1 `+forgotten+`
		  AND (user_credential_unlocks.agent_seen_at IS NULL OR `+lastSign+` > user_credential_unlocks.agent_seen_at))`,
		liveSince, liveSince)
	return err
}
