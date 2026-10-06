package db

import (
	"testing"
	"time"

	"tasks/internal/models"
)

// saveEpic records a Jira epic of a project as the roadmap import does: its
// title and labels.
func saveEpic(t *testing.T, d *DB, projectID, key string, labels ...string) {
	t.Helper()
	title := "Epic " + key
	if labels == nil {
		labels = []string{}
	}
	if _, err := d.saveMacroMetaFull(projectID, key, nil, nil, nil, nil, &title, nil, nil, &labels); err != nil {
		t.Fatal(err)
	}
}

// macroKeys lists the keys of the macros a project shows.
func macroKeys(t *testing.T, d *DB, projectID string) map[string]bool {
	t.Helper()
	macros, err := d.GetProjectMacros(projectID)
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]bool{}
	for _, m := range macros {
		if m.ProjectID != projectID {
			t.Fatalf("macro %s shows the project %q, want %q", m.Key, m.ProjectID, projectID)
		}
		keys[m.Key] = true
	}
	return keys
}

func TestAnEpicShowsInAProjectThroughItsLabel(t *testing.T) {
	d := testDB(t)
	delivery := spaceProject(t, d, "Delivery", "delivery-admin")
	bidder := spaceProject(t, d, "Bidder", "bidderAdmin")
	saveEpic(t, d, delivery.ID, "GODE-100", "Delivery-Admin")

	if !macroKeys(t, d, delivery.ID)["GODE-100"] {
		t.Fatal("the epic carrying the project label is missing")
	}
	if macroKeys(t, d, bidder.ID)["GODE-100"] {
		t.Fatal("an epic of another project's label shows in Bidder")
	}
}

func TestAnEpicShowsInAProjectThroughAMemberChild(t *testing.T) {
	d := testDB(t)
	delivery := spaceProject(t, d, "Delivery", "delivery-admin")
	bidder := spaceProject(t, d, "Bidder", "bidderAdmin")
	saveEpic(t, d, delivery.ID, "GODE-200")
	if err := d.ImportOrUpdateTasks(delivery.DefaultTrackerID, []models.Task{{Key: "GODE-1", Title: "Child", Status: models.StatusToClarify, Priority: models.PriorityMedium,
		Labels: []string{"bidderadmin"}, ParentKey: "GODE-200", Source: "jira", CreatedAt: time.Now(), UpdatedAt: time.Now()}}); err != nil {
		t.Fatal(err)
	}

	if !macroKeys(t, d, bidder.ID)["GODE-200"] {
		t.Fatal("the epic of a Bidder ticket is missing from Bidder")
	}
	if macroKeys(t, d, delivery.ID)["GODE-200"] {
		t.Fatal("the epic shows in Delivery, which holds none of its tickets")
	}
}

func TestAnEpicWithNoMemberChildAndNoLabelStaysOut(t *testing.T) {
	d := testDB(t)
	delivery := spaceProject(t, d, "Delivery", "delivery-admin")
	everything := spaceProject(t, d, "Everything", "")
	saveEpic(t, d, delivery.ID, "GODE-300", "ops")

	if macroKeys(t, d, delivery.ID)["GODE-300"] {
		t.Fatal("an epic with neither the label nor a member child shows in the labelled project")
	}
	if !macroKeys(t, d, everything.ID)["GODE-300"] {
		t.Fatal("an unlabelled project shows every epic of its trackers")
	}
	facets, err := d.GetTaskFacets(delivery.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range facets.Macros {
		if m.Key == "GODE-300" {
			t.Fatal("the board's macro filter offers an epic the project does not show")
		}
	}
}

func TestTwoProjectsOnOneSpaceShareOneEpicRecord(t *testing.T) {
	d := testDB(t)
	first := spaceProject(t, d, "Delivery", "")
	second := spaceProject(t, d, "Bidder", "")
	now := HorizonNow
	if _, err := d.saveMacroMetaFull(first.ID, "GODE-5", &now, nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	later := HorizonLater
	if _, err := d.saveMacroMetaFull(second.ID, "GODE-5", &later, nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	var rows int
	var projectID, trackerID string
	if err := d.conn.QueryRow("SELECT COUNT(*), MIN(project_id), MIN(tracker_id) FROM macros WHERE key = 'GODE-5'").Scan(&rows, &projectID, &trackerID); err != nil {
		t.Fatal(err)
	}
	if rows != 1 || trackerID != first.DefaultTrackerID || projectID != trackerSentinel(first.DefaultTrackerID) {
		t.Fatalf("GODE-5 rows = %d (project_id %q, tracker %q), want one tracker record", rows, projectID, trackerID)
	}
	for _, p := range []*models.Project{first, second} {
		macros, err := d.GetProjectMacros(p.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(macros) != 1 || macros[0].Horizon != HorizonLater {
			t.Fatalf("%s reads %+v, want the one record with the last horizon", p.Name, macros)
		}
	}
}

func TestLocalMilestonesStayOwnedByTheirProject(t *testing.T) {
	d := testDB(t)
	first, err := d.CreateProject(models.CreateProjectRequest{Name: "Alpha"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := d.CreateProject(models.CreateProjectRequest{Name: "Beta"})
	if err != nil {
		t.Fatal(err)
	}
	h := HorizonNow
	if _, err := d.SaveMacroMeta(first.ID, "M-1", &h, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	var projectID string
	var trackerID *string
	if err := d.conn.QueryRow("SELECT project_id, tracker_id FROM macros WHERE key = 'M-1'").Scan(&projectID, &trackerID); err != nil {
		t.Fatal(err)
	}
	if projectID != first.ID || trackerID != nil {
		t.Fatalf("M-1 is stored under %q, tracker %v; want Alpha's own row", projectID, trackerID)
	}
	if !macroKeys(t, d, first.ID)["M-1"] || macroKeys(t, d, second.ID)["M-1"] {
		t.Fatal("a local milestone must show in its project only")
	}
}

func TestEpicAdoptionKeepsTheTodosOfTheMergedRow(t *testing.T) {
	d := testDB(t)
	first := spaceProject(t, d, "Delivery", "")
	second := spaceProject(t, d, "Bidder", "")
	trackerID := first.DefaultTrackerID
	if _, err := d.conn.Exec("DROP INDEX " + macrosTrackerKeyIndex); err != nil {
		t.Fatal(err)
	}
	older, newer := time.Now().Add(-time.Hour).UTC(), time.Now().UTC()
	for _, row := range []struct {
		project, todos string
		at             time.Time
	}{
		{first.ID, "[]", newer},
		{second.ID, `[{"id":"t1","text":"Keep me","done":false}]`, older},
	} {
		if _, err := d.conn.Exec("INSERT INTO macros (project_id, key, tracker_id, title, todos, updated_at) VALUES (?, 'GODE-7', ?, 'Epic', ?, ?)", row.project, trackerID, row.todos, row.at); err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < 2; i++ {
		if err := d.adoptTrackerEpics(); err != nil {
			t.Fatalf("adoption pass %d: %v", i+1, err)
		}
	}
	var rows int
	var survivor, todos string
	if err := d.conn.QueryRow("SELECT COUNT(*), MIN(project_id), MIN(todos) FROM macros WHERE key = 'GODE-7'").Scan(&rows, &survivor, &todos); err != nil {
		t.Fatal(err)
	}
	if rows != 1 || survivor != first.ID || len(parseMacroTodos(todos)) != 1 {
		t.Fatalf("after the merge: %d row(s), survivor %q, todos %s; want Delivery's row with Bidder's todos", rows, survivor, todos)
	}
	if exists, err := d.indexExists(macrosTrackerKeyIndex); err != nil || !exists {
		t.Fatalf("the unique index is missing (%v)", err)
	}
}
