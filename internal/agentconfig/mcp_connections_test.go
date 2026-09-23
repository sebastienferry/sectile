package agentconfig

import (
	"os"
	"strings"
	"tasks/internal/testhome"
	"testing"
)

func TestMCPConnectionSwitchPreservesPolicy(t *testing.T) {
	for _, provider := range []string{"claude", "codex", "agy", "cursor", "gemini", "vibe"} {
		t.Run(provider, func(t *testing.T) {
			testhome.Temp(t)
			path, err := BootstrapMCP(provider, "/opt/sectile-agent", testServer, testKey)
			if err != nil {
				t.Fatal(err)
			}
			data := readMCPFixture(t, path)
			key := "mcpServers"
			if provider == "codex" || provider == "vibe" {
				key = "mcp_servers"
			}
			entry := func(data map[string]any) map[string]any {
				if provider == "vibe" {
					return data[key].([]any)[0].(map[string]any)
				}
				return data[key].(map[string]any)["sectile"].(map[string]any)
			}
			data["unrelated"] = "keep"
			entry(data)["startup_timeout_sec"] = int64(45)
			writeMCPFixture(t, path, data)
			for _, local := range []bool{true, false, true} {
				for _, transport := range []string{"http", "stdio"} {
					server := testServer
					if local {
						server = "http://127.0.0.1:4567"
					}
					if _, err := ConfigureMCP(provider, "/opt/sectile-agent", server, testKey, transport, local); err != nil {
						t.Fatal(err)
					}
					result := readMCPFixture(t, path)
					current := entry(result)
					if result["unrelated"] != "keep" || current["startup_timeout_sec"] == nil {
						t.Fatalf("lost settings: %#v", result)
					}
					raw, _ := os.ReadFile(path)
					if strings.Contains(string(raw), testKey) == local {
						t.Fatalf("wrong credential handling: %s", raw)
					}
					if transport == "http" {
						if current["command"] != nil || current["env"] != nil {
							t.Fatal("stale stdio transport")
						}
						urlField := "url"
						if provider == "agy" {
							urlField = "serverUrl"
						}
						if provider == "gemini" {
							urlField = "httpUrl"
						}
						if current[urlField] != server+"/mcp" {
							t.Fatalf("wrong endpoint: %#v", current)
						}
						if provider == "vibe" && current["transport"] != "streamable-http" {
							t.Fatal("wrong Vibe transport")
						}
					} else if current["url"] != nil || current["headers"] != nil || current["http_headers"] != nil || current["serverUrl"] != nil || current["httpUrl"] != nil {
						t.Fatal("stale HTTP transport")
					}
				}
			}
		})
	}
}

func TestLocalMCPRejectsNonLoopbackAndInvalidTransport(t *testing.T) {
	testhome.Temp(t)
	for _, server := range []string{testServer, "http://localhost:8091", "https://127.0.0.1:8091", "http://127.0.0.1", "http://127.0.0.1:8091/other"} {
		if _, err := ConfigureMCP("claude", "/opt/sectile-agent", server, "", "http", true); err == nil {
			t.Fatalf("accepted %s", server)
		}
	}
	if _, err := ConfigureMCP("claude", "/opt/sectile-agent", testServer, testKey, "invalid", false); err == nil {
		t.Fatal("accepted invalid transport")
	}
}
