package agentconfig

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"tasks/internal/testhome"
	"testing"
)

// writeSettingsFile writes a raw settings file for the drop to read.
func writeSettingsFile(t *testing.T, raw string) string {
	t.Helper()
	testhome.Temp(t)
	path, _ := SettingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func catalogueProviders(s Settings) []string {
	var out []string
	for _, engine := range s.Engines.Catalogue {
		out = append(out, engine.Provider)
	}
	return out
}

// A retired engine goes as if the owner had removed it: the project and task
// choices pointing at it go too, and they fall back to their defaults.
func TestRetiredEngineIsDroppedWithItsChoices(t *testing.T) {
	writeSettingsFile(t, `{"layout":3,"engines":{"catalogue":[
		{"id":"e-claude","name":"Claude","provider":"claude"},
		{"id":"e-gemini","name":"Gemini","provider":"gemini"}],
		"default":"e-claude","projects":{"p":"e-gemini"},"tasks":{"t":"e-gemini"}}}`)
	s, err := ReadSettings(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got := catalogueProviders(s); !reflect.DeepEqual(got, []string{"claude"}) {
		t.Fatalf("catalogue: %v", got)
	}
	if s.Engines.Projects != nil || s.Engines.Tasks != nil {
		t.Fatalf("choices naming the retired engine survived: %+v", s.Engines)
	}
	if got := ResolveTask(Config{ProjectID: "p"}, s, "t"); got.EngineID != "e-claude" {
		t.Fatalf("the task must run its project default, the workstation default: %+v", got)
	}
}

// A retired default engine gives way to the first remaining entry.
func TestRetiredDefaultEngineGivesWayToTheNextEntry(t *testing.T) {
	writeSettingsFile(t, `{"layout":3,"engines":{"catalogue":[
		{"id":"e-cursor","name":"Cursor","provider":"cursor"},
		{"id":"e-codex","name":"Codex","provider":"codex"}],"default":"e-cursor"}}`)
	s, _ := ReadSettings(t.TempDir())
	if s.Engines.Default != "e-codex" || s.DefaultEngine().Provider != "codex" {
		t.Fatalf("default engine: %+v", s.Engines)
	}
}

// A catalogue left empty runs the engine of a workstation stating nothing.
func TestOnlyRetiredEnginesLeaveTheImplicitEngine(t *testing.T) {
	writeSettingsFile(t, `{"layout":3,"engines":{"catalogue":[{"id":"e-vibe","name":"Vibe","provider":"vibe"}],"default":"e-vibe"},
		"seeded":{"defaultEngine":"e-vibe"}}`)
	s, _ := ReadSettings(t.TempDir())
	if got := catalogueProviders(s); !reflect.DeepEqual(got, []string{DefaultProvider}) {
		t.Fatalf("catalogue: %v", got)
	}
	if s.Engines.Default != implicitEngineID || s.Seeded.DefaultEngine != "" {
		t.Fatalf("default: %+v, seeded %q", s.Engines, s.Seeded.DefaultEngine)
	}
	if err := ValidateEngines(s.Engines); err != nil {
		t.Fatalf("the remaining catalogue must stay valid: %v", err)
	}
}

// Model lists, MCP choices and the initialization provider of a retired
// provider go; those of a supported one stay.
func TestRetiredProviderKeysAreDropped(t *testing.T) {
	writeSettingsFile(t, `{"layout":3,"engines":{"catalogue":[{"id":"e-claude","name":"Claude","provider":"claude"}],"default":"e-claude"},
		"defaults":{"initializationProvider":"vibe","aiProviderModels":{"gemini":["g"],"cursor":[],"vibe":["v"],"claude":["c"]}},
		"mcpConnections":{"gemini":{"transport":"http","target":"remote"},"cursor":{"transport":"http","target":"remote"},
			"vibe":{"transport":"stdio","target":"local"},"claude":{"transport":"http","target":"remote"}}}`)
	s, drop, err := func() (Settings, RetiredDrop, error) {
		s, _, drop, err := readConverted(t.TempDir())
		return s, drop, err
	}()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.Defaults.AIProviderModels, map[string][]string{"claude": {"c"}}) {
		t.Fatalf("model lists: %v", s.Defaults.AIProviderModels)
	}
	if _, ok := s.MCPConnections["claude"]; !ok || len(s.MCPConnections) != 1 {
		t.Fatalf("MCP connections: %v", s.MCPConnections)
	}
	if s.Defaults.InitializationProvider != "" {
		t.Fatalf("initialization provider: %q", s.Defaults.InitializationProvider)
	}
	want := RetiredDrop{ModelLists: []string{"cursor", "gemini", "vibe"}, MCPConnections: []string{"cursor", "gemini", "vibe"}, InitializationProvider: "vibe"}
	if !reflect.DeepEqual(drop, want) {
		t.Fatalf("drop report: %+v", drop)
	}
	if err := ValidateDefaults(s.Defaults); err != nil {
		t.Fatalf("the remaining defaults must stay valid: %v", err)
	}
}

// Engine fields of #305 naming a retired provider produce no engine: the level
// they belonged to resolves to its default.
func TestLegacyRetiredProviderFieldsProduceNoEngine(t *testing.T) {
	writeSettingsFile(t, `{"layout":2,"defaults":{"aiProvider":"gemini","aiModel":"gemini-pro"},
		"projectSettings":{"p":{"path":"/p","aiProvider":"cursor"}}}`)
	s, _ := ReadSettings(t.TempDir())
	for _, engine := range s.Engines.Catalogue {
		if RetiredProviders[engine.Provider] {
			t.Fatalf("a retired engine was converted: %+v", s.Engines)
		}
	}
	for _, project := range []string{"p", "other"} {
		if got := Resolve(Config{ProjectID: project}, s); got.AIProvider != DefaultProvider {
			t.Fatalf("%s resolves %q, want the implicit engine", project, got.AIProvider)
		}
	}
}

// The drop is persisted once, with a backup, and reported; a file naming no
// retired provider is left alone.
func TestMigrateSettingsPersistsTheDropOnce(t *testing.T) {
	previous := `{"layout":3,"engines":{"catalogue":[{"id":"e-claude","name":"Claude","provider":"claude"},{"id":"e-vibe","name":"Mistral","provider":"vibe"}],"default":"e-claude"}}`
	path := writeSettingsFile(t, previous)
	changed, drop, err := MigrateSettingsReport(t.TempDir())
	if !changed || err != nil {
		t.Fatalf("migration: %v %v", changed, err)
	}
	if !reflect.DeepEqual(drop.Engines, []string{"Mistral [vibe]"}) || !strings.Contains(drop.String(), "Mistral [vibe]") {
		t.Fatalf("drop report: %+v %q", drop, drop)
	}
	if backup, err := os.ReadFile(path + ".bak-layout3"); err != nil || string(backup) != previous {
		t.Fatalf("backup: %s %v", backup, err)
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "vibe") {
		t.Fatalf("rewritten file still names the retired provider:\n%s", raw)
	}
	if changed, drop, err := MigrateSettingsReport(t.TempDir()); changed || !drop.Empty() || err != nil {
		t.Fatalf("second start: %v %+v %v", changed, drop, err)
	}
	if matches, _ := filepath.Glob(path + ".bak-layout3-*"); len(matches) != 0 {
		t.Fatalf("a file with nothing to drop was backed up again: %v", matches)
	}
}

// A server that ran a retired provider seeds no engine, and its lists for
// retired providers are not copied.
func TestSeedNamingARetiredProviderCreatesNoEngine(t *testing.T) {
	for provider := range RetiredProviders {
		t.Run(provider, func(t *testing.T) {
			s := converted(Settings{})
			ApplyWorkstationSeed(&s, SeedDefaults{AIProvider: provider, AIProviderModels: map[string][]string{provider: {"m"}, "claude": {"c"}}}, "https://server")
			if got := catalogueProviders(s); !reflect.DeepEqual(got, []string{DefaultProvider}) || s.Seeded.DefaultEngine != "" {
				t.Fatalf("catalogue: %v, seeded %q", got, s.Seeded.DefaultEngine)
			}
			if !reflect.DeepEqual(s.Defaults.AIProviderModels, map[string][]string{"claude": {"c"}}) {
				t.Fatalf("model lists: %v", s.Defaults.AIProviderModels)
			}
			ApplyProjectSeed(&s, Config{ProjectID: "p"}, SeedProject{ProjectID: "p", AIProvider: provider}, "now")
			if s.Engines.Projects["p"] != "" {
				t.Fatalf("the project seed picked an engine: %+v", s.Engines)
			}
			if got := Resolve(Config{ProjectID: "p"}, s); got.AIProvider != DefaultProvider {
				t.Fatalf("project resolves %q", got.AIProvider)
			}
		})
	}
}
