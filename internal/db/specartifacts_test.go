package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"tasks/internal/trackerapi"
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
		// Nor anything the migrations after 25 change, which reopening replays.
		"DROP TABLE agent_capabilities",
		"ALTER TABLE projects ADD COLUMN tty_mode TEXT NOT NULL DEFAULT 'integrated'",
		"ALTER TABLE user_tracker_credentials DROP COLUMN unlock_generation",
		"DROP TABLE user_credential_unlocks",
		"DROP TABLE batch_members",
		"ALTER TABLE projects ADD COLUMN mono_repo INTEGER NOT NULL DEFAULT 1",
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

// specifyOwnedTask is a task at clarified in a project that opens its pull
// request at specification, with an agent answering spec_artifacts with
// answer and a forge that finds nothing.
func specifyOwnedTask(t *testing.T, answer func() (json.RawMessage, error)) (*DB, *models.Task, func() []string) {
	t.Helper()
	d := testDB(t)
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Early", IssueTracker: "local", PRCreationStage: "specified"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: p.ID, Title: "early", Labels: []string{"#clarified"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.conn.Exec("UPDATE tasks SET branch_name='ticket' WHERE id=?", task.ID); err != nil {
		t.Fatal(err)
	}
	var asked []string
	var askedMu sync.Mutex
	d.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		// Stage validation and the postback worker can call the agent concurrently.
		askedMu.Lock()
		asked = append(asked, op.Action)
		askedMu.Unlock()
		switch op.Action {
		case "spec_artifacts":
			return answer()
		case "git_evidence":
			return json.RawMessage(`{"sha":"agent-commit","branch":"ticket","clean":true}`), nil
		}
		return nil, fmt.Errorf("unknown local operation %q", op.Action)
	})
	d.prEvidenceLookup = func(string, string, string) (trackerapi.PullRequest, error) {
		return trackerapi.PullRequest{}, fmt.Errorf("no pull request for this branch")
	}
	return d, task, func() []string {
		askedMu.Lock()
		defer askedMu.Unlock()
		return slices.Clone(asked)
	}
}

// A workstation that drops the artefacts has nothing to open a pull request
// on at specification: the specified transition goes without one, and says
// the pull request moves to the implemented stage (#487).
func TestSpecifiedWithoutPullRequestWhenTheWorkstationDrops(t *testing.T) {
	d, task, _ := specifyOwnedTask(t, func() (json.RawMessage, error) { return json.RawMessage(`{"mode":"drop"}`), nil })
	got, _, err := d.TransitionTaskStage(task.ID, "specified", "spec written", "", "ticket")
	if err != nil || d.StageOfTask(got) != "specified" {
		t.Fatalf("a drop workstation must pass without a PR: %+v %v", got, err)
	}
	if got.PrURL != nil && *got.PrURL != "" {
		t.Fatalf("no PR must be recorded: %q", *got.PrURL)
	}
	// The notice is appended to the transition note posted on the ticket.
	set, err := d.validateStagePRs(task, "", "specify", "", "ticket", []string{""})
	if err != nil || set.notice != prDeferredNotice || len(set.urls) != 0 {
		t.Fatalf("the report must say the PR is deferred: %+v %v", set, err)
	}
}

// Keep, an agent that does not know the question, or no answer at all: the
// specified transition still needs its pull request.
func TestSpecifiedStillNeedsItsPullRequestOtherwise(t *testing.T) {
	for name, answer := range map[string]func() (json.RawMessage, error){
		"keep":    func() (json.RawMessage, error) { return json.RawMessage(`{"mode":"keep"}`), nil },
		"old":     func() (json.RawMessage, error) { return nil, fmt.Errorf(`unknown local operation "spec_artifacts"`) },
		"offline": func() (json.RawMessage, error) { return nil, fmt.Errorf("local operation requires a connected agent") },
		"garbled": func() (json.RawMessage, error) { return json.RawMessage(`{"mode":"maybe"}`), nil },
	} {
		t.Run(name, func(t *testing.T) {
			d, task, _ := specifyOwnedTask(t, answer)
			if _, _, err := d.TransitionTaskStage(task.ID, "specified", "spec written", "", "ticket"); err == nil {
				t.Fatal("the specified transition must still need its PR")
			}
		})
	}
}

// A pull request the stage names is checked as before, drop or not, and the
// implemented stage keeps requiring its own.
func TestDroppedArtefactsLeaveTheOtherPullRequestChecksAlone(t *testing.T) {
	d, task, asked := specifyOwnedTask(t, func() (json.RawMessage, error) { return json.RawMessage(`{"mode":"drop"}`), nil })
	if _, _, err := d.TransitionTaskStage(task.ID, "specified", "spec written", "https://forge/pull/9", "ticket"); err == nil {
		t.Fatal("a named PR the forge cannot confirm must still be refused")
	}
	for _, action := range asked() {
		if action == "spec_artifacts" {
			t.Fatal("a named PR must not ask the agent about the artefacts")
		}
	}
	if _, _, err := d.TransitionTaskStage(task.ID, "implemented", "code written", "", "ticket"); err == nil {
		t.Fatal("the implemented stage must still require its PR")
	}
}
