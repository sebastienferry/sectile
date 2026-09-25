package agentconfig

import (
	"reflect"
	"strings"
	"tasks/internal/models"
)

// Seed is the answer of GET /api/v1/agent/execution-seed: the execution
// settings a server stored before #305, served read-only so each workstation
// copies them once and keeps running what it ran before the upgrade.
type Seed struct {
	SchemaVersion int           `json:"schemaVersion"`
	Defaults      *SeedDefaults `json:"defaults,omitempty"`
	Project       *SeedProject  `json:"project,omitempty"`
}

// SeedDefaults are the deployment's values, with the terminal and the editor
// of the paired account.
type SeedDefaults struct {
	AIProvider                  string              `json:"aiProvider,omitempty"`
	AICommandTemplate           string              `json:"aiCommandTemplate,omitempty"`
	AICommandTemplateAutonomous string              `json:"aiCommandTemplateAutonomous,omitempty"`
	AIModel                     string              `json:"aiModel,omitempty"`
	AISkillModels               map[string]string   `json:"aiSkillModels,omitempty"`
	AIProviderModels            map[string][]string `json:"aiProviderModels,omitempty"`
	Terminal                    string              `json:"terminal,omitempty"`
	EditorCommand               string              `json:"editorCommand,omitempty"`
}

// SeedProject is what the server composed for a project before #305, project
// row over deployment. Terminal is the project row's own: the deployment's is
// already part of the defaults.
type SeedProject struct {
	ProjectID                   string            `json:"projectId"`
	AIProvider                  string            `json:"aiProvider,omitempty"`
	AICommandTemplate           string            `json:"aiCommandTemplate,omitempty"`
	AICommandTemplateAutonomous string            `json:"aiCommandTemplateAutonomous,omitempty"`
	AIModel                     string            `json:"aiModel,omitempty"`
	AISkillModels               map[string]string `json:"aiSkillModels,omitempty"`
	Terminal                    string            `json:"terminal,omitempty"`
	UseWorktrees                *bool             `json:"useWorktrees,omitempty"`
	SetupProviders              []string          `json:"setupProviders,omitempty"`
	SkillCommands               map[string]string `json:"skillCommands,omitempty"`
}

// HasSeededDefaults reports whether the workstation defaults were seeded,
// from whichever server: pairing to another server later does not seed them
// again.
func (s Settings) HasSeededDefaults() bool { return s.Seeded.hasSeededDefaults() }

// HasSeededProject reports whether the project's section was seeded.
func (s Settings) HasSeededProject(id string) bool { return s.Seeded.Projects[id] != "" }

// legacyEngine is what the pre-#305 agent ran: the server's composition with
// the local override applied over it.
type legacyEngine struct {
	provider, command, autonomous, terminal string
	models                                  ModelConfig
}

// legacyApply reproduces the former ApplyOverrides for the fields the seed
// has to preserve: server values, then the workstation's global override,
// then its per-project override.
func legacyApply(server legacyEngine, global, project Execution) legacyEngine {
	baseProvider := strings.TrimSpace(server.provider)
	if baseProvider == "" {
		baseProvider = DefaultProvider
	}
	baseCommand := EffectiveCommandTemplate(baseProvider, server.command)
	baseAutonomous := EffectiveCommandTemplate(baseProvider, server.autonomous)
	if own := strings.TrimSpace(global.AIProvider); own != "" {
		if own != baseProvider && global.AICommandTemplate == "" {
			baseCommand, baseAutonomous = "", ""
		}
		baseProvider = own
	}
	if global.AICommandTemplate != "" {
		baseCommand = global.AICommandTemplate
	}
	if global.AICommandTemplateAutonomous != "" {
		baseAutonomous = global.AICommandTemplateAutonomous
	}
	out := legacyEngine{provider: baseProvider, command: baseCommand, autonomous: baseAutonomous}
	if own := strings.TrimSpace(project.AIProvider); own != "" && own != baseProvider {
		out.provider, out.command, out.autonomous = own, "", ""
	} else if own != "" {
		out.provider = own
	}
	if strings.TrimSpace(project.AICommandTemplate) != "" {
		out.command = project.AICommandTemplate
	}
	if strings.TrimSpace(project.AICommandTemplateAutonomous) != "" {
		out.autonomous = project.AICommandTemplateAutonomous
	}
	out.command = EffectiveCommandTemplate(out.provider, out.command)
	out.autonomous = EffectiveCommandTemplate(out.provider, out.autonomous)
	out.terminal = firstSet(strings.TrimSpace(project.Terminal), strings.TrimSpace(global.Terminal), strings.TrimSpace(server.terminal))
	model := firstSet(project.AIModel, global.AIModel)
	out.models = MergeModels(ModelConfig{Model: model, SkillModels: global.AISkillModels}, server.models)
	return out
}

// ApplyWorkstationSeed copies the deployment's values into the workstation
// defaults, once, only into the keys the workstation does not set and only
// where they change what a project without values of its own resolves.
func ApplyWorkstationSeed(s *Settings, seed SeedDefaults, server string) {
	global := s.Defaults.Execution
	old := legacyApply(legacyEngine{
		provider: seed.AIProvider, command: seed.AICommandTemplate, autonomous: seed.AICommandTemplateAutonomous,
		models: ModelConfig{Model: seed.AIModel, SkillModels: seed.AISkillModels},
	}, global, Execution{})
	written := Defaults{}
	d := &s.Defaults
	current := func() Config { return Resolve(Config{}, Settings{Defaults: *d}) }
	if d.AIProvider == "" && current().AIProvider != old.provider {
		d.AIProvider, written.AIProvider = old.provider, old.provider
	}
	if d.AICommandTemplate == "" && old.command != "" && current().AICommandTemplate != old.command {
		d.AICommandTemplate, written.AICommandTemplate = old.command, old.command
	}
	if d.AICommandTemplateAutonomous == "" && old.autonomous != "" && current().AICommandTemplateAutonomous != old.autonomous {
		d.AICommandTemplateAutonomous, written.AICommandTemplateAutonomous = old.autonomous, old.autonomous
	}
	if d.AIModel == "" && old.models.Model != "" {
		d.AIModel, written.AIModel = old.models.Model, old.models.Model
	}
	for skill, model := range old.models.SkillModels {
		if strings.TrimSpace(d.AISkillModels[skill]) == "" {
			d.AISkillModels = setEntry(d.AISkillModels, skill, model)
			written.AISkillModels = setEntry(written.AISkillModels, skill, model)
		}
	}
	if d.Terminal == "" && strings.TrimSpace(seed.Terminal) != "" {
		d.Terminal, written.Terminal = strings.TrimSpace(seed.Terminal), strings.TrimSpace(seed.Terminal)
	}
	if d.EditorCommand == "" && strings.TrimSpace(seed.EditorCommand) != "" {
		d.EditorCommand, written.EditorCommand = strings.TrimSpace(seed.EditorCommand), strings.TrimSpace(seed.EditorCommand)
	}
	for provider, list := range NormalizeProviderModels(seed.AIProviderModels) {
		if _, set := d.AIProviderModels[provider]; set {
			continue
		}
		if d.AIProviderModels == nil {
			d.AIProviderModels = map[string][]string{}
		}
		d.AIProviderModels[provider] = list
	}
	s.Seeded.Defaults = server
	s.Seeded.DefaultValues = &written
}

// globalBeforeSeed is the workstation's own global statement: the defaults
// without what the defaults seed wrote and nobody changed since.
func globalBeforeSeed(s Settings) Execution {
	g := s.Defaults.Execution
	w := s.Seeded.DefaultValues
	if w == nil {
		return g
	}
	unset := func(value *string, seeded string) {
		if seeded != "" && *value == seeded {
			*value = ""
		}
	}
	unset(&g.AIProvider, w.AIProvider)
	unset(&g.AICommandTemplate, w.AICommandTemplate)
	unset(&g.AICommandTemplateAutonomous, w.AICommandTemplateAutonomous)
	unset(&g.AIModel, w.AIModel)
	unset(&g.Terminal, w.Terminal)
	if len(w.AISkillModels) > 0 {
		kept := map[string]string{}
		for skill, model := range g.AISkillModels {
			if w.AISkillModels[skill] != model {
				kept[skill] = model
			}
		}
		g.AISkillModels = kept
	}
	return g
}

// ApplyProjectSeed copies what the server composed for a project into its section,
// once, only into the keys the section does not set and only where they change
// the resolution: afterwards the project resolves the provider, command lines,
// models, terminal, worktrees, setup providers and skill command names the
// pre-#305 agent resolved.
func ApplyProjectSeed(s *Settings, c Config, seed SeedProject, at string) {
	id := c.ProjectID
	p := s.Project(id)
	global := globalBeforeSeed(*s)
	old := legacyApply(legacyEngine{
		provider: seed.AIProvider, command: seed.AICommandTemplate, autonomous: seed.AICommandTemplateAutonomous,
		terminal: seed.Terminal, models: ModelConfig{Model: seed.AIModel, SkillModels: seed.AISkillModels},
	}, global, p.Execution)
	current := func() Config {
		s.SetProject(id, p)
		return Resolve(c, *s)
	}
	if p.AIProvider == "" && current().AIProvider != old.provider {
		p.AIProvider = old.provider
	}
	if p.AICommandTemplate == "" && old.command != "" && current().AICommandTemplate != old.command {
		p.AICommandTemplate = old.command
	}
	if p.AICommandTemplateAutonomous == "" && old.autonomous != "" && current().AICommandTemplateAutonomous != old.autonomous {
		p.AICommandTemplateAutonomous = old.autonomous
	}
	if p.AIModel == "" && old.models.Model != "" && current().AIModel != old.models.Model {
		p.AIModel = old.models.Model
	}
	resolved := current()
	for skill, model := range old.models.SkillModels {
		if strings.TrimSpace(p.AISkillModels[skill]) == "" && resolved.AISkillModels[skill] != model {
			p.AISkillModels = setEntry(p.AISkillModels, skill, model)
		}
	}
	if p.Terminal == "" && old.terminal != "" && current().ExternalTerminalCommand != old.terminal {
		p.Terminal = old.terminal
	}
	if p.UseWorktrees == nil && seed.UseWorktrees != nil && current().UseWorktrees != *seed.UseWorktrees {
		value := *seed.UseWorktrees
		p.UseWorktrees = &value
	}
	if p.SetupProviders == nil {
		want := models.NormalizeSetupProviders(seed.SetupProviders)
		if len(want) == 0 {
			want = nil
		}
		if !reflect.DeepEqual(current().SetupProviders, want) {
			p.SetupProviders = append([]string{}, want...)
		}
	}
	resolved = current()
	for skill, name := range seed.SkillCommands {
		name = strings.TrimSpace(name)
		if name == "" || strings.TrimSpace(p.SkillCommands[skill]) != "" || !skillCommandName.MatchString(name) {
			continue
		}
		if commandOf(resolved, skill) != name {
			p.SkillCommands = setEntry(p.SkillCommands, skill, name)
		}
	}
	s.SetProject(id, p)
	if s.Seeded.Projects == nil {
		s.Seeded.Projects = map[string]string{}
	}
	s.Seeded.Projects[id] = at
}

func commandOf(c Config, skillID string) string {
	for _, skill := range c.Skills {
		if skill.ID == skillID {
			return skill.Command
		}
	}
	return ""
}

func setEntry(m map[string]string, key, value string) map[string]string {
	if m == nil {
		m = map[string]string{}
	}
	m[key] = value
	return m
}
