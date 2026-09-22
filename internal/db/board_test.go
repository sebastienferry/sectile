package db

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// The palette, the detection and the automatic sync all go through the tracker
// the project resolves to, never through its name. These tests pin that
// routing, and the merge the sync applies once the board answered.

func TestProjectStatusesComeFromABoardCapableTracker(t *testing.T) {
	fake := newFakeTracker()
	fake.statuses = []tracker.TrackerStatus{
		{ID: "1", Name: "To Do"},
		{ID: "2", Name: "In Progress"},
		{ID: "3", Name: "in progress"},
		{ID: "4", Name: "Done"},
	}

	database, project := jiraTestDB(t, fake)
	statuses, err := database.GetProjectTrackerStatuses(context.Background(), project.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Tracker order, duplicates folded case-insensitively.
	if len(statuses) != 3 || statuses[0] != "To Do" || statuses[2] != "Done" {
		t.Fatalf("the palette must mirror the tracker statuses: %#v", statuses)
	}
}

func TestProjectStatusesSurfaceTheTrackerFailure(t *testing.T) {
	fake := newFakeTracker()
	fake.statusErr = fmt.Errorf("gateway unavailable")

	database, project := jiraTestDB(t, fake)
	if _, err := database.GetProjectTrackerStatuses(context.Background(), project.ID); err == nil {
		t.Fatal("an unreadable status list must not be served as an empty palette")
	}
}

func TestProjectStatusesAreEmptyWithoutBoards(t *testing.T) {
	fake := newFakeTracker()
	fake.Capabilities = []tracker.Capability{tracker.CapSync, tracker.CapGet}

	database, project := jiraTestDB(t, fake)
	statuses, err := database.GetProjectTrackerStatuses(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("a tracker without boards names a limit, it does not fail: %v", err)
	}
	if len(statuses) != 0 {
		t.Fatalf("expected no status, got %#v", statuses)
	}
}

// GitHub keeps the path it had, fallback included: nothing of its observable
// behaviour changes with the capability dispatch.
func TestGithubProjectStatusesKeepTheirFallback(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Sectile", IssueTracker: "github", GithubRepo: ""})
	if err != nil {
		t.Fatal(err)
	}
	statuses, err := database.GetProjectTrackerStatuses(context.Background(), project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 2 || statuses[0] != "open" || statuses[1] != "closed" {
		t.Fatalf("the GitHub fallback must be untouched: %#v", statuses)
	}
}

func TestDetectBoardColumnsMirrorsTheBoard(t *testing.T) {
	fake := newFakeTracker()
	fake.boards = []models.TrackerBoard{{ID: "7", Name: "PE kanban", Type: "kanban"}, {ID: "5", Name: "PE scrum", Type: "scrum"}}
	fake.columns = []models.TrackerColumn{
		{Name: "To Do", Statuses: []string{"To Do", "Backlog"}},
		{Name: "Done", Statuses: []string{"Done"}},
	}

	database, project := jiraTestDB(t, fake)
	columns, err := database.DetectProjectBoardColumns(context.Background(), project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(columns) != 2 || columns[0].Name != "To Do" || len(columns[0].Statuses) != 2 {
		t.Fatalf("detection must return the board columns with their statuses: %+v", columns)
	}
	// Reading columns writes nothing: the board is only retained on a sync.
	refreshed, _ := database.GetProjectByID(project.ID)
	if refreshed.BoardID != "" {
		t.Fatalf("detection must not persist a board, got %q", refreshed.BoardID)
	}
}

// A board that groups no column is reported: the editor must keep the columns
// the user already has rather than fall back to one column per status.
func TestDetectBoardColumnsReportsAnEmptyBoard(t *testing.T) {
	fake := newFakeTracker()
	fake.boards = []models.TrackerBoard{{ID: "5", Name: "PE scrum", Type: "scrum"}}
	fake.columns = nil

	database, project := jiraTestDB(t, fake)
	if _, err := database.DetectProjectBoardColumns(context.Background(), project.ID); err == nil {
		t.Fatal("a board without column must be reported, not served as an empty mirror")
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
	// The stage mapped to the vanished column goes with it, the other stays.
	if len(refreshed.StageColumns["specified"]) != 1 || len(refreshed.StageColumns["reviewed"]) != 0 {
		t.Fatalf("stage mappings not pruned as expected: %+v", refreshed.StageColumns)
	}
}
