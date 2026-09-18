package agentconfig

import "testing"

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
	// Emptying a list is a decision that must survive: dropping the key would
	// read as "never configured", which falls back to the shipped list.
	if kept, ok := got["empty"]; !ok || len(kept) != 0 {
		t.Fatalf("an emptied provider must keep an empty entry: %v (present=%v)", kept, ok)
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
