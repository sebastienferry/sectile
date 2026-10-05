package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
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

// A Codex or Antigravity registration nobody saved from the desktop, written
// by `sectile-agent init` or with an earlier key, follows the key the daemon
// starts with; a provider without a registration still gets none (#717). An
// unmanaged Claude Code entry is only reported, and rewritten on Repair (ADR 0023).
func TestDaemonStartRefreshesAnUnsavedRegistrationsKey(t *testing.T) {
	home := testhome.Temp(t)
	agy := filepath.Join(home, ".gemini", "config", "mcp_config.json")
	if err := os.MkdirAll(filepath.Dir(agy), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agy, []byte(`{"mcpServers":{"sectile":{"serverUrl":"https://sectile.example.test/mcp","headers":{"Authorization":"Bearer old-key"}}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	claude := filepath.Join(home, ".claude.json")
	unmanaged := `{"mcpServers":{"sectile":{"type":"http","url":"https://sectile.example.test/mcp","headers":{"Authorization":"Bearer old-key"}}}}`
	if err := os.WriteFile(claude, []byte(unmanaged), 0600); err != nil {
		t.Fatal(err)
	}
	d := &agentDaemon{repoRoot: t.TempDir(), loopback: loopbackServer{url: "http://127.0.0.1:4567", desktopToken: "private"}, link: serverLink{serverURL: "https://sectile.example.test", token: "new-key"}}

	if err := d.refreshMCPConnections(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(agy)
	if err != nil || !strings.Contains(string(raw), "Bearer new-key") || strings.Contains(string(raw), "old-key") {
		t.Fatalf("registration not refreshed: %s %v", raw, err)
	}
	if raw, err := os.ReadFile(claude); err != nil || string(raw) != unmanaged {
		t.Fatalf("an unmanaged Claude Code entry was adopted: %s %v", raw, err)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "config.toml")); !os.IsNotExist(err) {
		t.Fatalf("a missing provider file was created: %v", err)
	}
}

// `sectile-agent pair` or the desktop stored a newer key and rewrote the
// registrations to it, then revoked the old one; a daemon restarting with the
// old key (TOKEN survives the self-restart) leaves them alone, saved desktop
// choice or not (#717, ADR 0049). A legacy stored connection without a server
// counts as the same server, as in resolveCredential.
func TestDaemonStartLeavesMCPAloneWhenTheDaemonHoldsAnOlderKey(t *testing.T) {
	for name, storedServer := range map[string]string{"same server": "https://sectile.example.test", "legacy connection without a server": ""} {
		t.Run(name, func(t *testing.T) {
			home := testhome.Temp(t)
			claude := filepath.Join(home, ".claude.json")
			unsaved := `{"mcpServers":{"sectile":{"type":"http","url":"https://sectile.example.test/mcp","headers":{"Authorization":"Bearer stored-key"}}}}`
			if err := os.WriteFile(claude, []byte(unsaved), 0600); err != nil {
				t.Fatal(err)
			}
			agy := filepath.Join(home, ".gemini", "config", "mcp_config.json")
			if err := os.MkdirAll(filepath.Dir(agy), 0700); err != nil {
				t.Fatal(err)
			}
			saved := `{"mcpServers":{"sectile":{"serverUrl":"https://sectile.example.test/mcp","headers":{"Authorization":"Bearer stored-key"}}}}`
			if err := os.WriteFile(agy, []byte(saved), 0600); err != nil {
				t.Fatal(err)
			}
			if err := agentconfig.WriteSettings(agentconfig.Settings{MCPConnections: map[string]agentconfig.MCPConnection{"agy": {Target: "remote", Transport: "http"}}}); err != nil {
				t.Fatal(err)
			}
			if err := agentconfig.WriteConnection(agentconfig.Connection{Server: storedServer, APIKey: "stored-key"}); err != nil {
				t.Fatal(err)
			}
			d := &agentDaemon{repoRoot: t.TempDir(), loopback: loopbackServer{url: "http://127.0.0.1:4567", desktopToken: "private"}, link: serverLink{serverURL: "https://sectile.example.test/", token: "older-key"}}

			if err := d.refreshMCPConnections(); err != nil {
				t.Fatal(err)
			}
			if raw, err := os.ReadFile(claude); err != nil || string(raw) != unsaved {
				t.Fatalf("an unsaved registration was given the older key: %s %v", raw, err)
			}
			if raw, err := os.ReadFile(agy); err != nil || string(raw) != saved {
				t.Fatalf("a saved desktop choice was given the older key: %s %v", raw, err)
			}
		})
	}
}

// A daemon restarted with an older key reports no repair for an entry already
// on the newer key `sectile-agent pair` stored, and a Repair writes that newer
// key, never its own (#717).
func TestDesktopMCPFollowsTheNewerStoredKeyWhenTheDaemonHoldsAnOlderOne(t *testing.T) {
	home := testhome.Temp(t)
	if err := agentconfig.WriteConnection(agentconfig.Connection{Server: "https://sectile.example.test", APIKey: "stored-key"}); err != nil {
		t.Fatal(err)
	}
	onNewer := `{"mcpServers":{"sectile":{"type":"http","url":"https://sectile.example.test/mcp","headers":{"Authorization":"Bearer stored-key"}}}}`
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(onNewer), 0600); err != nil {
		t.Fatal(err)
	}
	d := &agentDaemon{repoRoot: t.TempDir(), loopback: loopbackServer{desktopToken: "private"}, link: serverLink{serverURL: "https://sectile.example.test", token: "older-key"}}

	if response := decodeClaudeMCP(t, d); response.NeedsRepair || len(response.Entries) != 1 || !response.Entries[0].KeyMatches {
		t.Fatalf("an entry on the newer key was reported for repair: %+v", response)
	}
	if w := disconnectRequest(d, "POST", "/desktop/mcp?provider=claude", `{"target":"remote","transport":"http","repair":true}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if raw := readClaudeMCP(t, home); !strings.Contains(raw, "Bearer stored-key") || strings.Contains(raw, "older-key") {
		t.Fatalf("repair wrote the older key: %s", raw)
	}
	if response := decodeClaudeMCP(t, d); response.NeedsRepair || !response.Entries[0].Managed {
		t.Fatalf("repaired entry: %+v", response)
	}
}

// claudeMCPFixture is a ~/.claude.json Sectile did not write: the user entry
// and one project entry carry a key from an earlier pairing (#716).
const claudeMCPFixture = `{
  "mcpServers": {"sectile": {"type": "http", "url": "https://sectile.example.test/mcp", "headers": {"Authorization": "Bearer old-key"}}},
  "projects": {"/work/app": {"mcpServers": {"sectile": {"type": "http", "url": "https://sectile.example.test/mcp", "headers": {"Authorization": "Bearer old-key"}}, "other": {"command": "other-server"}}}}
}`

type claudeMCPResponse struct {
	Entries     []mcpEntryView `json:"entries"`
	NeedsRepair bool           `json:"needsRepair"`
}

func mcpTestDaemon(t *testing.T) (*agentDaemon, string) {
	t.Helper()
	home := testhome.Temp(t)
	d := &agentDaemon{repoRoot: t.TempDir(), loopback: loopbackServer{desktopToken: "private"}, link: serverLink{serverURL: "https://sectile.example.test", token: "secret-key"}}
	return d, home
}

func readClaudeMCP(t *testing.T, home string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(home, ".claude.json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestRefreshMCPConnectionsRewritesOnlyWhenTheKeyChanged(t *testing.T) {
	d, home := mcpTestDaemon(t)
	if w := disconnectRequest(d, "POST", "/desktop/mcp?provider=claude", `{"target":"remote","transport":"http"}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	// Tamper with the written entry: a refresh that rewrote it would restore the key.
	path := filepath.Join(home, ".claude.json")
	tampered := strings.ReplaceAll(readClaudeMCP(t, home), "Bearer secret-key", "Bearer tampered")
	if err := os.WriteFile(path, []byte(tampered), 0600); err != nil {
		t.Fatal(err)
	}
	if err := d.refreshMCPConnections(); err != nil {
		t.Fatal(err)
	}
	if readClaudeMCP(t, home) != tampered {
		t.Fatal("an unchanged registration was rewritten")
	}
	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)
	d.link.token = "rotated-key"
	if err := d.refreshMCPConnections(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readClaudeMCP(t, home), "Bearer rotated-key") {
		t.Fatalf("key not rewritten: %s", readClaudeMCP(t, home))
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil || settings.MCPConnections["claude"].Written != mcpFingerprint(d.link.serverURL, "rotated-key", executable) {
		t.Fatalf("fingerprint not saved: %+v %v", settings.MCPConnections, err)
	}
	if !strings.Contains(logs.String(), "MCP registration for claude rewritten") || strings.Contains(logs.String(), "secret-key") || strings.Contains(logs.String(), "rotated-key") {
		t.Fatalf("log: %s", logs.String())
	}
}

func TestRefreshMCPConnectionsContinuesPastAFailingProvider(t *testing.T) {
	d, home := mcpTestDaemon(t)
	// Codex is saved local, but this daemon has no loopback, so its rewrite fails.
	settings := agentconfig.Settings{MCPConnections: map[string]agentconfig.MCPConnection{
		"codex":  {Target: "local", Transport: "http"},
		"claude": {Target: "remote", Transport: "http"},
	}}
	if err := agentconfig.WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	err := d.refreshMCPConnections()
	if err == nil || !strings.Contains(err.Error(), "codex") {
		t.Fatalf("failure not reported: %v", err)
	}
	if !strings.Contains(readClaudeMCP(t, home), "Bearer secret-key") {
		t.Fatal("Claude was not rewritten past the failing provider")
	}
	saved, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil || saved.MCPConnections["claude"].Written == "" || saved.MCPConnections["codex"].Written != "" {
		t.Fatalf("fingerprints: %+v %v", saved.MCPConnections, err)
	}
}

func TestDesktopMCPReportsUnmanagedStaleEntries(t *testing.T) {
	d, home := mcpTestDaemon(t)
	path := filepath.Join(home, ".claude.json")
	if err := os.WriteFile(path, []byte(claudeMCPFixture), 0600); err != nil {
		t.Fatal(err)
	}
	w := disconnectRequest(d, "GET", "/desktop/mcp?provider=claude", "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "old-key") || strings.Contains(w.Body.String(), "secret-key") {
		t.Fatalf("response leaked a key: %s", w.Body.String())
	}
	var response claudeMCPResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Entries) != 2 || !response.NeedsRepair {
		t.Fatalf("response: %s", w.Body.String())
	}
	for _, entry := range response.Entries {
		if !entry.Stale || entry.Managed || entry.KeyMatches || !entry.URLMatches {
			t.Fatalf("entry: %+v", entry)
		}
	}
	if response.Entries[0].Scope != "user" || response.Entries[1].Scope != "project" || response.Entries[1].Project != "/work/app" {
		t.Fatalf("scopes: %+v", response.Entries)
	}
	if readClaudeMCP(t, home) != claudeMCPFixture {
		t.Fatal("a report changed the file")
	}
}

func TestDesktopMCPRepairRewritesUserEntryAndRemovesStaleProjectEntry(t *testing.T) {
	d, home := mcpTestDaemon(t)
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(claudeMCPFixture), 0600); err != nil {
		t.Fatal(err)
	}
	w := disconnectRequest(d, "POST", "/desktop/mcp?provider=claude", `{"target":"remote","transport":"http","repair":true}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(readClaudeMCP(t, home)), &data); err != nil {
		t.Fatal(err)
	}
	user := data["mcpServers"].(map[string]any)["sectile"].(map[string]any)
	if user["headers"].(map[string]any)["Authorization"] != "Bearer secret-key" {
		t.Fatalf("user entry: %#v", user)
	}
	servers := data["projects"].(map[string]any)["/work/app"].(map[string]any)["mcpServers"].(map[string]any)
	if servers["sectile"] != nil || servers["other"] == nil {
		t.Fatalf("project servers: %#v", servers)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if choice := settings.MCPConnections["claude"]; err != nil || choice.Target != "remote" || choice.Transport != "http" || choice.Written != mcpFingerprint(d.link.serverURL, "secret-key", executable) {
		t.Fatalf("choice: %+v %v", choice, err)
	}
	var response claudeMCPResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.NeedsRepair || len(response.Entries) != 1 || !response.Entries[0].Managed || response.Entries[0].Stale {
		t.Fatalf("response: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "secret-key") {
		t.Fatalf("response leaked the key: %s", w.Body.String())
	}
}

func decodeClaudeMCP(t *testing.T, d *agentDaemon) claudeMCPResponse {
	t.Helper()
	w := disconnectRequest(d, "GET", "/desktop/mcp?provider=claude", "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var response claudeMCPResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response
}

func TestDesktopMCPReportsATamperedManagedEntry(t *testing.T) {
	d, home := mcpTestDaemon(t)
	if w := disconnectRequest(d, "POST", "/desktop/mcp?provider=claude", `{"target":"remote","transport":"http"}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	// The fingerprint still matches, so only the entry content tells the key was replaced.
	tampered := strings.ReplaceAll(readClaudeMCP(t, home), "Bearer secret-key", "Bearer other-key")
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(tampered), 0600); err != nil {
		t.Fatal(err)
	}
	response := decodeClaudeMCP(t, d)
	if !response.NeedsRepair || len(response.Entries) != 1 {
		t.Fatalf("response: %+v", response)
	}
	if entry := response.Entries[0]; entry.Scope != "user" || !entry.Managed || entry.KeyMatches || !entry.Stale {
		t.Fatalf("entry: %+v", entry)
	}
}

func TestDesktopMCPAcceptsAKeylessLoopbackProjectEntryUnderALocalChoice(t *testing.T) {
	d, home := mcpTestDaemon(t)
	d.loopback.url = "http://127.0.0.1:4567"
	settings := agentconfig.Settings{MCPConnections: map[string]agentconfig.MCPConnection{"claude": {Target: "local", Transport: "http"}}}
	if err := agentconfig.WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	fixture := `{"projects": {"/work/app": {"mcpServers": {"sectile": {"type": "http", "url": "` + agentconfig.MCPURL(d.loopback.url) + `"}}}}}`
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(fixture), 0600); err != nil {
		t.Fatal(err)
	}
	response := decodeClaudeMCP(t, d)
	if response.NeedsRepair || len(response.Entries) != 1 {
		t.Fatalf("response: %+v", response)
	}
	if entry := response.Entries[0]; entry.Scope != "project" || !entry.KeyMatches || !entry.URLMatches || entry.Stale {
		t.Fatalf("entry: %+v", entry)
	}
}

func TestDesktopMCPReportsAMalformedClaudeFileWithoutFailing(t *testing.T) {
	d, home := mcpTestDaemon(t)
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"mcpServers": `), 0600); err != nil {
		t.Fatal(err)
	}
	w := disconnectRequest(d, "GET", "/desktop/mcp?provider=claude", "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var response struct {
		claudeMCPResponse
		EntriesError string `json:"entriesError"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.EntriesError == "" || response.NeedsRepair || response.Entries == nil || len(response.Entries) != 0 {
		t.Fatalf("response: %s", w.Body.String())
	}
}

func TestDesktopMCPRepairRejectsOtherProviders(t *testing.T) {
	d, home := mcpTestDaemon(t)
	w := disconnectRequest(d, "POST", "/desktop/mcp?provider=codex", `{"target":"remote","transport":"http","repair":true}`)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "Claude Code only") {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "config.toml")); !os.IsNotExist(err) {
		t.Fatalf("refused repair wrote the configuration: %v", err)
	}
}
