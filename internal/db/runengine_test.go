package db

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"tasks/internal/agentconfig"
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
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Engine"})
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
			// End the run, so the next launch does not find the task busy.
			if _, err := database.FinishRemoteRun(task.ID, op.RunID, "completed", "done"); err != nil {
				t.Fatal(err)
			}
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

// The engine a run shows before the agent's own report comes from what the
// workstation reported (#305): unknown without a report, never a server value.
func TestResolveTaskEngineReadsTheReport(t *testing.T) {
	database := engineDB(t)
	setLegacySettings(t, database, map[string]any{"ai_provider": "gemini", "ai_model": "global-model"})
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Resolve"})
	if err != nil {
		t.Fatal(err)
	}
	if provider, model := database.ResolveTaskEngine(project.ID, "ada", "laptop", "implement", "chosen"); provider != "" || model != "" {
		t.Fatalf("without a report the engine is unknown, got %q/%q", provider, model)
	}
	report := agentconfig.CapabilityReport{SchemaVersion: 1, DeviceID: "laptop", Projects: []agentconfig.Capability{{
		ProjectID: project.ID, Provider: "claude", Model: "project-model", SkillModels: map[string]string{"clarify": "clarify-model"}, ModelSlot: true,
	}}}
	if err := database.SaveCapabilities("ada", report); err != nil {
		t.Fatal(err)
	}
	if provider, model := database.ResolveTaskEngine(project.ID, "ada", "laptop", "implement", ""); provider != "claude" || model != "project-model" {
		t.Fatalf("report ignored: %q/%q", provider, model)
	}
	if _, model := database.ResolveTaskEngine(project.ID, "ada", "laptop", "clarify", ""); model != "clarify-model" {
		t.Fatalf("per-skill model ignored: %q", model)
	}
	// The launch outranks the configured model when the command line carries one.
	if _, model := database.ResolveTaskEngine(project.ID, "ada", "laptop", "clarify", "chosen"); model != "chosen" {
		t.Fatalf("launch choice lost: %q", model)
	}
	// An unknown device falls back to the person's latest report.
	if provider, _ := database.ResolveTaskEngine(project.ID, "ada", "other", "implement", ""); provider != "claude" {
		t.Fatalf("fallback to the latest report: %q", provider)
	}
	// Another person's report is never theirs.
	if provider, _ := database.ResolveTaskEngine(project.ID, "grace", "", "implement", ""); provider != "" {
		t.Fatalf("someone else's report was served: %q", provider)
	}
	// Without a model slot, neither the configured model nor a choice is claimed.
	report.Projects[0].ModelSlot, report.Projects[0].Model, report.Projects[0].SkillModels = false, "", nil
	if err := database.SaveCapabilities("ada", report); err != nil {
		t.Fatal(err)
	}
	if _, model := database.ResolveTaskEngine(project.ID, "ada", "laptop", "clarify", "chosen"); model != "" {
		t.Fatalf("a command line without a model slot claims %q", model)
	}
}

// Each workstation keeps its own report, replaced by its next one.
func TestEngineReportPerDevice(t *testing.T) {
	database := engineDB(t)
	save := func(device, provider string) {
		t.Helper()
		if err := database.SaveCapabilities("ada", agentconfig.CapabilityReport{SchemaVersion: 1, DeviceID: device, Projects: []agentconfig.Capability{
			{ProjectID: "p", Provider: provider, Models: []string{"a", "b"}, Headless: true},
			{ProjectID: " ", Provider: "ignored"},
		}}); err != nil {
			t.Fatal(err)
		}
	}
	save("laptop", "claude")
	save("desktop", "codex")
	save("laptop", "gemini")
	if report, ok := database.EngineReport("ada", "p", "laptop"); !ok || report.State != models.EngineReported || report.Provider != "gemini" ||
		len(report.Models) != 2 || !report.Headless || report.SkillModels == nil {
		t.Fatalf("laptop: %+v %v", report, ok)
	}
	if report, _ := database.EngineReport("ada", "p", "desktop"); report.Provider != "codex" {
		t.Fatalf("desktop: %+v", report)
	}
	if report, ok := database.EngineReport("ada", "q", ""); ok || report.State != models.EngineUnknown {
		t.Fatalf("an unreported project: %+v", report)
	}
}
