package agentconfig

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"tasks/internal/testhome"
)

func fullSandbox() *ClaudeSandbox {
	return &ClaudeSandbox{
		Enabled:        boolPtr(true),
		AllowedDomains: []string{"registry.npmjs.org"},
		AllowWrite:     []string{"~/.cache/go-build"},
		Allow:          []string{"Bash(make test:*)"},
		Deny:           []string{"Bash(git push:*)"},
	}
}

func TestNormalizeClaudeSandboxTrimsAndDedupes(t *testing.T) {
	got, err := NormalizeClaudeSandbox(ClaudeSandbox{
		AllowedDomains: []string{" b.example ", "a.example", "b.example"},
		Allow:          []string{"Read", "Read ", "Bash(ls:*)"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.AllowedDomains, []string{"b.example", "a.example"}) || !reflect.DeepEqual(got.Allow, []string{"Read", "Bash(ls:*)"}) {
		t.Fatalf("order or dedupe lost: %+v", got)
	}
	if got.Enabled != nil || got.AllowWrite != nil || got.Deny != nil {
		t.Fatalf("an unset value became set: %+v", got)
	}
}

func TestNormalizeClaudeSandboxRefusesAnEmptyEntry(t *testing.T) {
	for _, entry := range []string{"", "   "} {
		if _, err := NormalizeClaudeSandbox(ClaudeSandbox{Deny: []string{"Read", entry}}); !errors.Is(err, ErrEmptyEntry) || !strings.Contains(err.Error(), "deny") {
			t.Fatalf("%q: want an empty entry refusal naming the list, got %v", entry, err)
		}
	}
	if _, err := NormalizeClaudeSandbox(ClaudeSandbox{Allow: []string{"Bash(a\nb)"}}); err == nil {
		t.Fatal("a multi-line rule was accepted")
	}
	if err := ValidateProject(ProjectSettings{ClaudeSandbox: &ClaudeSandbox{Allow: []string{" "}}}); err == nil {
		t.Fatal("ValidateProject accepted an empty rule")
	}
}

func TestAddAllowKeepsOrderAndReportsAChange(t *testing.T) {
	c := ClaudeSandbox{Allow: []string{"Read"}}
	if !c.AddAllow("Bash(npm test:*)", " Read ", "", "Bash(npm test:*)") {
		t.Fatal("a new rule was not reported")
	}
	if c.AddAllow("Read", "Bash(npm test:*)") {
		t.Fatal("known rules were reported as a change")
	}
	if !reflect.DeepEqual(c.Allow, []string{"Read", "Bash(npm test:*)"}) {
		t.Fatalf("allow = %v", c.Allow)
	}
}

// The values survive a save, an emptied section is dropped, and the other keys
// of the project section stay.
func TestClaudeSandboxRoundTrip(t *testing.T) {
	testhome.Temp(t)
	s := Settings{ProjectSettings: map[string]ProjectSettings{
		"p1": {Path: "/r", ClaudeSandbox: fullSandbox()},
		"p2": {ClaudeSandbox: &ClaudeSandbox{Deny: []string{"Read"}}},
	}}
	if err := WriteSettings(s); err != nil {
		t.Fatal(err)
	}
	got, err := ReadSettings(t.TempDir())
	if err != nil || !reflect.DeepEqual(got.Project("p1").ClaudeSandbox, fullSandbox()) || got.ProjectPath("p1") != "/r" {
		t.Fatalf("values lost: %+v %v", got.Project("p1"), err)
	}
	if deny := got.Project("p2").ClaudeSandbox; deny == nil || !reflect.DeepEqual(deny.Deny, []string{"Read"}) {
		t.Fatalf("the other project's values changed: %+v", deny)
	}
	got.SetProject("p2", ProjectSettings{ClaudeSandbox: &ClaudeSandbox{}})
	if err := WriteSettings(got); err != nil {
		t.Fatal(err)
	}
	again, err := ReadSettings(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, kept := again.ProjectSettings["p2"]; kept {
		t.Fatal("an emptied section was kept")
	}
	if !reflect.DeepEqual(again.Project("p1").ClaudeSandbox, fullSandbox()) {
		t.Fatal("the remaining project lost its values")
	}
}

// The legacy repository file never states sandbox values: the workstation's
// survive WithRepositoryFile.
func TestClaudeSandboxSurvivesTheRepositoryFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".taskflow"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".taskflow", "agent.json"), []byte(`{"projects":{"p1":"/legacy"},"aiModels":{"p1":"m"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := WithRepositoryFile(Settings{ProjectSettings: map[string]ProjectSettings{"p1": {ClaudeSandbox: fullSandbox()}}}, root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Project("p1").ClaudeSandbox, fullSandbox()) {
		t.Fatalf("values lost: %+v", got.Project("p1"))
	}
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestClaudeSettingsFileWritesClaudesShape(t *testing.T) {
	testhome.Temp(t)
	path, err := writeClaudeSettingsFile("p1", fullSandbox(), "darwin")
	if err != nil {
		t.Fatal(err)
	}
	settings, _ := SettingsPath()
	if path != filepath.Join(filepath.Dir(settings), "claude", "p1.json") {
		t.Fatalf("path = %s", path)
	}
	want := map[string]any{
		"sandbox": map[string]any{
			"enabled":    true,
			"network":    map[string]any{"allowedDomains": []any{"registry.npmjs.org"}},
			"filesystem": map[string]any{"allowWrite": []any{"~/.cache/go-build"}},
		},
		"permissions": map[string]any{"allow": []any{"Bash(make test:*)"}, "deny": []any{"Bash(git push:*)"}},
	}
	if got := readJSON(t, path); !reflect.DeepEqual(got, want) {
		t.Fatalf("content = %v", got)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("file mode = %v %v", info.Mode(), err)
	}
	if info, err := os.Stat(filepath.Dir(path)); err != nil || info.Mode().Perm() != 0700 {
		t.Fatalf("directory mode = %v %v", info.Mode(), err)
	}
}

func TestClaudeSettingsFileWritesOnlyTheSetKeys(t *testing.T) {
	testhome.Temp(t)
	path, err := writeClaudeSettingsFile("p1", &ClaudeSandbox{Deny: []string{"Read"}}, "linux")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := readJSON(t, path), map[string]any{"permissions": map[string]any{"deny": []any{"Read"}}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("content = %v", got)
	}
	path, err = writeClaudeSettingsFile("p1", &ClaudeSandbox{Enabled: boolPtr(false)}, "linux")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := readJSON(t, path), map[string]any{"sandbox": map[string]any{"enabled": false}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("an explicit off was lost: %v", got)
	}
}

func TestClaudeSettingsFileLeavesTheSandboxOutOnWindows(t *testing.T) {
	testhome.Temp(t)
	path, err := writeClaudeSettingsFile("p1", fullSandbox(), "windows")
	if err != nil {
		t.Fatal(err)
	}
	if got := readJSON(t, path); got["sandbox"] != nil || got["permissions"] == nil {
		t.Fatalf("content = %v", got)
	}
	sandboxOnly := &ClaudeSandbox{Enabled: boolPtr(true), AllowedDomains: []string{"a.example"}}
	if path, err := writeClaudeSettingsFile("p1", sandboxOnly, "windows"); err != nil || path != "" {
		t.Fatalf("sandbox-only values on Windows wrote %q %v", path, err)
	}
}

func TestClaudeSettingsFileRemovesAStaleFile(t *testing.T) {
	testhome.Temp(t)
	path, err := writeClaudeSettingsFile("p1", fullSandbox(), "darwin")
	if err != nil || path == "" {
		t.Fatal(err)
	}
	for _, zero := range []*ClaudeSandbox{nil, {}} {
		got, err := writeClaudeSettingsFile("p1", zero, "darwin")
		if err != nil || got != "" {
			t.Fatalf("zero values returned %q %v", got, err)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stale file kept: %v", err)
		}
	}
}

func TestClaudeSettingsFileRefusesAPathAsProjectID(t *testing.T) {
	testhome.Temp(t)
	for _, id := range []string{"", "../p", "a/b", ".hidden"} {
		if _, err := writeClaudeSettingsFile(id, fullSandbox(), "darwin"); err == nil {
			t.Fatalf("%q accepted", id)
		}
	}
}

func TestMergeClaudeSandboxKeepsWhatTheStoreGained(t *testing.T) {
	base := ClaudeSandbox{Allow: []string{"Read", "Bash(ls:*)"}, AllowedDomains: []string{"a.example"}}
	stored := ClaudeSandbox{Allow: []string{"Read", "Bash(ls:*)", "Bash(npm test:*)"}, AllowedDomains: []string{"a.example"}}
	sent := ClaudeSandbox{Enabled: boolPtr(false), Allow: []string{"Read", "Edit"}}
	got := MergeClaudeSandbox(sent, base, stored)
	if !reflect.DeepEqual(got.Allow, []string{"Read", "Edit", "Bash(npm test:*)"}) {
		t.Fatalf("allow = %v", got.Allow)
	}
	if got.AllowedDomains != nil || got.Enabled == nil || *got.Enabled {
		t.Fatalf("a removal or the state was not the owner's: %+v", got)
	}
	if cleared := MergeClaudeSandbox(ClaudeSandbox{}, stored, stored); !cleared.IsZero() {
		t.Fatalf("clearing every entry kept %+v", cleared)
	}
}

func TestSandboxAutonomyPolicySurvivesSaveAndProjectResolution(t *testing.T) {
	enabled, blocked := true, false
	global := ClaudeSandbox{Enabled: &enabled, AutoAllowBashIfSandboxed: &enabled, AllowUnsandboxedCommands: &blocked, AdditionalDirectories: []string{" /shared ", "/shared"}, Deny: []string{"Bash(git push *)"}}
	normalized, err := NormalizeClaudeSandbox(global)
	if err != nil {
		t.Fatal(err)
	}
	merged := MergeClaudeSandbox(normalized, ClaudeSandbox{}, ClaudeSandbox{AdditionalDirectories: []string{"/approved-meanwhile"}})
	if !reflect.DeepEqual(merged.AdditionalDirectories, []string{"/shared", "/approved-meanwhile"}) {
		t.Fatalf("folders = %v", merged.AdditionalDirectories)
	}
	settings := Settings{}
	settings.Defaults.ClaudeSandbox = &merged
	project := settings.Project("p")
	project.ClaudeSandbox = &ClaudeSandbox{AutoAllowBashIfSandboxed: &blocked}
	settings.SetProject("p", project)
	resolved := settings.ResolvedClaudeSandbox("p")
	document := claudeSettingsDocument(*resolved, "darwin")
	sandbox := document["sandbox"].(map[string]any)
	if sandbox["enabled"] != true || sandbox["autoAllowBashIfSandboxed"] != false || sandbox["allowUnsandboxedCommands"] != false {
		t.Fatalf("policy = %v", sandbox)
	}
	permissions := document["permissions"].(map[string]any)
	if !reflect.DeepEqual(permissions["additionalDirectories"], merged.AdditionalDirectories) || !reflect.DeepEqual(permissions["deny"], global.Deny) {
		t.Fatalf("permissions = %v", permissions)
	}
	windows := claudeSettingsDocument(*resolved, "windows")
	if windows["sandbox"] != nil || windows["permissions"] == nil {
		t.Fatalf("Windows settings = %v", windows)
	}
	if (&ClaudeSandbox{AutoAllowBashIfSandboxed: &blocked}).IsZero() {
		t.Fatal("an explicit false policy was discarded")
	}
}
