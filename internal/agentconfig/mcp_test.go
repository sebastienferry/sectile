package agentconfig

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

const (
	testServer = "http://sectile.example.test:8090"
	testKey    = "sectile_0123456789abcdef"
)

func readRegistration(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data := map[string]any{}
	if strings.HasSuffix(path, "toml") {
		err = toml.Unmarshal(raw, &data)
	} else {
		err = json.Unmarshal(raw, &data)
	}
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// The registration addresses the server with the workstation key, refreshes
// both when they change, and leaves everything else in the file alone.
func TestBootstrapMCPPreservesConfigAndRefreshesServerAndKey(t *testing.T) {
	for _, provider := range []string{"codex", "claude", "agy", "gemini", "cursor", "vibe"} {
		t.Run(provider, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			path, err := BootstrapMCP(provider, "/opt/sectile", testServer, testKey)
			if err != nil {
				t.Fatal(err)
			}
			data := readRegistration(t, path)
			data["unrelated_setting"] = "keep"
			key := "mcpServers"
			if provider == "codex" || provider == "vibe" {
				key = "mcp_servers"
			}
			if provider == "vibe" {
				data[key] = append(data[key].([]any), map[string]any{"name": "other", "transport": "stdio", "command": "other-server"})
			} else {
				data[key].(map[string]any)["other"] = map[string]any{"command": "other-server"}
			}
			var raw []byte
			if strings.HasSuffix(path, "toml") {
				raw, err = toml.Marshal(data)
			} else {
				raw, err = json.Marshal(data)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err = BootstrapMCP(provider, "/opt/new sectile", "https://moved.example.test/", "sectile_rotated"); err != nil {
				t.Fatal(err)
			}
			result := readRegistration(t, path)
			if result["unrelated_setting"] != "keep" {
				t.Fatal("unrelated settings lost")
			}
			raw, _ = json.Marshal(result)
			// HTTP providers address /mcp; stdio providers pass the bare server
			// URL to the bridge. Both must carry the new server and key.
			for _, want := range []string{"other-server", "https://moved.example.test", "sectile_rotated"} {
				if !strings.Contains(string(raw), want) {
					t.Fatalf("missing %q in %s", want, raw)
				}
			}
			for _, stale := range []string{"sectile.example.test", testKey, "8091"} {
				if strings.Contains(string(raw), stale) {
					t.Fatalf("stale server or credential %q in %s", stale, raw)
				}
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if perm := info.Mode().Perm(); perm&0o077 != 0 {
				t.Fatalf("registration file readable by others: %o", perm)
			}
		})
	}
}

// Providers that can speak Streamable HTTP are pointed at the server directly;
// the others run the stdio bridge against that same server with the key in its
// environment. Neither mentions a local gateway.
func TestBootstrapMCPTransportPerProvider(t *testing.T) {
	for _, provider := range []string{"codex", "claude", "agy", "gemini", "cursor", "vibe"} {
		t.Run(provider, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			path, err := BootstrapMCP(provider, "/opt/sectile", testServer+"/", testKey)
			if err != nil {
				t.Fatal(err)
			}
			data := readRegistration(t, path)
			var entry map[string]any
			switch provider {
			case "vibe":
				entry = data["mcp_servers"].([]any)[0].(map[string]any)
			case "codex":
				entry = data["mcp_servers"].(map[string]any)["sectile"].(map[string]any)
			default:
				entry = data["mcpServers"].(map[string]any)["sectile"].(map[string]any)
			}
			if UsesHTTPMCP(provider) {
				urlField := map[string]string{"claude": "url", "cursor": "url", "gemini": "httpUrl"}[provider]
				if entry[urlField] != testServer+"/mcp" {
					t.Fatalf("%s = %v, want the server /mcp", urlField, entry[urlField])
				}
				if entry["command"] != nil || entry["args"] != nil {
					t.Fatalf("HTTP registration still runs a command: %v", entry)
				}
				headers, _ := entry["headers"].(map[string]any)
				if headers["Authorization"] != "Bearer "+testKey {
					t.Fatalf("headers = %v", entry["headers"])
				}
				if provider == "claude" && entry["type"] != "http" {
					t.Fatalf("claude registration type = %v", entry["type"])
				}
				return
			}
			if entry["command"] != "/opt/sectile" {
				t.Fatalf("command = %v", entry["command"])
			}
			args, _ := json.Marshal(entry["args"])
			if string(args) != `["mcp","--url","`+testServer+`"]` {
				t.Fatalf("args = %s", args)
			}
			env, _ := entry["env"].(map[string]any)
			if env["SECTILE_AGENT_TOKEN"] != testKey {
				t.Fatalf("env = %v", entry["env"])
			}
			if provider == "vibe" && entry["transport"] != "stdio" {
				t.Fatalf("vibe transport = %v", entry["transport"])
			}
		})
	}
}

func TestBootstrapMCPRejectsInvalidConfigWithoutOverwriting(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	path := filepath.Join(root, ".claude.json")
	raw := []byte("invalid json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := BootstrapMCP("claude", "/bin/sectile", testServer, testKey); err == nil {
		t.Fatal("invalid configuration accepted")
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(raw) {
		t.Fatal("existing config damaged")
	}
	if _, err := BootstrapMCP("custom", "/bin/sectile", testServer, testKey); err == nil {
		t.Fatal("unsupported provider silently accepted")
	}
	if _, err := BootstrapMCP("codex", "/bin/sectile", "", testKey); err == nil {
		t.Fatal("missing server accepted")
	}
	if _, err := BootstrapMCP("codex", "/bin/sectile", "ftp://sectile", testKey); err == nil {
		t.Fatal("non-HTTP server accepted")
	}
	if _, err := BootstrapMCP("codex", "/bin/sectile", testServer, " "); err == nil {
		t.Fatal("missing key accepted")
	}
	if _, err := BootstrapMCP("codex", "sectile", testServer, testKey); err == nil {
		t.Fatal("relative executable accepted")
	}
}

// Exercise the installed provider's reader rather than only our JSON writer.
func TestBootstrapMCPVisibleToAgy(t *testing.T) {
	agy, err := exec.LookPath("agy")
	if err != nil {
		t.Skip("agy is not installed")
	}
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	if _, err := BootstrapMCP("agy", "/usr/bin/true", testServer, testKey); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(agy, "mcp", "list")
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("agy mcp list: %v: %s", err, output)
	}
	if !strings.Contains(string(output), "sectile") || !strings.Contains(string(output), "/usr/bin/true") {
		t.Fatalf("agy did not discover Sectile: %s", output)
	}
}
