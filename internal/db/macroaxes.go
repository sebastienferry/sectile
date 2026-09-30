package db

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// An epic carries two axes of its own besides the roadmap horizon: a priority
// and a quarter (#627). They follow the horizon's model: decided in Sectile,
// stored on the macro, then written on the tracker's epic as labels when the
// tracker can carry them, and read back from those labels.
//
// The labels are the only thing written. A project whose epics cannot carry a
// label (a GitHub milestone, a local key, an epic of another project, a
// tracker with no epics) keeps both values in Sectile only.
//
// #626 protects the label prefixes the roadmap owns from the free label
// editor. "priority:", "quarter:" and the bare "2026-Q3" form are among them,
// whichever of the two tickets lands second adds them to that list.
const (
	PriorityLabelPrefix = "priority:"
	QuarterLabelPrefix  = "quarter:"
)

var (
	// epicPriorityPattern reads "p1", "P1", "priority:p1".
	epicPriorityPattern = regexp.MustCompile(`^(?:priority:)?p([0-3])$`)
	// quarterPattern reads "2026-Q4", "2026.q4", "2026 Q4". The year has four
	// digits and is not bounded: a roadmap runs on as many years as it wants.
	quarterPattern = regexp.MustCompile(`^(\d{4})[.\- ]q([1-4])$`)
)

// NormalizeEpicPriority returns "p0" to "p3", "" for none, or an error naming
// the value it cannot read.
func NormalizeEpicPriority(value string) (string, error) {
	clean := strings.ToLower(strings.TrimSpace(value))
	if clean == "" {
		return "", nil
	}
	m := epicPriorityPattern.FindStringSubmatch(clean)
	if m == nil {
		return "", fmt.Errorf("« %s » n'est pas une priorité d'épic : P0, P1, P2 ou P3 attendu", strings.TrimSpace(value))
	}
	return "p" + m[1], nil
}

// NormalizeQuarter returns "2026-Q4", "" for none, or an error naming the value
// it cannot read.
func NormalizeQuarter(value string) (string, error) {
	clean := strings.ToLower(strings.TrimSpace(value))
	if clean == "" {
		return "", nil
	}
	m := quarterPattern.FindStringSubmatch(clean)
	if m == nil {
		return "", fmt.Errorf("« %s » n'est pas un trimestre : AAAA-Qn attendu, par exemple 2026-Q4", strings.TrimSpace(value))
	}
	return m[1] + "-Q" + m[2], nil
}

// PriorityLabel is the label of a normalized priority, "" for none.
func PriorityLabel(priority string) string {
	if priority == "" {
		return ""
	}
	return PriorityLabelPrefix + priority
}

// QuarterLabel is the prefixed label of a normalized quarter, "" for none. It
// is the only form Sectile writes.
func QuarterLabel(quarter string) string {
	if quarter == "" {
		return ""
	}
	return QuarterLabelPrefix + strings.ToLower(quarter)
}

// AllPriorityLabels lists the four labels of the priority axis.
func AllPriorityLabels() []string {
	return []string{PriorityLabelPrefix + "p0", PriorityLabelPrefix + "p1", PriorityLabelPrefix + "p2", PriorityLabelPrefix + "p3"}
}

func cleanLabel(label string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(label), "#")))
}

// PriorityFromLabels reads the priority from labels, "" when none is valid.
func PriorityFromLabels(labels []string) string {
	for _, l := range labels {
		clean := cleanLabel(l)
		if !strings.HasPrefix(clean, PriorityLabelPrefix) {
			continue
		}
		if p, err := NormalizeEpicPriority(clean); err == nil && p != "" {
			return p
		}
	}
	return ""
}

// QuarterFromLabels reads the quarter from labels: the first valid prefixed
// label, else the first valid bare one, else "".
//
// The bare form is the convention teams used before any tool wrote the axis,
// and an epic does not stop belonging to a quarter for lack of a prefix. The
// prefixed form wins a disagreement: it is the one Sectile writes, so the most
// recently decided.
func QuarterFromLabels(labels []string) string {
	bare := ""
	for _, l := range labels {
		clean := cleanLabel(l)
		if rest, ok := strings.CutPrefix(clean, QuarterLabelPrefix); ok {
			if q, err := NormalizeQuarter(rest); err == nil && q != "" {
				return q
			}
			continue
		}
		if bare == "" {
			if q, err := NormalizeQuarter(clean); err == nil && q != "" && !strings.Contains(clean, " ") {
				bare = q
			}
		}
	}
	return bare
}

// isQuarterLabel tells a label of the quarter axis, in either form, from any
// other label. A "quarter:" label whose value is not a quarter still belongs
// to the axis: setting a quarter replaces it.
func isQuarterLabel(label string) bool {
	clean := cleanLabel(label)
	if strings.HasPrefix(clean, QuarterLabelPrefix) {
		return true
	}
	q, err := NormalizeQuarter(clean)
	return err == nil && q != "" && !strings.Contains(clean, " ")
}

// macroKeyLabelable tells whether a macro key names an epic of this project
// that can carry a label: not a milestone, not a local "M-<n>" key, not an epic
// of another project. Whether the tracker itself can is epicLabelsSupported.
func macroKeyLabelable(key string, proj *models.Project) bool {
	if proj == nil || isMilestoneKey(key) {
		return false
	}
	return belongsToProject(key, proj)
}

// epicLabelsSupported tells whether the project's tracker has epics and can
// write labels on them. It is false for a tracker that cannot be resolved: a
// value that cannot leave Sectile is kept in Sectile, which is the answer the
// panel has to give.
func (d *DB) epicLabelsSupported(proj *models.Project) bool {
	if proj == nil {
		return false
	}
	ts, err := d.TrackerForProject(proj)
	if err != nil || ts == nil {
		return false
	}
	return ts.Supports(tracker.CapEpic) && ts.Supports(tracker.CapUpdate) && ts.Supports(tracker.CapLabels)
}

// MacroLabelsWritable tells whether the labels of a macro, its horizon among
// them, are written on the tracker. The handler asks before queuing a write, so
// that a value meant to stay in Sectile never produces a failed activity.
func (d *DB) MacroLabelsWritable(projectID string, key string) bool {
	proj, err := d.GetProjectByID(strings.TrimSpace(projectID))
	if err != nil || proj == nil {
		return false
	}
	return macroKeyLabelable(key, proj) && d.epicLabelsSupported(proj)
}

// MacroAxesWritable tells whether an edit of the priority or the quarter of a
// macro is written on the tracker. It is MacroLabelsWritable, plus the epics of
// a roadmap project once the project opted in (#632), and only for an edit of
// one epic: bulk is an edit that covers several, such as the title seeding,
// and never reaches another team's epics.
func (d *DB) MacroAxesWritable(projectID string, key string, bulk bool) bool {
	proj, err := d.GetProjectByID(strings.TrimSpace(projectID))
	if err != nil || proj == nil {
		return false
	}
	return macroAxesWritable(key, proj, d.epicLabelsSupported(proj), bulk)
}

// FillMacroFlags sets the computed fields of a macro a handler returns, the
// ones GetProjectMacros fills on a list: whether its labels and its axes are
// written on the tracker, its origin and whether it is foreign. bulk is the
// answer for an edit covering several epics, which never writes on a foreign
// one.
func (d *DB) FillMacroFlags(projectID string, m *models.MacroMeta, bulk bool) {
	if m == nil {
		return
	}
	proj, err := d.GetProjectByID(strings.TrimSpace(projectID))
	if err != nil || proj == nil {
		return
	}
	supported := d.epicLabelsSupported(proj)
	m.LabelsWritable = supported && macroKeyLabelable(m.Key, proj)
	fillMacroOrigin(m, proj, supported)
	m.AxesWritable = macroAxesWritable(m.Key, proj, supported, bulk)
}

// macroAxis names the epic write a macroTracker call is for: they do not
// all open the same way on an epic of a roadmap project.
type macroAxis int

const (
	axisHorizon macroAxis = iota
	axisLabels
	axisPriority
	axisQuarter
)

// foreignAxisWritable tells whether an axis may be written on an epic of a
// roadmap project: only the priority and the quarter, only once the project
// opened RoadmapAxisWrites, and only while the epic's project is still
// declared. A project removed from the declaration takes the right with it.
func foreignAxisWritable(key string, proj *models.Project, axis macroAxis) bool {
	if axis != axisPriority && axis != axisQuarter {
		return false
	}
	return proj != nil && proj.RoadmapAxisWrites && isForeignMacro(key, proj) && isDeclaredRoadmapProject(proj, macroOrigin(key))
}

// macroAxesWritable is MacroAxesWritable once the project and the tracker's
// support are known, so that a list resolves the tracker once.
func macroAxesWritable(key string, proj *models.Project, supported bool, bulk bool) bool {
	if !supported {
		return false
	}
	if macroKeyLabelable(key, proj) {
		return true
	}
	return !bulk && foreignAxisWritable(key, proj, axisPriority)
}

// SaveMacroAxes stores the priority and the quarter of a macro. A nil pointer
// leaves that axis as it is; the values are normalized, and an unreadable one
// is refused before anything is written.
//
// It is kept apart from saveMacroMetaFull, whose other fields it never
// touches: an axis edit racing with a framing edit must not write back the
// framing it read.
func (d *DB) SaveMacroAxes(projectID string, key string, priority *string, quarter *string) (*models.MacroMeta, error) {
	projectID = strings.TrimSpace(projectID)
	key = strings.TrimSpace(key)
	if projectID == "" || key == "" {
		return nil, fmt.Errorf("projet et clé de macro obligatoires")
	}
	var cleanPriority, cleanQuarter string
	var err error
	if priority != nil {
		if cleanPriority, err = NormalizeEpicPriority(*priority); err != nil {
			return nil, err
		}
	}
	if quarter != nil {
		if cleanQuarter, err = NormalizeQuarter(*quarter); err != nil {
			return nil, err
		}
	}

	d.mu.Lock()
	d.ensureMacrosTable()
	tx, err := d.conn.Begin()
	if err != nil {
		d.mu.Unlock()
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec("INSERT INTO macros (project_id, key) VALUES (?, ?) ON CONFLICT (project_id, key) DO NOTHING", projectID, key); err != nil {
		d.mu.Unlock()
		return nil, err
	}
	current := models.MacroMeta{ProjectID: projectID, Key: key, Todos: []models.MacroTodo{}}
	var todosJSON string
	var closedInt int
	if err := tx.QueryRow(`
		SELECT horizon, description, framing_comment, todos, title, status, closed, priority, quarter FROM macros WHERE project_id = ? AND key = ?`+d.forUpdate(),
		projectID, key).Scan(&current.Horizon, &current.Description, &current.FramingComment, &todosJSON, &current.Title, &current.Status, &closedInt, &current.Priority, &current.Quarter); err != nil {
		d.mu.Unlock()
		return nil, err
	}
	current.Todos = parseMacroTodos(todosJSON)
	current.Closed = closedInt == 1
	if priority != nil {
		current.Priority = cleanPriority
	}
	if quarter != nil {
		current.Quarter = cleanQuarter
	}
	current.UpdatedAt = time.Now()
	_, execErr := tx.Exec("UPDATE macros SET priority = ?, quarter = ?, updated_at = ? WHERE project_id = ? AND key = ?",
		current.Priority, current.Quarter, current.UpdatedAt, projectID, key)
	if execErr == nil {
		execErr = tx.Commit()
	}
	d.mu.Unlock()
	if execErr != nil {
		return nil, execErr
	}
	return &current, nil
}

// PushMacroPriorityLabel mirrors the priority onto the tracker's epic: it adds
// the label of the chosen priority and removes the other three. An empty
// priority removes the axis altogether. Run from a queued activity only, as
// PushMacroHorizonLabel is.
func (d *DB) PushMacroPriorityLabel(ctx context.Context, projectID string, macroKey string, priority string) (string, error) {
	target := PriorityLabel(priority)
	removed := make([]string, 0, 4)
	for _, label := range AllPriorityLabels() {
		if label != target {
			removed = append(removed, label)
		}
	}
	if err := d.pushMacroLabels(ctx, projectID, macroKey, axisPriority, target, removed); err != nil {
		return "", fmt.Errorf("priorité posée dans Sectile mais pas sur %s : %w", strings.TrimSpace(macroKey), err)
	}
	if target == "" {
		return fmt.Sprintf("labels de priorité retirés de %s", strings.TrimSpace(macroKey)), nil
	}
	return fmt.Sprintf("« %s » posé sur %s", target, strings.TrimSpace(macroKey)), nil
}

// PushMacroQuarterLabel mirrors the quarter onto the tracker's epic: it adds the
// prefixed label and removes every other quarter label, in both forms. An
// empty quarter removes the axis altogether.
//
// The epic is read first. The bare form is an open set, "2026-Q3" and every
// other year, so the labels to remove cannot be listed without looking; and
// leaving "2026-Q3" beside a freshly set "quarter:2026-q4" would have the
// ticket carry two contradicting quarters for whoever opens it on the tracker.
func (d *DB) PushMacroQuarterLabel(ctx context.Context, projectID string, macroKey string, quarter string) (string, error) {
	key := strings.TrimSpace(macroKey)
	fail := func(err error) (string, error) {
		return "", fmt.Errorf("trimestre posé dans Sectile mais pas sur %s : %w", key, err)
	}
	ts, proj, err := d.macroTracker(projectID, key, axisQuarter)
	if err != nil {
		return fail(err)
	}
	readCtx, cancel := context.WithTimeout(ctx, macroWriteTimeout)
	epic, err := ts.GetIssue(readCtx, tracker.GetIssueRequest{Project: proj, Key: key})
	cancel()
	if err != nil {
		return fail(err)
	}
	target := QuarterLabel(quarter)
	removed := []string{}
	if epic != nil {
		for _, label := range epic.Labels {
			if isQuarterLabel(label) && cleanLabel(label) != target {
				removed = append(removed, label)
			}
		}
	}
	if err := d.pushMacroLabels(ctx, projectID, key, axisQuarter, target, removed); err != nil {
		return fail(err)
	}
	if target == "" {
		return fmt.Sprintf("labels de trimestre retirés de %s", key), nil
	}
	return fmt.Sprintf("« %s » posé sur %s", target, key), nil
}
