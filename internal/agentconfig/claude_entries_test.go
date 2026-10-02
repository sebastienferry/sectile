package agentconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"tasks/internal/testhome"
)

func writeClaudeFixture(t *testing.T, home, content string) string {
	t.Helper()
	path := filepath.Join(home, ".claude.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadClaudeEntries(t *testing.T) {
	home := testhome.Temp(t)
	entries, err := ReadClaudeEntries()
	if err != nil || entries != nil {
		t.Fatalf("missing file: %v %v", entries, err)
	}
	writeClaudeFixture(t, home, `{
		"mcpServers": {"sectile": {"type": "http", "url": "https://sectile.example.test/mcp", "headers": {"Authorization": "Bearer user-key"}}},
		"projects": {
			"/work/app": {"mcpServers": {"sectile": {"type": "http", "url": "https://old.example.test/mcp", "headers": {"Authorization": "Bearer old-key"}}}},
			"/work/bridge": {"mcpServers": {"sectile": {"command": "/opt/sectile-agent", "args": ["mcp"], "env": {"SECTILE_AGENT_TOKEN": "stdio-key"}}}},
			"/work/none": {"allowedTools": []},
			"/work/odd": "not an object",
			"/work/servers": {"mcpServers": "not an object"}
		}
	}`)
	entries, err = ReadClaudeEntries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("entries: %+v", entries)
	}
	user, app, bridge := entries[0], entries[1], entries[2]
	if user.Scope != "user" || user.Project != "" || user.URL != "https://sectile.example.test/mcp" || !user.KeyMatches("user-key") || user.KeyMatches("old-key") {
		t.Fatalf("user entry: %+v", user)
	}
	if app.Scope != "project" || app.Project != "/work/app" || app.URL != "https://old.example.test/mcp" || !app.KeyMatches("old-key") || app.KeyMatches("user-key") {
		t.Fatalf("project entry: %+v", app)
	}
	if bridge.Scope != "project" || bridge.Project != "/work/bridge" || bridge.URL != "" || !bridge.KeyMatches("stdio-key") {
		t.Fatalf("stdio entry: %+v", bridge)
	}
	if (ClaudeEntry{}).KeyMatches("") || (ClaudeEntry{}).KeyMatches("user-key") || user.KeyMatches("") {
		t.Fatal("an empty key matched")
	}

	// Neither a top-level mcpServers nor a projects map is required.
	writeClaudeFixture(t, home, `{"projects": {"/work/app": {"mcpServers": {"sectile": {"type": "http", "url": "https://sectile.example.test/mcp", "headers": {"Authorization": "Bearer user-key"}}}}}}`)
	if entries, err = ReadClaudeEntries(); err != nil || len(entries) != 1 || entries[0].Scope != "project" {
		t.Fatalf("without user entry: %+v %v", entries, err)
	}
	writeClaudeFixture(t, home, `{"projects": "not an object"}`)
	if entries, err = ReadClaudeEntries(); err != nil || len(entries) != 0 {
		t.Fatalf("non-object projects: %+v %v", entries, err)
	}
	writeClaudeFixture(t, home, `invalid json`)
	if _, err = ReadClaudeEntries(); err == nil {
		t.Fatal("invalid JSON accepted")
	}
}

func TestRemoveClaudeProjectEntries(t *testing.T) {
	home := testhome.Temp(t)
	path := writeClaudeFixture(t, home, `{
		"mcpServers": {"sectile": {"type": "http", "url": "https://sectile.example.test/mcp", "headers": {"Authorization": "Bearer user-key"}}},
		"projects": {
			"/work/app": {"allowedTools": ["mcp__sectile__get_task"], "mcpServers": {"sectile": {"type": "http", "url": "https://sectile.example.test/mcp", "headers": {"Authorization": "Bearer old-key"}}, "other": {"command": "other"}}},
			"/work/keep": {"mcpServers": {"sectile": {"type": "http", "url": "https://sectile.example.test/mcp", "headers": {"Authorization": "Bearer old-key"}}}}
		},
		"unrelated": "keep"
	}`)
	written, err := RemoveClaudeProjectEntries([]string{"/work/app", "/work/missing"})
	if err != nil || written != path {
		t.Fatalf("%s %v", written, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data["unrelated"] != "keep" || data["mcpServers"].(map[string]any)["sectile"] == nil {
		t.Fatalf("user scope changed: %s", raw)
	}
	projects := data["projects"].(map[string]any)
	app := projects["/work/app"].(map[string]any)
	servers := app["mcpServers"].(map[string]any)
	if servers["sectile"] != nil || servers["other"] == nil || len(app["allowedTools"].([]any)) != 1 {
		t.Fatalf("named project: %s", raw)
	}
	if projects["/work/keep"].(map[string]any)["mcpServers"].(map[string]any)["sectile"] == nil {
		t.Fatalf("other project lost its entry: %s", raw)
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("mode: %v %v", info.Mode(), err)
		}
	}
}
