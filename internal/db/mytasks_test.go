package db

import (
	"slices"
	"testing"

	"tasks/internal/models"
)

// myTasksFixture is a board with one GitHub, one Jira and one local project,
// whose tickets are assigned the way each tracker writes people.
type myTasksFixture struct {
	db                  *DB
	github, jira, local string
	me                  MyTasks
}

func newMyTasksFixture(t *testing.T) myTasksFixture {
	t.Helper()
	database, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	return seedMyTasksFixture(t, database)
}

// seedMyTasksFixture fills any engine's database, so that the PostgreSQL smoke
// test checks the same matching as the SQLite suite.
func seedMyTasksFixture(t *testing.T, database *DB) myTasksFixture {
	t.Helper()
	f := myTasksFixture{db: database}
	for _, target := range []struct {
		name, slug, tracker string
		id                  *string
	}{{"Hub", "hub", "github", &f.github}, {"Jay", "jay", "jira", &f.jira}, {"Loc", "loc", "local", &f.local}} {
		p, err := database.CreateProject(models.CreateProjectRequest{Name: target.name, Slug: target.slug, IssueTracker: target.tracker})
		if err != nil {
			t.Fatalf("CreateProject(%s): %v", target.name, err)
		}
		*target.id = p.ID
	}
	imported := []models.Task{
		{ProjectID: f.github, Key: "#1", Title: "gh mine", Source: "github", Assignee: "sebastienferry"},
		{ProjectID: f.github, Key: "#2", Title: "gh alice", Source: "github", Assignee: "alice"},
		{ProjectID: f.github, Key: "#3", Title: "gh mine spaced", Source: "github", Assignee: "SebastienFerry "},
		{ProjectID: f.github, Key: "#4", Title: "gh by name", Source: "github", Assignee: "Sébastien F.", Priority: models.PriorityHigh},
		{ProjectID: f.jira, Key: "PE-1", Title: "jira mine", Source: "jira", Assignee: "Sébastien Ferry"},
		{ProjectID: f.jira, Key: "PE-2", Title: "jira by name", Source: "jira", Assignee: "Sébastien F."},
		{ProjectID: f.jira, Key: "PE-3", Title: "jira by mail", Source: "jira", Assignee: "sferry@example.com"},
		{ProjectID: f.local, Key: "L-1", Title: "local by name", Source: "local", Assignee: "Sébastien F."},
		{ProjectID: f.local, Key: "L-2", Title: "local by mail", Source: "local", Assignee: "SFerry@Example.com"},
		{ProjectID: f.local, Key: "L-3", Title: "local by login", Source: "local", Assignee: "sebastienferry"},
		{ProjectID: f.local, Key: "L-4", Title: "local nobody", Source: "local"},
	}
	for i := range imported {
		imported[i].Status = models.StatusToClarify
		if imported[i].Priority == "" {
			imported[i].Priority = models.PriorityMedium
		}
	}
	if err := database.ImportOrUpdateTasks(imported); err != nil {
		t.Fatalf("ImportOrUpdateTasks: %v", err)
	}
	f.me = MyTasks{
		ByTracker: map[string]string{"github": "sebastienferry", "jira": "Sébastien Ferry"},
		Fallback:  []string{"Sébastien F.", "sferry@example.com"},
	}
	return f
}

func (f myTasksFixture) titles(t *testing.T, scope TaskScope, priority string) []string {
	t.Helper()
	tasks, err := f.db.GetTasksInScope(scope, "", "", priority, "", "", "", "", "", nil, nil, false)
	if err != nil {
		t.Fatalf("GetTasksInScope: %v", err)
	}
	titles := make([]string, 0, len(tasks))
	for _, task := range tasks {
		titles = append(titles, task.Title)
	}
	slices.Sort(titles)
	return titles
}

func assertTitles(t *testing.T, got []string, want ...string) {
	t.Helper()
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func checkMyTasksMatching(t *testing.T, f myTasksFixture) {
	t.Run("a GitHub project matches the login, trimmed and in any case", func(t *testing.T) {
		assertTitles(t, f.titles(t, TaskScope{ProjectID: f.github, Mine: &f.me}, ""), "gh mine", "gh mine spaced")
	})
	t.Run("a Jira project matches the display name, not the account name", func(t *testing.T) {
		assertTitles(t, f.titles(t, TaskScope{ProjectID: f.jira, Mine: &f.me}, ""), "jira mine")
	})
	t.Run("local tickets match the name and the e-mail, not a tracker login", func(t *testing.T) {
		assertTitles(t, f.titles(t, TaskScope{ProjectID: f.local, Mine: &f.me}, ""), "local by name", "local by mail")
	})
	t.Run("every project at once keeps each tracker's own", func(t *testing.T) {
		assertTitles(t, f.titles(t, TaskScope{Mine: &f.me}, ""),
			"gh mine", "gh mine spaced", "jira mine", "local by name", "local by mail")
	})
	t.Run("a tracker with no known identity falls back on the name and e-mail", func(t *testing.T) {
		githubOnly := MyTasks{ByTracker: map[string]string{"github": "sebastienferry"}, Fallback: f.me.Fallback}
		assertTitles(t, f.titles(t, TaskScope{ProjectID: f.jira, Mine: &githubOnly}, ""), "jira by name", "jira by mail")
	})
	t.Run("My Tasks narrows the other filters and is narrowed by them", func(t *testing.T) {
		nameOnly := MyTasks{Fallback: []string{"Sébastien F."}}
		assertTitles(t, f.titles(t, TaskScope{ProjectID: f.github, Mine: &nameOnly}, string(models.PriorityHigh)), "gh by name")
		assertTitles(t, f.titles(t, TaskScope{ProjectID: f.github, Mine: &f.me}, string(models.PriorityHigh)))
	})
	t.Run("nobody to match keeps nothing", func(t *testing.T) {
		assertTitles(t, f.titles(t, TaskScope{Mine: &MyTasks{Fallback: []string{"  "}}}, ""))
	})
	t.Run("no My Tasks filter keeps everything", func(t *testing.T) {
		if got := f.titles(t, TaskScope{ProjectID: f.local}, ""); len(got) != 4 {
			t.Fatalf("got %q, want the four local tickets", got)
		}
	})
}

func TestMyTasksMatching(t *testing.T) {
	checkMyTasksMatching(t, newMyTasksFixture(t))
}

func TestMyTasksInASavedView(t *testing.T) {
	f := newMyTasksFixture(t)
	name := "Hub and Jay"
	projects := []string{f.github, f.jira}
	view, err := f.db.CreateBoardView("u1", models.BoardViewRequest{Name: &name, ProjectIDs: &projects})
	if err != nil {
		t.Fatalf("CreateBoardView: %v", err)
	}
	assertTitles(t, f.titles(t, TaskScope{UserID: "u1", ViewID: view.ID, Mine: &f.me}, ""), "gh mine", "gh mine spaced", "jira mine")

	sources, err := f.db.TaskSourcesInScope(TaskScope{UserID: "u1", ViewID: view.ID})
	if err != nil || !slices.Equal(sources, []string{"github", "jira"}) {
		t.Fatalf("sources of the view: %q %v", sources, err)
	}
}

func TestTaskSourcesInScope(t *testing.T) {
	f := newMyTasksFixture(t)
	for _, c := range []struct {
		scope TaskScope
		want  []string
	}{
		{TaskScope{}, []string{"github", "jira", "local"}},
		{TaskScope{ProjectID: f.github}, []string{"github"}},
		{TaskScope{ProjectID: f.local}, []string{"local"}},
	} {
		got, err := f.db.TaskSourcesInScope(c.scope)
		if err != nil || !slices.Equal(got, c.want) {
			t.Errorf("TaskSourcesInScope(%+v) = %q %v, want %q", c.scope, got, err, c.want)
		}
	}
	if _, err := f.db.TaskSourcesInScope(TaskScope{UserID: "u1", ViewID: "nope"}); err == nil {
		t.Fatal("a view that is not the user's must be refused")
	}
}

// TestPostgresMyTasks runs the same matching on PostgreSQL, whose LOWER folds
// more than A-Z: both engines must agree on what matches.
func TestPostgresMyTasks(t *testing.T) {
	checkMyTasksMatching(t, seedMyTasksFixture(t, openPostgres(t)))
}
