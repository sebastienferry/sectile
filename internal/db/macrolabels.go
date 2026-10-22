package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"tasks/internal/tracker"
)

// An epic's free labels are the tracker's, not Sectile's: the sync records them
// as the tracker returns them (#626), and an edit made from the roadmap goes to
// the tracker first. The local copy changes only once the tracker accepted the
// write, so there is nothing to push again later and nothing to reconcile.
//
// The labels of the axes the roadmap owns are the exception. They are written
// by their own control, the horizon tabs for "roadmap:", the panel's priority
// and quarter fields for the axes of #627, the readiness chips for #633, and an edit of free labels may
// neither add nor remove one.

// axisLabelPrefixes are the label prefixes the roadmap owns on a project's
// epics: the horizon's, which is fixed, and the three the project names or
// leaves at their default (#635). The web mirrors it in
// epicAxisLabelPrefixes.
func (a axisPrefixes) axisLabelPrefixes() []string {
	return []string{RoadmapLabelPrefix, a.priority, a.quarter, a.readiness}
}

// isMacroAxisLabel tells a label written by one of the roadmap's own axes from a
// free label. The match ignores case and a leading "#", as HorizonFromLabels
// does. A bare quarter such as "2026-Q3" belongs to the quarter axis too: the
// import reads it as the epic's quarter (#627), so editing it as a free label
// would change the quarter behind the panel's back. A label under a prefix the
// project no longer uses is a free label.
func (a axisPrefixes) isMacroAxisLabel(label string) bool {
	clean := cleanLabel(label)
	for _, prefix := range a.axisLabelPrefixes() {
		if strings.HasPrefix(clean, prefix) {
			return true
		}
	}
	return quarterPattern.MatchString(clean)
}

// containsLabelFold tells whether a list carries a label, ignoring case: Jira
// labels are case-sensitive on the wire, but two labels differing only by case
// are one label to whoever reads the roadmap.
func containsLabelFold(list []string, label string) bool {
	for _, l := range list {
		if strings.EqualFold(l, label) {
			return true
		}
	}
	return false
}

// storedMacroLabels reads the labels recorded on a macro, none when the macro is
// not stored.
func (d *DB) storedMacroLabels(projectID string, key string) []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var raw string
	rowProject, _ := d.macroRowUnsafe(projectID, key)
	if err := d.conn.QueryRow("SELECT labels FROM macros WHERE project_id = ? AND key = ?", rowProject, key).Scan(&raw); err != nil {
		return []string{}
	}
	return parseMacroLabels(raw)
}

// labelHasSpace says whether a label holds a space, which Jira refuses in a
// label.
func labelHasSpace(label string) bool {
	return strings.ContainsAny(label, " \t\n\r")
}

// cleanLabelEdit trims one side of an edit and refuses what can never be
// written: an empty label, a label with a space, which Jira refuses, and a label
// of one of the roadmap's axes, under the project's prefixes.
func cleanLabelEdit(prefixes axisPrefixes, labels []string) ([]string, error) {
	out := make([]string, 0, len(labels))
	for _, raw := range labels {
		label := strings.TrimSpace(raw)
		if label == "" {
			return nil, fmt.Errorf("un label vide ne peut pas être posé")
		}
		if labelHasSpace(label) {
			return nil, fmt.Errorf("un label ne peut pas contenir d'espace : « %s »", label)
		}
		if prefixes.isMacroAxisLabel(label) {
			return nil, fmt.Errorf("« %s » appartient à un axe de la roadmap : il se règle depuis l'horizon, la priorité ou le trimestre", label)
		}
		if !containsLabelFold(out, label) {
			out = append(out, label)
		}
	}
	return out, nil
}

// ValidateMacroLabelEdit checks an edit of a macro's free labels before it is
// queued, and returns what is left to send once the labels the macro already
// carries, or does not carry, are dropped.
//
// Every refusal is a sentence, returned before anything is queued: a write that
// can only fail has no business appearing in the activity as a failure.
func (d *DB) ValidateMacroLabelEdit(projectID string, key string, add []string, remove []string) ([]string, []string, error) {
	projectID = strings.TrimSpace(projectID)
	key = strings.TrimSpace(key)
	prefixes := d.projectAxisPrefixes(projectID)
	cleanAdd, err := cleanLabelEdit(prefixes, add)
	if err != nil {
		return nil, nil, err
	}
	cleanRemove, err := cleanLabelEdit(prefixes, remove)
	if err != nil {
		return nil, nil, err
	}

	stored := d.storedMacroLabels(projectID, key)
	toAdd := make([]string, 0, len(cleanAdd))
	for _, label := range cleanAdd {
		if !containsLabelFold(stored, label) && !containsLabelFold(cleanRemove, label) {
			toAdd = append(toAdd, label)
		}
	}
	toRemove := make([]string, 0, len(cleanRemove))
	for _, label := range cleanRemove {
		for _, carried := range stored {
			// The tracker's own spelling is what it will match.
			if strings.EqualFold(carried, label) {
				toRemove = append(toRemove, carried)
				break
			}
		}
	}
	if len(toAdd) == 0 && len(toRemove) == 0 {
		return nil, nil, fmt.Errorf("rien à modifier sur les labels de %s", key)
	}

	if _, _, err := d.macroTracker(projectID, key, axisLabels); err != nil {
		return nil, nil, err
	}
	return toAdd, toRemove, nil
}

// PushMacroLabels writes an edit of a macro's free labels on the tracker's epic,
// then, and only then, on the local copy.
//
// It performs the tracker call itself, so it is only ever run from a queued
// activity (TrackerOpEpicLabels), as the horizon push is.
func (d *DB) PushMacroLabels(ctx context.Context, projectID string, macroKey string, add []string, remove []string) (string, error) {
	ts, proj, err := d.macroTracker(projectID, macroKey, axisLabels)
	if err != nil {
		return "", err
	}
	macroKey = strings.TrimSpace(macroKey)

	ctx, cancel := context.WithTimeout(ctx, macroWriteTimeout)
	defer cancel()
	// Only the delta travels: the other labels of the epic, those of the axes
	// included, stay as the tracker holds them.
	trk := d.epicTrackerUnsafe(proj, macroKey)
	if err := ts.UpdateIssue(tracker.WithTracker(ctx, trk), tracker.UpdateIssueRequest{
		Tracker:       trk,
		Key:           macroKey,
		Labels:        add,
		RemovedLabels: remove,
	}); err != nil {
		return "", err
	}

	if err := d.applyMacroLabelEdit(proj.ID, macroKey, add, remove); err != nil {
		return "", fmt.Errorf("labels de %s écrits sur le tracker, mais pas en local : %w", macroKey, err)
	}
	return fmt.Sprintf("labels de %s mis à jour (+%d, -%d)", macroKey, len(add), len(remove)), nil
}

// applyMacroLabelEdit applies an edit the tracker accepted to the stored list,
// under the row lock, so a sync committing meanwhile is merged rather than
// overwritten.
func (d *DB) applyMacroLabelEdit(projectID string, key string, add []string, remove []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	rowProject, rowTracker := d.macroRowUnsafe(projectID, key)
	return d.conn.WithTx(func(tx *sqlTx) error {
		if _, err := tx.Exec("INSERT INTO macros (project_id, key, tracker_id) VALUES (?, ?, ?) ON CONFLICT (project_id, key) DO NOTHING", rowProject, key, rowTracker); err != nil {
			return err
		}
		raw := "[]"
		if err := tx.QueryRow("SELECT labels FROM macros WHERE project_id = ? AND key = ?"+d.forUpdate(), rowProject, key).Scan(&raw); err != nil && err != sql.ErrNoRows {
			return err
		}
		next := []string{}
		for _, label := range parseMacroLabels(raw) {
			if !containsLabelFold(remove, label) {
				next = append(next, label)
			}
		}
		for _, label := range add {
			if !containsLabelFold(next, label) {
				next = append(next, label)
			}
		}
		payload, _ := json.Marshal(next)
		_, err := tx.Exec("UPDATE macros SET labels = ?, updated_at = CURRENT_TIMESTAMP WHERE project_id = ? AND key = ?", string(payload), rowProject, key)
		return err
	})
}
