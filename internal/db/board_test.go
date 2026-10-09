package db

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/models"
	"tasks/internal/testsqlite"
	"tasks/internal/tracker"
	"tasks/internal/trackerapi"
)

// The palette and the automatic sync go through the tracker, never through its
// name. These tests pin that routing, and the merge the sync applies once the
// board answered. The palette is read on the tracker since its board is
// configured there (#741).

func TestTrackerStatusesComeFromABoardCapableTracker(t *testing.T) {
	fake := newFakeTracker()
	fake.statuses = []tracker.TrackerStatus{
		{ID: "1", Name: "To Do"},
		{ID: "2", Name: "In Progress"},
		{ID: "3", Name: "in progress"},
		{ID: "4", Name: "Done"},
	}

	database, project := jiraTestDB(t, fake)
	statuses, err := database.GetTrackerStatusesAs(context.Background(), project.DefaultTrackerID)
	if err != nil {
		t.Fatal(err)
	}
	// Tracker order, duplicates folded case-insensitively.
	if len(statuses) != 3 || statuses[0] != "To Do" || statuses[2] != "Done" {
		t.Fatalf("the palette must mirror the tracker statuses: %#v", statuses)
	}
}

func TestTrackerStatusesSurfaceTheTrackerFailure(t *testing.T) {
	fake := newFakeTracker()
	fake.statusErr = fmt.Errorf("gateway unavailable")

	database, project := jiraTestDB(t, fake)
	if _, err := database.GetTrackerStatusesAs(context.Background(), project.DefaultTrackerID); err == nil {
		t.Fatal("an unreadable status list must not be served as an empty palette")
	}
}

func TestTrackerStatusesAreEmptyWithoutBoards(t *testing.T) {
	fake := newFakeTracker()
	fake.Capabilities = []tracker.Capability{tracker.CapSync, tracker.CapGet}

	database, project := jiraTestDB(t, fake)
	statuses, err := database.GetTrackerStatusesAs(context.Background(), project.DefaultTrackerID)
	if err != nil {
		t.Fatalf("a tracker without boards names a limit, it does not fail: %v", err)
	}
	if len(statuses) != 0 {
		t.Fatalf("expected no status, got %#v", statuses)
	}
}

// A sync on a project that never chose a board resolves the first scrum board
// and retains it, so the picker and the next sync agree on the same one.
func TestSyncResolvesAndPersistsTheFirstScrumBoard(t *testing.T) {
	fake := newFakeTracker()
	fake.boards = []models.TrackerBoard{{ID: "7", Name: "PE kanban", Type: "kanban"}, {ID: "5", Name: "PE scrum", Type: "scrum"}}
	fake.columns = []models.TrackerColumn{{Name: "To Do", Statuses: []string{"To Do"}}}

	database, project := jiraTestDB(t, fake)
	if _, err := database.SyncProjectBoardColumns(context.Background(), project.ID); err != nil {
		t.Fatal(err)
	}
	refreshed, err := database.GetProjectByID(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.BoardID != "5" {
		t.Fatalf("the resolved board must be retained, got %q", refreshed.BoardID)
	}
}

// The merge is the contract shared by "Detect" and the sync: the tracker owns
// the names and the order, the user keeps what the tracker claims nowhere.
func TestSyncBoardColumnsMergeKeepsTheUsersWork(t *testing.T) {
	fake := newFakeTracker()
	fake.boards = []models.TrackerBoard{{ID: "5", Name: "PE scrum", Type: "scrum"}}
	fake.columns = []models.TrackerColumn{
		{Name: "To Do", Statuses: []string{"To Do"}},
		{Name: "Done", Statuses: []string{"Done"}},
	}

	database, project := jiraTestDB(t, fake)
	boardID := "5"
	columns := []models.TrackerColumn{
		{Name: "To Do", Statuses: []string{"To Do", "In Review"}, Hidden: true},
		{Name: "Peer review", Statuses: []string{"Waiting"}},
		{Name: "Archive", Statuses: []string{"Done"}},
	}
	stages := map[string][]string{"specified": {"To Do"}, "reviewed": {"Archive"}}
	if _, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{
		BoardID: &boardID, TrackerColumns: &columns, StageColumns: &stages,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := database.SyncProjectBoardColumns(context.Background(), project.ID); err != nil {
		t.Fatal(err)
	}
	refreshed, err := database.GetProjectByID(project.ID)
	if err != nil {
		t.Fatal(err)
	}

	got := refreshed.TrackerColumns
	if len(got) != 3 {
		t.Fatalf("expected the two board columns plus the user one: %+v", got)
	}
	// The board gives names and order; the hidden flag and the hand-assigned
	// status the board claims nowhere survive.
	if got[0].Name != "To Do" || !got[0].Hidden || len(got[0].Statuses) != 2 || got[0].Statuses[1] != "In Review" {
		t.Fatalf("hand-assigned status or hidden flag lost: %+v", got[0])
	}
	// "Archive" only held a status the board claims: it is dropped.
	if got[2].Name != "Peer review" {
		t.Fatalf("a user column emptied by the board must be dropped: %+v", got)
	}
	// The stage mapped to the vanished column goes with it, the other stays:
	// the mapping is the project's own (#741), pruned with the tracker's.
	if len(refreshed.StageColumns["specified"]) != 1 || len(refreshed.StageColumns["reviewed"]) != 0 {
		t.Fatalf("stage mappings not pruned as expected: %+v", refreshed.StageColumns)
	}
}

// A GitHub tracker configured on the admin screen offers the single-select
// options of its repository's ProjectsV2 boards, as a GitHub project did
// before its tracker settings moved to the tracker (#741).
func TestAGithubTrackerStatusListReadsItsProjectsV2Options(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "tasks.db"), NewDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	var asked string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graphql" {
			t.Errorf("unexpected call: %s", r.URL)
		}
		var req struct{ Query string }
		_ = json.NewDecoder(r.Body).Decode(&req)
		asked = req.Query
		fmt.Fprint(w, `{"data":{"repository":{"projectsV2":{"nodes":[{"fields":{"nodes":[{"name":"Status","options":[{"name":"Todo"},{"name":"In Progress"},{"name":"Done"}]}]}}]}},"user":{"projectsV2":{"nodes":[{"fields":{"nodes":[{"name":"Status","options":[{"name":"todo"},{"name":"Blocked"}]}]}}]}}}}`)
	}))
	defer server.Close()
	database.trackers = &trackerapi.Client{GithubURL: server.URL, GithubToken: "test", HTTP: server.Client()}

	trk, err := database.CreateTrackerAs("admin", models.Tracker{Provider: "github", Scope: "acme/app"})
	if err != nil {
		t.Fatal(err)
	}
	statuses, err := database.GetTrackerStatusesAs(context.Background(), trk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(asked, "acme") || !strings.Contains(asked, "app") {
		t.Fatalf("the lookup must name the tracker's repository: %s", asked)
	}
	// Repository then user boards, duplicates folded case-insensitively.
	want := []string{"Todo", "In Progress", "Done", "Blocked"}
	if strings.Join(statuses, "|") != strings.Join(want, "|") {
		t.Fatalf("the palette must hold the ProjectsV2 options: %#v", statuses)
	}
}

// Without any ProjectsV2 option, a GitHub tracker keeps the two states an
// issue always has.
func TestAGithubTrackerStatusListFallsBackToOpenAndClosed(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "tasks.db"), NewDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"repository":{"projectsV2":{"nodes":[]}},"user":{"projectsV2":{"nodes":[]}}}}`)
	}))
	defer server.Close()
	database.trackers = &trackerapi.Client{GithubURL: server.URL, GithubToken: "test", HTTP: server.Client()}

	trk, err := database.CreateTrackerAs("admin", models.Tracker{Provider: "github", Scope: "acme/app"})
	if err != nil {
		t.Fatal(err)
	}
	statuses, err := database.GetTrackerStatusesAs(context.Background(), trk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(statuses, "|") != "open|closed" {
		t.Fatalf("the GitHub fallback must be kept: %#v", statuses)
	}
}
