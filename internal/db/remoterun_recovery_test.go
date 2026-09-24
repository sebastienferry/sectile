package db

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tasks/internal/models"
)

// recoveryStore opens one engine and returns a project the runs can live on, so
// the same expectations are checked wherever the deployment stores its data.
func recoveryStore(t *testing.T, engine string) (*DB, *models.Project) {
	t.Helper()
	var d *DB
	var err error
	switch engine {
	case "sqlite":
		d, err = NewDB(filepath.Join(t.TempDir(), "recovery.db"))
	default:
		d = openPostgres(t)
	}
	if err != nil {
		t.Fatal(err)
	}
	if engine == "sqlite" {
		t.Cleanup(func() { d.Close() })
	}
	no := false
	project, err := d.CreateProject(models.CreateProjectRequest{Name: "Recovery", RepoPath: "/not-mounted",
		IssueTracker: "local", UseWorktrees: &no, AIProvider: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	return d, project
}

func recoveryEngines(t *testing.T, body func(t *testing.T, d *DB, project *models.Project)) {
	t.Helper()
	for _, engine := range []string{"sqlite", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			d, project := recoveryStore(t, engine)
			body(t, d, project)
		})
	}
}

// A disconnection decides an outcome in the client's absence. Once the client
// is back, its own report is the truthful one, and the chain continues from the
// corrected status instead of stopping on an accident.
func TestOwnerRecoversARunCanceledByADisconnection(t *testing.T) {
	recoveryEngines(t, func(t *testing.T, d *DB, project *models.Project) {
		owner, err := d.SignInLocal("alice@example.com")
		if err != nil {
			t.Fatal(err)
		}
		task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "recovered"})
		if err != nil {
			t.Fatal(err)
		}
		run, err := d.StartAgentRun(task.ID, "clarify", RunLaunch{Mode: models.SkillModeAutonomous,
			ChainStop: "reviewed", UserID: owner.ID})
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := d.TransitionTaskStage(task.ID, "clarified", "", "", ""); err != nil {
			t.Fatal(err)
		}
		// The session ends while the agent is still working: the server closes
		// the run in its stead.
		if _, err := d.FinishRemoteRun(task.ID, run.ID, "canceled", models.RunDisconnectNote); err != nil {
			t.Fatal(err)
		}

		recovered, err := d.FinishRemoteRunAs(Actor{ID: owner.ID}, false, task.ID, run.ID, "completed", "clarified, really")
		if err != nil {
			t.Fatalf("the owner could not recover their run: %v", err)
		}
		if recovered.Status != "completed" || recovered.Summary != "clarified, really" {
			t.Fatalf("recovered run = %q/%q, want the reported outcome", recovered.Status, recovered.Summary)
		}
		if !awaitSkillActivity(t, d, task.ID, skillName(t, d, "specify")) {
			t.Fatal("the recovered run did not hand the chain back")
		}
		// The hand-back replays once: a recovered run is no longer canceled, so
		// the widened path cannot be entered a second time.
		before := countSkillActivities(t, d, task.ID, skillName(t, d, "specify"))
		if _, err := d.FinishRemoteRunAs(Actor{ID: owner.ID}, false, task.ID, run.ID, "completed", "again"); err != nil {
			t.Fatal(err)
		}
		time.Sleep(200 * time.Millisecond)
		if after := countSkillActivities(t, d, task.ID, skillName(t, d, "specify")); after != before {
			t.Fatalf("a repeated recovery enqueued %d more step(s)", after-before)
		}
	})
}

// A cancellation someone typed is a decision, not an accident: a late agent
// must not undo it. Neither may a colleague report on a run that is not theirs.
func TestADeliberateCancellationStaysFinal(t *testing.T) {
	recoveryEngines(t, func(t *testing.T, d *DB, project *models.Project) {
		owner, err := d.SignInLocal("alice@example.com")
		if err != nil {
			t.Fatal(err)
		}
		other, err := d.SignInLocal("bob@example.com")
		if err != nil {
			t.Fatal(err)
		}
		task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "stopped"})
		if err != nil {
			t.Fatal(err)
		}
		run, err := d.StartRemoteRunBy(owner.ID, task.ID, "implement", "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.FinishRemoteRunAs(Actor{ID: owner.ID}, false, task.ID, run.ID, "canceled", "Stopped from the board"); err != nil {
			t.Fatal(err)
		}
		if _, err := d.FinishRemoteRunAs(Actor{ID: owner.ID}, false, task.ID, run.ID, "completed", "too late"); err == nil {
			t.Fatal("a run canceled on purpose was rewritten")
		}
		if still, _ := d.GetActivityByID(run.ID); still.Status != "canceled" || still.Summary != "Stopped from the board" {
			t.Fatalf("the refused rewrite changed the run to %q/%q", still.Status, still.Summary)
		}

		// Ownership is unchanged by the recovery path: a disconnected run is
		// still nobody else's to report on.
		second, err := d.StartRemoteRunBy(owner.ID, task.ID, "implement", "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.FinishRemoteRun(task.ID, second.ID, "canceled", models.RunDisconnectNote); err != nil {
			t.Fatal(err)
		}
		if _, err := d.FinishRemoteRunAs(Actor{ID: other.ID}, false, task.ID, second.ID, "completed", "not mine"); !errors.Is(err, ErrRunNotYours) {
			t.Fatalf("a colleague recovered someone else's run: %v", err)
		}
	})
}

// A run that fell silent and was then closed by a disconnection carries the
// silence sentence ahead of the disconnect note. Its owner must still be able to
// report the real outcome: matching the note as a prefix missed it (#319).
func TestASilencedThenDisconnectedRunIsRecoverable(t *testing.T) {
	recoveryEngines(t, func(t *testing.T, d *DB, project *models.Project) {
		owner, err := d.SignInLocal("alice@example.com")
		if err != nil {
			t.Fatal(err)
		}
		task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "abandoned"})
		if err != nil {
			t.Fatal(err)
		}
		run, err := d.StartRemoteRunBy(owner.ID, task.ID, "implement", "")
		if err != nil {
			t.Fatal(err)
		}
		if err := d.NoteRemoteRun(run.ID, models.RunSilenceNote(8*time.Hour)); err != nil {
			t.Fatal(err)
		}
		closed, err := d.FinishRemoteRun(task.ID, run.ID, "canceled", models.RunDisconnectNote)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(closed.Summary, models.RunDisconnectNote) {
			t.Fatalf("summary %q no longer opens on the silence, the test proves nothing", closed.Summary)
		}
		recovered, err := d.FinishRemoteRunAs(Actor{ID: owner.ID}, false, task.ID, run.ID, "completed", "done after all")
		if err != nil || recovered.Status != "completed" {
			t.Fatalf("the owner could not recover a silenced then closed run: %+v %v", recovered, err)
		}
	})
}

// A silence was an observation. The outcome joins it rather than erasing the
// only trace of why the run looked quiet.
func TestAReportedOutcomeJoinsTheSilenceItFollows(t *testing.T) {
	recoveryEngines(t, func(t *testing.T, d *DB, project *models.Project) {
		owner, err := d.SignInLocal("alice@example.com")
		if err != nil {
			t.Fatal(err)
		}
		task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "quiet"})
		if err != nil {
			t.Fatal(err)
		}
		run, err := d.StartRemoteRunBy(owner.ID, task.ID, "implement", "")
		if err != nil {
			t.Fatal(err)
		}
		silence := models.RunSilenceNote(4 * time.Hour)
		if err := d.NoteRemoteRun(run.ID, silence); err != nil {
			t.Fatalf("NoteRemoteRun: %v", err)
		}
		noted, _ := d.GetActivityByID(run.ID)
		if noted.Status != "running" || !strings.Contains(noted.Summary, models.RunSilencePrefix) {
			t.Fatalf("a silence left the run at %q/%q, want a running run carrying the sentence", noted.Status, noted.Summary)
		}

		finished, err := d.FinishRemoteRunAs(Actor{ID: owner.ID}, false, task.ID, run.ID, "completed", "done at last")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(finished.Summary, models.RunSilencePrefix) || !strings.HasSuffix(finished.Summary, "done at last") {
			t.Fatalf("summary = %q, want both halves of the story", finished.Summary)
		}

		// A finished run is never annotated after the fact: the observation is
		// about a run in flight.
		if err := d.NoteRemoteRun(run.ID, "too late"); err != nil {
			t.Fatal(err)
		}
		if after, _ := d.GetActivityByID(run.ID); strings.Contains(after.Summary, "too late") {
			t.Fatalf("a finished run was annotated: %q", after.Summary)
		}
	})
}
