package agentconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"tasks/internal/testhome"
	"testing"
)

func TestSkillInstallsAsASingleFileCarryingItsArgument(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	testhome.Set(t, home)
	c := Config{SchemaVersion: Version, AIProvider: "claude", SetupProviders: []string{"codex"},
		Skills: []Skill{{ID: "clarify", Directory: "clarify-issue", Command: "/clarify-issue",
			Content: "Clarify the ticket.", CommandContent: "Clarify the ticket.\n\n## Ticket\n$ARGUMENTS\n"}}}
	if _, err := Scaffold(root, c); err != nil {
		t.Fatal(err)
	}
	// Claude resolves /clarify-issue from the skill directory alone, so the body it
	// installs has to be the one carrying the ticket reference.
	raw, err := os.ReadFile(filepath.Join(home, ".claude/skills/clarify-issue/SKILL.md"))
	if err != nil || !strings.Contains(string(raw), "$ARGUMENTS") {
		t.Fatalf("Claude skill lost its argument: %s %v", raw, err)
	}
	entries, err := os.ReadDir(filepath.Join(home, ".claude"))
	if err != nil {
		t.Fatal(err)
	}
	// The guard is on command sources: a skill directory is the only place a
	// /command may come from. Hooks and the settings that register them are not
	// command sources and are expected here.
	for _, entry := range entries {
		switch entry.Name() {
		case "skills", "hooks", "settings.json":
		default:
			t.Fatalf("second source for the same command: .claude/%s", entry.Name())
		}
	}
	// Codex does not substitute arguments, so a placeholder would be literal text.
	raw, err = os.ReadFile(filepath.Join(home, ".agents/skills/clarify-issue/SKILL.md"))
	if err != nil || strings.Contains(string(raw), "$ARGUMENTS") {
		t.Fatalf("unsubstituted placeholder installed: %s %v", raw, err)
	}
}

func TestScaffoldRetiresTheEarlierDestinations(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	testhome.Set(t, home)
	c := Config{SchemaVersion: Version, AIProvider: "claude", SetupProviders: []string{"codex", "agy"},
		Skills: []Skill{{ID: "clarify", Directory: "clarify-issue", Command: "/clarify-issue",
			Content: "skill", CommandContent: "command"}}}
	// Seed the layout the previous release installed, recorded as managed.
	earlier := map[string]string{
		".claude/commands/clarify-issue.md":     "command",
		".codex/skills/clarify-issue/SKILL.md":  "skill",
		".agy/skills/clarify-issue/SKILL.md":    "skill",
		".claude/skills/clarify-issue/SKILL.md": "skill",
	}
	manifest := map[string]string{}
	for path, content := range earlier {
		full := filepath.Join(home, path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		manifest[path] = digest([]byte(content))
	}
	path, err := ManifestPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Scaffold(root, c); err != nil {
		t.Fatal(err)
	}
	for _, retired := range []string{".claude/commands/clarify-issue.md", ".codex/skills/clarify-issue/SKILL.md", ".agy/skills/clarify-issue/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(home, retired)); !os.IsNotExist(err) {
			t.Fatalf("%s was not retired", retired)
		}
	}
	for _, installed := range []string{".claude/skills/clarify-issue/SKILL.md", ".agents/skills/clarify-issue/SKILL.md", ".gemini/config/skills/clarify-issue/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(home, installed)); err != nil {
			t.Fatalf("%s was not installed: %v", installed, err)
		}
	}
}
