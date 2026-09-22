package agentconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"tasks/internal/testhome"
	"testing"
)

func claudeConfig() Config {
	return Config{SchemaVersion: Version, AIProvider: "claude",
		Skills: []Skill{{ID: "clarify", Directory: "clarify-issue", Command: "/clarify-issue", Content: "skill", CommandContent: "command"}}}
}

func readSettings(t *testing.T, home string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(home, ".claude/settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	settings := map[string]any{}
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatalf("the settings Sectile wrote are not valid JSON: %v\n%s", err, raw)
	}
	return settings
}

// hookCommands lists the commands registered for one event, whoever owns them.
func hookCommands(t *testing.T, settings map[string]any, event string) []string {
	t.Helper()
	hooks, _ := settings["hooks"].(map[string]any)
	groups, _ := hooks[event].([]any)
	var commands []string
	for _, group := range groups {
		values, _ := group.(map[string]any)
		entries, _ := values["hooks"].([]any)
		for _, raw := range entries {
			entry, _ := raw.(map[string]any)
			if command, _ := entry["command"].(string); command != "" {
				commands = append(commands, command)
			}
		}
	}
	return commands
}

func TestHooksAreInstalledExecutableAndRegistered(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	testhome.Set(t, home)
	if _, err := Scaffold(root, claudeConfig()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, claudeHookDir, claudeHookFile)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("the hook was not installed: %v", err)
	}
	// Claude Code runs the file; a hook it cannot execute is a hook that
	// silently never fires.
	if info.Mode().Perm()&0100 == 0 {
		t.Fatalf("the hook is not executable: %v", info.Mode())
	}
	settings := readSettings(t, home)
	for _, event := range claudeHookEvents {
		commands := hookCommands(t, settings, event)
		if len(commands) != 1 || commands[0] != path {
			t.Fatalf("%s is registered as %v, expected %s", event, commands, path)
		}
	}
}

// The first release installed one script per event, on Notification and Stop.
// Upgrading must leave neither the files nor their registrations behind: a
// registration pointing at a removed script is a hook error on every turn.
func TestLegacyHookScriptsAreRetiredWithTheirRegistrations(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	testhome.Set(t, home)
	if err := os.MkdirAll(filepath.Join(home, claudeHookDir), 0755); err != nil {
		t.Fatal(err)
	}
	manifest := map[string]string{}
	for _, legacy := range retiredClaudeHookFiles {
		content := []byte("#!/bin/sh\n# legacy " + legacy + "\n")
		if err := os.WriteFile(filepath.Join(home, claudeHookDir, legacy), content, 0700); err != nil {
			t.Fatal(err)
		}
		manifest[filepath.Join(claudeHookDir, legacy)] = digest(content)
	}
	manifestPath, err := ManifestPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0755); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(manifest)
	if err := os.WriteFile(manifestPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	// The registrations point at a former home directory: ownership is read
	// from the file name, so they are still recognised as Sectile's.
	existing := `{
	  "hooks": {
	    "Notification": [
	      {"matcher": "", "hooks": [{"type": "command", "command": "/opt/mine/ping.sh"}]},
	      {"matcher": "", "hooks": [{"type": "command", "command": "/Users/old/.claude/hooks/sectile-notification.sh"}]}
	    ],
	    "Stop": [{"matcher": "", "hooks": [{"type": "command", "command": "/Users/old/.claude/hooks/sectile-stop.sh"}]}]
	  }
	}`
	if err := os.WriteFile(filepath.Join(home, ".claude/settings.json"), []byte(existing), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := Scaffold(root, claudeConfig()); err != nil {
		t.Fatal(err)
	}
	for _, legacy := range retiredClaudeHookFiles {
		if _, err := os.Stat(filepath.Join(home, claudeHookDir, legacy)); !os.IsNotExist(err) {
			t.Fatalf("the legacy script %s survived the upgrade: %v", legacy, err)
		}
	}
	settings := readSettings(t, home)
	current := filepath.Join(home, claudeHookDir, claudeHookFile)
	if got := hookCommands(t, settings, "Notification"); len(got) != 2 || got[0] != "/opt/mine/ping.sh" || got[1] != current {
		t.Fatalf("Notification after the upgrade: %v", got)
	}
	for _, event := range claudeHookEvents {
		if got := hookCommands(t, settings, event); len(got) == 0 || got[len(got)-1] != current {
			t.Fatalf("%s after the upgrade: %v", event, got)
		}
		for _, command := range hookCommands(t, settings, event) {
			for _, legacy := range retiredClaudeHookFiles {
				if strings.HasSuffix(command, legacy) {
					t.Fatalf("%s still registers the retired %s", event, legacy)
				}
			}
		}
	}
}

func TestHookRegistrationPreservesEverythingElse(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	testhome.Set(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0755); err != nil {
		t.Fatal(err)
	}
	existing := `{
	  "model": "opus",
	  "permissions": {"allow": ["Bash(git status)"]},
	  "hooks": {
	    "Notification": [{"matcher": "", "hooks": [{"type": "command", "command": "/opt/mine/ping.sh"}]}],
	    "PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "/opt/mine/audit.sh"}]}]
	  }
	}`
	if err := os.WriteFile(filepath.Join(home, ".claude/settings.json"), []byte(existing), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Scaffold(root, claudeConfig()); err != nil {
		t.Fatal(err)
	}
	settings := readSettings(t, home)
	if settings["model"] != "opus" {
		t.Fatalf("an unrelated key was lost: %+v", settings)
	}
	if _, ok := settings["permissions"]; !ok {
		t.Fatalf("the permissions the user configured were lost: %+v", settings)
	}
	// The user's own hooks share their events with Sectile's: both must still
	// be there, the third-party one first, with its matcher intact.
	for event, own := range map[string]string{"Notification": "/opt/mine/ping.sh", "PreToolUse": "/opt/mine/audit.sh"} {
		got := hookCommands(t, settings, event)
		if len(got) != 2 || got[0] != own || !strings.HasSuffix(got[1], "/"+claudeHookFile) {
			t.Fatalf("%s hooks after the merge: %v", event, got)
		}
	}
	hooks, _ := settings["hooks"].(map[string]any)
	groups, _ := hooks["PreToolUse"].([]any)
	if first, _ := groups[0].(map[string]any); first["matcher"] != "Bash" {
		t.Fatalf("the third-party matcher was rewritten: %+v", first)
	}
}

func TestHookRegistrationIsIdempotent(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	testhome.Set(t, home)
	if _, err := Scaffold(root, claudeConfig()); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(home, ".claude/settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Scaffold(root, claudeConfig()); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(home, ".claude/settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("a second run changed the settings:\n%s\n---\n%s", first, second)
	}
	if got := hookCommands(t, readSettings(t, home), "Stop"); len(got) != 1 {
		t.Fatalf("the Stop hook was registered %d times", len(got))
	}
}

func TestUnparseableSettingsAreLeftUntouchedAndReported(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	testhome.Set(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0755); err != nil {
		t.Fatal(err)
	}
	broken := "{ this is not JSON"
	path := filepath.Join(home, ".claude/settings.json")
	if err := os.WriteFile(path, []byte(broken), 0600); err != nil {
		t.Fatal(err)
	}
	reports, err := Scaffold(root, claudeConfig())
	if err != nil {
		t.Fatalf("an unreadable settings file aborted the whole setup: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != broken {
		t.Fatalf("the file was rewritten: %q %v", raw, err)
	}
	if !strings.Contains(strings.Join(reports, "\n"), "settings") {
		t.Fatalf("the failure was not reported: %v", reports)
	}
	// The rest of the setup still happened.
	if _, err := os.Stat(filepath.Join(home, ".claude/skills/clarify-issue/SKILL.md")); err != nil {
		t.Fatalf("the skills were not installed: %v", err)
	}
}

func TestNoHookIsWrittenForAnotherProvider(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	testhome.Set(t, home)
	config := claudeConfig()
	config.AIProvider = "codex"
	if _, err := Scaffold(root, config); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude/hooks")); !os.IsNotExist(err) {
		t.Fatalf("hooks were installed for a provider that has none: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude/settings.json")); !os.IsNotExist(err) {
		t.Fatalf("Claude settings were written while setting up another provider: %v", err)
	}
}
