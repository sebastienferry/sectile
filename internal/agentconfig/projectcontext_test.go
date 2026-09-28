package agentconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"tasks/internal/testhome"
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
	testhome.Set(t, home)
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
	block := func(start, end string) string {
		return start + "\nmanaged content\n" + end + "\n"
	}
	for _, tc := range []struct {
		name    string
		initial string
		want    string
		remove  bool
		failed  bool
	}{
		{"legacy with instructions", "# Personal instructions\n\n" + block(contextStart, contextEnd) + "## Footer\n", "# Personal instructions\n\n## Footer\n", false, false},
		{"sectile with instructions", "# Personal instructions\n\n" + block(sectileContextStart, sectileContextEnd) + "## Footer\n", "# Personal instructions\n\n## Footer\n", false, false},
		{"both marker spellings", "# Personal instructions\n\n" + block(contextStart, contextEnd) + block(sectileContextStart, sectileContextEnd) + "## Footer\n", "# Personal instructions\n\n## Footer\n", false, false},
		{"managed content only", block(sectileContextStart, sectileContextEnd), "", true, false},
		{"no managed markers", "# Personal instructions\n", "# Personal instructions\n", false, false},
		{"missing end", "# Personal instructions\n" + sectileContextStart + "\nmanaged content\n", "", false, true},
		{"orphan end", "# Personal instructions\n" + sectileContextEnd + "\n", "", false, true},
		{"mismatched markers", contextStart + "\nmanaged content\n" + sectileContextEnd + "\n", "", false, true},
		{"later incomplete block", block(contextStart, contextEnd) + sectileContextStart + "\nmanaged content\n", "", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, home := t.TempDir(), t.TempDir()
			testhome.Set(t, home)
			path := filepath.Join(root, "AGENTS.md")
			if err := os.WriteFile(path, []byte(tc.initial), 0644); err != nil {
				t.Fatal(err)
			}
			_, err := Scaffold(root, Config{SchemaVersion: Version})
			if tc.failed {
				if err == nil || !strings.Contains(err.Error(), "incomplete project context block") {
					t.Fatalf("expected incomplete-block error, got %v", err)
				}
				raw, readErr := os.ReadFile(path)
				if readErr != nil || string(raw) != tc.initial {
					t.Fatalf("file changed after error: %q, %v", raw, readErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			raw, readErr := os.ReadFile(path)
			if tc.remove {
				if !os.IsNotExist(readErr) {
					t.Fatalf("managed-only file was not removed: %v", readErr)
				}
			} else if readErr != nil || string(raw) != tc.want {
				t.Fatalf("AGENTS.md = %q, %v; want %q", raw, readErr, tc.want)
			}
		})
	}
}

func TestScaffoldRemovesManagedRegistrationFromCheckout(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	testhome.Set(t, home)
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
	testhome.Set(t, home)
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
