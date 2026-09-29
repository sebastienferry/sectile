package db

import (
	"context"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"tasks/internal/models"
	"tasks/internal/testsqlite"
	"tasks/internal/tracker"
)

// GetIssue serves the fake's epics by key, so the quarter push can read the
// labels it has to strip.
func (f *horizonTracker) GetIssue(ctx context.Context, req tracker.GetIssueRequest) (*models.Task, error) {
	for i := range f.epics {
		if f.epics[i].Key == req.Key {
			return &f.epics[i], nil
		}
	}
	return &models.Task{Key: req.Key}, nil
}

func TestNormalizeEpicPriority(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", ""}, {"  ", ""}, {"p0", "p0"}, {"P3", "p3"}, {" p1 ", "p1"}, {"priority:p2", "p2"}, {"PRIORITY:P2", "p2"},
	} {
		got, err := NormalizeEpicPriority(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("NormalizeEpicPriority(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
	for _, in := range []string{"P4", "p", "high", "P01", "p1p"} {
		if _, err := NormalizeEpicPriority(in); err == nil {
			t.Errorf("NormalizeEpicPriority(%q) should be refused", in)
		}
	}
}

func TestNormalizeQuarter(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", ""}, {"2026-Q4", "2026-Q4"}, {"2026.Q4", "2026-Q4"}, {"2026 Q4", "2026-Q4"}, {"2026-q1", "2026-Q1"}, {" 1999.q3 ", "1999-Q3"},
	} {
		got, err := NormalizeQuarter(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("NormalizeQuarter(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
	for _, in := range []string{"Q4", "2026-Q5", "2026-Q0", "26-Q4", "2026Q4", "bientôt", "quarter:2026-q4"} {
		if _, err := NormalizeQuarter(in); err == nil {
			t.Errorf("NormalizeQuarter(%q) should be refused", in)
		}
	}
}

func TestPriorityFromLabels(t *testing.T) {
	for _, tc := range []struct {
		labels []string
		want   string
	}{
		{nil, ""},
		{[]string{"roadmap:now", "priority:p1"}, "p1"},
		{[]string{"Priority:P1"}, "p1"},
		{[]string{"#priority:p0"}, "p0"},
		// An unreadable value is ignored, the next valid one is read.
		{[]string{"priority:p7", "priority:p2"}, "p2"},
		{[]string{"p1"}, ""},
	} {
		if got := PriorityFromLabels(tc.labels); got != tc.want {
			t.Errorf("PriorityFromLabels(%v) = %q, want %q", tc.labels, got, tc.want)
		}
	}
}

func TestQuarterFromLabels(t *testing.T) {
	for _, tc := range []struct {
		labels []string
		want   string
	}{
		{nil, ""},
		{[]string{"quarter:2026-q3"}, "2026-Q3"},
		{[]string{"2026-Q3"}, "2026-Q3"},
		// Both forms of one quarter count once.
		{[]string{"2026-Q3", "quarter:2026-q3"}, "2026-Q3"},
		// The prefixed form wins a disagreement, whatever the order.
		{[]string{"2026-Q3", "quarter:2026-q4"}, "2026-Q4"},
		{[]string{"quarter:soon"}, ""},
		{[]string{"quarter:soon", "2026-Q2"}, "2026-Q2"},
		{[]string{"release-2026-Q3"}, ""},
	} {
		if got := QuarterFromLabels(tc.labels); got != tc.want {
			t.Errorf("QuarterFromLabels(%v) = %q, want %q", tc.labels, got, tc.want)
		}
	}
}

func TestIsQuarterLabel(t *testing.T) {
	for label, want := range map[string]bool{
		"2026-Q3": true, "2026.q3": true, "quarter:2026-q4": true, "quarter:soon": true,
		"roadmap:now": false, "priority:p1": false, "release-2026-Q3": false, "2026-Q5": false,
	} {
		if got := isQuarterLabel(label); got != want {
			t.Errorf("isQuarterLabel(%q) = %v, want %v", label, got, want)
		}
	}
}

func TestSaveMacroAxesStoresAndClearsWithoutTouchingTheRest(t *testing.T) {
	database, proj := jiraProjectWithTracker(t, newHorizonTracker(nil))
	horizon := "next"
	description := "Le cadrage"
	if _, err := database.SaveMacroMeta(proj.ID, "PE-1", &horizon, &description, nil, nil); err != nil {
		t.Fatal(err)
	}

	p1, q := "P1", "2026.q4"
	saved, err := database.SaveMacroAxes(proj.ID, "PE-1", &p1, &q)
	if err != nil {
		t.Fatalf("axes not saved: %v", err)
	}
	if saved.Priority != "p1" || saved.Quarter != "2026-Q4" {
		t.Errorf("saved = %q / %q, want p1 / 2026-Q4", saved.Priority, saved.Quarter)
	}
	macros, err := database.GetProjectMacros(proj.ID)
	if err != nil || len(macros) != 1 {
		t.Fatalf("macros = %v, %v", macros, err)
	}
	m := macros[0]
	if m.Priority != "p1" || m.Quarter != "2026-Q4" || m.Horizon != "next" || m.Description != "Le cadrage" {
		t.Errorf("read back = %+v", m)
	}

	// A nil pointer leaves the axis alone, an empty value clears it.
	empty := ""
	if _, err := database.SaveMacroAxes(proj.ID, "PE-1", &empty, nil); err != nil {
		t.Fatal(err)
	}
	macros, _ = database.GetProjectMacros(proj.ID)
	if macros[0].Priority != "" || macros[0].Quarter != "2026-Q4" {
		t.Errorf("after clearing the priority = %q / %q", macros[0].Priority, macros[0].Quarter)
	}

	// Other edits leave the axes alone.
	later := "later"
	if _, err := database.SaveMacroMeta(proj.ID, "PE-1", &later, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	macros, _ = database.GetProjectMacros(proj.ID)
	if macros[0].Quarter != "2026-Q4" {
		t.Errorf("a horizon edit cleared the quarter: %+v", macros[0])
	}
}

func TestSaveMacroAxesRefusesAnUnreadableValue(t *testing.T) {
	database, proj := jiraProjectWithTracker(t, newHorizonTracker(nil))
	p2, bad := "p2", "2026-Q5"
	if _, err := database.SaveMacroAxes(proj.ID, "PE-1", &p2, &bad); err == nil {
		t.Fatal("an invalid quarter should be refused")
	}
	macros, _ := database.GetProjectMacros(proj.ID)
	for _, m := range macros {
		if m.Key == "PE-1" && m.Priority != "" {
			t.Errorf("the priority was saved alongside a refused quarter: %+v", m)
		}
	}
}

func TestMacrosAreLabelsWritableOnlyForTheProjectsOwnJiraEpics(t *testing.T) {
	database, proj := jiraProjectWithTracker(t, newHorizonTracker(nil))
	for _, key := range []string{"PE-1", "M-3", "OTHER-9"} {
		if _, err := database.SaveMacroAxes(proj.ID, key, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	macros, err := database.GetProjectMacros(proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, m := range macros {
		got[m.Key] = m.LabelsWritable
	}
	want := map[string]bool{"PE-1": true, "M-3": false, "OTHER-9": false}
	for key, w := range want {
		if got[key] != w {
			t.Errorf("%s writable = %v, want %v", key, got[key], w)
		}
		if database.MacroLabelsWritable(proj.ID, key) != w {
			t.Errorf("MacroLabelsWritable(%s) disagrees with the list", key)
		}
	}
}

func TestMacrosOfATrackerWithoutEpicsAreNotLabelsWritable(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "test.db"), NewDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	proj, err := database.CreateProject(models.CreateProjectRequest{Name: "Local", IssueTracker: "local"})
	if err != nil {
		t.Fatal(err)
	}
	p0 := "p0"
	if _, err := database.SaveMacroAxes(proj.ID, "EPIC-1", &p0, nil); err != nil {
		t.Fatal(err)
	}
	macros, _ := database.GetProjectMacros(proj.ID)
	if len(macros) != 1 || macros[0].LabelsWritable || macros[0].Priority != "p0" {
		t.Errorf("a local project keeps the value and writes nothing: %+v", macros)
	}
}

func sortedCopy(labels []string) []string {
	out := slices.Clone(labels)
	sort.Strings(out)
	return out
}

func TestPushMacroPriorityLabelWritesTheAxisExclusively(t *testing.T) {
	fake := newHorizonTracker(nil)
	database, proj := jiraProjectWithTracker(t, fake)

	if _, err := database.PushMacroPriorityLabel(t.Context(), proj.ID, "PE-1", "p2"); err != nil {
		t.Fatalf("push refused: %v", err)
	}
	if len(fake.writes) != 1 {
		t.Fatalf("want one write, got %d", len(fake.writes))
	}
	w := fake.writes[0]
	if !slices.Equal(w.Labels, []string{"priority:p2"}) {
		t.Errorf("added = %v", w.Labels)
	}
	if !slices.Equal(sortedCopy(w.RemovedLabels), []string{"priority:p0", "priority:p1", "priority:p3"}) {
		t.Errorf("removed = %v", w.RemovedLabels)
	}
	if w.Title != nil || w.Description != nil || w.Status != nil || w.Assignee != nil {
		t.Error("the write should carry labels and nothing else")
	}

	if _, err := database.PushMacroPriorityLabel(t.Context(), proj.ID, "PE-1", ""); err != nil {
		t.Fatal(err)
	}
	cleared := fake.writes[1]
	if len(cleared.Labels) != 0 || len(cleared.RemovedLabels) != 4 {
		t.Errorf("clearing should remove the four labels and add none: %+v", cleared)
	}
}

func TestPushMacroQuarterLabelStripsBothFormsOfTheOldQuarter(t *testing.T) {
	fake := newHorizonTracker([]models.Task{{Key: "PE-1", Labels: []string{"2026-Q3", "quarter:2026-q2", "roadmap:now", "team-a"}}})
	database, proj := jiraProjectWithTracker(t, fake)

	if _, err := database.PushMacroQuarterLabel(t.Context(), proj.ID, "PE-1", "2026-Q4"); err != nil {
		t.Fatalf("push refused: %v", err)
	}
	if len(fake.writes) != 1 {
		t.Fatalf("want one write, got %d", len(fake.writes))
	}
	w := fake.writes[0]
	if !slices.Equal(w.Labels, []string{"quarter:2026-q4"}) {
		t.Errorf("added = %v", w.Labels)
	}
	if !slices.Equal(sortedCopy(w.RemovedLabels), []string{"2026-Q3", "quarter:2026-q2"}) {
		t.Errorf("removed = %v, want both quarter forms and nothing else", w.RemovedLabels)
	}
}

func TestPushMacroQuarterLabelKeepsTheTargetAndClears(t *testing.T) {
	fake := newHorizonTracker([]models.Task{{Key: "PE-1", Labels: []string{"quarter:2026-q4", "2026-Q4"}}})
	database, proj := jiraProjectWithTracker(t, fake)

	if _, err := database.PushMacroQuarterLabel(t.Context(), proj.ID, "PE-1", "2026-Q4"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(fake.writes[0].RemovedLabels, []string{"2026-Q4"}) {
		t.Errorf("removed = %v, want only the bare duplicate", fake.writes[0].RemovedLabels)
	}

	if _, err := database.PushMacroQuarterLabel(t.Context(), proj.ID, "PE-1", ""); err != nil {
		t.Fatal(err)
	}
	cleared := fake.writes[1]
	if len(cleared.Labels) != 0 || !slices.Equal(sortedCopy(cleared.RemovedLabels), []string{"2026-Q4", "quarter:2026-q4"}) {
		t.Errorf("clearing = %+v", cleared)
	}
}

func TestPushMacroAxisFailureSaysTheValueStaysInSectile(t *testing.T) {
	fake := newHorizonTracker(nil)
	fake.failKey = "PE-1"
	database, proj := jiraProjectWithTracker(t, fake)

	_, err := database.PushMacroPriorityLabel(t.Context(), proj.ID, "PE-1", "p1")
	if err == nil || !strings.Contains(err.Error(), "posée dans Sectile mais pas sur PE-1") {
		t.Errorf("err = %v", err)
	}
	_, err = database.PushMacroQuarterLabel(t.Context(), proj.ID, "M-2", "2026-Q1")
	if err == nil || !strings.Contains(err.Error(), "posé dans Sectile mais pas sur M-2") {
		t.Errorf("a milestone refusal should say so: %v", err)
	}
}

func TestImportMacroHorizonsReadsThePriorityAndTheQuarter(t *testing.T) {
	fake := newHorizonTracker([]models.Task{
		{Key: "PE-1", Title: "Tracker wins", Labels: []string{"priority:p1", "2026-Q3"}},
		{Key: "PE-2", Title: "No label", Labels: []string{"roadmap:now"}},
		{Key: "PE-3", Title: "Both forms", Labels: []string{"2026-Q3", "quarter:2026-q4"}},
	})
	database, proj := jiraProjectWithTracker(t, fake)
	p3, q := "p3", "2027-Q1"
	for _, key := range []string{"PE-1", "PE-2"} {
		if _, err := database.SaveMacroAxes(proj.ID, key, &p3, &q); err != nil {
			t.Fatal(err)
		}
	}

	note, err := database.ImportMacroHorizons(t.Context(), proj.ID)
	if err != nil {
		t.Fatalf("import refused: %v", err)
	}
	if !strings.Contains(note, "1 priorisée(s)") || !strings.Contains(note, "2 datée(s)") {
		t.Errorf("summary = %q", note)
	}
	got := map[string]models.MacroMeta{}
	macros, _ := database.GetProjectMacros(proj.ID)
	for _, m := range macros {
		got[m.Key] = m
	}
	if got["PE-1"].Priority != "p1" || got["PE-1"].Quarter != "2026-Q3" {
		t.Errorf("PE-1 = %+v, the tracker labels should win", got["PE-1"])
	}
	if got["PE-2"].Priority != "p3" || got["PE-2"].Quarter != "2027-Q1" {
		t.Errorf("PE-2 = %+v, an epic without labels keeps its local values", got["PE-2"])
	}
	if got["PE-3"].Quarter != "2026-Q4" {
		t.Errorf("PE-3 = %+v, the prefixed form should win", got["PE-3"])
	}
	if len(fake.writes) != 0 {
		t.Errorf("the import wrote to the tracker: %+v", fake.writes)
	}
}

func TestPendingPushesCoverThePriorityAndTheQuarter(t *testing.T) {
	fake := newHorizonTracker([]models.Task{
		{Key: "PE-1", Labels: []string{"roadmap:now", "priority:p1", "quarter:2026-q4"}},
		{Key: "PE-2", Labels: []string{"roadmap:now"}},
		{Key: "PE-3", Labels: []string{"2026-Q4"}},
	})
	database, proj := jiraProjectWithTracker(t, fake)
	now := "now"
	p1, q := "p1", "2026-Q4"
	for _, key := range []string{"PE-1", "PE-2", "PE-3", "M-4"} {
		mustSaveMacro(t, database, proj.ID, key, &now)
		if _, err := database.SaveMacroAxes(proj.ID, key, &p1, &q); err != nil {
			t.Fatal(err)
		}
	}

	pending, err := database.PendingHorizonPushes(t.Context(), proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{}
	for _, m := range pending {
		keys = append(keys, m.Key)
	}
	sort.Strings(keys)
	// PE-1 is up to date; PE-2 lacks both axes; PE-3 lacks the horizon and the
	// priority, its bare quarter already reads right; M-4 is a milestone.
	if !slices.Equal(keys, []string{"PE-2", "PE-3"}) {
		t.Fatalf("pending = %v", keys)
	}

	pushed, failures, err := database.PushPendingHorizons(t.Context(), proj.ID)
	if err != nil || len(failures) != 0 || pushed != 2 {
		t.Fatalf("pushed %d, failures %v, err %v", pushed, failures, err)
	}
	perKey := map[string][]string{}
	for _, w := range fake.writes {
		perKey[w.Key] = append(perKey[w.Key], w.Labels...)
	}
	if !slices.Equal(sortedCopy(perKey["PE-2"]), []string{"priority:p1", "quarter:2026-q4"}) {
		t.Errorf("PE-2 writes = %v", perKey["PE-2"])
	}
	if !slices.Equal(sortedCopy(perKey["PE-3"]), []string{"priority:p1", "roadmap:now"}) {
		t.Errorf("PE-3 writes = %v, the quarter already matched", perKey["PE-3"])
	}
}

func TestPushPendingHorizonsNamesTheFailingAxis(t *testing.T) {
	fake := newHorizonTracker(nil)
	fake.failKey = "PE-2"
	database, proj := jiraProjectWithTracker(t, fake)
	p0 := "p0"
	if _, err := database.SaveMacroAxes(proj.ID, "PE-2", &p0, nil); err != nil {
		t.Fatal(err)
	}
	_, failures, err := database.PushPendingHorizons(t.Context(), proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) != 1 || !strings.HasPrefix(failures[0], "PE-2 (priorité) : ") {
		t.Errorf("failures = %v", failures)
	}
}
