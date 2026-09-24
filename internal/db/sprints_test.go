package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// sprintTracker is a Jira that owns its sprints: it records what it is asked
// and can be told to refuse the n-th creation or any update.
type sprintTracker struct {
	*fakeTracker
	mu        sync.Mutex
	requests  []tracker.SprintCreateRequest
	failAt    int // 1-based creation to refuse, 0 for none
	updateErr error
	moved     map[string][]string // sprint id ("" for the backlog) -> keys
	order     []string            // "move" and "update", in call order
	deleted   []string
	nextID    int
}

func newSprintTracker() *sprintTracker {
	fake := newFakeTracker()
	fake.Capabilities = append(fake.Capabilities, tracker.CapSprintManage)
	return &sprintTracker{fakeTracker: fake, moved: map[string][]string{}, nextID: 100}
}

func (s *sprintTracker) CreateSprint(ctx context.Context, req tracker.SprintCreateRequest) (models.TrackerSprint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, req)
	if s.failAt == len(s.requests) {
		return models.TrackerSprint{}, errors.New("400 sprint limit reached")
	}
	s.nextID++
	return models.TrackerSprint{ID: fmt.Sprint(s.nextID), Name: req.Name, State: "future",
		StartDate: req.Start.Format(time.RFC3339), EndDate: req.End.Format(time.RFC3339)}, nil
}

func (s *sprintTracker) UpdateSprint(ctx context.Context, project *models.Project, id string, patch models.SprintPatch) (models.TrackerSprint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.order = append(s.order, "update")
	if s.updateErr != nil {
		return models.TrackerSprint{}, s.updateErr
	}
	sprint := models.TrackerSprint{ID: id, Name: "Sprint " + id, State: "active"}
	if patch.Name != nil {
		sprint.Name = *patch.Name
	}
	if patch.State != nil {
		sprint.State = *patch.State
	}
	return sprint, nil
}

func (s *sprintTracker) DeleteSprint(ctx context.Context, project *models.Project, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleted = append(s.deleted, id)
	return nil
}

func (s *sprintTracker) SetSprint(ctx context.Context, sprintID string, keys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.order = append(s.order, "move")
	s.moved[sprintID] = append(s.moved[sprintID], keys...)
	return nil
}

func sprintDB(t *testing.T, boardID string) (*DB, *sprintTracker, *models.Project) {
	t.Helper()
	fake := newSprintTracker()
	database, project := jiraTestDB(t, fake.fakeTracker)
	database.TrackerRegistry().Register("jira", fake)
	if boardID != "" {
		if _, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{BoardID: &boardID}); err != nil {
			t.Fatal(err)
		}
	}
	return database, fake, project
}

func TestSprintBatchIsCreatedBackToBack(t *testing.T) {
	database, fake, project := sprintDB(t, "5")
	start := time.Date(2026, 10, 5, 9, 0, 0, 0, time.Local)
	created, err := database.CreateProjectSprints(context.Background(), project.ID, "Sprint {n}", 3, start, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 3 || created[0].Name != "Sprint 1" || created[2].Name != "Sprint 3" {
		t.Fatalf("created %+v", created)
	}
	wantStarts := []string{"2026-10-05", "2026-10-19", "2026-11-02"}
	wantEnds := []string{"2026-10-19", "2026-11-02", "2026-11-16"}
	for i, req := range fake.requests {
		if req.BoardID != "5" || req.Start.Format("2006-01-02") != wantStarts[i] {
			t.Errorf("sprint %d starts %s on board %s", i+1, req.Start, req.BoardID)
		}
		// Each sprint ends one second before the next one starts.
		if req.End.Add(time.Second).Format("2006-01-02") != wantEnds[i] {
			t.Errorf("sprint %d ends %s", i+1, req.End)
		}
	}
	reread, _ := database.GetProjectByID(project.ID)
	if len(reread.Sprints) != 3 || reread.Sprints[0].ID != created[0].ID {
		t.Fatalf("the mirror must hold Jira's answer, got %+v", reread.Sprints)
	}
}

func TestSprintBatchNaming(t *testing.T) {
	start := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		pattern      string
		index, count int
		want         string
	}{
		{"Sprint {n}", 2, 3, "Sprint 2"},
		{"Q4", 1, 1, "Q4"},
		{"Q4", 2, 3, "Q4 2"},
		{"", 1, 1, "Sprint 2026-10-05"},
	}
	for _, c := range cases {
		if got := sprintBatchName(c.pattern, c.index, c.count, start); got != c.want {
			t.Errorf("sprintBatchName(%q, %d, %d) = %q, want %q", c.pattern, c.index, c.count, got, c.want)
		}
	}
}

func TestSprintBatchIsRefusedBeforeAnyWrite(t *testing.T) {
	database, fake, project := sprintDB(t, "5")
	fake.sprints = []models.TrackerSprint{{ID: "9", Name: " sprint 2 ", State: "active"}}
	start := time.Now()
	for _, c := range []struct {
		count, weeks int
		want         string
	}{{0, 2, "de 1 à 12"}, {13, 2, "de 1 à 12"}, {3, 0, "semaines"}, {3, 5, "semaines"}, {3, 2, "existe déjà"}} {
		if _, err := database.CreateProjectSprints(context.Background(), project.ID, "Sprint {n}", c.count, start, c.weeks); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("count %d weeks %d: got %v, want %q", c.count, c.weeks, err, c.want)
		}
	}
	if len(fake.requests) != 0 {
		t.Fatalf("nothing may be created, got %d request(s)", len(fake.requests))
	}
}

func TestSprintBatchNeedsABoard(t *testing.T) {
	database, _, project := sprintDB(t, "")
	if _, err := database.CreateProjectSprints(context.Background(), project.ID, "S", 1, time.Now(), 2); err == nil || !strings.Contains(err.Error(), "board") {
		t.Fatalf("a project without a board must be refused, got %v", err)
	}
}

func TestSprintBatchKeepsWhatWasCreatedBeforeAFailure(t *testing.T) {
	database, fake, project := sprintDB(t, "5")
	fake.failAt = 3
	created, err := database.CreateProjectSprints(context.Background(), project.ID, "Sprint {n}", 3, time.Now(), 1)
	if err == nil || !strings.Contains(err.Error(), "2 sprint(s) créé(s), puis échec sur « Sprint 3 »") || !strings.Contains(err.Error(), "limit reached") {
		t.Fatalf("the failure must say what was created and why, got %v", err)
	}
	reread, _ := database.GetProjectByID(project.ID)
	if len(created) != 2 || len(reread.Sprints) != 2 {
		t.Fatalf("the two created sprints must be kept and mirrored: %+v %+v", created, reread.Sprints)
	}
}

func TestClosingASprintMovesItsOpenWorkFirst(t *testing.T) {
	database, fake, project := sprintDB(t, "5")
	sprints := []models.TrackerSprint{
		{ID: "1", Name: "Sprint 1", State: "active", StartDate: "2026-10-05T09:00:00+02:00"},
		{ID: "2", Name: "Sprint 2", State: "future", StartDate: "2026-10-19T09:00:00+02:00"},
	}
	if _, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{Sprints: &sprints}); err != nil {
		t.Fatal(err)
	}
	open, err := database.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "Open", Source: "local"})
	if err != nil {
		t.Fatal(err)
	}
	finished, _ := database.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "Finished", Source: "local"})
	boardOnly, _ := database.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "Board only", Source: "local"})
	database.mu.Lock()
	_, _ = database.conn.Exec("UPDATE tasks SET sprint = 'Sprint 1' WHERE id IN (?, ?, ?)", open.ID, finished.ID, boardOnly.ID)
	_, _ = database.conn.Exec("UPDATE tasks SET source = 'jira' WHERE id IN (?, ?)", open.ID, finished.ID)
	_, _ = database.conn.Exec("UPDATE tasks SET status = ? WHERE id = ?", string(models.StatusFinished), finished.ID)
	database.mu.Unlock()

	closed, next := "closed", "next"
	sprint, err := database.UpdateProjectSprint(context.Background(), project.ID, "1", models.SprintPatch{State: &closed, MoveOpenTo: &next})
	if err != nil {
		t.Fatal(err)
	}
	if sprint.State != "closed" || len(fake.order) != 2 || fake.order[0] != "move" || fake.order[1] != "update" {
		t.Fatalf("the open work moves before the close: %v %+v", fake.order, sprint)
	}
	if keys := fake.moved["2"]; len(keys) != 1 || keys[0] != open.Key {
		t.Fatalf("only the open work item moves to the next sprint, got %v", fake.moved)
	}
	moved, _ := database.GetTaskByID(open.ID)
	if moved.Sprint != "Sprint 2" {
		t.Fatalf("the moved work item must name its new sprint, got %q", moved.Sprint)
	}
	// A card that exists on the board alone is not sent to Jira, and follows.
	local, _ := database.GetTaskByID(boardOnly.ID)
	if local.Sprint != "Sprint 2" {
		t.Fatalf("a board-only card follows locally, got %q", local.Sprint)
	}
}

func TestClosingASprintThatNeverStartedMovesNothing(t *testing.T) {
	database, fake, project := sprintDB(t, "5")
	sprints := []models.TrackerSprint{
		{ID: "1", Name: "Sprint 1", State: "future", StartDate: "2026-10-05T09:00:00+02:00"},
		{ID: "2", Name: "Sprint 2", State: "future"},
	}
	if _, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{Sprints: &sprints}); err != nil {
		t.Fatal(err)
	}
	closed, next := "closed", "next"
	if _, err := database.UpdateProjectSprint(context.Background(), project.ID, "1", models.SprintPatch{State: &closed, MoveOpenTo: &next}); err == nil || !strings.Contains(err.Error(), "démarré") {
		t.Fatalf("closing a sprint that never started must be refused first, got %v", err)
	}
	if len(fake.order) != 0 {
		t.Fatalf("nothing may be moved or closed, got %v", fake.order)
	}
}

func TestAnUndatedFutureSprintIsNext(t *testing.T) {
	active := models.TrackerSprint{ID: "1", Name: "Sprint 1", State: "active", StartDate: "2026-10-05T09:00:00Z"}
	undated := models.TrackerSprint{ID: "2", Name: "Sprint 2", State: "future"}
	if next, ok := nextSprint([]models.TrackerSprint{active, undated}, active); !ok || next.ID != "2" {
		t.Fatalf("an undated future sprint must be next when no dated one follows, got %+v %v", next, ok)
	}
}

func TestARefusedSprintUpdateLeavesTheMirror(t *testing.T) {
	database, fake, project := sprintDB(t, "5")
	sprints := []models.TrackerSprint{{ID: "1", Name: "Sprint 1", State: "active"}}
	if _, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{Sprints: &sprints}); err != nil {
		t.Fatal(err)
	}
	fake.updateErr = errors.New("Sprint cannot be closed: 403 no permission.")
	closed := "closed"
	if _, err := database.UpdateProjectSprint(context.Background(), project.ID, "1", models.SprintPatch{State: &closed}); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("Jira's reason must reach the caller, got %v", err)
	}
	reread, _ := database.GetProjectByID(project.ID)
	if reread.Sprints[0].State != "active" {
		t.Fatalf("the mirror must be unchanged, got %+v", reread.Sprints)
	}
	// Moving the open work without closing is refused outright.
	next := "next"
	if _, err := database.UpdateProjectSprint(context.Background(), project.ID, "1", models.SprintPatch{MoveOpenTo: &next}); err == nil {
		t.Fatal("moveOpenTo without closing must be refused")
	}
}

func TestDeletingASprintForgetsIt(t *testing.T) {
	database, fake, project := sprintDB(t, "5")
	sprints := []models.TrackerSprint{{ID: "1", Name: "Sprint 1"}, {ID: "2", Name: "Sprint 2"}}
	if _, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{Sprints: &sprints}); err != nil {
		t.Fatal(err)
	}
	card, _ := database.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "In sprint 1", Source: "local"})
	database.mu.Lock()
	_, _ = database.conn.Exec("UPDATE tasks SET sprint = 'Sprint 1' WHERE id = ?", card.ID)
	database.mu.Unlock()
	if err := database.DeleteProjectSprint(context.Background(), project.ID, "1"); err != nil {
		t.Fatal(err)
	}
	if back, _ := database.GetTaskByID(card.ID); back.Sprint != "" {
		t.Fatalf("a deleted sprint's cards go back to the backlog, got %q", back.Sprint)
	}
	// A sprint this project does not know is never sent to the tracker.
	if err := database.DeleteProjectSprint(context.Background(), project.ID, "999"); err != nil || len(fake.deleted) != 1 {
		t.Fatalf("an unknown sprint is already gone for this project: %v %v", err, fake.deleted)
	}
	reread, _ := database.GetProjectByID(project.ID)
	if len(fake.deleted) != 1 || len(reread.Sprints) != 1 || reread.Sprints[0].ID != "2" {
		t.Fatalf("deleted %v, mirror %+v", fake.deleted, reread.Sprints)
	}
}

func TestAProjectWithoutManagedSprintsIsRefused(t *testing.T) {
	database, err := NewDB(t.TempDir() + "/tasks.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Repo", Slug: "repo", IssueTracker: "github", GithubRepo: "org/repo"})
	if err != nil {
		t.Fatal(err)
	}
	var unsupported *tracker.ErrUnsupported
	if _, err := database.CreateProjectSprints(context.Background(), project.ID, "S", 1, time.Now(), 2); !errors.As(err, &unsupported) {
		t.Fatalf("GitHub manages no sprint, got %v", err)
	}
}
