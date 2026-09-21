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

// setHome points os.UserHomeDir at a temporary directory on every platform: it
// reads HOME on POSIX and USERPROFILE on Windows, and a test that sets only the
// first installs into the developer's real home directory on the second.
func setHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
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

func writeSettings(t *testing.T, home, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".claude/settings.json")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
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

// installRetiredHooks seeds what an earlier release left on a workstation: the
// scripts under ~/.claude/hooks, recorded in the manifest with their digest.
func installRetiredHooks(t *testing.T, home string, names ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(home, claudeHookDir), 0755); err != nil {
		t.Fatal(err)
	}
	manifest := map[string]string{}
	for _, name := range names {
		content := []byte("#!/bin/sh\n# managed " + name + "\n")
		if err := os.WriteFile(filepath.Join(home, claudeHookDir, name), content, 0700); err != nil {
			t.Fatal(err)
		}
		manifest[filepath.Join(claudeHookDir, name)] = digest(content)
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
}

// Sectile no longer installs a Claude Code hook: setting up the provider leaves
// ~/.claude/hooks and ~/.claude/settings.json alone, and still installs the skills.
func TestNoHookIsInstalledOrRegistered(t *testing.T) {
	root, home := t.TempDir(), setHome(t)
	if _, err := Scaffold(root, claudeConfig()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, claudeHookDir)); !os.IsNotExist(err) {
		t.Fatalf("a hook directory was created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude/settings.json")); !os.IsNotExist(err) {
		t.Fatalf("Claude settings were written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude/skills/clarify-issue/SKILL.md")); err != nil {
		t.Fatalf("the skills were not installed: %v", err)
	}
}

// Every release that installed a hook is upgraded from: the scripts go with
// their registrations, whatever home directory or binary they pointed at, and
// nothing of the user's is touched.
func TestInstalledHooksAreRetiredWithTheirRegistrations(t *testing.T) {
	root, home := t.TempDir(), setHome(t)
	installRetiredHooks(t, home, retiredClaudeHookFiles...)
	writeSettings(t, home, `{
	  "model": "opus",
	  "permissions": {"allow": ["Bash(git status)"]},
	  "hooks": {
	    "Notification": [
	      {"matcher": "", "hooks": [{"type": "command", "command": "/opt/mine/ping.sh"}]},
	      {"matcher": "", "hooks": [{"type": "command", "command": "/Users/old/.claude/hooks/sectile-notification.sh"}]},
	      {"matcher": "", "hooks": [{"type": "command", "command": "/Users/old/.claude/hooks/sectile-hook.sh"}]}
	    ],
	    "Stop": [{"matcher": "", "hooks": [{"type": "command", "command": "/Users/old/.claude/hooks/sectile-stop.sh"}]}],
	    "PreToolUse": [
	      {"matcher": "Bash", "hooks": [{"type": "command", "command": "/opt/mine/audit.sh"}]},
	      {"matcher": "", "hooks": [{"type": "command", "command": "\"C:\\Program Files\\Sectile\\sectile-agent.exe\" sectile-hook"}]}
	    ],
	    "PostToolUse": [{"matcher": "", "hooks": [{"type": "command", "command": "'/Applications/Sectile.app/Contents/Resources/bin/sectile-agent' sectile-hook"}]}]
	  }
	}`)

	if _, err := Scaffold(root, claudeConfig()); err != nil {
		t.Fatal(err)
	}
	for _, name := range retiredClaudeHookFiles {
		if _, err := os.Stat(filepath.Join(home, claudeHookDir, name)); !os.IsNotExist(err) {
			t.Fatalf("the script %s survived the upgrade: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(home, claudeHookDir)); !os.IsNotExist(err) {
		t.Fatalf("the emptied hook directory was left behind: %v", err)
	}
	settings := readSettings(t, home)
	if settings["model"] != "opus" {
		t.Fatalf("an unrelated key was lost: %+v", settings)
	}
	if _, ok := settings["permissions"]; !ok {
		t.Fatalf("the permissions the user configured were lost: %+v", settings)
	}
	if got := hookCommands(t, settings, "Notification"); len(got) != 1 || got[0] != "/opt/mine/ping.sh" {
		t.Fatalf("Notification after the upgrade: %v", got)
	}
	if got := hookCommands(t, settings, "PreToolUse"); len(got) != 1 || got[0] != "/opt/mine/audit.sh" {
		t.Fatalf("PreToolUse after the upgrade: %v", got)
	}
	hooks, _ := settings["hooks"].(map[string]any)
	for _, event := range []string{"Stop", "PostToolUse"} {
		if entry, present := hooks[event]; present {
			t.Fatalf("%s kept an entry once Sectile's was removed: %v", event, entry)
		}
	}
	groups, _ := hooks["PreToolUse"].([]any)
	if first, _ := groups[0].(map[string]any); first["matcher"] != "Bash" {
		t.Fatalf("the third-party matcher was rewritten: %+v", first)
	}
}

// A settings file Sectile never wrote to is not rewritten: not even its
// formatting changes, and a second run changes nothing either.
func TestForeignSettingsAreLeftByteForByte(t *testing.T) {
	root, home := t.TempDir(), setHome(t)
	original := "{\n  \"model\": \"opus\",\n  \"hooks\": {\"Notification\": [{\"matcher\": \"\", \"hooks\": [{\"type\": \"command\", \"command\": \"/opt/mine/ping.sh\"}]}]}\n}\n"
	path := writeSettings(t, home, original)
	for range 2 {
		if _, err := Scaffold(root, claudeConfig()); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(path)
		if err != nil || string(raw) != original {
			t.Fatalf("the file was rewritten: %q %v", raw, err)
		}
	}
}

// Once the Sectile entries are gone the file is not written again, and an
// event or a hooks object emptied by the cleanup is not left behind.
func TestHookRetirementIsIdempotent(t *testing.T) {
	root, home := t.TempDir(), setHome(t)
	path := writeSettings(t, home, `{"model": "opus", "hooks": {"Stop": [{"matcher": "", "hooks": [{"type": "command", "command": "/Users/old/.claude/hooks/sectile-hook.sh"}]}]}}`)
	if _, err := Scaffold(root, claudeConfig()); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, present := readSettings(t, home)["hooks"]; present {
		t.Fatalf("an empty hooks object was left behind: %s", first)
	}
	if _, err := Scaffold(root, claudeConfig()); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("a second run changed the settings:\n%s\n---\n%s", first, second)
	}
}

func TestUnparseableSettingsAreLeftUntouchedAndReported(t *testing.T) {
	root, home := t.TempDir(), setHome(t)
	broken := "{ this is not JSON"
	path := writeSettings(t, home, broken)
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
	if _, err := os.Stat(filepath.Join(home, ".claude/skills/clarify-issue/SKILL.md")); err != nil {
		t.Fatalf("the skills were not installed: %v", err)
	}
}

// A hook script the user edited is theirs: the file survives, while the
// registration pointing at it is still Sectile's and goes.
func TestAnEditedHookScriptSurvivesButLosesItsRegistration(t *testing.T) {
	root, home := t.TempDir(), setHome(t)
	installRetiredHooks(t, home, "sectile-hook.sh")
	path := filepath.Join(home, claudeHookDir, "sectile-hook.sh")
	edited := []byte("#!/bin/sh\n# edited by hand\n")
	if err := os.WriteFile(path, edited, 0700); err != nil {
		t.Fatal(err)
	}
	writeSettings(t, home, `{"hooks": {"Stop": [{"matcher": "", "hooks": [{"type": "command", "command": "`+filepath.ToSlash(path)+`"}]}]}}`)

	if _, err := Scaffold(root, claudeConfig()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != string(edited) {
		t.Fatalf("the edited script did not survive: %q %v", raw, err)
	}
	if got := hookCommands(t, readSettings(t, home), "Stop"); len(got) != 0 {
		t.Fatalf("the registration of the edited script survived: %v", got)
	}
}

// Setting up another provider never creates Claude settings. The cleanup still
// runs on such a workstation, so a Claude Code session that is still used by
// hand is not left with a registration failing on every turn.
func TestAnotherProviderCreatesNoClaudeSettingsButStillRetiresHooks(t *testing.T) {
	root, home := t.TempDir(), setHome(t)
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

	writeSettings(t, home, `{"hooks": {"Stop": [{"matcher": "", "hooks": [{"type": "command", "command": "/Users/old/.claude/hooks/sectile-hook.sh"}]}]}}`)
	if _, err := Scaffold(root, config); err != nil {
		t.Fatal(err)
	}
	if got := hookCommands(t, readSettings(t, home), "Stop"); len(got) != 0 {
		t.Fatalf("the registration survived a setup for another provider: %v", got)
	}
}
