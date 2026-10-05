package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

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
// editor. A project may name its own prefixes for the three axes (#635); the
// constants below are the defaults of one that names none.
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

// axisPrefixes are the label prefixes a project reads and writes its epic axes
// under (#635), the defaults filled in. Every read and write of an axis label
// goes through one, so that a project naming "prio-" never sees "priority:".
type axisPrefixes struct {
	priority  string
	quarter   string
	readiness string
}

// defaultAxisPrefixes are the prefixes of a project that names none.
var defaultAxisPrefixes = axisPrefixes{priority: PriorityLabelPrefix, quarter: QuarterLabelPrefix, readiness: ReadinessLabelPrefix}

// prefixesFor resolves the prefixes of a project, the default of each axis it
// leaves empty. A nil project reads under the defaults.
func prefixesFor(proj *models.Project) axisPrefixes {
	out := defaultAxisPrefixes
	if proj == nil {
		return out
	}
	if v := proj.EpicAxisPrefixes.Priority; v != "" {
		out.priority = v
	}
	if v := proj.EpicAxisPrefixes.Quarter; v != "" {
		out.quarter = v
	}
	if v := proj.EpicAxisPrefixes.Readiness; v != "" {
		out.readiness = v
	}
	return out
}

// projectAxisPrefixes resolves the prefixes of a stored project, the defaults
// when it cannot be read: the write that follows fails on the project anyway.
func (d *DB) projectAxisPrefixes(projectID string) axisPrefixes {
	proj, err := d.GetProjectByID(strings.TrimSpace(projectID))
	if err != nil {
		return defaultAxisPrefixes
	}
	return prefixesFor(proj)
}

// ErrInvalidEpicAxisPrefix refuses an epic axis prefix a project cannot hold
// (#635). The refusal reads as a sentence naming the axis and the prefix.
var ErrInvalidEpicAxisPrefix = errors.New("invalid epic axis prefix")

type epicAxisPrefixError struct{ msg string }

func (e epicAxisPrefixError) Error() string        { return e.msg }
func (e epicAxisPrefixError) Is(target error) bool { return target == ErrInvalidEpicAxisPrefix }

// CleanEpicAxisPrefixes cleans the prefixes a person typed before they are
// stored: trimmed, lower-cased, a leading "#" removed. It refuses a prefix
// carrying whitespace, a prefix emptied by the cleaning, two axes whose
// prefixes start one another, and a prefix overlapping "roadmap:", the horizon
// prefix, which stays fixed. An empty prefix is the default of its axis, and
// the overlap is checked on the prefixes in effect.
func CleanEpicAxisPrefixes(in models.EpicAxisPrefixes) (models.EpicAxisPrefixes, error) {
	type axis struct {
		name     string
		typed    string
		fallback string
		clean    string
	}
	axes := []*axis{
		{name: "priorité", typed: in.Priority, fallback: PriorityLabelPrefix},
		{name: "trimestre", typed: in.Quarter, fallback: QuarterLabelPrefix},
		{name: "readiness", typed: in.Readiness, fallback: ReadinessLabelPrefix},
	}
	for _, a := range axes {
		typed := strings.TrimSpace(a.typed)
		if typed == "" {
			continue
		}
		clean := strings.ToLower(strings.TrimPrefix(typed, "#"))
		if strings.ContainsFunc(clean, unicode.IsSpace) {
			return models.EpicAxisPrefixes{}, epicAxisPrefixError{fmt.Sprintf("le préfixe « %s » de l'axe %s ne peut pas contenir d'espace", typed, a.name)}
		}
		if clean == "" {
			return models.EpicAxisPrefixes{}, epicAxisPrefixError{fmt.Sprintf("le préfixe de l'axe %s est vide une fois nettoyé : laissez le champ vide pour garder « %s »", a.name, a.fallback)}
		}
		a.clean = clean
	}
	effective := func(a *axis) string {
		if a.clean != "" {
			return a.clean
		}
		return a.fallback
	}
	overlap := func(x, y string) bool { return strings.HasPrefix(x, y) || strings.HasPrefix(y, x) }
	for i, a := range axes {
		if overlap(effective(a), RoadmapLabelPrefix) {
			return models.EpicAxisPrefixes{}, epicAxisPrefixError{fmt.Sprintf("le préfixe « %s » de l'axe %s empiète sur « %s », réservé à l'horizon", effective(a), a.name, RoadmapLabelPrefix)}
		}
		for _, b := range axes[i+1:] {
			if overlap(effective(a), effective(b)) {
				return models.EpicAxisPrefixes{}, epicAxisPrefixError{fmt.Sprintf("les préfixes des axes %s (« %s ») et %s (« %s ») se recouvrent : un label ne saurait pas à quel axe il appartient", a.name, effective(a), b.name, effective(b))}
			}
		}
	}
	return models.EpicAxisPrefixes{Priority: axes[0].clean, Quarter: axes[1].clean, Readiness: axes[2].clean}, nil
}

// parseEpicAxisPrefixes reads the stored column, none on an unreadable value.
func parseEpicAxisPrefixes(raw string) models.EpicAxisPrefixes {
	var out models.EpicAxisPrefixes
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &out) != nil {
		return models.EpicAxisPrefixes{}
	}
	return out
}

// priorityValuePattern is the value that follows the priority prefix. It is
// stricter than epicPriorityPattern, which also reads a typed "priority:p1":
// under "prio-", "prio-priority:p1" is no priority.
var priorityValuePattern = regexp.MustCompile(`^p[0-3]$`)

// PriorityLabel is the label of a normalized priority, "" for none.
func (a axisPrefixes) PriorityLabel(priority string) string {
	if priority == "" {
		return ""
	}
	return a.priority + priority
}

// QuarterLabel is the prefixed label of a normalized quarter, "" for none. It
// is the only form Sectile writes.
func (a axisPrefixes) QuarterLabel(quarter string) string {
	if quarter == "" {
		return ""
	}
	return a.quarter + strings.ToLower(quarter)
}

// ReadinessLabel is the label of a normalized readiness, "" for none.
func (a axisPrefixes) ReadinessLabel(readiness string) string {
	if readiness == "" {
		return ""
	}
	return a.readiness + readiness
}

// AllReadinessLabels lists the three labels of the readiness axis.
func (a axisPrefixes) AllReadinessLabels() []string {
	out := make([]string, 0, len(ReadinessLevels))
	for _, level := range ReadinessLevels {
		out = append(out, a.readiness+level)
	}
	return out
}

// AllPriorityLabels lists the four labels of the priority axis.
func (a axisPrefixes) AllPriorityLabels() []string {
	return []string{a.priority + "p0", a.priority + "p1", a.priority + "p2", a.priority + "p3"}
}

func cleanLabel(label string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(label), "#")))
}

// PriorityFromLabels reads the priority from labels, "" when none is valid.
// The value always follows the full prefix: under "p", "pp1" is P1 and "p1"
// is nothing.
func (a axisPrefixes) PriorityFromLabels(labels []string) string {
	for _, l := range labels {
		rest, ok := strings.CutPrefix(cleanLabel(l), a.priority)
		if ok && priorityValuePattern.MatchString(rest) {
			return rest
		}
	}
	return ""
}

// ReadinessFromLabels reads the readiness from labels, "" when none is valid.
// When an epic carries several, the most advanced wins: an epic somebody
// judged ready is not demoted by a leftover "readiness:idea".
func (a axisPrefixes) ReadinessFromLabels(labels []string) string {
	best := -1
	for _, l := range labels {
		clean := cleanLabel(l)
		rest, ok := strings.CutPrefix(clean, a.readiness)
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
// and an epic does not stop belonging to a quarter for lack of a prefix. It is
// read whatever the project's quarter prefix. The prefixed form wins a
// disagreement: it is the one Sectile writes, so the most recently decided.
func (a axisPrefixes) QuarterFromLabels(labels []string) string {
	bare := ""
	for _, l := range labels {
		clean := cleanLabel(l)
		if rest, ok := strings.CutPrefix(clean, a.quarter); ok {
			if q, err := NormalizeQuarter(rest); err == nil && q != "" {
				return q
			}
			continue
		}
		if bare == "" && isBareQuarter(clean) {
			bare, _ = NormalizeQuarter(clean)
		}
	}
	return bare
}

// isBareQuarter tells a cleaned label that is a quarter with no prefix.
func isBareQuarter(clean string) bool {
	q, err := NormalizeQuarter(clean)
	return err == nil && q != "" && !strings.Contains(clean, " ")
}

// isQuarterLabel tells a label of the quarter axis, in either form, from any
// other label. A label under the quarter prefix whose value is not a quarter
// still belongs to the axis: setting a quarter replaces it.
func (a axisPrefixes) isQuarterLabel(label string) bool {
	clean := cleanLabel(label)
	return strings.HasPrefix(clean, a.quarter) || isBareQuarter(clean)
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
	d.fillMacroCopies(proj, m)
}

// macroAxis names the epic write a macroTracker call is for: they do not
// all open the same way on an epic of a roadmap project.
type macroAxis int

const (
	axisHorizon macroAxis = iota
	axisLabels
	axisPriority
	axisQuarter
	// axisReadiness is the readiness a person decides (#633), which a roadmap
	// project's epic never opens to.
	axisReadiness
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
	// Only the labels under the current prefix are removed (#635): one left
	// under a former prefix is a free label now, and stays.
	prefixes := d.projectAxisPrefixes(projectID)
	target := prefixes.PriorityLabel(priority)
	removed := make([]string, 0, 4)
	for _, label := range prefixes.AllPriorityLabels() {
		if label != target {
			removed = append(removed, label)
		}
	}
	if err := d.pushMacroLabels(ctx, projectID, macroKey, axisPriority, target, removed); err != nil {
		return "", fmt.Errorf("priorité posée dans Sectile mais pas sur %s : %w", strings.TrimSpace(macroKey), err)
	}
	output := fmt.Sprintf("« %s » posé sur %s", target, strings.TrimSpace(macroKey))
	if target == "" {
		output = fmt.Sprintf("labels de priorité retirés de %s", strings.TrimSpace(macroKey))
	}
	return d.withEpicAxisField(ctx, projectID, macroKey, models.EpicAxisPriority, axisPriority, priority, output)
}

// withEpicAxisField writes the axis's mapped field once its label went
// through (#680), and appends what it did to the label's sentence. A field
// failure fails the push: the epic then stays in the pending pushes.
func (d *DB) withEpicAxisField(ctx context.Context, projectID, macroKey, axis string, mAxis macroAxis, value, output string) (string, error) {
	note, err := d.pushEpicAxisField(ctx, projectID, macroKey, axis, mAxis, value)
	if err != nil {
		return "", err
	}
	if note != "" {
		output += " ; " + note
	}
	return output, nil
}

// PushMacroReadinessLabel mirrors the readiness onto the tracker's epic: it
// adds the label of the chosen level and removes the other two. An empty
// readiness removes the axis altogether. The set is closed, so the epic is not
// read first, unlike the quarter.
func (d *DB) PushMacroReadinessLabel(ctx context.Context, projectID string, macroKey string, readiness string) (string, error) {
	key := strings.TrimSpace(macroKey)
	prefixes := d.projectAxisPrefixes(projectID)
	target := prefixes.ReadinessLabel(readiness)
	removed := make([]string, 0, len(ReadinessLevels))
	for _, label := range prefixes.AllReadinessLabels() {
		if label != target {
			removed = append(removed, label)
		}
	}
	if err := d.pushMacroLabels(ctx, projectID, key, axisReadiness, target, removed); err != nil {
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
	ts, proj, err := d.macroTracker(projectID, key, axisQuarter)
	if err != nil {
		return fail(err)
	}
	readCtx, cancel := context.WithTimeout(ctx, macroWriteTimeout)
	epic, err := ts.GetIssue(readCtx, tracker.GetIssueRequest{Tracker: d.trackerOfProjectUnsafe(proj), Key: key})
	cancel()
	if err != nil {
		return fail(err)
	}
	prefixes := prefixesFor(proj)
	target := prefixes.QuarterLabel(quarter)
	removed := []string{}
	if epic != nil {
		for _, label := range epic.Labels {
			if prefixes.isQuarterLabel(label) && cleanLabel(label) != target {
				removed = append(removed, label)
			}
		}
	}
	if err := d.pushMacroLabels(ctx, projectID, key, axisQuarter, target, removed); err != nil {
		return fail(err)
	}
	output := fmt.Sprintf("« %s » posé sur %s", target, key)
	if target == "" {
		output = fmt.Sprintf("labels de trimestre retirés de %s", key)
	}
	return d.withEpicAxisField(ctx, projectID, key, models.EpicAxisQuarter, axisQuarter, quarter, output)
}
