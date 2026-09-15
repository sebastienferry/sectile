package agentconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEffectiveProviderResolvesCustomTemplates(t *testing.T) {
	for _, tc := range []struct{ name, provider, template, want string }{
		{"named provider", "claude", "", "claude"},
		{"legacy empty provider", "", "", "agy"},
		{"custom wrapping a known cli", "custom", "/usr/local/bin/claude -p {prompt}", "claude"},
		{"custom quoted path", "custom", `"/opt/my tools/codex" {prompt}`, "codex"},
		{"custom without a command", "custom", "", "custom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := EffectiveProvider(tc.provider, tc.template); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveLocationsCoversEverySupportedProvider(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, tc := range []struct {
		provider, skills, mcp string
	}{
		{"claude", ".claude/skills", ".claude.json"},
		{"agy", ".agy/skills", ".gemini/config/mcp_config.json"},
		{"codex", ".codex/skills", ".codex/config.toml"},
		{"gemini", "", ".gemini/settings.json"},
		{"cursor", "", ".cursor/mcp.json"},
		{"vibe", "", ".vibe/config.toml"},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			loc, err := ResolveLocations(tc.provider)
			if err != nil {
				t.Fatal(err)
			}
			if loc.Home != home || loc.SkillDir != tc.skills || loc.MCPFile != tc.mcp {
				t.Fatalf("got %+v", loc)
			}
			if loc.InstallsSkills() != (tc.skills != "") {
				t.Fatal("skill convention mismatch")
			}
		})
	}
	if _, err := ResolveLocations("unknown"); err == nil {
		t.Fatal("unknown provider accepted")
	}
}

func TestScaffoldInstallsOnlyForTheSelectedProvider(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	c := Config{SchemaVersion: Version, AIProvider: "claude", Skills: []Skill{{ID: "clarify", Directory: "clarify-issue", Command: "/clarify-issue", Content: "skill", CommandContent: "command"}}}
	if _, err := Scaffold(root, c); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(home, ".claude/skills/clarify-issue/SKILL.md"))
	if err != nil || string(raw) != "skill" {
		t.Fatalf("skill not installed for the selected provider: %s %v", raw, err)
	}
	raw, err = os.ReadFile(filepath.Join(home, ".claude/commands/clarify-issue.md"))
	if err != nil || string(raw) != "command" {
		t.Fatalf("command not installed: %s %v", raw, err)
	}
	for _, other := range []string{".agy", ".gemini", ".agents", ".skills"} {
		if _, err := os.Stat(filepath.Join(home, other)); !os.IsNotExist(err) {
			t.Fatalf("%s written for an unselected provider", other)
		}
	}

	// Switching provider retires the previous installation and installs the new one.
	c.AIProvider = "agy"
	if _, err := Scaffold(root, c); err != nil {
		t.Fatal(err)
	}
	for _, retired := range []string{".claude/skills/clarify-issue/SKILL.md", ".claude/commands/clarify-issue.md"} {
		if _, err := os.Stat(filepath.Join(home, retired)); !os.IsNotExist(err) {
			t.Fatalf("%s survived the provider switch", retired)
		}
	}
	if raw, err := os.ReadFile(filepath.Join(home, ".agy/skills/clarify-issue/SKILL.md")); err != nil || string(raw) != "skill" {
		t.Fatalf("new provider not installed: %s %v", raw, err)
	}
}

func TestScaffoldInstallsNothingWithoutASkillConvention(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	c := Config{SchemaVersion: Version, AIProvider: "gemini", Skills: []Skill{{ID: "clarify", Directory: "clarify-issue", Content: "skill"}}}
	if _, err := Scaffold(root, c); err != nil {
		t.Fatalf("a provider without a skill convention must still prepare: %v", err)
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != ".config" {
			t.Fatalf("unexpected %s written for a provider without skills", entry.Name())
		}
	}
}

func TestScaffoldRejectsUnsupportedProvider(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	c := Config{SchemaVersion: Version, AIProvider: "custom", AICommandTemplate: "my-own-cli {prompt}"}
	if _, err := Scaffold(root, c); err == nil {
		t.Fatal("unsupported provider silently accepted")
	}
}

func TestSetupProvidersAlwaysIncludeTheRunningAgent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	got, err := SetupProviders(Config{AIProvider: "claude", SetupProviders: []string{"codex", "claude", " AGY ", ""}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"claude", "codex", "agy"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if _, err := SetupProviders(Config{AIProvider: "claude", SetupProviders: []string{"unknown"}}); err == nil {
		t.Fatal("unsupported additional provider accepted")
	}
}

func TestScaffoldInstallsForEveryRequestedAgent(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	c := Config{SchemaVersion: Version, AIProvider: "claude", SetupProviders: []string{"codex", "agy"},
		Skills: []Skill{{ID: "clarify", Directory: "clarify-issue", Command: "/clarify-issue", Content: "skill", CommandContent: "command"}}}
	if _, err := Scaffold(root, c); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{".claude/skills/clarify-issue/SKILL.md", ".codex/skills/clarify-issue/SKILL.md", ".agy/skills/clarify-issue/SKILL.md"} {
		raw, err := os.ReadFile(filepath.Join(home, path))
		if err != nil || string(raw) != "skill" {
			t.Fatalf("%s: %s %v", path, raw, err)
		}
	}
	// Unchecking an agent retires its installation.
	c.SetupProviders = []string{"agy"}
	if _, err := Scaffold(root, c); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex/skills/clarify-issue/SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("unchecked agent kept its installation")
	}
	if _, err := os.Stat(filepath.Join(home, ".agy/skills/clarify-issue/SKILL.md")); err != nil {
		t.Fatal("checked agent lost its installation")
	}
}
