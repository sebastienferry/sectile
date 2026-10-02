package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"tasks/internal/agentconfig"
	"tasks/internal/testhome"
	"testing"
)

func TestDesktopMCPChoiceSurvivesBootstrapAndRestart(t *testing.T) {
	home := testhome.Temp(t)
	d := &agentDaemon{repoRoot: t.TempDir(), loopback: loopbackServer{url: "http://127.0.0.1:4567", desktopToken: "private"}, link: serverLink{serverURL: "https://sectile.example.test", token: "secret-key"}}
	route := "/desktop/mcp?provider=codex"
	if w := disconnectRequest(d, "GET", route, ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"transport":"http"`) || !strings.Contains(w.Body.String(), `"target":"remote"`) {
		t.Fatal("MCP setup must default to remote HTTP", w.Code, w.Body.String())
	}
	for _, body := range []string{`{"target":"other","transport":"http"}`, `{"target":"local","transport":"invalid"}`} {
		if w := disconnectRequest(d, "POST", route, body); w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	// A retired provider (#614) has no configuration to read or write.
	for _, provider := range []string{"gemini", "cursor", "vibe"} {
		retired := "/desktop/mcp?provider=" + provider
		if w := disconnectRequest(d, "GET", retired, ""); w.Code != 400 {
			t.Fatal(provider, w.Code, w.Body.String())
		}
		if w := disconnectRequest(d, "POST", retired, `{"target":"remote","transport":"http"}`); w.Code != 400 {
			t.Fatal(provider, w.Code, w.Body.String())
		}
	}
	if w := disconnectRequest(d, "POST", route, `{"target":"local","transport":"http"}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if !d.localMCPEnabled() {
		t.Fatal("local MCP not enabled")
	}
	d.loopback.url = "http://127.0.0.1:4568"
	if err := d.refreshMCPConnections(); err != nil {
		t.Fatal(err)
	}
	config := agentconfig.Config{AIProvider: "codex"}
	if err := d.bootstrapLocalMCP(&config); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil || !strings.Contains(string(raw), "4568/mcp") || strings.Contains(string(raw), "secret-key") {
		t.Fatalf("wrong restored registration: %s %v", raw, err)
	}
	if w := disconnectRequest(d, "POST", route, `{"target":"remote","transport":"http"}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if d.localMCPEnabled() {
		t.Fatal("local MCP still enabled")
	}
	w := disconnectRequest(d, "GET", route, "")
	if strings.Contains(w.Body.String(), "secret-key") {
		t.Fatal("preview leaked credential")
	}
	request := httptest.NewRequest("POST", route, strings.NewReader(`{"target":"local","transport":"http"}`))
	response := httptest.NewRecorder()
	d.desktopHandler(response, request)
	if response.Code != 401 {
		t.Fatal("unauthenticated config write accepted")
	}
}

func TestLocalMCPOptInDoesNotOpenOtherSurfaces(t *testing.T) {
	testhome.Temp(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer paired" {
			t.Error("missing paired identity")
		}
		w.WriteHeader(200)
	}))
	defer upstream.Close()
	d := &agentDaemon{repoRoot: t.TempDir(), link: serverLink{serverURL: upstream.URL, token: "paired"}}
	if err := d.startLocalProxy(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer d.loopback.server.Close()
	check := func(path, origin, host string, want int) {
		t.Helper()
		req, _ := http.NewRequest("POST", d.loopback.url+path, nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if host != "" {
			req.Host = host
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Fatalf("%s got %d want %d", path, resp.StatusCode, want)
		}
	}
	check("/mcp", "", "", 401)
	settings := agentconfig.Settings{MCPConnections: map[string]agentconfig.MCPConnection{"codex": {Target: "local", Transport: "http"}}}
	if err := agentconfig.WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	check("/mcp", "", "", 200)
	check("/mcp", "https://evil.example", "", 403)
	check("/mcp", "", "evil.example", 403)
	check("/api/tasks", "", "", 401)
	check("/desktop/status", "", "", 401)
	settings.MCPConnections["codex"] = agentconfig.MCPConnection{Target: "remote", Transport: "http"}
	if err := agentconfig.WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	check("/mcp", "", "", 401)
}

// A registration nobody saved from the desktop, written by `sectile-agent
// init` or with an earlier key, follows the key the daemon starts with; a
// provider without a registration still gets none (#717).
func TestDaemonStartRefreshesAnUnsavedRegistrationsKey(t *testing.T) {
	home := testhome.Temp(t)
	claude := filepath.Join(home, ".claude.json")
	if err := os.WriteFile(claude, []byte(`{"mcpServers":{"sectile":{"type":"http","url":"https://sectile.example.test/mcp","headers":{"Authorization":"Bearer old-key"}}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	agy := filepath.Join(home, ".gemini", "config", "mcp_config.json")
	if err := os.MkdirAll(filepath.Dir(agy), 0700); err != nil {
		t.Fatal(err)
	}
	unregistered := `{"mcpServers":{"other":{"command":"other-server"}}}`
	if err := os.WriteFile(agy, []byte(unregistered), 0600); err != nil {
		t.Fatal(err)
	}
	d := &agentDaemon{repoRoot: t.TempDir(), loopback: loopbackServer{url: "http://127.0.0.1:4567", desktopToken: "private"}, link: serverLink{serverURL: "https://sectile.example.test", token: "new-key"}}

	if err := d.refreshMCPConnections(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(claude)
	if err != nil || !strings.Contains(string(raw), "Bearer new-key") || strings.Contains(string(raw), "old-key") {
		t.Fatalf("registration not refreshed: %s %v", raw, err)
	}
	if raw, err := os.ReadFile(agy); err != nil || string(raw) != unregistered {
		t.Fatalf("a provider without a registration was given one: %s %v", raw, err)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "config.toml")); !os.IsNotExist(err) {
		t.Fatalf("a missing provider file was created: %v", err)
	}
}
