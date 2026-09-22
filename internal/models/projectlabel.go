package models

import (
	"errors"
	"slices"
	"strings"
	"unicode"
)

// ErrProjectLabelWhitespace is returned when a project membership label carries
// whitespace inside it.
var ErrProjectLabelWhitespace = errors.New("project label cannot contain whitespace")

// ErrProjectLabelReserved is returned when a project membership label would be
// read as a workflow stage label.
var ErrProjectLabelReserved = errors.New("project label cannot start with \"#\" or name a workflow stage")

// workflowStageNames are the stage labels the workflow rewrites on every stage
// change. A membership label spelled like one of them would be stripped from the
// ticket at its next transition, silently taking it out of its project.
var workflowStageNames = []string{"untouched", "new", "clarified", "specified", "implemented", "reviewed", "finished", "closed"}

// NormalizeProjectLabel validates the membership label of a project. The value is
// written verbatim to the tracker, so it has to be one spelling everywhere:
// GitHub accepts a label with spaces, Jira does not and rewrites them to "-",
// which would make the value stored on the project differ from the value written
// to the ticket — and the membership filter match nothing. Surrounding whitespace
// is a typing accident and is trimmed; whitespace inside the value is refused.
func NormalizeProjectLabel(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	if strings.ContainsFunc(trimmed, unicode.IsSpace) {
		return "", ErrProjectLabelWhitespace
	}
	// The "#" prefix is the workflow's: label comparisons drop it, so "#team"
	// would be matched as "team" locally and as "#team" by the board query.
	if strings.HasPrefix(trimmed, "#") || slices.Contains(workflowStageNames, strings.ToLower(trimmed)) {
		return "", ErrProjectLabelReserved
	}
	return trimmed, nil
}
