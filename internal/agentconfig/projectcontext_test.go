package agentconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedLegacyCheckout reproduces the repository layout written before skills moved
// to the selected provider's user configuration.
func seedLegacyCheckout(t *testing.T, root string, skills map[string]string) {
	t.Helper()
	manifest := map[string]string{}
	for path, content := range skills {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		manifest[path] = digest([]byte(content))
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".taskflow"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".taskflow/agent-manifest.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestScaffoldRetiresLegacyCheckoutFiles(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	seedLegacyCheckout(t, root, map[string]string{
		".agents/skills/code-issue/SKILL.md": "managed",
		".claude/skills/code-issue/SKILL.md": "managed",
		".skills/code-issue/SKILL.md":        "managed",
		".claude/commands/code-issue.md":     "managed command",
		".gemini/skills/code-issue/SKILL.md": "edited by hand",
	})
	// The edited copy diverges from what the manifest recorded for it.
	edited := filepath.Join(root, ".gemini/skills/code-issue/SKILL.md")
	if err := os.WriteFile(edited, []byte("personal edits"), 0600); err != nil {
		t.Fatal(err)
	}
	personal := filepath.Join(root, ".claude/skills/personal/SKILL.md")
	if err := os.MkdirAll(filepath.Dir(personal), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(personal, []byte("never managed"), 0600); err != nil {
		t.Fatal(err)
	}
	c := Config{SchemaVersion: Version, Skills: []Skill{{ID: "implement", Directory: "code-issue", Content: "remote"}}}
	reports, err := Scaffold(root, c)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{".agents/skills/code-issue/SKILL.md", ".claude/skills/code-issue/SKILL.md", ".skills/code-issue/SKILL.md", ".claude/commands/code-issue.md"} {
		if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Fatalf("%s was not retired", path)
		}
	}
	for _, path := range []string{edited, personal} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("personal file removed: %v", err)
		}
	}
	if !strings.Contains(strings.Join(reports, "\n"), "Edited skill kept in the checkout") {
		t.Fatalf("edited copy not reported: %v", reports)
	}
	// Retirement happens once: a second run has nothing left to report or remove.
	if _, err := Scaffold(root, c); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".taskflow/agent-manifest.json"))
	if err != nil || strings.TrimSpace(string(raw)) != "{}" {
		t.Fatalf("checkout manifest not cleared: %s %v", raw, err)
	}
}

func TestScaffoldRemovesManagedContextBlock(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	initial := "# Personal instructions\n\n" + contextStart + "\nmanaged content\n" + contextEnd + "\n## Footer\n"
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Scaffold(root, Config{SchemaVersion: Version}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	if strings.Contains(got, contextStart) || strings.Contains(got, "managed content") {
		t.Fatalf("managed block kept: %s", got)
	}
	if !strings.Contains(got, "# Personal instructions") || !strings.Contains(got, "## Footer") {
		t.Fatalf("personal instructions lost: %s", got)
	}
}

func TestScaffoldRemovesManagedRegistrationFromCheckout(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(root, ".mcp.json")
	registration := `{"mcpServers":{"sectile":{"command":"/opt/sectile"},"other":{"command":"other-server"}}}`
	if err := os.WriteFile(path, []byte(registration), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Scaffold(root, Config{SchemaVersion: Version}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "sectile") || !strings.Contains(string(raw), "other-server") {
		t.Fatalf("unrelated registration lost or managed entry kept: %s", raw)
	}
	// A file holding nothing but the managed entry is removed entirely.
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"taskflow":{"command":"/opt/sectile"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Scaffold(root, Config{SchemaVersion: Version}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("empty registration file kept")
	}
}

func TestScaffoldLeavesUnparsableCheckoutFilesAlone(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(root, ".mcp.json")
	broken := []byte("not json at all")
	if err := os.WriteFile(path, broken, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Scaffold(root, Config{SchemaVersion: Version}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != string(broken) {
		t.Fatal("unparsable file rewritten")
	}
}
