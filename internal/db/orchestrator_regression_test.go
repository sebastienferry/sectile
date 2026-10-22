package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"tasks/internal/agentprotocol"
	"tasks/internal/models"
)

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

func awaitOrchestratorDispatch(t *testing.T, d *DB, dispatched <-chan agentprotocol.Operation) agentprotocol.Operation {
	t.Helper()
	select {
	case op := <-dispatched:
		if !d.jobs.drain(5 * time.Second) {
			t.Fatal("launch worker did not finish")
		}
		return op
	case <-time.After(5 * time.Second):
		t.Fatal("orchestrator did not dispatch the task")
		return agentprotocol.Operation{}
	}
}

func orchestratorDispatches(d *DB) <-chan agentprotocol.Operation {
	dispatched := make(chan agentprotocol.Operation, 10)
	d.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		if op.Action == "execute_skill" {
			dispatched <- op
		}
		return nil, nil
	})
	return dispatched
}

func TestOrchestratorKeepsCallerAndGuidelinesThroughEveryStep(t *testing.T) {
	d, project := orchestratorTestDB(t)
	guidelines := "Only work on request validation. Leave database schema changes to a human."
	if _, err := d.UpdateProject(project.ID, models.UpdateProjectRequest{OrchestratorPrompt: &guidelines}); err != nil {
		t.Fatal(err)
	}
	task := orchestratorTask(t, d, project.ID)
	dispatched := orchestratorDispatches(d)
	d.SetAgentSelection(func(userID, projectID, deviceID string) (string, error) {
		if userID != "alice" || projectID != project.ID {
			return "", fmt.Errorf("unexpected route owner %q or project %q", userID, projectID)
		}
		if deviceID != "" && deviceID != "alice-workstation" {
			return "", fmt.Errorf("unexpected device %q", deviceID)
		}
		return "alice-workstation", nil
	})
	result, err := d.EvaluateAndOrchestrateBacklog(context.Background(), project.ID, Actor{ID: "alice"}, models.OrchestrateProjectRequest{})
	if err != nil || len(result.Launched) != 1 {
		t.Fatalf("orchestrate: %+v, %v", result, err)
	}
	launch, err := d.GetActivityByID(result.Launched[0].ActivityID)
	if err != nil || launch == nil || launch.UserID != "alice" {
		t.Fatalf("launch lost caller: %+v, %v", launch, err)
	}
	for i, skill := range []string{"clarify", "specify", "implement"} {
		op := awaitOrchestratorDispatch(t, d, dispatched)
		if op.UserID != "alice" || op.DeviceID != "alice-workstation" || op.SkillID != skill {
			t.Fatalf("step %d dispatched to wrong owner/device/skill: %+v", i, op)
		}
		if !strings.Contains(op.Prompt, guidelines) || !strings.Contains(op.Prompt, "before making changes") || !strings.Contains(op.Prompt, "without making changes or advancing its stage") {
			t.Fatalf("step %s lost eligibility gate: %q", skill, op.Prompt)
		}
		out := d.runOutcomeOf(op.RunID)
		if out.UserID != "alice" || out.ChainStop != models.FullChainStopImplemented || out.Mode != models.SkillModeAutonomous {
			t.Fatalf("step %s lost chain metadata: %+v", skill, out)
		}
		// Model an agent reporting successful stage work; stage validation itself is
		// covered by the transition tests, independently of queue hand-back.
		nextStage := []string{"clarified", "specified", "implemented"}[i]
		if _, err := d.conn.Exec("UPDATE tasks SET labels = ? WHERE id = ?", `["#`+nextStage+`"]`, task.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := d.FinishRemoteRunAs(Actor{ID: "alice"}, false, task.ID, op.RunID, "completed", "Stage completed"); err != nil {
			t.Fatal(err)
		}
	}
	if !d.jobs.drain(5 * time.Second) {
		t.Fatal("chain did not drain")
	}
	select {
	case op := <-dispatched:
		t.Fatalf("chain continued past implemented: %+v", op)
	default:
	}
	count, err := d.ActiveRunsCountForProject(project.ID)
	if err != nil || count != 0 {
		t.Fatalf("completed chain still active: %d, %v", count, err)
	}
}

func TestOrchestratorLeavesImplementedTicketsForHumanReview(t *testing.T) {
	d, project := orchestratorTestDB(t)
	task := orchestratorTask(t, d, project.ID)
	if project.FullChainStopStage != models.FullChainStopReviewed {
		t.Fatalf("fixture should default to reviewed, got %s", project.FullChainStopStage)
	}
	if _, err := d.conn.Exec("UPDATE tasks SET labels = ? WHERE id = ?", `["#implemented"]`, task.ID); err != nil {
		t.Fatal(err)
	}
	result, err := d.EvaluateAndOrchestrateBacklog(context.Background(), project.ID, Actor{ID: "alice"}, models.OrchestrateProjectRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Launched) != 0 || result.EvaluatedCount != 0 {
		t.Fatalf("implemented ticket entered backlog orchestration: %+v", result)
	}
	if activities := mustActivities(t, d, task.ID); len(activities) != 0 {
		t.Fatalf("implemented ticket was touched: %+v", activities)
	}
}

func TestOrchestratorGuidelineRefusalDoesNotAdvanceOrContinue(t *testing.T) {
	d, project := orchestratorTestDB(t)
	guidelines := "Do not work on database schema changes."
	if _, err := d.UpdateProject(project.ID, models.UpdateProjectRequest{OrchestratorPrompt: &guidelines}); err != nil {
		t.Fatal(err)
	}
	task, err := d.CreateTask(models.CreateTaskRequest{
		ProjectID: project.ID, Title: "Add an audit column",
		Description: "Add an audit timestamp column to the database schema and migrate existing rows.",
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatched := orchestratorDispatches(d)
	if _, err := d.EvaluateAndOrchestrateBacklog(context.Background(), project.ID, Actor{ID: "alice"}, models.OrchestrateProjectRequest{}); err != nil {
		t.Fatal(err)
	}
	op := awaitOrchestratorDispatch(t, d, dispatched)
	if !strings.Contains(op.Prompt, guidelines) {
		t.Fatalf("agent never received project restrictions: %q", op.Prompt)
	}
	// An agent refuses the ticket without reporting a stage transition. Even a
	// zero-exit completion must not allow the chain to start the following skill.
	if _, err := d.FinishRemoteRun(task.ID, op.RunID, "completed", "Project guidelines exclude this task"); err != nil {
		t.Fatal(err)
	}
	fresh, err := d.GetTaskByID(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.StageOfTask(fresh) != "new" {
		t.Fatalf("refused ticket advanced: %+v", fresh)
	}
	if !strings.Contains(runSummary(t, d, op.RunID), "full chain stops here") {
		t.Fatal("refused chain did not record its stop")
	}
	if count := countSkillActivities(t, d, task.ID, skillName(t, d, "specify")); count != 0 {
		t.Fatalf("refused ticket started %d specification activities", count)
	}
}

func checkConcurrentOrchestratorCapacity(t *testing.T, a, b *DB) {
	t.Helper()
	project, err := a.CreateProject(models.CreateProjectRequest{Name: "Concurrent orchestration", IssueTracker: "local", OrchestratorEnabled: true, OrchestratorMaxConcurrent: 2})
	if err != nil {
		t.Fatal(err)
	}
	noop := func(context.Context, agentprotocol.Operation) (json.RawMessage, error) { return nil, nil }
	a.SetAgentOperations(noop)
	b.SetAgentOperations(noop)
	for i := 0; i < 12; i++ {
		orchestratorTask(t, a, project.ID)
	}
	// A manual session also consumes capacity; exactly one new task may enter.
	existing := orchestratorTask(t, a, project.ID)
	if _, err := a.StartAgentRun(existing.ID, "clarify", RunLaunch{ProjectID: project.ID}); err != nil {
		t.Fatal(err)
	}
	results := make(chan *models.OrchestrationResult, 16)
	race(16, a, b, func(i int, d *DB) {
		projectID := project.ID
		if i%2 == 1 {
			projectID = project.Slug
		}
		result, err := d.EvaluateAndOrchestrateBacklog(context.Background(), projectID, Actor{ID: "alice"}, models.OrchestrateProjectRequest{})
		if err != nil {
			t.Errorf("request %d: %v", i, err)
			return
		}
		results <- result
	})
	close(results)
	launched := 0
	for result := range results {
		launched += len(result.Launched)
	}
	if launched != 1 {
		t.Fatalf("concurrent requests launched %d tasks with one remaining slot", launched)
	}
	count, err := a.ActiveRunsCountForProject(project.ID)
	if err != nil || count != 2 {
		t.Fatalf("project cap is 2, active count = %d, err = %v", count, err)
	}
}

func TestOrchestratorConcurrentRequestsRespectCapacity(t *testing.T) {
	d, _ := orchestratorTestDB(t)
	checkConcurrentOrchestratorCapacity(t, d, d)
}

func TestPostgresOrchestratorConcurrentInstancesRespectCapacity(t *testing.T) {
	a, b := openPostgresPair(t)
	checkConcurrentOrchestratorCapacity(t, a, b)
}

func TestAgentProjectDiscoveryReportsOrchestrationCapacity(t *testing.T) {
	d, project := orchestratorTestDB(t)
	task := orchestratorTask(t, d, project.ID)
	if _, err := d.StartAgentRun(task.ID, "clarify", RunLaunch{ProjectID: project.ID}); err != nil {
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

func TestOrchestratorLocalGuidelinesSnapshotDoesNotOverwriteProject(t *testing.T) {
	d, project := orchestratorTestDB(t)
	original := "Web project policy"
	if _, err := d.UpdateProject(project.ID, models.UpdateProjectRequest{OrchestratorPrompt: &original}); err != nil {
		t.Fatal(err)
	}
	orchestratorTask(t, d, project.ID)
	dispatched := orchestratorDispatches(d)
	local := "Local AGENTS.md policy"
	result, err := d.EvaluateAndOrchestrateBacklog(context.Background(), project.ID, Actor{ID: "alice"}, models.OrchestrateProjectRequest{Guidelines: &local})
	if err != nil || len(result.Launched) != 1 {
		t.Fatalf("evaluate: %+v %v", result, err)
	}
	op := awaitOrchestratorDispatch(t, d, dispatched)
	if !strings.Contains(op.Prompt, local) || strings.Contains(op.Prompt, original) {
		t.Fatalf("wrong guideline snapshot: %s", op.Prompt)
	}
	stored, err := d.GetProjectByID(project.ID)
	if err != nil || stored.OrchestratorPrompt != original {
		t.Fatal("local snapshot changed shared policy")
	}
	oversized := strings.Repeat("x", models.OrchestratorGuidelinesMaxBytes+1)
	if _, err := d.EvaluateAndOrchestrateBacklog(context.Background(), project.ID, Actor{ID: "alice"}, models.OrchestrateProjectRequest{Guidelines: &oversized}); err == nil {
		t.Fatal("accepted oversized guidelines")
	}
}

func TestNamedAgentPoliciesShareTheProjectConcurrencyLimit(t *testing.T) {
	d, project := orchestratorTestDB(t)
	limit := 1
	if _, err := d.UpdateProject(project.ID, models.UpdateProjectRequest{OrchestratorMaxConcurrent: &limit}); err != nil {
		t.Fatal(err)
	}
	orchestratorTask(t, d, project.ID)
	orchestratorTask(t, d, project.ID)
	dispatched := orchestratorDispatches(d)
	docs, tests := "Documentation agent policy", "Test agent policy"
	first, err := d.EvaluateAndOrchestrateBacklog(context.Background(), project.ID, Actor{ID: "alice"}, models.OrchestrateProjectRequest{Guidelines: &docs})
	if err != nil || len(first.Launched) != 1 {
		t.Fatalf("first agent: %+v %v", first, err)
	}
	op := awaitOrchestratorDispatch(t, d, dispatched)
	if !strings.Contains(op.Prompt, docs) {
		t.Fatal("wrong agent policy dispatched")
	}
	second, err := d.EvaluateAndOrchestrateBacklog(context.Background(), project.ID, Actor{ID: "alice"}, models.OrchestrateProjectRequest{Guidelines: &tests})
	if err != nil || len(second.Launched) != 0 || second.ActiveRunsBefore != 1 {
		t.Fatalf("second agent bypassed shared capacity: %+v %v", second, err)
	}
}
