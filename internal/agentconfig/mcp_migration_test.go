package agentconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestMCPRegistrationMigration(t *testing.T) {
	for _, provider := range []string{"codex", "claude", "agy", "gemini", "cursor", "vibe"} {
		for _, state := range []string{"fresh", "legacy", "canonical", "both"} {
			t.Run(provider+"/"+state, func(t *testing.T) {
				root := t.TempDir()
				t.Setenv("HOME", t.TempDir())
				path, err := BootstrapMCP(root, provider, "/opt/sectile", "http://127.0.0.1:8091")
				if err != nil {
					t.Fatal(err)
				}
				key := "mcpServers"
				if provider == "codex" || provider == "vibe" {
					key = "mcp_servers"
				}
				policy := "disabled"
				switch provider {
				case "codex":
					policy = "disabled_tools"
				case "gemini":
					policy = "excludeTools"
				case "agy":
					policy = "disabledTools"
				}
				entry := func(legacy bool) map[string]any {
					name := "sectile"
					tool := "finish_run"
					if legacy {
						name = "taskflow"
						tool = "taskflow_finish_run"
					}
					e := map[string]any{"name": name, "command": "/old", "args": []any{"mcp"}, "env": map[string]any{"TOKEN": "secret"}, "startup_timeout_sec": int64(20)}
					if policy == "disabled" {
						e[policy] = true
					} else {
						e[policy] = []any{tool, "other_tool"}
					}
					return e
				}
				other := map[string]any{"name": "other", "command": "/unrelated", "disabled": true}
				data := map[string]any{"unrelated_setting": "keep"}
				if provider == "vibe" {
					entries := []any{other}
					if state == "legacy" || state == "both" {
						entries = append(entries, entry(true))
					}
					if state == "canonical" || state == "both" {
						entries = append(entries, entry(false))
					}
					data[key] = entries
				} else {
					entries := map[string]any{"other": other}
					if state == "legacy" || state == "both" {
						entries["taskflow"] = entry(true)
					}
					if state == "canonical" || state == "both" {
						entries["sectile"] = entry(false)
					}
					data[key] = entries
				}
				writeMCPFixture(t, path, data)
				for i := 0; i < 2; i++ {
					if _, err := BootstrapMCP(root, provider, "/opt/new sectile", "http://127.0.0.1:45123"); err != nil {
						t.Fatal(err)
					}
					result := readMCPFixture(t, path)
					if result["unrelated_setting"] != "keep" {
						t.Fatal("unrelated settings lost")
					}
					var managed map[string]any
					if provider == "vibe" {
						entries := result[key].([]any)
						if len(entries) != 2 || !reflect.DeepEqual(entries[0], other) {
							t.Fatalf("other registrations changed: %v", entries)
						}
						managed = entries[1].(map[string]any)
						if managed["name"] != "sectile" {
							t.Fatal("wrong registration name")
						}
					} else {
						entries := result[key].(map[string]any)
						if len(entries) != 2 || !reflect.DeepEqual(entries["other"], other) {
							t.Fatalf("other registrations changed: %v", entries)
						}
						if _, ok := entries["taskflow"]; ok {
							t.Fatal("legacy registration retained")
						}
						managed = entries["sectile"].(map[string]any)
					}
					if managed["command"] != "/opt/new sectile" {
						t.Fatal("transport not refreshed")
					}
					if state != "fresh" {
						if policy == "disabled" {
							if managed[policy] != true {
								t.Fatal("server restriction lost")
							}
						} else if !reflect.DeepEqual(managed[policy], []any{"finish_run", "other_tool"}) {
							t.Fatalf("tool policy lost: %v", managed)
						}
					}
					raw, _ := os.ReadFile(path)
					if strings.Contains(string(raw), "secret") || strings.Contains(string(raw), "8091") {
						t.Fatal("stale transport or credential persisted")
					}
					if provider == "agy" && strings.Contains(string(raw), "45123") {
						t.Fatal("shared registry contains process gateway")
					}
				}
			})
		}
	}
}

func TestMCPMigrationRejectsUnsafeConfiguration(t *testing.T) {
	for _, provider := range []string{"codex", "claude", "agy", "gemini", "cursor", "vibe"} {
		for _, issue := range []string{"conflict", "missing-policy", "invalid-entry", "unsupported-policy", "external-policy", "duplicate"} {
			if issue == "duplicate" && provider != "vibe" {
				continue
			}
			t.Run(provider+"/"+issue, func(t *testing.T) {
				root := t.TempDir()
				t.Setenv("HOME", t.TempDir())
				path, err := BootstrapMCP(root, provider, "/opt/sectile", "http://127.0.0.1:8091")
				if err != nil {
					t.Fatal(err)
				}
				old := map[string]any{"name": "taskflow", "disabled": true}
				current := map[string]any{"name": "sectile", "disabled": true}
				data := map[string]any{}
				switch issue {
				case "conflict":
					current["disabled"] = false
				case "missing-policy":
					delete(current, "disabled")
				case "unsupported-policy":
					old["customPolicy"] = map[string]any{"deny": "taskflow_finish_run"}
				case "external-policy":
					data["permissions"] = map[string]any{"deny": []any{"mcp__taskflow__taskflow_finish_run"}}
				}
				key := "mcpServers"
				if provider == "codex" || provider == "vibe" {
					key = "mcp_servers"
				}
				if provider == "vibe" {
					list := []any{old, current}
					if issue == "duplicate" {
						list = append(list, old)
					}
					if issue == "invalid-entry" {
						list = append(list, "invalid")
					}
					data[key] = list
				} else {
					entries := map[string]any{"taskflow": old, "sectile": current}
					if issue == "invalid-entry" {
						entries["taskflow"] = "invalid"
					}
					data[key] = entries
				}
				writeMCPFixture(t, path, data)
				before, _ := os.ReadFile(path)
				if _, err := BootstrapMCP(root, provider, "/opt/sectile", "http://127.0.0.1:45123"); err == nil {
					t.Fatal("unsafe configuration accepted")
				}
				after, _ := os.ReadFile(path)
				if string(before) != string(after) {
					t.Fatal("failure changed original bytes")
				}
			})
		}
	}
}

func TestMCPMigrationPreservesExternalPolicyFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	path, err := BootstrapMCP(root, "claude", "/opt/sectile", "http://127.0.0.1:8091")
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	policyPath := filepath.Join(root, ".claude/settings.local.json")
	if err := os.MkdirAll(filepath.Dir(policyPath), 0755); err != nil {
		t.Fatal(err)
	}
	policy := []byte(`{"permissions":{"deny":["mcp__taskflow__taskflow_finish_run"]}}`)
	if err := os.WriteFile(policyPath, policy, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := BootstrapMCP(root, "claude", "/opt/sectile", "http://127.0.0.1:45123"); err == nil || !strings.Contains(err.Error(), policyPath) {
		t.Fatalf("missing actionable error: %v", err)
	}
	after, _ := os.ReadFile(path)
	gotPolicy, _ := os.ReadFile(policyPath)
	if string(before) != string(after) || string(policy) != string(gotPolicy) {
		t.Fatal("external policy failure changed files")
	}
}

func TestMCPToolPolicyReferences(t *testing.T) {
	for _, name := range genericMCPTools {
		got, err := migrateMCPToolReference("taskflow_" + name)
		if err != nil || got != name {
			t.Fatalf("%s: %s %v", name, got, err)
		}
	}
	for _, pattern := range []string{"taskflow_*", "taskflow_finish*", "re:.*run", "*run", "mcp__taskflow__taskflow_finish_run"} {
		if _, err := migrateMCPToolReference(pattern); err == nil {
			t.Fatalf("ambiguous pattern accepted: %s", pattern)
		}
	}
	for _, value := range []any{"taskflow_finish_run", []any{false}} {
		if _, err := preservedMCPFields(map[string]any{"disabled_tools": value}, "codex"); err == nil {
			t.Fatal("malformed tool list accepted")
		}
	}
}

func writeMCPFixture(t *testing.T, path string, data map[string]any) {
	t.Helper()
	var raw []byte
	var err error
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
}
func readMCPFixture(t *testing.T, path string) map[string]any {
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

func TestMCPMigrationReadsExternalTOMLPolicies(t *testing.T) {
	for _, policy := range []string{"[mcp_servers.'taskflow']\ncommand = '/opt/sectile'\n", "invalid = ["} {
		t.Run(policy, func(t *testing.T) {
			root := t.TempDir()
			home := t.TempDir()
			t.Setenv("HOME", home)
			if err := os.MkdirAll(filepath.Join(home, ".codex"), 0755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(home, ".codex/config.toml")
			if err := os.WriteFile(path, []byte(policy), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := BootstrapMCP(root, "codex", "/opt/sectile", "http://127.0.0.1:8091"); err == nil {
				t.Fatal("unsafe external TOML accepted")
			}
			if _, err := os.Stat(filepath.Join(root, ".codex/config.toml")); !os.IsNotExist(err) {
				t.Fatal("registration written before policy reconciliation")
			}
			raw, _ := os.ReadFile(path)
			if string(raw) != policy {
				t.Fatal("external settings changed")
			}
		})
	}
}
