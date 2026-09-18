package db

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"tasks/internal/agentprotocol"
	"tasks/internal/models"
)

func engineDB(t *testing.T) *DB {
	t.Helper()
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func engineTask(t *testing.T, database *DB) *models.Task {
	t.Helper()
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Engine", AIProvider: "claude", AIModel: "claude-sonnet-5"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := database.CreateTask(models.CreateTaskRequest{Title: "Pick a model", ProjectID: project.ID})
	if err != nil {
		t.Fatal(err)
	}
	return task
}

// The launcher writes the engine it resolved, and the record has to read it
// back: this is the only trace of what a finished run ran against.
func TestRemoteRunRecordsItsEngine(t *testing.T) {
	database := engineDB(t)
	task := engineTask(t, database)

	run, err := database.StartAgentRun(task.ID, "implement", RunLaunch{Mode: models.SkillModeAutonomous, Provider: "claude", Model: "claude-opus-5"})
	if err != nil {
		t.Fatal(err)
	}
	if run.Provider != "claude" || run.Model != "claude-opus-5" {
		t.Fatalf("engine not carried by the created run: %q/%q", run.Provider, run.Model)
	}
	stored, err := database.GetActivityByID(run.ID)
	if err != nil || stored == nil {
		t.Fatalf("run not found: %v", err)
	}
	if stored.Provider != "claude" || stored.Model != "claude-opus-5" {
		t.Fatalf("engine not persisted: %q/%q", stored.Provider, stored.Model)
	}

	// The list view reads its own column list, which is exactly where a column
	// added in one place and forgotten in another shows up.
	activities, err := database.GetTaskActivities(task.ID)
	if err != nil || len(activities) == 0 {
		t.Fatalf("activities not listed: %v", err)
	}
	if activities[0].Model != "claude-opus-5" {
		t.Fatalf("engine missing from the task activity list: %q", activities[0].Model)
	}
}

// A run recorded before the engine was tracked reads as unknown, never as some
// particular model.
func TestRemoteRunWithoutEngineReadsEmpty(t *testing.T) {
	database := engineDB(t)
	task := engineTask(t, database)
	run, err := database.StartRemoteRun(task.ID, "clarify", "")
	if err != nil {
		t.Fatal(err)
	}
	stored, err := database.GetActivityByID(run.ID)
	if err != nil || stored == nil {
		t.Fatalf("run not found: %v", err)
	}
	if stored.Provider != "" || stored.Model != "" {
		t.Fatalf("an unreported engine must stay empty: %q/%q", stored.Provider, stored.Model)
	}
}

// The agent corrects the launcher's resolution once it has built the real
// command line, which is where the workstation override finally shows.
func TestSetRemoteRunEngineCorrectsTheRecord(t *testing.T) {
	database := engineDB(t)
	task := engineTask(t, database)
	run, err := database.StartAgentRun(task.ID, "implement", RunLaunch{Provider: "claude", Model: "claude-sonnet-5"})
	if err != nil {
		t.Fatal(err)
	}
	if err = database.SetRemoteRunEngine(run.ID, "claude", "workstation-model"); err != nil {
		t.Fatal(err)
	}
	stored, err := database.GetActivityByID(run.ID)
	if err != nil || stored == nil {
		t.Fatalf("run not found: %v", err)
	}
	if stored.Model != "workstation-model" {
		t.Fatalf("agent report ignored: %q", stored.Model)
	}

	// A late report must not rewrite a finished record, and an unknown run is
	// refused rather than silently ignored.
	if _, err = database.FinishRemoteRun(task.ID, run.ID, "completed", "done"); err != nil {
		t.Fatal(err)
	}
	if err = database.SetRemoteRunEngine(run.ID, "claude", "too-late"); err == nil {
		t.Fatal("a finished run must refuse an engine report")
	}
	if err = database.SetRemoteRunEngine("missing", "claude", "x"); err == nil {
		t.Fatal("an unknown run must refuse an engine report")
	}
	stored, _ = database.GetActivityByID(run.ID)
	if stored.Model != "workstation-model" {
		t.Fatalf("a refused report must leave the record alone: %q", stored.Model)
	}
}

// The queued path must carry the choice all the way to the agent: the launch,
// the job it files and the operation the worker sends are three hops, and a
// value dropped at any of them is invisible until a run uses the wrong model.
func TestQueuedLaunchCarriesTheModel(t *testing.T) {
	database := engineDB(t)
	task := engineTask(t, database)

	sent := make(chan agentprotocol.Operation, 4)
	database.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		sent <- op
		return json.RawMessage(`null`), nil
	})

	launch := func(mode, model string) agentprotocol.Operation {
		t.Helper()
		if _, _, err := database.EnqueueSkillOnTaskWithOverrides(task.ID, "clarify", "", mode, model); err != nil {
			t.Fatal(err)
		}
		select {
		case op := <-sent:
			return op
		case <-time.After(5 * time.Second):
			t.Fatal("the worker sent no operation")
			return agentprotocol.Operation{}
		}
	}

	op := launch(models.SkillModeAutonomous, "claude-opus-5")
	if op.Model != "claude-opus-5" {
		t.Fatalf("the queued launch lost the model: %q", op.Model)
	}
	if op.Mode != models.SkillModeAutonomous {
		t.Fatalf("the queued launch lost the mode: %q", op.Mode)
	}

	// A launch with no choice sends no override, which is what keeps the
	// configured levels in charge on this path too.
	if op = launch("", "  "); op.Model != "" {
		t.Fatalf("an untouched launch must send no model: %q", op.Model)
	}

	waitForIdleConnections(t, database)
}

// What the server can resolve on its own: the project over the global settings,
// with the launch choice on top. It is what a run shows until the agent reports.
func TestResolveTaskEngine(t *testing.T) {
	database := engineDB(t)
	// The legacy global template is cleared on purpose: it carries {prompt}, so
	// it would govern the command line and no model would reach it, which is
	// exactly what the last case below checks.
	if _, err := database.UpdateSettings(models.Settings{
		AIProvider: "gemini", AIModel: "global-model", AISkillModels: map[string]string{"clarify": "global-clarify"},
	}, "aiCommandTemplate"); err != nil {
		t.Fatal(err)
	}
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Resolve", AIProvider: "claude", AIModel: "project-model"})
	if err != nil {
		t.Fatal(err)
	}

	provider, model := database.ResolveTaskEngine(project.ID, "implement", "")
	if provider != "claude" || model != "project-model" {
		t.Fatalf("project level lost: %q/%q", provider, model)
	}
	// The most specific configured statement still wins between levels.
	if _, model = database.ResolveTaskEngine(project.ID, "clarify", ""); model != "global-clarify" {
		t.Fatalf("a global per-skill entry must survive a bare project model: %q", model)
	}
	// The launch outranks every configured level.
	if _, model = database.ResolveTaskEngine(project.ID, "clarify", "chosen-model"); model != "chosen-model" {
		t.Fatalf("launch choice lost: %q", model)
	}
	// A project with no provider of its own falls back to the global one.
	plain, err := database.CreateProject(models.CreateProjectRequest{Name: "Plain"})
	if err != nil {
		t.Fatal(err)
	}
	if provider, _ = database.ResolveTaskEngine(plain.ID, "implement", ""); provider != "gemini" {
		t.Fatalf("global provider lost: %q", provider)
	}

	// A template that governs the command line without a {model} slot carries no
	// model, so the run must not claim one it never ran against.
	templated, err := database.CreateProject(models.CreateProjectRequest{
		Name: "Templated", AIProvider: "claude", AIModel: "project-model", AICommandTemplate: `claude -p "{prompt}"`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, model = database.ResolveTaskEngine(templated.ID, "implement", "chosen-model"); model != "" {
		t.Fatalf("a template with no {model} slot must report no model: %q", model)
	}

	// A provider that takes no model reports none either, configured or chosen.
	flagless, err := database.CreateProject(models.CreateProjectRequest{Name: "Flagless", AIProvider: "agy", AIModel: "project-model"})
	if err != nil {
		t.Fatal(err)
	}
	if provider, model = database.ResolveTaskEngine(flagless.ID, "implement", "chosen-model"); provider != "agy" || model != "" {
		t.Fatalf("a flagless provider must report no model: %q/%q", provider, model)
	}
}

// The per-provider list is settings like any other: it survives a round trip and
// is normalised on the way in.
func TestSettingsKeepTheProviderModelLists(t *testing.T) {
	database := engineDB(t)
	saved, err := database.UpdateSettings(models.Settings{
		AIProvider:       "claude",
		AIProviderModels: map[string][]string{"claude": {"claude-opus-5", " ", "claude-opus-5", "claude-haiku-4-5"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := saved.AIProviderModels["claude"]; len(got) != 2 {
		t.Fatalf("list not normalised on write: %v", got)
	}
	read, err := database.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	got := read.AIProviderModels["claude"]
	if len(got) != 2 || got[0] != "claude-opus-5" || got[1] != "claude-haiku-4-5" {
		t.Fatalf("list not persisted in order: %v", got)
	}
}
