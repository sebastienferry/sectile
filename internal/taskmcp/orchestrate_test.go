package taskmcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tasks/internal/agentprotocol"
	"tasks/internal/db"
	"tasks/internal/models"
	"tasks/internal/testsqlite"
)

// orchestrate_project starts an orchestrator session; the session alone may
// launch or defer tickets, and every hard guard is the server's.
func TestOrchestrateProjectMCPTool(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "test.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		return nil, nil
	})

	project, err := database.CreateProject(models.CreateProjectRequest{
		Name:                      "MCPOrchestrator",
		IssueTracker:              "local",
		OrchestratorEnabled:       true,
		OrchestratorMaxConcurrent: 2,
		OrchestratorPrompt:        "Only implement backend service changes.",
	})
	if err != nil {
		t.Fatal(err)
	}
	ready, err := database.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "Setup Backend Service", Description: "Detailed task description for automated autonomous development."})
	if err != nil {
		t.Fatal(err)
	}
	vague, err := database.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "Polish", Description: "?"})
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := database.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "Wait for the vendor", Description: "Blocked on the vendor's API.", Labels: []string{"blocked"}})
	if err != nil {
		t.Fatal(err)
	}

	res, err := call(t, database, "orchestrate_project", map[string]any{"projectId": project.ID})
	if err != nil {
		t.Fatalf("call orchestrate_project failed: %v", err)
	}
	session, _ := res["session"].(map[string]any)
	runID, _ := session["id"].(string)
	if runID == "" || session["runScope"] != "orchestrator" {
		t.Fatalf("expected an orchestrator session, got %+v", res)
	}

	// Nobody but the running session may launch or defer, the starter included.
	if _, err := call(t, database, "launch_task_chain", map[string]any{"taskKey": ready.ID, "rationale": "now"}); err == nil {
		t.Fatal("launch_task_chain accepted a caller outside the session")
	}
	if _, err := call(t, database, "defer_task", map[string]any{"taskKey": vague.ID, "reason": "vague"}); err == nil {
		t.Fatal("defer_task accepted a caller outside the session")
	}

	inSession := func(name string, args map[string]any) (map[string]any, error) {
		t.Helper()
		return callAsInRun(t, database, &tester, runID, name, args)
	}
	state, err := inSession("get_orchestration_state", map[string]any{})
	if err != nil || state["projectId"] != project.ID || state["availableSlots"] != float64(2) {
		t.Fatalf("state from the session: %+v %v", state, err)
	}
	if _, err := inSession("launch_task_chain", map[string]any{"taskKey": blocked.ID, "rationale": "now"}); err == nil || !strings.Contains(err.Error(), "blocker label") {
		t.Fatalf("a blocked ticket was launched: %v", err)
	}
	launched, err := inSession("launch_task_chain", map[string]any{"taskKey": ready.ID, "rationale": "Unblocks the API work."})
	if err != nil {
		t.Fatalf("launch_task_chain: %v", err)
	}
	if run, _ := launched["run"].(map[string]any); run == nil || !strings.Contains(fmt.Sprint(run["prompt"]), "Unblocks the API work.") {
		t.Fatalf("the chain does not carry the rationale: %+v", launched)
	}
	if _, err := inSession("launch_task_chain", map[string]any{"taskKey": ready.ID, "rationale": "again"}); err == nil {
		t.Fatal("a busy ticket was launched twice")
	}
	if _, err := inSession("defer_task", map[string]any{"taskKey": vague.ID, "reason": "No acceptance criteria yet."}); err != nil {
		t.Fatalf("defer_task: %v", err)
	}
	decisions, err := database.OrchestratorDecisions(project.ID)
	if err != nil || len(decisions) != 2 {
		t.Fatalf("decisions: %+v %v", decisions, err)
	}
	if _, err := inSession("finish_run", map[string]any{"projectId": project.ID, "runId": runID, "status": "completed", "note": "Launched one, deferred one."}); err != nil {
		t.Fatalf("finish_run on the session: %v", err)
	}
	if _, err := inSession("defer_task", map[string]any{"taskKey": vague.ID, "reason": "late"}); err == nil {
		t.Fatal("a finished session still deferred a ticket")
	}
}

func TestOrchestrateProjectMCPDispatchesAsCaller(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "test.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	dispatched := make(chan agentprotocol.Operation, 1)
	database.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		if op.Action == "execute_skill" {
			dispatched <- op
		}
		return nil, nil
	})
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Caller orchestration", IssueTracker: "local", OrchestratorEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	database.SetAgentSelection(func(user, projectID, device string) (string, error) {
		if user != tester.UserID || projectID != project.ID {
			return "", fmt.Errorf("wrong caller route: %q / %q", user, projectID)
		}
		return "test-workstation", nil
	})
	if _, err := call(t, database, "orchestrate_project", map[string]any{"projectId": project.ID}); err != nil {
		t.Fatal(err)
	}
	select {
	case op := <-dispatched:
		if op.UserID != tester.UserID || op.TaskID != "" || op.Scope != agentprotocol.ScopeOrchestrator || op.SkillID != db.OrchestratorSkillID || op.DeviceID != "test-workstation" {
			t.Fatalf("unexpected dispatch: %+v", op)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("MCP caller's workstation never received the orchestrator session")
	}
}

func TestOrchestrateProjectMCPRefusesOverrides(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "test.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		return nil, nil
	})
	disabled, err := database.CreateProject(models.CreateProjectRequest{Name: "Disabled MCP orchestration", IssueTracker: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.CreateTask(models.CreateTaskRequest{ProjectID: disabled.ID, Title: "Validate requests", Description: "Reject invalid requests with a clear error response."}); err != nil {
		t.Fatal(err)
	}
	// An agent cannot enable the orchestrator nor raise the project's cap.
	for _, args := range []map[string]any{
		{"projectId": disabled.ID},
		{"projectId": disabled.ID, "force": true},
		{"projectId": disabled.ID, "maxConcurrent": 10},
	} {
		if res, err := call(t, database, "orchestrate_project", args); err == nil {
			t.Fatalf("orchestrate_project accepted %v: %+v", args, res)
		}
	}
	if count, err := database.ActiveRunsCountForProject(disabled.ID); err != nil || count != 0 {
		t.Fatalf("refused calls reserved slots: %d, %v", count, err)
	}

	capped, err := database.CreateProject(models.CreateProjectRequest{Name: "Capped MCP orchestration", IssueTracker: "local", OrchestratorEnabled: true, OrchestratorMaxConcurrent: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"Validate requests", "Validate responses"} {
		if _, err := database.CreateTask(models.CreateTaskRequest{ProjectID: capped.ID, Title: title, Description: "Reject invalid payloads with a clear error response."}); err != nil {
			t.Fatal(err)
		}
	}
	_, _ = call(t, database, "orchestrate_project", map[string]any{"projectId": capped.ID, "maxConcurrent": 10})
	if count, err := database.ActiveRunsCountForProject(capped.ID); err != nil || count > 1 {
		t.Fatalf("a caller raised the project cap of 1: %d, %v", count, err)
	}
}
