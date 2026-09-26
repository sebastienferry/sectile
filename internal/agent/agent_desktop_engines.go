package agent

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"tasks/internal/agentconfig"
)

// taskEnginesCapability is what /desktop/status announces once the agent keeps
// an engine catalogue and a per-task engine (#510). A desktop connected to an
// agent without it hides the engine column and the engine editors.
const taskEnginesCapability = "task-engines"

// enginesView is what the desktop's Engines section reads.
type enginesView struct {
	Catalogue []agentconfig.Engine `json:"catalogue"`
	Default   string               `json:"default"`
	// Providers are the providers an engine may name.
	Providers []string `json:"providers"`
	// ProviderModels are the models a launch may pick, per provider.
	ProviderModels map[string][]string `json:"providerModels"`
	// Projects and TaskCounts say what points at each engine, so removing one
	// can say what it affects.
	Projects   map[string]string `json:"projects"`
	TaskCounts map[string]int    `json:"taskCounts"`
}

// engineProviders are the providers the desktop offers for an engine.
var engineProviders = []string{"claude", "codex", "agy", "gemini", "cursor", "vibe", "custom"}

func enginesViewOf(settings agentconfig.Settings) enginesView {
	view := enginesView{
		Catalogue: append([]agentconfig.Engine{}, settings.Engines.Catalogue...), Default: settings.DefaultEngine().ID,
		Providers: append([]string{}, engineProviders...), ProviderModels: map[string][]string{},
		Projects: map[string]string{}, TaskCounts: map[string]int{},
	}
	for _, provider := range engineProviders {
		view.ProviderModels[provider] = agentconfig.ProviderModels(settings.Defaults, provider)
	}
	for project, id := range settings.Engines.Projects {
		if _, ok := settings.Engine(id); ok {
			view.Projects[project] = id
		}
	}
	for _, id := range settings.Engines.Tasks {
		if _, ok := settings.Engine(id); ok {
			view.TaskCounts[id]++
		}
	}
	return view
}

// desktopEngines reads and replaces the engine catalogue.
func (d *agentDaemon) desktopEngines(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(enginesViewOf(settings))
	case http.MethodPut:
		var input struct {
			Catalogue []agentconfig.Engine `json:"catalogue"`
			Default   string               `json:"default"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
			http.Error(w, "Invalid engine catalogue", 400)
			return
		}
		kept := false
		for _, engine := range input.Catalogue {
			kept = kept || (input.Default != "" && strings.TrimSpace(engine.ID) == input.Default)
		}
		if !kept && len(input.Catalogue) > 0 {
			http.Error(w, "Mark another engine as the default before removing the default engine", http.StatusConflict)
			return
		}
		var refused error
		d.prepareMu.Lock()
		settings, err := agentconfig.UpdateSettings(d.localSettingsRoot(), func(settings *agentconfig.Settings) error {
			refused = settings.ReplaceCatalogue(input.Catalogue, input.Default)
			return refused
		})
		d.prepareMu.Unlock()
		if refused != nil {
			http.Error(w, refused.Error(), 400)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		// The project default engines may have changed with the catalogue.
		d.reportCapabilitiesLater()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(enginesViewOf(settings))
	default:
		http.Error(w, "Method not allowed", 405)
	}
}

// engineSummary is what the ticket table shows of an engine.
type engineSummary struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

func summaryOf(e agentconfig.Engine) engineSummary {
	return engineSummary{ID: e.ID, Name: e.Name, Provider: e.Provider, Model: e.Model}
}

// taskEnginesView is what the ticket table of a project reads.
type taskEnginesView struct {
	ProjectDefault string            `json:"projectDefault"`
	Catalogue      []engineSummary   `json:"catalogue"`
	Tasks          map[string]string `json:"tasks"`
}

func taskEnginesViewOf(settings agentconfig.Settings, projectID string) taskEnginesView {
	view := taskEnginesView{ProjectDefault: settings.ProjectEngine(projectID).ID, Catalogue: []engineSummary{}, Tasks: map[string]string{}}
	for _, engine := range settings.Engines.Catalogue {
		view.Catalogue = append(view.Catalogue, summaryOf(engine))
	}
	for task, id := range settings.Engines.Tasks {
		if _, ok := settings.Engine(id); ok {
			view.Tasks[task] = id
		}
	}
	return view
}

// desktopTaskEngines reads the task engines of a project and switches one.
// The desktop computes the next engine itself: the agent stores an explicit
// target, so a repeated click is idempotent.
func (d *agentDaemon) desktopTaskEngines(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		projectID := strings.TrimSpace(r.URL.Query().Get("projectId"))
		if projectID == "" {
			http.Error(w, "Project required", 400)
			return
		}
		settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(taskEnginesViewOf(settings, projectID))
	case http.MethodPut:
		var input struct {
			ProjectID string `json:"projectId"`
			TaskID    string `json:"taskId"`
			EngineID  string `json:"engineId"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&input); err != nil ||
			strings.TrimSpace(input.ProjectID) == "" || strings.TrimSpace(input.TaskID) == "" {
			http.Error(w, "Project and task required", 400)
			return
		}
		errUnknown := errors.New("this engine is no longer in the catalogue")
		settings, err := agentconfig.UpdateSettings(d.localSettingsRoot(), func(settings *agentconfig.Settings) error {
			id := strings.TrimSpace(input.EngineID)
			if _, ok := settings.Engine(id); id != "" && !ok {
				return errUnknown
			}
			// Back on its project default engine, the task follows it again.
			if id == settings.ProjectEngine(input.ProjectID).ID {
				id = ""
			}
			settings.SetTaskEngine(input.TaskID, id)
			return nil
		})
		if errors.Is(err, errUnknown) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		// The capability report describes project default engines only: a task
		// switch changes nothing in it, and the run record says what ran.
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(taskEnginesViewOf(settings, input.ProjectID))
	default:
		http.Error(w, "Method not allowed", 405)
	}
}
