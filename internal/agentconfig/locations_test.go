package agentconfig

import (
	"os"
	"path/filepath"
	"tasks/internal/testhome"
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
	testhome.Set(t, home)
	for _, tc := range []struct {
		provider, skills, mcp string
		substitutes           bool
	}{
		{provider: "claude", skills: ".claude/skills", mcp: ".claude.json", substitutes: true},
		{provider: "agy", skills: ".gemini/config/skills", mcp: ".gemini/config/mcp_config.json"},
		{provider: "codex", skills: ".agents/skills", mcp: ".codex/config.toml"},
		{provider: "gemini", mcp: ".gemini/settings.json"},
		{provider: "cursor", mcp: ".cursor/mcp.json"},
		{provider: "vibe", mcp: ".vibe/config.toml"},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			loc, err := ResolveLocations(tc.provider)
			if err != nil {
				t.Fatal(err)
			}
			if loc.Home != home || loc.SkillDir != tc.skills || loc.MCPFile != tc.mcp || loc.SubstitutesArguments != tc.substitutes {
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
	testhome.Set(t, home)
	c := Config{SchemaVersion: Version, AIProvider: "claude", Skills: []Skill{{ID: "clarify", Directory: "clarify-issue", Command: "/clarify-issue", Content: "skill", CommandContent: "command"}}}
	if _, err := Scaffold(root, c); err != nil {
		t.Fatal(err)
	}
	// Claude substitutes arguments, so it receives the body that carries the ticket.
	raw, err := os.ReadFile(filepath.Join(home, ".claude/skills/clarify-issue/SKILL.md"))
	if err != nil || string(raw) != "command" {
		t.Fatalf("skill not installed for the selected provider: %s %v", raw, err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude/commands")); !os.IsNotExist(err) {
		t.Fatal("the legacy command file must not be installed")
	}
	for _, other := range []string{".gemini", ".agents", ".codex", ".skills"} {
		if _, err := os.Stat(filepath.Join(home, other)); !os.IsNotExist(err) {
			t.Fatalf("%s written for an unselected provider", other)
		}
	}

	// Switching provider retires the previous installation and installs the new one.
	c.AIProvider = "agy"
	if _, err := Scaffold(root, c); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude/skills/clarify-issue/SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("the previous provider's skill survived the switch")
	}
	// Antigravity does not substitute arguments, so it receives the plain body.
	if raw, err := os.ReadFile(filepath.Join(home, ".gemini/config/skills/clarify-issue/SKILL.md")); err != nil || string(raw) != "skill" {
		t.Fatalf("new provider not installed: %s %v", raw, err)
	}
}

func TestScaffoldInstallsNothingWithoutASkillConvention(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	testhome.Set(t, home)
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
	testhome.Set(t, home)
	c := Config{SchemaVersion: Version, AIProvider: "custom", AICommandTemplate: "my-own-cli {prompt}"}
	if _, err := Scaffold(root, c); err == nil {
		t.Fatal("unsupported provider silently accepted")
	}
}

func TestSetupProvidersAlwaysIncludeTheRunningAgent(t *testing.T) {
	testhome.Temp(t)
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
	testhome.Set(t, home)
	c := Config{SchemaVersion: Version, AIProvider: "claude", SetupProviders: []string{"codex", "agy"},
		Skills: []Skill{{ID: "clarify", Directory: "clarify-issue", Command: "/clarify-issue", Content: "skill", CommandContent: "command"}}}
	if _, err := Scaffold(root, c); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		".claude/skills/clarify-issue/SKILL.md":        "command",
		".agents/skills/clarify-issue/SKILL.md":        "skill",
		".gemini/config/skills/clarify-issue/SKILL.md": "skill",
	} {
		raw, err := os.ReadFile(filepath.Join(home, path))
		if err != nil || string(raw) != want {
			t.Fatalf("%s: %s %v", path, raw, err)
		}
	}
	// Unchecking an agent retires its installation.
	c.SetupProviders = []string{"agy"}
	if _, err := Scaffold(root, c); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".agents/skills/clarify-issue/SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("unchecked agent kept its installation")
	}
	if _, err := os.Stat(filepath.Join(home, ".gemini/config/skills/clarify-issue/SKILL.md")); err != nil {
		t.Fatal("checked agent lost its installation")
	}
}
