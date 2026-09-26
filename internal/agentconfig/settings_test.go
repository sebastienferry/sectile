package agentconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"tasks/internal/testhome"
	"testing"
)

func TestUserSettingsPreserveConnectionAndMigrate(t *testing.T) {
	testhome.Temp(t)
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".taskflow"), 0700)
	os.WriteFile(filepath.Join(root, ".taskflow", "agent.json"), []byte("{\"projects\":{\"p\":\"/repo\"},\"parallelism\":{\"p\":3}}"), 0600)
	settings, err := ReadSettings(root)
	if err != nil || settings.ProjectPath("p") != "/repo" {
		t.Fatal(settings, err)
	}
	path, _ := SettingsPath()
	os.MkdirAll(filepath.Dir(path), 0700)
	os.WriteFile(path, []byte("{\"server\":\"https://example.test\",\"secret\":\"encrypted\"}"), 0600)
	settings, err = ReadSettings(root)
	if err != nil || settings.Project("p").Parallelism != 3 {
		t.Fatal(settings, err)
	}
	p := settings.Project("p")
	p.Parallelism = 0
	settings.SetProject("p", p)
	if err := WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	var fields map[string]any
	json.Unmarshal(raw, &fields)
	if fields["secret"] != "encrypted" {
		t.Fatal("connection secret lost")
	}
	settings, err = ReadSettings(root)
	if err != nil || settings.Project("p").Parallelism != 0 {
		t.Fatal("cleared override restored", err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("settings permissions")
	}
}

func TestDisconnectionIsWorkstationOnlyAndSurvivesLegacyFallback(t *testing.T) {
	testhome.Temp(t)
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".taskflow"), 0700)
	os.WriteFile(filepath.Join(root, ".taskflow", "agent.json"), []byte(`{"projects":{"p":"/legacy"},"worktrees":{"p":true},"disconnectedProjects":{"other":true}}`), 0600)
	settings, err := ReadSettings(root)
	if err != nil || settings.ProjectPath("p") != "/legacy" || len(settings.DisconnectedProjects) != 0 {
		t.Fatal(settings, err)
	}
	path, _ := SettingsPath()
	os.MkdirAll(filepath.Dir(path), 0700)
	os.WriteFile(path, []byte(`{"server":"https://example.test","secret":"preserved","custom":42}`), 0600)
	settings.ProjectSettings = nil
	settings.DisconnectedProjects = map[string]bool{"p": true}
	if err := WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	settings, err = ReadSettings(root)
	if err != nil || !settings.DisconnectedProjects["p"] || len(settings.ProjectSettings) != 0 {
		t.Fatal(settings, err)
	}
	raw, _ := os.ReadFile(path)
	var fields map[string]any
	json.Unmarshal(raw, &fields)
	if fields["secret"] != "preserved" || fields["custom"] != float64(42) {
		t.Fatal("unrelated settings lost")
	}
	delete(settings.DisconnectedProjects, "p")
	if err := WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	settings, err = ReadSettings(root)
	if err != nil || len(settings.DisconnectedProjects) != 0 {
		t.Fatal("stale disconnection", settings, err)
	}
}

// Every legacy key lands in its new place, with the same meaning, and the next
// save rewrites the file in the current layout.
func TestLegacyLayoutIsFoldedAndRewritten(t *testing.T) {
	testhome.Temp(t)
	path, _ := SettingsPath()
	os.MkdirAll(filepath.Dir(path), 0700)
	legacy := `{
		"server": "https://example.test", "apiKey": "k", "custom": 42,
		"aiProvider": "claude", "aiCommandTemplate": "claude --x {prompt}", "aiCommandTemplateAutonomous": "claude -p {prompt}",
		"aiModel": "claude-opus-5", "aiSkillModels": {"implement": "claude-sonnet-5"}, "terminal": "ghostty",
		"projects": {"p": "/repo"}, "specRepos": {"p": "/specs"}, "worktrees": {"p": false}, "parallelism": {"p": 3},
		"terminals": {"p": "iterm"}, "aiProviders": {"p": "codex"}, "aiModels": {"p": "gpt-5"},
		"commands": {"p": "codex {prompt}"}, "commandsAutonomous": {"p": "codex exec {prompt}"},
		"repositories": {"github.com/o/r": "/other"}, "disconnectedProjects": {"gone": true},
		"skills": {"implement": "local"}, "mcpConnections": {"claude": {"transport": "http", "target": "local"}}
	}`
	os.WriteFile(path, []byte(legacy), 0600)
	got, err := ReadSettings(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	want := Settings{
		Defaults: Defaults{Execution: Execution{
			AIProvider: "claude", AICommandTemplate: "claude --x {prompt}", AICommandTemplateAutonomous: "claude -p {prompt}",
			AIModel: "claude-opus-5", AISkillModels: map[string]string{"implement": "claude-sonnet-5"}, Terminal: "ghostty",
		}},
		ProjectSettings: map[string]ProjectSettings{"p": {
			Path: "/repo", SpecPath: "/specs",
			Execution: Execution{
				AIProvider: "codex", AICommandTemplate: "codex {prompt}", AICommandTemplateAutonomous: "codex exec {prompt}",
				AIModel: "gpt-5", Terminal: "iterm", UseWorktrees: boolPtr(false), Parallelism: 3,
			},
		}},
		Repositories:         map[string]string{"github.com/o/r": "/other"},
		DisconnectedProjects: map[string]bool{"gone": true},
		MCPConnections:       map[string]MCPConnection{"claude": {Transport: "http", Target: "local"}},
		Skills:               map[string]string{"implement": "local"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fold:\n got  %+v\n want %+v", got, want)
	}
	if err := WriteSettings(got); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	var fields map[string]json.RawMessage
	json.Unmarshal(raw, &fields)
	for _, key := range legacyKeys {
		if _, ok := fields[key]; ok {
			t.Errorf("legacy key %q survived the rewrite", key)
		}
	}
	for _, key := range []string{"server", "apiKey", "custom"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("connection key %q lost", key)
		}
	}
	if string(fields["layout"]) != "2" {
		t.Fatalf("layout: %s", fields["layout"])
	}
	again, err := ReadSettings(t.TempDir())
	again.Layout = 0
	if err != nil || !reflect.DeepEqual(again, want) {
		t.Fatalf("the rewrite changed the meaning:\n got  %+v\n want %+v", again, want)
	}
	// Both levels resolve as they did from the legacy file.
	resolved := Resolve(Config{ProjectID: "p"}, again)
	if resolved.AIProvider != "codex" || resolved.AICommandTemplate != "codex {prompt}" || resolved.AIModel != "gpt-5" ||
		ResolveModel(resolved, "implement") != "claude-sonnet-5" || resolved.UseWorktrees || resolved.ExternalTerminalCommand != "iterm" {
		t.Fatalf("resolution after the rewrite: %+v", resolved)
	}
}

// An emptied map disappears from the file instead of keeping its content.
func TestWriteSettingsRemovesEmptiedMaps(t *testing.T) {
	testhome.Temp(t)
	s := Settings{
		Defaults:        Defaults{Execution: Execution{AISkillModels: map[string]string{"implement": "m"}}, AIProviderModels: map[string][]string{"claude": {"m"}}},
		ProjectSettings: map[string]ProjectSettings{"p": {SpecPath: "/specs", SkillCommands: map[string]string{"implement": "x"}}},
		Repositories:    map[string]string{"r": "/r"},
	}
	if err := WriteSettings(s); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadSettings(t.TempDir()); err != nil || got.SpecPath("p") != "/specs" || got.Repositories["r"] != "/r" {
		t.Fatalf("not stored: %+v %v", got, err)
	}
	if err := WriteSettings(Settings{ProjectSettings: map[string]ProjectSettings{}, Repositories: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadSettings(t.TempDir())
	if err != nil || len(got.ProjectSettings) != 0 || len(got.Repositories) != 0 || len(got.Defaults.AISkillModels) != 0 || got.Defaults.AIProviderModels != nil {
		t.Fatalf("an emptied value survived: %+v %v", got, err)
	}
}

// An empty setup provider list is the decision "none" and must survive a save.
func TestEmptySetupProvidersSurviveARoundTrip(t *testing.T) {
	testhome.Temp(t)
	if err := WriteSettings(Settings{ProjectSettings: map[string]ProjectSettings{"p": {Path: "/r", Execution: Execution{SetupProviders: []string{}}}}}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadSettings(t.TempDir())
	if err != nil || got.Project("p").SetupProviders == nil || got.Defaults.SetupProviders != nil {
		t.Fatalf("none became inherit, or inherit became none: %+v %v", got, err)
	}
}

// The seed markers survive a round trip.
func TestSeededMarkersRoundTrip(t *testing.T) {
	testhome.Temp(t)
	s := Settings{Seeded: Seeded{Defaults: "https://example.test", Projects: map[string]string{"p": "2026-09-25T00:00:00Z"}}}
	if err := WriteSettings(s); err != nil {
		t.Fatal(err)
	}
	got, err := ReadSettings(t.TempDir())
	if err != nil || !got.Seeded.hasSeededDefaults() || got.Seeded.Projects["p"] == "" {
		t.Fatalf("markers lost: %+v %v", got.Seeded, err)
	}
}

// The specification artefacts override (#487) lives in the project section:
// no key until one is saved, none once it is cleared, and a legacy
// per-project map is folded into it.
func TestSettingsSpecArtifactsAddNoKeyUntilUsed(t *testing.T) {
	testhome.Temp(t)
	path, _ := SettingsPath()
	hasKey := func() bool {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return strings.Contains(string(raw), `"specArtifacts"`)
	}
	if err := WriteSettings(Settings{ProjectSettings: map[string]ProjectSettings{"p": {Path: "/repo"}}}); err != nil {
		t.Fatal(err)
	}
	if hasKey() {
		t.Fatal("a workstation that never overrode the setting must not gain the key")
	}
	if err := WriteSettings(Settings{ProjectSettings: map[string]ProjectSettings{"p": {Path: "/repo", SpecArtifacts: "drop"}}}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadSettings(t.TempDir())
	if err != nil || got.Project("p").SpecArtifacts != "drop" {
		t.Fatalf("the override must be stored: %+v %v", got.Project("p"), err)
	}
	if resolved := Resolve(Config{ProjectID: "p", SpecArtifacts: "keep"}, got); !resolved.DropsSpecArtifacts() {
		t.Fatal("the workstation override must win over the server value")
	}
	if err := WriteSettings(Settings{ProjectSettings: map[string]ProjectSettings{"p": {Path: "/repo"}}}); err != nil {
		t.Fatal(err)
	}
	if hasKey() {
		t.Fatal("removing the last override must remove the key")
	}
	os.WriteFile(path, []byte(`{"specArtifacts":{"p":"drop"},"projects":{"p":"/repo"}}`), 0600)
	if got, err := ReadSettings(t.TempDir()); err != nil || got.Project("p").SpecArtifacts != "drop" || got.ProjectPath("p") != "/repo" {
		t.Fatalf("the legacy map must be folded: %+v %v", got.Project("p"), err)
	}
}
