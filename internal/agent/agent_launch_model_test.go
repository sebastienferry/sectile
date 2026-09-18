package agent

import (
	"strings"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
)

func launchConfig() agentconfig.Config {
	return agentconfig.Config{
		AIProvider:    "claude",
		AIModel:       "claude-sonnet-5",
		AISkillModels: map[string]string{"implement": "claude-haiku-4-5"},
		Skills: []agentconfig.Skill{
			{ID: "implement", Directory: "code-issue", Command: "code-issue"},
			{ID: "clarify", Directory: "clarify-issue", Command: "clarify-issue"},
		},
	}
}

// The launch names one run, which is more specific than any configured level,
// so it outranks the per-skill entry rather than being merged under it.
func TestLaunchModelOutranksEveryConfiguredLevel(t *testing.T) {
	config := launchConfig()
	got, err := LaunchModel(config, "implement", "claude-opus-5")
	if err != nil {
		t.Fatal(err)
	}
	if got != "claude-opus-5" {
		t.Fatalf("launch override lost to a per-skill entry: %q", got)
	}
	// The workstation override has already been folded into the config the
	// agent holds, so outranking the config is outranking it too.
	local := config
	local.AIModel = "workstation-model"
	if got, err = LaunchModel(local, "clarify", "claude-opus-5"); err != nil || got != "claude-opus-5" {
		t.Fatalf("launch override lost to the workstation model: %q (%v)", got, err)
	}
}

// Without an override the resolution is exactly the one that predates this
// capability, which is what keeps an untouched launch byte for byte identical.
func TestLaunchModelWithoutOverrideResolvesTheConfiguredLevels(t *testing.T) {
	config := launchConfig()
	for skill, want := range map[string]string{"implement": "claude-haiku-4-5", "clarify": "claude-sonnet-5"} {
		got, err := LaunchModel(config, skill, "")
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("%s resolved %q, want %q", skill, got, want)
		}
	}
	if got, err := LaunchModel(agentconfig.Config{}, "implement", "  "); err != nil || got != "" {
		t.Fatalf("nothing configured and nothing chosen must resolve nothing: %q (%v)", got, err)
	}
}

// The agent is the process that runs the command through sh -c, so it refuses a
// malformed identifier itself rather than trusting the server's check.
func TestLaunchModelRefusesAMalformedOverride(t *testing.T) {
	if _, err := LaunchModel(launchConfig(), "implement", "claude-opus-5; rm -rf ~"); err == nil {
		t.Fatal("an override carrying shell metacharacters must be refused")
	}
	line, err := dispatchCommand(launchConfig(), "#203", "implement", "execute_skill", "", "", models.SkillModeInteractive, "claude-opus-5 --dangerous")
	if err == nil {
		t.Fatalf("dispatch must refuse a malformed model, built %q", line)
	}
	if line != "" {
		t.Fatalf("no command line may be built for a refused model: %q", line)
	}
}

// The override reaches the command line the same way a configured model does,
// through the flag for a provider that takes one.
func TestDispatchCommandCarriesTheLaunchModel(t *testing.T) {
	line, err := dispatchCommand(launchConfig(), "#203", "implement", "execute_skill", "", "", models.SkillModeInteractive, "claude-opus-5")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(line, "--model claude-opus-5") {
		t.Fatalf("launch model missing from the command line: %q", line)
	}
	if strings.Contains(line, "claude-haiku-4-5") {
		t.Fatalf("the per-skill model must not survive the override: %q", line)
	}

	// No override: the configured per-skill model is what runs, as before.
	line, err = dispatchCommand(launchConfig(), "#203", "implement", "execute_skill", "", "", models.SkillModeInteractive, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(line, "--model claude-haiku-4-5") {
		t.Fatalf("configured model lost when no override is given: %q", line)
	}
}

// An autonomous launch carries the override too: the mode and the model are
// independent, and a headless run is where a stronger model matters most.
func TestDispatchCommandCarriesTheLaunchModelHeadless(t *testing.T) {
	config := launchConfig()
	line, err := dispatchCommand(config, "#203", "implement", "execute_skill", "", "", models.SkillModeAutonomous, "claude-opus-5")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(line, "--model claude-opus-5") {
		t.Fatalf("launch model missing from the headless command line: %q", line)
	}

	// With a dedicated autonomous template, the model reaches the line through
	// its {model} slot and nowhere else.
	templated := launchConfig()
	templated.AICommandTemplateAutonomous = `claude -p --model {model} "{prompt}"`
	line, err = dispatchCommand(templated, "#203", "implement", "execute_skill", "", "", models.SkillModeAutonomous, "claude-opus-5")
	if err != nil {
		t.Fatal(err)
	}
	// The template owns its own quoting, so the model is asserted on its value
	// rather than on a bare flag pair.
	if !strings.Contains(line, "claude-opus-5") || strings.Contains(line, "{model}") {
		t.Fatalf("the autonomous template must carry the launch model: %q", line)
	}
}

// A discussion resolves the project model and never reads the override: it is
// not a skill run, and its launch surface offers no model.
func TestDispatchCommandIgnoresTheOverrideForADiscussion(t *testing.T) {
	line, err := dispatchCommand(launchConfig(), "#203", "discuss", "discuss", "", "", models.SkillModeInteractive, "claude-opus-5")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(line, "claude-opus-5") {
		t.Fatalf("a discussion must ignore the launch model: %q", line)
	}
}

// What a run reports is what actually reached the command line, so a provider
// that takes no model and a template without the slot report no model at all.
func TestLaunchEngineReportsWhatReachedTheLine(t *testing.T) {
	provider, model := launchEngine(launchConfig(), "implement", "claude-opus-5", models.SkillModeInteractive)
	if provider != "claude" || model != "claude-opus-5" {
		t.Fatalf("engine reported %q/%q", provider, model)
	}

	flagless := launchConfig()
	flagless.AIProvider = "agy"
	if provider, model = launchEngine(flagless, "implement", "claude-opus-5", models.SkillModeInteractive); provider != "agy" || model != "" {
		t.Fatalf("a provider taking no model must report none: %q/%q", provider, model)
	}

	templated := launchConfig()
	templated.AICommandTemplate = `claude -p "{prompt}"`
	if _, model = launchEngine(templated, "implement", "claude-opus-5", models.SkillModeInteractive); model != "" {
		t.Fatalf("a template without a {model} slot must report no model: %q", model)
	}

	// The autonomous command is the one that runs in autonomous mode, so it is
	// the one that decides whether the model reaches the line.
	split := launchConfig()
	split.AICommandTemplate = `claude -p "{prompt}"`
	split.AICommandTemplateAutonomous = `claude -p --model {model} "{prompt}"`
	if _, model = launchEngine(split, "implement", "claude-opus-5", models.SkillModeAutonomous); model != "claude-opus-5" {
		t.Fatalf("the autonomous template carries the model: %q", model)
	}
	if _, model = launchEngine(split, "implement", "claude-opus-5", models.SkillModeInteractive); model != "" {
		t.Fatalf("the interactive template has no slot, so it reports no model: %q", model)
	}
}
