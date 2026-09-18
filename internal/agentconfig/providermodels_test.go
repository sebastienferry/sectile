package agentconfig

import "testing"

// A provider nobody configured still offers the models Sectile ships, otherwise
// a fresh install would show an empty list at launch.
func TestProviderModelsFallsBackToTheShippedList(t *testing.T) {
	if got := ProviderModels(nil, "claude"); len(got) == 0 {
		t.Fatal("an unconfigured provider must fall back to the shipped list")
	}
	if got := ProviderModels(map[string][]string{}, "gemini"); len(got) == 0 {
		t.Fatal("an empty configuration must fall back to the shipped list")
	}
	if got := ProviderModels(nil, "unknown-engine"); len(got) != 0 {
		t.Fatalf("a provider Sectile ships nothing for must offer nothing: %v", got)
	}
	if got := ProviderModels(nil, ""); got != nil {
		t.Fatalf("no provider must resolve no list: %v", got)
	}
}

// Configuring a provider replaces its list: a model removed in the interface has
// to disappear from the launch surfaces, which an additive merge would prevent.
func TestProviderModelsReplacesTheShippedList(t *testing.T) {
	configured := map[string][]string{"claude": {"claude-haiku-4-5"}}
	got := ProviderModels(configured, "claude")
	if len(got) != 1 || got[0] != "claude-haiku-4-5" {
		t.Fatalf("configured list must win outright: %v", got)
	}
	// Another provider keeps its own fallback rather than inheriting this one.
	if len(ProviderModels(configured, "codex")) == 0 {
		t.Fatal("configuring one provider must not empty another")
	}
}

// The shipped lists are copies: a caller that sorts or appends must not corrupt
// what the next caller reads.
func TestDefaultProviderModelsIsACopy(t *testing.T) {
	first := DefaultProviderModels()
	first["claude"][0] = "mutated"
	if got := DefaultProviderModels()["claude"][0]; got == "mutated" {
		t.Fatal("DefaultProviderModels handed out its own slice")
	}
	if got := ProviderModels(nil, "claude")[0]; got == "mutated" {
		t.Fatal("ProviderModels handed out the shipped slice")
	}
}

func TestNormalizeProviderModels(t *testing.T) {
	got := NormalizeProviderModels(map[string][]string{
		"  Claude ": {" claude-opus-5 ", "", "claude-opus-5", "claude-sonnet-5"},
		"  ":        {"orphan"},
		"empty":     {" "},
	})
	claude := got["claude"]
	if len(claude) != 2 || claude[0] != "claude-opus-5" || claude[1] != "claude-sonnet-5" {
		t.Fatalf("blank, duplicate and untrimmed entries survived: %v", claude)
	}
	if _, ok := got[""]; ok {
		t.Fatal("a blank provider key must be dropped")
	}
	if _, ok := got["empty"]; ok {
		t.Fatal("a provider left with no model must be dropped")
	}
	if NormalizeProviderModels(nil) != nil {
		t.Fatal("nothing configured must normalise to nothing")
	}
}

// The list is placed on a command line like any other model, so it is checked
// with the same rule, and the error has to name the provider: a rejected value
// is otherwise impossible to find in a map of lists.
func TestValidProviderModelsNamesTheProvider(t *testing.T) {
	if err := ValidProviderModels(map[string][]string{"claude": {"claude-opus-5", "anthropic/claude-sonnet-5"}}); err != nil {
		t.Fatalf("well-formed identifiers refused: %v", err)
	}
	err := ValidProviderModels(map[string][]string{"gemini": {"gemini-2.5-pro; rm -rf ~"}})
	if err == nil {
		t.Fatal("an identifier carrying shell metacharacters must be refused")
	}
	if want := "gemini"; !contains(err.Error(), want) {
		t.Fatalf("error %q does not name the provider %q", err, want)
	}
}

// A template carries the model only through its {model} slot, and a provider
// without a model flag runs without one: a run must not claim an engine the CLI
// never saw.
func TestEffectiveModel(t *testing.T) {
	cases := []struct {
		name                        string
		provider, template, resolve string
		want                        string
	}{
		{"provider with a flag", "claude", "", "claude-opus-5", "claude-opus-5"},
		{"provider without a flag", "agy", "", "claude-opus-5", ""},
		{"template with the slot", "claude", `claude --model {model} "{prompt}"`, "claude-opus-5", "claude-opus-5"},
		{"template without the slot", "claude", `claude "{prompt}"`, "claude-opus-5", ""},
		{"custom template without the slot", "custom", `my-cli --run`, "claude-opus-5", ""},
		{"nothing resolved", "claude", "", "", ""},
	}
	for _, c := range cases {
		if got := EffectiveModel(c.provider, c.template, c.resolve); got != c.want {
			t.Fatalf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
