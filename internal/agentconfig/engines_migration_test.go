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

// layout2Engine is what Resolve returned for a project before #510, written
// out independently of the conversion it checks.
func layout2Engine(s Settings, projectID string) Config {
	d, p := s.Defaults.Execution, s.ProjectSettings[projectID].Execution
	provider := firstSet(strings.TrimSpace(d.AIProvider), DefaultProvider)
	command, autonomous := d.AICommandTemplate, d.AICommandTemplateAutonomous
	if own := strings.TrimSpace(p.AIProvider); own != "" {
		if own != provider {
			command, autonomous = "", ""
		}
		provider = own
	}
	command, autonomous = firstSet(p.AICommandTemplate, command), firstSet(p.AICommandTemplateAutonomous, autonomous)
	m := MergeModels(ModelConfig{Model: p.AIModel, SkillModels: p.AISkillModels}, ModelConfig{Model: d.AIModel, SkillModels: d.AISkillModels})
	return Config{
		AIProvider: provider, AICommandTemplate: EffectiveCommandTemplate(provider, command),
		AICommandTemplateAutonomous: EffectiveCommandTemplate(provider, autonomous), AIModel: m.Model, AISkillModels: m.SkillModels,
	}
}

func mustJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

// sameEngine compares what two configurations run, skill by skill.
func sameEngine(a, b Config) bool {
	if a.AIProvider != b.AIProvider || a.AICommandTemplate != b.AICommandTemplate || a.AICommandTemplateAutonomous != b.AICommandTemplateAutonomous {
		return false
	}
	for _, skill := range []string{"", "clarify", "specify", "implement", "adjust"} {
		if ResolveModel(a, skill) != ResolveModel(b, skill) {
			return false
		}
	}
	return true
}

func TestConversionKeepsEveryProjectResolution(t *testing.T) {
	shapes := map[string]struct {
		settings Settings
		entries  int
	}{
		"defaults only": {Settings{Defaults: Defaults{Execution: Execution{
			AIProvider: "claude", AICommandTemplate: "claude --x {prompt}", AIModel: "claude-opus-5", AISkillModels: map[string]string{"implement": "claude-sonnet-5"},
		}}, ProjectSettings: map[string]ProjectSettings{"plain": {Path: "/plain"}}}, 1},
		"project with its own provider": {Settings{
			Defaults:        Defaults{Execution: Execution{AIProvider: "claude"}},
			ProjectSettings: map[string]ProjectSettings{"p": {Execution: Execution{AIProvider: "codex", AICommandTemplate: "codex {prompt}"}}, "plain": {Path: "/x"}},
		}, 2},
		"provider change drops the inherited templates": {Settings{
			Defaults:        Defaults{Execution: Execution{AIProvider: "gemini", AICommandTemplate: "gemini --x {prompt}", AICommandTemplateAutonomous: "gemini -p {prompt}"}},
			ProjectSettings: map[string]ProjectSettings{"p": {Execution: Execution{AIProvider: "claude"}}},
		}, 2},
		"defaults without a provider, project on another one": {Settings{
			Defaults:        Defaults{Execution: Execution{AICommandTemplate: "agy --x {prompt}"}},
			ProjectSettings: map[string]ProjectSettings{"p": {Execution: Execution{AIProvider: "claude"}}, "same": {Execution: Execution{AIProvider: "agy"}}},
		}, 2},
		"project with models only": {Settings{
			Defaults: Defaults{Execution: Execution{AIProvider: "claude", AICommandTemplate: "claude --x {prompt}", AISkillModels: map[string]string{"clarify": "claude-haiku-4-5"}}},
			ProjectSettings: map[string]ProjectSettings{"p": {Execution: Execution{
				AIModel: "claude-opus-5", AISkillModels: map[string]string{"implement": "claude-sonnet-5"},
			}}},
		}, 2},
		"identical profiles collapse": {Settings{
			Defaults: Defaults{Execution: Execution{AIProvider: "claude"}},
			ProjectSettings: map[string]ProjectSettings{
				"a": {Execution: Execution{AIProvider: "codex", AIModel: "gpt-5"}}, "b": {Execution: Execution{AIProvider: "codex", AIModel: " gpt-5 "}},
				"same-as-workstation": {Execution: Execution{AIProvider: "claude"}},
			},
		}, 2},
		"no engine field": {Settings{ProjectSettings: map[string]ProjectSettings{"p": {Path: "/p", Execution: Execution{Parallelism: 2}}}}, 1},
		"project only, workstation silent": {Settings{
			ProjectSettings: map[string]ProjectSettings{"p": {Execution: Execution{AIProvider: "claude"}}, "plain": {Path: "/x"}},
		}, 2},
		"legacy bare CLI name": {Settings{Defaults: Defaults{Execution: Execution{AIProvider: "claude", AICommandTemplate: "claude"}}}, 1},
		"legacy flat keys": {legacySettings{
			AIProvider: "claude", AIModel: "claude-opus-5", Projects: map[string]string{"p": "/p", "q": "/q"},
			AIProviders: map[string]string{"p": "codex"}, AIModels: map[string]string{"q": "claude-haiku-4-5"},
			Commands: map[string]string{"p": "codex {prompt}"},
		}.fold(), 3},
	}
	for name, shape := range shapes {
		t.Run(name, func(t *testing.T) {
			before := shape.settings
			after := converted(before)
			if len(after.Engines.Catalogue) != shape.entries {
				t.Fatalf("catalogue: got %d entries, want %d: %+v", len(after.Engines.Catalogue), shape.entries, after.Engines)
			}
			for _, id := range []string{"p", "q", "a", "b", "same", "plain", "same-as-workstation", "unknown"} {
				if got, want := Resolve(Config{ProjectID: id}, after), layout2Engine(before, id); !sameEngine(got, want) {
					t.Errorf("project %s:\n got  %+v\n want %+v", id, got, want)
				}
			}
			if after.Defaults.statesEngine() {
				t.Fatalf("engine fields left in the defaults: %+v", after.Defaults)
			}
			for id, p := range after.ProjectSettings {
				if p.statesEngine() {
					t.Fatalf("engine fields left in project %s: %+v", id, p)
				}
			}
			if err := ValidateEngines(after.Engines); err != nil {
				t.Fatalf("the conversion wrote an invalid catalogue: %v", err)
			}
			// Idempotent: a second conversion changes nothing.
			again := converted(after)
			changed := convertEngines(&again)
			if a, b := mustJSON(again), mustJSON(after); changed || a != b {
				t.Fatalf("a second conversion changed the settings:\n got  %s\n want %s", a, b)
			}
		})
	}
}

func TestConversionNamesAndIdentities(t *testing.T) {
	s := converted(Settings{
		Defaults: Defaults{Execution: Execution{AIProvider: "claude", AIModel: "claude-opus-5"}},
		ProjectSettings: map[string]ProjectSettings{
			"a": {Execution: Execution{AIProvider: "claude", AIModel: "claude-opus-5", AICommandTemplate: "claude --x {prompt}"}},
			"b": {Execution: Execution{AIProvider: "codex"}},
		},
	})
	names := []string{}
	for _, engine := range s.Engines.Catalogue {
		names = append(names, engine.Name)
		if !strings.HasPrefix(engine.ID, "e-") {
			t.Fatalf("identity: %q", engine.ID)
		}
	}
	if !reflect.DeepEqual(names, []string{"Claude - claude-opus-5", "Claude - claude-opus-5 (2)", "Codex - claude-opus-5"}) {
		t.Fatalf("names: %v", names)
	}
	if s.Engines.Default != s.Engines.Catalogue[0].ID {
		t.Fatal("the workstation profile is the default engine")
	}
	// The same file converts to the same identities, read after read.
	if again := converted(Settings{
		Defaults: Defaults{Execution: Execution{AIProvider: "claude", AIModel: "claude-opus-5"}},
		ProjectSettings: map[string]ProjectSettings{
			"a": {Execution: Execution{AIProvider: "claude", AIModel: "claude-opus-5", AICommandTemplate: "claude --x {prompt}"}},
			"b": {Execution: Execution{AIProvider: "codex"}},
		},
	}); !reflect.DeepEqual(again.Engines, s.Engines) {
		t.Fatalf("identities changed between two conversions:\n%+v\n%+v", again.Engines, s.Engines)
	}
	// An identity already taken by another profile gets a suffix.
	edited := s
	edited.Engines.Catalogue = append([]Engine{}, s.Engines.Catalogue...)
	edited.Engines.Catalogue[2].Model = "gpt-5"
	edited.ProjectSettings = map[string]ProjectSettings{"c": {Execution: Execution{AIProvider: "codex", AIModel: "claude-opus-5"}}}
	convertEngines(&edited)
	if id := edited.Engines.Projects["c"]; id != s.Engines.Catalogue[2].ID+"-2" {
		t.Fatalf("collision: got %q", id)
	}
}

func TestIdentitiesAreStableAcrossReads(t *testing.T) {
	testhome.Temp(t)
	path, _ := SettingsPath()
	os.MkdirAll(filepath.Dir(path), 0700)
	os.WriteFile(path, []byte(`{"layout":2,"defaults":{"aiProvider":"claude"},"projectSettings":{"p":{"aiProvider":"codex"}}}`), 0600)
	first, err := ReadSettings(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	second, _ := ReadSettings(t.TempDir())
	if !reflect.DeepEqual(first.Engines, second.Engines) || len(first.Engines.Catalogue) != 2 {
		t.Fatalf("identities changed between two reads:\n%+v\n%+v", first.Engines, second.Engines)
	}
}

func TestMigrateSettingsBacksUpOnce(t *testing.T) {
	testhome.Temp(t)
	path, _ := SettingsPath()
	if changed, err := MigrateSettings(t.TempDir()); changed || err != nil {
		t.Fatalf("a missing file needs no migration: %v %v", changed, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("the migration created a settings file")
	}
	os.MkdirAll(filepath.Dir(path), 0700)
	previous := []byte(`{"server":"https://s","layout":2,"defaults":{"aiProvider":"claude","terminal":"ghostty"},"projectSettings":{"p":{"path":"/p","aiModel":"claude-opus-5"}}}`)
	os.WriteFile(path, previous, 0600)
	changed, err := MigrateSettings(t.TempDir())
	if !changed || err != nil {
		t.Fatalf("migration: %v %v", changed, err)
	}
	backup, err := os.ReadFile(path + ".bak-layout2")
	if err != nil || string(backup) != string(previous) {
		t.Fatalf("backup: %s %v", backup, err)
	}
	raw, _ := os.ReadFile(path)
	var fields map[string]json.RawMessage
	json.Unmarshal(raw, &fields)
	if string(fields["layout"]) != "3" || string(fields["server"]) != `"https://s"` ||
		strings.Contains(string(fields["defaults"]), "aiProvider") || strings.Contains(string(fields["projectSettings"]), "aiModel") {
		t.Fatalf("rewritten file:\n%s", raw)
	}
	s, _ := ReadSettings(t.TempDir())
	if got := Resolve(Config{ProjectID: "p"}, s); got.AIProvider != "claude" || got.AIModel != "claude-opus-5" || got.ExternalTerminalCommand != "ghostty" {
		t.Fatalf("resolution after the migration: %+v", got)
	}
	// The next start finds nothing to do.
	if changed, err := MigrateSettings(t.TempDir()); changed || err != nil {
		t.Fatalf("second start: %v %v", changed, err)
	}
	// Engine fields written again by an older agent convert again, beside the first backup.
	os.WriteFile(path, []byte(strings.Replace(string(raw), `"layout": 3`, `"layout": 2`, 1)), 0600)
	if changed, _ := MigrateSettings(t.TempDir()); !changed {
		t.Fatal("a layout-2 stamp must be rewritten")
	}
	matches, _ := filepath.Glob(path + ".bak-layout2-*")
	if len(matches) != 1 {
		t.Fatalf("a second backup must not overwrite the first: %v", matches)
	}
	if again, _ := os.ReadFile(path + ".bak-layout2"); string(again) != string(previous) {
		t.Fatal("the first backup was overwritten")
	}
}
