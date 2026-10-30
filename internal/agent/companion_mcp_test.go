package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"tasks/internal/testhome"
	"testing"
)

func TestCompanionMCPUsesProcessOverridesAndKeepsKeysOffArguments(t *testing.T) {
	t.Setenv("SECTILE_COMPANION_MCP_URL", "http://127.0.0.1:9876")
	t.Setenv("TOKEN", "private-companion-key")
	t.Setenv("SECTILE_PAIRED_DEVICE_ID", "companion-device")
	claude := claudeConversationCommand(t.TempDir(), "", "", "", "", "", nil, nil)
	codex := codexConversationCommand(t.TempDir(), nil)
	// Inspect arguments without launching either CLI.
	if !strings.Contains(strings.Join(claude.Args, " "), "--strict-mcp-config") || !strings.Contains(strings.Join(codex.Args, " "), "mcp_servers.sectile.enabled=false") {
		t.Fatal("provider commands did not isolate the shared registration")
	}
	for _, args := range [][]string{claude.Args, codex.Args} {
		line := strings.Join(args, " ")
		if !strings.Contains(line, companionMCPName) || strings.Contains(line, "private-companion-key") {
			t.Fatal("invalid private MCP arguments")
		}
	}
	if !strings.Contains(strings.Join(codex.Env, "\n"), "SECTILE_AGENT_TOKEN=private-companion-key") {
		t.Fatal("bridge key was not forwarded through the environment")
	}
	var config struct {
		Servers map[string]any `json:"mcpServers"`
	}
	for i, arg := range claude.Args {
		if arg == "--mcp-config" {
			if err := json.Unmarshal([]byte(claude.Args[i+1]), &config); err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(config.Servers) != 1 || config.Servers[companionMCPName] == nil {
		t.Fatal("Claude must use only its companion MCP")
	}
	t.Setenv("SECTILE_COMPANION_MCP_URL", "")
	if len(companionMCPArgs("claude")) != 0 || conversationMCPName() != "sectile" {
		t.Fatal("Electron launch semantics changed")
	}
}

func TestCompanionMCPSetupNeverWritesSharedProviderFiles(t *testing.T) {
	home := testhome.Temp(t)
	t.Setenv("SECTILE_COMPANION_MCP_URL", "http://127.0.0.1:9876")
	for _, file := range []string{".claude.json", ".codex/config.toml"} {
		path := filepath.Join(home, file)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("existing Electron configuration"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	d := &agentDaemon{repoRoot: t.TempDir(), loopback: loopbackServer{desktopToken: "private"}, link: serverLink{serverURL: companionMCPURL(), token: "private"}}
	for _, provider := range []string{"claude", "codex"} {
		for _, method := range []string{"GET", "POST"} {
			w := disconnectRequest(d, method, "/desktop/mcp?provider="+provider, `{ "target":"local", "transport":"http" }`)
			if w.Code != 200 || !strings.Contains(w.Body.String(), `"scope":"session"`) || strings.Contains(w.Body.String(), "private") {
				t.Fatal("invalid scoped setup response", w.Code)
			}
		}
		if _, err := d.registerMCP(provider, "/unused"); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{".claude.json", ".codex/config.toml"} {
		data, err := os.ReadFile(filepath.Join(home, file))
		if err != nil || string(data) != "existing Electron configuration" {
			t.Fatal("shared registration changed")
		}
	}
}

func TestCompanionMCPRejectsRemoteEndpointWithoutSharedFallback(t *testing.T) {
	t.Setenv("SECTILE_COMPANION_MCP_URL", "https://unrelated.example.test")
	if args := companionMCPArgs("codex"); len(args) != 1 || args[0] != "--invalid-companion-mcp-endpoint" {
		t.Fatal("invalid endpoint fell back to a shared registration")
	}
}

func TestCompanionMCPTemplatesRequireTheScopedArgumentsEvenThroughWrappers(t *testing.T) {
	t.Setenv("SECTILE_COMPANION_MCP_URL", "http://127.0.0.1:9876")
	if _, err := consoleLaunch("claude", "", "env claude {prompt}", "", "/fixture", nil); err == nil {
		t.Fatal("wrapper silently used shared MCP")
	}
	line, err := consoleLaunch("claude", "", "env claude {mcpArgs} {prompt}", "", "/fixture", nil)
	if err != nil || !strings.Contains(line, "--strict-mcp-config") || !strings.Contains(line, companionMCPName) {
		t.Fatal("wrapper lost the selected provider's connection", err)
	}
}

func TestEmbeddedConversationUsesExplicitDaemonCredential(t *testing.T) {
	t.Setenv("SECTILE_COMPANION_MCP_URL", "http://127.0.0.1:9876")
	t.Setenv("TOKEN", "")
	daemon := &agentDaemon{link: serverLink{serverURL: companionMCPURL(), token: "fixture-private-key", deviceID: "preserved-device"}}
	env := daemon.companionEnvironment(map[string]string{"SECTILE_PROJECT_ID": "project"})
	if env["SECTILE_AGENT_TOKEN"] != "fixture-private-key" || env["SECTILE_DEVICE_ID"] != "preserved-device" {
		t.Fatal("embedded conversation lost its explicit machine credential")
	}
}

func TestNativeFleetMCPUsesSelectedOriginWithoutSharedRegistration(t *testing.T) {
	home := testhome.Temp(t)
	path := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	original := []byte("existing Electron configuration")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SECTILE_DESKTOP_MCP_URL", "https://fleet.example.test")
	t.Setenv("SECTILE_COMPANION_MCP_URL", "http://127.0.0.1:9876")
	daemon := &agentDaemon{link: serverLink{serverURL: "https://fleet.example.test", token: "fleet-private-key", deviceID: "fleet-device"}}
	if err := daemon.refreshMCPConnections(); err != nil {
		t.Fatal(err)
	}
	for _, provider := range []string{"claude", "codex"} {
		args := strings.Join(companionMCPArgs(provider), " ")
		if !strings.Contains(args, companionMCPName) || !strings.Contains(args, "https://fleet.example.test") || strings.Contains(args, "fleet-private-key") || strings.Contains(args, "127.0.0.1") {
			t.Fatalf("%s did not use the selected Fleet origin privately", provider)
		}
		if _, err := daemon.registerMCP(provider, "/unused"); err != nil {
			t.Fatal(err)
		}
	}
	env := daemon.companionEnvironment(map[string]string{})
	if env["SECTILE_AGENT_TOKEN"] != "fleet-private-key" || env["SECTILE_DEVICE_ID"] != "fleet-device" {
		t.Fatal("Fleet bridge lost the paired identity")
	}
	actual, err := os.ReadFile(path)
	if err != nil || string(actual) != string(original) {
		t.Fatal("native Fleet changed Electron's shared configuration")
	}
}

func TestNativeFleetMCPRejectsUnsafeOriginsWithoutSharedFallback(t *testing.T) {
	for _, endpoint := range []string{"http://fleet.example.test", "https://user:secret@fleet.example.test", "https://fleet.example.test/path", "https://fleet.example.test?key=secret", "https://fleet.example.test?", "https://fleet.example.test#fragment"} {
		t.Run(endpoint, func(t *testing.T) {
			t.Setenv("SECTILE_DESKTOP_MCP_URL", endpoint)
			for _, provider := range []string{"claude", "codex"} {
				args := companionMCPArgs(provider)
				if len(args) != 1 || args[0] != "--invalid-companion-mcp-endpoint" {
					t.Fatal("unsafe Fleet origin did not fail closed")
				}
			}
		})
	}
	for _, endpoint := range []string{"https://fleet.example.test", "http://127.0.0.1:9876", "http://localhost:9876", "http://[::1]:9876"} {
		t.Run(endpoint, func(t *testing.T) {
			t.Setenv("SECTILE_DESKTOP_MCP_URL", endpoint)
			if args := companionMCPArgs("codex"); len(args) < 2 {
				t.Fatal("valid Fleet origin was rejected")
			}
		})
	}
}
