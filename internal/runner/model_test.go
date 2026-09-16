package runner_test

import (
	"strings"
	"testing"

	"tasks/internal/models"
	"tasks/internal/runner"
)

func TestPrepareAIResolvesTheSkillModel(t *testing.T) {
	settings := &models.Settings{
		AIProvider:    "claude",
		RepoPath:      t.TempDir(),
		AIModel:       "base",
		AISkillModels: map[string]string{"implement": "strong"},
	}
	r := runner.NewRunner()

	inv, err := r.PrepareAI(settings, "implement", &models.Task{Key: "TEST-1"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if inv.Model != "strong" {
		t.Fatalf("implement must carry its own model, got %q", inv.Model)
	}
	if !hasStepContaining(inv.Steps, "CLAUDE (strong)") {
		t.Fatalf("the engine step must report the resolved model: %v", inv.Steps)
	}

	inv, err = r.PrepareAI(settings, "clarify", &models.Task{Key: "TEST-1"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if inv.Model != "base" {
		t.Fatalf("clarify must inherit the configured model, got %q", inv.Model)
	}
}

func TestPrepareAIWithoutModelReportsTheEngineAlone(t *testing.T) {
	settings := &models.Settings{AIProvider: "claude", RepoPath: t.TempDir()}
	inv, err := runner.NewRunner().PrepareAI(settings, "clarify", &models.Task{Key: "TEST-1"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if inv.Model != "" {
		t.Fatalf("nothing configured must resolve nothing, got %q", inv.Model)
	}
	if !hasStepContaining(inv.Steps, "🤖 Moteur IA : CLAUDE") || hasStepContaining(inv.Steps, "CLAUDE (") {
		t.Fatalf("the engine step must name the engine alone: %v", inv.Steps)
	}
}

// The session command line is what an attached terminal actually runs.
func TestSessionCommandLineCarriesTheModel(t *testing.T) {
	r := runner.NewRunner()

	line, cleanup, err := r.SessionCommandLine(&runner.AIInvocation{Provider: "claude", Model: "M", Prompt: "do it"})
	cleanup()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(line, "--model M") {
		t.Fatalf("model missing from the session command: %q", line)
	}

	line, cleanup, err = r.SessionCommandLine(&runner.AIInvocation{Provider: "agy", Model: "M", Prompt: "do it"})
	cleanup()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(line, "--model") {
		t.Fatalf("agy takes no model flag: %q", line)
	}

	line, cleanup, err = r.SessionCommandLine(&runner.AIInvocation{Provider: "claude", Prompt: "do it"})
	cleanup()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(line, "--model") || strings.Contains(line, "  ") {
		t.Fatalf("no model configured must leave the command untouched: %q", line)
	}
}

// A template keeps the command line it declares, and only its {model} slot.
func TestSessionCommandLineTemplateOwnsTheModel(t *testing.T) {
	r := runner.NewRunner()

	line, cleanup, err := r.SessionCommandLine(&runner.AIInvocation{
		Provider: "custom", Template: `my-cli --model {model} -p "{prompt}"`, Model: "M", Prompt: "do it",
	})
	cleanup()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(line, "--model M") || strings.Contains(line, "{model}") {
		t.Fatalf("{model} not substituted: %q", line)
	}

	line, cleanup, err = r.SessionCommandLine(&runner.AIInvocation{
		Provider: "custom", Template: `my-cli --model {model} -p "{prompt}"`, Prompt: "do it",
	})
	cleanup()
	if err != nil {
		t.Fatal(err)
	}
	// An unresolved slot takes its option with it; a dangling --model would eat
	// the -p that follows and the prompt would become a positional argument.
	if strings.Contains(line, "--model") || strings.Contains(line, "{model}") {
		t.Fatalf("unconfigured model must take its option away: %q", line)
	}
	if !strings.HasPrefix(line, "my-cli -p ") {
		t.Fatalf("the prompt flag must keep the prompt as its value: %q", line)
	}
}

func hasStepContaining(steps []string, needle string) bool {
	for _, step := range steps {
		if strings.Contains(step, needle) {
			return true
		}
	}
	return false
}
