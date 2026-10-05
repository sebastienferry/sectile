package agent

import (
	"encoding/json"
	"errors"
	"net/http"
	"runtime"
	"strings"

	"tasks/internal/agentconfig"
)

// workstationSandboxInput is a save of the workstation Sandbox category
// (#730).
type workstationSandboxInput struct {
	// ClaudeSandbox replaces the workstation values; an empty object clears
	// them.
	ClaudeSandbox *agentconfig.ClaudeSandbox `json:"claudeSandbox"`
	// ClaudeSandboxBase is the values the category read when it opened: the
	// save keeps what the store gained since, a rule moved up from a project
	// meanwhile included. Without it, ClaudeSandbox replaces all.
	ClaudeSandboxBase *agentconfig.ClaudeSandbox `json:"claudeSandboxBase"`
	// Projects is the whitelist, which only the owner writes: it replaces the
	// stored one.
	Projects []string `json:"projects"`
}

// desktopWorkstationSandbox reads and writes the workstation Sandbox values
// and their whitelist (#730). They have their own endpoint because the
// workstation defaults form replaces the defaults whole and knows nothing of
// them.
func (d *agentDaemon) desktopWorkstationSandbox(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeSandboxJSON(w, workstationSandboxPayload(settings))
	case http.MethodPut:
		var input workstationSandboxInput
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil || input.ClaudeSandbox == nil {
			http.Error(w, "Invalid Sandbox settings", 400)
			return
		}
		d.prepareMu.Lock()
		settings, err := agentconfig.UpdateSettings(d.localSettingsRoot(), func(settings *agentconfig.Settings) error {
			sandbox := *input.ClaudeSandbox
			if input.ClaudeSandboxBase != nil {
				stored := agentconfig.ClaudeSandbox{}
				if settings.Defaults.ClaudeSandbox != nil {
					stored = *settings.Defaults.ClaudeSandbox
				}
				sandbox = agentconfig.MergeClaudeSandbox(sandbox, *input.ClaudeSandboxBase, stored)
			}
			normalized, err := agentconfig.NormalizeClaudeSandbox(sandbox)
			if err != nil {
				return errInvalidSandbox{err}
			}
			settings.Defaults.ClaudeSandbox = nil
			if !normalized.IsZero() {
				settings.Defaults.ClaudeSandbox = &normalized
			}
			settings.Defaults.ClaudeSandboxProjects = knownProjects(*settings, input.Projects)
			return nil
		})
		d.prepareMu.Unlock()
		var invalid errInvalidSandbox
		if errors.As(err, &invalid) {
			http.Error(w, invalid.Error(), 400)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeSandboxJSON(w, workstationSandboxPayload(settings))
	default:
		http.Error(w, "Method not allowed", 405)
	}
}

// errInvalidSandbox refuses a save whose values do not normalize, with the
// reason NormalizeClaudeSandbox gives.
type errInvalidSandbox struct{ err error }

func (e errInvalidSandbox) Error() string { return "Invalid Sandbox settings: " + e.err.Error() }

func (e errInvalidSandbox) Unwrap() error { return e.err }

// knownProjects is the whitelist as saved: trimmed, each project once, in the
// order sent, restricted to the projects added to this workstation. A project
// removed meanwhile is dropped rather than refused.
func knownProjects(settings agentconfig.Settings, ids []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if _, known := settings.ProjectSettings[id]; !known || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// workstationSandboxPayload is the workstation Sandbox values as the desktop
// reads them.
func workstationSandboxPayload(settings agentconfig.Settings) map[string]any {
	projects := settings.Defaults.ClaudeSandboxProjects
	if projects == nil {
		projects = []string{}
	}
	return map[string]any{
		"claudeSandbox": claudeSandboxPayload(settings.Defaults.ClaudeSandbox),
		"projects":      projects,
		// Claude Code's sandbox does not run on Windows: only the rules apply.
		"platformSandbox": runtime.GOOS != "windows",
	}
}

// projectSandboxInheritance is what a project's Sandbox category shows of the
// workstation level (#730): whether the whitelist covers the project, and the
// workstation values it then inherits, nil otherwise.
func projectSandboxInheritance(settings agentconfig.Settings, projectID string) (bool, map[string]any) {
	if !settings.Defaults.CoversProject(projectID) {
		return false, nil
	}
	return true, claudeSandboxPayload(settings.Defaults.ClaudeSandbox)
}

// desktopPromoteSandboxRule moves one allow rule of a project up to the
// workstation allow rules, in one save (#730).
func (d *agentDaemon) desktopPromoteSandboxRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var input struct {
		ProjectID string `json:"projectId"`
		Rule      string `json:"rule"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&input); err != nil {
		http.Error(w, "Invalid rule", 400)
		return
	}
	id, rule := strings.TrimSpace(input.ProjectID), strings.TrimSpace(input.Rule)
	if id == "" || rule == "" {
		http.Error(w, "Project and rule required", 400)
		return
	}
	d.prepareMu.Lock()
	settings, err := agentconfig.UpdateSettings(d.localSettingsRoot(), func(settings *agentconfig.Settings) error {
		project := settings.Project(id)
		if project.ClaudeSandbox == nil {
			return errRuleNotFound
		}
		sandbox := *project.ClaudeSandbox
		var kept []string
		for _, entry := range sandbox.Allow {
			if strings.TrimSpace(entry) != rule {
				kept = append(kept, entry)
			}
		}
		if len(kept) == len(sandbox.Allow) {
			return errRuleNotFound
		}
		sandbox.Allow = kept
		project.ClaudeSandbox = &sandbox
		if sandbox.IsZero() {
			project.ClaudeSandbox = nil
		}
		settings.SetProject(id, project)
		global := agentconfig.ClaudeSandbox{}
		if settings.Defaults.ClaudeSandbox != nil {
			global = *settings.Defaults.ClaudeSandbox
			global.Allow = append([]string{}, global.Allow...)
		}
		global.AddAllow(rule)
		settings.Defaults.ClaudeSandbox = &global
		return nil
	})
	d.prepareMu.Unlock()
	if errors.Is(err, errRuleNotFound) {
		http.Error(w, "The project has no such allow rule", 404)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	covered, inherited := projectSandboxInheritance(settings, id)
	writeSandboxJSON(w, map[string]any{
		"claudeSandbox":        claudeSandboxPayload(settings.Project(id).ClaudeSandbox),
		"claudeSandboxGlobal":  inherited,
		"claudeSandboxCovered": covered,
	})
}

func writeSandboxJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

// errRuleNotFound stops a promotion naming a rule the project does not hold.
var errRuleNotFound = errors.New("rule not found")
