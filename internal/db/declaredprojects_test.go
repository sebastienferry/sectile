package db

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"tasks/internal/models"
	"tasks/internal/testsqlite"
	"tasks/internal/tracker"
)

// declaredTracker is a Jira-shaped tracker serving epics per project key, so the
// roadmap projects of #632 can be read, written and created in without a
// tracker. A key listed in failing answers every call with an error.
type declaredTracker struct {
	tracker.BaseTicketingSystem
	epics      map[string][]models.Task
	failing    map[string]bool
	listed     []string
	writes     []tracker.UpdateIssueRequest
	created    []tracker.CreateIssueRequest
	parents    map[string]string
	createErr  error
	parentErr  error
	nextNumber int
}

func newDeclaredTracker(epics map[string][]models.Task) *declaredTracker {
	return &declaredTracker{
		BaseTicketingSystem: tracker.BaseTicketingSystem{
			TrackerName: "jira",
			Capabilities: []tracker.Capability{
				tracker.CapUpdate, tracker.CapLabels, tracker.CapEpic, tracker.CapCreate,
			},
		},
		epics:      epics,
		failing:    map[string]bool{},
		parents:    map[string]string{},
		nextNumber: 100,
	}
}

func (f *declaredTracker) ListEpics(ctx context.Context, req tracker.ProjectRequest) ([]models.Task, error) {
	key := req.Project.JiraProject
	f.listed = append(f.listed, key)
	if f.failing[key] {
		return nil, errors.New("403 no permission")
	}
	return f.epics[key], nil
}

func (f *declaredTracker) UpdateIssue(ctx context.Context, req tracker.UpdateIssueRequest) error {
	f.writes = append(f.writes, req)
	return nil
}

func (f *declaredTracker) GetIssue(ctx context.Context, req tracker.GetIssueRequest) (*models.Task, error) {
	return &models.Task{Key: req.Key}, nil
}

func (f *declaredTracker) CreateIssue(ctx context.Context, req tracker.CreateIssueRequest) (*models.Task, error) {
	f.created = append(f.created, req)
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.nextNumber++
	key := req.Project.JiraProject + "-" + strconv.Itoa(f.nextNumber)
	return &models.Task{ID: "jira-" + req.Project.ID + "-" + key, ProjectID: req.Project.ID, Key: key, Title: req.Title}, nil
}

func (f *declaredTracker) SetParent(ctx context.Context, key string, parentKey string) error {
	if f.parentErr != nil {
		return f.parentErr
	}
	f.parents[key] = parentKey
	return nil
}

// declaredProject opens a database holding the Jira project PE declaring the
// roadmap projects given, its tracker being the fake.
func declaredProject(t *testing.T, fake *declaredTracker, declared ...string) (*DB, *models.Project) {
	t.Helper()
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "test.db"), NewDB)
	if err != nil {
		t.Fatalf("database not initialised: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	proj, err := database.CreateProject(models.CreateProjectRequest{
		Name:            "Platform",
		Slug:            "platform",
		IssueTracker:    "jira",
		JiraProject:     "PE",
		RoadmapProjects: declared,
	})
	if err != nil {
		t.Fatalf("project not created: %v", err)
	}
	database.TrackerRegistry().Register("jira", fake)
	return database, proj
}

func openAxisWrites(t *testing.T, database *DB, proj *models.Project) *models.Project {
	t.Helper()
	open := true
	updated, err := database.UpdateProject(proj.ID, models.UpdateProjectRequest{RoadmapAxisWrites: &open})
	if err != nil {
		t.Fatalf("opt-in not saved: %v", err)
	}
	return updated
}

func macrosByKey(t *testing.T, database *DB, projectID string) map[string]models.MacroMeta {
	t.Helper()
	metas, err := database.GetProjectMacros(projectID)
	if err != nil {
		t.Fatalf("cannot read macros: %v", err)
	}
	out := map[string]models.MacroMeta{}
	for _, m := range metas {
		out[m.Key] = m
	}
	return out
}

func TestMacroOriginAndForeignness(t *testing.T) {
	jira := &models.Project{IssueTracker: "jira", JiraProject: "PE", RoadmapProjects: []string{"DATA"}}
	github := &models.Project{IssueTracker: "github"}
	cases := []struct {
		key     string
		proj    *models.Project
		origin  string
		foreign bool
	}{
		{"PE-1", jira, "PE", false},
		{"pe-2", jira, "PE", false},
		{"DATA-12", jira, "DATA", true},
		{"OLD-3", jira, "OLD", true},
		{"M-4", jira, "", false},
		{"NOKEY", jira, "", false},
		{"DATA-12", github, "DATA", false},
		{"M-4", github, "", false},
	}
	for _, c := range cases {
		if got := macroOrigin(c.key); got != c.origin {
			t.Errorf("macroOrigin(%q) = %q, want %q", c.key, got, c.origin)
		}
		if got := isForeignMacro(c.key, c.proj); got != c.foreign {
			t.Errorf("isForeignMacro(%q, %s) = %v, want %v", c.key, c.proj.IssueTracker, got, c.foreign)
		}
	}
	if !isDeclaredRoadmapProject(jira, "data") || isDeclaredRoadmapProject(jira, "OLD") || isDeclaredRoadmapProject(jira, "") {
		t.Error("isDeclaredRoadmapProject must follow the current declaration, case ignored")
	}
}

func TestRoadmapAxisWritesIsClosedByDefaultAndWithoutADeclaration(t *testing.T) {
	database, proj := declaredProject(t, newDeclaredTracker(nil), "DATA")
	if proj.RoadmapAxisWrites {
		t.Fatal("a new project must start with the opt-in closed")
	}
	proj = openAxisWrites(t, database, proj)
	if !proj.RoadmapAxisWrites {
		t.Fatal("the opt-in must round-trip")
	}
	reread, _ := database.GetProjectByID(proj.ID)
	if !reread.RoadmapAxisWrites {
		t.Fatal("the opt-in must be stored")
	}

	empty := []string{}
	closed, err := database.UpdateProject(proj.ID, models.UpdateProjectRequest{RoadmapProjects: &empty})
	if err != nil {
		t.Fatal(err)
	}
	if closed.RoadmapAxisWrites {
		t.Error("emptying the declaration must close the opt-in")
	}

	local, err := database.CreateProject(models.CreateProjectRequest{Name: "Local", Slug: "local", IssueTracker: "local", RoadmapProjects: []string{"DATA"}, RoadmapAxisWrites: true})
	if err != nil {
		t.Fatal(err)
	}
	if local.RoadmapAxisWrites {
		t.Error("a project that is not a Jira project can never open the opt-in")
	}
}

func TestImportReadsEachDeclaredKeyAndNamesTheOneThatFails(t *testing.T) {
	fake := newDeclaredTracker(map[string][]models.Task{
		"PE":   {{Key: "PE-1", Title: "Ours", Labels: []string{"roadmap:now"}}},
		"DATA": {{Key: "DATA-12", Title: "Theirs", Labels: []string{"priority:p1", "quarter:2026-q4", "roadmap:next"}}},
	})
	fake.failing["OPS"] = true
	database, proj := declaredProject(t, fake, "DATA", "OPS")

	summary, err := database.ImportMacroHorizons(t.Context(), proj.ID)
	if err != nil {
		t.Fatalf("a failing declared key must not fail the read: %v", err)
	}
	if !strings.Contains(summary, "projet de roadmap OPS non lu") || strings.Contains(summary, "DATA non lu") {
		t.Errorf("summary = %q, want OPS named and DATA not", summary)
	}
	if strings.Join(fake.listed, ",") != "PE,DATA,OPS" {
		t.Errorf("listed = %v, want one request per key, own first", fake.listed)
	}

	byKey := macrosByKey(t, database, proj.ID)
	theirs, ok := byKey["DATA-12"]
	if !ok {
		t.Fatalf("the declared epic must be stored, got %v", byKey)
	}
	if theirs.Title != "Theirs" || theirs.Horizon != "next" || theirs.Priority != "p1" || theirs.Quarter != "2026-Q4" {
		t.Errorf("declared epic = %+v, want its title and axes read from its labels", theirs)
	}
	if !theirs.Foreign || theirs.Origin != "DATA" || theirs.LabelsWritable || theirs.AxesWritable {
		t.Errorf("declared epic flags = foreign %v origin %q labels %v axes %v", theirs.Foreign, theirs.Origin, theirs.LabelsWritable, theirs.AxesWritable)
	}
	if ours := byKey["PE-1"]; ours.Foreign || ours.Origin != "PE" || !ours.LabelsWritable || !ours.AxesWritable {
		t.Errorf("own epic flags = %+v", ours)
	}

	// No work item of a declared project is imported: the board is untouched.
	tasks, err := database.GetTasks("", "", "", "", proj.ID, "", "", "", "", nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Errorf("the epic read must import no task, got %d", len(tasks))
	}
}

func TestImportFailsWhenTheOwnKeyFails(t *testing.T) {
	fake := newDeclaredTracker(map[string][]models.Task{"DATA": {{Key: "DATA-1"}}})
	fake.failing["PE"] = true
	database, proj := declaredProject(t, fake, "DATA")
	if _, err := database.ImportMacroHorizons(t.Context(), proj.ID); err == nil {
		t.Fatal("the own key's failure must fail the read")
	}
}

func TestImportWithoutDeclarationReadsTheOwnKeyOnly(t *testing.T) {
	fake := newDeclaredTracker(map[string][]models.Task{"PE": {{Key: "PE-1"}}})
	database, proj := declaredProject(t, fake)
	if _, err := database.ImportMacroHorizons(t.Context(), proj.ID); err != nil {
		t.Fatal(err)
	}
	if strings.Join(fake.listed, ",") != "PE" {
		t.Errorf("listed = %v, want the own key alone", fake.listed)
	}
}

func TestARemovedDeclaredKeyKeepsItsStoredEpics(t *testing.T) {
	fake := newDeclaredTracker(map[string][]models.Task{"PE": {{Key: "PE-1"}}, "DATA": {{Key: "DATA-12", Title: "Theirs"}}})
	database, proj := declaredProject(t, fake, "DATA")
	if _, err := database.ImportMacroHorizons(t.Context(), proj.ID); err != nil {
		t.Fatal(err)
	}
	empty := []string{}
	if _, err := database.UpdateProject(proj.ID, models.UpdateProjectRequest{RoadmapProjects: &empty}); err != nil {
		t.Fatal(err)
	}
	fake.listed = nil
	if _, err := database.ImportMacroHorizons(t.Context(), proj.ID); err != nil {
		t.Fatal(err)
	}
	if strings.Join(fake.listed, ",") != "PE" {
		t.Errorf("listed = %v, a removed key must no longer be read", fake.listed)
	}
	if theirs, ok := macrosByKey(t, database, proj.ID)["DATA-12"]; !ok || !theirs.Foreign {
		t.Errorf("the stored epic of a removed key must stay, foreign, got %+v", theirs)
	}
}

func TestForeignEpicWritesFollowTheAxisAndTheOptIn(t *testing.T) {
	fake := newDeclaredTracker(nil)
	database, proj := declaredProject(t, fake, "DATA")
	ctx := t.Context()

	// Closed: every write refuses, and nothing reaches the tracker.
	if _, err := database.PushMacroPriorityLabel(ctx, proj.ID, "DATA-12", "p1"); err == nil {
		t.Error("the priority of a foreign epic must be refused while the opt-in is closed")
	}
	if database.MacroAxesWritable(proj.ID, "DATA-12", false) {
		t.Error("a foreign epic's axes must not be writable while the opt-in is closed")
	}

	proj = openAxisWrites(t, database, proj)
	if !database.MacroAxesWritable(proj.ID, "DATA-12", false) {
		t.Error("a panel edit of a declared epic's axes must be writable once opted in")
	}
	if database.MacroAxesWritable(proj.ID, "DATA-12", true) {
		t.Error("a bulk edit must never write on a foreign epic")
	}
	if database.MacroLabelsWritable(proj.ID, "DATA-12") {
		t.Error("the horizon and the free labels of a foreign epic stay unwritten whatever the opt-in")
	}
	if database.MacroAxesWritable(proj.ID, "OLD-3", false) {
		t.Error("an epic of a project no longer declared must not be writable")
	}

	if _, err := database.PushMacroPriorityLabel(ctx, proj.ID, "DATA-12", "p1"); err != nil {
		t.Fatalf("priority refused once opted in: %v", err)
	}
	if _, err := database.PushMacroQuarterLabel(ctx, proj.ID, "DATA-12", "2026-Q4"); err != nil {
		t.Fatalf("quarter refused once opted in: %v", err)
	}
	if _, err := database.PushMacroHorizonLabel(ctx, proj.ID, "DATA-12", "now"); err == nil {
		t.Error("the horizon of a foreign epic must stay refused once opted in")
	}
	if _, _, err := database.ValidateMacroLabelEdit(proj.ID, "DATA-12", []string{"team"}, nil); err == nil {
		t.Error("the free labels of a foreign epic must stay refused once opted in")
	}
	if len(fake.writes) != 2 {
		t.Fatalf("writes = %d, want the priority and the quarter only", len(fake.writes))
	}
	for _, w := range fake.writes {
		if w.Key != "DATA-12" || w.Title != nil || w.Description != nil || w.Priority != nil {
			t.Errorf("write = %+v, want labels only on DATA-12", w)
		}
	}

	// Undeclaring the project between the click and the run refuses the write.
	empty := []string{}
	if _, err := database.UpdateProject(proj.ID, models.UpdateProjectRequest{RoadmapProjects: &empty}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.PushMacroPriorityLabel(ctx, proj.ID, "DATA-12", "p2"); err == nil {
		t.Error("a key undeclared since must refuse the queued write")
	}
}

func TestPendingPushesNeverListAForeignEpic(t *testing.T) {
	fake := newDeclaredTracker(map[string][]models.Task{"PE": {{Key: "PE-1"}}, "DATA": {{Key: "DATA-12"}}})
	database, proj := declaredProject(t, fake, "DATA")
	proj = openAxisWrites(t, database, proj)
	if _, err := database.ImportMacroHorizons(t.Context(), proj.ID); err != nil {
		t.Fatal(err)
	}
	p1 := "p1"
	now := "now"
	for _, key := range []string{"PE-1", "DATA-12"} {
		if _, err := database.SaveMacroMeta(proj.ID, key, &now, nil, nil, nil); err != nil {
			t.Fatal(err)
		}
		if _, err := database.SaveMacroAxes(proj.ID, key, &p1, nil); err != nil {
			t.Fatal(err)
		}
	}
	pending, err := database.PendingHorizonPushes(t.Context(), proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].Key != "PE-1" {
		t.Errorf("pending = %v, want the own epic alone", pending)
	}
}

// saveLine gives macro PE-460 one slicing line and returns its id.
func saveLine(t *testing.T, database *DB, projectID string, line models.MacroTodo) string {
	t.Helper()
	todos := []models.MacroTodo{line}
	meta, err := database.SaveMacroMeta(projectID, "PE-460", nil, nil, nil, &todos)
	if err != nil {
		t.Fatal(err)
	}
	return meta.Todos[0].ID
}

func TestALineKeepsOneTargetOnly(t *testing.T) {
	database, proj := declaredProject(t, newDeclaredTracker(nil), "DATA")
	todos := []models.MacroTodo{{Text: "Do it", TargetProjectID: "other", TargetTrackerProject: " data "}}
	meta, err := database.SaveMacroMeta(proj.ID, "PE-460", nil, nil, nil, &todos)
	if err != nil {
		t.Fatal(err)
	}
	if got := meta.Todos[0]; got.TargetTrackerProject != "DATA" || got.TargetProjectID != "" {
		t.Errorf("line = %+v, want the tracker project alone", got)
	}
}

func TestALineCreatesItsStoryInADeclaredProject(t *testing.T) {
	fake := newDeclaredTracker(nil)
	database, proj := declaredProject(t, fake, "DATA")
	line := saveLine(t, database, proj.ID, models.MacroTodo{Text: "Export the data", TargetTrackerProject: "DATA"})

	meta, task, notice, err := database.CreateStoryFromMacroTodo(t.Context(), proj.ID, "PE-460", line)
	if err != nil {
		t.Fatalf("creation refused without the opt-in: %v", err)
	}
	if len(fake.created) != 1 || fake.created[0].Project.JiraProject != "DATA" || fake.created[0].Title != "Export the data" {
		t.Fatalf("created = %+v, want one story in DATA with the line's text", fake.created)
	}
	if fake.parents[task.Key] != "PE-460" {
		t.Errorf("parents = %v, want the epic as parent", fake.parents)
	}
	if task.ID != "" || !strings.HasPrefix(task.Key, "DATA-") {
		t.Errorf("task = %+v, want a DATA key and no local id", task)
	}
	if meta.Todos[0].StoryKey != task.Key {
		t.Errorf("line = %+v, want the created key recorded", meta.Todos[0])
	}
	if notice != "" {
		t.Errorf("notice = %q, want none when Jira took the story and its parent", notice)
	}
	tasks, _ := database.GetTasks("", "", "", "", proj.ID, "", "", "", "", nil, nil, false)
	if len(tasks) != 0 {
		t.Errorf("the story must not be imported, got %d tasks", len(tasks))
	}
	if _, _, _, err := database.CreateStoryFromMacroTodo(t.Context(), proj.ID, "PE-460", line); err == nil {
		t.Error("a second creation on the same line must be refused")
	}
	if len(fake.created) != 1 {
		t.Errorf("created = %d, the refusal must reach no tracker", len(fake.created))
	}
}

func TestALineAimedAtAnUndeclaredProjectIsRefusedBeforeAnyCall(t *testing.T) {
	fake := newDeclaredTracker(nil)
	database, proj := declaredProject(t, fake, "DATA")
	line := saveLine(t, database, proj.ID, models.MacroTodo{Text: "Export", TargetTrackerProject: "OPS"})
	if _, _, _, err := database.CreateStoryFromMacroTodo(t.Context(), proj.ID, "PE-460", line); err == nil || !strings.Contains(err.Error(), "n'est plus un projet de roadmap") {
		t.Fatalf("err = %v, want the undeclared key refused", err)
	}
	if len(fake.created) != 0 {
		t.Error("nothing may reach the tracker")
	}
}

func TestARefusedCreationRecordsNothingAndARefusedParentKeepsTheKey(t *testing.T) {
	fake := newDeclaredTracker(nil)
	fake.createErr = errors.New("400; fields this project requires on creation for Story: Team (customfield_1)")
	database, proj := declaredProject(t, fake, "DATA")
	line := saveLine(t, database, proj.ID, models.MacroTodo{Text: "Export", TargetTrackerProject: "DATA"})

	if _, _, _, err := database.CreateStoryFromMacroTodo(t.Context(), proj.ID, "PE-460", line); err == nil || !strings.Contains(err.Error(), "customfield_1") {
		t.Fatalf("err = %v, want Jira's refusal and the fields it asked for", err)
	}
	if got := macrosByKey(t, database, proj.ID)["PE-460"].Todos[0].StoryKey; got != "" {
		t.Errorf("story key = %q, a refused creation records nothing", got)
	}

	fake.createErr = nil
	fake.parentErr = errors.New("parent refused")
	_, task, notice, err := database.CreateStoryFromMacroTodo(t.Context(), proj.ID, "PE-460", line)
	if err != nil {
		t.Fatalf("a refused parent must not fail the creation: %v", err)
	}
	if !strings.Contains(notice, "non posé comme parent") {
		t.Errorf("notice = %q, want the refused parent named", notice)
	}
	if got := macrosByKey(t, database, proj.ID)["PE-460"].Todos[0].StoryKey; got != task.Key {
		t.Errorf("story key = %q, want %q kept", got, task.Key)
	}
}
