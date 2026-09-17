package agent

import (
	"strings"
	"testing"

	"tasks/internal/agentconfig"
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
		got, err := modeCommandLine(provider, "", "", "do the thing", models.SkillModeInteractive)
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
		"claude": "claude -p --permission-mode bypassPermissions 'do the thing'",
		"codex":  "codex exec 'do the thing'",
		"vibe":   "vibe -p --auto-approve 'do the thing'",
	}
	for provider, want := range cases {
		got, err := modeCommandLine(provider, "", "", "do the thing", models.SkillModeAutonomous)
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
		line, err := modeCommandLine(provider, "", "", "do the thing", models.SkillModeAutonomous)
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
	line, err := modeCommandLine("claude", "agy -i '{prompt}'", "", "do the thing", models.SkillModeAutonomous)
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
	autonomous, err := modeCommandLine("agy", template, "", "do the thing", models.SkillModeAutonomous)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if autonomous != "agy -p 'do the thing'" {
		t.Fatalf("autonomous: got %q", autonomous)
	}
	interactive, err := modeCommandLine("agy", template, "", "do the thing", models.SkillModeInteractive)
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
	got, err := modeCommandLine("agy", "", "", "do the thing", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "agy -i 'do the thing'" {
		t.Fatalf("got %q", got)
	}
}

// A headless run has nobody to answer a permission prompt. Without the
// provider's non-interactive approval mode the CLI is denied every tool it asks
// for, the Sectile MCP tools included, so the run ends having printed why it
// could not work and the board never moves. This is the one thing the headless
// command line must carry beyond the prompt.
func TestHeadlessCommandLineCarriesApprovalMode(t *testing.T) {
	cases := map[string]string{
		"claude": "--permission-mode bypassPermissions",
		"vibe":   "--auto-approve",
	}
	for provider, flag := range cases {
		got, err := modeCommandLine(provider, "", "", "do the thing", models.SkillModeAutonomous)
		if err != nil {
			t.Fatalf("%s: unexpected error %v", provider, err)
		}
		if !strings.Contains(got, flag) {
			t.Fatalf("%s: headless launch does not carry %q: %q", provider, flag, got)
		}
	}
	// The interactive form is where a human answers, and must not bypass anything.
	for provider := range cases {
		got, err := modeCommandLine(provider, "", "", "do the thing", models.SkillModeInteractive)
		if err != nil {
			t.Fatalf("%s: unexpected error %v", provider, err)
		}
		if strings.Contains(got, "bypassPermissions") || strings.Contains(got, "--auto-approve") {
			t.Fatalf("%s: interactive launch bypasses permissions: %q", provider, got)
		}
	}
}

// A discussion opens a live provider session with no prompt of its own. Run
// headless it becomes a CLI reading from a closed stdin, which exits at once;
// that is what a project defaulting to autonomous did to every discussion.
func TestLiveSessionsNeverRunHeadless(t *testing.T) {
	for _, launch := range []struct{ skill, action string }{
		{"discuss", ""},
		{"discuss", "open_terminal"},
		{"", "open_terminal"},
	} {
		if got := liveSessionMode(launch.skill, launch.action, models.SkillModeAutonomous); got != models.SkillModeInteractive {
			t.Fatalf("skill %q action %q resolved to %q, want interactive", launch.skill, launch.action, got)
		}
	}
	// Every other launch keeps the mode the server resolved for it.
	for _, skill := range []string{"clarify", "specify", "implement", "adjust", "pickup"} {
		if got := liveSessionMode(skill, skill, models.SkillModeAutonomous); got != models.SkillModeAutonomous {
			t.Fatalf("skill %q resolved to %q, want autonomous", skill, got)
		}
	}
}

// A command written for headless use is the one a headless launch runs, and it
// answers for itself: its author wrote it for that mode, so it needs no
// {mode:...} marker. The interactive command is left to interactive launches.
func TestDedicatedAutonomousCommandServesHeadlessLaunches(t *testing.T) {
	config := agentconfig.Config{
		AIProvider:                  "claude",
		AICommandTemplate:           `claude '{prompt}'`,
		AICommandTemplateAutonomous: `claude -p --permission-mode bypassPermissions '{prompt}'`,
	}
	for mode, want := range map[string]string{
		models.SkillModeInteractive: `claude 'do the thing'`,
		models.SkillModeAutonomous:  `claude -p --permission-mode bypassPermissions 'do the thing'`,
	} {
		got, err := launchCommandLine(config, "", "do the thing", mode)
		if err != nil {
			t.Fatalf("%s: %v", mode, err)
		}
		if got != want {
			t.Fatalf("%s: got %q, want %q", mode, got, want)
		}
	}
}

// Without a dedicated command nothing moves: the general one still owns the
// mode, and still has to declare that it can serve a headless launch.
func TestWithoutDedicatedCommandTheMarkerStillDecides(t *testing.T) {
	config := agentconfig.Config{AIProvider: "claude", AICommandTemplate: `claude '{prompt}'`}
	if _, err := launchCommandLine(config, "", "do the thing", models.SkillModeAutonomous); err == nil {
		t.Fatal("expected a refusal without the mode marker")
	}
	config.AICommandTemplate = `claude {mode:-p|} '{prompt}'`
	got, err := launchCommandLine(config, "", "do the thing", models.SkillModeAutonomous)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(strings.Fields(got), " ") != `claude -p 'do the thing'` {
		t.Fatalf("got %q", got)
	}
}
