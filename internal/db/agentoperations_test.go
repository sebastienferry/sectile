package db

import (
	"testing"
	"time"
)

func TestOperationTimeout(t *testing.T) {
	cases := []struct {
		action string
		want   time.Duration
		why    string
	}{
		{"spec_install", 7 * time.Minute, "installs a spec toolchain"},
		{"run_prompt", 12 * time.Minute, "runs a full agent prompt"},
		{"git_evidence", 15 * time.Second, "three local git plumbing calls"},
		{"git_status", 15 * time.Second, "local porcelain read"},
		{"cli_status", 30 * time.Second, "probes several CLI binaries"},
		{"prepare_workspace", 17 * time.Minute, "installs the worktree's JavaScript dependencies"},
		{"git_delete", 45 * time.Second, "may delete a remote branch"},
		{"unknown_action", 45 * time.Second, "unknown actions keep the default"},
	}
	for _, c := range cases {
		if got := operationTimeout(c.action); got != c.want {
			t.Errorf("operationTimeout(%q) = %s, want %s (%s)", c.action, got, c.want, c.why)
		}
	}
}

// A local inspection must fail well before the default, otherwise an agent that
// is gone stalls every caller for the full 45s the default allows.
func TestOperationTimeout_LocalInspectionsBeatTheDefault(t *testing.T) {
	def := operationTimeout("unknown_action")
	for action, budget := range localInspections {
		if budget >= def {
			t.Errorf("local inspection %q has a %s budget, not shorter than the %s default", action, budget, def)
		}
	}
}
