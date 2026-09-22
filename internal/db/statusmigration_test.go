package db

import (
	"path/filepath"
	"testing"

	"tasks/internal/models"
)

func TestOpenDBMigratesOnlyRetiredToSpecifyStatus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sectile.db")
	database, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}

	project, err := database.CreateProject(models.CreateProjectRequest{
		Name:         "Migration",
		RepoPath:     t.TempDir(),
		IssueTracker: "local",
	})
	if err != nil {
		t.Fatal(err)
	}
	retired, err := database.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "Retired status"})
	if err != nil {
		t.Fatal(err)
	}
	specified, err := database.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "Specified status"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.conn.Exec("UPDATE tasks SET status = CASE id WHEN ? THEN 'to_specify' WHEN ? THEN 'specified' END WHERE id IN (?, ?)", retired.ID, specified.ID, retired.ID, specified.ID); err != nil {
		t.Fatal(err)
	}
	forgetSchemaVersion(t, database)
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	database, err = NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	migrated, err := database.GetTaskByID(retired.ID)
	if err != nil {
		t.Fatal(err)
	}
	if migrated.Status != models.StatusClarified {
		t.Fatalf("retired status was not normalized: %q", migrated.Status)
	}
	preserved, err := database.GetTaskByID(specified.ID)
	if err != nil {
		t.Fatal(err)
	}
	if preserved.Status != models.StatusSpecified {
		t.Fatalf("specified status was changed: %q", preserved.Status)
	}
}
