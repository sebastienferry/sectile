package agentconfig

import (
	"reflect"
	"strings"
	"tasks/internal/models"
	"testing"
)

// serverRow is one level of the execution values a pre-#305 server stored: the
// deployment settings row or a project row.
type serverRow struct {
	provider, command, autonomous, model, terminal string
	skillModels                                    map[string]string
	useWorktrees                                   bool
	setupProviders                                 []string
	skillCommands                                  map[string]string
}

var stageSkills = []Skill{{ID: "clarify", Command: "/clarify-issue"}, {ID: "implement", Command: "/code-issue"}}

// oldServerConfig is the composition db.AgentConfig served before #305.
func oldServerConfig(id string, project, deployment serverRow) Config {
	c := Config{ProjectID: id, UseWorktrees: project.useWorktrees, SetupProviders: models.NormalizeSetupProviders(project.setupProviders)}
	if len(c.SetupProviders) == 0 {
		c.SetupProviders = nil
	}
	c.AIProvider = firstSet(project.provider, deployment.provider)
	c.AICommandTemplate = EffectiveCommandTemplate(c.AIProvider, firstSet(project.command, deployment.command))
	c.AICommandTemplateAutonomous = EffectiveCommandTemplate(c.AIProvider, firstSet(project.autonomous, deployment.autonomous))
	m := MergeModels(ModelConfig{Model: project.model, SkillModels: project.skillModels}, ModelConfig{Model: deployment.model, SkillModels: deployment.skillModels})
	c.AIModel, c.AISkillModels = m.Model, m.SkillModels
	c.ExternalTerminalCommand = firstSet(project.terminal, deployment.terminal)
	for _, skill := range stageSkills {
		if name := project.skillCommands[skill.ID]; name != "" {
			skill.Command = name
		}
		c.Skills = append(c.Skills, skill)
	}
	return c
}

// oldApply is the former ApplyOverrides over a legacy local file, for the
// fields the seed preserves. The terminal follows the former agent's order:
// project override, global override, then the server's value.
func oldApply(c Config, g, p Execution) Config {
	if p.UseWorktrees != nil {
		c.UseWorktrees = *p.UseWorktrees
	}
	baseProvider, baseCommand, baseAutonomous := c.AIProvider, c.AICommandTemplate, c.AICommandTemplateAutonomous
	if g.AIProvider != "" {
		if g.AIProvider != baseProvider && g.AICommandTemplate == "" {
			baseCommand, baseAutonomous = "", ""
		}
		baseProvider = g.AIProvider
	}
	if g.AICommandTemplate != "" {
		baseCommand = g.AICommandTemplate
	}
	if g.AICommandTemplateAutonomous != "" {
		baseAutonomous = g.AICommandTemplateAutonomous
	}
	c.AIProvider, c.AICommandTemplate, c.AICommandTemplateAutonomous = baseProvider, baseCommand, baseAutonomous
	if p.AIProvider != "" {
		c.AIProvider = p.AIProvider
		if p.AIProvider != baseProvider {
			c.AICommandTemplate, c.AICommandTemplateAutonomous = p.AICommandTemplate, p.AICommandTemplateAutonomous
		} else {
			c.AICommandTemplate, c.AICommandTemplateAutonomous = firstSet(p.AICommandTemplate, baseCommand), firstSet(p.AICommandTemplateAutonomous, baseAutonomous)
		}
	} else {
		c.AICommandTemplate, c.AICommandTemplateAutonomous = firstSet(p.AICommandTemplate, baseCommand), firstSet(p.AICommandTemplateAutonomous, baseAutonomous)
	}
	if c.AIProvider == "" {
		c.AIProvider = DefaultProvider
	}
	c.AICommandTemplate = EffectiveCommandTemplate(c.AIProvider, c.AICommandTemplate)
	c.AICommandTemplateAutonomous = EffectiveCommandTemplate(c.AIProvider, c.AICommandTemplateAutonomous)
	c.ExternalTerminalCommand = firstSet(p.Terminal, g.Terminal, c.ExternalTerminalCommand)
	m := MergeModels(ModelConfig{Model: firstSet(p.AIModel, g.AIModel), SkillModels: g.AISkillModels}, c.Models())
	c.AIModel, c.AISkillModels = m.Model, m.SkillModels
	return c
}

// seedsOf builds what the seed endpoint answers for those rows.
func seedsOf(id string, project, deployment serverRow) (SeedDefaults, SeedProject) {
	composed := oldServerConfig(id, project, deployment)
	worktrees := project.useWorktrees
	return SeedDefaults{
			AIProvider: deployment.provider, AICommandTemplate: deployment.command, AICommandTemplateAutonomous: deployment.autonomous,
			AIModel: deployment.model, AISkillModels: deployment.skillModels, Terminal: deployment.terminal,
		}, SeedProject{
			ProjectID: id, AIProvider: composed.AIProvider, AICommandTemplate: composed.AICommandTemplate,
			AICommandTemplateAutonomous: composed.AICommandTemplateAutonomous, AIModel: composed.AIModel,
			AISkillModels: composed.AISkillModels, Terminal: project.terminal, UseWorktrees: &worktrees,
			SetupProviders: project.setupProviders, SkillCommands: project.skillCommands,
		}
}

func TestSeedReproducesThePreUpgradeResolution(t *testing.T) {
	cases := []struct {
		name                string
		deployment, project serverRow
		global, local       Execution
	}{
		{name: "nothing local", deployment: serverRow{provider: "claude", command: "claude --x {prompt}", model: "opus"}, project: serverRow{useWorktrees: true}},
		{name: "global provider hides the server commands",
			deployment: serverRow{provider: "agy", command: "agy {prompt}"}, project: serverRow{provider: "codex", command: "codex {prompt}", useWorktrees: true},
			global: Execution{AIProvider: "claude"}},
		{name: "project row provider inherits the deployment command",
			deployment: serverRow{provider: "agy", command: "agy --x {prompt}", autonomous: "agy -p {prompt}"}, project: serverRow{provider: "claude", useWorktrees: true}},
		{name: "local project provider over a global command",
			deployment: serverRow{provider: "codex", command: "codex {prompt}"}, project: serverRow{useWorktrees: true},
			global: Execution{AIProvider: "claude", AICommandTemplate: "claude --y {prompt}"}, local: Execution{AIProvider: "gemini"}},
		{name: "models at every level",
			deployment: serverRow{model: "m1", skillModels: map[string]string{"implement": "m2"}}, project: serverRow{model: "m3", useWorktrees: true},
			global: Execution{AISkillModels: map[string]string{"clarify": "m4"}}, local: Execution{AIModel: "m5"}},
		{name: "project row models only", deployment: serverRow{provider: "claude"}, project: serverRow{model: "p1", skillModels: map[string]string{"clarify": "p2"}, useWorktrees: true}},
		{name: "worktrees off on the server", project: serverRow{useWorktrees: false}},
		{name: "worktrees off on the server, local override on", project: serverRow{useWorktrees: false}, local: Execution{UseWorktrees: boolPtr(true)}},
		{name: "setup providers and skill commands", project: serverRow{useWorktrees: true, setupProviders: []string{"codex"}, skillCommands: map[string]string{"implement": "build-it"}}},
		{name: "global terminal outranks the project row", deployment: serverRow{terminal: "wezterm"}, project: serverRow{terminal: "iterm", useWorktrees: true}, global: Execution{Terminal: "ghostty"}},
		{name: "project row terminal", deployment: serverRow{terminal: "wezterm"}, project: serverRow{terminal: "iterm", useWorktrees: true}},
		{name: "global provider equal to the deployment's", deployment: serverRow{provider: "agy"}, project: serverRow{provider: "codex", command: "codex {prompt}", useWorktrees: true}, global: Execution{AIProvider: "agy"}},
		{name: "legacy bare CLI name", deployment: serverRow{provider: "claude", command: "claude"}, project: serverRow{useWorktrees: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := oldApply(oldServerConfig("p", tc.project, tc.deployment), tc.global, tc.local)
			settings := Settings{Defaults: Defaults{Execution: tc.global}}
			if !tc.local.isZero() {
				settings.ProjectSettings = map[string]ProjectSettings{"p": {Path: "/repo", Execution: tc.local}}
			}
			defaults, project := seedsOf("p", tc.project, tc.deployment)
			ApplyWorkstationSeed(&settings, defaults, "https://server")
			ApplyProjectSeed(&settings, Config{ProjectID: "p", Skills: stageSkills}, project, "2026-09-25T00:00:00Z")
			got := Resolve(Config{ProjectID: "p", Skills: stageSkills}, settings)
			same := got.AIProvider == want.AIProvider && got.AICommandTemplate == want.AICommandTemplate &&
				got.AICommandTemplateAutonomous == want.AICommandTemplateAutonomous && got.ExternalTerminalCommand == want.ExternalTerminalCommand &&
				got.UseWorktrees == want.UseWorktrees && reflect.DeepEqual(got.SetupProviders, want.SetupProviders) &&
				reflect.DeepEqual(got.Skills, want.Skills)
			for _, skill := range []string{"", "clarify", "implement", "specify"} {
				same = same && ResolveModel(got, skill) == ResolveModel(want, skill)
			}
			if !same {
				t.Fatalf("after the seed:\n got  %+v\n want %+v\n settings %+v", got, want, settings)
			}
			// Local statements are never overwritten.
			if tc.local.AIProvider != "" && settings.Project("p").AIProvider != tc.local.AIProvider {
				t.Fatal("a local project value was overwritten")
			}
			if tc.global.AIProvider != "" && settings.Defaults.AIProvider != tc.global.AIProvider {
				t.Fatal("a local default was overwritten")
			}
		})
	}
}

func TestSeedMarksAndWritesOnlyWhatChangesTheOutcome(t *testing.T) {
	settings := Settings{}
	ApplyWorkstationSeed(&settings, SeedDefaults{AIProvider: "agy", EditorCommand: "cursor", AIProviderModels: map[string][]string{"claude": {"x"}}}, "https://server")
	if settings.Defaults.AIProvider != "" {
		t.Fatal("the provider default needs no seed")
	}
	if settings.Defaults.EditorCommand != "cursor" || !reflect.DeepEqual(settings.Defaults.AIProviderModels["claude"], []string{"x"}) {
		t.Fatalf("editor or model lists not seeded: %+v", settings.Defaults)
	}
	if !settings.HasSeededDefaults() || settings.Seeded.Defaults != "https://server" {
		t.Fatal("defaults not marked")
	}
	ApplyProjectSeed(&settings, Config{ProjectID: "p"}, SeedProject{ProjectID: "p", AIProvider: "agy"}, "now")
	if _, ok := settings.ProjectSettings["p"]; ok {
		t.Fatalf("a project seed that changes nothing must write nothing: %+v", settings.ProjectSettings["p"])
	}
	if !settings.HasSeededProject("p") {
		t.Fatal("project not marked")
	}
}

func TestSeedDoesNotTreatAChangedSeededDefaultAsTheServers(t *testing.T) {
	settings := Settings{}
	ApplyWorkstationSeed(&settings, SeedDefaults{AIProvider: "claude"}, "https://server")
	if settings.Defaults.AIProvider != "claude" {
		t.Fatal("deployment provider not seeded")
	}
	// The user picks codex later: it now outranks the server, as a local choice did before.
	settings.Defaults.AIProvider = "codex"
	ApplyProjectSeed(&settings, Config{ProjectID: "p"}, SeedProject{ProjectID: "p", AIProvider: "gemini"}, "now")
	if got := Resolve(Config{ProjectID: "p"}, settings).AIProvider; got != "codex" {
		t.Fatalf("a local choice lost against a server value: %q", got)
	}
	// While an untouched seeded default is the server's, and a project row outranks it.
	other := Settings{}
	ApplyWorkstationSeed(&other, SeedDefaults{AIProvider: "claude"}, "https://server")
	ApplyProjectSeed(&other, Config{ProjectID: "p"}, SeedProject{ProjectID: "p", AIProvider: "gemini"}, "now")
	if got := Resolve(Config{ProjectID: "p"}, other).AIProvider; got != "gemini" {
		t.Fatalf("the project row must outrank the deployment: %q", got)
	}
}

func TestSeedSkipsInvalidSkillCommands(t *testing.T) {
	settings := Settings{}
	ApplyProjectSeed(&settings, Config{ProjectID: "p", Skills: stageSkills}, SeedProject{ProjectID: "p", SkillCommands: map[string]string{"implement": "rm -rf", "clarify": "/clarify-issue"}}, "now")
	if cmds := settings.Project("p").SkillCommands; len(cmds) != 0 {
		t.Fatalf("an invalid or unchanged command name was seeded: %v", cmds)
	}
	if !strings.Contains(settings.Seeded.Projects["p"], "now") {
		t.Fatal("not marked")
	}
}
