package main

import (
	"strings"
	"testing"

	"tasks/internal/models"
)

// The interactive command lines are the ones the tool emitted before an
// execution mode existed. They must not move.
func TestInteractiveCommandLineIsUnchanged(t *testing.T) {
	cases := map[string]string{
		"agy":    "agy -i 'do the thing'",
		"claude": "claude 'do the thing'",
		"codex":  "codex 'do the thing'",
		"gemini": "gemini 'do the thing'",
		"vibe":   "vibe -p 'do the thing'",
		"cursor": "cursor agent 'do the thing'",
	}
	for provider, want := range cases {
		got, err := modeCommandLine(provider, "", "do the thing", models.SkillModeInteractive)
		if err != nil {
			t.Fatalf("%s: unexpected error %v", provider, err)
		}
		if got != want {
			t.Fatalf("%s: got %q, want %q", provider, got, want)
		}
	}
}

func TestHeadlessCommandLineCoversAttestedProviders(t *testing.T) {
	cases := map[string]string{
		"claude": "claude -p 'do the thing'",
		"codex":  "codex exec 'do the thing'",
		"vibe":   "vibe -p 'do the thing'",
	}
	for provider, want := range cases {
		got, err := modeCommandLine(provider, "", "do the thing", models.SkillModeAutonomous)
		if err != nil {
			t.Fatalf("%s: unexpected error %v", provider, err)
		}
		if got != want {
			t.Fatalf("%s: got %q, want %q", provider, got, want)
		}
	}
}

// A provider with no attested headless invocation is refused by name. Falling
// back to the interactive form would open a window inside a run nobody is
// watching, which is the failure the refusal exists to prevent.
func TestAutonomousLaunchRefusesUnsupportedProvider(t *testing.T) {
	for _, provider := range []string{"agy", "gemini", "cursor", "unknown"} {
		line, err := modeCommandLine(provider, "", "do the thing", models.SkillModeAutonomous)
		if err == nil {
			t.Fatalf("%s: expected a refusal, got command %q", provider, line)
		}
		if !strings.Contains(err.Error(), provider) {
			t.Fatalf("%s: refusal does not name the provider: %v", provider, err)
		}
		if line != "" {
			t.Fatalf("%s: a refused launch must not produce a command, got %q", provider, line)
		}
	}
}

func TestAutonomousLaunchRefusesTemplateWithoutModePlaceholder(t *testing.T) {
	line, err := modeCommandLine("claude", "agy -i '{prompt}'", "do the thing", models.SkillModeAutonomous)
	if err == nil {
		t.Fatalf("expected a refusal, got command %q", line)
	}
	if !strings.Contains(err.Error(), "template") {
		t.Fatalf("refusal should say the template decides the mode: %v", err)
	}
}

// A template carrying the placeholder owns the mode, and keeps winning over the
// provider defaults in both directions.
func TestTemplateModePlaceholderSelectsTheSide(t *testing.T) {
	template := "agy {mode:-p|-i} '{prompt}'"
	autonomous, err := modeCommandLine("agy", template, "do the thing", models.SkillModeAutonomous)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if autonomous != "agy -p 'do the thing'" {
		t.Fatalf("autonomous: got %q", autonomous)
	}
	interactive, err := modeCommandLine("agy", template, "do the thing", models.SkillModeInteractive)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if interactive != "agy -i 'do the thing'" {
		t.Fatalf("interactive: got %q", interactive)
	}
}

func TestResolveTemplateMode(t *testing.T) {
	cases := []struct {
		template    string
		autonomous  string
		interactive string
	}{
		{"agy {mode:-p|-i} x", "agy -p x", "agy -i x"},
		{"cli {mode:--headless|} run", "cli --headless run", "cli  run"},
		{"cli {mode:a|b} {mode:c|d}", "cli a c", "cli b d"},
		{"cli no placeholder", "cli no placeholder", "cli no placeholder"},
		{"cli {mode:unterminated", "cli {mode:unterminated", "cli {mode:unterminated"},
	}
	for _, tc := range cases {
		if got := resolveTemplateMode(tc.template, true); got != tc.autonomous {
			t.Fatalf("autonomous %q: got %q, want %q", tc.template, got, tc.autonomous)
		}
		if got := resolveTemplateMode(tc.template, false); got != tc.interactive {
			t.Fatalf("interactive %q: got %q, want %q", tc.template, got, tc.interactive)
		}
	}
}

func TestTemplateCarriesMode(t *testing.T) {
	if templateCarriesMode("agy -i '{prompt}'") {
		t.Fatal("a template with no placeholder must not claim the mode")
	}
	if !templateCarriesMode("agy {mode:-p|-i} '{prompt}'") {
		t.Fatal("a template with the placeholder owns the mode")
	}
}

// An empty mode is what an older server sends. It must read as interactive
// rather than refusing or running headless.
func TestEmptyModeReadsAsInteractive(t *testing.T) {
	got, err := modeCommandLine("agy", "", "do the thing", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "agy -i 'do the thing'" {
		t.Fatalf("got %q", got)
	}
}
