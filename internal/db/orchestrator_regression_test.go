package db

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"tasks/internal/agentprotocol"
	"tasks/internal/models"
)

// launchAndFail launches a ticket's chain from the session and ends its first
// step as failed, which stops the chain and frees its slot.
func launchAndFail(t *testing.T, d *DB, projectID, sessionRunID string, dispatched <-chan agentprotocol.Operation) {
	t.Helper()
	task := orchestratorTask(t, d, projectID)
	if _, err := d.LaunchTaskChain(context.Background(), sessionRunID, task.ID, "why"); err != nil {
		t.Fatal(err)
	}
	op := awaitOrchestratorDispatch(t, d, dispatched)
	if _, err := d.FinishRemoteRunAs(Actor{ID: "alice"}, false, task.ID, op.RunID, "failed", "Tests failed"); err != nil {
		t.Fatal(err)
	}
}

func finishSession(t *testing.T, d *DB, projectID, runID string) {
	t.Helper()
	if _, err := d.FinishOrchestratorSession(Actor{ID: "alice"}, false, projectID, runID, "completed", "Launched one, deferred none"); err != nil {
		t.Fatal(err)
	}
}

func TestFreedSlotWakesAnArmedOrchestrator(t *testing.T) {
	d, project := orchestratorTestDB(t)
	dispatched := orchestratorDispatches(d)
	session := startSession(t, d, project.ID, dispatched)
	task := orchestratorTask(t, d, project.ID)
	if _, err := d.LaunchTaskChain(context.Background(), session.RunID, task.ID, "why"); err != nil {
		t.Fatal(err)
	}
	step := awaitOrchestratorDispatch(t, d, dispatched)
	finishSession(t, d, project.ID, session.RunID)
	expectNoDispatch(t, d, dispatched, "a session that freed no slot ended")

	if _, err := d.FinishRemoteRunAs(Actor{ID: "alice"}, false, task.ID, step.RunID, "failed", "Tests failed"); err != nil {
		t.Fatal(err)
	}
	next := awaitOrchestratorDispatch(t, d, dispatched)
	if next.Scope != agentprotocol.ScopeOrchestrator || next.RunID == session.RunID || next.ProfileID != "docs" || next.DeviceID != "alice-workstation" {
		t.Fatalf("the freed slot did not wake a new session on the armed workstation: %+v", next)
	}
}

func TestChainEndingDuringASessionWakesOneSessionAfterIt(t *testing.T) {
	d, project := orchestratorTestDB(t)
	dispatched := orchestratorDispatches(d)
	session := startSession(t, d, project.ID, dispatched)
	launchAndFail(t, d, project.ID, session.RunID, dispatched)
	launchAndFail(t, d, project.ID, session.RunID, dispatched)
	expectNoDispatch(t, d, dispatched, "the session is still running")

	finishSession(t, d, project.ID, session.RunID)
	next := awaitOrchestratorDispatch(t, d, dispatched)
	if next.Scope != agentprotocol.ScopeOrchestrator {
		t.Fatalf("expected one follow-up session, got %+v", next)
	}
	expectNoDispatch(t, d, dispatched, "two chain ends coalesce into one follow-up session")
	if armed, _ := d.OrchestratorArming(project.ID); armed == nil || armed.WakePending {
		t.Fatalf("the pending wake-up was not consumed: %+v", armed)
	}
}

func TestSessionThatLaunchesNothingWakesNothing(t *testing.T) {
	d, project := orchestratorTestDB(t)
	dispatched := orchestratorDispatches(d)
	session := startSession(t, d, project.ID, dispatched)
	if err := d.DeferTask(session.RunID, orchestratorTask(t, d, project.ID).ID, "Lacks acceptance criteria."); err != nil {
		t.Fatal(err)
	}
	finishSession(t, d, project.ID, session.RunID)
	expectNoDispatch(t, d, dispatched, "an exhausted backlog stays quiet")
}

func TestStoppedOrDisabledOrchestrationWakesNothing(t *testing.T) {
	for _, stop := range []string{"stop", "disable"} {
		t.Run(stop, func(t *testing.T) {
			d, project := orchestratorTestDB(t)
			dispatched := orchestratorDispatches(d)
			session := startSession(t, d, project.ID, dispatched)
			task := orchestratorTask(t, d, project.ID)
			if _, err := d.LaunchTaskChain(context.Background(), session.RunID, task.ID, "why"); err != nil {
				t.Fatal(err)
			}
			step := awaitOrchestratorDispatch(t, d, dispatched)
			finishSession(t, d, project.ID, session.RunID)
			if stop == "stop" {
				if err := d.StopOrchestration(project.ID); err != nil {
					t.Fatal(err)
				}
			} else {
				disabled := false
				if _, err := d.UpdateProject(project.ID, models.UpdateProjectRequest{OrchestratorEnabled: &disabled}); err != nil {
					t.Fatal(err)
				}
			}
			if armed, _ := d.OrchestratorArming(project.ID); armed != nil {
				t.Fatalf("orchestration still started after %s: %+v", stop, armed)
			}
			// The running chain continues; its end wakes nothing.
			if _, err := d.FinishRemoteRunAs(Actor{ID: "alice"}, false, task.ID, step.RunID, "failed", "Tests failed"); err != nil {
				t.Fatal(err)
			}
			expectNoDispatch(t, d, dispatched, "orchestration was stopped")
		})
	}
}

func checkConcurrentOrchestratorStarts(t *testing.T, a, b *DB) {
	t.Helper()
	project, err := a.CreateProject(models.CreateProjectRequest{Name: "Concurrent orchestration", IssueTracker: "local", OrchestratorEnabled: true, OrchestratorMaxConcurrent: 2})
	if err != nil {
		t.Fatal(err)
	}
	noop := func(context.Context, agentprotocol.Operation) (json.RawMessage, error) { return nil, nil }
	a.SetAgentOperations(noop)
	b.SetAgentOperations(noop)
	started := make(chan string, 16)
	race(16, a, b, func(i int, d *DB) {
		run, err := d.StartOrchestration(project.ID, aliceStart)
		if err == nil {
			started <- run.ID
		} else if !errors.Is(err, ErrOrchestratorSessionActive) {
			t.Errorf("start %d: %v", i, err)
		}
	})
	close(started)
	if count := len(started); count != 1 {
		t.Fatalf("concurrent starts created %d orchestrator sessions, want one", count)
	}
}

func checkConcurrentOrchestratorCapacity(t *testing.T, a, b *DB) {
	t.Helper()
	project, err := a.CreateProject(models.CreateProjectRequest{Name: "Concurrent launches", IssueTracker: "local", OrchestratorEnabled: true, OrchestratorMaxConcurrent: 2})
	if err != nil {
		t.Fatal(err)
	}
	noop := func(context.Context, agentprotocol.Operation) (json.RawMessage, error) { return nil, nil }
	a.SetAgentOperations(noop)
	b.SetAgentOperations(noop)
	session, err := a.StartOrchestration(project.ID, aliceStart)
	if err != nil {
		t.Fatal(err)
	}
	// An orchestrated run already holds a slot; exactly one new task may enter.
	existing := orchestratorTask(t, a, project.ID)
	if _, err := a.StartAgentRun(existing.ID, "clarify", RunLaunch{ProjectID: project.ID, Prompt: orchestratedChainPrompt("", "", "")}); err != nil {
		t.Fatal(err)
	}
	var tasks []string
	for i := 0; i < 16; i++ {
		tasks = append(tasks, orchestratorTask(t, a, project.ID).ID)
	}
	launched := make(chan struct{}, 16)
	race(16, a, b, func(i int, d *DB) {
		if _, err := d.LaunchTaskChain(context.Background(), session.ID, tasks[i], "why"); err == nil {
			launched <- struct{}{}
		} else if !errors.Is(err, errOrchestratorCapacity) {
			t.Errorf("launch %d: %v", i, err)
		}
	})
	close(launched)
	if count := len(launched); count != 1 {
		t.Fatalf("concurrent launches admitted %d tickets with one remaining slot", count)
	}
	if count, err := a.ActiveRunsCountForProject(project.ID); err != nil || count != 2 {
		t.Fatalf("project cap is 2, active count = %d, err = %v", count, err)
	}
}

func TestOrchestratorConcurrentStartsCreateOneSession(t *testing.T) {
	d, _ := orchestratorTestDB(t)
	checkConcurrentOrchestratorStarts(t, d, d)
}

func TestPostgresOrchestratorConcurrentStartsCreateOneSession(t *testing.T) {
	a, b := openPostgresPair(t)
	checkConcurrentOrchestratorStarts(t, a, b)
}

func TestOrchestratorConcurrentLaunchesRespectCapacity(t *testing.T) {
	d, _ := orchestratorTestDB(t)
	checkConcurrentOrchestratorCapacity(t, d, d)
}

func TestPostgresOrchestratorConcurrentInstancesRespectCapacity(t *testing.T) {
	a, b := openPostgresPair(t)
	checkConcurrentOrchestratorCapacity(t, a, b)
}
