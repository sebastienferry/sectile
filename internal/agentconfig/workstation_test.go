package agentconfig

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func boolPtr(v bool) *bool { return &v }

// converted is a layout-2 fixture as ReadSettings hands it over: the engine
// settings of #305 converted into the catalogue (#510). It works on a copy, so
// a test can edit its fixture and convert it again.
func converted(s Settings) Settings {
	raw, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	var out Settings
	if err := json.Unmarshal(raw, &out); err != nil {
		panic(err)
	}
	convertEngines(&out)
	return out
}

func TestResolveIgnoresServerExecutionValues(t *testing.T) {
	// An older server still sends execution fields: none of them may survive.
	server := Config{
		ProjectID: "p", AIProvider: "codex", AICommandTemplate: "codex {prompt}", AICommandTemplateAutonomous: "codex exec {prompt}",
		AIModel: "gpt-5", AISkillModels: map[string]string{"implement": "o4-mini"}, ExternalTerminalCommand: "iterm",
		UseWorktrees: false, SetupProviders: []string{"claude"},
	}
	got := Resolve(server, Settings{})
	if got.AIProvider != DefaultProvider || got.AICommandTemplate != "" || got.AICommandTemplateAutonomous != "" ||
		got.AIModel != "" || got.AISkillModels != nil || got.ExternalTerminalCommand != "" || !got.UseWorktrees || got.SetupProviders != nil {
		t.Fatalf("a server execution value survived: %+v", got)
	}
}

func TestResolveDoesNotMutateContract(t *testing.T) {
	c := Config{Skills: []Skill{{ID: "implement", Content: "remote"}}}
	effective := Resolve(c, converted(Settings{Defaults: Defaults{Execution: Execution{AIProvider: "claude"}}, Skills: map[string]string{"implement": "local"}}))
	if c.Skills[0].Content != "remote" || effective.Skills[0].Content != "local" || effective.AIProvider != "claude" {
		t.Fatal("precedence or isolation failed")
	}
}

func TestResolveProjectOverDefaults(t *testing.T) {
	s := Settings{
		Defaults: Defaults{Execution: Execution{AIProvider: "gemini", AIModel: "gemini-pro", AICommandTemplate: "gemini --x {prompt}", AICommandTemplateAutonomous: "gemini -p {prompt}"}},
		ProjectSettings: map[string]ProjectSettings{
			"p1": {Execution: Execution{AIProvider: "claude", AIModel: "claude-opus-5"}},
		},
	}
	p1 := Resolve(Config{ProjectID: "p1"}, converted(s))
	if p1.AIProvider != "claude" || p1.AIModel != "claude-opus-5" {
		t.Fatalf("project section ignored: %+v", p1)
	}
	// A command written for gemini never serves claude.
	if p1.AICommandTemplate != "" || p1.AICommandTemplateAutonomous != "" {
		t.Fatalf("the inherited commands must be dropped on a provider change: %+v", p1)
	}
	p2 := Resolve(Config{ProjectID: "p2"}, converted(s))
	if p2.AIProvider != "gemini" || p2.AIModel != "gemini-pro" || p2.AICommandTemplate != "gemini --x {prompt}" || p2.AICommandTemplateAutonomous != "gemini -p {prompt}" {
		t.Fatalf("workstation defaults ignored: %+v", p2)
	}
	// The project's own command survives its provider change, each template independently.
	s.ProjectSettings["p1"] = ProjectSettings{Execution: Execution{AIProvider: "claude", AICommandTemplate: "my-claude {prompt}"}}
	own := Resolve(Config{ProjectID: "p1"}, converted(s))
	if own.AICommandTemplate != "my-claude {prompt}" || own.AICommandTemplateAutonomous != "" {
		t.Fatalf("project command lost or the other template kept: %+v", own)
	}
	// Same provider: the templates the project leaves empty are inherited.
	s.ProjectSettings["p1"] = ProjectSettings{Execution: Execution{AIProvider: "gemini", AICommandTemplateAutonomous: "gemini -p --y {prompt}"}}
	same := Resolve(Config{ProjectID: "p1"}, converted(s))
	if same.AICommandTemplate != "gemini --x {prompt}" || same.AICommandTemplateAutonomous != "gemini -p --y {prompt}" {
		t.Fatalf("same-provider inheritance broken: %+v", same)
	}
}

func TestResolveProviderChangeComparesAgainstTheDefaultProvider(t *testing.T) {
	// Defaults name no provider: their commands are for agy, which a project on claude cannot use.
	s := Settings{
		Defaults:        Defaults{Execution: Execution{AICommandTemplate: "agy --x {prompt}"}},
		ProjectSettings: map[string]ProjectSettings{"p": {Execution: Execution{AIProvider: "claude"}}},
	}
	if got := Resolve(Config{ProjectID: "p"}, converted(s)); got.AICommandTemplate != "" {
		t.Fatalf("an agy command reached claude: %q", got.AICommandTemplate)
	}
	s.ProjectSettings["p"] = ProjectSettings{Execution: Execution{AIProvider: "agy"}}
	if got := Resolve(Config{ProjectID: "p"}, converted(s)); got.AICommandTemplate != "agy --x {prompt}" {
		t.Fatalf("the same provider must inherit: %q", got.AICommandTemplate)
	}
}

func TestResolveDropsALegacyBareCLIName(t *testing.T) {
	got := Resolve(Config{ProjectID: "p"}, converted(Settings{Defaults: Defaults{Execution: Execution{AIProvider: "claude", AICommandTemplate: "claude"}}}))
	if got.AICommandTemplate != "" {
		t.Fatalf("a bare CLI name must not reach the runner: %q", got.AICommandTemplate)
	}
}

func TestResolveModelPrecedence(t *testing.T) {
	s := Settings{
		Defaults: Defaults{Execution: Execution{AIModel: "workstation", AISkillModels: map[string]string{"clarify": "workstation-clarify"}}},
		ProjectSettings: map[string]ProjectSettings{
			"p": {Execution: Execution{AIModel: "project", AISkillModels: map[string]string{"implement": "project-implement"}}},
		},
	}
	got := Resolve(Config{ProjectID: "p"}, converted(s))
	if ResolveModel(got, "implement") != "project-implement" {
		t.Fatal("project skill entry lost")
	}
	// A per-skill entry outranks a bare model whatever level the bare model sits on.
	if ResolveModel(got, "clarify") != "workstation-clarify" {
		t.Fatalf("a bare project model silenced the workstation skill entry: %q", ResolveModel(got, "clarify"))
	}
	if ResolveModel(got, "specify") != "project" {
		t.Fatal("project model must govern the skills no level singles out")
	}
	other := Resolve(Config{ProjectID: "other"}, converted(s))
	if ResolveModel(other, "specify") != "workstation" || ResolveModel(other, "implement") != "workstation" {
		t.Fatal("defaults ignored for a project without a section")
	}
}

func TestResolveWorktreesSetupProvidersAndTerminal(t *testing.T) {
	s := Settings{
		Defaults: Defaults{Execution: Execution{UseWorktrees: boolPtr(false), SetupProviders: []string{"codex", "unknown"}, Terminal: "ghostty"}},
		ProjectSettings: map[string]ProjectSettings{
			"on":   {Execution: Execution{UseWorktrees: boolPtr(true), SetupProviders: []string{}, Terminal: " iterm "}},
			"list": {Execution: Execution{SetupProviders: []string{"claude"}}},
		},
	}
	if got := Resolve(Config{ProjectID: "none"}, s); got.UseWorktrees || !reflect.DeepEqual(got.SetupProviders, []string{"codex"}) || got.ExternalTerminalCommand != "ghostty" {
		t.Fatalf("defaults not inherited: %+v", got)
	}
	// An empty project list is the decision "none", not "inherit".
	if got := Resolve(Config{ProjectID: "on"}, s); !got.UseWorktrees || got.SetupProviders != nil || got.ExternalTerminalCommand != "iterm" {
		t.Fatalf("project section ignored: %+v", got)
	}
	// The project list replaces the default one.
	if got := Resolve(Config{ProjectID: "list"}, s); !reflect.DeepEqual(got.SetupProviders, []string{"claude"}) {
		t.Fatalf("setup providers must be replaced: %v", got.SetupProviders)
	}
}

func TestResolveSkillCommands(t *testing.T) {
	c := Config{ProjectID: "p", Skills: []Skill{{ID: "implement", Command: "/code-issue"}, {ID: "clarify", Command: "/clarify-issue"}}}
	got := Resolve(c, Settings{ProjectSettings: map[string]ProjectSettings{"p": {SkillCommands: map[string]string{"implement": "build-it", "clarify": " "}}}})
	if got.Skills[0].Command != "build-it" || got.Skills[1].Command != "/clarify-issue" {
		t.Fatalf("skill command names: %+v", got.Skills)
	}
	if c.Skills[0].Command != "/code-issue" {
		t.Fatal("the contract was mutated")
	}
}

func TestExecutionLimitInheritance(t *testing.T) {
	s := Settings{
		Defaults: Defaults{Execution: Execution{Parallelism: 2}},
		ProjectSettings: map[string]ProjectSettings{
			"a": {Execution: Execution{Parallelism: 3}}, "high": {Execution: Execution{Parallelism: MaxParallelism + 4}}, "max": {Execution: Execution{Parallelism: MaxParallelism}},
		},
	}
	for id, want := range map[string]int{"a": 3, "b": 2, "high": MaxParallelism, "max": MaxParallelism} {
		if got := ExecutionLimit(id, true, s); got != want {
			t.Fatalf("%s: got %d, want %d", id, got, want)
		}
	}
	if ExecutionLimit("a", false, s) != 1 {
		t.Fatal("a shared checkout must be serialized")
	}
	if ExecutionLimit("b", true, Settings{}) != 1 {
		t.Fatal("without any setting a project runs one execution at a time")
	}
}

func TestProviderModelsEmptyKeyIsAChoice(t *testing.T) {
	d := Defaults{AIProviderModels: map[string][]string{"claude": {}, "codex": {"gpt-5"}}}
	if got := ProviderModels(d, "claude"); len(got) != 0 {
		t.Fatalf("an emptied list must offer nothing: %v", got)
	}
	if got := ProviderModels(d, "Codex"); !reflect.DeepEqual(got, []string{"gpt-5"}) {
		t.Fatalf("configured list ignored: %v", got)
	}
	if got := ProviderModels(d, "cursor"); !reflect.DeepEqual(got, DefaultProviderModels["cursor"]) {
		t.Fatalf("shipped list expected: %v", got)
	}
}

func TestValidateLevels(t *testing.T) {
	bad := []struct {
		name string
		err  error
	}{
		{"model", ValidateExecution(Execution{AIModel: "a b"})},
		{"custom without {prompt}", ValidateExecution(Execution{AIProvider: "custom", AICommandTemplate: "run it"})},
		{"parallelism", ValidateExecution(Execution{Parallelism: MaxParallelism + 1})},
		{"setup provider", ValidateExecution(Execution{SetupProviders: []string{"vim"}})},
		{"long command", ValidateExecution(Execution{AICommandTemplate: strings.Repeat("x", MaxCommandLength+1)})},
		{"provider", ValidateExecution(Execution{AIProvider: "nope"})},
		{"skill command", ValidateProject(ProjectSettings{SkillCommands: map[string]string{"implement": "two words"}})},
		{"provider models", ValidateDefaults(Defaults{AIProviderModels: map[string][]string{"claude": {"x;y"}}})},
		{"editor", ValidateDefaults(Defaults{EditorCommand: strings.Repeat("x", MaxCommandLength+1)})},
	}
	for _, c := range bad {
		if c.err == nil {
			t.Errorf("%s: accepted", c.name)
		}
	}
	if err := ValidateProject(ProjectSettings{SkillCommands: map[string]string{"implement": "/code-issue"}, Execution: Execution{Parallelism: 4}}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateExecution(Execution{AIProvider: "custom", AICommandTemplate: "x {prompt}"}); err != nil {
		t.Fatal(err)
	}
	// The engine settings of #305 live in the catalogue since #510.
	for name, err := range map[string]error{
		"defaults provider": ValidateDefaults(Defaults{Execution: Execution{AIProvider: "claude"}}),
		"defaults models":   ValidateDefaults(Defaults{Execution: Execution{AISkillModels: map[string]string{"implement": "m"}}}),
		"project template":  ValidateProject(ProjectSettings{Execution: Execution{AICommandTemplate: "x {prompt}"}}),
		"project model":     ValidateProject(ProjectSettings{Execution: Execution{AIModel: "m"}}),
	} {
		if !errors.Is(err, ErrEngineFields) {
			t.Errorf("%s: want ErrEngineFields, got %v", name, err)
		}
	}
}
