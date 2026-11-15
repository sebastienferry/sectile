package db

import (
	"strings"
	"testing"

	"tasks/internal/models"
)

// An agent adds the owner's answers to a running run of its own user and
// device (#904); another user's or device's run, an unknown run, a finished
// run and an oversized step are left alone.
func TestAgentRunStepLandsOnlyOnItsOwnRunningRun(t *testing.T) {
	database := engineDB(t)
	task := engineTask(t, database)
	run, err := database.StartAgentRun(task.ID, "clarify", RunLaunch{UserID: "carol", DeviceID: "laptop", Mode: models.SkillModeInteractive})
	if err != nil {
		t.Fatal(err)
	}
	step := "Owner answers:\nWhich scope? → Small\nToken? → [hidden]"

	refused := map[string]func() (bool, error){
		"another user":   func() (bool, error) { return database.AppendAgentRunStep("dave", "laptop", run.ID, step) },
		"another device": func() (bool, error) { return database.AppendAgentRunStep("carol", "desktop", run.ID, step) },
		"an unknown run": func() (bool, error) { return database.AppendAgentRunStep("carol", "laptop", "missing", step) },
		"a blank step":   func() (bool, error) { return database.AppendAgentRunStep("carol", "laptop", run.ID, "  ") },
		"a huge step": func() (bool, error) {
			return database.AppendAgentRunStep("carol", "laptop", run.ID, strings.Repeat("x", runStepLimit+1))
		},
	}
	for name, try := range refused {
		if added, _ := try(); added {
			t.Fatalf("%s added a step", name)
		}
	}
	added, err := database.AppendAgentRunStep("carol", "laptop", run.ID, step)
	if err != nil || !added {
		t.Fatalf("the owner's step was not added: %v", err)
	}
	stored, err := database.GetActivityByID(run.ID)
	if err != nil || stored == nil {
		t.Fatalf("run not found: %v", err)
	}
	if len(stored.Steps) == 0 || stored.Steps[len(stored.Steps)-1] != step {
		t.Fatalf("steps = %q, want the answers last", stored.Steps)
	}
	if strings.Count(strings.Join(stored.Steps, "\n"), "Owner answers:") != 1 {
		t.Fatalf("a refused step reached the activity: %q", stored.Steps)
	}

	if _, err := database.FinishRemoteRun(task.ID, run.ID, "completed", "done"); err != nil {
		t.Fatal(err)
	}
	if added, _ := database.AppendAgentRunStep("carol", "laptop", run.ID, step); added {
		t.Fatal("a finished run took a late step")
	}
}
