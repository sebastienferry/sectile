package main

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tasks/internal/agentconfig"
	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/models"
)

func headlessConfig(provider string, skills ...agentconfig.Skill) agentconfig.Config {
	if len(skills) == 0 {
		skills = []agentconfig.Skill{{ID: "implement", Directory: "code-issue", Command: "/code-issue"}}
	}
	return agentconfig.Config{AIProvider: provider, Skills: skills}
}

func TestHeadlessCommandLinePerProvider(t *testing.T) {
	cases := map[string]string{
		"claude": "claude -p ",
		"codex":  "codex exec ",
		"vibe":   "vibe -p ",
	}
	for provider, prefix := range cases {
		line, err := headlessCommandLine(provider, "do the thing")
		if err != nil {
			t.Fatalf("%s: %v", provider, err)
		}
		if !strings.HasPrefix(line, prefix) {
			t.Fatalf("%s: expected prefix %q, got %q", provider, prefix, line)
		}
		if !strings.Contains(line, "'do the thing'") {
			t.Fatalf("%s: prompt not quoted into the line: %q", provider, line)
		}
	}
}

func TestHeadlessCommandLineRefusesUnsupportedProvider(t *testing.T) {
	for _, provider := range []string{"agy", "gemini", "cursor", "whatever"} {
		_, err := headlessCommandLine(provider, "prompt")
		if err == nil {
			t.Fatalf("provider %q should be refused", provider)
		}
		if !strings.Contains(err.Error(), provider) {
			t.Fatalf("refusal should name the provider, got %q", err)
		}
	}
}

func TestDispatchCommandBranchesOnMode(t *testing.T) {
	config := headlessConfig("claude")
	launch := agentCommandContext{Task: models.Task{Key: "#123"}}

	interactive, err := dispatchCommand(config, "#123", "implement", "implement", "", "", launch)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(interactive, "claude -p ") {
		t.Fatalf("interactive launch should not use the print form: %q", interactive)
	}

	launch.Mode = models.SkillModeNonInteractive
	headless, err := dispatchCommand(config, "#123", "implement", "implement", "", "", launch)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(headless, "claude -p ") {
		t.Fatalf("non-interactive launch should use the print form: %q", headless)
	}
	if !strings.Contains(headless, "/code-issue #123") {
		t.Fatalf("headless line lost the skill command: %q", headless)
	}
}

func TestDispatchCommandRefusesHeadlessOnUnsupportedProvider(t *testing.T) {
	config := headlessConfig("cursor")
	launch := agentCommandContext{Task: models.Task{Key: "#123"}, Mode: models.SkillModeNonInteractive}
	_, err := dispatchCommand(config, "#123", "implement", "implement", "", "", launch)
	if err == nil || !strings.Contains(err.Error(), "cursor") {
		t.Fatalf("expected a refusal naming cursor, got %v", err)
	}
}

func TestDispatchCommandRefusesHeadlessTemplateWithoutModePlaceholder(t *testing.T) {
	config := headlessConfig("claude")
	config.AICommandTemplate = `mycli "{prompt}"`
	launch := agentCommandContext{Task: models.Task{Key: "#123"}, Mode: models.SkillModeNonInteractive}
	if _, err := dispatchCommand(config, "#123", "implement", "implement", "", "", launch); err == nil {
		t.Fatal("a template without {mode} should refuse a non-interactive launch")
	}

	config.AICommandTemplate = `mycli {mode} "{prompt}"`
	line, err := dispatchCommand(config, "#123", "implement", "implement", "", "", launch)
	if err != nil {
		t.Fatalf("a template carrying {mode} owns the mode: %v", err)
	}
	if !strings.Contains(line, models.SkillModeNonInteractive) {
		t.Fatalf("the template should receive the mode: %q", line)
	}
}

func TestTailBufferKeepsTheTail(t *testing.T) {
	buffer := &tailBuffer{cap: 8}
	if _, err := buffer.Write([]byte("0123456789abc")); err != nil {
		t.Fatal(err)
	}
	if got := buffer.String(); got != "6789abc" && len(got) > 8 {
		t.Fatalf("output is not bounded to the tail: %q", got)
	}
	if len(buffer.String()) > 8 {
		t.Fatalf("output exceeds the cap: %q", buffer.String())
	}
}

// headlessDaemon wires a real MCP-backed server so a headless run closes itself
// exactly as it does in production.
func headlessDaemon(t *testing.T) (*agentDaemon, *db.DB, *models.Task) {
	t.Helper()
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Headless run"})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SECTILE_SERVER_TOKEN", "headless-secret")
	server := httptest.NewServer(handlers.NewHandler(database).MCPHandler())
	t.Cleanup(server.Close)
	return &agentDaemon{serverURL: server.URL, token: "headless-secret"}, database, task
}

func awaitRunStatus(t *testing.T, database *db.DB, runID string) string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		activity, err := database.GetActivityByID(runID)
		if err == nil && activity != nil && activity.Status != "running" {
			return activity.Status
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("run %s never left the running state", runID)
	return ""
}

func TestHeadlessRunFinishesWithoutATerminalSession(t *testing.T) {
	daemon, database, task := headlessDaemon(t)
	run, err := database.StartRemoteRun(task.ID, "implement", "")
	if err != nil {
		t.Fatal(err)
	}
	payload := agentconfig.Dispatch{RunID: run.ID, TaskID: task.ID, TaskKey: task.Key, SkillID: "implement", Mode: models.SkillModeNonInteractive}
	if _, err := daemon.enqueueRun(task.ID, payload, "default", t.TempDir(), 1, false); err != nil {
		t.Fatal(err)
	}
	if err := daemon.runHeadless(task.ID, payload, t.TempDir(), "feat/x", "echo done-headless"); err != nil {
		t.Fatal(err)
	}
	if status := awaitRunStatus(t, database, run.ID); status != "completed" {
		t.Fatalf("a successful headless run should complete, got %q", status)
	}
	// Nothing was injected into a terminal session: the daemon has no manager here,
	// so reaching one would have panicked rather than silently opening a window.
	activity, err := database.GetActivityByID(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(activity.Summary+activity.Output, "done-headless") {
		t.Fatalf("captured output not reported on the run: %q / %q", activity.Summary, activity.Output)
	}
}

func TestHeadlessFailureDoesNotTransition(t *testing.T) {
	daemon, database, task := headlessDaemon(t)
	before := database.StageOfTask(task)
	run, err := database.StartRemoteRun(task.ID, "implement", "")
	if err != nil {
		t.Fatal(err)
	}
	payload := agentconfig.Dispatch{RunID: run.ID, TaskID: task.ID, TaskKey: task.Key, SkillID: "implement", Mode: models.SkillModeNonInteractive}
	if _, err := daemon.enqueueRun(task.ID, payload, "default", t.TempDir(), 1, false); err != nil {
		t.Fatal(err)
	}
	if err := daemon.runHeadless(task.ID, payload, t.TempDir(), "feat/x", "exit 3"); err != nil {
		t.Fatal(err)
	}
	if status := awaitRunStatus(t, database, run.ID); status != "failed" {
		t.Fatalf("a failing headless run should fail, got %q", status)
	}
	updated, err := database.GetTaskByID(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after := database.StageOfTask(updated); after != before {
		t.Fatalf("a failed headless run moved the stage from %q to %q", before, after)
	}
}
