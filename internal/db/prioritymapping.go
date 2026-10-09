package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// ErrInvalidPriorityMapping refuses a priority mapping edit naming an option
// or a level the stored mapping does not carry (#679).
var ErrInvalidPriorityMapping = errors.New("invalid priority mapping")

// priorityMappingTimeout bounds one discovery: a create screen and, at worst,
// the site's priority list.
const priorityMappingTimeout = 30 * time.Second

func parsePriorityMapping(raw string) models.PriorityMapping {
	var out models.PriorityMapping
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &out) != nil {
		return models.PriorityMapping{}
	}
	return out
}

// hasPriorityMapping reports whether the project's priority writes go through
// its mapping: a Jira project whose mapping was discovered. Before the first
// discovery, and on every other tracker, writes stay as they were.
func hasPriorityMapping(proj *models.Project) bool {
	return proj != nil && proj.IssueTracker == "jira" && !proj.PriorityMapping.Empty()
}

// CheckPriorityWritable refuses a priority the project's mapping only guessed,
// with a *models.GuessedPriorityError naming the levels it accepts. It is
// called before any local write, so a refused update leaves the card as it was
// and the next sync has nothing to undo.
func CheckPriorityWritable(proj *models.Project, level models.Priority) error {
	if !hasPriorityMapping(proj) || proj.PriorityMapping.Writable(level) {
		return nil
	}
	return &models.GuessedPriorityError{Level: level, Writable: proj.PriorityMapping.WritableLevels()}
}

// mappingProjectUnsafe is proj when its priority mapping governs a write on a
// ticket of trk, nil otherwise. The mapping stays on the project (#741) and
// was read from the project's default tracker, so a ticket of another of its
// trackers, another Jira space, writes its priority as it did before #679.
func (d *DB) mappingProjectUnsafe(proj *models.Project, trk *models.Tracker) *models.Project {
	if proj == nil {
		return nil
	}
	if trk != nil {
		if def := d.trackerOfProjectUnsafe(proj); def == nil || def.ID != trk.ID {
			return nil
		}
	}
	return proj
}

// priorityMappingForUnsafe is the mapping a priority sent to trk for proj goes
// through (#679), empty when the project has none that applies.
func (d *DB) priorityMappingForUnsafe(proj *models.Project, trk *models.Tracker) models.PriorityMapping {
	if p := d.mappingProjectUnsafe(proj, trk); hasPriorityMapping(p) {
		return p.PriorityMapping
	}
	return models.PriorityMapping{}
}

// editPriorityMapping applies a person's edit of the mapping to the stored
// one. Only the levels and the preferred options come from the edit: the
// options themselves, their names and their order are the tracker's, and an
// option the edit leaves out keeps its line. A line whose level changes, or
// whose guess is confirmed, becomes sure and hand-set, so no discovery
// changes it again.
func editPriorityMapping(stored, sent models.PriorityMapping) (models.PriorityMapping, error) {
	byID := make(map[string]models.PriorityMappingOption, len(sent.Options))
	for _, o := range sent.Options {
		if !models.IsPriorityLevel(o.Level) {
			return stored, fmt.Errorf("%w: %q is not a priority level", ErrInvalidPriorityMapping, o.Level)
		}
		byID[o.ID] = o
	}
	known := make(map[string]bool, len(stored.Options))
	out := models.PriorityMapping{Options: make([]models.PriorityMappingOption, 0, len(stored.Options))}
	for _, line := range stored.Options {
		known[line.ID] = true
		if edit, ok := byID[line.ID]; ok && (edit.Level != line.Level || (line.Guessed && !edit.Guessed)) {
			line.Level = edit.Level
			line.Guessed = false
			line.Manual = true
		}
		out.Options = append(out.Options, line)
	}
	for id := range byID {
		if !known[id] {
			return stored, fmt.Errorf("%w: the project has no priority option %q", ErrInvalidPriorityMapping, id)
		}
	}
	for level, id := range sent.Preferred {
		if !models.IsPriorityLevel(level) {
			return stored, fmt.Errorf("%w: %q is not a priority level", ErrInvalidPriorityMapping, level)
		}
		if id == "" {
			continue
		}
		if !known[id] {
			return stored, fmt.Errorf("%w: the project has no priority option %q", ErrInvalidPriorityMapping, id)
		}
		if out.Preferred == nil {
			out.Preferred = map[models.Priority]string{}
		}
		out.Preferred[level] = id
	}
	return out, nil
}

// RefreshPriorityMapping reads the project's priority scheme and folds it into
// the stored mapping (#679): new options are added, classified sure or
// guessed, options gone from the scheme are dropped, and no stored line
// changes. It answers a one-line summary, and "" for a tracker that has no
// priority scheme to map. fresh skips the adapter's caches, for a refresh a
// person asked for.
//
// The mapping stays on the project while tickets come from trackers (#741):
// the scheme is read from the project's default tracker, the one its
// IssueTracker reads through.
func (d *DB) RefreshPriorityMapping(ctx context.Context, projectID string, fresh bool) (string, error) {
	proj, err := d.GetProjectByID(projectID)
	if err != nil {
		return "", err
	}
	if proj == nil {
		return "", fmt.Errorf("project not found")
	}
	trk := d.trackerOfProjectUnsafe(proj)
	if trk == nil || trk.Provider != "jira" {
		return "", nil
	}
	ts, _, err := d.trackerReaderOf(trk)
	if err != nil {
		return "", err
	}
	reader, ok := ts.(tracker.PrioritySchemeReader)
	if !ok {
		return "", nil
	}
	ctx, cancel := context.WithTimeout(ctx, priorityMappingTimeout)
	defer cancel()
	scheme, err := reader.PriorityScheme(tracker.WithTracker(ctx, trk), trk, fresh)
	if err != nil {
		return "", err
	}
	if len(scheme) == 0 {
		return "", fmt.Errorf("the tracker lists no priority for project %s", proj.Name)
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	// Merged against the row as it is now, not as it was before the tracker
	// answered: an edit saved meanwhile is kept.
	current, err := d.getProjectByIDUnsafe(proj.ID)
	if err != nil {
		return "", err
	}
	if current == nil {
		return "", fmt.Errorf("project not found")
	}
	merged := models.MergePriorityMapping(current.PriorityMapping, scheme, reader.ClassifyPriority)
	raw, _ := json.Marshal(merged)
	if _, err := d.conn.Exec("UPDATE projects SET priority_mapping = ?, updated_at = ? WHERE id = ?", string(raw), time.Now(), proj.ID); err != nil {
		return "", err
	}
	guessed := 0
	for _, o := range merged.Options {
		if o.Guessed {
			guessed++
		}
	}
	return fmt.Sprintf("%d priorities, %d guessed", len(merged.Options), guessed), nil
}
