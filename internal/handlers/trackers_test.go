package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tasks/internal/models"
)

// trackerServer serves the tracker routes behind the guard main.go installs.
func trackerServer(t *testing.T, h *Handler) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(AdminTrackersPath, h.HandleAdminTrackers)
	mux.HandleFunc(AdminTrackersPath+"/", h.HandleAdminTrackers)
	mux.HandleFunc(TrackersPath, h.HandleTrackers)
	mux.HandleFunc(TrackersPath+"/", h.HandleTrackers)
	server := httptest.NewServer(h.EnableCORS(h.RequireSession(mux)))
	t.Cleanup(server.Close)
	return server
}

func TestOnlyAnAdminCanCreateOrEditATracker(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := trackerServer(t, h)
	_, alice := account(t, database, "alice@example.com") // the first account is the admin
	_, bob := account(t, database, "bob@example.com")

	status, body := call(t, server, alice, http.MethodPost, AdminTrackersPath, `{"provider":"jira","scope":"gode","name":"GODE"}`)
	if status != http.StatusCreated {
		t.Fatalf("admin create: %d %s", status, body)
	}
	var created models.Tracker
	if err := json.Unmarshal([]byte(body), &created); err != nil || created.Scope != "GODE" {
		t.Fatalf("created = %+v (%v)", created, err)
	}

	for _, route := range []struct{ method, path, body string }{
		{http.MethodGet, AdminTrackersPath, ""},
		{http.MethodPost, AdminTrackersPath, `{"provider":"jira","scope":"BE"}`},
		{http.MethodGet, AdminTrackersPath + "/" + created.ID, ""},
		{http.MethodPut, AdminTrackersPath + "/" + created.ID, `{"provider":"jira","scope":"GODE","name":"Renamed"}`},
		{http.MethodDelete, AdminTrackersPath + "/" + created.ID, ""},
		{http.MethodGet, AdminTrackersPath + "/" + created.ID + "/boards", ""},
	} {
		if status, body := call(t, server, nil, route.method, route.path, route.body); status != http.StatusUnauthorized {
			t.Errorf("anonymous %s %s: %d %s", route.method, route.path, status, body)
		}
		if status, body := call(t, server, bob, route.method, route.path, route.body); status != http.StatusForbidden {
			t.Errorf("member %s %s: %d %s", route.method, route.path, status, body)
		}
	}
	if !adminOnlyRoute(http.MethodGet, AdminTrackersPath) || !adminOnlyRoute(http.MethodPut, AdminTrackersPath+"/x") {
		t.Fatal("the admin tracker routes are missing from the admin-only table")
	}
	if adminOnlyRoute(http.MethodGet, TrackersPath) {
		t.Fatal("a member must be able to list the trackers")
	}

	if status, body := call(t, server, alice, http.MethodPut, AdminTrackersPath+"/"+created.ID, `{"provider":"jira","scope":"GODE","name":"Renamed"}`); status != http.StatusOK || !strings.Contains(body, "Renamed") {
		t.Fatalf("admin update: %d %s", status, body)
	}
	if status, body := call(t, server, alice, http.MethodDelete, AdminTrackersPath+"/"+created.ID, ""); status != http.StatusOK {
		t.Fatalf("admin delete of an unused tracker: %d %s", status, body)
	}
}

func TestAMemberCanListTrackersToPickFrom(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := trackerServer(t, h)
	_, _ = account(t, database, "alice@example.com")
	_, bob := account(t, database, "bob@example.com")
	gode, err := database.CreateTrackerAs("admin", models.Tracker{Provider: "jira", Scope: "GODE"})
	if err != nil {
		t.Fatal(err)
	}

	status, body := call(t, server, bob, http.MethodGet, TrackersPath, "")
	if status != http.StatusOK {
		t.Fatalf("member list: %d %s", status, body)
	}
	var listed []map[string]any
	if err := json.Unmarshal([]byte(body), &listed); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range listed {
		if entry["provider"] == "local" {
			t.Fatalf("a project's local board is listed: %v", entry)
		}
		found = found || entry["id"] == gode.ID
		if _, leaks := entry["trackerColumns"]; leaks {
			t.Fatalf("the member list carries the board mirror: %v", entry)
		}
	}
	if !found {
		t.Fatalf("GODE is not listed: %s", body)
	}
}

func TestAMemberWithoutAProjectOnTheTrackerCannotReadItsBacklog(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := trackerServer(t, h)
	_, alice := account(t, database, "alice@example.com")
	_, bob := account(t, database, "bob@example.com")
	lone, err := database.CreateTrackerAs("admin", models.Tracker{Provider: "jira", Scope: "LONE"})
	if err != nil {
		t.Fatal(err)
	}
	backlog := TrackersPath + "/" + lone.ID + "/backlog"

	if status, body := call(t, server, bob, http.MethodGet, backlog, ""); status != http.StatusForbidden {
		t.Fatalf("member, no project on the tracker: %d %s", status, body)
	}
	if status, body := call(t, server, alice, http.MethodGet, backlog, ""); status != http.StatusOK {
		t.Fatalf("admin: %d %s", status, body)
	}
	if _, err := database.CreateProject(models.CreateProjectRequest{Name: "Lone", Label: "lone", Trackers: []models.ProjectTracker{{TrackerID: lone.ID}}}); err != nil {
		t.Fatal(err)
	}
	if status, body := call(t, server, bob, http.MethodGet, backlog, ""); status != http.StatusOK {
		t.Fatalf("member once a project selects the tracker: %d %s", status, body)
	}
}
