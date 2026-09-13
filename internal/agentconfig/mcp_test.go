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

func TestBootstrapMCPPreservesConfigAndRefreshesGateway(t *testing.T) {
	for _, provider := range []string{"codex", "claude", "agy", "gemini", "cursor", "vibe"} {
		t.Run(provider, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("HOME", t.TempDir())
			path, err := BootstrapMCP(root, provider, "/opt/taskflow", "http://127.0.0.1:8091")
			if err != nil {
				t.Fatal(err)
			}
			read := func() map[string]any {
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
			data := read()
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
			if _, err = BootstrapMCP(root, provider, "/opt/new taskflow", "http://127.0.0.1:45123"); err != nil {
				t.Fatal(err)
			}
			result := read()
			if result["unrelated_setting"] != "keep" {
				t.Fatal("unrelated settings lost")
			}
			raw, _ = json.Marshal(result)
			wantValues := []string{"other-server", "/opt/new taskflow"}
			if provider == "agy" {
				if path != filepath.Join(os.Getenv("HOME"), ".gemini/config/mcp_config.json") || strings.Contains(string(raw), "45123") {
					t.Fatalf("agy must use its user registry without a process-specific gateway: %s", raw)
				}
			} else {
				wantValues = append(wantValues, "45123")
			}
			for _, want := range wantValues {
				if !strings.Contains(string(raw), want) {
					t.Fatalf("missing %q in %s", want, raw)
				}
			}
			if strings.Contains(string(raw), "8091") || strings.Contains(string(raw), "TOKEN") {
				t.Fatalf("stale gateway or credential in %s", raw)
			}
		})
	}
}

func TestBootstrapMCPRejectsInvalidConfigWithoutOverwriting(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".mcp.json")
	raw := []byte("invalid json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := BootstrapMCP(root, "claude", "/bin/taskflow", "http://127.0.0.1:8091"); err == nil {
		t.Fatal("invalid configuration accepted")
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(raw) {
		t.Fatal("existing config damaged")
	}
	if _, err := BootstrapMCP(root, "custom", "/bin/taskflow", "http://127.0.0.1:8091"); err == nil {
		t.Fatal("unsupported provider silently accepted")
	}
	if _, err := BootstrapMCP(root, "codex", "/bin/taskflow", ""); err == nil {
		t.Fatal("missing gateway accepted")
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
	if _, err := BootstrapMCP(root, "agy", "/usr/bin/true", "http://127.0.0.1:8091"); err != nil {
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
