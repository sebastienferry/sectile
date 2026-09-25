package db

import (
	"maps"
	"slices"
	"strings"
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
// An assignee matches an identity when both are the same once trimmed and
// folded to lower case; nothing else is folded. The folding is lowerASCII's on
// both sides, so the two engines agree on what matches.
//
// A tracker with a known identity matches on it alone; every other source,
// a NULL one included, matches on the fallback. With no identity at all the
// condition keeps nothing: an empty board is the right answer.
func (d *DB) myTasksCondition(mine MyTasks) (string, []interface{}) {
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

	fallback := foldIdentities(mine.Fallback)
	if len(fallback) > 0 {
		cond := assignee + " IN (" + placeholders(len(fallback)) + ")"
		var fallbackArgs []interface{}
		if len(trackers) > 0 {
			cond = "COALESCE(source, '') NOT IN (" + placeholders(len(trackers)) + ") AND " + cond
			for _, tracker := range trackers {
				fallbackArgs = append(fallbackArgs, tracker)
			}
		}
		for _, identity := range fallback {
			fallbackArgs = append(fallbackArgs, identity)
		}
		parts = append(parts, "("+cond+")")
		args = append(args, fallbackArgs...)
	}

	if len(parts) == 0 {
		return "1 = 0", nil
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}

// foldIdentity is the Go half of the comparison myTasksCondition makes.
func foldIdentity(identity string) string {
	return asciiLower(strings.TrimSpace(identity))
}

// foldIdentities folds, deduplicates and drops the empty ones, keeping order.
func foldIdentities(identities []string) []string {
	out := make([]string, 0, len(identities))
	for _, identity := range identities {
		if folded := foldIdentity(identity); folded != "" && !slices.Contains(out, folded) {
			out = append(out, folded)
		}
	}
	return out
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

	projectCond, projectArgs, labelCond, labelArgs, err := d.taskScopeUnsafe(scope)
	if err != nil {
		return nil, err
	}
	query := "SELECT DISTINCT COALESCE(NULLIF(source, ''), 'local') FROM tasks"
	scopeCond, scopeArgs := joinScope(projectCond, projectArgs, labelCond, labelArgs)
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
