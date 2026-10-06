package db

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"tasks/internal/models"
)

func TestCleanEpicAxisPrefixes(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   models.EpicAxisPrefixes
		want models.EpicAxisPrefixes
	}{
		{"none", models.EpicAxisPrefixes{}, models.EpicAxisPrefixes{}},
		{"cleaned", models.EpicAxisPrefixes{Priority: " #Prio- ", Quarter: "Target/", Readiness: "STAGE-"}, models.EpicAxisPrefixes{Priority: "prio-", Quarter: "target/", Readiness: "stage-"}},
		{"blank is the default", models.EpicAxisPrefixes{Priority: "   "}, models.EpicAxisPrefixes{}},
		// A prefix equal to its default is stored as typed.
		{"default typed", models.EpicAxisPrefixes{Priority: "priority:"}, models.EpicAxisPrefixes{Priority: "priority:"}},
		{"one axis only", models.EpicAxisPrefixes{Quarter: "q-"}, models.EpicAxisPrefixes{Quarter: "q-"}},
	} {
		got, err := CleanEpicAxisPrefixes(tc.in)
		if err != nil {
			t.Errorf("%s: refused: %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: got %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

func TestCleanEpicAxisPrefixesRefusals(t *testing.T) {
	for _, tc := range []struct {
		name  string
		in    models.EpicAxisPrefixes
		names []string
	}{
		{"space", models.EpicAxisPrefixes{Priority: "prio p"}, []string{"« prio p »", "priorité"}},
		{"emptied", models.EpicAxisPrefixes{Readiness: "  #  "}, []string{"readiness", "vide"}},
		{"emptied hash", models.EpicAxisPrefixes{Quarter: "#"}, []string{"trimestre"}},
		{"prefix of another axis", models.EpicAxisPrefixes{Priority: "p", Quarter: "priority:"}, []string{"priorité", "trimestre"}},
		// The quarter keeps its default "quarter:", which "q" starts.
		{"prefix of a default", models.EpicAxisPrefixes{Priority: "q"}, []string{"priorité", "trimestre", "quarter:"}},
		{"same prefix twice", models.EpicAxisPrefixes{Priority: "x-", Readiness: "X-"}, []string{"priorité", "readiness"}},
		{"starts roadmap", models.EpicAxisPrefixes{Priority: "roadmap:prio-"}, []string{"roadmap:", "horizon"}},
		{"started by roadmap", models.EpicAxisPrefixes{Quarter: "road"}, []string{"roadmap:", "trimestre"}},
	} {
		_, err := CleanEpicAxisPrefixes(tc.in)
		if err == nil {
			t.Errorf("%s: should be refused", tc.name)
			continue
		}
		if !errors.Is(err, ErrInvalidEpicAxisPrefix) {
			t.Errorf("%s: %v does not match ErrInvalidEpicAxisPrefix", tc.name, err)
		}
		for _, want := range tc.names {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: %q does not name %q", tc.name, err, want)
			}
		}
	}
}

var customPrefixes = prefixesFor(&models.Project{EpicAxisPrefixes: models.EpicAxisPrefixes{Priority: "prio-", Quarter: "target/", Readiness: "stage-"}})

func TestPrefixesForFillsTheDefaults(t *testing.T) {
	if prefixesFor(nil) != defaultAxisPrefixes {
		t.Errorf("a nil project reads under the defaults")
	}
	got := prefixesFor(&models.Project{EpicAxisPrefixes: models.EpicAxisPrefixes{Quarter: "q-"}})
	if got.priority != "priority:" || got.quarter != "q-" || got.readiness != "readiness:" {
		t.Errorf("got %+v", got)
	}
}

func TestAxisReadsFollowTheProjectsPrefixes(t *testing.T) {
	a := customPrefixes
	for _, tc := range []struct {
		labels                       []string
		priority, quarter, readiness string
	}{
		{[]string{"prio-p1", "target/2026-q4", "stage-ready"}, "p1", "2026-Q4", "ready"},
		{[]string{"Prio-P2"}, "p2", "", ""},
		// The default prefixes are free labels once the project names others.
		{[]string{"priority:p1", "quarter:2026-q4", "readiness:ready"}, "", "", ""},
		// The bare quarter is read whatever the prefix.
		{[]string{"2026-Q3"}, "", "2026-Q3", ""},
		{[]string{"2026-Q3", "target/2026-q4"}, "", "2026-Q4", ""},
		// Only the existing values are read under a prefix.
		{[]string{"prio-p4", "target/2026-q5", "stage-done"}, "", "", ""},
		// The value follows the full prefix: "prio-priority:p1" is no priority.
		{[]string{"prio-priority:p1"}, "", "", ""},
		{[]string{"stage-idea", "stage-shaping"}, "", "", "shaping"},
	} {
		if got := a.PriorityFromLabels(tc.labels); got != tc.priority {
			t.Errorf("PriorityFromLabels(%v) = %q, want %q", tc.labels, got, tc.priority)
		}
		if got := a.QuarterFromLabels(tc.labels); got != tc.quarter {
			t.Errorf("QuarterFromLabels(%v) = %q, want %q", tc.labels, got, tc.quarter)
		}
		if got := a.ReadinessFromLabels(tc.labels); got != tc.readiness {
			t.Errorf("ReadinessFromLabels(%v) = %q, want %q", tc.labels, got, tc.readiness)
		}
	}

	short := prefixesFor(&models.Project{EpicAxisPrefixes: models.EpicAxisPrefixes{Priority: "p"}})
	if got := short.PriorityFromLabels([]string{"p1"}); got != "" {
		t.Errorf("under \"p\", \"p1\" is no priority, got %q", got)
	}
	if got := short.PriorityFromLabels([]string{"pp1"}); got != "p1" {
		t.Errorf("under \"p\", \"pp1\" is P1, got %q", got)
	}
}

func TestAxisLabelsFollowTheProjectsPrefixes(t *testing.T) {
	a := customPrefixes
	if got := a.PriorityLabel("p2"); got != "prio-p2" {
		t.Errorf("PriorityLabel = %q", got)
	}
	if got := a.QuarterLabel("2026-Q4"); got != "target/2026-q4" {
		t.Errorf("QuarterLabel = %q", got)
	}
	if got := a.ReadinessLabel("shaping"); got != "stage-shaping" {
		t.Errorf("ReadinessLabel = %q", got)
	}
	if !slices.Equal(a.AllPriorityLabels(), []string{"prio-p0", "prio-p1", "prio-p2", "prio-p3"}) {
		t.Errorf("AllPriorityLabels = %v", a.AllPriorityLabels())
	}
	if !slices.Equal(a.AllReadinessLabels(), []string{"stage-idea", "stage-shaping", "stage-ready"}) {
		t.Errorf("AllReadinessLabels = %v", a.AllReadinessLabels())
	}
	for label, want := range map[string]bool{
		"target/2026-q4": true, "target/soon": true, "2026-Q3": true,
		"quarter:2026-q4": false, "prio-p1": false,
	} {
		if got := a.isQuarterLabel(label); got != want {
			t.Errorf("isQuarterLabel(%q) = %v, want %v", label, got, want)
		}
	}
	for label, want := range map[string]bool{
		"prio-p1": true, "#Target/2026-q4": true, "stage-ready": true, "roadmap:now": true, "2026-Q3": true,
		"priority:p1": false, "quarter:2026-q4": false, "readiness:ready": false, "client-acme": false,
	} {
		if got := a.isMacroAxisLabel(label); got != want {
			t.Errorf("isMacroAxisLabel(%q) = %v, want %v", label, got, want)
		}
	}
}

// setAxisPrefixes stores prefixes on a project through the settings path.
func setAxisPrefixes(t *testing.T, database *DB, proj *models.Project, prefixes models.EpicAxisPrefixes) *models.Project {
	t.Helper()
	updated, err := database.UpdateProject(proj.ID, models.UpdateProjectRequest{EpicAxisPrefixes: &prefixes})
	if err != nil {
		t.Fatalf("prefixes refused: %v", err)
	}
	return updated
}

func TestProjectStoresItsAxisPrefixes(t *testing.T) {
	database, proj := jiraProjectWithTracker(t, newHorizonTracker(nil))
	if proj.EpicAxisPrefixes != (models.EpicAxisPrefixes{}) {
		t.Fatalf("a new project names no prefix, got %+v", proj.EpicAxisPrefixes)
	}

	updated := setAxisPrefixes(t, database, proj, models.EpicAxisPrefixes{Priority: "Prio-", Quarter: "#target/"})
	if updated.EpicAxisPrefixes != (models.EpicAxisPrefixes{Priority: "prio-", Quarter: "target/"}) {
		t.Errorf("stored = %+v", updated.EpicAxisPrefixes)
	}
	read, _ := database.GetProjectByID(proj.ID)
	if read.EpicAxisPrefixes.Priority != "prio-" {
		t.Errorf("read back = %+v", read.EpicAxisPrefixes)
	}
	listed, _ := database.GetProjects()
	for _, p := range listed {
		if p.ID == proj.ID && p.EpicAxisPrefixes.Quarter != "target/" {
			t.Errorf("listed = %+v", p.EpicAxisPrefixes)
		}
	}

	// A refusal stores nothing.
	bad := models.EpicAxisPrefixes{Priority: "p p"}
	if _, err := database.UpdateProject(proj.ID, models.UpdateProjectRequest{EpicAxisPrefixes: &bad}); !errors.Is(err, ErrInvalidEpicAxisPrefix) {
		t.Fatalf("err = %v", err)
	}
	read, _ = database.GetProjectByID(proj.ID)
	if read.EpicAxisPrefixes.Priority != "prio-" {
		t.Errorf("a refused save changed the prefixes: %+v", read.EpicAxisPrefixes)
	}

	// An update that does not name them keeps them; a cleared field is the default.
	name := "Platform 2"
	if _, err := database.UpdateProject(proj.ID, models.UpdateProjectRequest{Name: &name}); err != nil {
		t.Fatal(err)
	}
	read, _ = database.GetProjectByID(proj.ID)
	if read.EpicAxisPrefixes.Quarter != "target/" {
		t.Errorf("an unrelated update dropped the prefixes: %+v", read.EpicAxisPrefixes)
	}
	cleared := setAxisPrefixes(t, database, proj, models.EpicAxisPrefixes{Quarter: "target/"})
	if cleared.EpicAxisPrefixes.Priority != "" || prefixesFor(cleared).priority != PriorityLabelPrefix {
		t.Errorf("a cleared priority prefix is the default, got %+v", cleared.EpicAxisPrefixes)
	}
}

func TestCreateProjectCleansAndRefusesAxisPrefixes(t *testing.T) {
	database, _ := jiraProjectWithTracker(t, newHorizonTracker(nil))
	proj, err := database.CreateProject(models.CreateProjectRequest{Name: "Data", IssueTracker: "jira", JiraProject: "DATA", EpicAxisPrefixes: models.EpicAxisPrefixes{Readiness: " Stage- "}})
	if err != nil {
		t.Fatal(err)
	}
	if proj.EpicAxisPrefixes.Readiness != "stage-" {
		t.Errorf("created = %+v", proj.EpicAxisPrefixes)
	}
	if _, err := database.CreateProject(models.CreateProjectRequest{Name: "Ops", IssueTracker: "jira", JiraProject: "OPS", EpicAxisPrefixes: models.EpicAxisPrefixes{Quarter: "roadmap:q"}}); !errors.Is(err, ErrInvalidEpicAxisPrefix) {
		t.Errorf("err = %v", err)
	}
}

func TestImportReadsUnderTheProjectsPrefixes(t *testing.T) {
	fake := newHorizonTracker([]models.Task{
		{Key: "PE-1", Labels: []string{"prio-p1", "target/2026-q4", "stage-ready"}},
		{Key: "PE-2", Labels: []string{"priority:p1", "quarter:2026-q1", "readiness:idea"}},
		{Key: "PE-3", Labels: []string{"2026-Q3"}},
	})
	database, proj := jiraProjectWithTracker(t, fake)
	setAxisPrefixes(t, database, proj, models.EpicAxisPrefixes{Priority: "prio-", Quarter: "target/", Readiness: "stage-"})
	p3 := "p3"
	if _, err := database.SaveMacroAxes(proj.ID, "PE-2", &p3, nil, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := database.ImportMacroHorizons(t.Context(), proj.ID); err != nil {
		t.Fatal(err)
	}
	got := map[string]models.MacroMeta{}
	macros, _ := database.GetProjectMacros(proj.ID)
	for _, m := range macros {
		got[m.Key] = m
	}
	if m := got["PE-1"]; m.Priority != "p1" || m.Quarter != "2026-Q4" || m.Readiness != "ready" {
		t.Errorf("PE-1 = %+v", m)
	}
	// Under the former prefixes nothing is read: the local value stays.
	if m := got["PE-2"]; m.Priority != "p3" || m.Quarter != "" || m.Readiness != "" {
		t.Errorf("PE-2 = %+v", m)
	}
	if m := got["PE-3"]; m.Quarter != "2026-Q3" {
		t.Errorf("PE-3 = %+v, the bare quarter is read whatever the prefix", m)
	}
}

func TestPushesWriteUnderTheProjectsPrefixes(t *testing.T) {
	fake := newHorizonTracker([]models.Task{{Key: "PE-1", Labels: []string{"priority:p1", "target/2026-q2", "target/soon", "2026-Q3", "quarter:2026-q1"}}})
	database, proj := jiraProjectWithTracker(t, fake)
	setAxisPrefixes(t, database, proj, models.EpicAxisPrefixes{Priority: "prio-", Quarter: "target/", Readiness: "stage-"})
	ctx := t.Context()

	if _, err := database.PushMacroPriorityLabel(ctx, proj.ID, "PE-1", "p2"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.PushMacroQuarterLabel(ctx, proj.ID, "PE-1", "2026-Q4"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.PushMacroReadinessLabel(ctx, proj.ID, "PE-1", "shaping"); err != nil {
		t.Fatal(err)
	}
	if len(fake.writes) != 3 {
		t.Fatalf("writes = %d", len(fake.writes))
	}
	priority, quarter, readiness := fake.writes[0], fake.writes[1], fake.writes[2]
	if !slices.Equal(priority.Labels, []string{"prio-p2"}) || !slices.Equal(sortedCopy(priority.RemovedLabels), []string{"prio-p0", "prio-p1", "prio-p3"}) {
		t.Errorf("priority write = +%v -%v", priority.Labels, priority.RemovedLabels)
	}
	// "priority:p1" and "quarter:2026-q1" sit under former prefixes: left alone.
	if !slices.Equal(quarter.Labels, []string{"target/2026-q4"}) || !slices.Equal(sortedCopy(quarter.RemovedLabels), []string{"2026-Q3", "target/2026-q2", "target/soon"}) {
		t.Errorf("quarter write = +%v -%v", quarter.Labels, quarter.RemovedLabels)
	}
	if !slices.Equal(readiness.Labels, []string{"stage-shaping"}) || !slices.Equal(sortedCopy(readiness.RemovedLabels), []string{"stage-idea", "stage-ready"}) {
		t.Errorf("readiness write = +%v -%v", readiness.Labels, readiness.RemovedLabels)
	}
	for _, w := range fake.writes {
		if slices.Contains(w.RemovedLabels, "priority:p1") || slices.Contains(w.RemovedLabels, "quarter:2026-q1") {
			t.Errorf("a label under a former prefix was removed: %v", w.RemovedLabels)
		}
	}
}

func TestChangingAPrefixMigratesNothingAndListsThePush(t *testing.T) {
	fake := newHorizonTracker([]models.Task{{Key: "PE-1", Labels: []string{"roadmap:now", "priority:p1"}}})
	database, proj := jiraProjectWithTracker(t, fake)
	if _, err := database.ImportMacroHorizons(t.Context(), proj.ID); err != nil {
		t.Fatal(err)
	}
	pending, err := database.PendingHorizonPushes(t.Context(), proj.ID)
	if err != nil || len(pending) != 0 {
		t.Fatalf("under the default prefix PE-1 is up to date: %v, %v", pending, err)
	}

	setAxisPrefixes(t, database, proj, models.EpicAxisPrefixes{Priority: "prio-"})
	if len(fake.writes) != 0 {
		t.Errorf("changing a prefix wrote on the tracker: %+v", fake.writes)
	}
	macros, _ := database.GetProjectMacros(proj.ID)
	if len(macros) != 1 || macros[0].Priority != "p1" {
		t.Fatalf("the stored priority must be kept: %+v", macros)
	}

	pending, err = database.PendingHorizonPushes(t.Context(), proj.ID)
	if err != nil || len(pending) != 1 || pending[0].Key != "PE-1" {
		t.Fatalf("PE-1 is late under the new prefix: %v, %v", pending, err)
	}
	if _, failures, err := database.PushPendingHorizons(t.Context(), proj.ID); err != nil || len(failures) != 0 {
		t.Fatalf("push: %v, %v", failures, err)
	}
	if len(fake.writes) != 1 || !slices.Equal(fake.writes[0].Labels, []string{"prio-p1"}) {
		t.Errorf("confirmed push = %+v", fake.writes)
	}

	// The former label is a free label now.
	if _, _, err := database.ValidateMacroLabelEdit(proj.ID, "PE-1", nil, []string{"priority:p1"}); err != nil {
		t.Errorf("a label under a former prefix is free: %v", err)
	}
}

func TestFreeLabelEditFollowsTheProjectsPrefixes(t *testing.T) {
	database, proj, _ := syncedEpic(t, "priority:p1")
	setAxisPrefixes(t, database, proj, models.EpicAxisPrefixes{Priority: "prio-"})
	for _, label := range []string{"prio-p1", "Prio-P0", "roadmap:now", "2026-Q3", "quarter:2026-q4"} {
		if _, _, err := database.ValidateMacroLabelEdit(proj.ID, "PE-1", []string{label}, nil); err == nil || !strings.Contains(err.Error(), "appartient à un axe de la roadmap") {
			t.Errorf("%q: err = %v, want the axis refusal", label, err)
		}
	}
	add, _, err := database.ValidateMacroLabelEdit(proj.ID, "PE-1", []string{"priority:p2"}, nil)
	if err != nil || !slices.Equal(add, []string{"priority:p2"}) {
		t.Errorf("a label under a former prefix is free: %v, %v", add, err)
	}
}

func TestForeignEpicsUseTheRoadmapProjectsPrefixes(t *testing.T) {
	fake := newDeclaredTracker(nil)
	database, proj := declaredProject(t, fake, "DATA")
	proj = openAxisWrites(t, database, proj)
	setAxisPrefixes(t, database, proj, models.EpicAxisPrefixes{Priority: "prio-"})

	if _, err := database.PushMacroPriorityLabel(t.Context(), proj.ID, "DATA-12", "p1"); err != nil {
		t.Fatal(err)
	}
	if len(fake.writes) != 1 || !slices.Equal(fake.writes[0].Labels, []string{"prio-p1"}) {
		t.Errorf("writes = %+v", fake.writes)
	}
}
