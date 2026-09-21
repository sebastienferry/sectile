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

// A synchronisation activity is not attached to a task: EnqueueSync files it
// under a synthetic id ("sync-<project>"), and the agent queue does the same for
// work that belongs to a project rather than a ticket.
//
// task_activities declares a foreign key to tasks. SQLite does not enforce
// foreign keys unless asked, and this package never asks, so the row went in.
// PostgreSQL always enforces them, so the insert is rejected and the activity
// simply never appears — a synchronisation that runs and leaves no trace.
func TestPostgresActivityWithoutATask(t *testing.T) {
	d := openPostgres(t)
	if _, err := d.conn.Exec(`INSERT INTO projects (id, name, slug) VALUES ('p1','P','p')`); err != nil {
		t.Fatalf("seeding a project: %v", err)
	}

	act := models.TaskActivity{
		ID:        "sync-activity-1",
		TaskID:    "sync-p1", // deliberately not a row in tasks
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

	got, err := d.GetTaskActivities("sync-p1")
	if err != nil {
		t.Fatalf("GetTaskActivities: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("%d activity(ies), want 1 — the synchronisation left no trace", len(got))
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
