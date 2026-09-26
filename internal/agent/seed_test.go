package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"tasks/internal/agentconfig"
	"tasks/internal/testhome"
)

// seedServer answers the seed route with seed, or fails with status when set,
// and records the capability reports it receives.
type seedServer struct {
	mu      sync.Mutex
	status  int
	seed    agentconfig.Seed
	fetches int
	reports []agentconfig.CapabilityReport
}

func (s *seedServer) handler(t *testing.T, config agentconfig.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		switch r.URL.Path {
		case "/api/v1/agent/execution-seed":
			s.fetches++
			if s.status != 0 {
				http.Error(w, "unavailable", s.status)
				return
			}
			_ = json.NewEncoder(w).Encode(s.seed)
		case "/api/v1/agent/capabilities":
			var report agentconfig.CapabilityReport
			_ = json.NewDecoder(r.Body).Decode(&report)
			s.reports = append(s.reports, report)
			w.WriteHeader(http.StatusNoContent)
		case "/api/v1/agent/projects":
			_ = json.NewEncoder(w).Encode(agentconfig.Projects{SchemaVersion: 1, Projects: []agentconfig.Project{{ID: "p", Name: "P", GitRemoteURL: config.GitRemoteURL}}})
		default:
			_ = json.NewEncoder(w).Encode(config)
		}
	})
}

func TestSeedOnFirstResolution(t *testing.T) {
	d, config := disconnectFixture(t)
	worktrees := false
	srv := &seedServer{seed: agentconfig.Seed{SchemaVersion: 1,
		Defaults: &agentconfig.SeedDefaults{AIProvider: "claude", AIModel: "opus", EditorCommand: "cursor"},
		Project:  &agentconfig.SeedProject{ProjectID: "p", AIProvider: "codex", AICommandTemplate: "codex {prompt}", UseWorktrees: &worktrees},
	}}
	server := httptest.NewServer(srv.handler(t, config))
	defer server.Close()
	d.link.serverURL = server.URL

	// The fixture's own project statements survive: its project engine and its
	// worktrees are kept, only what the workstation leaves unset is seeded.
	if _, _, err := d.localProjectRoot(context.Background(), config); err != nil {
		t.Fatal(err)
	}
	settings, err := agentconfig.ReadSettings(d.repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !settings.HasSeededDefaults() || !settings.HasSeededProject("p") {
		t.Fatalf("not marked: %+v", settings.Seeded)
	}
	if engine := settings.DefaultEngine(); engine.Provider != "claude" || engine.Model != "opus" || settings.Defaults.EditorCommand != "cursor" {
		t.Fatalf("defaults: %+v %+v", engine, settings.Defaults)
	}
	p, engine := settings.Project("p"), settings.ProjectEngine("p")
	if engine.Provider != "agy" || engine.Command != "custom {prompt}" || p.UseWorktrees == nil || !*p.UseWorktrees {
		t.Fatalf("project section: %+v %+v", p, engine)
	}
	// Seeded once: a changed server value is never taken again.
	srv.mu.Lock()
	srv.seed.Project.AIProvider = "gemini"
	before := srv.fetches
	srv.mu.Unlock()
	if _, _, err := d.localProjectRoot(context.Background(), config); err != nil {
		t.Fatal(err)
	}
	settings, _ = agentconfig.ReadSettings(d.repoRoot)
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if srv.fetches != before || settings.ProjectEngine("p").Provider != "agy" {
		t.Fatalf("seeded twice: fetches %d → %d, provider %q", before, srv.fetches, settings.ProjectEngine("p").Provider)
	}
}

func TestSeedFailureMarksNothing(t *testing.T) {
	d, config := disconnectFixture(t)
	srv := &seedServer{status: http.StatusServiceUnavailable}
	server := httptest.NewServer(srv.handler(t, config))
	defer server.Close()
	d.link.serverURL = server.URL
	d.seedSettings(context.Background(), config)
	settings, _ := agentconfig.ReadSettings(d.repoRoot)
	if settings.HasSeededDefaults() || settings.HasSeededProject("p") {
		t.Fatal("a failed seed must mark nothing")
	}
	// The next resolution retries.
	srv.mu.Lock()
	srv.status = 0
	srv.seed = agentconfig.Seed{SchemaVersion: 1, Defaults: &agentconfig.SeedDefaults{}, Project: &agentconfig.SeedProject{ProjectID: "p"}}
	srv.mu.Unlock()
	d.seedSettings(context.Background(), config)
	settings, _ = agentconfig.ReadSettings(d.repoRoot)
	if !settings.HasSeededDefaults() || !settings.HasSeededProject("p") {
		t.Fatal("the retry did not seed")
	}
}

func TestSeedSkipsADisconnectedProject(t *testing.T) {
	d, config := disconnectFixture(t)
	srv := &seedServer{seed: agentconfig.Seed{SchemaVersion: 1, Defaults: &agentconfig.SeedDefaults{}, Project: &agentconfig.SeedProject{ProjectID: "p", AIProvider: "codex"}}}
	server := httptest.NewServer(srv.handler(t, config))
	defer server.Close()
	d.link.serverURL = server.URL
	if _, err := agentconfig.UpdateSettings(d.repoRoot, func(s *agentconfig.Settings) error {
		s.DisconnectedProjects = map[string]bool{"p": true}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	d.seedSettings(context.Background(), config)
	settings, _ := agentconfig.ReadSettings(d.repoRoot)
	if settings.HasSeededProject("p") || settings.ProjectEngine("p").Provider != "agy" {
		t.Fatal("a disconnected project was seeded")
	}
	if !settings.HasSeededDefaults() {
		t.Fatal("the defaults must still be seeded")
	}
}

func TestCapabilityOf(t *testing.T) {
	defaults := agentconfig.Defaults{AIProviderModels: map[string][]string{"claude": {"opus", "sonnet"}}}
	c := capabilityOf(agentconfig.Config{ProjectID: "p", AIProvider: "claude", AIModel: "opus", AISkillModels: map[string]string{"implement": "sonnet"}}, defaults)
	if c.Provider != "claude" || c.Model != "opus" || c.SkillModels["implement"] != "sonnet" || !c.ModelSlot || !c.Headless || len(c.Models) != 2 {
		t.Fatalf("claude: %+v", c)
	}
	// A template without a {model} slot runs the CLI's own model.
	c = capabilityOf(agentconfig.Config{ProjectID: "p", AIProvider: "claude", AICommandTemplate: "claude --x {prompt}", AIModel: "opus"}, defaults)
	if c.Model != "" || c.ModelSlot || len(c.Models) != 0 || c.Headless {
		t.Fatalf("template without slot: %+v", c)
	}
	// A provider without a headless mode.
	c = capabilityOf(agentconfig.Config{ProjectID: "p", AIProvider: "agy"}, defaults)
	if c.Headless {
		t.Fatalf("agy: %+v", c)
	}
}

func TestCapabilitiesAreReportedAfterADesktopSave(t *testing.T) {
	d, config := disconnectFixture(t)
	srv := &seedServer{status: http.StatusNotFound}
	server := httptest.NewServer(srv.handler(t, config))
	defer server.Close()
	d.link.serverURL = server.URL
	// The project runs its own engine: editing that engine is what the report
	// follows (#510).
	settings, _ := agentconfig.ReadSettings(d.repoRoot)
	catalogue := settings.Engines.Catalogue
	for i := range catalogue {
		if catalogue[i].ID == settings.ProjectEngine("p").ID {
			catalogue[i].Provider, catalogue[i].Model = "claude", "opus"
		}
	}
	body, _ := json.Marshal(map[string]any{"catalogue": catalogue, "default": settings.DefaultEngine().ID})
	w := disconnectRequest(d, http.MethodPut, "/desktop/engines", string(body))
	if w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		srv.mu.Lock()
		n := len(srv.reports)
		var last agentconfig.CapabilityReport
		if n > 0 {
			last = srv.reports[n-1]
		}
		srv.mu.Unlock()
		if n > 0 && len(last.Projects) == 1 && last.Projects[0].Provider == "claude" {
			// The fixture's project command has no {model} slot: no model is announced.
			if last.Projects[0].Model != "" || last.Projects[0].ModelSlot || last.Projects[0].ProjectID != "p" {
				t.Fatalf("report: %+v", last)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("no report after the save: %+v", srv.reports)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestDesktopWorkstationValidatesAndRoundTrips(t *testing.T) {
	testhome.Temp(t)
	d := &agentDaemon{repoRoot: t.TempDir(), loopback: loopbackServer{desktopToken: "private"}}
	put := func(body map[string]any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(http.MethodPut, "/desktop/workstation", bytes.NewReader(raw))
		r.Header.Set("Authorization", "Bearer private")
		w := httptest.NewRecorder()
		d.desktopHandler(w, r)
		return w
	}
	for name, body := range map[string]map[string]any{
		"model":          {"aiModel": "a; rm -rf /"},
		"custom":         {"aiProvider": "custom"},
		"no prompt":      {"aiProvider": "custom", "aiCommandTemplate": "run"},
		"parallelism":    {"parallelism": 11},
		"setup provider": {"setupProviders": []string{"vim"}},
		"provider list":  {"aiProviderModels": map[string][]string{"claude": {"a b"}}},
	} {
		if w := put(body); w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d %s", name, w.Code, w.Body.String())
		}
	}
	if settings, _ := agentconfig.ReadSettings(d.repoRoot); settings.Layout != 0 {
		t.Fatal("a refused save wrote the file")
	}
	// The engine fields moved to the catalogue (#510): an older desktop is told so.
	if w := put(map[string]any{"aiProvider": "claude"}); w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "engine catalogue") {
		t.Fatalf("engine fields: %d %s", w.Code, w.Body.String())
	}
	if w := put(map[string]any{"aiSkillModels": map[string]string{"clarify": ""}, "aiProviderModels": map[string][]string{"claude": {}}, "useWorktrees": false, "setupProviders": []string{}, "editorCommand": " zed "}); w.Code != http.StatusNoContent {
		t.Fatal(w.Code, w.Body.String())
	}
	r := httptest.NewRequest(http.MethodGet, "/desktop/workstation", nil)
	r.Header.Set("Authorization", "Bearer private")
	w := httptest.NewRecorder()
	d.desktopHandler(w, r)
	var view workstationView
	if err := json.Unmarshal(w.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.Defaults.StatesEngine() || view.Defaults.EditorCommand != "zed" || view.Effective.DefaultEngine.Provider != agentconfig.DefaultProvider {
		t.Fatalf("defaults: %+v %+v", view.Defaults, view.Effective)
	}
	if list, ok := view.Effective.AIProviderModels["claude"]; !ok || len(list) != 0 {
		t.Fatalf("an emptied list must stay a choice: %v", view.Effective.AIProviderModels)
	}
	if view.Effective.UseWorktrees || view.Effective.EditorCommand != "zed" || view.Defaults.SetupProviders == nil {
		t.Fatalf("effective: %+v / %+v", view.Effective, view.Defaults)
	}
	if len(view.ProviderModels["codex"]) == 0 {
		t.Fatal("the shipped lists must be offered")
	}
}

func TestDesktopProjectSavesEveryExecutionField(t *testing.T) {
	d, config := disconnectFixture(t)
	srv := &seedServer{status: http.StatusNotFound}
	server := httptest.NewServer(srv.handler(t, config))
	defer server.Close()
	d.link.serverURL = server.URL
	save := func(body map[string]any) *httptest.ResponseRecorder {
		body["projectId"], body["path"] = "p", d.repoRoot
		raw, _ := json.Marshal(body)
		return disconnectRequest(d, http.MethodPost, "/desktop/projects", string(raw))
	}
	if w := save(map[string]any{"skillCommands": map[string]string{"implement": "two words"}}); w.Code != http.StatusBadRequest {
		t.Fatal("an invalid skill command was accepted", w.Code)
	}
	if w := save(map[string]any{"parallelism": 0}); w.Code != http.StatusBadRequest {
		t.Fatal("parallelism 0 was accepted", w.Code)
	}
	if w := save(map[string]any{"aiSkillModels": map[string]string{"implement": "sonnet"}}); w.Code != http.StatusBadRequest {
		t.Fatal("an engine field of #305 was accepted", w.Code)
	}
	if w := save(map[string]any{
		"setupProviders": []string{"codex"}, "skillCommands": map[string]string{"implement": "/build-it"}, "parallelism": 4,
	}); w.Code != http.StatusNoContent {
		t.Fatal(w.Code, w.Body.String())
	}
	p, _ := agentconfig.ReadSettings(d.repoRoot)
	section := p.Project("p")
	if len(section.SetupProviders) != 1 || section.SkillCommands["implement"] != "/build-it" || section.Parallelism != 4 {
		t.Fatalf("section: %+v", section)
	}
	if w := save(map[string]any{"inheritSetupProviders": true, "inheritSkillCommands": true, "inheritParallelism": true}); w.Code != http.StatusNoContent {
		t.Fatal(w.Code, w.Body.String())
	}
	p, _ = agentconfig.ReadSettings(d.repoRoot)
	section = p.Project("p")
	if section.SetupProviders != nil || section.SkillCommands != nil || section.Parallelism != 0 {
		t.Fatalf("inherit left values: %+v", section)
	}
}
