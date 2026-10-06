package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// A Jira project may carry its epic priority and quarter in custom fields of
// its epics as well as in labels (#680). The field, its kind and its option
// map are project data a person picked from an epic's edit screen: nothing
// about a field is compiled in, and a project that maps none keeps the labels
// only, with no extra tracker request.

// ErrInvalidEpicAxisFields refuses an axis field setting a project cannot
// hold. The refusal reads as a sentence naming the axis.
var ErrInvalidEpicAxisFields = errors.New("invalid epic axis fields")

type epicAxisFieldsError struct{ msg string }

func (e epicAxisFieldsError) Error() string        { return e.msg }
func (e epicAxisFieldsError) Is(target error) bool { return target == ErrInvalidEpicAxisFields }

// epicAxisFieldsTimeout bounds one read of an epic's edit screen.
const epicAxisFieldsTimeout = 30 * time.Second

func parseEpicAxisFields(raw string) models.EpicAxisFields {
	var out models.EpicAxisFields
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &out) != nil {
		return models.EpicAxisFields{}
	}
	return out
}

// normalizeEpicAxisValue reads an axis value of a map line, "" with an error
// when it is not one.
func normalizeEpicAxisValue(axis, value string) (string, error) {
	var clean string
	var err error
	if axis == models.EpicAxisPriority {
		clean, err = NormalizeEpicPriority(value)
	} else {
		clean, err = NormalizeQuarter(value)
	}
	if err == nil && clean == "" {
		err = fmt.Errorf("an empty value has no option")
	}
	return clean, err
}

// cleanEpicAxisField checks one axis's field as a person sent it and returns
// it normalized: values in their stored form, option paths trimmed.
func cleanEpicAxisField(axis string, in models.EpicAxisField) (models.EpicAxisField, error) {
	refuse := func(format string, args ...any) (models.EpicAxisField, error) {
		return models.EpicAxisField{}, epicAxisFieldsError{fmt.Sprintf("%s field: ", axis) + fmt.Sprintf(format, args...)}
	}
	out := models.EpicAxisField{ID: strings.TrimSpace(in.ID), Name: strings.TrimSpace(in.Name), Kind: strings.TrimSpace(in.Kind)}
	if out.ID == "" {
		return refuse("the field id is empty")
	}
	if out.Kind != models.EpicFieldSelect && out.Kind != models.EpicFieldCascade {
		return refuse("kind %q is neither %q nor %q", in.Kind, models.EpicFieldSelect, models.EpicFieldCascade)
	}
	for value, path := range in.Options {
		clean, err := normalizeEpicAxisValue(axis, value)
		if err != nil {
			return refuse("%v", err)
		}
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if out.Kind == models.EpicFieldCascade {
			parent, child, ok := strings.Cut(path, "/")
			if !ok || parent == "" || child == "" || strings.Contains(child, "/") {
				return refuse("option %q of a cascading field is not \"parent/child\"", path)
			}
		}
		if out.Options == nil {
			out.Options = map[string]string{}
		}
		out.Options[clean] = path
	}
	for _, value := range in.Manual {
		clean, err := normalizeEpicAxisValue(axis, value)
		if err != nil {
			return refuse("%v", err)
		}
		out.Manual = appendUnique(out.Manual, clean)
	}
	return out, nil
}

func appendUnique(list []string, value string) []string {
	if slices.Contains(list, value) {
		return list
	}
	return append(list, value)
}

// editEpicAxisFields applies a person's edit of the axis fields to the stored
// ones. A nil axis removes its field. Another field replaces the whole entry,
// its map included. An edit of the stored field's map marks every value whose
// line was set, changed or cleared as set by hand, so that no deduction fills
// or changes it again.
func editEpicAxisFields(stored, sent models.EpicAxisFields) (models.EpicAxisFields, error) {
	out := models.EpicAxisFields{}
	for _, axis := range []string{models.EpicAxisPriority, models.EpicAxisQuarter} {
		in := sent.Field(axis)
		if in == nil {
			continue
		}
		field, err := cleanEpicAxisField(axis, *in)
		if err != nil {
			return stored, err
		}
		if before := stored.Field(axis); before != nil && before.ID == field.ID {
			field.Manual = mergeManual(before, field)
		}
		sort.Strings(field.Manual)
		if axis == models.EpicAxisPriority {
			out.Priority = &field
		} else {
			out.Quarter = &field
		}
	}
	return out, nil
}

// mergeManual is the hand-set values of an edited map: those already hand-set
// and those whose line differs from the stored one.
func mergeManual(before *models.EpicAxisField, after models.EpicAxisField) []string {
	manual := append([]string{}, before.Manual...)
	for _, value := range after.Manual {
		manual = appendUnique(manual, value)
	}
	for value, path := range after.Options {
		if before.Options[value] != path {
			manual = appendUnique(manual, value)
		}
	}
	for value := range before.Options {
		if after.Options[value] == "" {
			manual = appendUnique(manual, value)
		}
	}
	return manual
}

// storeLearnedEpicAxisOptions folds options deduced at write time into the
// stored map of an axis field, filling holes only. It merges against the row
// as it is now, so a map edited meanwhile keeps its edit, and does nothing
// when the axis is no longer mapped to that field.
func (d *DB) storeLearnedEpicAxisOptions(projectID, axis, fieldID string, deduced map[string]string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	current, err := d.getProjectByIDUnsafe(projectID)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("project not found")
	}
	field := current.EpicAxisFields.Field(axis)
	if field == nil || field.ID != fieldID {
		return nil
	}
	filled, added := models.FillEpicAxisOptions(*field, deduced)
	if !added {
		return nil
	}
	fields := current.EpicAxisFields
	if axis == models.EpicAxisPriority {
		fields.Priority = &filled
	} else {
		fields.Quarter = &filled
	}
	raw, _ := json.Marshal(fields)
	_, err = d.conn.Exec("UPDATE projects SET epic_axis_fields = ?, updated_at = ? WHERE id = ?", string(raw), time.Now(), current.ID)
	return err
}

// EpicAxisFieldDiscovery is what the settings show to map an axis to a field:
// the candidate fields of one epic's edit screen with the option maps the
// deductions give for each axis.
type EpicAxisFieldDiscovery struct {
	EpicKey    string                   `json:"epicKey,omitempty"`
	NoEpic     bool                     `json:"noEpic,omitempty"`
	Candidates []EpicAxisFieldCandidate `json:"candidates"`
}

// EpicAxisFieldCandidate is one candidate field and its deduced maps, keyed
// by axis.
type EpicAxisFieldCandidate struct {
	models.EpicFieldCandidate
	Deduced map[string]map[string]string `json:"deduced"`
}

// epicAxisFieldsForUnsafe are the axis fields an epic read on trk for proj
// asks (#680). The fields stay on the project (#741) and were discovered on
// its default tracker, so the epics of another of its trackers, perhaps on
// another site, are read without them, as before #680.
func (d *DB) epicAxisFieldsForUnsafe(proj *models.Project, trk *models.Tracker) models.EpicAxisFields {
	if proj == nil {
		return models.EpicAxisFields{}
	}
	if trk != nil {
		if def := d.trackerOfProjectUnsafe(proj); def == nil || def.ID != trk.ID {
			return models.EpicAxisFields{}
		}
	}
	return proj.EpicAxisFields
}

// EpicAxisFieldCandidates reads the closed-list custom fields of one epic of
// the project. Field ids are site-wide, but which fields an epic carries is
// a property of its project's screens, so the epic is one of the project's
// own, open ones first. A project with no epic yet answers NoEpic without a
// tracker request.
func (d *DB) EpicAxisFieldCandidates(ctx context.Context, projectID string) (EpicAxisFieldDiscovery, error) {
	out := EpicAxisFieldDiscovery{Candidates: []EpicAxisFieldCandidate{}}
	proj, err := d.GetProjectByID(strings.TrimSpace(projectID))
	if err != nil || proj == nil {
		return out, fmt.Errorf("project not found")
	}
	if proj.IssueTracker != "jira" {
		return out, fmt.Errorf("axis fields can only be mapped on a Jira project")
	}
	epicKey, err := d.discoveryEpic(proj)
	if err != nil {
		return out, err
	}
	if epicKey == "" {
		out.NoEpic = true
		return out, nil
	}
	// The fields stay on the project while its epics come from its default
	// tracker (#741): the candidates are read there.
	trk := d.trackerOfProjectUnsafe(proj)
	ts, err := d.TrackerFor(trk)
	if err != nil {
		return out, err
	}
	manager, ok := ts.(tracker.EpicAxisFieldManager)
	if !ok {
		return out, fmt.Errorf("the %s tracker has no epic fields", ts.Name())
	}
	ctx, cancel := context.WithTimeout(ctx, epicAxisFieldsTimeout)
	defer cancel()
	candidates, err := manager.EpicAxisFieldCandidates(tracker.WithTracker(ctx, trk), trk, epicKey)
	if err != nil {
		return out, err
	}
	out.EpicKey = epicKey
	for _, c := range candidates {
		out.Candidates = append(out.Candidates, EpicAxisFieldCandidate{
			EpicFieldCandidate: c,
			Deduced: map[string]map[string]string{
				models.EpicAxisPriority: models.DeduceEpicAxisOptions(models.EpicAxisPriority, c),
				models.EpicAxisQuarter:  models.DeduceEpicAxisOptions(models.EpicAxisQuarter, c),
			},
		})
	}
	return out, nil
}

// discoveryEpic picks the epic whose edit screen lists the candidates: one of
// the project's own, an open one when there is any. "" when there is none.
func (d *DB) discoveryEpic(proj *models.Project) (string, error) {
	metas, err := d.GetProjectMacros(proj.ID)
	if err != nil {
		return "", err
	}
	closed := ""
	for _, meta := range metas {
		if !macroKeyLabelable(meta.Key, proj) {
			continue
		}
		if !meta.Closed {
			return meta.Key, nil
		}
		if closed == "" {
			closed = meta.Key
		}
	}
	return closed, nil
}

// pushEpicAxisField writes an axis value on the epic's mapped field, once its
// label was written. It answers a sentence for the activity, "" when the
// project maps no field to the axis or its tracker has none, in which case no
// request is made.
//
// A value with no option, or a field the epic's edit screen does not carry,
// leaves the field as it is and is said in the sentence: the label went
// through, which is what the axis promised before fields existed. A quarter
// missing from the map is first looked up among the field's options on that
// epic, and what the deduction finds there is stored on the project.
func (d *DB) pushEpicAxisField(ctx context.Context, projectID string, macroKey string, axis string, mAxis macroAxis, value string) (string, error) {
	proj, err := d.GetProjectByID(strings.TrimSpace(projectID))
	if err != nil || proj == nil || proj.IssueTracker != "jira" {
		return "", nil
	}
	field := proj.EpicAxisFields.Field(axis)
	if field == nil {
		return "", nil
	}
	ts, proj, err := d.macroTracker(projectID, macroKey, mAxis)
	if err != nil {
		return "", err
	}
	trk := d.trackerOfProjectUnsafe(proj)
	manager, ok := ts.(tracker.EpicAxisFieldManager)
	if !ok {
		return "", nil
	}
	key := strings.TrimSpace(macroKey)
	name := field.Name
	if name == "" {
		name = field.ID
	}
	path := field.OptionFor(value)
	// A line a person cleared by hand stays cleared: no deduction refills it,
	// so the screen is not read for it.
	if value != "" && path == "" && axis == models.EpicAxisQuarter && !field.IsManual(value) {
		learned, present, err := d.learnEpicAxisOption(ctx, manager, proj, trk, key, axis, *field, value)
		if err != nil {
			return "", fmt.Errorf("label written but field %q not read on %s: %w", name, key, err)
		}
		if !present {
			return fmt.Sprintf("field %q is not on the edit screen of %s: left as it is", name, key), nil
		}
		path = learned
	}
	if value != "" && path == "" {
		return fmt.Sprintf("%s has no option in field %q: field left as it is", value, name), nil
	}
	writeCtx, cancel := context.WithTimeout(tracker.WithTracker(ctx, trk), macroWriteTimeout)
	defer cancel()
	if err := manager.SetEpicAxisField(writeCtx, trk, key, *field, path); err != nil {
		return "", fmt.Errorf("label written but field %q not: %w", name, err)
	}
	if path == "" {
		return fmt.Sprintf("field %q cleared on %s", name, key), nil
	}
	return fmt.Sprintf("field %q set on %s", name, key), nil
}

// learnEpicAxisOption looks a value missing from a field's map up among the
// options the epic's edit screen offers, and stores what the deduction finds
// there, filling holes only. It answers the option found, "" for none, and
// whether the epic's screen carries the field at all.
func (d *DB) learnEpicAxisOption(ctx context.Context, manager tracker.EpicAxisFieldManager, proj *models.Project, trk *models.Tracker, key, axis string, field models.EpicAxisField, value string) (string, bool, error) {
	readCtx, cancel := context.WithTimeout(tracker.WithTracker(ctx, trk), epicAxisFieldsTimeout)
	candidates, err := manager.EpicAxisFieldCandidates(readCtx, trk, key)
	cancel()
	if err != nil {
		return "", false, err
	}
	for _, c := range candidates {
		if c.ID != field.ID {
			continue
		}
		deduced := models.DeduceEpicAxisOptions(axis, c)
		if err := d.storeLearnedEpicAxisOptions(proj.ID, axis, field.ID, deduced); err != nil {
			return "", true, err
		}
		return deduced[value], true, nil
	}
	return "", false, nil
}

// epicAxisFieldValue is the axis value an epic's mapped field carries, "" when
// the axis maps no field, the field is empty or its option has no line in the
// map: the label then decides.
func epicAxisFieldValue(field *models.EpicAxisField, epic models.Task) string {
	if field == nil {
		return ""
	}
	return field.ValueOf(epic.AxisFieldValues[field.ID])
}

// epicAxisFieldLate tells whether an epic's mapped field disagrees with the
// value decided in Sectile. A value with no option cannot be written, so its
// field alone never makes the epic late.
func epicAxisFieldLate(field *models.EpicAxisField, epic models.Task, value string) bool {
	if field == nil || value == "" {
		return false
	}
	option := field.OptionFor(value)
	return option != "" && epic.AxisFieldValues[field.ID] != option
}
