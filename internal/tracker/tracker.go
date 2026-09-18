// Package tracker is the common ground between issue trackers.
//
// The application knows how to assign a work item, move it to a sprint, attach
// it to an epic, transition it, comment it. What differs from one tracker to the
// next is whether the operation exists at all: Jira has sprints and teams,
// GitHub has neither: it carries no sprint and no team. Rather than
// scattering `if source == "jira"` across the call sites, a tracker declares
// what it can do, and the caller asks.
//
// Two consequences that matter more than the abstraction itself. A refusal now
// says what is missing, and says it the same way everywhere. And adding a
// tracker means writing one adapter, not finding every place that assumed Jira.
package tracker

import (
	"context"
	"fmt"
	"strings"
)

// Capability is one operation a tracker may or may not support.
type Capability string

const (
	// CapCreate creates a work item in the tracker.
	CapCreate Capability = "create"
	// CapUpdate updates a work item in the tracker.
	CapUpdate Capability = "update"
	// CapDelete deletes or archives/closes a work item in the tracker.
	CapDelete Capability = "delete"
	// CapSync imports or synchronizes work items from the tracker.
	CapSync Capability = "sync"
	// CapGet fetches a single work item from the tracker.
	CapGet Capability = "get"
	// CapAssign sets or clears who owns a work item.
	CapAssign Capability = "assign"
	// CapTransition moves a work item to a status the tracker names.
	CapTransition Capability = "transition"
	// CapSprint puts work items into an iteration, or back to the backlog.
	CapSprint Capability = "sprint"
	// CapTeam sets the team a work item belongs to.
	CapTeam Capability = "team"
	// CapEpic attaches a work item to a parent epic, or detaches it.
	CapEpic Capability = "epic"
	// CapLabels adds and removes labels.
	CapLabels Capability = "labels"
	// CapComment posts a comment.
	CapComment Capability = "comment"
	// CapBoard reads the project's structure: boards, columns, statuses and
	// issue types.
	CapBoard Capability = "board"
)

// CapabilityLabel names an operation in the language the interface speaks, for
// the message a refusal produces.
func CapabilityLabel(c Capability) string {
	switch c {
	case CapCreate:
		return "la création de ticket"
	case CapUpdate:
		return "la mise à jour de ticket"
	case CapDelete:
		return "la suppression de ticket"
	case CapSync:
		return "la synchronisation des tickets"
	case CapGet:
		return "la consultation de ticket"
	case CapAssign:
		return "l'assignation"
	case CapTransition:
		return "le changement de statut"
	case CapSprint:
		return "le changement de sprint"
	case CapTeam:
		return "le changement d'équipe"
	case CapEpic:
		return "le rattachement à un épic"
	case CapLabels:
		return "la modification des labels"
	case CapComment:
		return "les commentaires"
	case CapBoard:
		return "la lecture du board"
	}
	return string(c)
}

// Person is whoever a work item can be assigned to, in the tracker's own terms.
type Person struct {
	// ID is what the tracker assigns by. Jira takes an account id, never a name.
	ID          string
	DisplayName string
	Email       string
	AvatarURL   string
	Active      bool
}

// Writer performs the writes on an existing work item. Every method may return
// ErrUnsupported, and callers are expected to ask Supports first when they want
// to explain rather than fail.
type Writer interface {
	// Name identifies the tracker, as the project configuration spells it.
	Name() string
	// Supports reports whether an operation exists on this tracker.
	Supports(c Capability) bool

	// Assign sets the owner of a work item, or clears it when personID is empty.
	Assign(ctx context.Context, key string, personID string) error
	// Transition moves a work item to a status named as the tracker names it.
	Transition(ctx context.Context, key string, status string) error
	// SetSprint moves work items into a sprint, or back to the backlog when
	// sprintID is empty. Trackers that take batches take them here.
	SetSprint(ctx context.Context, sprintID string, keys []string) error
	// SetTeam sets the team of a work item, or clears it.
	SetTeam(ctx context.Context, key string, teamID string) error
	// SetParent attaches a work item to an epic, or detaches it.
	SetParent(ctx context.Context, key string, parentKey string) error
	// UpdateLabels adds and removes labels in a single call.
	UpdateLabels(ctx context.Context, key string, add []string, remove []string) error
	// SearchAssignable looks up who a work item can be assigned to.
	SearchAssignable(ctx context.Context, key string, query string, limit int) ([]Person, error)
}

// ErrUnsupported is returned by an operation the tracker does not have. It
// carries the capability so the caller can say which one, without matching on a
// message.
type ErrUnsupported struct {
	Tracker    string
	Capability Capability
}

func (e *ErrUnsupported) Error() string {
	name := e.Tracker
	if name == "" {
		name = "ce tracker"
	}
	return fmt.Sprintf("%s ne prend pas en charge %s", strings.ToUpper(name), CapabilityLabel(e.Capability))
}

// Unsupported builds the error, for adapters that decline an operation.
func Unsupported(trackerName string, c Capability) error {
	return &ErrUnsupported{Tracker: trackerName, Capability: c}
}

// IsUnsupported reports whether an error is a missing capability rather than a
// failure. A caller shows the two differently: one is a limit, the other a
// problem.
func IsUnsupported(err error) bool {
	if err == nil {
		return false
	}
	var target *ErrUnsupported
	return asUnsupported(err, &target)
}

func asUnsupported(err error, target **ErrUnsupported) bool {
	for err != nil {
		if e, ok := err.(*ErrUnsupported); ok {
			*target = e
			return true
		}
		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrapper.Unwrap()
	}
	return false
}

// Has reports whether a capability is in a list, for adapters that declare theirs
// as a slice.
func Has(list []Capability, c Capability) bool {
	for _, item := range list {
		if item == c {
			return true
		}
	}
	return false
}
