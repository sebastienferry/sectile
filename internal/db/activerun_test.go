package db

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tasks/internal/models"
	"tasks/internal/skills"
	"tasks/internal/tracker"
)

func activeRunDB(t *testing.T) (*DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "activerun.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	seedProjectAndUser(t, d)
	seedTask(t, d)
	return d, path
}

func addRun(d *DB, id, skillID, status string, concurrent bool) error {
	now := time.Now()
	act := models.TaskActivity{ID: id, TaskID: "t1", SkillID: skillID, SkillName: skillID, Status: status, CreatedAt: now, Concurrent: concurrent}
	if status != "queued" {
		act.StartedAt = &now
	}
	return d.AddTaskActivity(act)
}

// The index refuses a second ordinary active run, queued included, and nothing
// else: a launch record, a tracker write, a finished run, a concurrent run and
// a project activity never count.
func TestTheIndexAllowsOneOrdinaryActiveRun(t *testing.T) {
	d, _ := activeRunDB(t)
	if err := addRun(d, "queued", "clarify", "queued", false); err != nil {
		t.Fatal(err)
	}
	if err := addRun(d, "second", "remote_run", "running", false); !isUniqueViolation(err, activeRunIndex) {
		t.Fatalf("a second ordinary run was not refused by the index: %v", err)
	}
	for _, ok := range []struct{ id, skill, status string }{
		{"launch", "agent_launch", "running"},
		{"write", "tracker_op", "queued"},
		{"done", "implement", "completed"},
	} {
		if err := addRun(d, ok.id, ok.skill, ok.status, false); err != nil {
			t.Fatalf("%s was refused: %v", ok.id, err)
		}
	}
	if err := addRun(d, "forced", "remote_run", "running", true); err != nil {
		t.Fatalf("a concurrent run was refused: %v", err)
	}
	for _, id := range []string{"sync-a", "sync-b"} {
		if err := d.AddTaskActivity(models.TaskActivity{ID: id, ProjectID: "p1", SkillID: "clarify", Status: "running", CreatedAt: time.Now()}); err != nil {
			t.Fatalf("project activity %s was refused: %v", id, err)
		}
	}
	// A repeated id is a primary key collision, not a busy task.
	if err := addRun(d, "launch", "agent_launch", "running", false); err == nil || isUniqueViolation(err, activeRunIndex) {
		t.Fatalf("a primary key collision read as a busy task: %v", err)
	}

	active, err := d.ActiveRunOnTask("t1")
	if err != nil || active == nil || active.ID != "forced" {
		t.Fatalf("a running run must be reported before a queued one: %+v %v", active, err)
	}
}

// A refused queued launch answers ErrTaskBusy with the run in the way, and
// queues nothing.
func TestEnqueueOnABusyTaskIsRefused(t *testing.T) {
	d, _ := activeRunDB(t)
	if err := addRun(d, "running", "remote_run", "running", false); err != nil {
		t.Fatal(err)
	}
	_, _, err := d.EnqueueSkillOnTask("t1", "clarify", "")
	var busy *TaskBusyError
	if !errors.Is(err, ErrTaskBusy) || !errors.As(err, &busy) || busy.Active == nil || busy.Active.ID != "running" {
		t.Fatalf("enqueue on a busy task: %v", err)
	}
	var queued int
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM task_activities WHERE task_id = 't1' AND status = 'queued'").Scan(&queued); err != nil || queued != 0 {
		t.Fatalf("a refused enqueue left %d queued rows (%v)", queued, err)
	}
}

// "Launch anyway" and a client declaring its own run stay possible next to an
// active run (#308); an ordinary agent launch does not.
func TestForcedAndClientRunsAreConcurrent(t *testing.T) {
	d, _ := activeRunDB(t)
	first, err := d.StartAgentRun("t1", "implement", RunLaunch{})
	if err != nil || first.Concurrent {
		t.Fatalf("first launch: %+v %v", first, err)
	}
	if _, err := d.StartAgentRun("t1", "implement", RunLaunch{}); !errors.Is(err, ErrTaskBusy) {
		t.Fatalf("a second ordinary launch: %v", err)
	}
	forced, err := d.StartAgentRun("t1", "implement", RunLaunch{Force: true})
	if err != nil || !forced.Concurrent {
		t.Fatalf("a forced launch: %+v %v", forced, err)
	}
	client, err := d.StartRemoteRunBy("u1", "t1", "clarify", "")
	if err != nil || !client.Concurrent {
		t.Fatalf("a client-declared run: %+v %v", client, err)
	}
	reported, err := d.SyncRemoteRunStatus("agent-run", "t1", "p1", "#1", "clarify", "running", "", nil)
	if err != nil || !reported.Concurrent {
		t.Fatalf("an agent-reported run: %+v %v", reported, err)
	}
}

// Every skill the catalog can queue on a task is a run for the index. A skill
// added to the catalog without a migration recreating the index fails here.
func TestActiveRunSkillsCoverTheCatalog(t *testing.T) {
	known := map[string]bool{}
	for _, id := range activeRunSkillIDs {
		known[id] = true
	}
	for _, s := range skills.StageSkills {
		if !known[s.ID] {
			t.Errorf("catalog skill %q is missing from activeRunSkillIDs and from the index", s.ID)
		}
	}
}

// An existing database whose task already holds two active runs upgrades: the
// remote run stays ordinary, the other becomes concurrent, and the index exists.
func TestMigrationNineKeepsSurplusRunsAsConcurrent(t *testing.T) {
	d, path := activeRunDB(t)
	for _, stmt := range []string{
		"DROP INDEX idx_activities_one_active_run",
		"ALTER TABLE task_activities DROP COLUMN concurrent",
		// Nor anything the migrations after 9 add, which reopening replays.
		"DROP INDEX idx_task_activities_macro_running",
		"DROP INDEX idx_task_activities_macro",
		"ALTER TABLE task_activities DROP COLUMN macro_key",
		"ALTER TABLE projects DROP COLUMN roadmap_projects",
		"ALTER TABLE web_sessions DROP COLUMN last_seen_at",
		"ALTER TABLE projects DROP COLUMN repositories",
		"ALTER TABLE projects DROP COLUMN repositories_migration",
		"ALTER TABLE tasks DROP COLUMN repository",
		"ALTER TABLE tasks DROP COLUMN changed_repositories",
		"ALTER TABLE task_activities DROP COLUMN waiting_reason",
		"ALTER TABLE task_activities DROP COLUMN waiting_session",
		"DROP TABLE server_tracker_credentials",
		"ALTER TABLE projects ADD COLUMN github_token TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE projects ADD COLUMN gitlab_token TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE user_tracker_credentials DROP COLUMN account",
		"DELETE FROM schema_migrations WHERE version >= 9",
		`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action, status, created_at) VALUES
			('old-skill', 't1', 'clarify', 'clarify', 'run', 'running', '2026-09-01 10:00:00'),
			('old-run', 't1', 'remote_run', 'clarify', 'run', 'running', '2026-09-01 11:00:00'),
			('old-queued', 't1', 'implement', 'implement', 'run', 'queued', '2026-09-01 09:00:00'),
			('old-done', 't1', 'implement', 'implement', 'run', 'completed', '2026-09-01 08:00:00')`,
	} {
		if _, err := d.conn.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	d.Close()

	reopened, err := NewDB(path)
	if err != nil {
		t.Fatalf("upgrading: %v", err)
	}
	defer reopened.Close()
	want := map[string]int{"old-run": 0, "old-skill": 1, "old-queued": 1, "old-done": 0}
	for id, concurrent := range want {
		var got int
		if err := reopened.conn.QueryRow("SELECT concurrent FROM task_activities WHERE id = ?", id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != concurrent {
			t.Errorf("%s: concurrent = %d, want %d", id, got, concurrent)
		}
	}
	// Opening a SQLite store also ends every interrupted run, so the index is
	// read from the schema rather than tried with an insert.
	var indexes int
	if err := reopened.conn.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?", activeRunIndex).Scan(&indexes); err != nil || indexes != 1 {
		t.Fatalf("the index is missing after the upgrade: %d %v", indexes, err)
	}
}

// The output limit counts characters, so a cut never splits one.
func TestRunOutputIsCutOnACharacterBoundary(t *testing.T) {
	d, _ := activeRunDB(t)
	if err := addRun(d, "r1", "remote_run", "running", true); err != nil {
		t.Fatal(err)
	}
	chunk := strings.Repeat("é", RemoteRunOutputLimit/2+3)
	for i := 0; i < 3; i++ {
		if err := d.AppendRemoteRunOutput("t1", "r1", chunk); err != nil {
			t.Fatal(err)
		}
	}
	run, _ := d.GetActivityByID("r1")
	body := strings.TrimSuffix(run.Output, remoteRunOutputTruncated)
	if body == run.Output || strings.Count(run.Output, remoteRunOutputTruncated) != 1 {
		t.Fatal("the output was not marked as truncated exactly once")
	}
	if len([]rune(body)) != RemoteRunOutputLimit || strings.Trim(body, "é") != "" {
		t.Fatalf("the kept output holds %d characters, or a split one", len([]rune(body)))
	}
	if err := d.AppendRemoteRunOutput("t1", "missing", "x"); err == nil {
		t.Fatal("appending to an unknown run must fail")
	}
}

// FR5 on the queued path: a session somebody declared by hand is concurrent,
// so the index lets it through, but the task is still busy for a new run.
func TestEnqueueNextToAConcurrentRunIsRefused(t *testing.T) {
	d, _ := activeRunDB(t)
	client, err := d.StartRemoteRunBy("u1", "t1", "clarify", "")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = d.EnqueueSkillOnTask("t1", "clarify", "")
	var busy *TaskBusyError
	if !errors.As(err, &busy) || busy.Active == nil || busy.Active.ID != client.ID {
		t.Fatalf("enqueue next to a client session: %v", err)
	}
}

// editingTracker edits the task while the issue is being created, as a person
// or an agent post-back would during a slow tracker call.
type editingTracker struct {
	tracker.BaseTicketingSystem
	edit func()
}

func (e *editingTracker) CreateIssue(ctx context.Context, req tracker.CreateIssueRequest) (*models.Task, error) {
	e.edit()
	return &models.Task{Key: "REM-1"}, nil
}

func convertibleTask(t *testing.T, d *DB) {
	t.Helper()
	if _, err := d.conn.Exec(`INSERT INTO tasks (id, project_id, key, title, status, priority, source) VALUES ('t2','p1','P-1','Before','backlog','medium','local')`); err != nil {
		t.Fatal(err)
	}
}

// An edit made during the tracker call neither erases the conversion claim nor
// is overwritten by the conversion.
func TestAnEditDuringAConversionKeepsBoth(t *testing.T) {
	d, _ := activeRunDB(t)
	convertibleTask(t, d)
	title := "Edited meanwhile"
	fake := &editingTracker{BaseTicketingSystem: tracker.BaseTicketingSystem{TrackerName: "remote", Capabilities: []tracker.Capability{tracker.CapCreate}}}
	fake.edit = func() {
		if _, err := d.UpdateTask("t2", models.UpdateTaskRequest{Title: &title}); err != nil {
			t.Errorf("editing during the conversion: %v", err)
		}
	}
	d.TrackerRegistry().Register("remote", fake)

	if _, err := d.ConvertTaskToRemote(context.Background(), "t2", "remote"); err != nil {
		t.Fatalf("conversion: %v", err)
	}
	task, _ := d.GetTaskByID("t2")
	if task.Key != "REM-1" || task.Source != "remote" || task.Title != title {
		t.Fatalf("task = %s / %s / %q", task.Key, task.Source, task.Title)
	}
}

// A claim left by a server that stopped mid-conversion expires instead of
// refusing the task forever; a fresh one still refuses.
func TestAStaleConversionClaimIsTakenOver(t *testing.T) {
	d, _ := activeRunDB(t)
	convertibleTask(t, d)
	fake := &editingTracker{BaseTicketingSystem: tracker.BaseTicketingSystem{TrackerName: "remote", Capabilities: []tracker.Capability{tracker.CapCreate}}, edit: func() {}}
	d.TrackerRegistry().Register("remote", fake)

	if _, err := d.conn.Exec(`UPDATE tasks SET source = 'converting', updated_at = ? WHERE id = 't2'`, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := d.ConvertTaskToRemote(context.Background(), "t2", "remote"); err == nil {
		t.Fatal("a conversion in progress must refuse a second one")
	}
	if _, err := d.conn.Exec(`UPDATE tasks SET updated_at = ? WHERE id = 't2'`, time.Now().Add(-convertClaimExpiry-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := d.ConvertTaskToRemote(context.Background(), "t2", "remote"); err != nil {
		t.Fatalf("a stale claim was not taken over: %v", err)
	}
}
