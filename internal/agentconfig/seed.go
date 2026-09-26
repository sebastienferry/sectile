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

// profile is the engine a legacy resolution ran.
func (e legacyEngine) profile() Engine {
	return normalizeProfile(Engine{
		Provider: e.provider, Command: e.command, CommandAutonomous: e.autonomous,
		Model: e.models.Model, SkillModels: e.models.SkillModels,
	})
}

// catalogueUnstated reports a workstation whose default engine is the one a
// workstation stating nothing runs: the one the conversion gives a file whose
// defaults name no engine. Projects may still have engines of their own.
func (s Settings) catalogueUnstated() bool {
	id := s.DefaultEngine().ID
	return id == "" || id == implicitEngineID
}

// ApplyWorkstationSeed copies the deployment's values into the workstation,
// once, only where the workstation states nothing: the engine the server
// composed becomes the workstation default engine of a catalogue still
// unstated (#510), and the terminal, the editor and the model lists fill the
// keys the defaults leave unset.
func ApplyWorkstationSeed(s *Settings, seed SeedDefaults, server string) {
	old := legacyApply(legacyEngine{
		provider: seed.AIProvider, command: seed.AICommandTemplate, autonomous: seed.AICommandTemplateAutonomous,
		models: ModelConfig{Model: seed.AIModel, SkillModels: seed.AISkillModels},
	}, Execution{}, Execution{})
	written := Defaults{}
	d := &s.Defaults
	if s.catalogueUnstated() && !sameProfile(s.DefaultEngine(), old.profile()) {
		if implicit := s.DefaultEngine().ID; !s.enginePicked(implicit) {
			s.removeEngine(implicit)
		}
		id := s.findOrCreate(old.profile())
		s.Engines.Default = id
		s.Seeded.DefaultEngine = id
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

// enginePicked reports a project or a task pointing at the engine.
func (s Settings) enginePicked(id string) bool {
	for _, m := range []map[string]string{s.Engines.Projects, s.Engines.Tasks} {
		for _, picked := range m {
			if picked == id {
				return true
			}
		}
	}
	return false
}

func (s *Settings) removeEngine(id string) {
	kept := s.Engines.Catalogue[:0]
	for _, engine := range s.Engines.Catalogue {
		if engine.ID != id {
			kept = append(kept, engine)
		}
	}
	s.Engines.Catalogue = kept
}

// globalBeforeSeed is the workstation's own global statement: the default
// engine unless the seed created it or the workstation states none, and the
// terminal unless the defaults seed wrote it and nobody changed it since.
func globalBeforeSeed(s Settings) Execution {
	var g Execution
	if engine := s.DefaultEngine(); engine.ID != s.Seeded.DefaultEngine && !s.catalogueUnstated() {
		g = engine.execution()
	}
	g.Terminal = s.Defaults.Terminal
	if w := s.Seeded.DefaultValues; w != nil && w.Terminal != "" && g.Terminal == w.Terminal {
		g.Terminal = ""
	}
	return g
}

// ApplyProjectSeed copies what the server composed for a project, once, only
// where the project states nothing and only where it changes the resolution:
// afterwards the project resolves the provider, command lines, models,
// terminal, worktrees, setup providers and skill command names the pre-#305
// agent resolved. The engine becomes a catalogue entry the project picks
// (#510), found or created, never an edit of an existing entry.
func ApplyProjectSeed(s *Settings, c Config, seed SeedProject, at string) {
	id := c.ProjectID
	p := s.Project(id)
	global := globalBeforeSeed(*s)
	// A project that picked an engine states that engine: the seed only fills
	// what the pick leaves unset, as it filled what a section left unset.
	local := Execution{Terminal: p.Terminal}
	pick, picked := s.Engine(s.Engines.Projects[id])
	if picked {
		local = pick.execution()
		local.Terminal = p.Terminal
	}
	old := legacyApply(legacyEngine{
		provider: seed.AIProvider, command: seed.AICommandTemplate, autonomous: seed.AICommandTemplateAutonomous,
		terminal: seed.Terminal, models: ModelConfig{Model: seed.AIModel, SkillModels: seed.AISkillModels},
	}, global, local)
	if picked {
		// A pick states its per-skill models, which no pre-#305 project level did.
		old.models = MergeModels(ModelConfig{Model: old.models.Model, SkillModels: pick.SkillModels}, old.models)
	}
	current := func() Config {
		s.SetProject(id, p)
		return Resolve(c, *s)
	}
	resolved, want := current(), old.profile()
	if resolved.AIProvider != want.Provider || resolved.AICommandTemplate != want.Command ||
		resolved.AICommandTemplateAutonomous != want.CommandAutonomous ||
		!sameProfile(Engine{Model: resolved.AIModel, SkillModels: resolved.AISkillModels}, Engine{Model: want.Model, SkillModels: want.SkillModels}) {
		s.SetProjectEngine(id, s.findOrCreate(want))
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
		if !reflect.DeepEqual(configuredSetupProviders(*s, p), want) {
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

// configuredSetupProviders is the setup provider list the levels state, before
// the catalogue providers join it: what the server's list is compared with.
func configuredSetupProviders(s Settings, p ProjectSettings) []string {
	var out []string
	if s.Defaults.SetupProviders != nil {
		out = models.NormalizeSetupProviders(s.Defaults.SetupProviders)
	}
	if p.SetupProviders != nil {
		out = models.NormalizeSetupProviders(p.SetupProviders)
	}
	if len(out) == 0 {
		return nil
	}
	return out
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

// Capability is what a workstation will run for one project: the engine a
// web launch announces before the run, and the models it may pick (#305).
type Capability struct {
	ProjectID   string            `json:"projectId"`
	Provider    string            `json:"provider"`
	Model       string            `json:"model"`
	SkillModels map[string]string `json:"skillModels"`
	Models      []string          `json:"models"`
	ModelSlot   bool              `json:"modelSlot"`
	Headless    bool              `json:"headless"`
}

// CapabilityReport is the body of PUT /api/v1/agent/capabilities. DeviceID is
// the one the agent presents on its WebSocket, which is how the server finds
// the report of the workstation a launch goes to.
type CapabilityReport struct {
	SchemaVersion int          `json:"schemaVersion"`
	DeviceID      string       `json:"deviceId"`
	Projects      []Capability `json:"projects"`
}
