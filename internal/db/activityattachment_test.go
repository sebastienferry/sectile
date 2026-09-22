package db

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"tasks/internal/models"
)

// legacyActivitiesTable is task_activities as every database written before
// #310 declares it: task_id NOT NULL, carrying a made-up identifier for
// anything that is not a ticket, and no project_id at all.
const legacyActivitiesTable = `CREATE TABLE task_activities (
	id TEXT PRIMARY KEY,
	task_id TEXT NOT NULL,
	skill_id TEXT NOT NULL,
	skill_name TEXT NOT NULL,
	action TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'completed',
	summary TEXT NOT NULL DEFAULT '',
	output TEXT NOT NULL DEFAULT '',
	steps TEXT NOT NULL DEFAULT '[]',
	prompt TEXT NOT NULL DEFAULT '',
	started_at DATETIME,
	completed_at DATETIME,
	error TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	user_id TEXT NOT NULL DEFAULT '',
	run_provider TEXT NOT NULL DEFAULT '',
	run_model TEXT NOT NULL DEFAULT '',
	run_mode TEXT NOT NULL DEFAULT '',
	launch_stage TEXT NOT NULL DEFAULT '',
	chain_stop_stage TEXT NOT NULL DEFAULT '',
	waiting_since DATETIME
);`

// openLegacyDatabase writes a database at the pre-#310 schema, holding one row
// of each kind the backfill has to deal with, and returns its path. The file is
// closed: the migration is what the caller opens it to observe.
func openLegacyDatabase(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "legacy.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatalf("creating the database: %v", err)
	}
	for _, stmt := range []string{
		`DELETE FROM projects`,
		`INSERT INTO projects (id, name, slug) VALUES ('p1', 'P1', 'p1')`,
		`INSERT INTO tasks (id, project_id, key, title) VALUES ('t1', 'p1', 'K-1', 'A ticket')`,
		`DROP TABLE task_activities`,
		legacyActivitiesTable,
		`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action) VALUES ('a-project', 'sync-p1', 'sync_github', 'Sync', 'Sync')`,
		`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action) VALUES ('a-all', 'sync-all', 'sync_all', 'Sync', 'Sync')`,
		`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action) VALUES ('a-github', 'sync-github', 'sync_github', 'Sync', 'Sync')`,
		`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action) VALUES ('a-gone', 'sync-deleted-project', 'sync_jira', 'Sync', 'Sync')`,
		`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action) VALUES ('a-task', 't1', 'clarify', 'Clarify', 'Clarify')`,
		`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action) VALUES ('a-spec', 'spec-framework-p1', 'install_spec_framework', 'Install', 'Install')`,
		`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action) VALUES ('a-op', 'tracker-op-p1', 'tracker_op', 'Écriture', 'Écriture')`,
	} {
		if _, err := d.conn.Exec(stmt); err != nil {
			t.Fatalf("seeding the legacy database (%s): %v", stmt, err)
		}
	}
	forgetSchemaVersion(t, d)
	if err := d.Close(); err != nil {
		t.Fatalf("closing the legacy database: %v", err)
	}
	return path
}

// attachment is how one row ended up, read straight from the table so the test
// sees the storage rather than a reader's interpretation of it.
func attachment(t *testing.T, d *DB, id string) (taskID, projectID string) {
	t.Helper()
	if err := d.conn.QueryRow(
		`SELECT COALESCE(task_id, ''), COALESCE(project_id, '') FROM task_activities WHERE id = ?`, id,
	).Scan(&taskID, &projectID); err != nil {
		t.Fatalf("reading activity %q: %v", id, err)
	}
	return taskID, projectID
}

// The migration is the one part of #310 that touches data somebody already has.
// Every row survives it: a suffix naming a project becomes a project activity,
// everything else becomes a global one, and a real ticket is left alone.
func TestMigrationMovesSyncRowsWithoutLosingAny(t *testing.T) {
	path := openLegacyDatabase(t)

	d, err := NewDB(path)
	if err != nil {
		t.Fatalf("reopening the database: %v", err)
	}
	defer d.Close()

	var count int
	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM task_activities`).Scan(&count); err != nil {
		t.Fatalf("counting activities: %v", err)
	}
	if count != 7 {
		t.Fatalf("%d activity(ies) after the migration, want 7: the migration deletes nothing", count)
	}

	for _, tc := range []struct {
		id, wantTask, wantProject string
	}{
		{"a-project", "", "p1"},
		{"a-all", "", ""},
		{"a-github", "", ""},
		{"a-gone", "", ""},
		{"a-task", "t1", ""},
		{"a-spec", "", "p1"},
		{"a-op", "", "p1"},
	} {
		gotTask, gotProject := attachment(t, d, tc.id)
		if gotTask != tc.wantTask || gotProject != tc.wantProject {
			t.Errorf("%s: task_id = %q, project_id = %q, want %q and %q", tc.id, gotTask, gotProject, tc.wantTask, tc.wantProject)
		}
	}

	history, err := d.GetProjectActivities("p1")
	if err != nil {
		t.Fatalf("GetProjectActivities: %v", err)
	}
	inHistory := map[string]bool{}
	for _, a := range history {
		inHistory[a.ID] = true
	}
	for _, id := range []string{"a-project", "a-spec", "a-op"} {
		if !inHistory[id] {
			t.Errorf("%s is not in its project's history after the migration: %v", id, inHistory)
		}
	}
}

// Starting again changes nothing: the shape is already there, and every
// statement of the backfill is written to find nothing left to do.
func TestMigrationIsANoOpOnASecondStart(t *testing.T) {
	path := openLegacyDatabase(t)

	first, err := NewDB(path)
	if err != nil {
		t.Fatalf("first start: %v", err)
	}
	forgetSchemaVersion(t, first)
	first.Close()

	second, err := NewDB(path)
	if err != nil {
		t.Fatalf("second start: %v", err)
	}
	defer second.Close()

	for _, tc := range []struct{ id, wantTask, wantProject string }{
		{"a-project", "", "p1"},
		{"a-task", "t1", ""},
	} {
		gotTask, gotProject := attachment(t, second, tc.id)
		if gotTask != tc.wantTask || gotProject != tc.wantProject {
			t.Errorf("%s moved on the second start: task_id = %q, project_id = %q", tc.id, gotTask, gotProject)
		}
	}
}

// The exclusivity of the two attachments is enforced by the database, not by a
// convention somebody has to remember. SQLite checks it even though this
// package leaves its foreign keys off, which is why the CHECK is what the
// ticket really asks for.
func TestTheDatabaseRefusesAnActivityCarryingBothAttachments(t *testing.T) {
	d, err := NewDB(filepath.Join(t.TempDir(), "check.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	if _, err := d.conn.Exec(`INSERT INTO projects (id, name, slug) VALUES ('p1','P1','p1')`); err != nil {
		t.Fatalf("seeding a project: %v", err)
	}
	if _, err := d.conn.Exec(`INSERT INTO tasks (id, project_id, key, title) VALUES ('t1','p1','K-1','T')`); err != nil {
		t.Fatalf("seeding a task: %v", err)
	}
	if _, err := d.conn.Exec(
		`INSERT INTO task_activities (id, task_id, project_id, skill_id, skill_name, action) VALUES ('both','t1','p1','s','S','A')`,
	); err == nil {
		t.Error("an activity carrying both a task and a project was accepted")
	}
}

// A project's activity history is what is attached to it, not what happens to
// mention it. The old filter matched a.prompt LIKE '%id%', so an activity of
// another project that merely named this one landed in it.
func TestTheProjectFilterMatchesTheAttachmentOnly(t *testing.T) {
	d, err := NewDB(filepath.Join(t.TempDir(), "filter.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	for _, stmt := range []string{
		`DELETE FROM projects`,
		`INSERT INTO projects (id, name, slug) VALUES ('p1','P1','slug-one')`,
		`INSERT INTO projects (id, name, slug) VALUES ('p2','P2','slug-two')`,
		`INSERT INTO tasks (id, project_id, key, title) VALUES ('t1','p1','K-1','T')`,
	} {
		if _, err := d.conn.Exec(stmt); err != nil {
			t.Fatalf("seeding (%s): %v", stmt, err)
		}
	}

	now := time.Now()
	for _, act := range []models.TaskActivity{
		{ID: "on-project", ProjectID: "p1", SkillID: "sync_github", SkillName: "Sync", Action: "Sync", CreatedAt: now},
		{ID: "on-task", TaskID: "t1", SkillID: "clarify", SkillName: "Clarify", Action: "Clarify", CreatedAt: now},
		{ID: "mentions-p1", ProjectID: "p2", SkillID: "sync_jira", SkillName: "Sync", Action: "Sync", Prompt: "p1", CreatedAt: now},
	} {
		if err := d.AddTaskActivity(act); err != nil {
			t.Fatalf("recording %s: %v", act.ID, err)
		}
	}

	for _, filter := range []string{"p1", "slug-one"} {
		list, err := d.GetActivities(filter, "", "", "", "", 0)
		if err != nil {
			t.Fatalf("GetActivities(%q): %v", filter, err)
		}
		seen := map[string]bool{}
		for _, a := range list {
			seen[a.ID] = true
		}
		if !seen["on-project"] || !seen["on-task"] {
			t.Errorf("filtering on %q returned %d activity(ies), missing the project's own: %v", filter, len(list), seen)
		}
		if seen["mentions-p1"] {
			t.Errorf("filtering on %q returned an activity of another project whose prompt merely names it", filter)
		}

		stats, err := d.GetActivityStats(filter)
		if err != nil {
			t.Fatalf("GetActivityStats(%q): %v", filter, err)
		}
		if stats.Total != len(list) {
			t.Errorf("filtering on %q: the statistics count %d activity(ies), the list returns %d", filter, stats.Total, len(list))
		}
	}
}

// The ticket's own acceptance criterion: a synchronisation launched for a
// project appears in that project's history. On PostgreSQL the same path is
// covered by TestPostgresActivityAttachedToAProject.
func TestASynchronisationAppearsInItsProjectHistory(t *testing.T) {
	d, err := NewDB(filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	if _, err := d.conn.Exec(`INSERT INTO projects (id, name, slug) VALUES ('p1','P1','p1')`); err != nil {
		t.Fatalf("seeding a project: %v", err)
	}

	act, err := d.EnqueueSync("github", "", "p1")
	if err != nil {
		t.Fatalf("EnqueueSync: %v", err)
	}
	if act.TaskID != "" || act.ProjectID != "p1" {
		t.Errorf("the queued activity carries task_id = %q and project_id = %q", act.TaskID, act.ProjectID)
	}

	gotTask, gotProject := attachment(t, d, act.ID)
	if gotTask != "" || gotProject != "p1" {
		t.Errorf("stored as task_id = %q, project_id = %q, want no task and project p1", gotTask, gotProject)
	}

	history, err := d.GetProjectActivities("p1")
	if err != nil {
		t.Fatalf("GetProjectActivities: %v", err)
	}
	if len(history) != 1 || history[0].ID != act.ID {
		t.Fatalf("the synchronisation left no trace in its project's history: %+v", history)
	}

	// A global synchronisation belongs to nobody, and stays out of a single
	// project's history.
	global, err := d.EnqueueSync("all", "", "")
	if err != nil {
		t.Fatalf("EnqueueSync (all): %v", err)
	}
	gotTask, gotProject = attachment(t, d, global.ID)
	if gotTask != "" || gotProject != "" {
		t.Errorf("a global synchronisation was attached to task %q / project %q", gotTask, gotProject)
	}
}

// The JSON contract is additive: a task_id that is now null still serialises as
// an empty string, so nothing reading taskId has to learn about null.
func TestTheSerialisedActivityKeepsItsContract(t *testing.T) {
	payload, err := json.Marshal(models.TaskActivity{ID: "a", ProjectID: "p1", Steps: []string{}})
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshalling: %v", err)
	}
	if got, ok := decoded["taskId"]; !ok || got != "" {
		t.Errorf("taskId = %#v, want the empty string: a null attachment must not reach the interface as null", got)
	}
	if decoded["projectId"] != "p1" {
		t.Errorf("projectId = %#v, want p1", decoded["projectId"])
	}
}
