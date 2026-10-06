package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tasks/internal/models"
	"tasks/internal/tracker"
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

// boardFake stands in for a tracker that has boards, for the configuration
// routes an admin uses on a tracker.
type boardFake struct {
	tracker.BaseTicketingSystem
}

func (b *boardFake) ListBoards(ctx context.Context, req tracker.BoardsRequest) ([]models.TrackerBoard, error) {
	return []models.TrackerBoard{{ID: "5", Name: "PE", Type: "scrum"}}, nil
}

func (b *boardFake) ListBoardColumns(ctx context.Context, req tracker.BoardRequest) ([]models.TrackerColumn, error) {
	return []models.TrackerColumn{
		{Name: "To Do", Statuses: []string{"To Do", "Backlog"}},
		{Name: "Done", Statuses: []string{"Done"}},
	}, nil
}

func (b *boardFake) ListStatuses(ctx context.Context, req tracker.ProjectRequest) ([]tracker.TrackerStatus, error) {
	return []tracker.TrackerStatus{{ID: "1", Name: "To Do"}, {ID: "2", Name: "In Review"}}, nil
}

// The board, its columns and the status palette are configured on the
// tracker, by an admin (#741, D11): the routes that did it through a project
// are gone, and the tracker's own answer what they answered.
func TestAnAdminConfiguresTheBoardOnTheTrackerNotOnAProject(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	database.TrackerRegistry().Register("jira", &boardFake{tracker.BaseTicketingSystem{
		TrackerName:  "jira",
		Capabilities: []tracker.Capability{tracker.CapBoard},
	}})
	server := trackerServer(t, h)
	_, alice := account(t, database, "alice@example.com")
	pe, err := database.CreateTrackerAs("admin", models.Tracker{Provider: "jira", Scope: "PE"})
	if err != nil {
		t.Fatal(err)
	}
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Platform", Trackers: []models.ProjectTracker{{TrackerID: pe.ID}}})
	if err != nil {
		t.Fatal(err)
	}

	status, body := call(t, server, alice, http.MethodGet, AdminTrackersPath+"/"+pe.ID+"/boards", "")
	if status != http.StatusOK || !strings.Contains(body, `"id":"5"`) {
		t.Fatalf("boards: %d %s", status, body)
	}
	status, body = call(t, server, alice, http.MethodPost, AdminTrackersPath+"/"+pe.ID+"/board-columns", `{"boardId":"5"}`)
	if status != http.StatusOK {
		t.Fatalf("board import: %d %s", status, body)
	}
	var imported models.Tracker
	if err := json.Unmarshal([]byte(body), &imported); err != nil {
		t.Fatal(err)
	}
	if imported.BoardID != "5" || len(imported.TrackerColumns) != 2 || len(imported.TrackerColumns[0].Statuses) != 2 {
		t.Fatalf("the board columns must land on the tracker: %+v", imported)
	}
	status, body = call(t, server, alice, http.MethodGet, AdminTrackersPath+"/"+pe.ID+"/tracker-statuses", "")
	if status != http.StatusOK || !strings.Contains(body, "In Review") {
		t.Fatalf("statuses: %d %s", status, body)
	}

	// The project reads the mirror through from its tracker.
	read, err := database.GetProjectByID(project.ID)
	if err != nil || read == nil || len(read.TrackerColumns) != 2 {
		t.Fatalf("the project must read the tracker's columns: %+v (%v)", read, err)
	}

	// The project-level aliases no longer configure anything.
	for _, path := range []string{"/api/projects/" + project.ID + "/boards", "/api/projects/" + project.ID + "/tracker-statuses", "/api/projects/" + project.ID + "/issue-types"} {
		rr := httptest.NewRecorder()
		h.HandleProjectDetail(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if strings.Contains(rr.Body.String(), "In Review") || strings.Contains(rr.Body.String(), `"name":"PE"`) {
			t.Fatalf("%s still answers the tracker configuration: %s", path, rr.Body.String())
		}
	}
	rr := httptest.NewRecorder()
	h.HandleProjectDetail(rr, httptest.NewRequest(http.MethodGet, "/api/projects/detected-statuses?projectId="+project.ID, nil))
	if strings.Contains(rr.Body.String(), `"columns"`) {
		t.Fatalf("the project detection route still answers: %s", rr.Body.String())
	}
}

// projectServer serves the project routes behind the guard main.go installs.
func projectServer(t *testing.T, h *Handler) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/projects", h.HandleProjects)
	mux.HandleFunc("/api/projects/", h.HandleProjectDetail)
	server := httptest.NewServer(h.EnableCORS(h.RequireSession(mux)))
	t.Cleanup(server.Close)
	return server
}

// trackerConfiguration is a project payload carrying every tracker setting an
// older client still sends with the project.
const trackerConfiguration = `"boardId":"9","trackerColumns":[{"name":"Doing","statuses":["In Progress"]}],` +
	`"stageColumns":{"implemented":["Doing"]},"issueTypes":["Bug"],"autoSyncEnabled":true,"autoSyncIntervalMin":2`

// A tracker is configured by an admin (#741, D11): a member's project save
// that still carries the board, the mapping or the background sync leaves the
// tracker as it was, and the rest of the save goes through.
func TestAMemberCannotConfigureATrackerThroughAProject(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := projectServer(t, h)
	_, _ = account(t, database, "alice@example.com") // the first account is the admin
	_, bob := account(t, database, "bob@example.com")
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Platform", IssueTracker: "jira", JiraProject: "PE"})
	if err != nil {
		t.Fatal(err)
	}
	before, err := database.GetTrackerByID(project.DefaultTrackerID)
	if err != nil || before == nil {
		t.Fatalf("tracker: %+v (%v)", before, err)
	}

	status, body := call(t, server, bob, http.MethodPut, "/api/projects/"+project.ID, `{"description":"Renamed by a member",`+trackerConfiguration+`}`)
	if status != http.StatusOK || !strings.Contains(body, "Renamed by a member") {
		t.Fatalf("member save: %d %s", status, body)
	}
	after, err := database.GetTrackerByID(project.DefaultTrackerID)
	if err != nil || after == nil {
		t.Fatalf("tracker: %+v (%v)", after, err)
	}
	if after.BoardID != before.BoardID || len(after.TrackerColumns) != len(before.TrackerColumns) || len(after.StageColumns) != len(before.StageColumns) ||
		len(after.IssueTypes) != len(before.IssueTypes) || after.AutoSyncEnabled != before.AutoSyncEnabled || after.AutoSyncIntervalMin != before.AutoSyncIntervalMin {
		t.Fatalf("a member configured the tracker through a project:\nbefore %+v\nafter  %+v", before, after)
	}

	status, body = call(t, server, bob, http.MethodPost, "/api/projects", `{"name":"Billing","issueTracker":"jira","jiraProject":"BILL",`+trackerConfiguration+`}`)
	if status != http.StatusCreated {
		t.Fatalf("member create: %d %s", status, body)
	}
	var created models.Project
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatal(err)
	}
	fresh, err := database.GetTrackerByID(created.DefaultTrackerID)
	if err != nil || fresh == nil || fresh.Scope != "BILL" {
		t.Fatalf("the member's project must still get its tracker: %+v (%v)", fresh, err)
	}
	if fresh.BoardID != "" || len(fresh.TrackerColumns) != 0 || len(fresh.StageColumns) != 0 || len(fresh.IssueTypes) != 0 || fresh.AutoSyncEnabled {
		t.Fatalf("a member configured a new tracker through a project: %+v", fresh)
	}
}

// An admin's project save still writes the tracker configuration through to
// the project's default tracker, for the clients that send it.
func TestAnAdminStillConfiguresTheTrackerThroughAProject(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := projectServer(t, h)
	_, alice := account(t, database, "alice@example.com")
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Platform", IssueTracker: "jira", JiraProject: "PE"})
	if err != nil {
		t.Fatal(err)
	}

	status, body := call(t, server, alice, http.MethodPut, "/api/projects/"+project.ID, `{`+trackerConfiguration+`}`)
	if status != http.StatusOK {
		t.Fatalf("admin save: %d %s", status, body)
	}
	after, err := database.GetTrackerByID(project.DefaultTrackerID)
	if err != nil || after == nil {
		t.Fatalf("tracker: %+v (%v)", after, err)
	}
	if after.BoardID != "9" || len(after.TrackerColumns) != 1 || len(after.StageColumns["implemented"]) != 1 || !after.AutoSyncEnabled || after.AutoSyncIntervalMin != 2 {
		t.Fatalf("the admin's configuration must reach the tracker: %+v", after)
	}

	status, body = call(t, server, alice, http.MethodPost, "/api/projects", `{"name":"Billing","issueTracker":"jira","jiraProject":"BILL",`+trackerConfiguration+`}`)
	if status != http.StatusCreated {
		t.Fatalf("admin create: %d %s", status, body)
	}
	var created models.Project
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatal(err)
	}
	fresh, err := database.GetTrackerByID(created.DefaultTrackerID)
	if err != nil || fresh == nil || fresh.BoardID != "9" || len(fresh.IssueTypes) != 1 || !fresh.AutoSyncEnabled {
		t.Fatalf("the admin's new tracker must carry the configuration: %+v (%v)", fresh, err)
	}
}
