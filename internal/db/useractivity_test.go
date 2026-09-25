package db

import (
	"testing"
	"time"
)

func TestAUsedSessionCountsAsActive(t *testing.T) {
	database := identityDB(t)
	alice, _ := database.UpsertUser("https://issuer.example|alice", "alice@example.com", "Alice")
	bob, _ := database.UpsertUser("https://issuer.example|bob", "bob@example.com", "Bob")
	aliceToken, _, _ := database.CreateWebSession(alice)
	_, _, _ = database.CreateWebSession(bob)

	if count, err := database.ActiveUserCount(ActiveUserWindow); err != nil || count != 0 {
		t.Fatalf("before any use: %d (%v), want 0: holding a cookie is not being active", count, err)
	}
	database.UserForWebSession(aliceToken)
	if count, err := database.ActiveUserCount(ActiveUserWindow); err != nil || count != 1 {
		t.Fatalf("after alice's request: %d (%v), want 1", count, err)
	}
	activity, err := database.UserActivity()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := activity[alice]; !ok {
		t.Fatalf("alice's activity is missing: %v", activity)
	}
	if _, ok := activity[bob]; ok {
		t.Fatalf("bob never used his session but has an activity: %v", activity)
	}
}

func TestAStaleOrRevokedSessionIsNotActive(t *testing.T) {
	database := identityDB(t)
	alice, _ := database.UpsertUser("https://issuer.example|alice", "alice@example.com", "Alice")
	bob, _ := database.UpsertUser("https://issuer.example|bob", "bob@example.com", "Bob")
	aliceToken, _, _ := database.CreateWebSession(alice)
	bobToken, _, _ := database.CreateWebSession(bob)
	database.UserForWebSession(aliceToken)
	database.UserForWebSession(bobToken)

	if _, err := database.conn.Exec(`UPDATE web_sessions SET last_seen_at = ? WHERE user_id = ?`,
		time.Now().UTC().Add(-2*ActiveUserWindow), alice); err != nil {
		t.Fatal(err)
	}
	if err := database.RevokeWebSession(bobToken); err != nil {
		t.Fatal(err)
	}
	if count, err := database.ActiveUserCount(ActiveUserWindow); err != nil || count != 0 {
		t.Fatalf("stale and revoked sessions counted: %d (%v)", count, err)
	}
}

// The mark is written at most once per interval, not on every read.
func TestTheSessionMarkIsThrottled(t *testing.T) {
	database := identityDB(t)
	alice, _ := database.UpsertUser("https://issuer.example|alice", "alice@example.com", "Alice")
	token, _, _ := database.CreateWebSession(alice)
	database.UserForWebSession(token)
	first, _ := database.UserActivity()
	database.UserForWebSession(token)
	second, _ := database.UserActivity()
	if !first[alice].Equal(second[alice]) {
		t.Fatalf("a second read within the interval rewrote the mark: %v then %v", first[alice], second[alice])
	}
}

func TestActiveRunCountsReportEveryActiveStatus(t *testing.T) {
	database := identityDB(t)
	counts, err := database.ActiveRunCounts()
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range activeRunStatuses {
		if value, ok := counts[status]; !ok || value != 0 {
			t.Errorf("status %s = %d (present %v), want a zero on an empty board", status, value, ok)
		}
	}
}

func TestCountUsers(t *testing.T) {
	database := identityDB(t)
	alice, _ := database.UpsertUser("https://issuer.example|alice", "alice@example.com", "Alice")
	_, _ = database.UpsertUser("https://issuer.example|bob", "bob@example.com", "Bob")
	if _, err := database.SetUserRole(alice, RoleAdmin); err != nil {
		t.Fatal(err)
	}
	counts, err := database.CountUsers()
	if err != nil {
		t.Fatal(err)
	}
	if counts.Total < 2 || counts.Admins < 1 || counts.Blocked != 0 {
		t.Fatalf("counts = %+v", counts)
	}
}
