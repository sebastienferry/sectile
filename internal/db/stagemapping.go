package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"tasks/internal/models"
)

// The stage→columns mapping says which columns of a tracker's board each
// workflow stage sits in (#741). It is read both ways: a ticket's tracker
// status gives its stage on import, and a stage move gives the status written
// back to the tracker.
//
// A tracker carries the default mapping, which an admin sets in
// Administration → Trackers. Each project selecting the tracker may map the
// stages onto the same columns its own way (project_trackers.stage_columns),
// from its settings, a member as well as an admin; the columns themselves and
// the statuses they group stay the tracker's. An empty project mapping reads
// the tracker's. A ticket shown by two projects may therefore resolve to two
// stages: the owner chose a mapping per project and tracker knowing it.
//
// Which mapping applies to a ticket, the way the Jira priority mapping finds
// its project (mappingProjectUnsafe):
//
//  1. the project in context, when it selects the ticket's tracker: the
//     project a write or a stage move is made from (the board's project, the
//     run's run_project_id, the project an API request names);
//  2. else the single project the ticket belongs to (memberProjectIDsUnsafe);
//  3. else the tracker's own mapping: a ticket of several projects read with
//     none in context, or of none.
//
// Steps 1 and 2 fall back on the tracker's mapping when the project has none
// of its own for that tracker.

// ErrInvalidStageColumns refuses a project stage mapping naming a stage that
// does not exist, a column its tracker does not have, or a tracker the project
// does not select (#741).
var ErrInvalidStageColumns = errors.New("correspondance étapes → colonnes invalide")

// projectStageColumnsUnsafe is the project's own mapping that applies to a
// ticket of the tracker, by the rule above; nil when the tracker's applies.
// members are the projects the ticket belongs to.
func (d *DB) projectStageColumnsUnsafe(trackerID, contextProjectID string, members []string) map[string][]string {
	index := d.membershipUnsafe()
	if index == nil || trackerID == "" {
		return nil
	}
	if contextProjectID != "" {
		if p, ok := index.project(contextProjectID); ok && slices.Contains(p.Trackers, trackerID) {
			return p.StageColumns[trackerID]
		}
	}
	if len(members) == 1 {
		if p, ok := index.project(members[0]); ok && slices.Contains(p.Trackers, trackerID) {
			return p.StageColumns[trackerID]
		}
	}
	return nil
}

// withStageMappingUnsafe is trk carrying the stage mapping that applies to a
// ticket of it: a copy holding the project's own mapping when one applies,
// trk itself otherwise. StageForTrackerStatus and TrackerStatusForStage read
// the result.
func (d *DB) withStageMappingUnsafe(trk *models.Tracker, contextProjectID string, members []string) *models.Tracker {
	if trk == nil {
		return nil
	}
	own := d.projectStageColumnsUnsafe(trk.ID, contextProjectID, members)
	if len(own) == 0 {
		return trk
	}
	mapped := *trk
	mapped.StageColumns = own
	return &mapped
}

// stageTrackerOfTaskUnsafe is the ticket's tracker carrying the stage mapping
// that applies to it. contextProjectID, when set, is the project the caller
// acts for; otherwise the ticket's own context counts (ContextProjectID: the
// project it was read for, or the one its run works for).
func (d *DB) stageTrackerOfTaskUnsafe(task *models.Task, contextProjectID string) *models.Tracker {
	trk := d.trackerOfTaskUnsafe(task)
	if trk == nil {
		return nil
	}
	if strings.TrimSpace(contextProjectID) == "" {
		contextProjectID = task.ContextProjectID
	}
	members := task.ProjectIDs
	if members == nil {
		if task.TrackerID != "" {
			members = d.memberProjectIDsUnsafe(task.TrackerID, task.Labels)
		} else if task.ProjectID != "" {
			members = []string{task.ProjectID}
		}
	}
	return d.withStageMappingUnsafe(trk, strings.TrimSpace(contextProjectID), members)
}

// columnNamesOf lists the names of a tracker's columns.
func columnNamesOf(columns []models.TrackerColumn) map[string]bool {
	names := make(map[string]bool, len(columns))
	for _, col := range columns {
		names[col.Name] = true
	}
	return names
}

// pruneStageColumns keeps, of a mapping, the columns named in names, and the
// stages left with one.
func pruneStageColumns(stages map[string][]string, names map[string]bool) map[string][]string {
	kept := map[string][]string{}
	for stage, cols := range stages {
		var stay []string
		for _, c := range cols {
			if names[c] {
				stay = append(stay, c)
			}
		}
		if len(stay) > 0 {
			kept[stage] = stay
		}
	}
	return kept
}

// knownStage says whether stage is one of the six workflow stages.
func knownStage(stage string) bool {
	return slices.Contains(workflowStageOrder, stage)
}

// checkStageColumns validates a project mapping sent for trk: every stage is a
// workflow stage, every column one of the tracker's. A stage mapped onto no
// column is dropped, and a column named twice is kept once.
func checkStageColumns(trk *models.Tracker, stages map[string][]string) (map[string][]string, error) {
	names := columnNamesOf(trk.TrackerColumns)
	out := map[string][]string{}
	for stage, cols := range stages {
		if !knownStage(stage) {
			return nil, fmt.Errorf("%w : l'étape %q n'existe pas", ErrInvalidStageColumns, stage)
		}
		var kept []string
		for _, c := range cols {
			if !names[c] {
				return nil, fmt.Errorf("%w : le tracker %s n'a pas de colonne %q", ErrInvalidStageColumns, trk.Name, c)
			}
			if !slices.Contains(kept, c) {
				kept = append(kept, c)
			}
		}
		if len(kept) > 0 {
			out[stage] = kept
		}
	}
	return out, nil
}

// sameStageColumns says whether two mappings assign the same columns, in the
// same order, to the same stages.
func sameStageColumns(a, b map[string][]string) bool {
	return maps.EqualFunc(pruneEmptyStages(a), pruneEmptyStages(b), slices.Equal[[]string])
}

func pruneEmptyStages(stages map[string][]string) map[string][]string {
	out := map[string][]string{}
	for stage, cols := range stages {
		if len(cols) > 0 {
			out[stage] = cols
		}
	}
	return out
}

// setProjectStageColumnsOn stores a project's own mapping for one of its
// trackers; an empty one reads the tracker's again.
func setProjectStageColumnsOn(tx *sqlTx, projectID, trackerID string, stages map[string][]string) error {
	if stages == nil {
		stages = map[string][]string{}
	}
	raw, _ := json.Marshal(stages)
	_, err := tx.Exec(`UPDATE project_trackers SET stage_columns = ? WHERE project_id = ? AND tracker_id = ?`, string(raw), projectID, trackerID)
	return err
}

// applyProjectStageColumnsUnsafe writes the project mappings a project save
// carries, once its trackers are selected: TrackerStageColumns per tracker,
// strictly, and the legacy StageColumns for the default tracker, leniently, so
// that an older client sending back what it read never fails its save nor
// turns the tracker's mapping into the project's.
func (d *DB) applyProjectStageColumnsUnsafe(tx *sqlTx, p *models.Project, req models.UpdateProjectRequest) error {
	if req.StageColumns == nil && req.TrackerStageColumns == nil {
		return nil
	}
	selected := map[string]*models.Tracker{}
	var defaultID string
	rows, err := tx.Query(`SELECT pt.tracker_id, p.default_tracker_id FROM project_trackers pt JOIN projects p ON p.id = pt.project_id WHERE pt.project_id = ? ORDER BY pt.position`, p.ID)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id, &defaultID); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		t, err := trackerByIDOn(tx, id)
		if err != nil {
			return err
		}
		if t != nil {
			selected[id] = t
		}
	}
	if req.StageColumns != nil && len(ids) > 0 {
		def := selected[defaultID]
		if def == nil {
			def = selected[ids[0]]
		}
		if def != nil && (req.TrackerStageColumns == nil || (*req.TrackerStageColumns)[def.ID] == nil) {
			stages := map[string][]string{}
			for stage, cols := range *req.StageColumns {
				if knownStage(stage) {
					stages[stage] = cols
				}
			}
			stages = pruneStageColumns(stages, columnNamesOf(def.TrackerColumns))
			if sameStageColumns(stages, def.StageColumns) {
				stages = map[string][]string{}
			}
			if err := setProjectStageColumnsOn(tx, p.ID, def.ID, stages); err != nil {
				return err
			}
		}
	}
	if req.TrackerStageColumns == nil {
		return nil
	}
	for trackerID, stages := range *req.TrackerStageColumns {
		trk := selected[strings.TrimSpace(trackerID)]
		if trk == nil {
			return fmt.Errorf("%w : le projet ne sélectionne pas le tracker %q", ErrInvalidStageColumns, trackerID)
		}
		checked, err := checkStageColumns(trk, stages)
		if err != nil {
			return err
		}
		if err := setProjectStageColumnsOn(tx, p.ID, trk.ID, checked); err != nil {
			return err
		}
	}
	return nil
}

// pruneProjectStageColumnsOn drops, from every project's own mapping for the
// tracker, the columns it no longer has: a board import or an admin's edit
// removing a column removes it from the projects' mappings as from the
// tracker's.
func pruneProjectStageColumnsOn(tx *sqlTx, trk *models.Tracker) error {
	rows, err := tx.Query(`SELECT project_id, stage_columns FROM project_trackers WHERE tracker_id = ?`, trk.ID)
	if err != nil {
		return err
	}
	type pruned struct {
		projectID string
		stages    map[string][]string
	}
	var changed []pruned
	names := columnNamesOf(trk.TrackerColumns)
	for rows.Next() {
		var projectID, raw string
		if err := rows.Scan(&projectID, &raw); err != nil {
			rows.Close()
			return err
		}
		stages := parseStageColumns(raw)
		if len(stages) == 0 {
			continue
		}
		if kept := pruneStageColumns(stages, names); !sameStageColumns(kept, stages) {
			changed = append(changed, pruned{projectID, kept})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, c := range changed {
		if err := setProjectStageColumnsOn(tx, c.projectID, trk.ID, c.stages); err != nil {
			return err
		}
	}
	return nil
}
