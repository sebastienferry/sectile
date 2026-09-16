package agentconfig

import "testing"

func TestResolveSkillModelPrecedence(t *testing.T) {
	c := ModelConfig{Model: "base", SkillModels: map[string]string{"implement": "strong"}}
	if got := ResolveSkillModel(c, "implement"); got != "strong" {
		t.Fatalf("skill entry ignored: %q", got)
	}
	if got := ResolveSkillModel(c, "clarify"); got != "base" {
		t.Fatalf("skill without entry must inherit: %q", got)
	}
	if got := ResolveSkillModel(c, ""); got != "base" {
		t.Fatalf("no skill must resolve the level model: %q", got)
	}
	if got := ResolveSkillModel(ModelConfig{}, "implement"); got != "" {
		t.Fatalf("nothing configured must resolve nothing: %q", got)
	}
}

// A level speaks for every skill it does not single out, so a bare model on the
// more specific level outranks a per-skill entry on the less specific one.
func TestMergeModelsLevelByLevel(t *testing.T) {
	high := ModelConfig{Model: "project"}
	low := ModelConfig{Model: "global", SkillModels: map[string]string{"implement": "global-implement"}}
	merged := MergeModels(high, low)
	if got := ResolveSkillModel(merged, "implement"); got != "project" {
		t.Fatalf("bare project model must outrank a global skill entry: %q", got)
	}
	if got := ResolveSkillModel(merged, "clarify"); got != "project" {
		t.Fatalf("project model must apply to every other skill: %q", got)
	}
}

func TestMergeModelsInheritsWhenLevelIsEmpty(t *testing.T) {
	high := ModelConfig{SkillModels: map[string]string{"clarify": "cheap"}}
	low := ModelConfig{Model: "global", SkillModels: map[string]string{"implement": "global-implement"}}
	merged := MergeModels(high, low)
	for skill, want := range map[string]string{"clarify": "cheap", "implement": "global-implement", "specify": "global"} {
		if got := ResolveSkillModel(merged, skill); got != want {
			t.Fatalf("skill %s resolved %q, want %q", skill, got, want)
		}
	}
}

func TestMergeModelsKeepsNothingConfigured(t *testing.T) {
	merged := MergeModels(ModelConfig{}, ModelConfig{})
	if merged.Model != "" || len(merged.SkillModels) != 0 {
		t.Fatalf("empty levels must merge to nothing: %+v", merged)
	}
}

func TestValidModel(t *testing.T) {
	for _, value := range []string{"", "claude-opus-5", "gpt-5-codex", "gemini-2.5-pro", "anthropic/claude-sonnet-5", "org:model@v1"} {
		if err := ValidModel(value); err != nil {
			t.Fatalf("%q rejected: %v", value, err)
		}
	}
	for _, value := range []string{"claude opus", "model; rm -rf ~", "$(whoami)", "-p", "`id`", "a&b", "m|n"} {
		if err := ValidModel(value); err == nil {
			t.Fatalf("%q accepted", value)
		}
	}
}

func TestValidModelConfigNamesTheSkill(t *testing.T) {
	err := ValidModelConfig(ModelConfig{SkillModels: map[string]string{"implement": "bad model"}})
	if err == nil {
		t.Fatal("unsafe per-skill model accepted")
	}
	if got := err.Error(); got == "" || !contains(got, "implement") {
		t.Fatalf("error must name the skill: %v", err)
	}
}

func TestModelArgsPerProvider(t *testing.T) {
	for _, provider := range []string{"claude", "codex", "gemini", "cursor"} {
		args := ModelArgs(provider, "M")
		if len(args) != 2 || args[0] != "--model" || args[1] != "M" {
			t.Fatalf("%s: %v", provider, args)
		}
	}
	for _, provider := range []string{"agy", "vibe", "custom", ""} {
		if args := ModelArgs(provider, "M"); args != nil {
			t.Fatalf("%s must take no model flag: %v", provider, args)
		}
	}
	if args := ModelArgs("claude", "  "); args != nil {
		t.Fatalf("no model configured must add no flag: %v", args)
	}
}

func TestExpandModel(t *testing.T) {
	if got := ExpandModel(`claude --model {model} -p "{prompt}"`, "M"); got != `claude --model M -p "{prompt}"` {
		t.Fatalf("placeholder not substituted: %q", got)
	}
	template := `claude -p "{prompt}"`
	if got := ExpandModel(template, "M"); got != template {
		t.Fatalf("template without the slot must be untouched: %q", got)
	}
}

// An unresolved slot leaves with the option it belongs to. Erasing the marker
// alone left `--model  -p "…"`, where the CLI reads -p as the model name and the
// prompt degrades to a positional argument.
func TestExpandModelWithoutModelTakesItsOptionAway(t *testing.T) {
	for _, c := range []struct{ template, want string }{
		{`claude --model {model} -p "{prompt}"`, `claude -p "{prompt}"`},
		{`claude -m {model} -p "{prompt}"`, `claude -p "{prompt}"`},
		{`claude --model={model} -p "{prompt}"`, `claude -p "{prompt}"`},
		{`claude --model "{model}" -p "{prompt}"`, `claude -p "{prompt}"`},
		{`claude --model '{model}' -p "{prompt}"`, `claude -p "{prompt}"`},
		{`claude -p "{prompt}" --model {model}`, `claude -p "{prompt}"`},
		{`claude --model {model} --fallback {model} -p "{prompt}"`, `claude -p "{prompt}"`},
		// No option to carry away: the slot is positional and its neighbours stay.
		{`my-cli {model} run -p "{prompt}"`, `my-cli run -p "{prompt}"`},
		{`{model} -p "{prompt}"`, `-p "{prompt}"`},
		// The prompt is what the command line exists to carry, so a token holding
		// both markers keeps everything but the model marker.
		{`my-cli -p"{prompt}"{model}`, `my-cli -p"{prompt}"`},
	} {
		if got := ExpandModel(c.template, ""); got != c.want {
			t.Fatalf("%q expanded to %q, want %q", c.template, got, c.want)
		}
	}
	if got := ExpandModel(`claude --model {model} -p "{prompt}"`, "   "); got != `claude -p "{prompt}"` {
		t.Fatalf("a blank model must behave like no model: %q", got)
	}
}

func TestConfigValidateRejectsUnsafeModel(t *testing.T) {
	c := Config{SchemaVersion: Version, ProjectID: "p", AIProvider: "claude", AIModel: "claude; rm -rf ~"}
	if err := c.Validate(); err == nil {
		t.Fatal("unsafe model accepted by the contract")
	}
	c.AIModel = "claude-opus-5"
	if err := c.Validate(); err != nil {
		t.Fatalf("well-formed model rejected: %v", err)
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func TestApplyOverridesModel(t *testing.T) {
	c := Config{ProjectID: "p", AIModel: "project", AISkillModels: map[string]string{"implement": "project-implement"}}

	kept := ApplyOverrides(c, Overrides{})
	if ResolveModel(kept, "implement") != "project-implement" || ResolveModel(kept, "clarify") != "project" {
		t.Fatalf("no override must keep the server configuration: %+v", kept.Models())
	}

	overridden := ApplyOverrides(c, Overrides{AIModel: "workstation"})
	if got := ResolveModel(overridden, "implement"); got != "workstation" {
		t.Fatalf("workstation model must outrank the project skill entry: %q", got)
	}

	perSkill := ApplyOverrides(c, Overrides{AISkillModels: map[string]string{"clarify": "workstation-clarify"}})
	if got := ResolveModel(perSkill, "clarify"); got != "workstation-clarify" {
		t.Fatalf("workstation skill entry ignored: %q", got)
	}
	if got := ResolveModel(perSkill, "implement"); got != "project-implement" {
		t.Fatalf("a workstation skill entry must not erase the project's: %q", got)
	}
}
