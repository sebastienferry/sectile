package db

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// defaultTrackerID is the tracker a project's new tickets go to, and the one
// its tracker ids are formatted with (#741).
func defaultTrackerID(t *testing.T, d *DB, projectID string) string {
	t.Helper()
	trk := d.ProjectDefaultTracker(projectID)
	if trk == nil {
		t.Fatalf("project %s has no tracker", projectID)
	}
	return trk.ID
}

func TestCreatingAJiraProjectCreatesItsTrackerAndLinksIt(t *testing.T) {
	d := testDB(t)
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Delivery", IssueTracker: "jira", JiraProject: "gode", TrackerUrl: "https://acme.atlassian.net", BoardID: "12"})
	if err != nil {
		t.Fatal(err)
	}
	trackers, err := d.ProjectTrackers(p.ID)
	if err != nil || len(trackers) != 1 {
		t.Fatalf("trackers = %+v (%v), want one", trackers, err)
	}
	trk := trackers[0]
	if trk.Provider != "jira" || trk.Scope != "GODE" || trk.Site != "https://acme.atlassian.net" || trk.Identity != "jira|acme.atlassian.net|GODE" || trk.BoardID != "12" {
		t.Fatalf("tracker = %+v", trk)
	}
	if p.DefaultTrackerID != trk.ID || len(p.Trackers) != 1 || p.Trackers[0] != (models.ProjectTracker{TrackerID: trk.ID, Identity: trk.Identity}) {
		t.Fatalf("project = default %q, trackers %+v", p.DefaultTrackerID, p.Trackers)
	}
}

func TestTwoProjectsNamingOneJiraSpaceShareOneTracker(t *testing.T) {
	d := testDB(t)
	first, err := d.CreateProject(models.CreateProjectRequest{Name: "Delivery", IssueTracker: "jira", JiraProject: "GODE", TrackerUrl: "https://acme.atlassian.net"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := d.CreateProject(models.CreateProjectRequest{Name: "Bidder", IssueTracker: "jira", JiraProject: "GODE", TrackerUrl: "https://ACME.atlassian.net/"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := d.CreateProject(models.CreateProjectRequest{Name: "Backend", IssueTracker: "jira", JiraProject: "BE", TrackerUrl: "https://acme.atlassian.net"})
	if err != nil {
		t.Fatal(err)
	}
	if a, b := defaultTrackerID(t, d, first.ID), defaultTrackerID(t, d, second.ID); a != b {
		t.Fatalf("two projects on GODE have the trackers %s and %s", a, b)
	}
	if defaultTrackerID(t, d, other.ID) == defaultTrackerID(t, d, first.ID) {
		t.Fatal("BE and GODE share a tracker")
	}
	all, _ := d.GetTrackers()
	jira := 0
	for _, trk := range all {
		if trk.Provider == "jira" {
			jira++
		}
	}
	if jira != 2 {
		t.Fatalf("%d Jira trackers, want 2", jira)
	}
}

func TestABoardColumnSyncWritesTheMirrorOnTheTracker(t *testing.T) {
	fake := newFakeTracker()
	fake.boards = []models.TrackerBoard{{ID: "5", Name: "PE board", Type: "scrum"}}
	fake.columns = []models.TrackerColumn{{Name: "To Do", Statuses: []string{"To Do"}}, {Name: "Doing", Statuses: []string{"In Progress"}}}
	fake.sprints = []models.TrackerSprint{{ID: "9", Name: "Sprint 9", State: "active"}}
	database, project := jiraTestDB(t, fake)

	if _, err := database.SyncProjectBoardColumns(context.Background(), project.ID); err != nil {
		t.Fatal(err)
	}
	trk, err := database.GetTrackerByID(defaultTrackerID(t, database, project.ID))
	if err != nil || trk == nil {
		t.Fatal(err)
	}
	if trk.BoardID != "5" || len(trk.TrackerColumns) != 2 || trk.TrackerColumns[1].Name != "Doing" || len(trk.Sprints) != 1 {
		t.Fatalf("the tracker carries no mirror: %+v", trk)
	}
	var stored string
	if err := database.conn.QueryRow("SELECT tracker_columns FROM projects WHERE id = ?", project.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != "[]" {
		t.Fatalf("the sync wrote the project's own columns: %s", stored)
	}
}

func TestAProjectReadsItsBoardMirrorFromItsDefaultTracker(t *testing.T) {
	d := testDB(t)
	first, err := d.CreateProject(models.CreateProjectRequest{Name: "Delivery", IssueTracker: "jira", JiraProject: "GODE"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := d.CreateProject(models.CreateProjectRequest{Name: "Bidder", IssueTracker: "jira", JiraProject: "GODE"})
	if err != nil {
		t.Fatal(err)
	}
	columns := []models.TrackerColumn{{Name: "Review", Statuses: []string{"In Review"}}}
	if _, err := d.UpdateTrackerMirror(defaultTrackerID(t, d, first.ID), func(trk *models.Tracker) {
		trk.BoardID, trk.TrackerColumns, trk.StageColumns = "7", columns, map[string][]string{"reviewed": {"Review"}}
	}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{first.ID, second.ID} {
		p, err := d.GetProjectByID(id)
		if err != nil || p == nil {
			t.Fatal(err)
		}
		if p.BoardID != "7" || len(p.TrackerColumns) != 1 || p.TrackerColumns[0].Name != "Review" || p.StageColumns["reviewed"][0] != "Review" {
			t.Fatalf("%s does not read the tracker's mirror: board %q columns %+v stages %+v", p.Name, p.BoardID, p.TrackerColumns, p.StageColumns)
		}
	}
	// A project edit of the mapping lands on the tracker, which the other
	// project then reads.
	stages := map[string][]string{"implemented": {"Review"}}
	if _, err := d.UpdateProject(second.ID, models.UpdateProjectRequest{StageColumns: &stages}); err != nil {
		t.Fatal(err)
	}
	p, _ := d.GetProjectByID(first.ID)
	if len(p.StageColumns["implemented"]) != 1 || len(p.StageColumns["reviewed"]) != 0 {
		t.Fatalf("the mapping saved from one project is not the tracker's: %+v", p.StageColumns)
	}
}

func TestDeletingAProjectKeepsTheTicketsOfItsSharedTracker(t *testing.T) {
	d := testDB(t)
	first, err := d.CreateProject(models.CreateProjectRequest{Name: "Delivery", IssueTracker: "jira", JiraProject: "GODE"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("UPDATE projects SET created_at = ? WHERE id = ?", time.Now().Add(-time.Hour), first.ID); err != nil {
		t.Fatal(err)
	}
	second, err := d.CreateProject(models.CreateProjectRequest{Name: "Bidder", IssueTracker: "jira", JiraProject: "GODE"})
	if err != nil {
		t.Fatal(err)
	}
	local, err := d.CreateProject(models.CreateProjectRequest{Name: "Notes"})
	if err != nil {
		t.Fatal(err)
	}
	trackerID := defaultTrackerID(t, d, first.ID)
	if err := d.ImportOrUpdateTasks(trackerID, []models.Task{{Key: "GODE-1", Title: "Shared", Source: "jira", Status: models.StatusToClarify, Labels: []string{}}}); err != nil {
		t.Fatal(err)
	}
	note, err := d.CreateTask(models.CreateTaskRequest{ProjectID: local.ID, Title: "A local note"})
	if err != nil {
		t.Fatal(err)
	}

	if err := d.DeleteProject(first.ID); err != nil {
		t.Fatal(err)
	}
	shared, err := d.GetTaskByID("jira-" + trackerID + "-GODE-1")
	if err != nil || shared == nil {
		t.Fatalf("the shared tracker's ticket is gone: %v", err)
	}
	if shared.TrackerID != trackerID || shared.ProjectID != second.ID {
		t.Fatalf("the ticket stays on its tracker, shown by the other project: %+v", shared)
	}
	if links, _ := d.ProjectTrackers(first.ID); len(links) != 0 {
		t.Fatalf("the deleted project still links %+v", links)
	}

	if err := d.DeleteProject(local.ID); err != nil {
		t.Fatal(err)
	}
	moved, err := d.GetTaskByID(note.ID)
	if err != nil || moved == nil {
		t.Fatalf("the local ticket is gone: %v", err)
	}
	if moved.ProjectID != "default" || moved.TrackerID != defaultTrackerID(t, d, "default") {
		t.Fatalf("a local ticket moves to the default project and its tracker: %+v", moved)
	}
}

func TestATrackerStillLinkedToAProjectCannotBeDeleted(t *testing.T) {
	d := testDB(t)
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Delivery", IssueTracker: "jira", JiraProject: "GODE"})
	if err != nil {
		t.Fatal(err)
	}
	trackerID := defaultTrackerID(t, d, p.ID)
	if err := d.DeleteTrackerAs("admin", trackerID); !errors.Is(err, ErrTrackerInUse) {
		t.Fatalf("a linked tracker was deleted: %v", err)
	}
	if trk, _ := d.GetTrackerByID(trackerID); trk == nil {
		t.Fatal("the tracker is gone")
	}
	free, err := d.CreateTrackerAs("admin", models.Tracker{Provider: "github", Scope: "Acme/App"})
	if err != nil {
		t.Fatal(err)
	}
	if free.Scope != "Acme/App" || free.Identity != "github|api.github.com|acme/app" {
		t.Fatalf("created tracker = %+v", free)
	}
	if _, err := d.CreateTrackerAs("admin", models.Tracker{Provider: "github", Scope: "acme/app"}); err == nil {
		t.Fatal("a second tracker of one identity was created")
	}
	if err := d.DeleteTrackerAs("admin", free.ID); err != nil {
		t.Fatalf("a tracker nobody uses is deleted: %v", err)
	}
}

// A tracker holding tickets keeps its source: they were read from it. One
// holding none may change it, and its next background pass reads the new
// source whole.
func TestATrackerHoldingTicketsKeepsItsSource(t *testing.T) {
	d := testDB(t)
	used, err := d.CreateTrackerAs("admin", models.Tracker{Provider: "jira", Scope: "GODE"})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.ImportOrUpdateTasks(used.ID, []models.Task{{Key: "GODE-1", Title: "Imported", Source: "jira", Status: models.StatusToClarify, Labels: []string{}}}); err != nil {
		t.Fatal(err)
	}
	read, err := d.GetTrackerByID(used.ID)
	if err != nil || read == nil || read.TicketCount != 1 {
		t.Fatalf("the tracker must count its ticket: %+v (%v)", read, err)
	}
	moved := *read
	moved.Scope = "OTHER"
	if _, err := d.UpdateTrackerAs("admin", moved); !errors.Is(err, ErrTrackerSourceInUse) {
		t.Fatalf("a tracker holding tickets changed its scope: %v", err)
	}
	moved = *read
	moved.Site = "https://elsewhere.atlassian.net"
	if _, err := d.UpdateTrackerAs("admin", moved); !errors.Is(err, ErrTrackerSourceInUse) {
		t.Fatalf("a tracker holding tickets changed its site: %v", err)
	}
	renamed := *read
	renamed.Name = "Delivery"
	if saved, err := d.UpdateTrackerAs("admin", renamed); err != nil || saved.Name != "Delivery" || saved.Scope != "GODE" || saved.TicketCount != 1 {
		t.Fatalf("a tracker holding tickets is still renamed: %+v (%v)", saved, err)
	}

	empty, err := d.CreateTrackerAs("admin", models.Tracker{Provider: "jira", Scope: "BE"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec(`INSERT INTO auto_sync_trackers (tracker_id, last_pass_at, last_full_sync_at) VALUES (?, ?, ?)`, empty.ID, time.Now().UTC(), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	empty.Scope = "OPS"
	saved, err := d.UpdateTrackerAs("admin", *empty)
	if err != nil || saved.Scope != "OPS" || saved.TicketCount != 0 {
		t.Fatalf("an empty tracker changes its scope: %+v (%v)", saved, err)
	}
	var lastFull sql.NullTime
	if err := d.conn.QueryRow(`SELECT last_full_sync_at FROM auto_sync_trackers WHERE tracker_id = ?`, empty.ID).Scan(&lastFull); err != nil {
		t.Fatal(err)
	}
	if lastFull.Valid {
		t.Fatalf("the tracker's next pass must read its new source whole, last full read %v", lastFull.Time)
	}
}

// A project's own tracker renamed in place through its tracker fields reads
// another source: its next background pass reads it whole.
func TestATrackerRenamedInPlaceReadsItsNewSourceWhole(t *testing.T) {
	d := testDB(t)
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Delivery", IssueTracker: "jira", JiraProject: "GODE"})
	if err != nil {
		t.Fatal(err)
	}
	trackerID := defaultTrackerID(t, d, p.ID)
	if _, err := d.conn.Exec(`INSERT INTO auto_sync_trackers (tracker_id, last_pass_at, last_full_sync_at) VALUES (?, ?, ?)`, trackerID, time.Now().UTC(), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	other := "OPS"
	if _, err := d.UpdateProject(p.ID, models.UpdateProjectRequest{JiraProject: &other}); err != nil {
		t.Fatal(err)
	}
	if trk, _ := d.GetTrackerByID(trackerID); trk == nil || trk.Scope != "OPS" {
		t.Fatalf("the project's own tracker must be renamed in place: %+v", trk)
	}
	var lastFull sql.NullTime
	if err := d.conn.QueryRow(`SELECT last_full_sync_at FROM auto_sync_trackers WHERE tracker_id = ?`, trackerID).Scan(&lastFull); err != nil {
		t.Fatal(err)
	}
	if lastFull.Valid {
		t.Fatalf("the renamed tracker kept its last full read %v", lastFull.Time)
	}
}

// syncJira runs one Jira synchronisation of a project the way the queue does.
func syncJira(t *testing.T, d *DB, projectID, activityID string) {
	t.Helper()
	activity := models.TaskActivity{ID: activityID, ProjectID: projectID, SkillID: "sync_jira", Status: "running", CreatedAt: time.Now()}
	if err := d.AddTaskActivity(activity); err != nil {
		t.Fatal(err)
	}
	settings, _ := d.GetSettings()
	d.processSyncJob(context.Background(), SkillJob{SkillID: "sync_jira", ActivityID: activityID, ProjectID: projectID, TrackerID: defaultTrackerID(t, d, projectID)}, settings)
}

func TestTwoProjectsOnOneSpaceSeeOneRecordPerTicketAfterASync(t *testing.T) {
	fake := newFakeTracker()
	fake.tasks = []models.Task{{Key: "PE-1", Title: "Shared", Status: models.StatusToClarify, Source: "jira", CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	database, first := jiraTestDB(t, fake)
	second, err := database.CreateProject(models.CreateProjectRequest{Name: "Bidder", IssueTracker: "jira", JiraProject: "PE"})
	if err != nil {
		t.Fatal(err)
	}
	syncJira(t, database, first.ID, "sync-first")
	syncJira(t, database, second.ID, "sync-second")

	var rows int
	if err := database.conn.QueryRow("SELECT COUNT(*) FROM tasks WHERE key = 'PE-1'").Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("%d records of PE-1, want one for the two projects", rows)
	}
	task, err := database.GetTaskByID("jira-" + defaultTrackerID(t, database, second.ID) + "-PE-1")
	if err != nil || task == nil || task.TrackerID != defaultTrackerID(t, database, first.ID) {
		t.Fatalf("the record is the tracker's: %+v %v", task, err)
	}
}

func TestAStageChangeWritesBackThroughTheTrackerOfTheTask(t *testing.T) {
	fake := newFakeTracker()
	fake.tasks = []models.Task{{Key: "PE-1", Title: "One", Status: models.StatusToClarify, Source: "jira", CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	database, project := jiraTestDB(t, fake)
	syncJira(t, database, project.ID, "sync-for-stage")
	trackerID := defaultTrackerID(t, database, project.ID)

	if _, _, err := database.TransitionTaskStageBy("u-ada", "jira-"+trackerID+"-PE-1", "clarified", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if who := fake.updatedBy(t); who != "u-ada" {
		t.Fatalf("the write must run as its author, got %q", who)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.updatedOn != trackerID || fake.updatedCtxOn != trackerID {
		t.Fatalf("the stage write named the tracker %q (request) and %q (context), want %q", fake.updatedOn, fake.updatedCtxOn, trackerID)
	}
}

func TestATrackerUpdateJobSetsTheTrackerInTheContext(t *testing.T) {
	fake := newFakeTracker()
	fake.tasks = []models.Task{{Key: "PE-1", Title: "One", Status: models.StatusToClarify, Source: "jira", CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	database, project := jiraTestDB(t, fake)
	syncJira(t, database, project.ID, "sync-for-update-job")
	trackerID := defaultTrackerID(t, database, project.ID)

	activity := models.TaskActivity{ID: "update-job", TaskID: "jira-" + trackerID + "-PE-1", SkillID: "tracker_update", Status: "queued", CreatedAt: time.Now()}
	if err := database.AddTaskActivity(activity); err != nil {
		t.Fatal(err)
	}
	database.processTrackerUpdateJob(context.Background(), SkillJob{ActivityID: activity.ID, TaskID: activity.TaskID, SkillID: "tracker_update", ActingUser: "u-ada"})
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.updatedCtxOn != trackerID || fake.updatedOn != trackerID {
		t.Fatalf("the update named the tracker %q (context) and %q (request), want %q", fake.updatedCtxOn, fake.updatedOn, trackerID)
	}
}

func TestACommentIsWrittenWithTheTrackerOfTheTask(t *testing.T) {
	fake := newFakeTracker()
	fake.tasks = []models.Task{{Key: "PE-1", Title: "One", Status: models.StatusToClarify, Source: "jira", CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	database, project := jiraTestDB(t, fake)
	syncJira(t, database, project.ID, "sync-for-comment-tracker")
	trackerID := defaultTrackerID(t, database, project.ID)

	if err := database.AddTaskCommentAs(tracker.WithActingUser(context.Background(), "u-ada"), "jira-"+trackerID+"-PE-1", "Hello"); err != nil {
		t.Fatal(err)
	}
	if fake.commentedOn != trackerID || fake.commentedCtxOn != trackerID {
		t.Fatalf("the comment named the tracker %q (request) and %q (context), want %q", fake.commentedOn, fake.commentedCtxOn, trackerID)
	}
}

func TestAMergedAwayTicketIdStillResolvesThroughItsAlias(t *testing.T) {
	d := testDB(t)
	first, second := twoProjectsOnPE(t, d)
	now := time.Now().UTC()
	legacyTask(t, d, "jira-a-PE-1", first.ID, "PE-1", "implemented", now, `[]`, `[]`)
	legacyTask(t, d, "jira-b-PE-1", second.ID, "PE-1", "new", now, `[]`, `[]`)
	adopt(t, d)

	task, err := d.GetTaskByID("jira-b-PE-1")
	if err != nil || task == nil || task.ID != "jira-a-PE-1" {
		t.Fatalf("the merged-away id resolves to %+v (%v), want the survivor", task, err)
	}
	if err := d.ImportOrUpdateTasks("", []models.Task{{
		ID:        "jira-b-PE-1",
		ProjectID: second.ID,
		Key:       "PE-1",
		Title:     "Renamed",
		Source:    "jira",
		Status:    models.StatusToClarify,
		Labels:    []string{},
	}}); err != nil {
		t.Fatal(err)
	}
	var rows int
	_ = d.conn.QueryRow("SELECT COUNT(*) FROM tasks WHERE key = 'PE-1'").Scan(&rows)
	if renamed, _ := d.GetTaskByID("jira-a-PE-1"); rows != 1 || renamed == nil || renamed.Title != "Renamed" {
		t.Fatalf("an import through the alias updates the survivor: %d rows, %+v", rows, renamed)
	}
}

func TestAKeyMatchingTwoTrackersIsRefusedAsAmbiguous(t *testing.T) {
	d := testDB(t)
	app, err := d.CreateProject(models.CreateProjectRequest{Name: "App", IssueTracker: "github", GithubRepo: "acme/app"})
	if err != nil {
		t.Fatal(err)
	}
	api, err := d.CreateProject(models.CreateProjectRequest{Name: "Api", IssueTracker: "github", GithubRepo: "acme/api"})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []*models.Project{app, api} {
		if err := d.ImportOrUpdateTasks(defaultTrackerID(t, d, p.ID), []models.Task{{Key: "#12", Title: p.Name, Source: "github", Status: models.StatusToClarify, Labels: []string{}}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := d.GetTaskByID("#12"); !errors.Is(err, ErrTaskKeyAmbiguous) {
		t.Fatalf("a key two trackers carry must be refused, got %v", err)
	}
	if task, err := d.GetTaskByID("gh-" + defaultTrackerID(t, d, api.ID) + "-12"); err != nil || task == nil || task.Title != "Api" {
		t.Fatalf("the id still names one ticket: %+v %v", task, err)
	}
}

// twoProjectsOnGode creates two projects on the GODE space, and imports three
// tickets into their shared tracker.
func twoProjectsOnGode(t *testing.T, d *DB) (first, second *models.Project, trackerID string) {
	t.Helper()
	var err error
	if first, err = d.CreateProject(models.CreateProjectRequest{Name: "Delivery", IssueTracker: "jira", JiraProject: "GODE"}); err != nil {
		t.Fatal(err)
	}
	if second, err = d.CreateProject(models.CreateProjectRequest{Name: "Bidder", IssueTracker: "jira", JiraProject: "GODE"}); err != nil {
		t.Fatal(err)
	}
	trackerID = defaultTrackerID(t, d, first.ID)
	if err := d.ImportOrUpdateTasks(trackerID, []models.Task{
		{Key: "GODE-1", Title: "Backend work", Source: "jira", Status: models.StatusToClarify, Labels: []string{"Backend"}},
		{Key: "GODE-2", Title: "Frontend work", Source: "jira", Status: models.StatusToClarify, Labels: []string{"frontend"}},
		{Key: "GODE-3", Title: "Unlabelled", Source: "jira", Status: models.StatusToClarify, Labels: []string{}},
	}); err != nil {
		t.Fatal(err)
	}
	return first, second, trackerID
}

func taskKeys(tasks []models.Task) map[string]int {
	keys := map[string]int{}
	for _, task := range tasks {
		keys[task.Key]++
	}
	return keys
}

func TestAProjectListsEveryTicketOfItsTracker(t *testing.T) {
	d := testDB(t)
	first, second, _ := twoProjectsOnGode(t, d)
	for _, p := range []*models.Project{first, second} {
		for _, ref := range []string{p.ID, p.Slug} {
			tasks, err := d.GetTasksInScope(TaskScope{ProjectID: ref}, "", "", "", "", "", "", "", "", nil, nil, false)
			if err != nil {
				t.Fatal(err)
			}
			if keys := taskKeys(tasks); len(keys) != 3 || keys["GODE-1"] != 1 || keys["GODE-2"] != 1 || keys["GODE-3"] != 1 {
				t.Fatalf("%s (%s) lists %v, want the three tickets of its tracker", p.Name, ref, keys)
			}
		}
		facets, err := d.GetTaskFacetsInScope(TaskScope{ProjectID: p.ID})
		if err != nil {
			t.Fatal(err)
		}
		if facets.Total != 3 {
			t.Fatalf("%s counts %d tickets in its facets, want 3", p.Name, facets.Total)
		}
	}
	other, err := d.CreateProject(models.CreateProjectRequest{Name: "Backend", IssueTracker: "jira", JiraProject: "BE"})
	if err != nil {
		t.Fatal(err)
	}
	if tasks, _ := d.GetTasksInScope(TaskScope{ProjectID: other.ID}, "", "", "", "", "", "", "", "", nil, nil, false); len(tasks) != 0 {
		t.Fatalf("a project on another space lists %v", taskKeys(tasks))
	}
}

func TestAllProjectsListsASharedTicketOnce(t *testing.T) {
	d := testDB(t)
	first, second, _ := twoProjectsOnGode(t, d)
	user := "u-ada"
	for _, p := range []*models.Project{first, second} {
		if err := d.BookmarkProject(user, p.ID); err != nil {
			t.Fatal(err)
		}
	}
	tasks, err := d.GetTasksInScope(TaskScope{UserID: user, ProjectID: "all"}, "", "", "", "", "", "", "", "", nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	keys := taskKeys(tasks)
	for _, key := range []string{"GODE-1", "GODE-2", "GODE-3"} {
		if keys[key] != 1 {
			t.Fatalf("all projects list %s %d times, want once: %v", key, keys[key], keys)
		}
	}
}

func TestASavedViewStillFiltersByLabelOverTrackerScope(t *testing.T) {
	d := testDB(t)
	_, second, _ := twoProjectsOnGode(t, d)
	name, projects, labels := "Backend", []string{second.ID}, []string{"backend"}
	view, err := d.CreateBoardView("u-ada", models.BoardViewRequest{Name: &name, ProjectIDs: &projects, Labels: &labels})
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := d.GetTasksInScope(TaskScope{UserID: "u-ada", ViewID: view.ID}, "", "", "", "", "", "", "", "", nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if keys := taskKeys(tasks); len(keys) != 1 || keys["GODE-1"] != 1 {
		t.Fatalf("the view lists %v, want the labelled ticket of the tracker only", keys)
	}
}

// A project alone on its tracker that switches to a space another project's
// tracker already names joins that tracker: its own is not renamed onto an
// identity that exists. Its old tracker keeps its tickets, unlinked (#741).
func TestAProjectSwitchingToAnExistingTrackerJoinsItInsteadOfRenamingItsOwn(t *testing.T) {
	d := testDB(t)
	p1, err := d.CreateProject(models.CreateProjectRequest{Name: "Delivery", IssueTracker: "jira", JiraProject: "GODE"})
	if err != nil {
		t.Fatal(err)
	}
	p2, err := d.CreateProject(models.CreateProjectRequest{Name: "Backend", IssueTracker: "jira", JiraProject: "BE"})
	if err != nil {
		t.Fatal(err)
	}
	gode, be := defaultTrackerID(t, d, p1.ID), defaultTrackerID(t, d, p2.ID)
	if err := d.ImportOrUpdateTasks(gode, []models.Task{{Key: "GODE-1", Title: "Gode", Source: "jira", Status: models.StatusToClarify, Labels: []string{}}}); err != nil {
		t.Fatal(err)
	}
	if err := d.ImportOrUpdateTasks(be, []models.Task{{Key: "BE-1", Title: "Be", Source: "jira", Status: models.StatusToClarify, Labels: []string{}}}); err != nil {
		t.Fatal(err)
	}

	key := "BE"
	switched, err := d.UpdateProject(p1.ID, models.UpdateProjectRequest{JiraProject: &key})
	if err != nil {
		t.Fatalf("switching to an existing space failed: %v", err)
	}
	if switched.DefaultTrackerID != be || len(switched.Trackers) != 1 || switched.Trackers[0].TrackerID != be {
		t.Fatalf("the project must join the BE tracker %s: default %q, trackers %+v", be, switched.DefaultTrackerID, switched.Trackers)
	}
	all, _ := d.GetTrackers()
	identities := map[string]int{}
	for _, trk := range all {
		identities[trk.Identity]++
	}
	for identity, n := range identities {
		if n != 1 {
			t.Fatalf("%d trackers share the identity %s", n, identity)
		}
	}
	old, _ := d.GetTrackerByID(gode)
	if old == nil || old.Scope != "GODE" {
		t.Fatalf("the old tracker must stay GODE, not be renamed: %+v", old)
	}
	if links, _ := d.ProjectTrackers(p2.ID); len(links) != 1 || links[0].ID != be {
		t.Fatalf("the other project keeps its tracker: %+v", links)
	}
	ticket, err := d.GetTaskByID("jira-" + gode + "-GODE-1")
	if err != nil || ticket == nil || ticket.TrackerID != gode {
		t.Fatalf("the GODE ticket stays on its tracker: %+v %v", ticket, err)
	}
	tasks, err := d.GetTasksInScope(TaskScope{ProjectID: p1.ID}, "", "", "", "", "", "", "", "", nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if keys := taskKeys(tasks); len(keys) != 1 || keys["BE-1"] != 1 {
		t.Fatalf("the switched project lists %v, want the BE tickets", keys)
	}
	if err := d.DeleteTrackerAs("admin", gode); !errors.Is(err, ErrTrackerInUse) {
		t.Fatalf("the old tracker still holds its ticket and cannot be deleted: %v", err)
	}
}
