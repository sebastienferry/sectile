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
	// Migrations 33 and 34 come after it and are replayed too.
	if _, err := d.conn.Exec("ALTER TABLE projects DROP COLUMN branch_name_format"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE macros DROP COLUMN priority"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE macros DROP COLUMN quarter"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE macros DROP COLUMN readiness"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE task_activities DROP COLUMN credential_missing"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE projects DROP COLUMN roadmap_axis_writes"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE user_tracker_credentials DROP COLUMN kind"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE user_tracker_credentials DROP COLUMN version"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE user_tracker_credentials DROP COLUMN disconnected_at"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE user_tracker_credentials DROP COLUMN refresh_claimed_at"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("DROP TABLE jira_oauth_flows"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("DROP TABLE tracker_oauth_apps"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE projects DROP COLUMN epic_axis_prefixes"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE macros DROP COLUMN todos_mirror_ref"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE macros DROP COLUMN todos_mirror_hash"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE macros DROP COLUMN todos_mirror_error"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE macros DROP COLUMN todos_mirror_credential"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE macros DROP COLUMN todos_mirror_at"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE macros DROP COLUMN framing_mirror_ref"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE macros DROP COLUMN framing_mirror_hash"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE macros DROP COLUMN framing_mirror_error"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE macros DROP COLUMN framing_mirror_credential"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE macros DROP COLUMN framing_mirror_at"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE projects DROP COLUMN priority_mapping"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE projects DROP COLUMN epic_axis_fields"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("DELETE FROM schema_migrations WHERE version >= 32"); err != nil {
		t.Fatal(err)
	}
	undoTrackerMigration(d)
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
