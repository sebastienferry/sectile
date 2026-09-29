package runner

import (
	"strings"
	"testing"

	"tasks/internal/models"
)

// A task without a branch yet is told the one the agent would create for it
// under the default format (#621), not the legacy "<key>-<title>" name.
func TestFallbackBranchNameIsTheDefaultFormat(t *testing.T) {
	settings := &models.Settings{AIProvider: "claude", RepoPath: t.TempDir()}
	inv, err := NewRunner().PrepareAI(settings, "implement", &models.Task{Key: "TEST-1", Title: "Fix the thing"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inv.Prompt, "feat/test-1") || strings.Contains(inv.Prompt, "TEST-1-fix") {
		t.Fatalf("the prompt does not name feat/test-1: %s", inv.Prompt)
	}
	assigned := "AUC-9"
	inv, err = NewRunner().PrepareAI(settings, "implement", &models.Task{Key: "TEST-1", BranchName: &assigned}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inv.Prompt, "AUC-9") {
		t.Fatalf("the prompt does not name the assigned branch: %s", inv.Prompt)
	}
}
