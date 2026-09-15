package tracker

import (
	"fmt"
	"strings"
	"sync"
	"tasks/internal/models"
)

// Registry manages registered TicketingSystem adapters and resolves them
// for tasks and projects.
type Registry struct {
	mu       sync.RWMutex
	trackers map[string]TicketingSystem
}

// NewRegistry initializes an empty tracker registry.
func NewRegistry() *Registry {
	return &Registry{
		trackers: make(map[string]TicketingSystem),
	}
}

// Register adds or replaces a TicketingSystem for a given name (case-insensitive).
func (r *Registry) Register(name string, ts TicketingSystem) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.trackers[strings.ToLower(strings.TrimSpace(name))] = ts
}

// Get retrieves a TicketingSystem by name.
func (r *Registry) Get(name string) (TicketingSystem, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ts, ok := r.trackers[strings.ToLower(strings.TrimSpace(name))]
	return ts, ok
}

// ForProject returns the TicketingSystem configured for a given project.
// If the project specifies no tracker or "local", the local adapter is returned if registered.
func (r *Registry) ForProject(proj *models.Project) (TicketingSystem, error) {
	if proj == nil {
		if local, ok := r.Get("local"); ok {
			return local, nil
		}
		return nil, fmt.Errorf("projet manquant")
	}

	trackerName := strings.ToLower(strings.TrimSpace(proj.IssueTracker))
	if trackerName == "" || trackerName == "local" {
		// Try heuristic detection if tracker not explicitly set
		if proj.GithubRepo != "" {
			trackerName = "github"
		} else if proj.LinearTeam != "" {
			trackerName = "linear"
		} else {
			trackerName = "local"
		}
	}

	ts, ok := r.Get(trackerName)
	if !ok {
		return nil, fmt.Errorf("aucun tracker distant configuré pour le type %q", trackerName)
	}
	return ts, nil
}

// ForTask resolves the TicketingSystem for a given task, checking the task's
// source first and falling back to its project configuration.
func (r *Registry) ForTask(task *models.Task, proj *models.Project) (TicketingSystem, error) {
	if task == nil {
		return r.ForProject(proj)
	}

	source := strings.ToLower(strings.TrimSpace(task.Source))
	if source != "" {
		if ts, ok := r.Get(source); ok {
			return ts, nil
		}
	}

	// Detect by task key or URL prefixes if source is missing
	if strings.HasPrefix(task.Key, "FRE-") || (task.ExternalURL != nil && strings.Contains(*task.ExternalURL, "linear.app")) {
		if ts, ok := r.Get("linear"); ok {
			return ts, nil
		}
	}
	if strings.HasPrefix(task.Key, "gh-") || strings.HasPrefix(task.Key, "#") || strings.HasPrefix(task.Key, "GH-#") ||
		(task.ExternalURL != nil && strings.Contains(*task.ExternalURL, "github.com")) {
		if ts, ok := r.Get("github"); ok {
			return ts, nil
		}
	}

	return r.ForProject(proj)
}
