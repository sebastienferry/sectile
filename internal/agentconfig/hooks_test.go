package agentconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	t.Setenv("HOME", home)
	if _, err := Scaffold(root, claudeConfig()); err != nil {
		t.Fatal(err)
	}
	for _, hook := range claudeHooks {
		path := filepath.Join(home, claudeHookDir, hook.File)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("hook %s was not installed: %v", hook.File, err)
		}
		// Claude Code runs the file; a hook it cannot execute is a hook that
		// silently never fires.
		if info.Mode().Perm()&0100 == 0 {
			t.Fatalf("hook %s is not executable: %v", hook.File, info.Mode())
		}
		commands := hookCommands(t, readSettings(t, home), hook.Event)
		if len(commands) != 1 || commands[0] != path {
			t.Fatalf("%s is registered as %v, expected %s", hook.Event, commands, path)
		}
	}
}

func TestHookRegistrationPreservesEverythingElse(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
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
	if got := hookCommands(t, settings, "PreToolUse"); len(got) != 1 || got[0] != "/opt/mine/audit.sh" {
		t.Fatalf("a third-party hook on another event was touched: %v", got)
	}
	// The user's own Notification hook shares the event with Sectile's: both must
	// still be there, the third-party one first.
	got := hookCommands(t, settings, "Notification")
	if len(got) != 2 || got[0] != "/opt/mine/ping.sh" || !strings.HasSuffix(got[1], "sectile-notification.sh") {
		t.Fatalf("Notification hooks after the merge: %v", got)
	}
}

func TestHookRegistrationIsIdempotent(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
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
	t.Setenv("HOME", home)
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
	t.Setenv("HOME", home)
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
