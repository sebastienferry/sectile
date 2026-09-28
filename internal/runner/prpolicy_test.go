package runner

import (
	"strings"
	"testing"

	"tasks/internal/models"
)

// Clarification may own pull request creation (#580): its prompt points to
// the project policy like the specification's and the implementation's.
func TestPRPolicyReminderNamesTheThreeOwners(t *testing.T) {
	settings := &models.Settings{AIProvider: "claude", RepoPath: t.TempDir()}
	for _, skill := range []string{"clarify", "specify", "implement"} {
		inv, err := NewRunner().PrepareAI(settings, skill, &models.Task{Key: "TEST-1"}, "")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(inv.Prompt, "Clarification owns creation only for clarified timing, and only in its final round") {
			t.Errorf("%s prompt lacks the clarification owner: %s", skill, inv.Prompt)
		}
	}
}
