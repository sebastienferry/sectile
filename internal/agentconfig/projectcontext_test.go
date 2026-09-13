package agentconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldPreservesProjectContext(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".taskflow"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".taskflow/config.json"), []byte(`{"customPlugin":{"active":true},"githubRepo":"old/repo"}`), 0600); err != nil {
		t.Fatal(err)
	}
	initial := "# Personal instructions\n" + contextStart + "\nold content\n" + contextEnd + "\n## Footer\n"
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}
	c := Config{SchemaVersion: Version, ProjectID: "project", GithubRepo: "new/repo", IssueTracker: "github"}
	for i := 0; i < 3; i++ {
		if _, err := Scaffold(root, c); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, ".taskflow/config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]any
	if err := json.Unmarshal(raw, &values); err != nil {
		t.Fatal(err)
	}
	if values["customPlugin"] == nil || values["githubRepo"] != "new/repo" {
		t.Fatalf("context lost: %s", raw)
	}
	raw, err = os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(raw), contextStart) != 1 || !strings.Contains(string(raw), "# Personal instructions") || !strings.Contains(string(raw), "## Footer") || !strings.Contains(string(raw), "new/repo") {
		t.Fatalf("instructions lost: %s", raw)
	}
}

func TestScaffoldRejectsContextSymlinkOutsideCheckout(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	dst := filepath.Join(outside, "personal.md")
	if err := os.WriteFile(dst, []byte("personal"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dst, filepath.Join(root, "AGENTS.md")); err != nil {
		t.Skip(err)
	}
	if _, err := Scaffold(root, Config{SchemaVersion: Version}); err == nil {
		t.Fatal("external symlink accepted")
	}
	raw, _ := os.ReadFile(dst)
	if string(raw) != "personal" {
		t.Fatal("external file changed")
	}
}
