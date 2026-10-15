package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"tasks/internal/agentconfig"
	"tasks/internal/testhome"
	"testing"
)

func setupFixture(t *testing.T) *agentDaemon {
	t.Helper()
	testhome.Temp(t)
	return &agentDaemon{repoRoot: t.TempDir(), loopback: loopbackServer{url: "http://127.0.0.1:4567", desktopToken: "private"}, link: serverLink{serverURL: "https://sectile.example.test", token: "secret-pairing-key"}}
}
func readSetupTest(t *testing.T, d *agentDaemon, provider string) setupView {
	t.Helper()
	rec := disconnectRequest(d, "GET", "/desktop/setup?provider="+provider, "")
	if rec.Code != 200 {
		t.Fatalf("read: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "secret-pairing-key") {
		t.Fatal("pairing secret exposed")
	}
	var view setupView
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	return view
}
func acknowledgeSetupTest(t *testing.T, d *agentDaemon, provider, step, decision string) {
	t.Helper()
	view := readSetupTest(t, d, provider)
	raw, _ := json.Marshal(map[string]string{"step": step, "decision": decision, "fingerprint": view.Steps[step].Fingerprint})
	rec := disconnectRequest(d, "POST", "/desktop/setup?provider="+provider, string(raw))
	if rec.Code != 200 {
		t.Fatalf("ack: %d %s", rec.Code, rec.Body.String())
	}
}
func TestSetupDecisionsPersistAndPermissionChangesInvalidateOnlyReview(t *testing.T) {
	d := setupFixture(t)
	acknowledgeSetupTest(t, d, "codex", "skills", "skipped")
	acknowledgeSetupTest(t, d, "codex", "sdd", "skipped")
	acknowledgeSetupTest(t, d, "codex", "permissions", "retained")
	before := readSetupTest(t, d, "codex")
	if before.Steps["skills"].Status != "skipped" || before.Steps["permissions"].Status != "success" {
		t.Fatalf("decisions not retained: %+v", before)
	}
	restarted := &agentDaemon{repoRoot: d.repoRoot, loopback: d.loopback, link: serverLink{serverURL: d.link.serverURL, token: d.link.token}}
	if view := readSetupTest(t, restarted, "codex"); view.Steps["sdd"].Status != "skipped" {
		t.Fatal("restart lost skip")
	}
	if view := readSetupTest(t, d, "claude"); view.Steps["skills"].Status != "pending" {
		t.Fatal("provider decisions leaked")
	}
	_, err := agentconfig.UpdateSettings(d.localSettingsRoot(), func(s *agentconfig.Settings) error { s.Defaults.CodexApprovalsReviewer = "auto_review"; return nil })
	if err != nil {
		t.Fatal(err)
	}
	after := readSetupTest(t, d, "codex")
	if after.Steps["permissions"].Status != "pending" || after.Steps["skills"].Status != "skipped" {
		t.Fatalf("wrong invalidation: %+v", after)
	}
	raw, _ := json.Marshal(map[string]string{"step": "permissions", "decision": "retained", "fingerprint": before.Steps["permissions"].Fingerprint})
	if rec := disconnectRequest(d, "POST", "/desktop/setup?provider=codex", string(raw)); rec.Code != 409 {
		t.Fatalf("stale review accepted: %d", rec.Code)
	}
}
func TestSetupMCPReadsActualEntryAndPairingWithoutExposingCredentials(t *testing.T) {
	for _, provider := range []string{"claude", "codex", "agy"} {
		t.Run(provider, func(t *testing.T) {
			d := setupFixture(t)
			executable, _ := os.Executable()
			acknowledgeSetupTest(t, d, provider, "skills", "skipped")
			acknowledgeSetupTest(t, d, provider, "sdd", "skipped")
			acknowledgeSetupTest(t, d, provider, "permissions", "retained")
			if readSetupTest(t, d, provider).Complete {
				t.Fatal("missing MCP completed")
			}
			if _, err := agentconfig.ConfigureMCP(provider, executable, d.link.serverURL, d.link.token, "http", false); err != nil {
				t.Fatal(err)
			}
			if view := readSetupTest(t, d, provider); !view.Complete {
				t.Fatalf("valid config not accepted: %+v", view)
			}
			d.link.token = "rotated-key"
			if view := readSetupTest(t, d, provider); view.Steps["mcp"].Status != "pending" || view.Steps["skills"].Status != "skipped" {
				t.Fatalf("pairing invalidation: %+v", view)
			}
			loc, _ := agentconfig.ResolveLocations(provider)
			if err := os.WriteFile(filepath.Join(loc.Home, loc.MCPFile), []byte("malformed"), 0600); err != nil {
				t.Fatal(err)
			}
			if view := readSetupTest(t, d, provider); view.Steps["mcp"].Status != "unknown" {
				t.Fatal("unreadable config classified as incomplete")
			}
		})
	}
}
func TestSetupVerifiesInstalledSkillsAndRetainsSkipAcrossChangedAssets(t *testing.T) {
	d := setupFixture(t)
	config := agentconfig.Config{SchemaVersion: agentconfig.Version, Skills: []agentconfig.Skill{{ID: "implement", Directory: "implement-issue", Content: "Test skill"}}}
	result, err := installProviderSkills(d.repoRoot, config, "codex")
	if err != nil || !result.Success {
		t.Fatalf("install: %+v %v", result, err)
	}
	if err := d.recordSetupAssets("codex", "skills", "p", d.repoRoot, ""); err != nil {
		t.Fatal(err)
	}
	view := readSetupTest(t, d, "codex")
	if view.Steps["skills"].Status != "success" {
		t.Fatal("installed skills not verified")
	}
	loc, _ := agentconfig.ResolveLocations("codex")
	if err := os.WriteFile(filepath.Join(loc.Home, loc.SkillDir, "implement-issue/SKILL.md"), []byte("edited"), 0600); err != nil {
		t.Fatal(err)
	}
	if readSetupTest(t, d, "codex").Steps["skills"].Status != "pending" {
		t.Fatal("edited asset not invalidated")
	}
	acknowledgeSetupTest(t, d, "codex", "skills", "skipped")
	if err := os.Remove(filepath.Join(loc.Home, loc.SkillDir, "implement-issue/SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if readSetupTest(t, d, "codex").Steps["skills"].Status != "skipped" {
		t.Fatal("explicit skip depended on plugin/assets")
	}
}
func TestSetupUnknownPairingRejectsAcknowledgementsAndInvalidDecisions(t *testing.T) {
	d := setupFixture(t)
	for _, body := range []string{`{"step":"mcp","decision":"skipped"}`, `{"step":"skills","decision":"retained"}`, `{}`} {
		if rec := disconnectRequest(d, "POST", "/desktop/setup?provider=codex", body); rec.Code != 400 {
			t.Fatalf("invalid decision accepted: %s", body)
		}
	}
	if rec := disconnectRequest(d, "GET", "/desktop/setup?provider=custom", ""); rec.Code != 400 {
		t.Fatal("custom provider accepted")
	}
	d.link.token = ""
	for _, step := range readSetupTest(t, d, "codex").Steps {
		if step.Status != "unknown" {
			t.Fatal("missing pairing inferred incomplete")
		}
	}
	r := httptest.NewRequest("GET", "/desktop/setup?provider=codex", nil)
	w := httptest.NewRecorder()
	d.desktopHandler(w, r)
	if w.Code != 401 {
		t.Fatal("setup was unauthenticated")
	}
}

func TestSetupSDDAssetObservationExcludesIssueArtifacts(t *testing.T) {
	d := setupFixture(t)
	root := t.TempDir()
	for path, content := range map[string]string{"openspec/config.yaml": "schema: spec-driven", "openspec/changes/example/proposal.md": "proposal", ".agents/skills/openspec-apply-change/SKILL.md": "framework skill"} {
		absolute := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(absolute), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.recordSetupAssets("codex", "sdd", "p", root, "openspec"); err != nil {
		t.Fatal(err)
	}
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		t.Fatal(err)
	}
	saved := settings.Setup["codex"]["sdd"]
	if saved.Root != root || saved.Framework != "openspec" || len(saved.Assets) != 2 || !setupAssetsValid(saved) {
		t.Fatalf("wrong scope/assets: %+v", saved)
	}
	if err := os.WriteFile(filepath.Join(root, "openspec/changes/example/proposal.md"), []byte("changed proposal"), 0600); err != nil {
		t.Fatal(err)
	}
	if !setupAssetsValid(saved) {
		t.Fatal("issue artifact invalidated SDD")
	}
	if err := os.Remove(filepath.Join(root, ".agents/skills/openspec-apply-change/SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if setupAssetsValid(saved) {
		t.Fatal("missing framework skill not detected")
	}
}

func TestSetupSDDChangedRootFrameworkAndUnavailableSource(t *testing.T) {
	d, config := disconnectFixture(t)
	config.AIProvider = "claude"
	config.SpecFramework = "openspec"
	var framework atomic.Value
	framework.Store("openspec")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := config
		response.SpecFramework = framework.Load().(string)
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	d.link.serverURL = server.URL
	d.link.token = "paired"
	path := filepath.Join(d.repoRoot, "openspec/config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("schema: spec-driven"), 0600); err != nil {
		t.Fatal(err)
	}
	d.queue.runs = map[string]*controlledRun{"active": {exited: make(chan struct{})}}
	if rec := disconnectRequest(d, "POST", "/desktop/project?id=p&action=framework&provider=claude", ""); rec.Code != 409 {
		t.Fatalf("active project deployment: %d", rec.Code)
	}
	d.queue.runs = nil
	framework.Store("none")
	if rec := disconnectRequest(d, "POST", "/desktop/project?id=p&action=framework&provider=claude", ""); rec.Code != 400 {
		t.Fatalf("missing framework deployment: %d", rec.Code)
	}
	framework.Store("openspec")
	if rec := disconnectRequest(d, "POST", "/desktop/project?id=p&action=framework&provider=claude", ""); rec.Code != 200 {
		t.Fatalf("framework deployment: %d %s", rec.Code, rec.Body.String())
	}
	acknowledgeSetupTest(t, d, "claude", "skills", "skipped")
	if readSetupTest(t, d, "claude").Steps["sdd"].Status != "success" {
		t.Fatal("SDD installation not read back")
	}
	framework.Store("speckit")
	if readSetupTest(t, d, "claude").Steps["sdd"].Status != "pending" {
		t.Fatal("framework change not invalidated")
	}
	framework.Store("openspec")
	other := t.TempDir()
	if _, err := gitLocal(context.Background(), other, "init"); err != nil {
		t.Fatal(err)
	}
	_, err := agentconfig.UpdateSettings(d.localSettingsRoot(), func(s *agentconfig.Settings) error {
		p := s.Project("p")
		p.Path = other
		s.SetProject("p", p)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	view := readSetupTest(t, d, "claude")
	if view.Steps["sdd"].Status != "pending" || view.Steps["skills"].Status != "skipped" {
		t.Fatalf("root invalidation leaked: %+v", view)
	}
	_, err = agentconfig.UpdateSettings(d.localSettingsRoot(), func(s *agentconfig.Settings) error {
		p := s.Project("p")
		p.Path = d.repoRoot
		s.SetProject("p", p)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	server.Close()
	if readSetupTest(t, d, "claude").Steps["sdd"].Status != "unknown" {
		t.Fatal("unavailable project inferred incomplete")
	}
}

func TestSetupMCPTransportReadbackAndChangedBridge(t *testing.T) {
	for _, provider := range []string{"claude", "codex", "agy"} {
		for _, mode := range []string{"local-http", "remote-stdio"} {
			t.Run(provider+"/"+mode, func(t *testing.T) {
				d := setupFixture(t)
				executable, _ := os.Executable()
				local := mode == "local-http"
				target, transport, server := "remote", "stdio", d.link.serverURL
				if local {
					target, transport, server = "local", "http", d.loopback.url
				}
				if _, err := agentconfig.ConfigureMCP(provider, executable, server, d.link.token, transport, local); err != nil {
					t.Fatal(err)
				}
				_, err := agentconfig.UpdateSettings(d.localSettingsRoot(), func(s *agentconfig.Settings) error {
					s.MCPConnections = map[string]agentconfig.MCPConnection{provider: {Target: target, Transport: transport}}
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				view := readSetupTest(t, d, provider)
				connection := view.Steps["mcp"].Configuration
				if view.Steps["mcp"].Status != "success" || !connection.Valid || connection.Target != target || connection.Transport != transport {
					t.Fatalf("wrong transport readback: %+v", view)
				}
				if !local {
					if _, err := agentconfig.ConfigureMCP(provider, filepath.Join(t.TempDir(), "other-agent"), server, d.link.token, transport, false); err != nil {
						t.Fatal(err)
					}
					if readSetupTest(t, d, provider).Steps["mcp"].Status != "pending" {
						t.Fatal("changed bridge accepted")
					}
				}
			})
		}
	}
}
