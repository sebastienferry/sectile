package db

import (
	"bytes"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tasks/internal/secrets"
)

// unlockRows counts the unlocks one person holds.
func unlockRows(t *testing.T, d *DB, userID string) int {
	t.Helper()
	var n int
	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM user_credential_unlocks WHERE user_id = ?`, userID).Scan(&n); err != nil {
		t.Fatalf("counting unlocks: %v", err)
	}
	return n
}

// sealedAndUnlocked stores a sealed Jira credential for a person, which leaves
// it unlocked, and backdates that unlock to unlockedAt.
func sealedAndUnlocked(t *testing.T, d *DB, userID string, unlockedAt time.Time) {
	t.Helper()
	if err := d.EnsureUser(userID); err != nil {
		t.Fatal(err)
	}
	if err := d.SetUserTrackerCredential(userID, "jira", "https://acme.atlassian.net", "ada@example.com", "sealed-token", "open sesame"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec(`UPDATE user_credential_unlocks SET unlocked_at = ? WHERE user_id = ?`, unlockedAt.UTC(), userID); err != nil {
		t.Fatal(err)
	}
}

// browserSession opens a web session for a person, last seen at seenAt.
func browserSession(t *testing.T, d *DB, userID string, seenAt time.Time) string {
	t.Helper()
	token, _, err := d.CreateWebSession(userID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec(`UPDATE web_sessions SET created_at = ?, last_seen_at = ? WHERE token_hash = ?`, seenAt.UTC(), seenAt.UTC(), hashSecret(token)); err != nil {
		t.Fatal(err)
	}
	return token
}

// serverInstance registers an instance last seen at lastSeen.
func serverInstance(t *testing.T, d *DB, id string, lastSeen time.Time) {
	t.Helper()
	if _, err := d.conn.Exec(`INSERT INTO server_instances (id, hostname, pid, started_at, last_seen, address) VALUES (?, 'h', 1, ?, ?, '')`,
		id, lastSeen.UTC(), lastSeen.UTC()); err != nil {
		t.Fatal(err)
	}
}

// agentOn records an agent of a person on an instance, disconnected at
// disconnectedAt unless it is zero.
func agentOn(t *testing.T, d *DB, userID, instanceID string, connectedAt, disconnectedAt time.Time) {
	t.Helper()
	var disconnected any
	if !disconnectedAt.IsZero() {
		disconnected = disconnectedAt.UTC()
	}
	if _, err := d.conn.Exec(`INSERT INTO agent_presence (user_id, project_id, instance_id, device_id, connected_at, disconnected_at) VALUES (?, 'p1', ?, 'd1', ?, ?)`,
		userID, instanceID, connectedAt.UTC(), disconnected); err != nil {
		t.Fatal(err)
	}
}

// An unlock is written to the database, so a server that restarts on the same
// database still has it (US1).
func TestAnUnlockSurvivesARestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.db")
	first, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.SetUserTrackerCredential("u1", "jira", "https://acme.atlassian.net", "", "sealed-token", "open sesame"); err != nil {
		t.Fatal(err)
	}
	first.Close()

	restarted, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if _, _, token, err := restarted.userTrackerCredential("u1", "jira"); err != nil || token != "sealed-token" {
		t.Fatalf("the unlock must survive the restart: %q %v", token, err)
	}
	credentials, err := restarted.UserTrackerCredentials("u1")
	if err != nil || len(credentials) != 1 || !credentials[0].Unlocked {
		t.Fatalf("the profile must show it unlocked: %+v %v", credentials, err)
	}
}

// Two handles on one database stand for two replicas: an unlock made through
// one is honoured by the other, and so is a lock (US2).
func TestEveryReplicaSeesTheSameUnlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.db")
	a, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	if err := a.SetUserTrackerCredential("u1", "jira", "https://acme.atlassian.net", "", "sealed-token", "open sesame"); err != nil {
		t.Fatal(err)
	}
	if err := a.LockUserTrackerCredential("u1", "jira"); err != nil {
		t.Fatal(err)
	}
	if err := a.UnlockUserTrackerCredential("u1", "jira", "open sesame"); err != nil {
		t.Fatal(err)
	}
	if _, _, token, err := b.userTrackerCredential("u1", "jira"); err != nil || token != "sealed-token" {
		t.Fatalf("replica B must use the unlock made on A: %q %v", token, err)
	}
	if err := a.LockUserTrackerCredential("u1", "jira"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := b.userTrackerCredential("u1", "jira"); !errors.Is(err, ErrCredentialLocked) {
		t.Fatalf("a lock on A must reach B: %v", err)
	}
	credentials, err := b.UserTrackerCredentials("u1")
	if err != nil || len(credentials) != 1 || credentials[0].Unlocked {
		t.Fatalf("B's profile must show it locked: %+v %v", credentials, err)
	}
}

// The stored unlock is sealed under the server key: neither the passphrase,
// the derived key nor the token can be read from the row (FR-8).
func TestAStoredUnlockHoldsNoSecretInClear(t *testing.T) {
	database := testDB(t)
	if err := database.SetUserTrackerCredential("u1", "jira", "", "", "sealed-token", "open sesame"); err != nil {
		t.Fatal(err)
	}
	var wrapped []byte
	var salt []byte
	if err := database.conn.QueryRow(`SELECT u.wrapped_key, c.salt FROM user_credential_unlocks u JOIN user_tracker_credentials c ON c.user_id = u.user_id AND c.tracker = u.tracker`).Scan(&wrapped, &salt); err != nil {
		t.Fatal(err)
	}
	derived := secrets.DeriveKey("open sesame", salt)
	for _, secret := range [][]byte{[]byte("open sesame"), []byte("sealed-token"), derived[:], []byte(hex.EncodeToString(derived[:]))} {
		if bytes.Contains(wrapped, secret) {
			t.Fatalf("the unlock row carries %q in clear", secret)
		}
	}

	// Moved to another person, it opens nothing.
	if err := database.SetUserTrackerCredential("u2", "jira", "", "", "other-token", "open sesame"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.conn.Exec(`UPDATE user_credential_unlocks SET wrapped_key = ? WHERE user_id = 'u2'`, wrapped); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := database.userTrackerCredential("u2", "jira"); !errors.Is(err, ErrCredentialLocked) {
		t.Fatalf("a moved unlock must not open: %v", err)
	}
}

// An unlock the server key cannot open, because the key was replaced, is a
// locked credential, and the row is not kept.
func TestAnUnlockTheServerKeyCannotOpenIsForgotten(t *testing.T) {
	database := testDB(t)
	if err := database.SetUserTrackerCredential("u1", "jira", "", "", "sealed-token", "open sesame"); err != nil {
		t.Fatal(err)
	}
	other, err := secrets.ServerKey(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	database.serverKey = other
	credentials, err := database.UserTrackerCredentials("u1")
	if err != nil || len(credentials) != 1 || credentials[0].Unlocked {
		t.Fatalf("the profile must show it locked: %+v %v", credentials, err)
	}
	if _, _, _, err := database.userTrackerCredential("u1", "jira"); !errors.Is(err, ErrCredentialLocked) {
		t.Fatalf("it must be locked: %v", err)
	}
	if n := unlockRows(t, database, "u1"); n != 0 {
		t.Fatalf("the unusable unlock must be forgotten, %d left", n)
	}
}

// Without a server key there is nowhere to keep an unlock: saying "unlocked"
// would only hold until the next restart (FR-9).
func TestUnlockingWithoutAServerKeyIsRefused(t *testing.T) {
	database := testDB(t)
	if err := database.SetUserTrackerCredential("u1", "jira", "", "", "sealed-token", "open sesame"); err != nil {
		t.Fatal(err)
	}
	if err := database.LockUserTrackerCredential("u1", "jira"); err != nil {
		t.Fatal(err)
	}
	database.serverKeyErr = errors.New("volume en lecture seule")
	err := database.UnlockUserTrackerCredential("u1", "jira", "open sesame")
	if !errors.Is(err, ErrServerKeyUnavailable) || !strings.Contains(err.Error(), secrets.KeyEnvVar) {
		t.Fatalf("the unlock must be refused and name the way out: %v", err)
	}
	if n := unlockRows(t, database, "u1"); n != 0 {
		t.Fatalf("nothing must be stored, %d unlock(s) found", n)
	}
	// A wrong passphrase is still a wrong passphrase.
	if err := database.UnlockUserTrackerCredential("u1", "jira", "wrong"); !errors.Is(err, secrets.ErrWrongKey) {
		t.Fatalf("a wrong passphrase: %v", err)
	}
}

// The sweep: an unlock lasts while its owner is present, and 30 minutes more
// (US3, US4, FR-2).
func TestTheSweepForgetsOnlyTheUnlocksOfPeopleWhoLeft(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	long := now.Add(-3 * time.Hour)
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, d *DB)
		kept  bool
	}{
		{"nobody present", func(t *testing.T, d *DB) {}, false},
		{"session seen 29 minutes ago", func(t *testing.T, d *DB) {
			browserSession(t, d, "u1", now.Add(-29*time.Minute))
		}, true},
		{"session seen 31 minutes ago", func(t *testing.T, d *DB) {
			browserSession(t, d, "u1", now.Add(-31*time.Minute))
		}, false},
		{"fresh session signed out", func(t *testing.T, d *DB) {
			token := browserSession(t, d, "u1", now.Add(-time.Minute))
			if err := d.RevokeWebSession(token); err != nil {
				t.Fatal(err)
			}
		}, false},
		{"fresh session expired", func(t *testing.T, d *DB) {
			token := browserSession(t, d, "u1", now.Add(-time.Minute))
			if _, err := d.conn.Exec(`UPDATE web_sessions SET expires_at = ? WHERE token_hash = ?`, now.Add(-time.Second), hashSecret(token)); err != nil {
				t.Fatal(err)
			}
		}, false},
		{"another person's fresh session", func(t *testing.T, d *DB) {
			if err := d.EnsureUser("u2"); err != nil {
				t.Fatal(err)
			}
			browserSession(t, d, "u2", now.Add(-time.Minute))
		}, false},
		{"agent connected to a live instance for hours", func(t *testing.T, d *DB) {
			serverInstance(t, d, "i1", now.Add(-5*time.Second))
			agentOn(t, d, "u1", "i1", long, time.Time{})
		}, true},
		{"agent disconnected 29 minutes ago", func(t *testing.T, d *DB) {
			serverInstance(t, d, "i1", now.Add(-5*time.Second))
			agentOn(t, d, "u1", "i1", long, now.Add(-29*time.Minute))
		}, true},
		{"agent disconnected 31 minutes ago", func(t *testing.T, d *DB) {
			serverInstance(t, d, "i1", now.Add(-5*time.Second))
			agentOn(t, d, "u1", "i1", long, now.Add(-31*time.Minute))
		}, false},
		{"agent on an instance dead for 20 minutes", func(t *testing.T, d *DB) {
			serverInstance(t, d, "i1", now.Add(-20*time.Minute))
			agentOn(t, d, "u1", "i1", long, time.Time{})
		}, true},
		{"agent on an instance dead for 31 minutes", func(t *testing.T, d *DB) {
			serverInstance(t, d, "i1", now.Add(-31*time.Minute))
			agentOn(t, d, "u1", "i1", long, time.Time{})
		}, false},
		{"agent of a reclaimed instance seen 20 minutes ago", func(t *testing.T, d *DB) {
			if _, err := d.conn.Exec(`UPDATE user_credential_unlocks SET agent_seen_at = ?`, now.Add(-20*time.Minute)); err != nil {
				t.Fatal(err)
			}
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := testDB(t)
			sealedAndUnlocked(t, d, "u1", long)
			tc.setup(t, d)
			if _, err := d.ForgetIdleUnlocks(now); err != nil {
				t.Fatal(err)
			}
			if kept := unlockRows(t, d, "u1") == 1; kept != tc.kept {
				t.Fatalf("kept = %v, want %v", kept, tc.kept)
			}
		})
	}
}

// An unlock made less than 30 minutes ago is never idle, whatever the sessions
// say: the person just typed the passphrase.
func TestARecentUnlockIsNeverIdle(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	d := testDB(t)
	sealedAndUnlocked(t, d, "u1", now.Add(-10*time.Minute))
	browserSession(t, d, "u1", now.Add(-2*time.Hour))
	if forgotten, err := d.ForgetIdleUnlocks(now); err != nil || forgotten != 0 {
		t.Fatalf("a recent unlock must be kept: %d %v", forgotten, err)
	}
	if forgotten, err := d.ForgetIdleUnlocks(now.Add(21 * time.Minute)); err != nil || forgotten != 1 {
		t.Fatalf("31 minutes after the unlock it must go: %d %v", forgotten, err)
	}
}

// Signing out forgets the unlocks at once when nothing else of the person is
// connected, whatever the time of the unlock (US5, FR-5).
func TestForgetUnlocksIfAbsent(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, d *DB)
		kept  bool
	}{
		{"sole session signed out", func(t *testing.T, d *DB) {}, false},
		{"another session seen 5 minutes ago", func(t *testing.T, d *DB) {
			browserSession(t, d, "u1", now.Add(-5*time.Minute))
		}, true},
		{"another session seen 40 minutes ago", func(t *testing.T, d *DB) {
			browserSession(t, d, "u1", now.Add(-40*time.Minute))
		}, false},
		{"agent connected", func(t *testing.T, d *DB) {
			serverInstance(t, d, "i1", now.Add(-5*time.Second))
			agentOn(t, d, "u1", "i1", now.Add(-time.Hour), time.Time{})
		}, true},
		{"agent disconnected a minute ago", func(t *testing.T, d *DB) {
			serverInstance(t, d, "i1", now.Add(-5*time.Second))
			agentOn(t, d, "u1", "i1", now.Add(-time.Hour), now.Add(-time.Minute))
		}, false},
		{"agent on a dead instance", func(t *testing.T, d *DB) {
			serverInstance(t, d, "i1", now.Add(-10*time.Minute))
			agentOn(t, d, "u1", "i1", now.Add(-time.Hour), time.Time{})
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := testDB(t)
			sealedAndUnlocked(t, d, "u1", now.Add(-time.Minute))
			token := browserSession(t, d, "u1", now.Add(-time.Second))
			tc.setup(t, d)
			if err := d.RevokeWebSession(token); err != nil {
				t.Fatal(err)
			}
			if err := d.ForgetUnlocksIfAbsent("u1", now); err != nil {
				t.Fatal(err)
			}
			if kept := unlockRows(t, d, "u1") == 1; kept != tc.kept {
				t.Fatalf("kept = %v, want %v", kept, tc.kept)
			}
		})
	}
}

// Blocking an account forgets its unlocks at once, whatever agent of theirs is
// connected (US6).
func TestBlockingAnAccountForgetsItsUnlocks(t *testing.T) {
	d := testDB(t)
	now := time.Now().UTC()
	if err := d.EnsureUser("admin"); err != nil {
		t.Fatal(err)
	}
	if err := d.setUserRole("admin", RoleAdmin); err != nil {
		t.Fatal(err)
	}
	sealedAndUnlocked(t, d, "u1", now)
	serverInstance(t, d, "i1", now)
	agentOn(t, d, "u1", "i1", now, time.Time{})
	if _, err := d.SetUserBlocked("u1", true); err != nil {
		t.Fatal(err)
	}
	if n := unlockRows(t, d, "u1"); n != 0 {
		t.Fatalf("a blocked account keeps no unlock, %d left", n)
	}
}

// When an instance dies holding a person's only agent, the agent's last sign
// of life outlives its agent_presence row: the unlock is kept 30 minutes from
// the instance's last heartbeat, not dropped with the row (US4).
func TestAReclaimedAgentStillCountsForTheIdleWindow(t *testing.T) {
	d := testDB(t)
	now := time.Now().UTC()
	sealedAndUnlocked(t, d, "u1", now.Add(-3*time.Hour))
	lastHeartbeat := now.Add(-10 * time.Minute)
	serverInstance(t, d, "dead", lastHeartbeat)
	agentOn(t, d, "u1", "dead", now.Add(-3*time.Hour), time.Time{})

	if _, err := d.reclaimDeadInstances(now); err != nil {
		t.Fatal(err)
	}
	var agents int
	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM agent_presence`).Scan(&agents); err != nil || agents != 0 {
		t.Fatalf("the reclaim must drop the agent row: %d %v", agents, err)
	}
	if forgotten, err := d.ForgetIdleUnlocks(now); err != nil || forgotten != 0 {
		t.Fatalf("the unlock must be kept 30 minutes after the last heartbeat: %d %v", forgotten, err)
	}
	if forgotten, err := d.ForgetIdleUnlocks(lastHeartbeat.Add(31 * time.Minute)); err != nil || forgotten != 1 {
		t.Fatalf("and forgotten after: %d %v", forgotten, err)
	}
}

// A single-process store restarting drops every agent row: an agent that was
// connected keeps its owner's unlock for the idle window, which is what lets
// it reconnect after a redeploy and still write under their name (US1, US4).
func TestARestartKeepsTheUnlockOfAPersonWithOnlyAnAgent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.db")
	first, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	sealedAndUnlocked(t, first, "u1", now.Add(-3*time.Hour))
	serverInstance(t, first, "old", now.Add(-5*time.Second))
	agentOn(t, first, "u1", "old", now.Add(-3*time.Hour), time.Time{})
	first.Close()

	restarted, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if forgotten, err := restarted.ForgetIdleUnlocks(time.Now().UTC()); err != nil || forgotten != 0 {
		t.Fatalf("the agent's presence must survive the restart: %d %v", forgotten, err)
	}
	if forgotten, err := restarted.ForgetIdleUnlocks(now.Add(31 * time.Minute)); err != nil || forgotten != 1 {
		t.Fatalf("and expire 30 minutes after it: %d %v", forgotten, err)
	}
}

// A server that was down longer than the idle window forgets the stale
// unlocks before it serves (FR-3).
func TestStartingAnInstanceSweepsFirst(t *testing.T) {
	d := testDB(t)
	sealedAndUnlocked(t, d, "u1", time.Now().Add(-time.Hour))
	stop, err := d.StartInstance()
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if n := unlockRows(t, d, "u1"); n != 0 {
		t.Fatalf("the stale unlock must be gone when StartInstance returns, %d left", n)
	}
}

// Deleting an account takes its unlocks with it.
func TestDeletingAnAccountForgetsItsUnlocks(t *testing.T) {
	d := testDB(t)
	if err := d.EnsureUser("admin"); err != nil {
		t.Fatal(err)
	}
	if err := d.setUserRole("admin", RoleAdmin); err != nil {
		t.Fatal(err)
	}
	sealedAndUnlocked(t, d, "u1", time.Now())
	if err := d.DeleteUser("u1"); err != nil {
		t.Fatal(err)
	}
	if n := unlockRows(t, d, "u1"); n != 0 {
		t.Fatalf("a deleted account keeps no unlock, %d left", n)
	}
}
