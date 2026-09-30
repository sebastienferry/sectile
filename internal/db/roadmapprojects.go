package db

import (
	"encoding/json"
	"strings"
	"unicode"

	"tasks/internal/models"
)

// A project may declare roadmap projects: other Jira projects whose epics its
// roadmap also reads, and whose story keys its slicing attaches to a line,
// because the specification names their work too.
//
// Sectile never changes an existing item of theirs (#632). Two things are
// allowed, each on a gesture naming one item: a slicing line may create a new
// story there, under its epic, and a project that opened RoadmapAxisWrites may
// write the priority and the quarter of one of their epics from its panel. No
// pass over several epics ever writes there, and no work item of theirs other
// than an epic is imported.

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

// roadmapAxisWritesAllowed is the stored value of the axis opt-in: it can only
// be open on a Jira project that declares at least one roadmap project, so that
// emptying the declaration also closes the writes it opened.
func roadmapAxisWritesAllowed(open bool, issueTracker string, declared []string) bool {
	return open && issueTracker == "jira" && len(declared) > 0
}

// macroOrigin is the tracker project key an epic key carries: "DATA-12" reads
// "DATA". A milestone or a key without a project prefix has none.
func macroOrigin(key string) string {
	if isMilestoneKey(key) {
		return ""
	}
	prefix, _, found := strings.Cut(strings.ToUpper(strings.TrimSpace(key)), "-")
	if !found || prefix == "" {
		return ""
	}
	return prefix
}

// isForeignMacro tells an epic of another Jira project, declared or no longer
// declared, from one of the project's own. Only a Jira project has any: a
// milestone, a local key or a GitHub or GitLab macro is never foreign.
func isForeignMacro(key string, proj *models.Project) bool {
	if proj == nil || proj.IssueTracker != "jira" || macroOrigin(key) == "" {
		return false
	}
	return !belongsToProject(key, proj)
}

// isDeclaredRoadmapProject reports whether a Jira project key is in the
// project's current declaration, which a key saved earlier may have left.
func isDeclaredRoadmapProject(proj *models.Project, key string) bool {
	if proj == nil {
		return false
	}
	key = strings.ToUpper(strings.TrimSpace(key))
	for _, declared := range proj.RoadmapProjects {
		if declared == key {
			return true
		}
	}
	return false
}

// fillMacroOrigin computes what a macro read says of where its epic comes from:
// its origin, whether it is foreign, and whether a panel edit of its priority
// or quarter is written on the tracker.
func fillMacroOrigin(m *models.MacroMeta, proj *models.Project, supported bool) {
	m.Origin = macroOrigin(m.Key)
	m.Foreign = isForeignMacro(m.Key, proj)
	m.AxesWritable = macroAxesWritable(m.Key, proj, supported, false)
}

// roadmapProjectView is the project as a request to one of its roadmap
// projects sees it: the same id, site and credentials, another Jira key. It
// declares nothing, so nothing reading it can follow the declaration further.
func roadmapProjectView(proj *models.Project, key string) *models.Project {
	view := *proj
	view.JiraProject = strings.ToUpper(strings.TrimSpace(key))
	view.RoadmapProjects = nil
	view.RoadmapAxisWrites = false
	return &view
}
