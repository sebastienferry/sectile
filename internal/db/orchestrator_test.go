package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"tasks/internal/testsqlite"
)

func orchestratorTestDB(t *testing.T) (*DB, *models.Project) {
	t.Helper()
	d, err := testsqlite.New(t, filepath.Join(t.TempDir(), "test.db"), NewDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	d.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		return nil, nil
	})
	project, err := d.CreateProject(models.CreateProjectRequest{
		Name:                      "Orchestrated",
		IssueTracker:              "local",
		OrchestratorEnabled:       true,
		OrchestratorMaxConcurrent: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	return d, project
}

func orchestratorTask(t *testing.T, d *DB, projectID string) *models.Task {
	t.Helper()
	task, err := d.CreateTask(models.CreateTaskRequest{
		ProjectID: projectID, Title: "Implement request validation",
		Description: "Reject invalid requests and cover the boundary cases with tests.",
	})
	if err != nil {
		t.Fatal(err)
	}
	return task
}

// orchestratorDispatches records every launch sent to a workstation, sessions
// and chain steps alike.
func orchestratorDispatches(d *DB) <-chan agentprotocol.Operation {
	dispatched := make(chan agentprotocol.Operation, 32)
	d.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		if op.Action == "execute_skill" {
			dispatched <- op
		}
		return nil, nil
	})
	return dispatched
}

func awaitOrchestratorDispatch(t *testing.T, d *DB, dispatched <-chan agentprotocol.Operation) agentprotocol.Operation {
	t.Helper()
	select {
	case op := <-dispatched:
		if !d.jobs.drain(5 * time.Second) {
			t.Fatal("launch worker did not finish")
		}
		return op
	case <-time.After(5 * time.Second):
		t.Fatal("nothing was dispatched")
		return agentprotocol.Operation{}
	}
}

func expectNoDispatch(t *testing.T, d *DB, dispatched <-chan agentprotocol.Operation, why string) {
	t.Helper()
	if !d.jobs.drain(5 * time.Second) {
		t.Fatal("launch worker did not finish")
	}
	select {
	case op := <-dispatched:
		t.Fatalf("%s, yet %s was dispatched: %+v", why, op.SkillID, op)
	default:
	}
}

var aliceStart = OrchestrationStart{UserID: "alice", DeviceID: "alice-workstation", ProfileID: "docs", ProfileName: "Docs agent"}

// startSession starts orchestration and returns the session's dispatch.
func startSession(t *testing.T, d *DB, projectID string, dispatched <-chan agentprotocol.Operation) agentprotocol.Operation {
	t.Helper()
	if _, err := d.StartOrchestration(projectID, aliceStart); err != nil {
		t.Fatal(err)
	}
	op := awaitOrchestratorDispatch(t, d, dispatched)
	if op.Scope != agentprotocol.ScopeOrchestrator || op.SkillID != OrchestratorSkillID {
		t.Fatalf("expected an orchestrator session, got %+v", op)
	}
	return op
}

func TestStartOrchestrationDispatchesOneSessionToTheArmedWorkstation(t *testing.T) {
	d, project := orchestratorTestDB(t)
	dispatched := orchestratorDispatches(d)
	op := startSession(t, d, project.ID, dispatched)
	if op.UserID != "alice" || op.DeviceID != "alice-workstation" || op.ProfileID != "docs" || op.ProfileName != "Docs agent" ||
		op.TaskID != "" || op.Mode != models.SkillModeAutonomous || op.ProjectID != project.ID {
		t.Fatalf("session dispatched with the wrong identity: %+v", op)
	}
	session, err := d.ActiveOrchestratorSession(project.ID)
	if err != nil || session == nil || session.ID != op.RunID || session.RunScope != orchestratorRunScope || session.TaskID != "" {
		t.Fatalf("active session = %+v (%v), want run %s", session, err, op.RunID)
	}
	if isOrchestratedPrompt(op.Prompt) {
		t.Fatal("the session's prompt opens with the chain prefix, so the session would take a ticket slot")
	}
	if count, err := d.ActiveRunsCountForProject(project.ID); err != nil || count != 0 {
		t.Fatalf("the session took a ticket slot: %d (%v)", count, err)
	}

	again, err := d.StartOrchestration(project.ID, aliceStart)
	if !errors.Is(err, ErrOrchestratorSessionActive) || again == nil || again.ID != op.RunID {
		t.Fatalf("second start = %+v, %v; want the running session and ErrOrchestratorSessionActive", again, err)
	}
	expectNoDispatch(t, d, dispatched, "a second start while a session runs")

	other := aliceStart
	other.ProfileID, other.ProfileName = "tests", "Tests agent"
	if _, err := d.StartOrchestration(project.ID, other); !errors.Is(err, ErrOrchestratorArmedElsewhere) || !strings.Contains(err.Error(), "Docs agent") {
		t.Fatalf("starting another profile = %v, want a refusal naming the armed one", err)
	}
	elsewhere := aliceStart
	elsewhere.DeviceID = "bob-workstation"
	if _, err := d.StartOrchestration(project.ID, elsewhere); !errors.Is(err, ErrOrchestratorArmedElsewhere) {
		t.Fatalf("starting on another workstation = %v, want ErrOrchestratorArmedElsewhere", err)
	}
}

func TestStartOrchestrationRefusesDisabledAndUnknownProjects(t *testing.T) {
	d, project := orchestratorTestDB(t)
	disabled := false
	if _, err := d.UpdateProject(project.ID, models.UpdateProjectRequest{OrchestratorEnabled: &disabled}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.StartOrchestration(project.ID, aliceStart); !errors.Is(err, ErrOrchestratorDisabled) {
		t.Fatalf("disabled project = %v", err)
	}
	if _, err := d.StartOrchestration("missing", aliceStart); !errors.Is(err, ErrOrchestratorProjectNotFound) {
		t.Fatalf("unknown project = %v", err)
	}
}

func TestSessionPromptCarriesTheSharedPolicyOnlyWithoutAProfile(t *testing.T) {
	d, project := orchestratorTestDB(t)
	shared := "Only take tickets labelled backend."
	if _, err := d.UpdateProject(project.ID, models.UpdateProjectRequest{OrchestratorPrompt: &shared}); err != nil {
		t.Fatal(err)
	}
	project, _ = d.GetProjectByID(project.ID)
	if prompt := orchestratorSessionPrompt(project, OrchestrationStart{}); !strings.Contains(prompt, shared) {
		t.Fatalf("a session without a profile lost the shared policy: %q", prompt)
	}
	// A profile's policy is its file, which the workstation adds.
	if prompt := orchestratorSessionPrompt(project, aliceStart); strings.Contains(prompt, shared) {
		t.Fatalf("a profile's session carries the shared policy too: %q", prompt)
	}
}

func TestLaunchTaskChainAppliesEveryHardGuard(t *testing.T) {
	ctx := context.Background()
	d, project := orchestratorTestDB(t)
	dispatched := orchestratorDispatches(d)
	session := startSession(t, d, project.ID, dispatched)

	ready := orchestratorTask(t, d, project.ID)
	if _, err := d.LaunchTaskChain(ctx, "", ready.ID, "why"); !errors.Is(err, ErrNotOrchestratorSession) {
		t.Fatalf("a call without a session = %v", err)
	}
	if _, err := d.LaunchTaskChain(ctx, "not-a-run", ready.ID, "why"); !errors.Is(err, ErrNotOrchestratorSession) {
		t.Fatalf("a call from an unknown run = %v", err)
	}
	worker, err := d.StartAgentRun(orchestratorTask(t, d, project.ID).ID, "clarify", RunLaunch{UserID: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.LaunchTaskChain(ctx, worker.ID, ready.ID, "why"); !errors.Is(err, ErrNotOrchestratorSession) {
		t.Fatalf("a call from a ticket run = %v", err)
	}

	blocked, err := d.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "Blocked work", Description: "Waiting on a vendor answer before anything.", Labels: []string{"needs-info"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.LaunchTaskChain(ctx, session.RunID, blocked.ID, "why"); err == nil || !strings.Contains(err.Error(), "needs-info") {
		t.Fatalf("blocked ticket = %v, want a refusal naming the label", err)
	}
	implemented := orchestratorTask(t, d, project.ID)
	if _, err := d.conn.Exec("UPDATE tasks SET labels = ? WHERE id = ?", `["#implemented"]`, implemented.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.LaunchTaskChain(ctx, session.RunID, implemented.ID, "why"); err == nil || !strings.Contains(err.Error(), "implemented") {
		t.Fatalf("implemented ticket = %v, want a refusal", err)
	}
	other, err := d.CreateProject(models.CreateProjectRequest{Name: "Elsewhere", IssueTracker: "local"})
	if err != nil {
		t.Fatal(err)
	}
	foreign := orchestratorTask(t, d, other.ID)
	if _, err := d.LaunchTaskChain(ctx, session.RunID, foreign.ID, "why"); err == nil {
		t.Fatal("the orchestrator launched a ticket of another project")
	}
	busy := orchestratorTask(t, d, project.ID)
	if _, err := d.StartAgentRun(busy.ID, "clarify", RunLaunch{UserID: "bob"}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.LaunchTaskChain(ctx, session.RunID, busy.ID, "why"); err == nil {
		t.Fatal("the orchestrator started a second agent on a ticket somebody works on")
	}
	expectNoDispatch(t, d, dispatched, "every launch was refused")

	rationale := "Unblocks the two validation tickets behind it."
	run, err := d.LaunchTaskChain(ctx, session.RunID, ready.ID, rationale)
	if err != nil {
		t.Fatal(err)
	}
	op := awaitOrchestratorDispatch(t, d, dispatched)
	if op.SkillID != "clarify" || op.TaskID != ready.ID || op.UserID != "alice" || op.DeviceID != "alice-workstation" {
		t.Fatalf("chain dispatched with the wrong identity: %+v", op)
	}
	if !isOrchestratedPrompt(op.Prompt) || !strings.Contains(op.Prompt, rationale) || !strings.Contains(op.Prompt, "Docs agent") {
		t.Fatalf("chain prompt lost its prefix, rationale or profile: %q", op.Prompt)
	}
	if out := d.runOutcomeOf(op.RunID); out.ChainStop != models.FullChainStopImplemented || out.Mode != models.SkillModeAutonomous {
		t.Fatalf("chain metadata = %+v (launch %s)", out, run.ID)
	}

	second := orchestratorTask(t, d, project.ID)
	if _, err := d.LaunchTaskChain(ctx, session.RunID, second.ID, "fills the last slot"); err != nil {
		t.Fatal(err)
	}
	awaitOrchestratorDispatch(t, d, dispatched)
	third := orchestratorTask(t, d, project.ID)
	if _, err := d.LaunchTaskChain(ctx, session.RunID, third.ID, "over the cap"); !errors.Is(err, errOrchestratorCapacity) {
		t.Fatalf("a launch over the cap = %v, want errOrchestratorCapacity", err)
	}

	decisions, err := d.OrchestratorDecisions(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	launched := map[string]string{}
	for _, decision := range decisions {
		if decision.Decision == OrchestratorDecisionLaunched {
			launched[decision.TaskID] = decision.Reason
		}
	}
	if len(launched) != 2 || launched[ready.ID] != rationale {
		t.Fatalf("launch decisions = %+v", decisions)
	}

	if err := d.StopOrchestration(project.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.LaunchTaskChain(ctx, session.RunID, third.ID, "after stop"); err == nil || !strings.Contains(err.Error(), "stopped") {
		t.Fatalf("a launch after stop = %v", err)
	}
}

func TestOrchestratedChainCarriesItsReasonNotAGate(t *testing.T) {
	d, project := orchestratorTestDB(t)
	shared := "Only work on request validation."
	if _, err := d.UpdateProject(project.ID, models.UpdateProjectRequest{OrchestratorPrompt: &shared}); err != nil {
		t.Fatal(err)
	}
	task := orchestratorTask(t, d, project.ID)
	dispatched := orchestratorDispatches(d)
	session := startSession(t, d, project.ID, dispatched)
	rationale := "Smallest ticket with full context."
	if _, err := d.LaunchTaskChain(context.Background(), session.RunID, task.ID, rationale); err != nil {
		t.Fatal(err)
	}
	for i, skill := range []string{"clarify", "specify", "implement"} {
		op := awaitOrchestratorDispatch(t, d, dispatched)
		if op.SkillID != skill || op.UserID != "alice" || op.DeviceID != "alice-workstation" {
			t.Fatalf("step %d = %+v", i, op)
		}
		if !strings.Contains(op.Prompt, rationale) || strings.Contains(op.Prompt, shared) || strings.Contains(op.Prompt, "assess whether this ticket") {
			t.Fatalf("step %s should carry the reason and neither the policy nor an eligibility gate: %q", skill, op.Prompt)
		}
		next := []string{"clarified", "specified", "implemented"}[i]
		if _, err := d.conn.Exec("UPDATE tasks SET labels = ? WHERE id = ?", `["#`+next+`"]`, task.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := d.FinishRemoteRunAs(Actor{ID: "alice"}, false, task.ID, op.RunID, "completed", "Stage completed"); err != nil {
			t.Fatal(err)
		}
	}
	// The chain reached implemented, which frees its slot; the session still
	// runs, so the wake-up waits for it rather than starting a second one.
	expectNoDispatch(t, d, dispatched, "the chain stops at implemented and a session is still running")
	if armed, _ := d.OrchestratorArming(project.ID); armed == nil || !armed.WakePending {
		t.Fatalf("the freed slot was not remembered: %+v", armed)
	}
}

func TestOnlyOrchestratedRunsTakeASlot(t *testing.T) {
	d, project := orchestratorTestDB(t)
	manual := orchestratorTask(t, d, project.ID)
	if _, err := d.StartAgentRun(manual.ID, "adjust", RunLaunch{UserID: "someone", Prompt: "Fix the review comments"}); err != nil {
		t.Fatal(err)
	}
	if count, err := d.ActiveRunsCountForProject(project.ID); err != nil || count != 0 {
		t.Fatalf("a manual run took a slot: count %d (err: %v)", count, err)
	}
	orchestrated := orchestratorTask(t, d, project.ID)
	if _, err := d.StartAgentRun(orchestrated.ID, "clarify", RunLaunch{UserID: "admin", Prompt: orchestratedChainPrompt("", "", "why")}); err != nil {
		t.Fatal(err)
	}
	if count, err := d.ActiveRunsCountForProject(project.ID); err != nil || count != 1 {
		t.Fatalf("an orchestrated run on a workstation took no slot: count %d (err: %v)", count, err)
	}
}

func TestDeferredTicketIsFlaggedUntilItChanges(t *testing.T) {
	d, project := orchestratorTestDB(t)
	dispatched := orchestratorDispatches(d)
	session := startSession(t, d, project.ID, dispatched)
	task := orchestratorTask(t, d, project.ID)
	if err := d.DeferTask(session.RunID, task.ID, ""); err == nil {
		t.Fatal("a deferral without a reason was accepted")
	}
	if err := d.DeferTask("not-a-run", task.ID, "no"); !errors.Is(err, ErrNotOrchestratorSession) {
		t.Fatalf("a deferral from another caller = %v", err)
	}
	reason := "Waits for the API contract of PE-12."
	if err := d.DeferTask(session.RunID, task.ID, reason); err != nil {
		t.Fatal(err)
	}
	state, err := d.OrchestrationState(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Decisions) != 1 || state.Decisions[0].Reason != reason || !state.Decisions[0].UnchangedSinceDecision ||
		state.Decisions[0].Decision != OrchestratorDecisionDeferred || state.Session == nil || state.Armed == nil || state.AvailableSlots != 2 {
		t.Fatalf("state after a deferral = %+v", state)
	}
	time.Sleep(10 * time.Millisecond)
	title := "Implement request validation, contract agreed"
	if _, err := d.UpdateTask(task.ID, models.UpdateTaskRequest{Title: &title}); err != nil {
		t.Fatal(err)
	}
	decisions, err := d.OrchestratorDecisions(project.ID)
	if err != nil || len(decisions) != 1 || decisions[0].UnchangedSinceDecision {
		t.Fatalf("an edited ticket still reads as unchanged since its deferral: %+v (%v)", decisions, err)
	}
}

// A workstation that is connected but refuses the session fails it; only an
// offline one makes it wait.
func TestRefusingWorkstationFailsTheSessionAndKeepsOrchestrationStarted(t *testing.T) {
	d, project := orchestratorTestDB(t)
	d.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		return nil, fmt.Errorf("no local agent connected")
	})
	run, err := d.StartOrchestration(project.ID, aliceStart)
	if err != nil {
		t.Fatal(err)
	}
	if !d.jobs.drain(5 * time.Second) {
		t.Fatal("dispatch did not finish")
	}
	failed, err := d.GetActivityByID(run.ID)
	if err != nil || failed == nil || failed.Status != "failed" || !strings.Contains(failed.Summary, "could not reach its workstation") {
		t.Fatalf("session after an unreachable workstation = %+v (%v)", failed, err)
	}
	if armed, _ := d.OrchestratorArming(project.ID); armed == nil {
		t.Fatal("an unreachable workstation stopped orchestration")
	}
	if session, _ := d.ActiveOrchestratorSession(project.ID); session != nil {
		t.Fatalf("a failed session still reads as active: %+v", session)
	}
}

func TestAgentProjectDiscoveryReportsOrchestrationCapacity(t *testing.T) {
	d, project := orchestratorTestDB(t)
	task := orchestratorTask(t, d, project.ID)
	if _, err := d.StartAgentRun(task.ID, "clarify", RunLaunch{ProjectID: project.ID, Prompt: orchestratedChainPrompt("", "", "")}); err != nil {
		t.Fatal(err)
	}
	discovered, err := d.AgentProjects()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range discovered.Projects {
		if entry.ID != project.ID {
			continue
		}
		if !entry.OrchestratorEnabled || entry.OrchestratorMaxConcurrent != 2 || entry.OrchestratorActiveRuns == nil || *entry.OrchestratorActiveRuns != 1 {
			t.Fatalf("incorrect orchestration capacity: %+v", entry)
		}
		return
	}
	t.Fatal("orchestrated project missing from discovery")
}

func TestOrchestratorAdmissionReadsTheCurrentProjectCap(t *testing.T) {
	d, project := orchestratorTestDB(t)
	dispatched := orchestratorDispatches(d)
	session := startSession(t, d, project.ID, dispatched)
	for i := 0; i < 2; i++ {
		if _, err := d.LaunchTaskChain(context.Background(), session.RunID, orchestratorTask(t, d, project.ID).ID, "why"); err != nil {
			t.Fatal(err)
		}
		awaitOrchestratorDispatch(t, d, dispatched)
	}
	// Lowering the cap applies at the next admission.
	limit := 1
	if _, err := d.UpdateProject(project.ID, models.UpdateProjectRequest{OrchestratorMaxConcurrent: &limit}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.LaunchTaskChain(context.Background(), session.RunID, orchestratorTask(t, d, project.ID).ID, "why"); !errors.Is(err, errOrchestratorCapacity) {
		t.Fatalf("admission ignored the lowered project cap: %v", err)
	}
}

func TestManualChainDoesNotRepeatItsLaunchPrompt(t *testing.T) {
	d, project := orchestratorTestDB(t)
	task := orchestratorTask(t, d, project.ID)
	dispatched := orchestratorDispatches(d)
	manual := "Focus the clarification on the error messages."
	if _, _, err := d.enqueueSkillWithOptions(task.ID, project.ID, "clarify", manual, true, models.SkillModeAutonomous, "", queuedSkillOptions{UserID: "alice"}); err != nil {
		t.Fatal(err)
	}
	op := awaitOrchestratorDispatch(t, d, dispatched)
	if !strings.Contains(op.Prompt, manual) {
		t.Fatalf("first step lost its launch prompt: %q", op.Prompt)
	}
	if _, err := d.conn.Exec("UPDATE tasks SET labels = ? WHERE id = ?", `["#clarified"]`, task.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.FinishRemoteRunAs(Actor{ID: "alice"}, false, task.ID, op.RunID, "completed", "Stage completed"); err != nil {
		t.Fatal(err)
	}
	next := awaitOrchestratorDispatch(t, d, dispatched)
	if next.SkillID != "specify" || strings.Contains(next.Prompt, manual) {
		t.Fatalf("manual prompt leaked into the next step: %+v", next)
	}
	if next.UserID != "alice" {
		t.Fatalf("chained step lost its caller: %+v", next)
	}
}

// An armed workstation that is offline when a slot frees keeps the session
// queued, with no failover to another workstation, and gets it when it
// connects again (ADR 0064).
func TestOfflineWorkstationKeepsTheWokenSessionUntilItConnects(t *testing.T) {
	d, project := orchestratorTestDB(t)
	dispatched := orchestratorDispatches(d)
	online := true
	d.SetAgentSelection(func(user, projectID, device string) (string, error) {
		if !online {
			return "", fmt.Errorf("no local agent connected")
		}
		return device, nil
	})
	first := startSession(t, d, project.ID, dispatched)
	finishSession(t, d, project.ID, first.RunID)

	online = false
	d.wakeOrchestrator(project.ID, false)
	expectNoDispatch(t, d, dispatched, "the armed workstation is offline")
	waiting, err := d.ActiveOrchestratorSession(project.ID)
	if err != nil || waiting == nil || waiting.Status != "queued" {
		t.Fatalf("the woken session should wait, queued: %+v %v", waiting, err)
	}
	if state, _ := d.RunDispatchState(waiting.ID); state != orchestratorWaitingDispatch {
		t.Fatalf("dispatch state = %q", state)
	}
	// Tools stay closed to a session that has not started.
	if err := d.DeferTask(waiting.ID, orchestratorTask(t, d, project.ID).ID, "too early"); !errors.Is(err, ErrNotOrchestratorSession) {
		t.Fatalf("a waiting session deferred a ticket: %v", err)
	}

	// Another workstation connecting takes nothing.
	online = true
	d.ResumeOrchestratorSessions("alice", "bob-workstation")
	expectNoDispatch(t, d, dispatched, "only the armed workstation runs the session")

	d.ResumeOrchestratorSessions("alice", "alice-workstation")
	op := awaitOrchestratorDispatch(t, d, dispatched)
	if op.RunID != waiting.ID || op.DeviceID != "alice-workstation" || op.ProfileID != "docs" {
		t.Fatalf("resumed dispatch = %+v", op)
	}
	if resumed, _ := d.GetActivityByID(waiting.ID); resumed == nil || resumed.Status != "running" {
		t.Fatalf("resumed session = %+v", resumed)
	}
	// A second connection of the same workstation sends nothing twice.
	d.ResumeOrchestratorSessions("alice", "alice-workstation")
	expectNoDispatch(t, d, dispatched, "the session was already sent")
}

func TestStoppingCancelsASessionWaitingForItsWorkstation(t *testing.T) {
	d, project := orchestratorTestDB(t)
	dispatched := orchestratorDispatches(d)
	online := true
	d.SetAgentSelection(func(user, projectID, device string) (string, error) {
		if !online {
			return "", fmt.Errorf("no local agent connected")
		}
		return device, nil
	})
	first := startSession(t, d, project.ID, dispatched)
	finishSession(t, d, project.ID, first.RunID)
	online = false
	d.wakeOrchestrator(project.ID, false)
	expectNoDispatch(t, d, dispatched, "the armed workstation is offline")
	waiting, _ := d.ActiveOrchestratorSession(project.ID)
	if waiting == nil {
		t.Fatal("no session waits")
	}
	if err := d.StopOrchestration(project.ID); err != nil {
		t.Fatal(err)
	}
	if canceled, _ := d.GetActivityByID(waiting.ID); canceled == nil || canceled.Status != "canceled" {
		t.Fatalf("a stopped project kept its waiting session: %+v", canceled)
	}
	online = true
	d.ResumeOrchestratorSessions("alice", "alice-workstation")
	expectNoDispatch(t, d, dispatched, "orchestration was stopped")
}
