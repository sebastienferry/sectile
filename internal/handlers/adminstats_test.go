package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// The admin page counts the people using the board and the runs not over, and
// only an admin reads it.
func TestAdminStatsCountActiveUsersAndRuns(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := guardedServer(t, h)

	aliceID, alice := account(t, database, "alice@example.com")
	bobID, bob := account(t, database, "bob@example.com")
	_, _ = account(t, database, "carol@example.com") // signed in once, never used since

	if status, body := call(t, server, bob, http.MethodGet, AdminStatsPath, ""); status != http.StatusForbidden || !strings.Contains(body, msgAdminOnly) {
		t.Fatalf("a member read the admin stats: %d %s", status, body)
	}

	status, body := call(t, server, alice, http.MethodGet, AdminStatsPath, "")
	if status != http.StatusOK {
		t.Fatalf("admin stats: %d %s", status, body)
	}
	var stats struct {
		Users struct {
			Total, Admins, Blocked, Active int
		}
		Runs struct {
			Active   int
			ByStatus map[string]int
		}
		ActiveWindowSeconds int
	}
	if err := json.Unmarshal([]byte(body), &stats); err != nil {
		t.Fatalf("decoding %s: %v", body, err)
	}
	// Alice and Bob both reached the server through their session; Carol's
	// session was never used, so it holds a cookie and nothing more.
	if stats.Users.Active != 2 {
		t.Errorf("active users = %d, want 2 (%s)", stats.Users.Active, body)
	}
	if stats.Users.Total < 3 || stats.Users.Admins < 1 {
		t.Errorf("roster counts = %+v, want at least three accounts and one admin", stats.Users)
	}
	if stats.Runs.Active != 0 || stats.Runs.ByStatus["running"] != 0 {
		t.Errorf("runs = %+v on an empty board", stats.Runs)
	}
	if _, ok := stats.Runs.ByStatus["queued"]; !ok {
		t.Errorf("every active status is reported, at zero when empty: %+v", stats.Runs.ByStatus)
	}
	if stats.ActiveWindowSeconds <= 0 {
		t.Errorf("the window the count is read over is not reported")
	}

	// The users view says who is active, row by row.
	status, body = call(t, server, alice, http.MethodGet, "/api/users", "")
	if status != http.StatusOK {
		t.Fatalf("users: %d %s", status, body)
	}
	var roster struct {
		Users []struct {
			ID           string  `json:"id"`
			Active       bool    `json:"active"`
			LastActiveAt *string `json:"lastActiveAt"`
		} `json:"users"`
	}
	if err := json.Unmarshal([]byte(body), &roster); err != nil {
		t.Fatalf("decoding %s: %v", body, err)
	}
	seen := map[string]bool{}
	for _, user := range roster.Users {
		if user.Active && user.LastActiveAt == nil {
			t.Errorf("%s is active with no last activity", user.ID)
		}
		seen[user.ID] = user.Active
	}
	if !seen[aliceID] || !seen[bobID] {
		t.Errorf("alice and bob should read as active: %s", body)
	}
}
