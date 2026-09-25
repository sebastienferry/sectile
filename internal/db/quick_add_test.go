package db

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"tasks/internal/models"
	"tasks/internal/secrets"
	"tasks/internal/tracker"
	"testing"
)

// creatingTracker files work items and records what it was asked to file.
type creatingTracker struct {
	tracker.BaseTicketingSystem
	got tracker.CreateIssueRequest
	as  string
	err error
}

func newCreatingTracker(name string) *creatingTracker {
	return &creatingTracker{BaseTicketingSystem: tracker.BaseTicketingSystem{TrackerName: name, Capabilities: []tracker.Capability{tracker.CapCreate}}}
}

func (f *creatingTracker) CreateIssue(ctx context.Context, req tracker.CreateIssueRequest) (*models.Task, error) {
	f.got, f.as = req, tracker.ActingUser(ctx)
	if f.err != nil {
		return nil, f.err
	}
	url := "https://tracker.example/browse/PE-42"
	return &models.Task{Key: "PE-42", ExternalURL: &url}, nil
}

func countTitled(t *testing.T, database *DB, title string) int {
	t.Helper()
	var count int
	if err := database.conn.QueryRow("SELECT COUNT(*) FROM tasks WHERE title = ?", title).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

// A request naming a tracker the project does not use is refused: the first
// project, which CreateTask falls back to, tracks locally, so a Jira request on
// it would otherwise have been filed as a local card.
func TestStrictRemoteCreationRejectsATrackerTheProjectDoesNotUse(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	_, err = database.CreateTask(models.CreateTaskRequest{Title: "Do not silently create locally", Source: "jira", RequireRemoteCreation: true})
	if err == nil {
		t.Fatal("a mismatched remote creation must fail")
	}
	if count := countTitled(t, database, "Do not silently create locally"); count != 0 {
		t.Fatal("task created despite remote failure", count)
	}
	task, err := database.CreateTask(models.CreateTaskRequest{Title: "Local task", Source: "local", RequireRemoteCreation: true})
	if err != nil || task == nil {
		t.Fatal("local projects must remain supported", err)
	}
}

func TestStrictRemoteCreationRejectsATrackerThatCannotCreate(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.TrackerRegistry().Register("readonly", &tracker.BaseTicketingSystem{TrackerName: "readonly"})
	for _, name := range []string{"gitlab", "readonly"} {
		project, err := database.CreateProject(models.CreateProjectRequest{Name: name, IssueTracker: name})
		if err != nil {
			t.Fatal(err)
		}
		title := "Filed on " + name
		if _, err := database.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: title, RequireRemoteCreation: true}); err == nil {
			t.Fatalf("%s: a tracker that cannot create must refuse the task", name)
		}
		if count := countTitled(t, database, title); count != 0 {
			t.Fatalf("%s: no local row may be written, found %d", name, count)
		}
	}
}

func TestStrictRemoteCreationForwardsTypeAndParentAsTheCaller(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	fake := newCreatingTracker("jira")
	database.TrackerRegistry().Register("jira", fake)
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Platform", IssueTracker: "jira", JiraProject: "PE"})
	if err != nil {
		t.Fatal(err)
	}

	ctx := tracker.WithActingUser(context.Background(), "usr_ada")
	task, err := database.CreateTaskAs(ctx, models.CreateTaskRequest{ProjectID: project.ID, Title: "Filed", IssueType: " Story ", ParentKey: " PE-10 ", RequireRemoteCreation: true})
	if err != nil {
		t.Fatal(err)
	}
	if fake.got.IssueType != "Story" || fake.got.ParentKey != "PE-10" {
		t.Fatalf("type and parent must reach the tracker: %+v", fake.got)
	}
	if fake.as != "usr_ada" {
		t.Fatalf("the tracker must be called as the caller, got %q", fake.as)
	}
	if task.Key != "PE-42" || task.Status != models.StatusToClarify {
		t.Fatalf("created task: %+v", task)
	}

	// A sealed credential stays recognisable through the wrapping.
	fake.err = secrets.ErrSealed
	_, err = database.CreateTaskAs(ctx, models.CreateTaskRequest{ProjectID: project.ID, Title: "Sealed", RequireRemoteCreation: true})
	if !errors.Is(err, secrets.ErrSealed) || !strings.HasPrefix(err.Error(), "Jira issue creation failed: ") {
		t.Fatalf("sealed credential: %v", err)
	}
	if count := countTitled(t, database, "Sealed"); count != 0 {
		t.Fatalf("no local row may be written on a refusal, found %d", count)
	}
}
