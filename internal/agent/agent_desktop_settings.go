package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
)

// projectSettingsInput is what the desktop project dialog saves. A field left
// nil keeps the stored value; an inherit flag removes the project's statement
// so the workstation default applies again.
type projectSettingsInput struct {
	ProjectID string `json:"projectId"`
	Path      string `json:"path"`
	// DefaultEngine picks the project default engine from the catalogue
	// (#510); InheritDefaultEngine follows the workstation default engine.
	DefaultEngine        *string `json:"defaultEngine"`
	InheritDefaultEngine bool    `json:"inheritDefaultEngine"`
	// The engine fields of #305 are only read to refuse them: a desktop that
	// still sends them predates the engine catalogue.
	AIProvider                  *string           `json:"aiProvider"`
	AIModel                     *string           `json:"aiModel"`
	AISkillModels               map[string]string `json:"aiSkillModels"`
	AICommandTemplate           *string           `json:"aiCommandTemplate"`
	AICommandTemplateAutonomous *string           `json:"aiCommandTemplateAutonomous"`
	InheritWorktrees            bool              `json:"inheritWorktrees"`
	Parallelism                 *int              `json:"parallelism"`
	InheritParallelism          bool              `json:"inheritParallelism"`
	UseWorktrees                *bool             `json:"useWorktrees"`
	Terminal                    *string           `json:"terminal"`
	InheritTerminal             bool              `json:"inheritTerminal"`
	SetupProviders              *[]string         `json:"setupProviders"`
	InheritSetupProviders       bool              `json:"inheritSetupProviders"`
	SkillCommands               map[string]string `json:"skillCommands"`
	InheritSkillCommands        bool              `json:"inheritSkillCommands"`
	// SpecArtifacts overrides the project's choice to keep or drop the tasks'
	// specification artefacts on this workstation (#487);
	// InheritSpecArtifacts removes the override.
	SpecArtifacts        *string `json:"specArtifacts"`
	InheritSpecArtifacts bool    `json:"inheritSpecArtifacts"`
	// SpecPath is the specifications folder on this workstation; empty
	// clears the override, so a mono-repo checkout carries the
	// specifications again.
	SpecPath *string `json:"specPath"`
}

// statesEngine reports an input carrying an engine field of #305.
func (in projectSettingsInput) statesEngine() bool {
	set := func(value *string) bool { return value != nil && strings.TrimSpace(*value) != "" }
	return set(in.AIProvider) || set(in.AIModel) || set(in.AICommandTemplate) || set(in.AICommandTemplateAutonomous) ||
		len(compactStrings(in.AISkillModels)) > 0
}

// applyEngine sets the project default engine the input picks.
func (in projectSettingsInput) applyEngine(settings *agentconfig.Settings) error {
	if in.InheritDefaultEngine {
		settings.SetProjectEngine(in.ProjectID, "")
		return nil
	}
	if in.DefaultEngine == nil {
		return nil
	}
	id := strings.TrimSpace(*in.DefaultEngine)
	if _, ok := settings.Engine(id); id != "" && !ok {
		return fmt.Errorf("engine %q is not in the catalogue", id)
	}
	settings.SetProjectEngine(in.ProjectID, id)
	return nil
}

// apply folds the input onto a project section.
func (in projectSettingsInput) apply(p agentconfig.ProjectSettings) agentconfig.ProjectSettings {
	text := func(field *string, value *string, inherit bool) {
		if inherit {
			*field = ""
		} else if value != nil {
			*field = strings.TrimSpace(*value)
		}
	}
	text(&p.Terminal, in.Terminal, in.InheritTerminal)
	text(&p.SpecArtifacts, in.SpecArtifacts, in.InheritSpecArtifacts)
	if in.InheritWorktrees {
		p.UseWorktrees = nil
	} else if in.UseWorktrees != nil {
		value := *in.UseWorktrees
		p.UseWorktrees = &value
	}
	if in.InheritParallelism {
		p.Parallelism = 0
	} else if in.Parallelism != nil {
		p.Parallelism = *in.Parallelism
		if p.Parallelism == 0 {
			// Zero means "inherit" in the file; an explicit zero is out of range.
			p.Parallelism = -1
		}
	}
	if in.InheritSetupProviders {
		p.SetupProviders = nil
	} else if in.SetupProviders != nil {
		p.SetupProviders = trimList(*in.SetupProviders)
	}
	if in.InheritSkillCommands {
		p.SkillCommands = nil
	} else if in.SkillCommands != nil {
		p.SkillCommands = compactStrings(in.SkillCommands)
	}
	return p
}

// compactStrings drops blank entries, and returns nil when nothing is left so
// the emptied map leaves the file.
func compactStrings(in map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range in {
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if key != "" && value != "" {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// trimList keeps an empty list empty rather than nil: for setup providers an
// empty list is the decision "none".
func trimList(in []string) []string {
	out := []string{}
	for _, value := range in {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

// fieldSource tells the desktop where a value comes from.
type fieldSource struct {
	Value     any    `json:"value"`
	Inherited any    `json:"inherited"`
	Source    string `json:"source"`
}

// executionFields describes each execution setting of a project: its
// effective value, the value it would have without the project's statement,
// and which level states it. Server values never appear: the workstation owns
// them (#305).
func executionFields(config agentconfig.Config, settings agentconfig.Settings) map[string]fieldSource {
	id := config.ProjectID
	section := settings.Project(id)
	effective := agentconfig.Resolve(config, settings)
	bare := settings
	bare.ProjectSettings = map[string]agentconfig.ProjectSettings{}
	bare.Engines.Projects = nil
	inherited := agentconfig.Resolve(config, bare)
	defaults := settings.Defaults
	source := func(project, workstation bool) string {
		switch {
		case project:
			return "project"
		case workstation:
			return "workstation"
		}
		return "default"
	}
	commands := func(c agentconfig.Config) map[string]string {
		out := map[string]string{}
		for _, skill := range c.Skills {
			out[skill.ID] = skill.Command
		}
		return out
	}
	_, picked := settings.Engine(settings.Engines.Projects[id])
	return map[string]fieldSource{
		"defaultEngine":  {effective.EngineID, settings.DefaultEngine().ID, source(picked, true)},
		"terminal":       {effective.ExternalTerminalCommand, inherited.ExternalTerminalCommand, source(section.Terminal != "", defaults.Terminal != "")},
		"useWorktrees":   {effective.UseWorktrees, inherited.UseWorktrees, source(section.UseWorktrees != nil, defaults.UseWorktrees != nil)},
		"parallelism":    {agentconfig.ExecutionLimit(id, true, settings), agentconfig.ExecutionLimit(id, true, bare), source(section.Parallelism != 0, defaults.Parallelism != 0)},
		"setupProviders": {emptyList(effective.SetupProviders), emptyList(inherited.SetupProviders), source(section.SetupProviders != nil, defaults.SetupProviders != nil)},
		"skillCommands":  {commands(effective), commands(inherited), source(len(section.SkillCommands) > 0, false)},
	}
}

func emptyMap(in map[string]string) map[string]string {
	if in == nil {
		return map[string]string{}
	}
	return in
}

func emptyList(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

// skillNames lists the stage skills of the project, for the desktop to offer
// a per-skill model and command name.
func skillNames(config agentconfig.Config) []map[string]string {
	out := []map[string]string{}
	for _, skill := range config.Skills {
		out = append(out, map[string]string{"id": skill.ID, "command": skill.Command})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["id"] < out[j]["id"] })
	return out
}

// withoutExecution strips the execution fields an older server still sends,
// so the desktop never shows a server value as if it applied.
func withoutExecution(c agentconfig.Config) agentconfig.Config {
	c.AIProvider, c.AICommandTemplate, c.AICommandTemplateAutonomous, c.AIModel = "", "", "", ""
	c.AISkillModels, c.ExternalTerminalCommand, c.SetupProviders, c.UseWorktrees = nil, "", nil, false
	return c
}

// workstationView is what the desktop's workstation screen reads.
type workstationView struct {
	Defaults       agentconfig.Defaults `json:"defaults"`
	Effective      workstationEffective `json:"effective"`
	ProviderModels map[string][]string  `json:"providerModels"`
	SetupProviders []string             `json:"setupProviders"`
	Seeded         agentconfig.Seeded   `json:"seeded"`
}

// workstationEffective is what a project without a section of its own runs.
// Its engine is the workstation default engine (#510).
type workstationEffective struct {
	DefaultEngine    engineSummary       `json:"defaultEngine"`
	Terminal         string              `json:"terminal"`
	EditorCommand    string              `json:"editorCommand"`
	UseWorktrees     bool                `json:"useWorktrees"`
	Parallelism      int                 `json:"parallelism"`
	AIProviderModels map[string][]string `json:"aiProviderModels"`
}

// desktopWorkstation reads and writes the workstation defaults. The agent is
// the only writer of the execution sections of the local file.
func (d *agentDaemon) desktopWorkstation(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(d.workstationViewOf(settings))
	case http.MethodPut:
		var input agentconfig.Defaults
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
			http.Error(w, "Invalid workstation settings", 400)
			return
		}
		input = normalizeDefaults(input)
		// A desktop that predates the engine catalogue still sends the engine
		// fields: refused with a message saying why, never silently dropped.
		if err := agentconfig.ValidateDefaults(input); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		d.prepareMu.Lock()
		_, err := agentconfig.UpdateSettings(d.localSettingsRoot(), func(settings *agentconfig.Settings) error {
			settings.Defaults = input
			return nil
		})
		d.prepareMu.Unlock()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		d.reportCapabilitiesLater()
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", 405)
	}
}

func (d *agentDaemon) workstationViewOf(settings agentconfig.Settings) workstationView {
	effective := agentconfig.Resolve(agentconfig.Config{}, agentconfig.Settings{Defaults: settings.Defaults, Engines: settings.Engines})
	shipped := map[string][]string{}
	for provider := range agentconfig.DefaultProviderModels {
		shipped[provider] = agentconfig.ProviderModels(agentconfig.Defaults{}, provider)
	}
	configured := map[string][]string{}
	for provider := range shipped {
		configured[provider] = agentconfig.ProviderModels(settings.Defaults, provider)
	}
	for provider := range settings.Defaults.AIProviderModels {
		configured[provider] = agentconfig.ProviderModels(settings.Defaults, provider)
	}
	editor := settings.Editor()
	if editor == "" {
		editor = agentconfig.DefaultEditor
	}
	terminal := effective.ExternalTerminalCommand
	if terminal == "" {
		terminal = d.resolveTerminalForProject(context.Background(), "", "")
	}
	return workstationView{
		Defaults: settings.Defaults,
		Effective: workstationEffective{
			DefaultEngine: summaryOf(settings.DefaultEngine()),
			Terminal:      terminal, EditorCommand: editor, UseWorktrees: effective.UseWorktrees,
			Parallelism: agentconfig.ExecutionLimit("", true, settings), AIProviderModels: configured,
		},
		ProviderModels: shipped,
		SetupProviders: append([]string{}, models.SetupProviders...),
		Seeded:         settings.Seeded,
	}
}

// normalizeDefaults trims what a form sends.
func normalizeDefaults(in agentconfig.Defaults) agentconfig.Defaults {
	in.AIProvider = strings.TrimSpace(in.AIProvider)
	in.AICommandTemplate = strings.TrimSpace(in.AICommandTemplate)
	in.AICommandTemplateAutonomous = strings.TrimSpace(in.AICommandTemplateAutonomous)
	in.AIModel = strings.TrimSpace(in.AIModel)
	in.AISkillModels = compactStrings(in.AISkillModels)
	in.Terminal = strings.TrimSpace(in.Terminal)
	in.EditorCommand = strings.TrimSpace(in.EditorCommand)
	if in.SetupProviders != nil {
		in.SetupProviders = trimList(in.SetupProviders)
	}
	in.AIProviderModels = agentconfig.NormalizeProviderModels(in.AIProviderModels)
	if reflect.DeepEqual(in.AIProviderModels, map[string][]string{}) {
		in.AIProviderModels = nil
	}
	return in
}
