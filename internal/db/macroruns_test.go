package db

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"tasks/internal/models"
)

func macroRunDB(t *testing.T) (*DB, *models.Project) {
	t.Helper()
	database, err := NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Platform", Slug: "platform", IssueTracker: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SaveMacroMeta(project.ID, "M-7", nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	return database, project
}

func TestMacroRunLifecycle(t *testing.T) {
	database, project := macroRunDB(t)
	run, err := database.StartMacroRun(project.ID, "m-7", "realign_macro", RunLaunch{UserID: "u1", Mode: models.SkillModeInteractive})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if run.TaskID != "" || run.ProjectID != project.ID || run.Action != RunActionAgent || run.MacroKey != "M-7" {
		t.Fatalf("unexpected run %+v", run)
	}
	if _, err := database.StartMacroRun(project.ID, "M-7", "realign_macro", RunLaunch{}); !errors.Is(err, ErrMacroRunBusy) {
		t.Fatalf("a second run on a busy macro must be refused, got %v", err)
	}
	// The run is not a task activity: nothing a task reads sees it.
	if active, _ := database.ActiveRunOnTask(""); active != nil {
		t.Fatalf("a macro run must not look like a task run: %+v", active)
	}

	if _, err := database.FinishMacroRunAs(Actor{ID: "u2"}, false, project.ID, "M-7", run.ID, "completed", "done"); !errors.Is(err, ErrRunNotYours) {
		t.Fatalf("another user must not close the run, got %v", err)
	}
	finished, err := database.FinishMacroRunAs(Actor{ID: "u1"}, false, project.ID, "M-7", run.ID, "completed", "done")
	if err != nil || finished.Status != "completed" {
		t.Fatalf("finish: %+v %v", finished, err)
	}
	if active, _ := database.ActiveRunOnMacro(project.ID, "M-7"); active != nil {
		t.Fatalf("a finished run leaves the macro free, got %+v", active)
	}
	runs, err := database.MacroRuns(project.ID, "M-7", 10)
	if err != nil || len(runs) != 1 || runs[0].Status != "completed" {
		t.Fatalf("history: %+v %v", runs, err)
	}
}

func TestMacroRunReuseAndSessionClosure(t *testing.T) {
	database, project := macroRunDB(t)
	run, err := database.StartMacroRunBy("u1", project.ID, "M-7", "realign_macro", "")
	if err != nil {
		t.Fatal(err)
	}
	reused, err := database.StartMacroRunBy("u1", project.ID, "M-7", "", run.ID)
	if err != nil || reused.ID != run.ID {
		t.Fatalf("a launcher's run must be reusable by id: %+v %v", reused, err)
	}
	if _, err := database.StartMacroRunBy("u1", project.ID, "M-8", "", run.ID); err == nil {
		t.Fatal("a run must not be reused under another macro")
	}
	// A session closing the runs it adopted names no task: the macro run is
	// found through its id.
	closed, err := database.FinishRemoteRun("", run.ID, "canceled", models.RunDisconnectNote)
	if err != nil || closed.Status != "canceled" {
		t.Fatalf("session closure: %+v %v", closed, err)
	}
}

func TestMacroRunNeedsAKnownMacro(t *testing.T) {
	database, project := macroRunDB(t)
	if _, err := database.StartMacroRun(project.ID, "M-99", "realign_macro", RunLaunch{}); err == nil {
		t.Fatal("an unknown macro must be refused")
	}
	if _, err := database.StartMacroRun("missing", "M-7", "realign_macro", RunLaunch{}); err == nil {
		t.Fatal("an unknown project must be refused")
	}
}

// Two launches in the same instant must not both find the macro free.
func TestConcurrentMacroLaunchesStartOneRun(t *testing.T) {
	database, project := macroRunDB(t)
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := database.StartMacroRun(project.ID, "M-7", "realign_macro", RunLaunch{})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	started := 0
	for err := range results {
		if err == nil {
			started++
		} else if !errors.Is(err, ErrMacroRunBusy) {
			t.Fatalf("a concurrent launch must be refused as busy, got %v", err)
		}
	}
	if started != 1 {
		t.Fatalf("exactly one run must start, got %d", started)
	}
}

// The server's own closure names nobody: it never rewrites an outcome a
// disconnection already recorded, which only the identified owner may do.
func TestAnonymousClosureDoesNotReopenADisconnectedRun(t *testing.T) {
	database, project := macroRunDB(t)
	run, err := database.StartMacroRun(project.ID, "M-7", "realign_macro", RunLaunch{UserID: "u1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.FinishMacroRunAs(Actor{}, true, project.ID, "M-7", run.ID, "canceled", models.RunDisconnectNote); err != nil {
		t.Fatal(err)
	}
	if _, err := database.FinishMacroRunAs(Actor{}, true, project.ID, "M-7", run.ID, "completed", "late"); err == nil {
		t.Fatal("an anonymous closure must not rewrite a disconnection")
	}
	if finished, err := database.FinishMacroRunAs(Actor{ID: "u1"}, false, project.ID, "M-7", run.ID, "completed", "done"); err != nil || finished.Status != "completed" {
		t.Fatalf("the owner may still correct it: %+v %v", finished, err)
	}
}

// A macro run that fell silent before its session was closed carries the
// silence sentence ahead of the disconnect note, and stays its owner's to
// correct (#319).
func TestASilencedThenDisconnectedMacroRunIsRecoverable(t *testing.T) {
	database, project := macroRunDB(t)
	run, err := database.StartMacroRun(project.ID, "M-7", "realign_macro", RunLaunch{UserID: "u1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.NoteRemoteRun(run.ID, models.RunSilenceNote(8*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := database.FinishRemoteRun("", run.ID, "canceled", models.RunDisconnectNote); err != nil {
		t.Fatal(err)
	}
	finished, err := database.FinishMacroRunAs(Actor{ID: "u1"}, false, project.ID, "M-7", run.ID, "completed", "done")
	if err != nil || finished.Status != "completed" {
		t.Fatalf("the owner could not recover a silenced then closed run: %+v %v", finished, err)
	}
}
