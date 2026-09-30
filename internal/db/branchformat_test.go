package db

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/models"
)

func TestBranchNameFormatPersistence(t *testing.T) {
	d, project := modeTestDB(t)
	if project.BranchNameFormat != "" {
		t.Fatalf("a new project has format %q, want the default (empty)", project.BranchNameFormat)
	}
	created, err := d.CreateProject(models.CreateProjectRequest{Name: "Jira", IssueTracker: "local", BranchNameFormat: " {key} "})
	if err != nil {
		t.Fatal(err)
	}
	if created.BranchNameFormat != "{key}" {
		t.Fatalf("create stored %q, want the trimmed {key}", created.BranchNameFormat)
	}
	for _, c := range []struct{ sent, want string }{
		{"feat/{key}-{title}", "feat/{key}-{title}"},
		{"   ", ""},
		{"{key_lower}", "{key_lower}"},
	} {
		format := c.sent
		if _, err := d.UpdateProject(project.ID, models.UpdateProjectRequest{BranchNameFormat: &format}); err != nil {
			t.Fatalf("update with %q: %v", c.sent, err)
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
		if reread.BranchNameFormat != c.want {
			t.Fatalf("after sending %q the project reads %q, want %q", c.sent, reread.BranchNameFormat, c.want)
		}
		projects, err := d.GetProjects()
		if err != nil {
			t.Fatal(err)
		}
		for _, listed := range projects {
			if listed.ID == project.ID && listed.BranchNameFormat != c.want {
				t.Fatalf("the project list reads %q, want %q", listed.BranchNameFormat, c.want)
			}
		}
		config, err := d.AgentConfig(project.ID, "")
		if err != nil {
			t.Fatal(err)
		}
		if config.BranchNameFormat != c.want {
			t.Fatalf("the agent configuration carries %q, want %q", config.BranchNameFormat, c.want)
		}
	}
}

func TestBranchNameFormatRefusedOnSave(t *testing.T) {
	d, project := modeTestDB(t)
	stored := "{key}"
	if _, err := d.UpdateProject(project.ID, models.UpdateProjectRequest{BranchNameFormat: &stored}); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"feat/{title}", "feat/{id}-{key}", "feat/{key", "feat//{key}"} {
		bad := format
		_, err := d.UpdateProject(project.ID, models.UpdateProjectRequest{BranchNameFormat: &bad})
		if !errors.Is(err, ErrInvalidBranchNameFormat) {
			t.Fatalf("update with %q returned %v, want ErrInvalidBranchNameFormat", format, err)
		}
		// The message is the renderer's own, in the interface's language.
		if strings.Contains(err.Error(), "invalid branch name format") || !strings.Contains(err.Error(), "format de nom de branche") {
			t.Fatalf("update with %q refused with %q, want the French reason alone", format, err.Error())
		}
		reread, err := d.GetProjectByID(project.ID)
		if err != nil {
			t.Fatal(err)
		}
		if reread.BranchNameFormat != stored {
			t.Fatalf("a refused update left %q, want %q", reread.BranchNameFormat, stored)
		}
		if _, err := d.CreateProject(models.CreateProjectRequest{Name: "Refused " + format, IssueTracker: "local", BranchNameFormat: format}); !errors.Is(err, ErrInvalidBranchNameFormat) {
			t.Fatalf("create with %q returned %v, want ErrInvalidBranchNameFormat", format, err)
		}
	}
}

func TestBranchNameFormatMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrade.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateProject(models.CreateProjectRequest{Name: "Before", IssueTracker: "local"}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE projects DROP COLUMN branch_name_format"); err != nil {
		t.Fatal(err)
	}
	// Migrations 34 and 35 come after it and are replayed too: a database
	// stamped 35 would never run 33 again.
	if _, err := d.conn.Exec("ALTER TABLE macros DROP COLUMN priority"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE macros DROP COLUMN quarter"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE macros DROP COLUMN labels"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE task_activities DROP COLUMN credential_missing"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("DELETE FROM schema_migrations WHERE version >= 33"); err != nil {
		t.Fatal(err)
	}
	d.Close()
	d, err = NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	projects, err := d.GetProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) == 0 {
		t.Fatal("the project did not survive the upgrade")
	}
	for _, p := range projects {
		if p.BranchNameFormat != "" {
			t.Fatalf("project %s reads format %q after the upgrade, want the default (empty)", p.Name, p.BranchNameFormat)
		}
	}
}
