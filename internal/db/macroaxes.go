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
	// ReadinessLabelPrefix is the prefix of the readiness axis (#633). The
	// set of its labels is closed, like the priority's.
	ReadinessLabelPrefix = "readiness:"
)

// ReadinessLevels lists the readiness levels in funnel order. The index is the
// rank used when an epic carries several: the most advanced wins.
var ReadinessLevels = []string{"idea", "shaping", "ready"}

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

// NormalizeReadiness returns "idea", "shaping" or "ready", "" for none, or an
// error naming the value it cannot read. It reads "Ready", "readiness:ready"
// and "#ready" alike.
func NormalizeReadiness(value string) (string, error) {
	clean := cleanLabel(value)
	if clean == "" {
		return "", nil
	}
	level := strings.TrimPrefix(clean, ReadinessLabelPrefix)
	if readinessRank(level) < 0 {
		return "", fmt.Errorf("« %s » n'est pas une readiness d'épic : idea, shaping ou ready attendu", strings.TrimSpace(value))
	}
	return level, nil
}

// readinessRank is the position of a level in ReadinessLevels, -1 when it is
// not one.
func readinessRank(level string) int {
	for i, l := range ReadinessLevels {
		if l == level {
			return i
		}
	}
	return -1
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

// ReadinessLabel is the label of a normalized readiness, "" for none.
func ReadinessLabel(readiness string) string {
	if readiness == "" {
		return ""
	}
	return ReadinessLabelPrefix + readiness
}

// AllReadinessLabels lists the three labels of the readiness axis.
func AllReadinessLabels() []string {
	out := make([]string, 0, len(ReadinessLevels))
	for _, level := range ReadinessLevels {
		out = append(out, ReadinessLabelPrefix+level)
	}
	return out
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

// ReadinessFromLabels reads the readiness from labels, "" when none is valid.
// When an epic carries several, the most advanced wins: an epic somebody
// judged ready is not demoted by a leftover "readiness:idea".
func ReadinessFromLabels(labels []string) string {
	best := -1
	for _, l := range labels {
		clean := cleanLabel(l)
		rest, ok := strings.CutPrefix(clean, ReadinessLabelPrefix)
		if !ok {
			continue
		}
		if rank := readinessRank(rest); rank > best {
			best = rank
		}
	}
	if best < 0 {
		return ""
	}
	return ReadinessLevels[best]
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

// MacroLabelsWritable tells whether the epic axes of a macro are written on the
// tracker. The handler asks before queuing a write, so that a value meant to
// stay in Sectile never produces a failed activity.
func (d *DB) MacroLabelsWritable(projectID string, key string) bool {
	proj, err := d.GetProjectByID(strings.TrimSpace(projectID))
	if err != nil || proj == nil {
		return false
	}
	return macroKeyLabelable(key, proj) && d.epicLabelsSupported(proj)
}

// SaveMacroAxes stores the priority, the quarter and the readiness of a macro. A nil pointer
// leaves that axis as it is; the values are normalized, and an unreadable one
// is refused before anything is written.
//
// It is kept apart from saveMacroMetaFull, whose other fields it never
// touches: an axis edit racing with a framing edit must not write back the
// framing it read.
func (d *DB) SaveMacroAxes(projectID string, key string, priority *string, quarter *string, readiness *string) (*models.MacroMeta, error) {
	projectID = strings.TrimSpace(projectID)
	key = strings.TrimSpace(key)
	if projectID == "" || key == "" {
		return nil, fmt.Errorf("projet et clé de macro obligatoires")
	}
	var cleanPriority, cleanQuarter, cleanReadiness string
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
	if readiness != nil {
		if cleanReadiness, err = NormalizeReadiness(*readiness); err != nil {
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
	var todosJSON, labelsJSON string
	var closedInt int
	// The labels are read too: the client replaces its copy of the macro with
	// the one returned, and a copy without them would hide the epic's free
	// labels until the next reload.
	if err := tx.QueryRow(`
		SELECT horizon, description, framing_comment, todos, title, status, closed, priority, quarter, readiness, labels FROM macros WHERE project_id = ? AND key = ?`+d.forUpdate(),
		projectID, key).Scan(&current.Horizon, &current.Description, &current.FramingComment, &todosJSON, &current.Title, &current.Status, &closedInt, &current.Priority, &current.Quarter, &current.Readiness, &labelsJSON); err != nil {
		d.mu.Unlock()
		return nil, err
	}
	current.Todos = parseMacroTodos(todosJSON)
	current.Labels = parseMacroLabels(labelsJSON)
	current.Closed = closedInt == 1
	if priority != nil {
		current.Priority = cleanPriority
	}
	if quarter != nil {
		current.Quarter = cleanQuarter
	}
	if readiness != nil {
		current.Readiness = cleanReadiness
	}
	current.UpdatedAt = time.Now()
	_, execErr := tx.Exec("UPDATE macros SET priority = ?, quarter = ?, readiness = ?, updated_at = ? WHERE project_id = ? AND key = ?",
		current.Priority, current.Quarter, current.Readiness, current.UpdatedAt, projectID, key)
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
	if err := d.pushMacroLabels(ctx, projectID, macroKey, target, removed); err != nil {
		return "", fmt.Errorf("priorité posée dans Sectile mais pas sur %s : %w", strings.TrimSpace(macroKey), err)
	}
	if target == "" {
		return fmt.Sprintf("labels de priorité retirés de %s", strings.TrimSpace(macroKey)), nil
	}
	return fmt.Sprintf("« %s » posé sur %s", target, strings.TrimSpace(macroKey)), nil
}

// PushMacroReadinessLabel mirrors the readiness onto the tracker's epic: it
// adds the label of the chosen level and removes the other two. An empty
// readiness removes the axis altogether. The set is closed, so the epic is not
// read first, unlike the quarter.
func (d *DB) PushMacroReadinessLabel(ctx context.Context, projectID string, macroKey string, readiness string) (string, error) {
	key := strings.TrimSpace(macroKey)
	target := ReadinessLabel(readiness)
	removed := make([]string, 0, len(ReadinessLevels))
	for _, label := range AllReadinessLabels() {
		if label != target {
			removed = append(removed, label)
		}
	}
	if err := d.pushMacroLabels(ctx, projectID, key, target, removed); err != nil {
		return "", fmt.Errorf("readiness posée dans Sectile mais pas sur %s : %w", key, err)
	}
	if target == "" {
		return fmt.Sprintf("labels de readiness retirés de %s", key), nil
	}
	return fmt.Sprintf("« %s » posé sur %s", target, key), nil
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
	ts, proj, err := d.macroTracker(projectID, key)
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
	if err := d.pushMacroLabels(ctx, projectID, key, target, removed); err != nil {
		return fail(err)
	}
	if target == "" {
		return fmt.Sprintf("labels de trimestre retirés de %s", key), nil
	}
	return fmt.Sprintf("« %s » posé sur %s", target, key), nil
}
