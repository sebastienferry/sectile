package tracker

import "context"

// BasicWriter covers the trackers driven through their CLI, which today means
// GitHub. They carry no sprint, no team and no epic link, and saying so
// explicitly is the whole point: the caller gets a sentence naming what is
// missing instead of a silent no-op or a Jira shaped error.
//
// The operations they do support are not wired here yet: they still go through
// the runner's own field sync, which batches title, description, status and
// labels in one CLI call. Declaring the capabilities now is what lets the call
// sites stop testing the tracker's name.
type BasicWriter struct {
	name         string
	capabilities []Capability
	assign       func(ctx context.Context, key string, personID string) error
	transition   func(ctx context.Context, key string, status string) error
	labels       func(ctx context.Context, key string, add []string, remove []string) error
}

// NewGithubWriter describes what GitHub Issues answers to.
func NewGithubWriter() *BasicWriter {
	return &BasicWriter{name: "github", capabilities: []Capability{CapLabels, CapComment, CapAssign}}
}

func (w *BasicWriter) Name() string { return w.name }

func (w *BasicWriter) Supports(c Capability) bool { return Has(w.capabilities, c) }

func (w *BasicWriter) Assign(ctx context.Context, key string, personID string) error {
	if w.assign == nil {
		return Unsupported(w.name, CapAssign)
	}
	return w.assign(ctx, key, personID)
}

func (w *BasicWriter) Transition(ctx context.Context, key string, status string) error {
	if w.transition == nil {
		return Unsupported(w.name, CapTransition)
	}
	return w.transition(ctx, key, status)
}

func (w *BasicWriter) SetSprint(ctx context.Context, sprintID string, keys []string) error {
	return Unsupported(w.name, CapSprint)
}

func (w *BasicWriter) SetTeam(ctx context.Context, key string, teamID string) error {
	return Unsupported(w.name, CapTeam)
}

func (w *BasicWriter) SetParent(ctx context.Context, key string, parentKey string) error {
	return Unsupported(w.name, CapEpic)
}

func (w *BasicWriter) UpdateLabels(ctx context.Context, key string, add []string, remove []string) error {
	if w.labels == nil {
		return Unsupported(w.name, CapLabels)
	}
	return w.labels(ctx, key, add, remove)
}

func (w *BasicWriter) SearchAssignable(ctx context.Context, key string, query string, limit int) ([]Person, error) {
	return nil, Unsupported(w.name, CapAssign)
}
