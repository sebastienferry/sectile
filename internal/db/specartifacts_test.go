package db

import (
	"errors"
	"path/filepath"
	"testing"

	"tasks/internal/models"
)

// A project keeps its specification artefacts unless it says otherwise, drops
// them once told to, and an update that does not name the field leaves it as
// it is (#487).
func TestProjectSpecArtifactsRoundTrip(t *testing.T) {
	database := testDB(t)

	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Specs"})
	if err != nil {
		t.Fatal(err)
	}
	if project.SpecArtifacts != models.SpecArtifactsKeep {
		t.Fatalf("a new project must keep its artefacts, got %q", project.SpecArtifacts)
	}

	drop := "drop"
	updated, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{SpecArtifacts: &drop})
	if err != nil {
		t.Fatal(err)
	}
	if updated.SpecArtifacts != models.SpecArtifactsDrop {
		t.Fatalf("saving drop: got %q", updated.SpecArtifacts)
	}

	name := "Specs renamed"
	if _, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{Name: &name}); err != nil {
		t.Fatal(err)
	}
	reread, err := database.GetProjectByID(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reread.SpecArtifacts != models.SpecArtifactsDrop {
		t.Fatalf("an update omitting the field changed it to %q", reread.SpecArtifacts)
	}
	config, err := database.AgentConfig(project.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if config.SpecArtifacts != models.SpecArtifactsDrop {
		t.Fatalf("the agent config does not carry the setting: %q", config.SpecArtifacts)
	}

	keep := "keep"
	back, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{SpecArtifacts: &keep})
	if err != nil {
		t.Fatal(err)
	}
	if back.SpecArtifacts != models.SpecArtifactsKeep {
		t.Fatalf("saving keep: got %q", back.SpecArtifacts)
	}

	created, err := database.CreateProject(models.CreateProjectRequest{Name: "Dropping", SpecArtifacts: "drop"})
	if err != nil {
		t.Fatal(err)
	}
	if created.SpecArtifacts != models.SpecArtifactsDrop {
		t.Fatalf("creating with drop: got %q", created.SpecArtifacts)
	}
}

// A value the project cannot hold is refused with the accepted values named,
// on create and on update, and nothing is stored.
func TestProjectSpecArtifactsRefusesUnknownValues(t *testing.T) {
	database := testDB(t)

	if _, err := database.CreateProject(models.CreateProjectRequest{Name: "Bad", SpecArtifacts: "discard"}); !errors.Is(err, ErrInvalidSpecArtifacts) {
		t.Fatalf("create with an unknown value: %v", err)
	}
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Specs"})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"discard", ""} {
		value := value
		if _, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{SpecArtifacts: &value}); !errors.Is(err, ErrInvalidSpecArtifacts) {
			t.Fatalf("update with %q: %v", value, err)
		}
	}
	if ErrInvalidSpecArtifacts.Error() != "specArtifacts must be keep or drop" {
		t.Fatalf("the refusal must name the accepted values: %q", ErrInvalidSpecArtifacts)
	}
	reread, err := database.GetProjectByID(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reread.SpecArtifacts != models.SpecArtifactsKeep {
		t.Fatalf("a refused update changed the setting to %q", reread.SpecArtifacts)
	}
}

// A project written before the setting existed reads as keep once upgraded.
func TestMigrationTwentyFiveKeepsExistingProjectsArtefacts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spec.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatalf("creating the database: %v", err)
	}
	for _, stmt := range []string{
		"ALTER TABLE projects DROP COLUMN spec_artifacts",
		`INSERT INTO projects (id, name, slug) VALUES ('p1', 'Old', 'old')`,
		"DELETE FROM schema_migrations WHERE version >= 25",
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
	project, err := reopened.GetProjectByID("p1")
	if err != nil || project == nil {
		t.Fatalf("reading the upgraded project: %+v %v", project, err)
	}
	if project.SpecArtifacts != models.SpecArtifactsKeep {
		t.Fatalf("an upgraded project must keep its artefacts, got %q", project.SpecArtifacts)
	}
}
