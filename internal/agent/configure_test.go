package agent

import (
	"strings"
	"testing"

	"tasks/internal/agentconfig"
)

// isolateSettings points the settings path at a scratch home. os.UserHomeDir
// reads USERPROFILE on Windows and HOME elsewhere, so both are set: overwriting
// only one leaves the test writing into the developer's real settings.
func isolateSettings(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

func TestConfigureSetsParallelism(t *testing.T) {
	isolateSettings(t)
	root := t.TempDir()
	if _, err := Configure([]string{"--repo", root, "--project", "p", "--parallelism", "3"}); err != nil {
		t.Fatalf("set parallelism: %v", err)
	}
	settings, err := agentconfig.ReadSettings(root)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if settings.Parallelism["p"] != 3 {
		t.Fatalf("stored parallelism = %v, want 3", settings.Parallelism["p"])
	}
	if limit := agentconfig.ExecutionLimit("p", true, settings); limit != 3 {
		t.Fatalf("effective limit = %d, want 3", limit)
	}
}

func TestConfigurePreservesOtherProjects(t *testing.T) {
	isolateSettings(t)
	root := t.TempDir()
	if err := agentconfig.WriteSettings(agentconfig.Overrides{
		Projects:    map[string]string{"p": "/repo", "other": "/other"},
		Parallelism: map[string]int{"other": 2},
	}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	if _, err := Configure([]string{"--repo", root, "--project", "p", "--parallelism", "4"}); err != nil {
		t.Fatalf("set parallelism: %v", err)
	}
	settings, err := agentconfig.ReadSettings(root)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if settings.Parallelism["p"] != 4 || settings.Parallelism["other"] != 2 {
		t.Fatalf("parallelism = %v, want p=4 other=2", settings.Parallelism)
	}
	if settings.Projects["p"] != "/repo" || settings.Projects["other"] != "/other" {
		t.Fatalf("projects = %v, want both mappings preserved", settings.Projects)
	}
}

func TestConfigureRejectsOutOfRange(t *testing.T) {
	isolateSettings(t)
	root := t.TempDir()
	for _, value := range []string{"0", "-1", "11"} {
		if _, err := Configure([]string{"--repo", root, "--project", "p", "--parallelism", value}); err == nil {
			t.Fatalf("parallelism %s was accepted, want a range error", value)
		}
	}
	settings, err := agentconfig.ReadSettings(root)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if _, stored := settings.Parallelism["p"]; stored {
		t.Fatalf("a refused value was stored: %v", settings.Parallelism)
	}
}

func TestConfigureRequiresProjectToSet(t *testing.T) {
	isolateSettings(t)
	if _, err := Configure([]string{"--repo", t.TempDir(), "--parallelism", "2"}); err == nil {
		t.Fatal("parallelism without a project was accepted, want an error")
	}
}

func TestConfigureReportsCurrentSettings(t *testing.T) {
	isolateSettings(t)
	root := t.TempDir()
	if err := agentconfig.WriteSettings(agentconfig.Overrides{
		Projects:    map[string]string{"mapped": "/repo", "plain": "/other"},
		Parallelism: map[string]int{"mapped": 5},
		Worktrees:   map[string]bool{"mapped": true},
	}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	report, err := Configure([]string{"--repo", root})
	if err != nil {
		t.Fatalf("report settings: %v", err)
	}
	if !strings.Contains(report, "mapped") || !strings.Contains(report, "5") {
		t.Fatalf("report omits the stored value:\n%s", report)
	}
	if !strings.Contains(report, "1 (default)") {
		t.Fatalf("report does not mark the unset project as a default:\n%s", report)
	}
}

func TestConfigureReportsNothingConfigured(t *testing.T) {
	isolateSettings(t)
	report, err := Configure([]string{"--repo", t.TempDir()})
	if err != nil {
		t.Fatalf("report settings: %v", err)
	}
	if !strings.Contains(report, "No project is configured") {
		t.Fatalf("unexpected report:\n%s", report)
	}
}
