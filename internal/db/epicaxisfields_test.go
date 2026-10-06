package db

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"tasks/internal/models"
	"tasks/internal/testsqlite"
	"tasks/internal/tracker"
)

// Every field and option here is synthetic: no test names a field or an
// option of a real site (#680).

// fieldTracker is the horizon fake with custom fields on its epics: the
// candidates of every epic's edit screen, and the field writes it received.
type fieldTracker struct {
	*horizonTracker
	candidates      []models.EpicFieldCandidate
	candidateReads  int
	fieldWrites     []fieldWrite
	failFieldWrites bool
}

type fieldWrite struct {
	key, field, path string
}

func (f *fieldTracker) EpicAxisFieldCandidates(ctx context.Context, project *models.Project, epicKey string) ([]models.EpicFieldCandidate, error) {
	f.candidateReads++
	return f.candidates, nil
}

func (f *fieldTracker) SetEpicAxisField(ctx context.Context, project *models.Project, epicKey string, field models.EpicAxisField, optionPath string) error {
	f.fieldWrites = append(f.fieldWrites, fieldWrite{epicKey, field.ID, optionPath})
	if f.failFieldWrites {
		return errors.New("option refused")
	}
	return nil
}

var _ tracker.EpicAxisFieldManager = (*fieldTracker)(nil)

func priorityCandidate() models.EpicFieldCandidate {
	return models.EpicFieldCandidate{ID: "cf-epic-priority", Name: "Epic rank", Kind: models.EpicFieldSelect, Options: []models.EpicFieldOption{
		{ID: "o0", Value: "P0"}, {ID: "o1", Value: "P1"}, {ID: "o9", Value: "Someday"},
	}}
}

func quarterCandidate() models.EpicFieldCandidate {
	return models.EpicFieldCandidate{ID: "cf-epic-quarter", Name: "Epic period", Kind: models.EpicFieldCascade, Options: []models.EpicFieldOption{
		{ID: "y26", Value: "2026", Children: []models.EpicFieldOption{{ID: "q3", Value: "Q3"}, {ID: "q4", Value: "Q4"}}},
	}}
}

// fieldProject opens a Jira project whose priority and quarter are mapped to
// the synthetic fields above, the priority map holding p0 and p1 and the
// quarter map holding 2026-Q3 only.
func fieldProject(t *testing.T, epics []models.Task) (*DB, *models.Project, *fieldTracker) {
	t.Helper()
	fake := &fieldTracker{horizonTracker: newHorizonTracker(epics), candidates: []models.EpicFieldCandidate{priorityCandidate(), quarterCandidate()}}
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "test.db"), NewDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	proj, err := database.CreateProject(models.CreateProjectRequest{Name: "Platform", Slug: "platform", IssueTracker: "jira", JiraProject: "PE"})
	if err != nil {
		t.Fatal(err)
	}
	database.TrackerRegistry().Register("jira", fake)
	fields := models.EpicAxisFields{
		Priority: &models.EpicAxisField{ID: "cf-epic-priority", Name: "Epic rank", Kind: models.EpicFieldSelect, Options: map[string]string{"p0": "o0", "p1": "o1"}},
		Quarter:  &models.EpicAxisField{ID: "cf-epic-quarter", Name: "Epic period", Kind: models.EpicFieldCascade, Options: map[string]string{"2026-Q3": "y26/q3"}},
	}
	proj, err = database.UpdateProject(proj.ID, models.UpdateProjectRequest{EpicAxisFields: &fields})
	if err != nil {
		t.Fatal(err)
	}
	return database, proj, fake
}

func TestEditEpicAxisFieldsValidates(t *testing.T) {
	for _, c := range []struct {
		name  string
		field models.EpicAxisField
	}{
		{"no id", models.EpicAxisField{Kind: models.EpicFieldSelect}},
		{"unknown kind", models.EpicAxisField{ID: "cf-x", Kind: "text"}},
		{"value outside the axis", models.EpicAxisField{ID: "cf-x", Kind: models.EpicFieldSelect, Options: map[string]string{"p7": "o"}}},
		{"cascade path without a child", models.EpicAxisField{ID: "cf-x", Kind: models.EpicFieldCascade, Options: map[string]string{"p1": "parent"}}},
	} {
		_, err := editEpicAxisFields(models.EpicAxisFields{}, models.EpicAxisFields{Priority: &c.field})
		if !errors.Is(err, ErrInvalidEpicAxisFields) {
			t.Errorf("%s: err = %v, want a refusal", c.name, err)
		}
	}
	got, err := editEpicAxisFields(models.EpicAxisFields{}, models.EpicAxisFields{Quarter: &models.EpicAxisField{
		ID: " cf-q ", Kind: models.EpicFieldSelect, Options: map[string]string{"2026 q4": " a ", "2027-Q1": ""},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"2026-Q4": "a"}; !reflect.DeepEqual(got.Quarter.Options, want) || got.Quarter.ID != "cf-q" {
		t.Fatalf("cleaned = %+v", got.Quarter)
	}
}

func TestEditEpicAxisFieldsMarksHandEditsOfTheStoredField(t *testing.T) {
	stored := models.EpicAxisFields{Priority: &models.EpicAxisField{
		ID: "cf-p", Kind: models.EpicFieldSelect, Options: map[string]string{"p0": "a", "p1": "b", "p2": "c"},
	}}
	sent := models.EpicAxisFields{Priority: &models.EpicAxisField{
		ID: "cf-p", Kind: models.EpicFieldSelect, Options: map[string]string{"p0": "a", "p1": "z", "p3": "d"},
	}}
	got, err := editEpicAxisFields(stored, sent)
	if err != nil {
		t.Fatal(err)
	}
	// p1 changed, p2 cleared, p3 added; p0 untouched.
	if want := []string{"p1", "p2", "p3"}; !slices.Equal(got.Priority.Manual, want) {
		t.Fatalf("manual = %v, want %v", got.Priority.Manual, want)
	}

	// Another field replaces the entry: nothing of the former map is hand-set.
	other, err := editEpicAxisFields(got, models.EpicAxisFields{Priority: &models.EpicAxisField{ID: "cf-other", Kind: models.EpicFieldSelect}})
	if err != nil {
		t.Fatal(err)
	}
	if len(other.Priority.Manual) != 0 || len(other.Priority.Options) != 0 {
		t.Fatalf("switched field = %+v", other.Priority)
	}

	// No field removes the mapping.
	removed, err := editEpicAxisFields(got, models.EpicAxisFields{})
	if err != nil || removed.Priority != nil {
		t.Fatalf("removed = %+v, %v", removed, err)
	}
}

func TestEpicAxisFieldsAreStoredOnTheProject(t *testing.T) {
	database, proj, _ := fieldProject(t, nil)
	stored, err := database.GetProjectByID(proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.EpicAxisFields.Priority == nil || stored.EpicAxisFields.Priority.Options["p1"] != "o1" {
		t.Fatalf("stored = %+v", stored.EpicAxisFields)
	}
	if _, err := database.UpdateProject(proj.ID, models.UpdateProjectRequest{EpicAxisFields: &models.EpicAxisFields{
		Priority: &models.EpicAxisField{ID: "", Kind: models.EpicFieldSelect},
	}}); !errors.Is(err, ErrInvalidEpicAxisFields) {
		t.Fatalf("an empty field id was accepted: %v", err)
	}
}

func TestEpicAxisFieldCandidatesNeedAnEpic(t *testing.T) {
	database, proj, fake := fieldProject(t, nil)
	got, err := database.EpicAxisFieldCandidates(t.Context(), proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.NoEpic || len(got.Candidates) != 0 || fake.candidateReads != 0 {
		t.Fatalf("no epic: %+v, %d reads", got, fake.candidateReads)
	}

	// A milestone is no epic; an epic of the project is.
	now := "now"
	mustSaveMacro(t, database, proj.ID, "M-1", &now)
	mustSaveMacro(t, database, proj.ID, "PE-4", &now)
	got, err = database.EpicAxisFieldCandidates(t.Context(), proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.NoEpic || got.EpicKey != "PE-4" || len(got.Candidates) != 2 {
		t.Fatalf("discovery = %+v", got)
	}
	if want := map[string]string{"p0": "o0", "p1": "o1"}; !reflect.DeepEqual(got.Candidates[0].Deduced[models.EpicAxisPriority], want) {
		t.Errorf("priority deduced = %v", got.Candidates[0].Deduced)
	}
	if want := map[string]string{"2026-Q3": "y26/q3", "2026-Q4": "y26/q4"}; !reflect.DeepEqual(got.Candidates[1].Deduced[models.EpicAxisQuarter], want) {
		t.Errorf("quarter deduced = %v", got.Candidates[1].Deduced)
	}
}

func TestPushWritesTheLabelThenTheField(t *testing.T) {
	database, proj, fake := fieldProject(t, nil)
	out, err := database.PushMacroPriorityLabel(t.Context(), proj.ID, "PE-1", "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.writes) != 1 || !slices.Equal(fake.writes[0].Labels, []string{"priority:p1"}) {
		t.Fatalf("label writes = %+v", fake.writes)
	}
	if want := []fieldWrite{{"PE-1", "cf-epic-priority", "o1"}}; !reflect.DeepEqual(fake.fieldWrites, want) {
		t.Fatalf("field writes = %+v", fake.fieldWrites)
	}
	if !strings.Contains(out, `field "Epic rank" set on PE-1`) {
		t.Errorf("output = %q", out)
	}

	// Cleared: the labels go, and the field is emptied.
	if _, err := database.PushMacroPriorityLabel(t.Context(), proj.ID, "PE-1", ""); err != nil {
		t.Fatal(err)
	}
	if last := fake.fieldWrites[len(fake.fieldWrites)-1]; last.path != "" {
		t.Errorf("clear wrote %+v", last)
	}
}

func TestPushNamesAValueWithNoOption(t *testing.T) {
	database, proj, fake := fieldProject(t, nil)
	out, err := database.PushMacroPriorityLabel(t.Context(), proj.ID, "PE-1", "p3")
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.writes) != 1 || len(fake.fieldWrites) != 0 {
		t.Fatalf("label writes %d, field writes %+v", len(fake.writes), fake.fieldWrites)
	}
	if !strings.Contains(out, `p3 has no option in field "Epic rank"`) {
		t.Errorf("output = %q", out)
	}
}

func TestPushLearnsAMissingQuarterAndStoresIt(t *testing.T) {
	database, proj, fake := fieldProject(t, nil)
	if _, err := database.PushMacroQuarterLabel(t.Context(), proj.ID, "PE-1", "2026-Q4"); err != nil {
		t.Fatal(err)
	}
	if want := []fieldWrite{{"PE-1", "cf-epic-quarter", "y26/q4"}}; !reflect.DeepEqual(fake.fieldWrites, want) {
		t.Fatalf("field writes = %+v", fake.fieldWrites)
	}
	stored, _ := database.GetProjectByID(proj.ID)
	if got := stored.EpicAxisFields.Quarter.Options; got["2026-Q4"] != "y26/q4" || got["2026-Q3"] != "y26/q3" {
		t.Fatalf("stored map = %v", got)
	}

	// Known now: the next write reads no screen.
	reads := fake.candidateReads
	if _, err := database.PushMacroQuarterLabel(t.Context(), proj.ID, "PE-2", "2026-Q4"); err != nil {
		t.Fatal(err)
	}
	if fake.candidateReads != reads {
		t.Errorf("a known quarter read the edit screen again")
	}
}

func TestPushLeavesAQuarterClearedByHand(t *testing.T) {
	database, proj, fake := fieldProject(t, nil)
	fields := proj.EpicAxisFields
	quarter := *fields.Quarter
	quarter.Options = map[string]string{}
	fields.Quarter = &quarter
	if _, err := database.UpdateProject(proj.ID, models.UpdateProjectRequest{EpicAxisFields: &fields}); err != nil {
		t.Fatal(err)
	}
	out, err := database.PushMacroQuarterLabel(t.Context(), proj.ID, "PE-1", "2026-Q3")
	if err != nil {
		t.Fatal(err)
	}
	if fake.candidateReads != 0 || len(fake.fieldWrites) != 0 || !strings.Contains(out, "2026-Q3 has no option") {
		t.Fatalf("a cleared line was looked up or written: %d reads, %+v, %q", fake.candidateReads, fake.fieldWrites, out)
	}
}

func TestPushSaysTheFieldIsAbsentFromTheEpic(t *testing.T) {
	database, proj, fake := fieldProject(t, nil)
	fake.candidates = []models.EpicFieldCandidate{priorityCandidate()}
	out, err := database.PushMacroQuarterLabel(t.Context(), proj.ID, "PE-1", "2027-Q1")
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.fieldWrites) != 0 || !strings.Contains(out, `field "Epic period" is not on the edit screen of PE-1`) {
		t.Fatalf("output = %q, field writes %+v", out, fake.fieldWrites)
	}
}

func TestPushFailsWhenTheFieldWriteFailsAfterTheLabel(t *testing.T) {
	database, proj, fake := fieldProject(t, nil)
	fake.failFieldWrites = true
	_, err := database.PushMacroPriorityLabel(t.Context(), proj.ID, "PE-1", "p0")
	if err == nil || !strings.Contains(err.Error(), `label written but field "Epic rank" not`) {
		t.Fatalf("err = %v", err)
	}
	if len(fake.writes) != 1 {
		t.Errorf("the label should have been written first: %+v", fake.writes)
	}
}

func TestPushWithoutAFieldMakesNoFieldRequest(t *testing.T) {
	fake := &fieldTracker{horizonTracker: newHorizonTracker(nil)}
	database, proj := jiraProjectWithTracker(t, fake.horizonTracker)
	database.TrackerRegistry().Register("jira", fake)
	if _, err := database.PushMacroQuarterLabel(t.Context(), proj.ID, "PE-1", "2026-Q4"); err != nil {
		t.Fatal(err)
	}
	if fake.candidateReads != 0 || len(fake.fieldWrites) != 0 {
		t.Fatalf("a project with no field touched fields: %d reads, %+v", fake.candidateReads, fake.fieldWrites)
	}
}

func TestImportReadsTheFieldBeforeTheLabel(t *testing.T) {
	database, proj, _ := fieldProject(t, []models.Task{
		{Key: "PE-1", Labels: []string{"priority:p1"}, AxisFieldValues: map[string]string{"cf-epic-priority": "o0", "cf-epic-quarter": "y26/q3"}},
		{Key: "PE-2", Labels: []string{"priority:p1", "2026-Q4"}, AxisFieldValues: map[string]string{"cf-epic-priority": "o9"}},
	})
	if _, err := database.ImportMacroHorizons(t.Context(), proj.ID); err != nil {
		t.Fatal(err)
	}
	got := map[string]models.MacroMeta{}
	macros, _ := database.GetProjectMacros(proj.ID)
	for _, m := range macros {
		got[m.Key] = m
	}
	if got["PE-1"].Priority != "p0" || got["PE-1"].Quarter != "2026-Q3" {
		t.Errorf("PE-1 = %+v, the field should win", got["PE-1"])
	}
	// An option no line maps is ignored: the label decides.
	if got["PE-2"].Priority != "p1" || got["PE-2"].Quarter != "2026-Q4" {
		t.Errorf("PE-2 = %+v, the labels should decide", got["PE-2"])
	}
}

func TestPendingPushesIncludeAFieldThatDisagrees(t *testing.T) {
	database, proj, fake := fieldProject(t, []models.Task{
		{Key: "PE-1", Labels: []string{"priority:p1"}, AxisFieldValues: map[string]string{"cf-epic-priority": "o1"}},
		{Key: "PE-2", Labels: []string{"priority:p1"}, AxisFieldValues: map[string]string{"cf-epic-priority": "o0"}},
		{Key: "PE-3", Labels: []string{"priority:p1"}},
		{Key: "PE-4", Labels: []string{"quarter:2027-q2"}},
	})
	p1, q := "p1", "2027-Q2"
	for _, key := range []string{"PE-1", "PE-2", "PE-3"} {
		if _, err := database.SaveMacroAxes(proj.ID, key, &p1, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := database.SaveMacroAxes(proj.ID, "PE-4", nil, &q, nil); err != nil {
		t.Fatal(err)
	}
	pending, err := database.PendingHorizonPushes(t.Context(), proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{}
	for _, m := range pending {
		keys = append(keys, m.Key)
	}
	slices.Sort(keys)
	// PE-1 agrees; PE-2 and PE-3 disagree on the field; PE-4's quarter has no
	// option, so its empty field cannot make it late.
	if !slices.Equal(keys, []string{"PE-2", "PE-3"}) {
		t.Fatalf("pending = %v", keys)
	}
	if _, failures, err := database.PushPendingHorizons(t.Context(), proj.ID); err != nil || len(failures) != 0 {
		t.Fatalf("push: %v, %v", failures, err)
	}
	written := map[string]string{}
	for _, w := range fake.fieldWrites {
		written[w.key] = w.path
	}
	if want := map[string]string{"PE-2": "o1", "PE-3": "o1"}; !reflect.DeepEqual(written, want) {
		t.Fatalf("field writes = %v", written)
	}
}
