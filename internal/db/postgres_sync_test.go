package db

import (
	"testing"
	"time"

	"tasks/internal/models"
)

// ImportOrUpdateTasks is the write path a tracker synchronisation takes. It
// inserts 28 columns in one statement, including nullable timestamps and the
// pinned flag, and it is the first thing that runs after a board is
// reconnected — so it is where a remaining engine difference shows up as
// "the sync did nothing".
func TestPostgresImportOrUpdateTasks(t *testing.T) {
	d := openPostgres(t)
	if _, err := d.conn.Exec(`INSERT INTO projects (id, name, slug, github_repo) VALUES ('p1','P','p','owner/repo')`); err != nil {
		t.Fatalf("seeding a project: %v", err)
	}

	now := time.Now().UTC()
	extURL := "https://github.com/owner/repo/issues/1"
	tasks := []models.Task{
		{
			ID:               "gh-p1-1",
			ProjectID:        "p1",
			Key:              "#1",
			Title:            "Imported",
			Description:      "from the tracker",
			Status:           "backlog",
			Priority:         "high",
			Labels:           []string{"bug"},
			Assignee:         "someone",
			Source:           "github",
			ExternalURL:      &extURL,
			IssueType:        "Bug",
			TrackerStatus:    "open",
			TrackerCreatedAt: &now,
			TrackerUpdatedAt: &now,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}

	if err := d.ImportOrUpdateTasks(tasks); err != nil {
		t.Fatalf("ImportOrUpdateTasks (insert): %v", err)
	}
	got, err := d.GetTasks("", "", "", "", "p1", "", "", "", "", nil, nil, false)
	if err != nil {
		t.Fatalf("GetTasks: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("%d task(s) after import, want 1 — the sync reported success but the board stayed empty", len(got))
	}

	// Second pass: the same ticket, changed. This is the update branch, which
	// carries the CASE WHEN ? != '' expressions.
	tasks[0].Title = "Imported and renamed"
	tasks[0].Status = "clarified"
	if err := d.ImportOrUpdateTasks(tasks); err != nil {
		t.Fatalf("ImportOrUpdateTasks (update): %v", err)
	}
	if got, err = d.GetTasks("", "", "", "", "p1", "", "", "", "", nil, nil, false); err != nil {
		t.Fatalf("GetTasks after update: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("%d task(s) after the second import, want 1", len(got))
	}
	if got[0].Title != "Imported and renamed" {
		t.Fatalf("title = %q, the update did not land", got[0].Title)
	}
}

// A synchronisation activity is not attached to a task: it belongs to the
// project it synchronises, or to nothing when it covers every tracker.
//
// Until #310 it was filed under a made-up "sync-<project>" task_id. PostgreSQL
// enforces foreign keys, so PR #304 had to drop the one on task_id for the row
// to go in at all — a synchronisation that ran and left no trace. Now the
// attachment has its own column, and the foreign key is back.
func TestPostgresActivityAttachedToAProject(t *testing.T) {
	d := openPostgres(t)
	if _, err := d.conn.Exec(`INSERT INTO projects (id, name, slug) VALUES ('p1','P','p')`); err != nil {
		t.Fatalf("seeding a project: %v", err)
	}

	act := models.TaskActivity{
		ID:        "sync-activity-1",
		ProjectID: "p1",
		SkillID:   "sync",
		SkillName: "Synchronisation",
		Action:    "Sync du projet",
		Status:    "completed",
		Summary:   "12 tickets importés",
		CreatedAt: time.Now().UTC(),
	}
	if err := d.AddTaskActivity(act); err != nil {
		t.Fatalf("AddTaskActivity for a project-level activity: %v", err)
	}

	got, err := d.GetProjectActivities("p1")
	if err != nil {
		t.Fatalf("GetProjectActivities: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("%d activity(ies), want 1 — the synchronisation left no trace", len(got))
	}
	if got[0].TaskID != "" {
		t.Errorf("task_id = %q, want empty: the activity belongs to no ticket", got[0].TaskID)
	}

	// The foreign key is enforced again, and so is the exclusivity of the two
	// attachments. Both are the point of the ticket, and PostgreSQL is the only
	// engine here that enforces the first.
	if _, err := d.conn.Exec(`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action) VALUES ('orphan', 'no-such-task', 's', 'S', 'A')`); err == nil {
		t.Error("an activity pointing at a task that does not exist was accepted")
	}
	if _, err := d.conn.Exec(`INSERT INTO task_activities (id, task_id, project_id, skill_id, skill_name, action) VALUES ('both', NULL, 'p1', 's', 'S', 'A')`); err != nil {
		t.Fatalf("a project activity was refused: %v", err)
	}
	if _, err := d.conn.Exec(`INSERT INTO tasks (id, project_id, key, title) VALUES ('t1', 'p1', 'K-1', 'T')`); err != nil {
		t.Fatalf("seeding a task: %v", err)
	}
	if _, err := d.conn.Exec(`INSERT INTO task_activities (id, task_id, project_id, skill_id, skill_name, action) VALUES ('mixed', 't1', 'p1', 's', 'S', 'A')`); err == nil {
		t.Error("an activity carrying both a task and a project was accepted")
	}
}

// The other three foreign keys point at users. A row written for a user that
// was never created would fail the same way, silently, on PostgreSQL only.
// This asserts the referenced row is required to exist — if the application
// ever files one of these under a synthetic id, this is where it shows up.
func TestPostgresUserOwnedRowsRequireTheirUser(t *testing.T) {
	d := openPostgres(t)

	for _, tc := range []struct {
		table string
		stmt  string
	}{
		{"web_sessions", `INSERT INTO web_sessions (token_hash, user_id, expires_at) VALUES ('h1', 'nobody', CURRENT_TIMESTAMP)`},
		{"device_credentials", `INSERT INTO device_credentials (id, user_id, token_hash) VALUES ('d1', 'nobody', 'h2')`},
	} {
		if _, err := d.conn.Exec(tc.stmt); err == nil {
			t.Errorf("%s accepted a row for a user that does not exist; if the application does this, it breaks on PostgreSQL", tc.table)
		}
	}
}

// The migration has to run on PostgreSQL too. A database created since PR #304
// holds the same made-up identifiers — that PR dropped the foreign key precisely
// so they would go in — and leaving them behind would make the restored foreign
// key impossible to create.
func TestPostgresMigratesLegacyActivityRows(t *testing.T) {
	d := openPostgres(t)
	dsn := postgresDSN(t)

	// Put the table back the way a pre-#310 database declares it, and fill it
	// with one row of each kind. RewriteDDL maps the types on the way through.
	for _, stmt := range []string{
		`INSERT INTO projects (id, name, slug) VALUES ('p1', 'P1', 'p1')`,
		`INSERT INTO tasks (id, project_id, key, title) VALUES ('t1', 'p1', 'K-1', 'A ticket')`,
		`DROP TABLE task_activities`,
		legacyActivitiesTable,
		`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action) VALUES ('a-project', 'sync-p1', 'sync_github', 'Sync', 'Sync')`,
		`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action) VALUES ('a-all', 'sync-all', 'sync_all', 'Sync', 'Sync')`,
		`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action) VALUES ('a-task', 't1', 'clarify', 'Clarify', 'Clarify')`,
	} {
		if _, err := d.conn.Exec(stmt); err != nil {
			t.Fatalf("seeding the legacy table (%s): %v", stmt, err)
		}
	}
	forgetSchemaVersion(t, d)
	d.Close()

	migrated, err := Open(Config{Driver: DriverPostgres, DSN: dsn})
	if err != nil {
		t.Fatalf("reopening the database: %v", err)
	}
	t.Cleanup(func() { migrated.Close() })

	var count int
	if err := migrated.conn.QueryRow(`SELECT COUNT(*) FROM task_activities`).Scan(&count); err != nil {
		t.Fatalf("counting activities: %v", err)
	}
	if count != 3 {
		t.Fatalf("%d activity(ies) after the migration, want 3: nothing is deleted", count)
	}
	for _, tc := range []struct{ id, wantTask, wantProject string }{
		{"a-project", "", "p1"},
		{"a-all", "", ""},
		{"a-task", "t1", ""},
	} {
		var gotTask, gotProject string
		if err := migrated.conn.QueryRow(
			`SELECT COALESCE(task_id, ''), COALESCE(project_id, '') FROM task_activities WHERE id = ?`, tc.id,
		).Scan(&gotTask, &gotProject); err != nil {
			t.Fatalf("reading %s: %v", tc.id, err)
		}
		if gotTask != tc.wantTask || gotProject != tc.wantProject {
			t.Errorf("%s: task_id = %q, project_id = %q, want %q and %q", tc.id, gotTask, gotProject, tc.wantTask, tc.wantProject)
		}
	}

	// The constraints the migration exists to restore are really there.
	if _, err := migrated.conn.Exec(`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action) VALUES ('orphan', 'no-such-task', 's', 'S', 'A')`); err == nil {
		t.Error("after the migration, an activity pointing at no task was still accepted")
	}
	if _, err := migrated.conn.Exec(`INSERT INTO task_activities (id, task_id, project_id, skill_id, skill_name, action) VALUES ('mixed', 't1', 'p1', 's', 'S', 'A')`); err == nil {
		t.Error("after the migration, an activity carrying both attachments was still accepted")
	}

	// A project's activities go with it, as ON DELETE CASCADE promises. Its
	// tickets' activities are deleted first, as DeleteTask does: the foreign key
	// on task_id carries no cascade, deliberately.
	if _, err := migrated.conn.Exec(`DELETE FROM task_activities WHERE task_id IS NOT NULL`); err != nil {
		t.Fatalf("clearing the tickets' activities: %v", err)
	}
	if _, err := migrated.conn.Exec(`DELETE FROM tasks WHERE project_id = 'p1'`); err != nil {
		t.Fatalf("clearing the project's tasks: %v", err)
	}
	if _, err := migrated.conn.Exec(`DELETE FROM projects WHERE id = 'p1'`); err != nil {
		t.Fatalf("deleting the project: %v", err)
	}
	if err := migrated.conn.QueryRow(`SELECT COUNT(*) FROM task_activities WHERE id = 'a-project'`).Scan(&count); err != nil {
		t.Fatalf("counting after the project was deleted: %v", err)
	}
	if count != 0 {
		t.Error("the project's activity outlived the project")
	}
}

// A migration that stopped halfway is picked up again on the next start. The
// column is not the guard — the foreign key it exists to restore is — so a run
// interrupted after project_id was added does not leave the table without its
// constraints for ever.
func TestPostgresResumesAHalfMigratedTable(t *testing.T) {
	d := openPostgres(t)
	dsn := postgresDSN(t)

	for _, stmt := range []string{
		`INSERT INTO projects (id, name, slug) VALUES ('p1', 'P1', 'p1')`,
		`DROP TABLE task_activities`,
		legacyActivitiesTable,
		// As far as a previous attempt got before failing.
		`ALTER TABLE task_activities ALTER COLUMN task_id DROP NOT NULL`,
		`ALTER TABLE task_activities ADD COLUMN project_id TEXT`,
		`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action) VALUES ('a-project', 'sync-p1', 'sync_github', 'Sync', 'Sync')`,
	} {
		if _, err := d.conn.Exec(stmt); err != nil {
			t.Fatalf("seeding the half-migrated table (%s): %v", stmt, err)
		}
	}
	forgetSchemaVersion(t, d)
	d.Close()

	migrated, err := Open(Config{Driver: DriverPostgres, DSN: dsn})
	if err != nil {
		t.Fatalf("reopening the database: %v", err)
	}
	t.Cleanup(func() { migrated.Close() })

	var gotTask, gotProject string
	if err := migrated.conn.QueryRow(
		`SELECT COALESCE(task_id, ''), COALESCE(project_id, '') FROM task_activities WHERE id = 'a-project'`,
	).Scan(&gotTask, &gotProject); err != nil {
		t.Fatalf("reading the activity: %v", err)
	}
	if gotTask != "" || gotProject != "p1" {
		t.Errorf("task_id = %q, project_id = %q, want no task and project p1", gotTask, gotProject)
	}
	if _, err := migrated.conn.Exec(`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action) VALUES ('orphan', 'no-such-task', 's', 'S', 'A')`); err == nil {
		t.Error("the foreign key was not restored on the second attempt")
	}
	if _, err := migrated.conn.Exec(`INSERT INTO task_activities (id, project_id, skill_id, skill_name, action) VALUES ('ghost', 'no-such-project', 's', 'S', 'A')`); err == nil {
		t.Error("the project foreign key was not restored on the second attempt")
	}
}
