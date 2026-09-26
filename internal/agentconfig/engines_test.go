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

// catalogue is a workstation with three engines, the second one being the
// workstation default engine.
func catalogue() Settings {
	return Settings{Engines: Engines{
		Catalogue: []Engine{
			{ID: "e-opus", Name: "Claude Opus", Provider: "claude", Model: "claude-opus-5", SkillModels: map[string]string{"implement": "claude-sonnet-5"}},
			{ID: "e-agy", Name: "Antigravity", Provider: "agy", Command: "agy --x {prompt}"},
			{ID: "e-codex", Name: "Codex", Provider: "codex", CommandAutonomous: "codex exec --y {prompt}"},
		},
		Default:  "e-agy",
		Projects: map[string]string{"p": "e-opus"},
		Tasks:    map[string]string{"t-switched": "e-codex", "t-dangling": "e-gone"},
	}}
}

func TestEngineResolutionFallsBack(t *testing.T) {
	s := catalogue()
	for _, tc := range []struct{ project, task, want string }{
		{"p", "t-switched", "e-codex"},
		{"p", "t-plain", "e-opus"},
		{"p", "t-dangling", "e-opus"},
		{"other", "t-plain", "e-agy"},
		{"other", "t-switched", "e-codex"},
	} {
		if got := s.TaskEngine(tc.project, tc.task).ID; got != tc.want {
			t.Errorf("%s/%s: got %s, want %s", tc.project, tc.task, got, tc.want)
		}
	}
	s.Engines.Projects["p"] = "e-gone"
	if got := s.ProjectEngine("p").ID; got != "e-agy" {
		t.Fatalf("a project pick naming no engine must read as none: %s", got)
	}
	s.Engines.Default = "e-gone"
	if got := s.DefaultEngine().ID; got != "e-opus" {
		t.Fatalf("without a valid default the first engine is the default: %s", got)
	}
	if got := (Settings{}).DefaultEngine(); got.Provider != DefaultProvider || got.ID != "" {
		t.Fatalf("a file without a catalogue runs the implicit engine: %+v", got)
	}
}

func TestResolveTaskRunsTheTaskEngine(t *testing.T) {
	s := catalogue()
	c := Config{ProjectID: "p"}
	project := Resolve(c, s)
	if project.AIProvider != "claude" || project.AIModel != "claude-opus-5" || ResolveModel(project, "implement") != "claude-sonnet-5" ||
		project.EngineID != "e-opus" || project.OffProjectDefaultEngine {
		t.Fatalf("project default engine: %+v", project)
	}
	task := ResolveTask(c, s, "t-switched")
	// Nothing is inherited from another engine: no model, no per-skill model.
	if task.AIProvider != "codex" || task.AIModel != "" || task.AISkillModels != nil || task.AICommandTemplate != "" ||
		task.AICommandTemplateAutonomous != "codex exec --y {prompt}" || task.EngineName != "Codex" || !task.OffProjectDefaultEngine {
		t.Fatalf("task engine: %+v", task)
	}
	if plain := ResolveTask(c, s, "t-plain"); plain.EngineID != "e-opus" || plain.OffProjectDefaultEngine {
		t.Fatalf("a task never switched runs its project default engine: %+v", plain)
	}
	// A switch to the project default engine is on it.
	s.SetTaskEngine("t-same", "e-opus")
	if same := ResolveTask(c, s, "t-same"); same.OffProjectDefaultEngine {
		t.Fatal("a task switched to its project default engine is on it")
	}
	// A removed engine falls back to the project default engine.
	if err := s.ReplaceCatalogue([]Engine{s.Engines.Catalogue[0], s.Engines.Catalogue[1]}, "e-agy"); err != nil {
		t.Fatal(err)
	}
	if got := ResolveTask(c, s, "t-switched"); got.EngineID != "e-opus" {
		t.Fatalf("removed task engine: %+v", got)
	}
}

func TestSetupProvidersCoverTheCatalogue(t *testing.T) {
	s := catalogue()
	s.Engines.Catalogue = append(s.Engines.Catalogue,
		Engine{ID: "e-custom", Name: "Wrapper", Provider: "custom", Command: "wrap {prompt}"},
		Engine{ID: "e-gemini", Name: "Gemini", Provider: "gemini"})
	s.Defaults.SetupProviders = []string{"codex"}
	got := Resolve(Config{ProjectID: "p"}, s)
	// The configured list first, then the catalogue providers that take skills;
	// the running provider (claude) is set up anyway.
	if !reflect.DeepEqual(got.SetupProviders, []string{"codex", "agy"}) {
		t.Fatalf("setup providers: %v", got.SetupProviders)
	}
	providers, err := SetupProviders(got)
	if err != nil || !reflect.DeepEqual(providers, []string{"claude", "codex", "agy"}) {
		t.Fatalf("providers set up: %v %v", providers, err)
	}
	// "None" configured is still extended by the catalogue.
	s.ProjectSettings = map[string]ProjectSettings{"p": {Execution: Execution{SetupProviders: []string{}}}}
	if got := Resolve(Config{ProjectID: "p"}, s); !reflect.DeepEqual(got.SetupProviders, []string{"agy", "codex"}) {
		t.Fatalf("setup providers over none: %v", got.SetupProviders)
	}
	// A single engine adds nothing.
	if got := Resolve(Config{ProjectID: "p"}, Settings{}); got.SetupProviders != nil {
		t.Fatalf("single engine: %v", got.SetupProviders)
	}
}

func TestReplaceCatalogue(t *testing.T) {
	s := catalogue()
	list := append([]Engine{}, s.Engines.Catalogue...)
	list[0].Name, list[0].Provider = "  Opus renamed ", "codex"
	list = append(list, Engine{Name: "Sonnet", Provider: "claude", Model: "claude-sonnet-5"})
	if err := s.ReplaceCatalogue(list, "e-agy"); err != nil {
		t.Fatal(err)
	}
	added := s.Engines.Catalogue[3]
	if !strings.HasPrefix(added.ID, "e-") || len(added.ID) != 14 || s.Engines.Catalogue[0].Name != "Opus renamed" {
		t.Fatalf("new engine or edit: %+v", s.Engines.Catalogue)
	}
	// An edit keeps the identity: the project still points at it.
	if s.ProjectEngine("p").ID != "e-opus" || s.ProjectEngine("p").Provider != "codex" {
		t.Fatalf("edit lost the project pick: %+v", s.Engines)
	}
	// The dangling task choice is pruned on the way.
	if _, ok := s.Engines.Tasks["t-dangling"]; ok {
		t.Fatal("a dangling task choice survived")
	}
	// Removing an engine drops what points at it.
	if err := s.ReplaceCatalogue([]Engine{s.Engines.Catalogue[1], s.Engines.Catalogue[3]}, "e-agy"); err != nil {
		t.Fatal(err)
	}
	if s.Engines.Projects != nil || s.Engines.Tasks != nil {
		t.Fatalf("choices naming removed engines survived: %+v", s.Engines)
	}
	for name, tc := range map[string]struct {
		list []Engine
		def  string
	}{
		"missing default": {[]Engine{s.Engines.Catalogue[1]}, "e-gone"},
		"removed default": {[]Engine{s.Engines.Catalogue[1]}, "e-agy-other"},
		"unknown id":      {[]Engine{{ID: "e-forged", Name: "X", Provider: "claude"}}, "e-forged"},
		"empty":           {nil, ""},
	} {
		before := s.Engines
		if err := s.ReplaceCatalogue(tc.list, tc.def); err == nil {
			t.Errorf("%s: accepted", name)
		}
		if !reflect.DeepEqual(s.Engines, before) {
			t.Errorf("%s: a refused catalogue changed the settings", name)
		}
	}
}

func TestValidateEngines(t *testing.T) {
	ok := Engine{ID: "e-1", Name: "Claude", Provider: "claude"}
	many := make([]Engine, MaxEngines+1)
	for i := range many {
		many[i] = Engine{ID: "e-" + strings.Repeat("x", i+1), Name: strings.Repeat("n", i+1), Provider: "claude"}
	}
	for name, tc := range map[string]struct {
		engines Engines
		want    string
	}{
		"none":           {Engines{}, "at least one"},
		"too many":       {Engines{Catalogue: many, Default: many[0].ID}, "at most"},
		"no name":        {Engines{Catalogue: []Engine{{ID: "e-1", Provider: "claude"}}, Default: "e-1"}, "name"},
		"long name":      {Engines{Catalogue: []Engine{{ID: "e-1", Name: strings.Repeat("n", MaxEngineName+1), Provider: "claude"}}, Default: "e-1"}, "longer than"},
		"duplicate name": {Engines{Catalogue: []Engine{ok, {ID: "e-2", Name: " claude ", Provider: "codex"}}, Default: "e-1"}, `engine "claude": the name is already used by engine "Claude"`},
		"duplicate id":   {Engines{Catalogue: []Engine{ok, {ID: "e-1", Name: "Other", Provider: "codex"}}, Default: "e-1"}, "duplicate identifier"},
		"no provider":    {Engines{Catalogue: []Engine{{ID: "e-1", Name: "X"}}, Default: "e-1"}, "provider is required"},
		"bad provider":   {Engines{Catalogue: []Engine{{ID: "e-1", Name: "X", Provider: "nope"}}, Default: "e-1"}, `engine "X"`},
		"custom empty":   {Engines{Catalogue: []Engine{{ID: "e-1", Name: "Wrap", Provider: "custom"}}, Default: "e-1"}, `engine "Wrap": the custom provider`},
		"custom prompt":  {Engines{Catalogue: []Engine{{ID: "e-1", Name: "Wrap", Provider: "custom", Command: "wrap it"}}, Default: "e-1"}, `engine "Wrap"`},
		"bad model":      {Engines{Catalogue: []Engine{{ID: "e-1", Name: "Opus", Provider: "claude", Model: "a b"}}, Default: "e-1"}, `engine "Opus"`},
		"bad skill":      {Engines{Catalogue: []Engine{{ID: "e-1", Name: "Opus", Provider: "claude", SkillModels: map[string]string{"implement": "a;b"}}}, Default: "e-1"}, `engine "Opus"`},
		"long template":  {Engines{Catalogue: []Engine{{ID: "e-1", Name: "Opus", Provider: "claude", Command: strings.Repeat("x", MaxCommandLength+1)}}, Default: "e-1"}, `engine "Opus"`},
		"no default":     {Engines{Catalogue: []Engine{ok}}, "default engine"},
	} {
		err := ValidateEngines(tc.engines)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want %q", name, err, tc.want)
		}
	}
	// Two engines with the same provider are told apart by their names.
	same := Engines{Catalogue: []Engine{ok, {ID: "e-2", Name: "Claude Sonnet", Provider: "claude", Model: "claude-sonnet-5"},
		{ID: "e-3", Name: "Wrap", Provider: "custom", Command: "wrap {prompt}"}}, Default: "e-2"}
	if err := ValidateEngines(same); err != nil {
		t.Fatal(err)
	}
}

func TestEnginesRoundTripThroughTheFile(t *testing.T) {
	testhome.Temp(t)
	s := catalogue()
	if err := WriteSettings(s); err != nil {
		t.Fatal(err)
	}
	got, err := ReadSettings(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.Engines.Tasks = map[string]string{"t-switched": "e-codex"} // the dangling one is pruned on write
	if !reflect.DeepEqual(got.Engines, s.Engines) {
		t.Fatalf("round trip:\n got  %+v\n want %+v", got.Engines, s.Engines)
	}
	// An emptied map leaves the file.
	got.SetTaskEngine("t-switched", "")
	if err := WriteSettings(got); err != nil {
		t.Fatal(err)
	}
	path, _ := SettingsPath()
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), `"tasks"`) || strings.Contains(string(raw), "t-switched") {
		t.Fatalf("an emptied task map stayed in the file:\n%s", raw)
	}
	// A file saved for the first time states the engine it runs.
	os.Remove(path)
	if err := WriteSettings(Settings{}); err != nil {
		t.Fatal(err)
	}
	fresh, _ := ReadSettings(t.TempDir())
	if len(fresh.Engines.Catalogue) != 1 || fresh.Engines.Default != implicitEngineID || fresh.DefaultEngine().Provider != DefaultProvider {
		t.Fatalf("first save: %+v", fresh.Engines)
	}
}

// An agent that predates #510 replaces the keys it owns and keeps the others:
// the catalogue survives it, and the engine fields its desktop wrote again are
// converted on the next read, reusing identical entries (US6.9).
func TestEnginesSurviveAnOlderAgentSave(t *testing.T) {
	testhome.Temp(t)
	s := catalogue()
	s.ProjectSettings = map[string]ProjectSettings{"p": {Path: "/repo"}}
	if err := WriteSettings(s); err != nil {
		t.Fatal(err)
	}
	path, _ := SettingsPath()
	raw, _ := os.ReadFile(path)
	var fields map[string]json.RawMessage
	json.Unmarshal(raw, &fields)
	fields["layout"] = json.RawMessage(`2`)
	fields["defaults"] = json.RawMessage(`{"aiProvider":"codex","aiCommandTemplateAutonomous":"codex exec --y {prompt}"}`)
	fields["projectSettings"] = json.RawMessage(`{"p":{"path":"/repo"},"q":{"path":"/q","aiProvider":"claude","aiModel":"claude-haiku-4-5"}}`)
	raw, _ = json.Marshal(fields)
	os.WriteFile(path, raw, 0600)

	got, err := ReadSettings(filepath.Join(t.TempDir(), "none"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Engines.Default != "e-agy" || got.Engines.Projects["p"] != "e-opus" || got.Engines.Tasks["t-switched"] != "e-codex" {
		t.Fatalf("the older agent's save lost the choices: %+v", got.Engines)
	}
	// The workstation profile is the existing Codex engine; q gets a new one.
	if len(got.Engines.Catalogue) != 4 || got.ProjectEngine("q").Provider != "claude" || got.ProjectEngine("q").Model != "claude-haiku-4-5" {
		t.Fatalf("conversion of the older agent's fields: %+v", got.Engines)
	}
	if got.Defaults.StatesEngine() || got.Project("q").StatesEngine() {
		t.Fatalf("engine fields survived the conversion: %+v", got)
	}
}
