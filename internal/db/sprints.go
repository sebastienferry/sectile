package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// Sprint management writes to the tracker, synchronously, and mirrors its
// answer in projects.sprints, the column the board synchronisation already
// refreshes. It does not go through the tracker operation queue: a sprint must
// exist on the tracker, with the id the tracker gave it, before a work item can
// be moved into it or a timeline can show it. A local-only sprint could never
// receive a ticket.

// Batch creation bounds: at most a quarter of weekly sprints at once, and a
// sprint lasts one to four weeks.
const (
	maxSprintBatch = 12
	minSprintWeeks = 1
	maxSprintWeeks = 4
)

// sprintManagerOf resolves the project's tracker and checks it manages its own
// sprints, with the refusal the interface shows when it does not.
func (d *DB) sprintManagerOf(proj *models.Project) (tracker.TicketingSystem, tracker.SprintManager, error) {
	ts, err := d.TrackerForProject(proj)
	if err != nil {
		return nil, nil, err
	}
	manager, ok := ts.(tracker.SprintManager)
	if !ts.Supports(tracker.CapSprintManage) || !ok {
		return nil, nil, tracker.Unsupported(ts.Name(), tracker.CapSprintManage)
	}
	return ts, manager, nil
}

// sprintBatchName names the index-th sprint (1-based) of a batch of count:
// "{n}" is replaced by the index; without it a single sprint keeps the pattern
// as typed and a batch is numbered after it. An empty pattern is dated.
func sprintBatchName(pattern string, index, count int, start time.Time) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		pattern = "Sprint " + start.Format("2006-01-02")
	}
	if strings.Contains(pattern, "{n}") {
		return strings.ReplaceAll(pattern, "{n}", fmt.Sprint(index))
	}
	if count == 1 {
		return pattern
	}
	return fmt.Sprintf("%s %d", pattern, index)
}

func sprintNameKey(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(name), " "))
}

// CreateProjectSprints creates count consecutive sprints of weeks weeks each
// on the project's board, the first starting on start. Everything that can be
// refused is refused before the first write; a failure in the middle keeps and
// mirrors the sprints already created, and says how many there were.
func (d *DB) CreateProjectSprints(ctx context.Context, projectID, pattern string, count int, start time.Time, weeks int) ([]models.TrackerSprint, error) {
	if count < 1 || count > maxSprintBatch {
		return nil, fmt.Errorf("de 1 à %d sprints à la fois, pas %d", maxSprintBatch, count)
	}
	if weeks < minSprintWeeks || weeks > maxSprintWeeks {
		return nil, fmt.Errorf("un sprint dure de %d à %d semaines, pas %d", minSprintWeeks, maxSprintWeeks, weeks)
	}
	proj, err := d.GetProjectByID(projectID)
	if err != nil || proj == nil {
		return nil, fmt.Errorf("projet non trouvé")
	}
	ts, manager, err := d.sprintManagerOf(proj)
	if err != nil {
		return nil, err
	}
	boardID := strings.TrimSpace(proj.BoardID)
	if boardID == "" {
		return nil, fmt.Errorf("choisissez d'abord un board dans les options du projet")
	}

	// A taken name is refused against the board as the tracker sees it now, not
	// only against the mirror, which may be a synchronisation behind.
	taken := map[string]bool{}
	for _, sp := range proj.Sprints {
		taken[sprintNameKey(sp.Name)] = true
	}
	if live, err := ts.ListSprints(ctx, tracker.BoardRequest{Project: proj, BoardID: boardID}); err == nil {
		for _, sp := range live {
			taken[sprintNameKey(sp.Name)] = true
		}
	}
	names := make([]string, count)
	for i := range names {
		names[i] = sprintBatchName(pattern, i+1, count, start)
		if taken[sprintNameKey(names[i])] {
			return nil, fmt.Errorf("le sprint « %s » existe déjà sur le board : essayez « %s bis »", names[i], strings.TrimSpace(pattern))
		}
		taken[sprintNameKey(names[i])] = true
	}

	created := make([]models.TrackerSprint, 0, count)
	cursor := start
	var failure error
	for _, name := range names {
		next := cursor.AddDate(0, 0, 7*weeks)
		sprint, err := manager.CreateSprint(ctx, tracker.SprintCreateRequest{Project: proj, BoardID: boardID, Name: name, Start: cursor, End: next.Add(-time.Second)})
		if err != nil {
			failure = fmt.Errorf("%d sprint(s) créé(s), puis échec sur « %s » : %w", len(created), name, err)
			break
		}
		created = append(created, sprint)
		cursor = next
	}
	if len(created) > 0 {
		if err := d.mirrorSprints(proj.ID, func(list []models.TrackerSprint) []models.TrackerSprint {
			for _, sp := range created {
				list = replaceSprint(list, sp)
			}
			return list
		}); err != nil && failure == nil {
			failure = err
		}
	}
	return created, failure
}

// UpdateProjectSprint renames, re-dates or changes the state of one sprint.
// Closing with MoveOpenTo moves the sprint's unfinished work items first, to the
// next sprint or to the backlog, so that nothing is left behind in a closed one.
func (d *DB) UpdateProjectSprint(ctx context.Context, projectID, sprintID string, patch models.SprintPatch) (*models.TrackerSprint, error) {
	proj, err := d.GetProjectByID(projectID)
	if err != nil || proj == nil {
		return nil, fmt.Errorf("projet non trouvé")
	}
	ts, manager, err := d.sprintManagerOf(proj)
	if err != nil {
		return nil, err
	}
	current, found := findSprint(proj.Sprints, sprintID)
	if !found {
		return nil, fmt.Errorf("sprint %s inconnu de ce projet : synchronisez le board", sprintID)
	}
	if patch.MoveOpenTo != nil {
		closing := patch.State != nil && strings.EqualFold(strings.TrimSpace(*patch.State), "closed")
		if !closing {
			return nil, fmt.Errorf("le déplacement des tickets ouverts n'accompagne que la clôture d'un sprint")
		}
		if err := d.moveOpenWork(ctx, proj, ts, current, strings.TrimSpace(*patch.MoveOpenTo)); err != nil {
			return nil, err
		}
	}
	updated, err := manager.UpdateSprint(ctx, proj, current.ID, patch)
	if err != nil {
		return nil, err
	}
	if updated.ID == "" {
		updated.ID = current.ID
	}
	// The name the tasks carry follows a rename, since they refer to it.
	if updated.Name != "" && updated.Name != current.Name {
		d.mu.Lock()
		_, _ = d.conn.Exec("UPDATE tasks SET sprint = ?, updated_at = ? WHERE project_id = ? AND sprint = ?", updated.Name, time.Now(), proj.ID, current.Name)
		d.mu.Unlock()
	}
	if err := d.mirrorSprints(proj.ID, func(list []models.TrackerSprint) []models.TrackerSprint {
		return replaceSprint(list, updated)
	}); err != nil {
		return nil, err
	}
	return &updated, nil
}

// moveOpenWork moves the unfinished work items of a sprint on the tracker,
// then locally, before the sprint closes.
func (d *DB) moveOpenWork(ctx context.Context, proj *models.Project, ts tracker.TicketingSystem, sprint models.TrackerSprint, destination string) error {
	targetID, targetName := "", ""
	switch destination {
	case "backlog":
	case "next":
		next, ok := nextSprint(proj.Sprints, sprint)
		if !ok {
			return fmt.Errorf("aucun sprint à venir après « %s » : créez-en un, ou déplacez vers le backlog", sprint.Name)
		}
		targetID, targetName = next.ID, next.Name
	default:
		return fmt.Errorf("destination invalide %q : next ou backlog", destination)
	}
	d.mu.RLock()
	rows, err := d.conn.Query("SELECT id, key FROM tasks WHERE project_id = ? AND sprint = ? AND status NOT IN (?, ?)",
		proj.ID, sprint.Name, string(models.StatusFinished), string(models.StatusDone))
	if err != nil {
		d.mu.RUnlock()
		return err
	}
	var ids, keys []string
	for rows.Next() {
		var id, key string
		if rows.Scan(&id, &key) == nil {
			ids, keys = append(ids, id), append(keys, key)
		}
	}
	rows.Close()
	d.mu.RUnlock()
	if len(keys) == 0 {
		return nil
	}
	if err := ts.SetSprint(tracker.WithProject(ctx, proj.ID), targetID, keys); err != nil {
		return fmt.Errorf("déplacement des %d ticket(s) ouvert(s) refusé, le sprint n'est pas clôturé : %w", len(keys), err)
	}
	now := time.Now()
	d.mu.Lock()
	for _, id := range ids {
		_, _ = d.conn.Exec("UPDATE tasks SET sprint = ?, updated_at = ? WHERE id = ?", targetName, now, id)
	}
	d.mu.Unlock()
	return nil
}

// DeleteProjectSprint deletes a sprint on the tracker, then from the mirror.
func (d *DB) DeleteProjectSprint(ctx context.Context, projectID, sprintID string) error {
	proj, err := d.GetProjectByID(projectID)
	if err != nil || proj == nil {
		return fmt.Errorf("projet non trouvé")
	}
	_, manager, err := d.sprintManagerOf(proj)
	if err != nil {
		return err
	}
	if err := manager.DeleteSprint(ctx, proj, sprintID); err != nil {
		return err
	}
	return d.mirrorSprints(proj.ID, func(list []models.TrackerSprint) []models.TrackerSprint {
		kept := list[:0]
		for _, sp := range list {
			if sp.ID != sprintID {
				kept = append(kept, sp)
			}
		}
		return kept
	})
}

// mirrorSprints rewrites the project's sprint list from its current value.
func (d *DB) mirrorSprints(projectID string, change func([]models.TrackerSprint) []models.TrackerSprint) error {
	proj, err := d.GetProjectByID(projectID)
	if err != nil || proj == nil {
		return fmt.Errorf("projet non trouvé")
	}
	list := change(append([]models.TrackerSprint{}, proj.Sprints...))
	_, err = d.UpdateProject(projectID, models.UpdateProjectRequest{Sprints: &list})
	return err
}

func replaceSprint(list []models.TrackerSprint, sprint models.TrackerSprint) []models.TrackerSprint {
	for i := range list {
		if list[i].ID == sprint.ID {
			list[i] = sprint
			return list
		}
	}
	return append(list, sprint)
}

func findSprint(list []models.TrackerSprint, id string) (models.TrackerSprint, bool) {
	for _, sp := range list {
		if sp.ID == strings.TrimSpace(id) {
			return sp, true
		}
	}
	return models.TrackerSprint{}, false
}

// nextSprint is the first sprint not closed that starts after the given one,
// by start date. Dates are compared as instants: two sites may write them with
// different offsets.
func nextSprint(list []models.TrackerSprint, after models.TrackerSprint) (models.TrackerSprint, bool) {
	from, ok := sprintStart(after)
	if !ok {
		return models.TrackerSprint{}, false
	}
	var best models.TrackerSprint
	var bestStart time.Time
	found := false
	for _, sp := range list {
		start, ok := sprintStart(sp)
		if sp.ID == after.ID || sp.State == "closed" || !ok || start.Before(from) {
			continue
		}
		if !found || start.Before(bestStart) {
			best, bestStart, found = sp, start, true
		}
	}
	return best, found
}

func sprintStart(sp models.TrackerSprint) (time.Time, bool) {
	raw := strings.TrimSpace(sp.StartDate)
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05.000Z07:00", "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
