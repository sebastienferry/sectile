package models

import (
	"errors"
	"strings"
	"unicode"
)

// ErrProjectLabelWhitespace is returned when a project membership label carries
// whitespace inside it.
var ErrProjectLabelWhitespace = errors.New("project label cannot contain whitespace")

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
	return trimmed, nil
}
