package agentconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// registeredHookCommand is what every event must carry after a refresh: this
// very binary, quoted for this host's shell, followed by the hook subcommand.
// Under `go test` the executable is the test binary, which is exactly what
// Scaffold registers.
func registeredHookCommand(t *testing.T) string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return claudeHookCommand(executable, runtime.GOOS)
}

// setHome points os.UserHomeDir at a directory the test owns. HOME alone is
// not enough: on Windows os.UserHomeDir reads USERPROFILE, so a test setting
// only HOME there installs into the developer's real home directory and then
// reads an empty temporary one back.
func setHome(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

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

// The hook is a subcommand of the agent binary, not a script: Claude Code
// hands the registered command to the host shell, and on Windows that shell is
// cmd.exe, which resolves a .sh file through its file association instead of
// running it. Nothing is installed under ~/.claude/hooks any more.
func TestEveryEventRegistersTheAgentSubcommand(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	setHome(t, home)
	if _, err := Scaffold(root, claudeConfig()); err != nil {
		t.Fatal(err)
	}
	command := registeredHookCommand(t)
	settings := readSettings(t, home)
	for _, event := range claudeHookEvents {
		commands := hookCommands(t, settings, event)
		if len(commands) != 1 || commands[0] != command {
			t.Fatalf("%s is registered as %v, expected %s", event, commands, command)
		}
	}
	if entries, err := os.ReadDir(filepath.Join(home, claudeHookDir)); err == nil && len(entries) > 0 {
		t.Fatalf("a hook file was installed where the subcommand is the hook: %v", entries)
	}
}

// Claude Code passes the command to the host shell, so an installation under a
// path holding a space has to survive that shell's quoting. The target is a
// parameter rather than runtime.GOOS, which is the only way one host can
// assert the registration written on another.
func TestTheHookCommandIsQuotedForTheTargetShell(t *testing.T) {
	for _, test := range []struct {
		name       string
		executable string
		goos       string
		want       string
	}{
		{
			name:       "windows",
			executable: `C:\Users\me\sectile-agent.exe`,
			goos:       "windows",
			want:       `"C:\Users\me\sectile-agent.exe" sectile-hook`,
		},
		{
			name:       "windows under Program Files",
			executable: `C:\Program Files\Sectile\sectile-agent.exe`,
			goos:       "windows",
			want:       `"C:\Program Files\Sectile\sectile-agent.exe" sectile-hook`,
		},
		{
			name:       "linux",
			executable: "/usr/local/bin/sectile-agent",
			goos:       "linux",
			want:       "'/usr/local/bin/sectile-agent' sectile-hook",
		},
		{
			name:       "darwin under a path with a space",
			executable: "/Applications/Sectile 2.app/Contents/sectile-agent",
			goos:       "darwin",
			want:       "'/Applications/Sectile 2.app/Contents/sectile-agent' sectile-hook",
		},
		{
			name:       "a POSIX path holding a quote",
			executable: "/home/o'brien/sectile-agent",
			goos:       "linux",
			want:       `'/home/o'\''brien/sectile-agent' sectile-hook`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := claudeHookCommand(test.executable, test.goos); got != test.want {
				t.Fatalf("the command for %s is %s, expected %s", test.goos, got, test.want)
			}
			// Whatever the quoting, the registration must still be recognised
			// as Sectile's: that is what replaces it in place on the next
			// refresh instead of leaving a second, dead entry behind.
			if !sectileHookRegistration(test.want) {
				t.Fatalf("Sectile does not recognise its own registration %s", test.want)
			}
		})
	}
}

// Ownership is read from the trailing subcommand, or from the name of a script
// Sectile once installed. Nothing else is Sectile's, and claiming a hook the
// user wrote would delete it on the next refresh.
func TestOwnershipClaimsOnlySectileRegistrations(t *testing.T) {
	for _, command := range []string{
		"/opt/mine/ping.sh",
		"/opt/mine/hook",
		"sectile-hook",
		"/usr/bin/sectile-hook-notifier",
		"'/opt/mine/agent' sectile-hook --dry-run",
		"",
	} {
		if sectileHookRegistration(command) {
			t.Fatalf("Sectile claimed a registration it does not own: %q", command)
		}
	}
	for _, command := range []string{
		"/Users/old/.claude/hooks/sectile-hook.sh",
		"/Users/old/.claude/hooks/sectile-notification.sh",
		"/Users/old/.claude/hooks/sectile-stop.sh",
	} {
		if !sectileHookRegistration(command) {
			t.Fatalf("Sectile no longer recognises the registration it wrote: %q", command)
		}
	}
}

// The first release installed one script per event, the second a single
// sectile-hook.sh, and this one installs no script at all. Upgrading must
// leave neither the files nor their registrations behind: a registration
// pointing at a removed script is a hook error on every turn.
func TestLegacyHookScriptsAreRetiredWithTheirRegistrations(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	setHome(t, home)
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
	    "Stop": [{"matcher": "", "hooks": [{"type": "command", "command": "/Users/old/.claude/hooks/sectile-stop.sh"}]}],
	    "PreToolUse": [{"matcher": "", "hooks": [{"type": "command", "command": "/Users/old/.claude/hooks/sectile-hook.sh"}]}]
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
	current := registeredHookCommand(t)
	if got := hookCommands(t, settings, "Notification"); len(got) != 2 || got[0] != "/opt/mine/ping.sh" || got[1] != current {
		t.Fatalf("Notification after the upgrade: %v", got)
	}
	// The script's own event had exactly one registration before the upgrade
	// and must have exactly one after it, replaced rather than doubled.
	if got := hookCommands(t, settings, "PreToolUse"); len(got) != 1 || got[0] != current {
		t.Fatalf("PreToolUse after the upgrade: %v", got)
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
	setHome(t, home)
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
	command := registeredHookCommand(t)
	for event, own := range map[string]string{"Notification": "/opt/mine/ping.sh", "PreToolUse": "/opt/mine/audit.sh"} {
		got := hookCommands(t, settings, event)
		if len(got) != 2 || got[0] != own || got[1] != command {
			t.Fatalf("%s hooks after the merge: %v", event, got)
		}
	}
	hooks, _ := settings["hooks"].(map[string]any)
	groups, _ := hooks["PreToolUse"].([]any)
	if first, _ := groups[0].(map[string]any); first["matcher"] != "Bash" {
		t.Fatalf("the third-party matcher was rewritten: %+v", first)
	}
}

// A hook script the user edited is theirs: refresh only retires a managed file
// whose content still matches the manifest. The registration is Sectile's all
// the same, and is replaced, so the edited script stops being run rather than
// being run beside the subcommand.
func TestAHookScriptTheUserEditedSurvivesButLosesItsRegistration(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	setHome(t, home)
	if err := os.MkdirAll(filepath.Join(home, claudeHookDir), 0755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(home, claudeHookDir, "sectile-hook.sh")
	mine := []byte("#!/bin/sh\n# mine now\nexit 0\n")
	if err := os.WriteFile(script, mine, 0700); err != nil {
		t.Fatal(err)
	}
	manifestPath, err := ManifestPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0755); err != nil {
		t.Fatal(err)
	}
	// The manifest records the content Sectile installed, not the edited one.
	raw, _ := json.Marshal(map[string]string{
		filepath.Join(claudeHookDir, "sectile-hook.sh"): digest([]byte("#!/bin/sh\n# as installed\n")),
	})
	if err := os.WriteFile(manifestPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	existing := `{"hooks": {"Stop": [{"matcher": "", "hooks": [{"type": "command", "command": "` +
		filepath.ToSlash(script) + `"}]}]}}`
	if err := os.WriteFile(filepath.Join(home, ".claude/settings.json"), []byte(existing), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := Scaffold(root, claudeConfig()); err != nil {
		t.Fatal(err)
	}
	kept, err := os.ReadFile(script)
	if err != nil || string(kept) != string(mine) {
		t.Fatalf("the script the user edited was not left alone: %q %v", kept, err)
	}
	if got := hookCommands(t, readSettings(t, home), "Stop"); len(got) != 1 || got[0] != registeredHookCommand(t) {
		t.Fatalf("Stop still registers the edited script: %v", got)
	}
}

func TestHookRegistrationIsIdempotent(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	setHome(t, home)
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
	setHome(t, home)
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
	setHome(t, home)
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
