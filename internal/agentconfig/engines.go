package agentconfig

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// MaxEngines bounds the catalogue: the ticket table cycles through all of it.
const MaxEngines = 20

// MaxEngineName bounds an engine name, shown in a tooltip and a select.
const MaxEngineName = 64

// Engine is one named invocation profile of the workstation (#510): which CLI
// runs and how. An empty model or template means the provider's own default.
type Engine struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Provider          string            `json:"provider"`
	Model             string            `json:"model,omitempty"`
	SkillModels       map[string]string `json:"skillModels,omitempty"`
	Command           string            `json:"command,omitempty"`
	CommandAutonomous string            `json:"commandAutonomous,omitempty"`
}

// Engines is the catalogue and the choices that point into it, by engine ID.
// It lives under its own top-level key rather than inside defaults or
// projectSettings, which an agent that predates #510 replaces as a whole when
// it saves: that agent keeps a key it does not own.
type Engines struct {
	Catalogue []Engine `json:"catalogue"`
	// Default is the workstation default engine.
	Default string `json:"default,omitempty"`
	// Projects holds each project's default engine, by project primary key.
	Projects map[string]string `json:"projects,omitempty"`
	// Tasks holds the engine a task was switched to, by task full ID.
	Tasks map[string]string `json:"tasks,omitempty"`
}

// providerNames are the display names a converted engine is named after.
var providerNames = map[string]string{
	"claude": "Claude", "codex": "Codex", "agy": "Antigravity", "gemini": "Gemini",
	"cursor": "Cursor", "vibe": "Vibe", "custom": "Custom",
}

// implicitEngine is what a workstation whose file states no catalogue runs.
func implicitEngine() Engine {
	return Engine{Name: providerNames[DefaultProvider], Provider: DefaultProvider}
}

// provider reads the engine's provider with its default.
func (e Engine) provider() string {
	if provider := strings.ToLower(strings.TrimSpace(e.Provider)); provider != "" {
		return provider
	}
	return DefaultProvider
}

// execution is the engine as a #305 level, which is how it is validated.
func (e Engine) execution() Execution {
	return Execution{
		AIProvider: strings.TrimSpace(e.Provider), AICommandTemplate: e.Command,
		AICommandTemplateAutonomous: e.CommandAutonomous, AIModel: e.Model, AISkillModels: e.SkillModels,
	}
}

// Engine returns the catalogue entry id names.
func (s Settings) Engine(id string) (Engine, bool) {
	if id == "" {
		return Engine{}, false
	}
	for _, engine := range s.Engines.Catalogue {
		if engine.ID == id {
			return engine, true
		}
	}
	return Engine{}, false
}

// DefaultEngine is the workstation default engine: the entry the catalogue
// marks, else its first entry, else the implicit one of a file never saved.
func (s Settings) DefaultEngine() Engine {
	if engine, ok := s.Engine(s.Engines.Default); ok {
		return engine
	}
	if len(s.Engines.Catalogue) > 0 {
		return s.Engines.Catalogue[0]
	}
	return implicitEngine()
}

// ProjectEngine is the project default engine: the project's pick when it
// names an entry, else the workstation default engine.
func (s Settings) ProjectEngine(projectID string) Engine {
	if engine, ok := s.Engine(s.Engines.Projects[projectID]); ok {
		return engine
	}
	return s.DefaultEngine()
}

// TaskEngine is the engine a task runs: its switch when it names an entry,
// else its project default engine.
func (s Settings) TaskEngine(projectID, taskID string) Engine {
	if engine, ok := s.Engine(s.Engines.Tasks[taskID]); ok {
		return engine
	}
	return s.ProjectEngine(projectID)
}

// SetTaskEngine stores a task's engine; an empty ID clears it.
func (s *Settings) SetTaskEngine(taskID, engineID string) {
	s.Engines.Tasks = setChoice(s.Engines.Tasks, taskID, engineID)
}

// SetProjectEngine stores a project's default engine; an empty ID clears it.
func (s *Settings) SetProjectEngine(projectID, engineID string) {
	s.Engines.Projects = setChoice(s.Engines.Projects, projectID, engineID)
}

func setChoice(m map[string]string, key, engineID string) map[string]string {
	if engineID == "" {
		delete(m, key)
		if len(m) == 0 {
			return nil
		}
		return m
	}
	if m == nil {
		m = map[string]string{}
	}
	m[key] = engineID
	return m
}

// ReplaceCatalogue installs the catalogue the desktop edited. An entry without
// an ID is new and gets one; an ID that left the catalogue takes the project
// and task choices naming it along. The default must name an entry.
func (s *Settings) ReplaceCatalogue(list []Engine, defaultID string) error {
	catalogue := make([]Engine, 0, len(list))
	for _, engine := range list {
		engine.ID = strings.TrimSpace(engine.ID)
		if engine.ID == "" {
			engine.ID = newEngineID()
		} else if _, known := s.Engine(engine.ID); !known {
			// Identities are the agent's to give, so none is ever reused.
			return fmt.Errorf("engine %q: unknown identifier %q", strings.TrimSpace(engine.Name), engine.ID)
		}
		engine.Name = strings.TrimSpace(engine.Name)
		engine.Provider = strings.ToLower(strings.TrimSpace(engine.Provider))
		engine.Model = strings.TrimSpace(engine.Model)
		if len(engine.SkillModels) == 0 {
			engine.SkillModels = nil
		}
		catalogue = append(catalogue, engine)
	}
	next := Engines{Catalogue: catalogue, Default: defaultID, Projects: s.Engines.Projects, Tasks: s.Engines.Tasks}
	if err := ValidateEngines(next); err != nil {
		return err
	}
	if seeded, ok := s.Engine(s.Seeded.DefaultEngine); ok {
		// An engine the seed created and the owner edited is the owner's now.
		if edited, kept := (Settings{Engines: next}).Engine(seeded.ID); !kept || !sameProfile(seeded, edited) {
			s.Seeded.DefaultEngine = ""
		}
	}
	s.Engines = next
	s.pruneEngines()
	return nil
}

// newEngineID is the identity of an engine created from the desktop. A
// converted engine gets a derived one instead (engines_migration.go).
func newEngineID() string {
	return "e-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
}

// pruneEngines drops the choices naming no catalogue entry, and the maps they
// empty, so a removed engine leaves the file with its last reference.
func (s *Settings) pruneEngines() {
	for _, m := range []*map[string]string{&s.Engines.Projects, &s.Engines.Tasks} {
		for key, id := range *m {
			if _, ok := s.Engine(id); !ok {
				delete(*m, key)
			}
		}
		if len(*m) == 0 {
			*m = nil
		}
	}
}

// ValidateEngines checks a catalogue before it is written. Every message names
// the engine it is about.
func ValidateEngines(e Engines) error {
	if len(e.Catalogue) == 0 {
		return fmt.Errorf("the catalogue needs at least one engine")
	}
	if len(e.Catalogue) > MaxEngines {
		return fmt.Errorf("the catalogue holds at most %d engines", MaxEngines)
	}
	ids, names := map[string]bool{}, map[string]string{}
	for _, engine := range e.Catalogue {
		name := strings.TrimSpace(engine.Name)
		if name == "" {
			return fmt.Errorf("every engine needs a name")
		}
		label := fmt.Sprintf("engine %q", name)
		if len(name) > MaxEngineName {
			return fmt.Errorf("%s: the name is longer than %d characters", label, MaxEngineName)
		}
		if engine.ID == "" || ids[engine.ID] {
			return fmt.Errorf("%s: invalid or duplicate identifier %q", label, engine.ID)
		}
		ids[engine.ID] = true
		key := strings.ToLower(name)
		if other, taken := names[key]; taken {
			return fmt.Errorf("engine %q: the name is already used by engine %q", name, other)
		}
		names[key] = name
		if strings.TrimSpace(engine.Provider) == "" {
			return fmt.Errorf("%s: a provider is required", label)
		}
		if err := ValidateExecution(engine.execution()); err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
		if engine.provider() == "custom" && !strings.Contains(engine.Command, "{prompt}") {
			return fmt.Errorf("%s: the custom provider needs an interactive command template containing {prompt}", label)
		}
	}
	if !ids[e.Default] {
		return fmt.Errorf("the default engine %q is not in the catalogue", e.Default)
	}
	return nil
}

// statesEngine reports a level stating any engine field of #305.
func (e Execution) statesEngine() bool {
	return strings.TrimSpace(e.AIProvider) != "" || strings.TrimSpace(e.AICommandTemplate) != "" ||
		strings.TrimSpace(e.AICommandTemplateAutonomous) != "" || strings.TrimSpace(e.AIModel) != "" ||
		len(e.AISkillModels) > 0
}

// StatesEngine reports a level still carrying an engine field of #305, which
// a desktop save may no longer send (#510).
func (e Execution) StatesEngine() bool { return e.statesEngine() }

// clearEngine drops the engine fields of #305 from a level.
func (e *Execution) clearEngine() {
	e.AIProvider, e.AICommandTemplate, e.AICommandTemplateAutonomous, e.AIModel = "", "", "", ""
	e.AISkillModels = nil
}
