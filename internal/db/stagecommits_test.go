package db

import (
	"path/filepath"
	"testing"

	"tasks/internal/models"
)

func TestPushStageCommitsPersistence(t *testing.T) {
	d, project := modeTestDB(t)
	if project.PushStageCommits {
		t.Fatal("stage publication must default to off")
	}
	created, err := d.CreateProject(models.CreateProjectRequest{Name: "Published", IssueTracker: "local", PushStageCommits: true})
	if err != nil {
		t.Fatal(err)
	}
	if !created.PushStageCommits {
		t.Fatal("create lost publication setting")
	}
	for _, enabled := range []bool{true, false} {
		if _, err := d.UpdateProject(project.ID, models.UpdateProjectRequest{PushStageCommits: &enabled}); err != nil {
			t.Fatal(err)
		}
		// An unrelated partial update must preserve the setting.
		name := "Renamed"
		if _, err := d.UpdateProject(project.ID, models.UpdateProjectRequest{Name: &name}); err != nil {
			t.Fatal(err)
		}
		reread, err := d.GetProjectByID(project.ID)
		if err != nil {
			t.Fatal(err)
		}
		if reread.PushStageCommits != enabled {
			t.Fatalf("reread setting = %v, want %v", reread.PushStageCommits, enabled)
		}
		projects, err := d.GetProjects()
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, listed := range projects {
			if listed.ID == project.ID {
				found = true
				if listed.PushStageCommits != enabled {
					t.Fatal("list lost publication setting")
				}
			}
		}
		if !found {
			t.Fatal("project missing from list")
		}
		config, err := d.AgentConfig(project.ID, "")
		if err != nil {
			t.Fatal(err)
		}
		if config.PushStageCommits != enabled {
			t.Fatal("agent config lost publication setting")
		}
	}
}

func TestPushStageCommitsMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrade.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE macros DROP COLUMN labels"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE projects DROP COLUMN push_stage_commits"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("DELETE FROM schema_migrations WHERE version >= 32"); err != nil {
		t.Fatal(err)
	}
	d.Close()
	for range 2 {
		d, err = NewDB(path)
		if err != nil {
			t.Fatal(err)
		}
		p, err := d.GetProjectByID("default")
		if err != nil {
			t.Fatal(err)
		}
		if p.PushStageCommits {
			t.Fatal("upgrade must leave publication off")
		}
		d.Close()
	}
}
