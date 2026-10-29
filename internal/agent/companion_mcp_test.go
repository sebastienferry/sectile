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
