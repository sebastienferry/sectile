package agentconfig

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"tasks/internal/models"
)

// SettingsLayout is the layout WriteSettings emits. A file without a layout
// predates #305: its flat keys and per-project maps are folded on read and
// rewritten in this layout on the next save. Layout 3 (#510) moves the engine
// settings into the engine catalogue.
const SettingsLayout = 3

// DefaultProvider is the provider a workstation runs when neither its defaults
// nor the project section name one.
const DefaultProvider = "agy"

// DefaultEditor opens a task or a project when the workstation names no editor.
const DefaultEditor = "code"

// MaxCommandLength bounds a command template, a terminal and an editor, which
// all end up on a command line.
const MaxCommandLength = 4096

// Execution holds the settings that decide how this workstation invokes an AI
// CLI. The workstation owns them (#305): the server neither stores nor serves
// them any more. Every field is optional; an empty one inherits.
type Execution struct {
	AIProvider                  string            `json:"aiProvider,omitempty"`
	AICommandTemplate           string            `json:"aiCommandTemplate,omitempty"`
	AICommandTemplateAutonomous string            `json:"aiCommandTemplateAutonomous,omitempty"`
	AIModel                     string            `json:"aiModel,omitempty"`
	AISkillModels               map[string]string `json:"aiSkillModels,omitempty"`
	Terminal                    string            `json:"terminal,omitempty"`
	// UseWorktrees is a pointer because "off" is a statement: nil inherits.
	UseWorktrees *bool `json:"useWorktrees,omitempty"`
	// Parallelism 0 inherits.
	Parallelism int `json:"parallelism,omitempty"`
	// SetupProviders nil inherits; an empty list is the decision "none". No
	// omitempty: it would drop the empty list and turn "none" into "inherit".
	SetupProviders []string `json:"setupProviders"`
}

// Defaults is the workstation level, applied to every project without a
// statement of its own.
type Defaults struct {
	Execution
	// AIProviderModels lists what a launch may pick, per provider. A present key
	// is a choice, even empty; an absent one falls back to the shipped list.
	AIProviderModels map[string][]string `json:"aiProviderModels,omitempty"`
	EditorCommand    string              `json:"editorCommand,omitempty"`
}

// ProjectSettings is one project's section, keyed by the project's primary key.
type ProjectSettings struct {
	// Path is the project's checkout on this workstation.
	Path string `json:"path,omitempty"`
	// SpecPath is the project's specifications folder, a Git checkout or a
	// plain folder. Without one, a mono-repo project uses its code checkout and
	// a multi-repo project has none.
	SpecPath string `json:"specPath,omitempty"`
	Execution
	// SkillCommands replaces the slash command a stage runs, by skill ID. The
	// name depends on what is installed in the local CLI.
	SkillCommands map[string]string `json:"skillCommands,omitempty"`
	// SpecArtifacts overrides whether this workstation keeps or drops the
	// tasks' specification artefacts (#487): "keep" or "drop"; empty follows
	// the server.
	SpecArtifacts string `json:"specArtifacts,omitempty"`
}

// Seeded records the one-time copies of the server values (US6), so they are
// never taken again even if the server values change later.
type Seeded struct {
	// Defaults is the server the workstation defaults were seeded from.
	Defaults string `json:"defaults,omitempty"`
	// DefaultValues are the values the defaults seed wrote. A project seed
	// needs to tell them from what the workstation had set itself, which
	// outranked the server before #305 while a seeded value did not.
	DefaultValues *Defaults `json:"defaultValues,omitempty"`
	// DefaultEngine is the catalogue engine the defaults seed created (#510):
	// the server's choice, not the workstation's own statement.
	DefaultEngine string `json:"defaultEngine,omitempty"`
	// Projects maps a project to the time its section was seeded.
	Projects map[string]string `json:"projects,omitempty"`
}

// hasSeededDefaults ignores a marker naming no server.
func (s Seeded) hasSeededDefaults() bool { return strings.TrimSpace(s.Defaults) != "" }

// isZero reports an execution level that states nothing.
func (e Execution) isZero() bool {
	return strings.TrimSpace(e.AIProvider) == "" && strings.TrimSpace(e.AICommandTemplate) == "" &&
		strings.TrimSpace(e.AICommandTemplateAutonomous) == "" && strings.TrimSpace(e.AIModel) == "" &&
		len(e.AISkillModels) == 0 && strings.TrimSpace(e.Terminal) == "" && e.UseWorktrees == nil &&
		e.Parallelism == 0 && e.SetupProviders == nil
}

// IsZero reports a project section that states nothing and can be dropped.
func (p ProjectSettings) IsZero() bool {
	return strings.TrimSpace(p.Path) == "" && strings.TrimSpace(p.SpecPath) == "" && p.Execution.isZero() && len(p.SkillCommands) == 0 &&
		strings.TrimSpace(p.SpecArtifacts) == ""
}

// Project returns the project's section, empty when it has none.
func (s Settings) Project(id string) ProjectSettings { return s.ProjectSettings[id] }

// SetProject stores a project section, dropping it when it states nothing.
func (s *Settings) SetProject(id string, p ProjectSettings) {
	if p.IsZero() {
		delete(s.ProjectSettings, id)
		return
	}
	if s.ProjectSettings == nil {
		s.ProjectSettings = map[string]ProjectSettings{}
	}
	s.ProjectSettings[id] = p
}

// ProjectPath is the project's checkout on this workstation, "" when unmapped.
func (s Settings) ProjectPath(id string) string {
	return strings.TrimSpace(s.ProjectSettings[id].Path)
}

// SpecPath is the project's specifications folder, "" when none is set.
func (s Settings) SpecPath(id string) string {
	return strings.TrimSpace(s.ProjectSettings[id].SpecPath)
}

// Editor is the editor this workstation opens a task or a project with.
func (s Settings) Editor() string {
	if editor := strings.TrimSpace(s.Defaults.EditorCommand); editor != "" {
		return editor
	}
	return ""
}

// Terminal is the terminal a project's section or the workstation defaults
// name, "" when neither does.
func (s Settings) Terminal(projectID string) string {
	if terminal := strings.TrimSpace(s.ProjectSettings[projectID].Terminal); terminal != "" {
		return terminal
	}
	return strings.TrimSpace(s.Defaults.Terminal)
}

// Resolve builds the configuration a workstation runs from the server's method
// (identity, skills, workflow) and its local file (the invocation). Every
// execution field the server may still send, an older server does, is
// discarded first: the local file is the only source (#305).
//
// Without a task, the invocation is the project default engine (#510): the
// project's pick from the engine catalogue, else the workstation default
// engine. The other settings follow the #305 levels: the project section
// speaks over the workstation defaults.
func Resolve(c Config, s Settings) Config {
	return resolve(c, s, s.ProjectEngine(c.ProjectID))
}

// ResolveTask builds the configuration a task runs: the engine it was switched
// to in the desktop ticket table, else its project default engine (#510).
func ResolveTask(c Config, s Settings, taskID string) Config {
	return resolve(c, s, s.TaskEngine(c.ProjectID, taskID))
}

func resolve(c Config, s Settings, engine Engine) Config {
	c.Skills = append([]Skill{}, c.Skills...)
	defaults := s.Defaults
	project := s.ProjectSettings[c.ProjectID]

	provider := engine.provider()
	c.AIProvider = provider
	// Legacy values hold a bare CLI name here; the runner never used it.
	c.AICommandTemplate = EffectiveCommandTemplate(provider, engine.Command)
	c.AICommandTemplateAutonomous = EffectiveCommandTemplate(provider, engine.CommandAutonomous)
	profile := normalizeProfile(engine)
	c.AIModel, c.AISkillModels = profile.Model, profile.SkillModels
	c.EngineID, c.EngineName = engine.ID, engine.Name
	c.OffProjectDefaultEngine = engine.ID != s.ProjectEngine(c.ProjectID).ID

	c.UseWorktrees = true
	if defaults.UseWorktrees != nil {
		c.UseWorktrees = *defaults.UseWorktrees
	}
	if project.UseWorktrees != nil {
		c.UseWorktrees = *project.UseWorktrees
	}
	// The project list replaces the default one rather than adding to it.
	c.SetupProviders = nil
	if defaults.SetupProviders != nil {
		c.SetupProviders = models.NormalizeSetupProviders(defaults.SetupProviders)
	}
	if project.SetupProviders != nil {
		c.SetupProviders = models.NormalizeSetupProviders(project.SetupProviders)
	}
	c.SetupProviders = withCatalogueProviders(c, s)
	c.ExternalTerminalCommand = s.Terminal(c.ProjectID)
	if value := project.SpecArtifacts; value == "keep" || value == "drop" {
		c.SpecArtifacts = value
	}

	for i := range c.Skills {
		id := c.Skills[i].ID
		if name := strings.TrimSpace(project.SkillCommands[id]); name != "" {
			c.Skills[i].Command = name
		}
		if id == "adjust" {
			for _, legacy := range []string{"review"} {
				if strings.TrimSpace(s.Skills[id]) == "" && strings.TrimSpace(s.Skills[legacy]) != "" {
					c.Skills[i].RequiresReconciliation = true
				}
			}
		}
		if content, ok := s.Skills[id]; ok {
			if id == "adjust" {
				content += "\n" + c.Skills[i].Content
			}
			c.Skills[i].Content = content
			c.Skills[i].CommandContent = content + "\n\n## Ticket\n$ARGUMENTS\n"
		}
	}
	return c
}

// withCatalogueProviders extends the configured setup providers with the
// provider of every catalogue engine that takes Sectile's skills, so a task
// switched to any of them finds its skills and MCP registration in place
// (#510). The configured order comes first; the running provider is set up
// anyway and is not repeated.
func withCatalogueProviders(c Config, s Settings) []string {
	out := append([]string{}, c.SetupProviders...)
	seen := map[string]bool{EffectiveProvider(c.AIProvider, c.AICommandTemplate): true}
	for _, provider := range out {
		seen[provider] = true
	}
	for _, engine := range s.Engines.Catalogue {
		provider := engine.provider()
		if seen[provider] {
			continue
		}
		seen[provider] = true
		if loc, err := ResolveLocations(provider); err == nil && loc.InstallsSkills() {
			out = append(out, provider)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// DefaultProviderModels is the list Sectile ships per provider, offered at
// launch until the workstation configures its own.
var DefaultProviderModels = map[string][]string{
	"claude": {"claude-fable-5-1", "claude-fable-5", "claude-fable", "claude-opus-5", "claude-sonnet-5", "claude-haiku-4-5"},
	"codex":  {"gpt-5-codex", "gpt-5", "o4-mini"},
	"agy":    {"gemini-3.8-pro", "gemini-3.8-flash", "gemini-3.8", "gemini-3.0-pro", "gemini-2.5-pro", "gemini-2.5-flash"},
	"gemini": {"gemini-3.8-pro", "gemini-3.8-flash", "gemini-3.8", "gemini-3.0-pro", "gemini-2.5-pro", "gemini-2.5-flash"},
	"cursor": {"auto", "claude-sonnet-5", "gpt-5"},
}

// ProviderModels is what a launch may pick for provider: the configured list,
// even empty, else the shipped one.
func ProviderModels(d Defaults, provider string) []string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if configured, ok := d.AIProviderModels[provider]; ok {
		return append([]string{}, configured...)
	}
	return append([]string{}, DefaultProviderModels[provider]...)
}

// skillCommandName accepts a single word, with an optional leading slash; the
// name becomes a file name in the CLI's skill directory.
var skillCommandName = regexp.MustCompile(`^/?[A-Za-z0-9][A-Za-z0-9_-]*$`)

// ValidateExecution checks one level before it is written, so an invalid value
// never reaches the file. The caller names the level in its own message.
func ValidateExecution(e Execution) error {
	provider := strings.TrimSpace(e.AIProvider)
	if err := ValidProvider(provider); err != nil {
		return err
	}
	for name, template := range map[string]string{"aiCommandTemplate": e.AICommandTemplate, "aiCommandTemplateAutonomous": e.AICommandTemplateAutonomous} {
		if len(template) > MaxCommandLength {
			return fmt.Errorf("%s is longer than %d characters", name, MaxCommandLength)
		}
		if strings.TrimSpace(template) != "" && provider == "custom" && !strings.Contains(template, "{prompt}") {
			return fmt.Errorf("the custom provider's %s must contain {prompt}", name)
		}
	}
	if err := ValidModelConfig(ModelConfig{Model: e.AIModel, SkillModels: e.AISkillModels}); err != nil {
		return err
	}
	if len(e.Terminal) > MaxCommandLength {
		return fmt.Errorf("terminal is longer than %d characters", MaxCommandLength)
	}
	if e.Parallelism < 0 || e.Parallelism > MaxParallelism {
		return fmt.Errorf("parallelism must be between 1 and %d", MaxParallelism)
	}
	for _, provider := range e.SetupProviders {
		if len(models.NormalizeSetupProviders([]string{provider})) == 0 {
			return fmt.Errorf("unsupported setup provider %q", provider)
		}
	}
	return nil
}

// ErrEngineFields refuses a level still carrying the engine settings of #305,
// which live in the engine catalogue since #510.
var ErrEngineFields = errors.New("engine settings moved to the engine catalogue; update the desktop app")

// ValidateDefaults checks the workstation level.
func ValidateDefaults(d Defaults) error {
	if d.statesEngine() {
		return ErrEngineFields
	}
	if err := ValidateExecution(d.Execution); err != nil {
		return err
	}
	if err := ValidProviderModels(d.AIProviderModels); err != nil {
		return err
	}
	if len(d.EditorCommand) > MaxCommandLength {
		return fmt.Errorf("editorCommand is longer than %d characters", MaxCommandLength)
	}
	return nil
}

// ValidateProject checks a project section.
func ValidateProject(p ProjectSettings) error {
	if p.statesEngine() {
		return ErrEngineFields
	}
	if err := ValidateExecution(p.Execution); err != nil {
		return err
	}
	for skill, name := range p.SkillCommands {
		if name = strings.TrimSpace(name); name != "" && !skillCommandName.MatchString(name) {
			return fmt.Errorf("skill %q: command %q must be a single word", skill, name)
		}
	}
	switch p.SpecArtifacts {
	case "", "keep", "drop":
	default:
		return fmt.Errorf("specArtifacts must be keep or drop")
	}
	return nil
}
