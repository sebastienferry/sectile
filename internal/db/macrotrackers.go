package db

import (
	"fmt"
	"log"
	"strings"

	"tasks/internal/models"
)

// macrosTrackerKeyIndex keeps one macro row per Jira epic of a tracker (#741).
const macrosTrackerKeyIndex = "ux_macros_tracker_key"

// identityScope reads the scope out of a tracker identity, the part after its
// last "|" (see models.TrackerIdentity).
func identityScope(identity string) string {
	if i := strings.LastIndex(identity, "|"); i >= 0 {
		return identity[i+1:]
	}
	return identity
}

// belongsToTracker tells an epic of a Jira tracker by its key: the tracker's
// Jira key, then a dash (#741).
func belongsToTracker(key, jiraKey string) bool {
	prefix := strings.ToUpper(strings.TrimSpace(jiraKey))
	return prefix != "" && strings.HasPrefix(strings.ToUpper(strings.TrimSpace(key)), prefix+"-")
}

// projectJiraKeys lists the Jira keys of a project's trackers, its own tracker
// fields' first.
func projectJiraKeys(proj *models.Project) []string {
	var keys []string
	if key := strings.TrimSpace(proj.JiraProject); key != "" {
		keys = append(keys, key)
	}
	for _, t := range proj.Trackers {
		if strings.HasPrefix(t.Identity, "jira|") {
			keys = append(keys, identityScope(t.Identity))
		}
	}
	return keys
}

// epicTrackerUnsafe is the tracker of the project that holds an epic: the Jira
// tracker whose key prefixes the epic's, else the project's default tracker.
func (d *DB) epicTrackerUnsafe(proj *models.Project, key string) *models.Tracker {
	if proj == nil {
		return nil
	}
	for _, pt := range proj.Trackers {
		if strings.HasPrefix(pt.Identity, "jira|") && belongsToTracker(key, identityScope(pt.Identity)) {
			if t := d.trackerByIDUnsafe(pt.TrackerID); t != nil {
				return t
			}
		}
	}
	return d.trackerOfProjectUnsafe(proj)
}

// macroRowUnsafe says which macro row a project's macro is (#741): a Jira epic
// of one of the project's trackers is a tracker record, one row whatever
// projects show it, found by its tracker and key; a new one is written under
// the tracker's sentinel. Any other macro, a milestone or a local M-<n>, is the
// project's own row. It returns the row's project_id and its tracker, nil for
// none.
func (d *DB) macroRowUnsafe(projectID, key string) (string, any) {
	projectID, key = strings.TrimSpace(projectID), strings.TrimSpace(key)
	if key == "" || isMilestoneKey(key) {
		return projectID, nil
	}
	p, ok := d.membershipUnsafe().project(projectID)
	if !ok || len(p.Trackers) == 0 {
		return projectID, nil
	}
	args := []any{key}
	for _, id := range p.Trackers {
		args = append(args, id)
	}
	var rowProject, trackerID string
	err := d.conn.QueryRow(`SELECT project_id, tracker_id FROM macros WHERE key = ? AND tracker_id IN (`+placeholders(len(p.Trackers))+`) ORDER BY updated_at DESC LIMIT 1`, args...).Scan(&rowProject, &trackerID)
	if err == nil {
		return rowProject, trackerID
	}
	for _, id := range p.Trackers {
		if t := d.trackerByIDUnsafe(id); t != nil && t.Provider == "jira" && belongsToTracker(key, t.Scope) {
			return trackerSentinel(t.ID), t.ID
		}
	}
	return p.ID, nil
}

// macroTitleUnsafe reads the title of a project's macro, empty when it has
// none or is not stored.
func (d *DB) macroTitleUnsafe(projectID, key string) string {
	rowProject, _ := d.macroRowUnsafe(projectID, key)
	var title string
	_ = d.conn.QueryRow("SELECT title FROM macros WHERE project_id = ? AND key = ?", rowProject, key).Scan(&title)
	return title
}

// macroScopeUnsafe selects, over the macros table, the macros the projects
// show (#741): their own rows that name no tracker (milestones, local M-<n>,
// the epics of roadmap projects), and the epics of their trackers that carry
// the project's label or hold one of the project's tickets. A project with no
// label shows every epic of its trackers.
func (d *DB) macroScopeUnsafe(projectRefs []string) (string, []interface{}) {
	index := d.membershipUnsafe()
	var clauses []string
	var args []interface{}
	seen := map[string]bool{}
	for _, ref := range projectRefs {
		p, ok := index.project(ref)
		if !ok || seen[p.ID] {
			continue
		}
		seen[p.ID] = true
		clause := "(project_id IN (?, ?) AND tracker_id IS NULL)"
		args = append(args, p.ID, p.Slug)
		if len(p.Trackers) > 0 {
			in := placeholders(len(p.Trackers))
			epics := fmt.Sprintf("tracker_id IN (%s)", in)
			for _, id := range p.Trackers {
				args = append(args, id)
			}
			if label := strings.TrimSpace(p.Label); label != "" {
				labelCond, labelArgs := viewLabelScope([]string{label}, d.lowerASCII("labels"))
				member, memberArgs := projectMembershipOn(p, "t.", d.lowerASCII("t.labels"))
				epics += " AND (" + labelCond + " OR EXISTS (SELECT 1 FROM tasks t WHERE t.parent_key = macros.key AND " + member + "))"
				args = append(append(args, labelArgs...), memberArgs...)
			}
			clause = "(" + clause + " OR (" + epics + "))"
		}
		clauses = append(clauses, clause)
	}
	if len(clauses) == 0 {
		return "1 = 0", nil
	}
	return "(" + strings.Join(clauses, " OR ") + ")", args
}

// macroScopeOfUnsafe is macroScopeUnsafe over one project, or over the
// projects the user bookmarked ("all"), or nothing.
func (d *DB) macroScopeOfUnsafe(projectID, userID string) (string, []interface{}) {
	if projectID != "" && projectID != "all" {
		return d.macroScopeUnsafe([]string{projectID})
	}
	if strings.TrimSpace(userID) != "" {
		return d.macroScopeUnsafe(d.bookmarkedProjectsUnsafe(userID))
	}
	return "", nil
}

// adoptTrackerEpics merges the macro rows two projects kept for one Jira epic
// of a tracker (#741): the most recently updated row wins, takes the todos of
// another when it has none, and a clash of two todo lists is logged. The
// unique index that keeps one row per epic is then created. Idempotent, under
// the migration lock, like adoptTrackers.
func (d *DB) adoptTrackerEpics() error {
	unlock, err := d.dialect.LockForMigration(d.conn)
	if err != nil {
		return fmt.Errorf("taking the migration lock: %w", err)
	}
	defer unlock()
	indexed, err := d.indexExists(macrosTrackerKeyIndex)
	if err != nil || indexed {
		return err
	}
	err = d.conn.WithTx(func(tx *sqlTx) error {
		rows, err := tx.Query(`SELECT tracker_id, key FROM macros WHERE tracker_id IS NOT NULL GROUP BY tracker_id, key HAVING COUNT(*) > 1`)
		if err != nil {
			return err
		}
		type epic struct{ tracker, key string }
		var duplicates []epic
		for rows.Next() {
			var e epic
			if err := rows.Scan(&e.tracker, &e.key); err != nil {
				rows.Close()
				return err
			}
			duplicates = append(duplicates, e)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, e := range duplicates {
			if err := mergeDuplicateEpic(tx, e.tracker, e.key); err != nil {
				return err
			}
		}
		_, err = tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS " + macrosTrackerKeyIndex + " ON macros (tracker_id, key) WHERE tracker_id IS NOT NULL")
		return err
	})
	d.trackerCache.clear()
	return err
}

// mergeDuplicateEpic keeps the most recently updated row of an epic and
// deletes the others, copying their todos onto it when it has none.
func mergeDuplicateEpic(tx *sqlTx, trackerID, key string) error {
	rows, err := tx.Query(`SELECT project_id, todos FROM macros WHERE tracker_id = ? AND key = ? ORDER BY updated_at DESC, project_id`, trackerID, key)
	if err != nil {
		return err
	}
	type row struct{ project, todos string }
	var list []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.project, &r.todos); err != nil {
			rows.Close()
			return err
		}
		list = append(list, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(list) < 2 {
		return err
	}
	survivor := list[0]
	hasTodos := func(raw string) bool { return len(parseMacroTodos(raw)) > 0 }
	for _, loser := range list[1:] {
		switch {
		case hasTodos(loser.todos) && !hasTodos(survivor.todos):
			if _, err := tx.Exec(`UPDATE macros SET todos = ? WHERE project_id = ? AND key = ?`, loser.todos, survivor.project, key); err != nil {
				return err
			}
			survivor.todos = loser.todos
		case hasTodos(loser.todos):
			log.Printf("[Trackers] épic %s : les todos de la copie du projet %s sont écartés au profit de celle du projet %s, plus récente", key, loser.project, survivor.project)
		}
		if _, err := tx.Exec(`DELETE FROM macros WHERE project_id = ? AND key = ?`, loser.project, key); err != nil {
			return err
		}
		log.Printf("[Trackers] épic %s : copie du projet %s fusionnée dans celle du projet %s", key, loser.project, survivor.project)
	}
	return nil
}
