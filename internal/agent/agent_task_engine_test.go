package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
	"tasks/internal/testhome"
)

// engineSettings is a workstation whose project p runs Opus by default, with
// task t-codex switched to Codex (#510).
func engineSettings() agentconfig.Settings {
	return agentconfig.Settings{Engines: agentconfig.Engines{
		Catalogue: []agentconfig.Engine{
			{ID: "e-opus", Name: "Claude Opus", Provider: "claude", Model: "claude-opus-5", SkillModels: map[string]string{"implement": "claude-sonnet-5"}},
			{ID: "e-codex", Name: "Codex", Provider: "codex", Model: "gpt-5"},
		},
		Default: "e-opus",
		Tasks:   map[string]string{"t-codex": "e-codex"},
	}}
}

func TestOneOffModelAppliesOnlyOnTheProjectDefaultEngine(t *testing.T) {
	base := agentconfig.Config{ProjectID: "p", Skills: launchConfig().Skills}
	settings := engineSettings()
	onDefault := agentconfig.ResolveTask(base, settings, "t-plain")
	if got, _ := LaunchModel(onDefault, "implement", "claude-haiku-4-5"); got != "claude-haiku-4-5" {
		t.Fatalf("on the project default engine the one-off model outranks the engine: %q", got)
	}
	switched := agentconfig.ResolveTask(base, settings, "t-codex")
	if got, _ := LaunchModel(switched, "implement", "claude-haiku-4-5"); got != "gpt-5" {
		t.Fatalf("off the project default engine the one-off model is ignored: %q", got)
	}
	// The command line and the run record agree, whatever the mode.
	for _, mode := range []string{models.SkillModeInteractive, models.SkillModeAutonomous} {
		line, err := dispatchCommand(switched, "#203", "implement", "execute_skill", "", "", mode, "claude-haiku-4-5")
		if err != nil {
			t.Fatal(err)
		}
		provider, model := launchEngine(switched, "implement", "claude-haiku-4-5", mode)
		if provider != "codex" || model != "gpt-5" || !strings.Contains(line, "gpt-5") || strings.Contains(line, "claude") {
			t.Fatalf("%s: line %q, record %s/%s", mode, line, provider, model)
		}
	}
}

func TestDiscussionOpensTheTaskEngine(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"claude", "codex"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	switched := agentconfig.ResolveTask(agentconfig.Config{ProjectID: "p"}, engineSettings(), "t-codex")
	line, err := dispatchCommand(switched, "#203", "discuss", "discuss", "", "", models.SkillModeInteractive, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(line, "codex") || strings.Contains(line, "claude") {
		t.Fatalf("a discussion must open the task engine: %q", line)
	}
}

// Two tasks of one project are dispatched with their own engines, and the
// switched task finds its provider's skills installed before its CLI starts.
func TestDispatchRunsTheTaskEngineAndSetsItUp(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	testhome.Temp(t)
	for _, args := range [][]string{{"init"}, {"-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "Initial"}} {
		if _, err := gitLocal(ctx, root, args...); err != nil {
			t.Fatal(err)
		}
	}
	settings := agentconfig.Settings{Engines: agentconfig.Engines{
		Catalogue: []agentconfig.Engine{{ID: "e-agy", Name: "Antigravity", Provider: "agy"}, {ID: "e-codex", Name: "Codex", Provider: "codex", Model: "gpt-5"}},
		Default:   "e-agy", Tasks: map[string]string{"task-switched": "e-codex"},
	}}
	settings.SetProject("p", agentconfig.ProjectSettings{Path: root})
	if err := agentconfig.WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	config := agentconfig.Config{SchemaVersion: 1, ProjectID: "p", UseWorktrees: true, AIProvider: "claude",
		Skills: []agentconfig.Skill{{ID: "implement", Directory: "code-issue", Command: "/code-issue", Content: "remote instructions"}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/agent/config":
			_ = json.NewEncoder(w).Encode(config)
		case "/api/tasks/SW-1":
			_ = json.NewEncoder(w).Encode(models.Task{ID: "task-switched", Key: "SW-1", ProjectID: "p"})
		case "/api/tasks/PL-2":
			_ = json.NewEncoder(w).Encode(models.Task{ID: "task-plain", Key: "PL-2", ProjectID: "p"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	d := &agentDaemon{repoRoot: root, loopback: loopbackServer{url: "http://127.0.0.1:8091"}, link: serverLink{serverURL: srv.URL, token: "token", projectID: "p"}}
	switched, _, _, _, err := d.prepareDispatch(ctx, "SW-1")
	if err != nil {
		t.Fatal(err)
	}
	plain, _, _, _, err := d.prepareDispatch(ctx, "PL-2")
	if err != nil {
		t.Fatal(err)
	}
	if switched.AIProvider != "codex" || switched.AIModel != "gpt-5" || switched.EngineID != "e-codex" {
		t.Fatalf("switched task: %+v", switched)
	}
	if plain.AIProvider != "agy" || plain.EngineID != "e-agy" {
		t.Fatalf("plain task: %+v", plain)
	}
	// Every catalogue provider that takes skills has them, whichever task ran.
	for _, path := range []string{".agents/skills/code-issue/SKILL.md", ".gemini/config/skills/code-issue/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), path)); err != nil {
			t.Fatalf("%s not installed: %v", path, err)
		}
	}
}

func TestHeadlessRefusalNamesTheEngine(t *testing.T) {
	d, config := disconnectFixture(t)
	// The fixture's project engine is agy with a template that has no mode
	// marker: it cannot run headless. A task switched to Claude can.
	if _, err := agentconfig.UpdateSettings(d.repoRoot, func(s *agentconfig.Settings) error {
		list := append(s.Engines.Catalogue, agentconfig.Engine{Name: "Claude", Provider: "claude"})
		if err := s.ReplaceCatalogue(list, s.Engines.Default); err != nil {
			return err
		}
		s.SetTaskEngine("t-claude", s.Engines.Catalogue[len(s.Engines.Catalogue)-1].ID)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	payload := agentconfig.Dispatch{RunID: "run-1", SkillID: "implement", Mode: models.SkillModeAutonomous}
	settings, _ := agentconfig.ReadSettings(d.repoRoot)
	_, err := d.admitProjectRun(context.Background(), "t-plain", payload, config)
	if err == nil || !strings.Contains(err.Error(), `engine "`+settings.ProjectEngine("p").Name+`"`) {
		t.Fatalf("the refusal must name the engine: %v", err)
	}
	payload.RunID = "run-2"
	if _, err := d.admitProjectRun(context.Background(), "t-claude", payload, config); err != nil {
		t.Fatalf("a task switched to a headless engine is admitted: %v", err)
	}
}

func TestDesktopEnginesEndpoints(t *testing.T) {
	d, _ := disconnectFixture(t)
	get := func(target string, out any) {
		t.Helper()
		w := disconnectRequest(d, http.MethodGet, target, "")
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s: %d %s", target, w.Code, w.Body.String())
		}
		if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
			t.Fatal(err)
		}
	}
	put := func(target string, body any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		return disconnectRequest(d, http.MethodPut, target, string(raw))
	}
	var view enginesView
	get("/desktop/engines", &view)
	// The fixture's project section stated a template: its engine joined the
	// implicit default one.
	if len(view.Catalogue) != 2 || view.Default == "" || view.Projects["p"] == "" || len(view.ProviderModels["claude"]) == 0 {
		t.Fatalf("catalogue: %+v", view)
	}
	projectEngine := view.Projects["p"]

	// Refusals leave the catalogue as it was.
	for name, tc := range map[string]struct {
		body any
		code int
	}{
		"duplicate name": {map[string]any{"catalogue": append(view.Catalogue, agentconfig.Engine{Name: strings.ToUpper(view.Catalogue[0].Name), Provider: "codex"}), "default": view.Default}, 400},
		"custom":         {map[string]any{"catalogue": append(view.Catalogue, agentconfig.Engine{Name: "Wrap", Provider: "custom"}), "default": view.Default}, 400},
		"bad model":      {map[string]any{"catalogue": append(view.Catalogue, agentconfig.Engine{Name: "Opus", Provider: "claude", Model: "a b"}), "default": view.Default}, 400},
		"drop default":   {map[string]any{"catalogue": view.Catalogue[1:], "default": view.Default}, 409},
		"empty":          {map[string]any{"catalogue": []agentconfig.Engine{}, "default": ""}, 400},
	} {
		if w := put("/desktop/engines", tc.body); w.Code != tc.code {
			t.Errorf("%s: %d %s", name, w.Code, w.Body.String())
		}
	}
	var unchanged enginesView
	get("/desktop/engines", &unchanged)
	if len(unchanged.Catalogue) != 2 {
		t.Fatalf("a refused save changed the catalogue: %+v", unchanged)
	}

	// Add an engine and switch a task to it.
	w := put("/desktop/engines", map[string]any{"catalogue": append(view.Catalogue, agentconfig.Engine{Name: "Codex", Provider: "codex"}), "default": view.Default})
	if w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	json.Unmarshal(w.Body.Bytes(), &view)
	codex := view.Catalogue[2].ID
	if w := put("/desktop/task-engines", map[string]any{"projectId": "p", "taskId": "t-1", "engineId": codex}); w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := put("/desktop/task-engines", map[string]any{"projectId": "p", "taskId": "t-1", "engineId": "e-gone"}); w.Code != http.StatusNotFound {
		t.Fatalf("unknown engine: %d", w.Code)
	}
	var tasks taskEnginesView
	get("/desktop/task-engines?projectId=p", &tasks)
	if tasks.ProjectDefault != projectEngine || tasks.Tasks["t-1"] != codex || len(tasks.Catalogue) != 3 || tasks.Catalogue[2].Name != "Codex" {
		t.Fatalf("task engines: %+v", tasks)
	}
	get("/desktop/engines", &view)
	if view.TaskCounts[codex] != 1 {
		t.Fatalf("task count: %+v", view.TaskCounts)
	}
	// Back on its project default engine, the task follows it again.
	if w := put("/desktop/task-engines", map[string]any{"projectId": "p", "taskId": "t-2", "engineId": projectEngine}); w.Code != http.StatusOK {
		t.Fatal(w.Code)
	}
	if settings, _ := agentconfig.ReadSettings(d.repoRoot); settings.Engines.Tasks["t-2"] != "" {
		t.Fatalf("a task on its project default engine stores no choice: %+v", settings.Engines.Tasks)
	}
	// Removing the engine drops the task choice.
	if w := put("/desktop/engines", map[string]any{"catalogue": view.Catalogue[:2], "default": view.Default}); w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	var pruned taskEnginesView
	get("/desktop/task-engines?projectId=p", &pruned)
	if len(pruned.Tasks) != 0 {
		t.Fatalf("a removed engine's task choice survived: %+v", pruned.Tasks)
	}
	if w := disconnectRequest(d, http.MethodGet, "/desktop/task-engines", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("project required: %d", w.Code)
	}
}

func TestStatusAnnouncesTaskEngines(t *testing.T) {
	d, _ := disconnectFixture(t)
	var status struct {
		Capabilities []string `json:"capabilities"`
	}
	w := disconnectRequest(d, http.MethodGet, "/desktop/status", "")
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatal(err, w.Body.String())
	}
	found := false
	for _, capability := range status.Capabilities {
		found = found || capability == taskEnginesCapability
	}
	if !found {
		t.Fatalf("capabilities: %v", status.Capabilities)
	}
}

// The report describes the project default engine: a task switch leaves it
// unchanged, the run record says what ran.
func TestCapabilityReportIgnoresTaskSwitches(t *testing.T) {
	d, config := disconnectFixture(t)
	srv := &seedServer{status: http.StatusNotFound}
	server := httptest.NewServer(srv.handler(t, config))
	defer server.Close()
	d.link.serverURL = server.URL
	report := func() agentconfig.Capability {
		t.Helper()
		if err := d.reportCapabilities(context.Background()); err != nil {
			t.Fatal(err)
		}
		srv.mu.Lock()
		defer srv.mu.Unlock()
		last := srv.reports[len(srv.reports)-1]
		if len(last.Projects) != 1 {
			t.Fatalf("report: %+v", last)
		}
		return last.Projects[0]
	}
	before := report()
	if _, err := agentconfig.UpdateSettings(d.repoRoot, func(s *agentconfig.Settings) error {
		if err := s.ReplaceCatalogue(append(s.Engines.Catalogue, agentconfig.Engine{Name: "Claude", Provider: "claude", Model: "opus"}), s.Engines.Default); err != nil {
			return err
		}
		s.SetTaskEngine("t-1", s.Engines.Catalogue[len(s.Engines.Catalogue)-1].ID)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if after := report(); after.Provider != before.Provider || after.Model != before.Model || after.Provider != "agy" {
		t.Fatalf("a task switch changed the report: %+v → %+v", before, after)
	}
}
