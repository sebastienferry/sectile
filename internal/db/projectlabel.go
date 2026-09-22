package db

import (
	"encoding/json"
	"strings"

	"tasks/internal/models"
)

// The membership label attributes a tracker work item to a Sectile project when
// several projects share one tracker project. The sync still imports the whole
// tracker project; what the label narrows is the display, which is why the filter
// lives here, in the task query, rather than in the tracker adapters.

// likeEscape is the escape character used by the membership LIKE pattern. It is
// passed explicitly with ESCAPE, which both SQLite and PostgreSQL accept.
const likeEscape = `\`

// escapeLike neutralises the LIKE wildcards of a value used as a literal inside a
// pattern. Without it a label carrying an underscore — a common spelling — would
// match any character in that position and attribute a foreign ticket.
func escapeLike(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch r {
		case '\\', '%', '_':
			b.WriteString(likeEscape)
		}
		b.WriteRune(r)
	}
	return b.String()
}

// membershipPattern matches the label as a whole element of the JSON labels array.
// Matching it inside its quotes is what makes "team-alpha" miss "team-alphabet";
// both sides are lower-cased because PostgreSQL's LIKE is case-sensitive while
// SQLite's is not, and every other label comparison in this codebase ignores case.
// The label is JSON-encoded the way the labels column is written: encoding/json
// escapes "&", "<", ">", the backslash and the quote, so "R&D" is stored as
// "R\u0026D" and a raw pattern would never find it.
func membershipPattern(label string) string {
	encoded, _ := json.Marshal(strings.ToLower(label))
	return "%" + escapeLike(string(encoded)) + "%"
}

// cleanLabel is the spelling used to compare two labels: trimmed, lower-cased and
// without the "#" that the workflow labels carry.
func cleanLabel(label string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(label), "#")))
}

// hasProjectLabel says whether a label set carries a project's membership label.
func hasProjectLabel(labels []string, projectLabel string) bool {
	want := cleanLabel(projectLabel)
	if want == "" {
		return false
	}
	for _, l := range labels {
		if cleanLabel(l) == want {
			return true
		}
	}
	return false
}

// addProjectLabel stamps a project's membership label on a label set. A work item
// created in a project that filters on a label has to carry it, otherwise the card
// the user just created is filtered out of its own board.
func addProjectLabel(labels []string, proj *models.Project) []string {
	if proj == nil {
		return labels
	}
	label := strings.TrimSpace(proj.ProjectLabel)
	if label == "" || hasProjectLabel(labels, label) {
		return labels
	}
	return append(labels, label)
}

// membershipConditionUnsafe returns the extra WHERE term restricting a task list to
// the work items belonging to their project, and its arguments. A project without a
// membership label claims all of its tasks, so a deployment where nobody configured
// one gets "" and runs exactly the query it ran before this filter existed.
//
// Unsafe: the caller already holds d.mu.
func (d *DB) membershipConditionUnsafe(projectID, userID string) (string, []interface{}) {
	query := "SELECT id, slug, project_label FROM projects WHERE TRIM(project_label) != ''"
	args := []interface{}{}
	// A single selected project only needs its own label; the bookmarked scope
	// needs every labelled project, each answering for its own tasks.
	if projectID != "" && projectID != "all" {
		query += " AND (id = ? OR slug = ?)"
		args = append(args, projectID, projectID)
	}

	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return "", nil
	}
	defer rows.Close()

	var conditions []string
	var condArgs []interface{}
	for rows.Next() {
		var id, slug, label string
		if err := rows.Scan(&id, &slug, &label); err != nil {
			continue
		}
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		// A task carries its project's id or its slug (projectScope copes with
		// both), so both are excluded from the term that lets other projects through.
		conditions = append(conditions, "(project_id NOT IN (?, ?) OR LOWER(labels) LIKE ? ESCAPE '"+likeEscape+"')")
		condArgs = append(condArgs, id, slug, membershipPattern(label))
	}
	if len(conditions) == 0 {
		return "", nil
	}
	return strings.Join(conditions, " AND "), condArgs
}
