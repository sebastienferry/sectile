package db

import (
	"path/filepath"
	"testing"

	"tasks/internal/models"
)

// A synchronisation copies the tracker's priority only when the tracker
// carries one (#882). GitHub and GitLab have none and report medium, which
// must not overwrite the level set in Sectile; Jira's own priority still wins.
func TestSyncKeepsTheSectilePriorityOfTicketsWithoutTrackerPriority(t *testing.T) {
	checkSyncKeepsTheSectilePriority(t, func(t *testing.T) *DB {
		d, err := NewDB(filepath.Join(t.TempDir(), "syncpriority.db"))
		if err != nil {
			t.Fatalf("creating database: %v", err)
		}
		t.Cleanup(func() { d.Close() })
		return d
	})
}

// The update binds a boolean in a CASE WHEN, which PostgreSQL types on its own.
func TestPostgresSyncKeepsTheSectilePriority(t *testing.T) {
	checkSyncKeepsTheSectilePriority(t, openPostgres)
}

func checkSyncKeepsTheSectilePriority(t *testing.T, open func(*testing.T) *DB) {
	cases := []struct {
		source string
		key    string
		want   models.Priority
	}{
		{source: "github", key: "#100", want: models.PriorityHigh},
		{source: "gitlab", key: "#200", want: models.PriorityHigh},
		{source: "jira", key: "PROJ-300", want: models.PriorityMedium},
	}
	for _, c := range cases {
		t.Run(c.source, func(t *testing.T) {
			d := open(t)
			seedProjectAndUser(t, d)

			synced := models.Task{
				ProjectID: "p1",
				Key:       c.key,
				Title:     "Imported",
				Status:    models.StatusToClarify,
				Priority:  models.PriorityMedium,
				Source:    c.source,
			}
			if err := d.ImportOrUpdateTasks("", []models.Task{synced}); err != nil {
				t.Fatalf("first import: %v", err)
			}
			id := syncedTaskID(t, d, c.key)
			if got := syncedPriority(t, d, id); got != models.PriorityMedium {
				t.Fatalf("a first import stores %q, want medium", got)
			}

			high := models.PriorityHigh
			if _, err := d.UpdateTask(id, models.UpdateTaskRequest{Priority: &high}); err != nil {
				t.Fatalf("setting the priority in Sectile: %v", err)
			}

			synced.Title = "Imported - renamed"
			if err := d.ImportOrUpdateTasks("", []models.Task{synced}); err != nil {
				t.Fatalf("re-import: %v", err)
			}
			if got := syncedPriority(t, d, id); got != c.want {
				t.Errorf("after a re-import carrying medium, priority = %q, want %q", got, c.want)
			}
			task, err := d.GetTaskByID(id)
			if err != nil || task == nil {
				t.Fatalf("reading %s back: %v", id, err)
			}
			if task.Title != "Imported - renamed" {
				t.Errorf("the re-import still writes the other fields: title = %q", task.Title)
			}
		})
	}
}

func syncedTaskID(t *testing.T, d *DB, key string) string {
	t.Helper()
	var id string
	if err := d.conn.QueryRow("SELECT id FROM tasks WHERE key = ?", key).Scan(&id); err != nil {
		t.Fatalf("finding the imported ticket %s: %v", key, err)
	}
	return id
}

func syncedPriority(t *testing.T, d *DB, id string) models.Priority {
	t.Helper()
	var priority string
	if err := d.conn.QueryRow("SELECT priority FROM tasks WHERE id = ?", id).Scan(&priority); err != nil {
		t.Fatalf("reading the priority of %s: %v", id, err)
	}
	return models.Priority(priority)
}
