package db

import (
	"slices"
	"strings"
	"testing"

	"tasks/internal/models"
)

func TestNormalizeReadiness(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", ""}, {"  ", ""}, {"idea", "idea"}, {"Ready", "ready"}, {" shaping ", "shaping"},
		{"readiness:ready", "ready"}, {"#Readiness:Ready", "ready"},
	} {
		got, err := NormalizeReadiness(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("NormalizeReadiness(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
	for _, in := range []string{"soon", "done", "readiness:soon", "readiness:", "priority:p1"} {
		if _, err := NormalizeReadiness(in); err == nil {
			t.Errorf("NormalizeReadiness(%q) should be refused", in)
		}
	}
}

func TestReadinessFromLabels(t *testing.T) {
	for _, tc := range []struct {
		labels []string
		want   string
	}{
		{nil, ""},
		{[]string{"roadmap:now", "readiness:shaping"}, "shaping"},
		{[]string{"Readiness:Idea"}, "idea"},
		{[]string{"#readiness:ready"}, "ready"},
		// Two levels read as the most advanced, whatever the order.
		{[]string{"readiness:ready", "readiness:idea"}, "ready"},
		{[]string{"readiness:idea", "readiness:shaping"}, "shaping"},
		// An unknown value is read as absent.
		{[]string{"readiness:soon"}, ""},
		{[]string{"readiness:soon", "readiness:idea"}, "idea"},
		{[]string{"ready"}, ""},
	} {
		if got := ReadinessFromLabels(tc.labels); got != tc.want {
			t.Errorf("ReadinessFromLabels(%v) = %q, want %q", tc.labels, got, tc.want)
		}
	}
	if got := ReadinessLabel("ready"); got != "readiness:ready" {
		t.Errorf("ReadinessLabel(ready) = %q", got)
	}
	if got := ReadinessLabel(""); got != "" {
		t.Errorf("ReadinessLabel('') = %q", got)
	}
	if !slices.Equal(AllReadinessLabels(), []string{"readiness:idea", "readiness:shaping", "readiness:ready"}) {
		t.Errorf("AllReadinessLabels = %v", AllReadinessLabels())
	}
}

func TestSaveMacroAxesStoresAndClearsTheReadiness(t *testing.T) {
	database, proj := jiraProjectWithTracker(t, newHorizonTracker(nil))
	horizon := "next"
	description := "Le cadrage"
	if _, err := database.SaveMacroMeta(proj.ID, "PE-1", &horizon, &description, nil, nil); err != nil {
		t.Fatal(err)
	}
	p1, q := "p1", "2026-Q4"
	if _, err := database.SaveMacroAxes(proj.ID, "PE-1", &p1, &q, nil); err != nil {
		t.Fatal(err)
	}

	ready := "Ready"
	saved, err := database.SaveMacroAxes(proj.ID, "PE-1", nil, nil, &ready)
	if err != nil {
		t.Fatalf("readiness not saved: %v", err)
	}
	if saved.Readiness != "ready" {
		t.Errorf("saved = %q, want ready", saved.Readiness)
	}
	macros, _ := database.GetProjectMacros(proj.ID)
	m := macros[0]
	if m.Readiness != "ready" || m.Priority != "p1" || m.Quarter != "2026-Q4" || m.Horizon != "next" || m.Description != "Le cadrage" {
		t.Errorf("read back = %+v", m)
	}

	// Another axis edit leaves the readiness alone, an empty value clears it.
	p2 := "p2"
	if _, err := database.SaveMacroAxes(proj.ID, "PE-1", &p2, nil, nil); err != nil {
		t.Fatal(err)
	}
	macros, _ = database.GetProjectMacros(proj.ID)
	if macros[0].Readiness != "ready" {
		t.Errorf("a priority edit changed the readiness: %+v", macros[0])
	}
	empty := ""
	if _, err := database.SaveMacroAxes(proj.ID, "PE-1", nil, nil, &empty); err != nil {
		t.Fatal(err)
	}
	macros, _ = database.GetProjectMacros(proj.ID)
	if macros[0].Readiness != "" || macros[0].Priority != "p2" {
		t.Errorf("after clearing = %+v", macros[0])
	}

	bad := "done"
	if _, err := database.SaveMacroAxes(proj.ID, "PE-1", &p1, nil, &bad); err == nil {
		t.Fatal("an unknown readiness should be refused")
	}
	macros, _ = database.GetProjectMacros(proj.ID)
	if macros[0].Priority != "p2" {
		t.Errorf("the priority was saved alongside a refused readiness: %+v", macros[0])
	}
}

func TestPushMacroReadinessLabelWritesTheAxisExclusively(t *testing.T) {
	fake := newHorizonTracker(nil)
	database, proj := jiraProjectWithTracker(t, fake)

	note, err := database.PushMacroReadinessLabel(t.Context(), proj.ID, "PE-1", "shaping")
	if err != nil {
		t.Fatalf("push refused: %v", err)
	}
	if note != "« readiness:shaping » posé sur PE-1" {
		t.Errorf("note = %q", note)
	}
	if len(fake.writes) != 1 {
		t.Fatalf("want one write, got %d", len(fake.writes))
	}
	w := fake.writes[0]
	if !slices.Equal(w.Labels, []string{"readiness:shaping"}) {
		t.Errorf("added = %v", w.Labels)
	}
	if !slices.Equal(sortedCopy(w.RemovedLabels), []string{"readiness:idea", "readiness:ready"}) {
		t.Errorf("removed = %v", w.RemovedLabels)
	}
	if w.Title != nil || w.Description != nil || w.Status != nil || w.Assignee != nil {
		t.Error("the write should carry labels and nothing else")
	}

	note, err = database.PushMacroReadinessLabel(t.Context(), proj.ID, "PE-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if note != "labels de readiness retirés de PE-1" {
		t.Errorf("note = %q", note)
	}
	cleared := fake.writes[1]
	if len(cleared.Labels) != 0 || !slices.Equal(sortedCopy(cleared.RemovedLabels), []string{"readiness:idea", "readiness:ready", "readiness:shaping"}) {
		t.Errorf("clearing should remove the three labels and add none: %+v", cleared)
	}
}

func TestPushMacroReadinessFailureSaysTheLevelStaysInSectile(t *testing.T) {
	fake := newHorizonTracker(nil)
	fake.failKey = "PE-1"
	database, proj := jiraProjectWithTracker(t, fake)

	_, err := database.PushMacroReadinessLabel(t.Context(), proj.ID, "PE-1", "ready")
	if err == nil || !strings.Contains(err.Error(), "readiness posée dans Sectile mais pas sur PE-1") {
		t.Errorf("err = %v", err)
	}
	_, err = database.PushMacroReadinessLabel(t.Context(), proj.ID, "M-2", "ready")
	if err == nil || !strings.Contains(err.Error(), "readiness posée dans Sectile mais pas sur M-2") {
		t.Errorf("a milestone refusal should say so: %v", err)
	}
}

func TestImportMacroHorizonsReadsTheReadiness(t *testing.T) {
	fake := newHorizonTracker([]models.Task{
		{Key: "PE-1", Title: "Tracker wins", Labels: []string{"readiness:ready"}},
		{Key: "PE-2", Title: "No label", Labels: []string{"roadmap:now"}},
		{Key: "PE-3", Title: "Two levels", Labels: []string{"readiness:ready", "readiness:idea"}},
		{Key: "PE-4", Title: "Unknown", Labels: []string{"readiness:soon"}},
	})
	database, proj := jiraProjectWithTracker(t, fake)
	idea := "idea"
	for _, key := range []string{"PE-1", "PE-2", "PE-4"} {
		if _, err := database.SaveMacroAxes(proj.ID, key, nil, nil, &idea); err != nil {
			t.Fatal(err)
		}
	}

	note, err := database.ImportMacroHorizons(t.Context(), proj.ID)
	if err != nil {
		t.Fatalf("import refused: %v", err)
	}
	if !strings.Contains(note, "2 jugée(s)") {
		t.Errorf("summary = %q", note)
	}
	got := map[string]models.MacroMeta{}
	macros, _ := database.GetProjectMacros(proj.ID)
	for _, m := range macros {
		got[m.Key] = m
	}
	for key, want := range map[string]string{"PE-1": "ready", "PE-2": "idea", "PE-3": "ready", "PE-4": "idea"} {
		if got[key].Readiness != want {
			t.Errorf("%s readiness = %q, want %q", key, got[key].Readiness, want)
		}
	}
	if len(fake.writes) != 0 {
		t.Errorf("the import wrote to the tracker: %+v", fake.writes)
	}

	// PE-2 kept a level its epic does not carry, so it is pending; PE-1 and
	// PE-3 read right.
	pending, err := database.PendingHorizonPushes(t.Context(), proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{}
	for _, m := range pending {
		keys = append(keys, m.Key)
	}
	slices.Sort(keys)
	if !slices.Equal(keys, []string{"PE-2", "PE-4"}) {
		t.Errorf("pending = %v", keys)
	}
}

func TestPendingPushesCoverTheReadinessOnly(t *testing.T) {
	fake := newHorizonTracker([]models.Task{
		{Key: "PE-1", Labels: []string{"priority:p1", "readiness:ready"}},
		{Key: "PE-2", Labels: []string{"priority:p1"}},
	})
	database, proj := jiraProjectWithTracker(t, fake)
	p1, ready := "p1", "ready"
	for _, key := range []string{"PE-1", "PE-2", "M-3", "OTHER-9"} {
		if _, err := database.SaveMacroAxes(proj.ID, key, &p1, nil, &ready); err != nil {
			t.Fatal(err)
		}
	}

	pushed, failures, err := database.PushPendingHorizons(t.Context(), proj.ID)
	if err != nil || len(failures) != 0 || pushed != 1 {
		t.Fatalf("pushed %d, failures %v, err %v", pushed, failures, err)
	}
	// PE-2 differs on the readiness only, so only that axis is written; the
	// milestone and the other project's epic are never pushed.
	if len(fake.writes) != 1 || fake.writes[0].Key != "PE-2" || !slices.Equal(fake.writes[0].Labels, []string{"readiness:ready"}) {
		t.Errorf("writes = %+v", fake.writes)
	}
}

func TestPushPendingHorizonsNamesTheFailingReadiness(t *testing.T) {
	fake := newHorizonTracker(nil)
	fake.failKey = "PE-2"
	database, proj := jiraProjectWithTracker(t, fake)
	shaping := "shaping"
	if _, err := database.SaveMacroAxes(proj.ID, "PE-2", nil, nil, &shaping); err != nil {
		t.Fatal(err)
	}
	_, failures, err := database.PushPendingHorizons(t.Context(), proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) != 1 || !strings.HasPrefix(failures[0], "PE-2 (readiness) : ") {
		t.Errorf("failures = %v", failures)
	}
}
