package db

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"tasks/internal/models"
)

// sharedBoard gives two labelled projects one Jira tracker whose board has a
// "Doing" and a "Review" column, the tracker mapping #implemented onto Doing.
func sharedBoard(t *testing.T, d *DB) (delivery, bidder *models.Project, trackerID string) {
	t.Helper()
	delivery = spaceProject(t, d, "Delivery", "delivery")
	bidder = spaceProject(t, d, "Bidder", "bidder")
	trackerID = delivery.DefaultTrackerID
	if bidder.DefaultTrackerID != trackerID {
		t.Fatalf("the two projects must share one tracker: %q, %q", trackerID, bidder.DefaultTrackerID)
	}
	if _, err := d.UpdateTrackerMirror(trackerID, func(trk *models.Tracker) {
		trk.TrackerColumns = []models.TrackerColumn{{Name: "Doing", Statuses: []string{"In Progress"}}, {Name: "Review", Statuses: []string{"In Review"}}}
		trk.StageColumns = map[string][]string{"implemented": {"Doing"}}
	}); err != nil {
		t.Fatal(err)
	}
	return delivery, bidder, trackerID
}

// setOwnMapping saves a project's own mapping for a tracker.
func setOwnMapping(d *DB, projectID, trackerID string, stages map[string][]string) error {
	mapping := models.StageMappings{trackerID: stages}
	_, err := d.UpdateProject(projectID, models.UpdateProjectRequest{TrackerStageColumns: &mapping})
	return err
}

// importInReview writes tickets in the "In Review" status, one per key, each
// with its labels.
func importInReview(t *testing.T, d *DB, trackerID string, labels map[string][]string) {
	t.Helper()
	var tickets []models.Task
	for key, l := range labels {
		tickets = append(tickets, models.Task{
			Key: key, Title: key, Status: models.StatusToClarify, Priority: models.PriorityMedium, Labels: l,
			TrackerStatus: "In Review", Source: "jira", CreatedAt: time.Now(), UpdatedAt: time.Now(),
		})
	}
	if err := d.ImportOrUpdateTasks(trackerID, tickets); err != nil {
		t.Fatal(err)
	}
}

func ticketNamed(t *testing.T, d *DB, key string) *models.Task {
	t.Helper()
	task, err := d.GetTaskByID(key)
	if err != nil || task == nil {
		t.Fatalf("%s: %+v (%v)", key, task, err)
	}
	return task
}

// Two projects selecting one tracker each map its stages their own way
// (#741): a ticket reads its stage, and writes its status back, through the
// project in context, else its single project, else the tracker.
func TestEachProjectMapsTheStagesOfItsTrackerItsOwnWay(t *testing.T) {
	d := testDB(t)
	delivery, bidder, trackerID := sharedBoard(t, d)
	if err := setOwnMapping(d, delivery.ID, trackerID, map[string][]string{"implemented": {"Review"}}); err != nil {
		t.Fatal(err)
	}
	importInReview(t, d, trackerID, map[string][]string{"GODE-1": {"delivery"}, "GODE-2": {"bidder"}, "GODE-3": {"delivery", "bidder"}})

	// Import: In Review is #implemented in Delivery only.
	if got := d.StageOfTask(ticketNamed(t, d, "GODE-1")); got != "implemented" {
		t.Fatalf("Delivery's ticket reads stage %q, want implemented by its project's mapping", got)
	}
	if got := d.StageOfTask(ticketNamed(t, d, "GODE-2")); got == "implemented" {
		t.Fatalf("Bidder's ticket must read the tracker's mapping, not Delivery's")
	}
	// A ticket of both projects, read with none in context, reads the tracker's.
	if got := d.StageOfTask(ticketNamed(t, d, "GODE-3")); got == "implemented" {
		t.Fatalf("a ticket of two projects read out of context must read the tracker's mapping")
	}
	// Listed in Delivery, it reads Delivery's.
	listed, err := d.GetTasks("", "", "", "", delivery.ID, "", "", "", "", nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range listed {
		if listed[i].Key == "GODE-3" {
			found = true
			if got := d.StageOfTask(&listed[i]); got != "implemented" {
				t.Fatalf("GODE-3 listed in Delivery reads stage %q, want implemented", got)
			}
		}
	}
	if !found {
		t.Fatal("GODE-3 is not listed in Delivery")
	}

	// Write-back: #implemented lands on each mapping's column.
	for _, c := range []struct{ key, context, want string }{
		{"GODE-1", "", "In Review"},
		{"GODE-2", "", "In Progress"},
		{"GODE-3", "", "In Progress"},
		{"GODE-3", delivery.ID, "In Review"},
		{"GODE-3", bidder.ID, "In Progress"},
	} {
		trk := d.stageTrackerOfTaskUnsafe(ticketNamed(t, d, c.key), c.context)
		if got := TrackerStatusForStage(trk, "implemented"); got != c.want {
			t.Fatalf("%s in context %q: #implemented writes %q, want %q", c.key, c.context, got, c.want)
		}
	}
	labels := []string{"bidder", "#implemented"}
	updated, err := d.UpdateTask(ticketNamed(t, d, "GODE-2").ID, models.UpdateTaskRequest{Labels: &labels})
	if err != nil {
		t.Fatal(err)
	}
	if updated.TrackerStatus != "In Progress" {
		t.Fatalf("Bidder's ticket moved to #implemented writes %q, want the tracker's In Progress", updated.TrackerStatus)
	}

	// A move made from a board follows that board's project (#741, rule 1).
	shared := ticketNamed(t, d, "GODE-3").ID
	labels = []string{"delivery", "bidder", "#implemented"}
	updated, err = d.UpdateTask(shared, models.UpdateTaskRequest{Labels: &labels, StageProjectID: &delivery.ID})
	if err != nil {
		t.Fatal(err)
	}
	if updated.TrackerStatus != "In Review" {
		t.Fatalf("a stage move from Delivery's board writes %q, want Delivery's In Review", updated.TrackerStatus)
	}
	for _, c := range []struct{ project, want string }{{delivery.ID, "implemented"}, {bidder.ID, ""}} {
		moved, _, err := d.MoveTaskToTrackerStatusIn(context.Background(), shared, "In Review", c.project)
		if err != nil {
			t.Fatal(err)
		}
		has := slices.Contains(moved.Labels, "#implemented")
		if (c.want == "implemented") != has {
			t.Fatalf("In Review dropped on %s's board gives labels %v", c.project, moved.Labels)
		}
	}

	// The tracker keeps its own mapping.
	trk, _ := d.GetTrackerByID(trackerID)
	if len(trk.StageColumns["implemented"]) != 1 || trk.StageColumns["implemented"][0] != "Doing" {
		t.Fatalf("a project mapping changed the tracker's: %+v", trk.StageColumns)
	}
}

// A project's own mapping survives a save of its trackers, goes back to the
// tracker's when emptied, and is refused when it names an unknown stage, a
// column the tracker lacks or a tracker the project does not select (#741).
func TestAProjectStageMappingIsKeptResetAndChecked(t *testing.T) {
	d := testDB(t)
	delivery, _, trackerID := sharedBoard(t, d)
	if err := setOwnMapping(d, delivery.ID, trackerID, map[string][]string{"implemented": {"Review"}}); err != nil {
		t.Fatal(err)
	}
	label := "delivery"
	if _, err := d.UpdateProject(delivery.ID, models.UpdateProjectRequest{Trackers: &[]models.ProjectTracker{{TrackerID: trackerID}}, Label: &label}); err != nil {
		t.Fatal(err)
	}
	p, _ := d.GetProjectByID(delivery.ID)
	if !p.Trackers[0].OwnStageColumns || p.StageColumns["implemented"][0] != "Review" || p.Trackers[0].TrackerStageColumns["implemented"][0] != "Doing" {
		t.Fatalf("saving the trackers lost the project's mapping: %+v %+v", p.StageColumns, p.Trackers)
	}

	for name, stages := range map[string]map[string][]string{
		"unknown stage":  {"deployed": {"Review"}},
		"unknown column": {"implemented": {"Shipped"}},
	} {
		if err := setOwnMapping(d, delivery.ID, trackerID, stages); !errors.Is(err, ErrInvalidStageColumns) {
			t.Fatalf("%s: %v, want ErrInvalidStageColumns", name, err)
		}
	}
	if err := setOwnMapping(d, delivery.ID, "elsewhere", map[string][]string{}); !errors.Is(err, ErrInvalidStageColumns) {
		t.Fatalf("a tracker the project does not select: %v, want ErrInvalidStageColumns", err)
	}

	if err := setOwnMapping(d, delivery.ID, trackerID, map[string][]string{}); err != nil {
		t.Fatal(err)
	}
	p, _ = d.GetProjectByID(delivery.ID)
	if p.Trackers[0].OwnStageColumns || p.StageColumns["implemented"][0] != "Doing" {
		t.Fatalf("an empty mapping must read the tracker's again: %+v %+v", p.StageColumns, p.Trackers)
	}

	// An older client sending back the tracker's mapping keeps it inherited.
	inherited := map[string][]string{"implemented": {"Doing"}}
	if _, err := d.UpdateProject(delivery.ID, models.UpdateProjectRequest{StageColumns: &inherited}); err != nil {
		t.Fatal(err)
	}
	p, _ = d.GetProjectByID(delivery.ID)
	if p.Trackers[0].OwnStageColumns {
		t.Fatalf("the tracker's mapping sent back became the project's: %+v", p.Trackers)
	}
}

// A board import dropping a column drops it from the projects' own mappings
// as from the tracker's (#741).
func TestABoardLosingAColumnPrunesTheProjectsMappings(t *testing.T) {
	d := testDB(t)
	delivery, _, trackerID := sharedBoard(t, d)
	if err := setOwnMapping(d, delivery.ID, trackerID, map[string][]string{"implemented": {"Review"}, "specified": {"Doing"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.UpdateTrackerMirror(trackerID, func(trk *models.Tracker) {
		trk.TrackerColumns = []models.TrackerColumn{{Name: "Doing", Statuses: []string{"In Progress"}}}
	}); err != nil {
		t.Fatal(err)
	}
	p, _ := d.GetProjectByID(delivery.ID)
	if !p.Trackers[0].OwnStageColumns || len(p.StageColumns["implemented"]) != 0 || p.StageColumns["specified"][0] != "Doing" {
		t.Fatalf("the project's mapping must lose the dropped column only: %+v", p.StageColumns)
	}
}

// A save naming another of the project's trackers through the legacy fields
// makes it the default and keeps the project's own mapping for it (#741).
func TestATrackerBecomingTheDefaultKeepsTheProjectMapping(t *testing.T) {
	d := testDB(t)
	delivery, _, gode := sharedBoard(t, d)
	other, err := d.CreateProject(models.CreateProjectRequest{Name: "Backend", IssueTracker: "jira", JiraProject: "BE"})
	if err != nil {
		t.Fatal(err)
	}
	be := other.DefaultTrackerID
	if _, err := d.UpdateTrackerMirror(be, func(trk *models.Tracker) {
		trk.TrackerColumns = []models.TrackerColumn{{Name: "QA", Statuses: []string{"Testing"}}}
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.UpdateProject(delivery.ID, models.UpdateProjectRequest{Trackers: &[]models.ProjectTracker{{TrackerID: gode}, {TrackerID: be}}}); err != nil {
		t.Fatal(err)
	}
	if err := setOwnMapping(d, delivery.ID, be, map[string][]string{"implemented": {"QA"}}); err != nil {
		t.Fatal(err)
	}
	kind, key := "jira", "BE"
	if _, err := d.UpdateProject(delivery.ID, models.UpdateProjectRequest{IssueTracker: &kind, JiraProject: &key, JoinTrackerOnly: true}); err != nil {
		t.Fatal(err)
	}
	p, _ := d.GetProjectByID(delivery.ID)
	if p.DefaultTrackerID != be || len(p.StageColumns["implemented"]) != 1 || p.StageColumns["implemented"][0] != "QA" {
		t.Fatalf("BE must be the default with the project's mapping: default %q, stages %+v", p.DefaultTrackerID, p.StageColumns)
	}
}
