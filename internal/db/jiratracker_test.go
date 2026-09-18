package db

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"tasks/internal/models"
	"tasks/internal/tracker"
	"testing"
	"time"
)

// fakeTracker stands in for a tracker with a full read side. The point of these
// tests is the routing: the database must ask the resolved tracker rather than
// name one, and must keep working for a tracker that has none of it.
type fakeTracker struct {
	tracker.BaseTicketingSystem
	tasks   []models.Task
	boards  []models.TrackerBoard
	columns []models.TrackerColumn
	sprints []models.TrackerSprint
	members map[string][]models.TeamMember
	// memberErr makes the members endpoint fail, which must not fail a sync.
	memberErr error
	calls     []string
	// syncedAs records who the sync ran as, which is what decides whether a
	// personal tracker credential can be resolved at all.
	syncedAs string
}

func newFakeTracker() *fakeTracker {
	return &fakeTracker{
		BaseTicketingSystem: tracker.BaseTicketingSystem{
			TrackerName: "jira",
			Capabilities: []tracker.Capability{
				tracker.CapSync, tracker.CapGet, tracker.CapUpdate, tracker.CapBoard,
				tracker.CapTeam, tracker.CapSprint, tracker.CapEpic,
			},
		},
		members: map[string][]models.TeamMember{},
	}
}

func (f *fakeTracker) record(call string) { f.calls = append(f.calls, call) }

func (f *fakeTracker) called(call string) bool {
	for _, c := range f.calls {
		if c == call {
			return true
		}
	}
	return false
}

func (f *fakeTracker) FormatTaskID(projectID, key, rawID string) string {
	return fmt.Sprintf("jira-%s-%s", projectID, strings.ToUpper(key))
}

func (f *fakeTracker) SyncIssues(ctx context.Context, req tracker.SyncRequest) ([]models.Task, error) {
	f.record("sync")
	f.syncedAs = tracker.ActingUser(ctx)
	return append([]models.Task{}, f.tasks...), nil
}

func (f *fakeTracker) ListBoards(ctx context.Context, req tracker.BoardsRequest) ([]models.TrackerBoard, error) {
	f.record("boards")
	return f.boards, nil
}

func (f *fakeTracker) ListBoardColumns(ctx context.Context, req tracker.BoardRequest) ([]models.TrackerColumn, error) {
	f.record("columns")
	return f.columns, nil
}

func (f *fakeTracker) ListSprints(ctx context.Context, req tracker.BoardRequest) ([]models.TrackerSprint, error) {
	f.record("sprints")
	return f.sprints, nil
}

func (f *fakeTracker) ListIssueTypes(ctx context.Context, req tracker.ProjectRequest) ([]string, error) {
	f.record("issuetypes")
	return []string{"Story", "Bug"}, nil
}

func (f *fakeTracker) SearchTeams(ctx context.Context, req tracker.TeamSearchRequest) ([]models.TrackerTeam, error) {
	f.record("searchteams")
	return []models.TrackerTeam{{ID: "team-1", Name: "Platform"}}, nil
}

func (f *fakeTracker) TeamMembers(ctx context.Context, req tracker.TeamRequest) ([]models.TeamMember, error) {
	f.record("members")
	if f.memberErr != nil {
		return nil, f.memberErr
	}
	return f.members[req.TeamID], nil
}

// jiraTestDB gives a database whose jira adapter is the fake one, with a
// project already configured for it.
func jiraTestDB(t *testing.T, fake *fakeTracker) (*DB, *models.Project) {
	t.Helper()
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	database.TrackerRegistry().Register("jira", fake)
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Platform", IssueTracker: "jira", JiraProject: "PE"})
	if err != nil {
		t.Fatal(err)
	}
	return database, project
}

func TestSyncOnATrackerWithBoardsImportsRefreshesTeamsAndColumns(t *testing.T) {
	fake := newFakeTracker()
	fake.tasks = []models.Task{{
		Key: "PE-1", Title: "Imported", Status: models.StatusToClarify, Priority: models.PriorityMedium,
		Source: "jira", TrackerStatus: "To Do", Team: "Platform", TeamID: "team-1",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}}
	fake.boards = []models.TrackerBoard{{ID: "5", Name: "PE board", Type: "scrum"}}
	fake.columns = []models.TrackerColumn{{Name: "To Do", Statuses: []string{"To Do"}}, {Name: "Done", Statuses: []string{"Done"}}}
	fake.sprints = []models.TrackerSprint{{ID: "9", Name: "Sprint 9", State: "active"}}
	fake.members = map[string][]models.TeamMember{"team-1": {{AccountID: "acc-1", DisplayName: "Ada", Active: true}}}

	database, project := jiraTestDB(t, fake)
	boardID := "5"
	if _, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{BoardID: &boardID}); err != nil {
		t.Fatal(err)
	}

	activity := models.TaskActivity{ID: "sync-jira", TaskID: "sync-" + project.ID, SkillID: "sync_jira", Status: "running", CreatedAt: time.Now()}
	if err := database.AddTaskActivity(activity); err != nil {
		t.Fatal(err)
	}
	settings, err := database.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	database.processSyncJob(context.Background(), SkillJob{SkillID: "sync_jira", ActivityID: activity.ID, ProjectID: project.ID}, settings)

	result, err := database.GetActivityByID(activity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "completed" {
		t.Fatalf("sync did not complete: %#v", result)
	}

	// The work item takes the identity its tracker gives it.
	task, err := database.GetTaskByID("jira-" + project.ID + "-PE-1")
	if err != nil || task == nil {
		t.Fatalf("imported task not found under its canonical id: %v", err)
	}

	// Teams and board followed, through the tracker rather than a name test.
	if !fake.called("members") || !fake.called("columns") || !fake.called("sprints") {
		t.Fatalf("the sync must refresh teams and the board: %v", fake.calls)
	}
	teams, err := database.ListProjectTeams(project.ID, true)
	if err != nil || len(teams) != 1 || len(teams[0].Members) != 1 || teams[0].Members[0].AccountID != "acc-1" {
		t.Fatalf("team members not stored: %v %+v", err, teams)
	}
	refreshed, err := database.GetProjectByID(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(refreshed.TrackerColumns) != 2 || refreshed.TrackerColumns[1].Name != "Done" {
		t.Fatalf("columns not imported: %+v", refreshed.TrackerColumns)
	}
	if len(refreshed.Sprints) != 1 || refreshed.Sprints[0].State != "active" {
		t.Fatalf("sprints not imported: %+v", refreshed.Sprints)
	}
}

// A team whose members cannot be read is kept, with its members unknown: the
// work items are what the sync is for.
func TestSyncSurvivesAnUnreadableTeam(t *testing.T) {
	fake := newFakeTracker()
	fake.memberErr = fmt.Errorf("gateway unavailable")
	fake.tasks = []models.Task{{Key: "PE-2", Title: "Still imported", Status: models.StatusToClarify, Source: "jira", Team: "Platform", TeamID: "team-1", CreatedAt: time.Now(), UpdatedAt: time.Now()}}

	database, project := jiraTestDB(t, fake)
	activity := models.TaskActivity{ID: "sync-degraded", TaskID: "sync-" + project.ID, SkillID: "sync_jira", Status: "running", CreatedAt: time.Now()}
	if err := database.AddTaskActivity(activity); err != nil {
		t.Fatal(err)
	}
	settings, _ := database.GetSettings()
	database.processSyncJob(context.Background(), SkillJob{SkillID: "sync_jira", ActivityID: activity.ID, ProjectID: project.ID}, settings)

	result, err := database.GetActivityByID(activity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "completed" {
		t.Fatalf("an unreadable team must not fail the sync: %#v", result)
	}
	if _, err := database.GetTaskByID("jira-" + project.ID + "-PE-2"); err != nil {
		t.Fatalf("the work item must be imported anyway: %v", err)
	}
	steps := strings.Join(result.Steps, " | ")
	if !strings.Contains(steps, "gateway unavailable") {
		t.Fatalf("the failure must be reported in the activity: %s", steps)
	}
}

func TestProjectStructureRoutesThroughTheResolvedTracker(t *testing.T) {
	fake := newFakeTracker()
	fake.boards = []models.TrackerBoard{{ID: "5", Name: "PE board", Type: "scrum"}}
	database, project := jiraTestDB(t, fake)

	boards, err := database.ListProjectTrackerBoards(project.ID)
	if err != nil || len(boards) != 1 || boards[0].ID != "5" {
		t.Fatalf("boards: %v %+v", err, boards)
	}
	types, err := database.ListProjectIssueTypes(project.ID)
	if err != nil || len(types) != 2 {
		t.Fatalf("issue types: %v %v", err, types)
	}
	teams, err := database.SearchTrackerTeams(project.ID, "plat")
	if err != nil || len(teams) != 1 || teams[0].ID != "team-1" {
		t.Fatalf("team search: %v %+v", err, teams)
	}
	if !fake.called("boards") || !fake.called("issuetypes") || !fake.called("searchteams") {
		t.Fatalf("every read must reach the tracker: %v", fake.calls)
	}
}

// A tracker without boards or teams answers a limit, not a failure, and says
// which capability is missing.
func TestATrackerWithoutBoardsAnswersAnUnsupportedCapability(t *testing.T) {
	database := testDB(t)
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Forge", IssueTracker: "github", GithubRepo: "acme/app"})
	if err != nil {
		t.Fatal(err)
	}
	for name, call := range map[string]func() error{
		"boards": func() error { _, err := database.ListProjectTrackerBoards(project.ID); return err },
		"types":  func() error { _, err := database.ListProjectIssueTypes(project.ID); return err },
		"teams":  func() error { _, err := database.SearchTrackerTeams(project.ID, "x"); return err },
		"members": func() error {
			_, err := database.RefreshTeamMembersNow(project.ID, "team-1")
			return err
		},
		"columns": func() error { _, err := database.SyncProjectBoardColumns(project.ID); return err },
	} {
		err := call()
		if !tracker.IsUnsupported(err) {
			t.Errorf("%s: expected an unsupported-capability error, got %v", name, err)
		}
		if err != nil && !strings.Contains(strings.ToLower(err.Error()), "github") {
			t.Errorf("%s: the refusal must name the tracker: %v", name, err)
		}
	}
}

// A personal tracker credential belongs to a person, and a synchronisation runs
// in a queue that outlives their request. So whoever asked has to travel with
// the job, or the sync resolves the server credential and fails on a
// deployment where only personal tokens exist.
func TestASyncRunsAsWhoeverAskedForIt(t *testing.T) {
	fake := newFakeTracker()
	fake.tasks = []models.Task{{Key: "PE-1", Title: "One", Status: models.StatusToClarify, Source: "jira", CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	database, project := jiraTestDB(t, fake)

	// The job is run here rather than queued: the worker would run it in
	// parallel and overwrite what this test is watching.
	activity := models.TaskActivity{ID: "sync-as", TaskID: "sync-" + project.ID, SkillID: "sync_jira", Status: "running", CreatedAt: time.Now()}
	if err := database.AddTaskActivity(activity); err != nil {
		t.Fatal(err)
	}
	settings, _ := database.GetSettings()

	database.processSyncJob(context.Background(), SkillJob{SkillID: "sync_jira", ActivityID: activity.ID, ProjectID: project.ID, ActingUser: "u-ada"}, settings)
	if fake.syncedAs != "u-ada" {
		t.Fatalf("the sync must run as the person who asked, got %q", fake.syncedAs)
	}

	// A timer asks on nobody's behalf, and resolves the server credential.
	fake.syncedAs = "sentinel"
	database.processSyncJob(context.Background(), SkillJob{SkillID: "sync_jira", ActivityID: activity.ID, ProjectID: project.ID}, settings)
	if fake.syncedAs != "" {
		t.Fatalf("an unattended sync must name nobody, got %q", fake.syncedAs)
	}
}
