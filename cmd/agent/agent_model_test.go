package main

import (
	"strings"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
)

// A provider that takes a model gets the flag; one that does not is launched
// exactly as before, which is what keeps agy and vibe working.
func TestAgentCommandLineModelFlag(t *testing.T) {
	cases := map[string]string{
		"claude": "claude --model M 'do it'",
		"codex":  "codex --model M 'do it'",
		"gemini": "gemini --model M 'do it'",
		"cursor": "cursor agent --model M 'do it'",
		"agy":    "agy -i 'do it'",
		"vibe":   "vibe -p 'do it'",
	}
	for provider, want := range cases {
		got, err := agentCommandLine(provider, "", "M", "do it")
		if err != nil {
			t.Fatalf("%s: %v", provider, err)
		}
		if got != want {
			t.Fatalf("%s built %q, want %q", provider, got, want)
		}
	}
}

// The regression that matters: an unconfigured model must reproduce the command
// lines Sectile built before model selection existed.
func TestAgentCommandLineWithoutModelIsUnchanged(t *testing.T) {
	cases := map[string]string{
		"claude": "claude 'do it'",
		"codex":  "codex 'do it'",
		"gemini": "gemini 'do it'",
		"cursor": "cursor agent 'do it'",
		"agy":    "agy -i 'do it'",
		"vibe":   "vibe -p 'do it'",
	}
	for provider, want := range cases {
		got, err := agentCommandLine(provider, "", "", "do it")
		if err != nil {
			t.Fatalf("%s: %v", provider, err)
		}
		if got != want {
			t.Fatalf("%s built %q, want %q", provider, got, want)
		}
	}
}

// A template owns the command line: no flag is spliced in, and {model} is the
// only way the model reaches it.
func TestAgentCommandLineTemplateOwnsTheModel(t *testing.T) {
	got, err := agentCommandLine("claude", `claude -p "{prompt}"`, "M", "do it")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "--model") {
		t.Fatalf("template must not receive an injected flag: %q", got)
	}

	got, err = agentCommandLine("custom", `my-cli --model {model} -p "{prompt}"`, "M", "do it")
	if err != nil {
		t.Fatal(err)
	}
	// expandAgentTemplate quotes every substituted value, which is exactly what
	// keeps a model identifier from changing the meaning of the command line.
	if !strings.Contains(got, "--model 'M'") {
		t.Fatalf("{model} not substituted: %q", got)
	}

	got, err = agentCommandLine("custom", `my-cli --model {model} -p "{prompt}"`, "", "do it")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "{model}") {
		t.Fatalf("unconfigured model must erase the placeholder: %q", got)
	}
}

// dispatchCommand is where a skill picks up its own model.
func TestDispatchCommandResolvesPerSkillModel(t *testing.T) {
	config := agentconfig.Config{
		AIProvider:    "claude",
		AIModel:       "base",
		AISkillModels: map[string]string{"implement": "strong"},
		Skills: []agentconfig.Skill{
			{ID: "implement", Directory: "code-issue", Command: "/code-issue"},
			{ID: "clarify", Directory: "clarify-issue", Command: "/clarify-issue"},
		},
	}
	line, err := dispatchCommand(config, "TASK-1", "implement", "execute_skill", "", "", models.SkillModeInteractive)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(line, "--model strong") {
		t.Fatalf("implement must run against its own model: %q", line)
	}
	line, err = dispatchCommand(config, "TASK-1", "clarify", "execute_skill", "", "", models.SkillModeInteractive)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(line, "--model base") {
		t.Fatalf("clarify must inherit the project model: %q", line)
	}
}

// An autonomous run builds its line through headlessCommandLine, a path the
// interactive builders never touch. The model has to survive it too, or every
// unattended run silently falls back to the CLI default.
func TestHeadlessCommandLineCarriesTheModel(t *testing.T) {
	for provider, want := range map[string]string{
		"claude": "claude -p --permission-mode bypassPermissions --model M 'do it'",
		"codex":  "codex exec --model M 'do it'",
		"vibe":   "vibe -p --auto-approve 'do it'",
	} {
		got, err := modeCommandLine(provider, "", "M", "do it", models.SkillModeAutonomous)
		if err != nil {
			t.Fatalf("%s: %v", provider, err)
		}
		if got != want {
			t.Fatalf("%s: got %q, want %q", provider, got, want)
		}
	}
}

// With no model configured an autonomous line is byte for byte the one built
// before this change.
func TestHeadlessCommandLineUnchangedWithoutModel(t *testing.T) {
	for provider, want := range map[string]string{
		"claude": "claude -p --permission-mode bypassPermissions 'do it'",
		"codex":  "codex exec 'do it'",
		"vibe":   "vibe -p --auto-approve 'do it'",
	} {
		got, err := modeCommandLine(provider, "", "", "do it", models.SkillModeAutonomous)
		if err != nil {
			t.Fatalf("%s: %v", provider, err)
		}
		if got != want {
			t.Fatalf("%s: got %q, want %q", provider, got, want)
		}
	}
}
