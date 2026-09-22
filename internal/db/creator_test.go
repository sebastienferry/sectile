package db

import (
	"path/filepath"
	"testing"

	"tasks/internal/models"
)

func TestTaskCreatorPersistence(t *testing.T) {
	d, err := NewDB(filepath.Join(t.TempDir(), "creator.db"))
	if err != nil {
		t.Fatalf("creating database: %v", err)
	}
	defer d.Close()

	seedProjectAndUser(t, d)

	// 1. Test CreateTask with Creator and CreatorAvatar
	req := models.CreateTaskRequest{
		ProjectID:     "p1",
		Title:         "Feature with Creator",
		Creator:       "alice",
		CreatorAvatar: "https://example.com/alice.png",
		Assignee:      "bob",
	}

	created, err := d.CreateTask(req)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if created.Creator != "alice" {
		t.Errorf("created.Creator = %q, want %q", created.Creator, "alice")
	}
	if created.CreatorAvatar != "https://example.com/alice.png" {
		t.Errorf("created.CreatorAvatar = %q, want %q", created.CreatorAvatar, "https://example.com/alice.png")
	}

	// 2. Test GetTaskByID
	fetched, err := d.GetTaskByID(created.ID)
	if err != nil {
		t.Fatalf("GetTaskByID(%q): %v", created.ID, err)
	}
	if fetched == nil {
		t.Fatalf("task %q not found", created.ID)
	}
	if fetched.Creator != "alice" {
		t.Errorf("fetched.Creator = %q, want %q", fetched.Creator, "alice")
	}
	if fetched.CreatorAvatar != "https://example.com/alice.png" {
		t.Errorf("fetched.CreatorAvatar = %q, want %q", fetched.CreatorAvatar, "https://example.com/alice.png")
	}

	// 3. Test GetTasks
	tasks, err := d.GetTasks("", "", "", "", "p1", "", "", "", "", nil, nil, false)
	if err != nil {
		t.Fatalf("GetTasks: %v", err)
	}
	if len(tasks) == 0 {
		t.Fatalf("GetTasks returned 0 tasks")
	}
	if tasks[0].Creator != "alice" {
		t.Errorf("tasks[0].Creator = %q, want %q", tasks[0].Creator, "alice")
	}
	if tasks[0].CreatorAvatar != "https://example.com/alice.png" {
		t.Errorf("tasks[0].CreatorAvatar = %q, want %q", tasks[0].CreatorAvatar, "https://example.com/alice.png")
	}

	// 4. Test ImportOrUpdateTasks insert
	syncTask := models.Task{
		ProjectID:     "p1",
		Key:           "#100",
		Title:         "Imported from tracker",
		Creator:       "octocat",
		CreatorAvatar: "https://avatars.githubusercontent.com/u/1",
		Status:        models.StatusToClarify,
		Priority:      models.PriorityMedium,
	}
	if err := d.ImportOrUpdateTasks([]models.Task{syncTask}); err != nil {
		t.Fatalf("ImportOrUpdateTasks: %v", err)
	}

	imported, err := d.GetTaskByID("#100")
	if err != nil {
		t.Fatalf("GetTaskByID(#100): %v", err)
	}
	if imported.Creator != "octocat" {
		t.Errorf("imported.Creator = %q, want %q", imported.Creator, "octocat")
	}
	if imported.CreatorAvatar != "https://avatars.githubusercontent.com/u/1" {
		t.Errorf("imported.CreatorAvatar = %q, want %q", imported.CreatorAvatar, "https://avatars.githubusercontent.com/u/1")
	}

	// 5. Test ImportOrUpdateTasks update preserves or updates creator
	syncTaskUpdate := models.Task{
		ProjectID:     "p1",
		Key:           "#100",
		Title:         "Imported from tracker - Updated",
		Creator:       "octocat",
		CreatorAvatar: "https://avatars.githubusercontent.com/u/1-new",
		Status:        models.StatusClarified,
		Priority:      models.PriorityHigh,
	}
	if err := d.ImportOrUpdateTasks([]models.Task{syncTaskUpdate}); err != nil {
		t.Fatalf("ImportOrUpdateTasks update: %v", err)
	}

	updated, err := d.GetTaskByID("#100")
	if err != nil {
		t.Fatalf("GetTaskByID(#100) after update: %v", err)
	}
	if updated.Creator != "octocat" {
		t.Errorf("updated.Creator = %q, want %q", updated.Creator, "octocat")
	}
	if updated.CreatorAvatar != "https://avatars.githubusercontent.com/u/1-new" {
		t.Errorf("updated.CreatorAvatar = %q, want %q", updated.CreatorAvatar, "https://avatars.githubusercontent.com/u/1-new")
	}
}

func TestMigration2TasksCreator(t *testing.T) {
	path := filepath.Join(t.TempDir(), "migration2.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatalf("creating database: %v", err)
	}
	defer d.Close()

	// Verify schema version is at least 2
	version, err := d.schemaVersion()
	if err != nil {
		t.Fatalf("schemaVersion: %v", err)
	}
	if version < 2 {
		t.Fatalf("schemaVersion = %d, want >= 2", version)
	}

	// Verify columns exist by querying them directly
	var creator, creatorAvatar string
	row := d.conn.QueryRow("SELECT creator, creator_avatar FROM tasks LIMIT 1")
	// If no rows, ErrNoRows is expected, but column error is not
	if err := row.Scan(&creator, &creatorAvatar); err != nil && err.Error() != "sql: no rows in result set" {
		t.Fatalf("querying creator columns: %v", err)
	}
}
