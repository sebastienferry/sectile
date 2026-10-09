package db

import (
	"maps"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// MyTasks is who "me" is for the My Tasks filter (#468): one identity per
// tracker whose personal credential was confirmed, and the account's name and
// e-mail for every other source, local tickets included.
//
// A ticket's assignee is written the way its tracker writes people, a GitHub
// login or a Jira display name, which is almost never the name of the Sectile
// account. Matching it against that name alone found nothing.
type MyTasks struct {
	// ByTracker maps a tracker to the account its tracker confirmed:
	// "github" → "sebastienferry".
	ByTracker map[string]string
	// Fallback is the account's name and e-mail.
	Fallback []string
}

// myTasksCondition is the SQL condition keeping the tickets assigned to me.
// A tracker identity matches an assignee when both are the same once trimmed
// and folded to lower case; nothing else is folded. The folding is
// lowerASCII's on both sides, so the two engines agree on what matches: the
// tracker wrote the identity itself.
//
// The account's name and e-mail were written by a person, not by the
// tracker, and a Jira display name often drops the accents the account keeps:
// "Sebastien FERRY" for "Sébastien FERRY". They match regardless of case and
// accents. No engine folds accents the way the other does, so the folding is
// done here, on the assignees the base holds, and the query compares the
// ones kept exactly.
//
// A tracker with a known identity matches on it alone; every other source,
// a NULL one included, matches on the fallback. With no identity at all the
// condition keeps nothing: an empty board is the right answer. The caller
// holds d.mu.
func (d *DB) myTasksCondition(mine MyTasks) (string, []interface{}, error) {
	assignee := d.lowerASCII("TRIM(assignee)")
	var parts []string
	var args []interface{}

	identities := map[string]string{}
	for tracker, identity := range mine.ByTracker {
		tracker = strings.ToLower(strings.TrimSpace(tracker))
		if folded := foldIdentity(identity); tracker != "" && folded != "" {
			identities[tracker] = folded
		}
	}
	// Sorted, so the same identities always make the same query.
	trackers := slices.Sorted(maps.Keys(identities))
	for _, tracker := range trackers {
		parts = append(parts, "(source = ? AND "+assignee+" = ?)")
		args = append(args, tracker, identities[tracker])
	}

	assignees, err := d.assigneesMatchingUnsafe(mine.Fallback)
	if err != nil {
		return "", nil, err
	}
	if len(assignees) > 0 {
		cond := "TRIM(assignee) IN (" + placeholders(len(assignees)) + ")"
		var fallbackArgs []interface{}
		if len(trackers) > 0 {
			cond = "COALESCE(source, '') NOT IN (" + placeholders(len(trackers)) + ") AND " + cond
			for _, tracker := range trackers {
				fallbackArgs = append(fallbackArgs, tracker)
			}
		}
		for _, value := range assignees {
			fallbackArgs = append(fallbackArgs, value)
		}
		parts = append(parts, "("+cond+")")
		args = append(args, fallbackArgs...)
	}

	if len(parts) == 0 {
		return "1 = 0", nil, nil
	}
	return "(" + strings.Join(parts, " OR ") + ")", args, nil
}

// assigneesMatchingUnsafe lists the trimmed assignees of the base that match
// one of the names, regardless of case and accents, sorted so the same base
// always makes the same query. The caller holds d.mu.
func (d *DB) assigneesMatchingUnsafe(names []string) ([]string, error) {
	wanted := map[string]bool{}
	for _, name := range names {
		if folded := looseIdentity(name); folded != "" {
			wanted[folded] = true
		}
	}
	if len(wanted) == 0 {
		return nil, nil
	}
	rows, err := d.conn.Query("SELECT DISTINCT TRIM(assignee) FROM tasks WHERE TRIM(COALESCE(assignee, '')) <> ''")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var matching []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		if wanted[looseIdentity(value)] && !slices.Contains(matching, value) {
			matching = append(matching, value)
		}
	}
	slices.Sort(matching)
	return matching, rows.Err()
}

// looseIdentity folds a name for the fallback comparison: trimmed, in lower
// case, and without its accents, "Sébastien FERRY" reading "sebastien ferry".
func looseIdentity(identity string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.TrimSpace(identity)) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// foldIdentity is the Go half of the comparison myTasksCondition makes.
func foldIdentity(identity string) string {
	return asciiLower(strings.TrimSpace(identity))
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

// TaskSourcesInScope lists the sources of the tickets a scope selects, before
// any other filter: the trackers My Tasks has to know an identity for. A
// ticket with no recorded source counts as local.
func (d *DB) TaskSourcesInScope(scope TaskScope) ([]string, error) {
	// "All projects" is the bookmarks, seeded as GetTasksInScope seeds them,
	// so both answer over the same tickets.
	if scope.UserID != "" && scope.ViewID == "" && (scope.ProjectID == "" || scope.ProjectID == "all") {
		_ = d.EnsureDefaultBookmark(scope.UserID)
	}
	d.mu.RLock()
	defer d.mu.RUnlock()

	taskCond, taskArgs, _, _, labelCond, labelArgs, err := d.taskScopeUnsafe(scope)
	if err != nil {
		return nil, err
	}
	query := "SELECT DISTINCT COALESCE(NULLIF(source, ''), 'local') FROM tasks"
	scopeCond, scopeArgs := joinScope(taskCond, taskArgs, labelCond, labelArgs)
	if scopeCond != "" {
		query += " WHERE " + scopeCond
	}
	rows, err := d.conn.Query(query, scopeArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sources := []string{}
	for rows.Next() {
		var source string
		if err := rows.Scan(&source); err != nil {
			return nil, err
		}
		if source == sourceConverting {
			source = "local"
		}
		if !slices.Contains(sources, source) {
			sources = append(sources, source)
		}
	}
	slices.Sort(sources)
	return sources, rows.Err()
}
