package db

import (
	"encoding/json"
	"strings"
	"unicode"

	"tasks/internal/models"
)

// A project may declare roadmap projects: other Jira projects whose story keys
// its slicing attaches to a line, because the specification names their work
// too. They are read, never written: Sectile creates no story there, writes no
// parent, moves nothing to a sprint and sets no label, and the only thing a key
// of theirs does is attach to a line on import.

// NormalizeRoadmapProjects cleans a declared list: split on commas and blanks,
// upper-cased, duplicates dropped, and the project's own key left out, since it
// is always read.
func NormalizeRoadmapProjects(raw []string, ownKey string) []string {
	own := strings.ToUpper(strings.TrimSpace(ownKey))
	seen := map[string]bool{}
	out := []string{}
	for _, entry := range raw {
		for _, key := range strings.FieldsFunc(entry, func(r rune) bool { return r == ',' || r == ';' || unicode.IsSpace(r) }) {
			key = strings.ToUpper(strings.TrimSpace(key))
			if key == "" || key == own || seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, key)
		}
	}
	return out
}

// parseRoadmapProjects decodes the stored list, tolerating a damaged row as an
// empty one: a list nobody can read declares nothing.
func parseRoadmapProjects(raw string) []string {
	var list []string
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &list) != nil {
		return []string{}
	}
	return list
}

// isRoadmapProjectKey reports whether a work item key belongs to one of the
// project's declared roadmap projects, which Sectile never writes to.
func isRoadmapProjectKey(proj *models.Project, key string) bool {
	if proj == nil {
		return false
	}
	prefix, _, found := strings.Cut(strings.ToUpper(strings.TrimSpace(key)), "-")
	if !found {
		return false
	}
	for _, declared := range proj.RoadmapProjects {
		if declared == prefix {
			return true
		}
	}
	return false
}
