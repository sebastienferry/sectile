package agentconfig

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"tasks/internal/testhome"
)

// The commands Claude Code runs outside its sandbox (#764).

func TestClaudeSettingsFileWritesTheExcludedCommands(t *testing.T) {
	testhome.Temp(t)
	sandbox := &ClaudeSandbox{Enabled: boolPtr(true), ExcludedCommands: []string{"git *", "glab *"}}
	path, err := writeClaudeSettingsFile("p1", sandbox, "darwin")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"sandbox": map[string]any{"enabled": true, "excludedCommands": []any{"git *", "glab *"}}}
	if got := readJSON(t, path); !reflect.DeepEqual(got, want) {
		t.Fatalf("content = %v", got)
	}
}

func TestClaudeSettingsFileWritesExcludedCommandsAlone(t *testing.T) {
	testhome.Temp(t)
	path, err := writeClaudeSettingsFile("p1", &ClaudeSandbox{ExcludedCommands: []string{"gh *"}}, "linux")
	if err != nil || path == "" {
		t.Fatalf("an excluded command alone wrote %q %v", path, err)
	}
	if got, want := readJSON(t, path), map[string]any{"sandbox": map[string]any{"excludedCommands": []any{"gh *"}}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("content = %v", got)
	}
}

func TestClaudeSettingsFileLeavesExcludedCommandsOutOnWindows(t *testing.T) {
	testhome.Temp(t)
	if path, err := writeClaudeSettingsFile("p1", &ClaudeSandbox{ExcludedCommands: []string{"git *"}}, "windows"); err != nil || path != "" {
		t.Fatalf("excluded commands alone on Windows wrote %q %v", path, err)
	}
	path, err := writeClaudeSettingsFile("p1", &ClaudeSandbox{ExcludedCommands: []string{"git *"}, Deny: []string{"Bash(sudo *)"}}, "windows")
	if err != nil {
		t.Fatal(err)
	}
	if got := readJSON(t, path); got["sandbox"] != nil || got["permissions"] == nil {
		t.Fatalf("content = %v", got)
	}
}

func TestExcludedCommandsAreNormalizedLikeTheOtherLists(t *testing.T) {
	if (&ClaudeSandbox{ExcludedCommands: []string{"git *"}}).IsZero() {
		t.Fatal("a list of excluded commands reads as stating nothing")
	}
	got, err := NormalizeClaudeSandbox(ClaudeSandbox{ExcludedCommands: []string{" glab * ", "glab *", "git"}})
	if err != nil {
		t.Fatal(err)
	}
	// Entries are stored as typed: a bare name stays an exact match.
	if !reflect.DeepEqual(got.ExcludedCommands, []string{"glab *", "git"}) {
		t.Fatalf("excludedCommands = %v", got.ExcludedCommands)
	}
	for _, bad := range []string{"  ", "git fetch *\nrm -rf /"} {
		_, err := NormalizeClaudeSandbox(ClaudeSandbox{ExcludedCommands: []string{bad}})
		if err == nil || !strings.HasPrefix(err.Error(), "excludedCommands: ") {
			t.Fatalf("%q: err = %v", bad, err)
		}
	}
	if _, err := NormalizeClaudeSandbox(ClaudeSandbox{ExcludedCommands: []string{""}}); !errors.Is(err, ErrEmptyEntry) {
		t.Fatalf("empty entry: err = %v", err)
	}
}

func TestMergeClaudeSandboxKeepsAnExcludedCommandTheStoreGained(t *testing.T) {
	base := ClaudeSandbox{ExcludedCommands: []string{"git fetch *", "gh *"}}
	stored := ClaudeSandbox{ExcludedCommands: []string{"git fetch *", "gh *", "glab *"}}
	sent := ClaudeSandbox{ExcludedCommands: []string{"git fetch *"}}
	if got := MergeClaudeSandbox(sent, base, stored); !reflect.DeepEqual(got.ExcludedCommands, []string{"git fetch *", "glab *"}) {
		t.Fatalf("excludedCommands = %v", got.ExcludedCommands)
	}
}

func TestResolvedClaudeSandboxJoinsTheExcludedCommands(t *testing.T) {
	s := Settings{Defaults: Defaults{ClaudeSandbox: &ClaudeSandbox{ExcludedCommands: []string{"git fetch *", "gh *"}}}}
	s.SetProject("p", ProjectSettings{Path: "/p", ClaudeSandbox: &ClaudeSandbox{ExcludedCommands: []string{"gh *", "glab *"}}})
	got := s.ResolvedClaudeSandbox("p")
	if got == nil || !reflect.DeepEqual(got.ExcludedCommands, []string{"git fetch *", "gh *", "glab *"}) {
		t.Fatalf("resolved = %+v", got)
	}
}
