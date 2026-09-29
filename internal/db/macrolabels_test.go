package db

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"tasks/internal/models"
	"tasks/internal/testsqlite"
)

// macroByKey reads a project's macros back and indexes them by key.
func macroByKey(t *testing.T, database *DB, projectID string) map[string]models.MacroMeta {
	t.Helper()
	metas, err := database.GetProjectMacros(projectID)
	if err != nil {
		t.Fatalf("cannot read the macros back: %v", err)
	}
	out := map[string]models.MacroMeta{}
	for _, m := range metas {
		out[m.Key] = m
	}
	return out
}

func TestImportMacroHorizonsKeepsTheEpicLabelsInOrder(t *testing.T) {
	fake := newHorizonTracker([]models.Task{
		{Key: "PE-1", Title: "Billing", Labels: []string{"domain-billing", "client-acme", "roadmap:next"}},
		{Key: "PE-2", Title: "Bare"},
	})
	database, proj := jiraProjectWithTracker(t, fake)

	if _, err := database.ImportMacroHorizons(t.Context(), proj.ID); err != nil {
		t.Fatalf("read refused: %v", err)
	}

	byKey := macroByKey(t, database, proj.ID)
	want := []string{"domain-billing", "client-acme", "roadmap:next"}
	if got := byKey["PE-1"].Labels; !reflect.DeepEqual(got, want) {
		t.Errorf("PE-1 labels = %v, want %v, horizon label included", got, want)
	}
	if got := byKey["PE-2"].Labels; got == nil || len(got) != 0 {
		t.Errorf("PE-2 labels = %#v, want an empty list", got)
	}
}

func TestImportMacroHorizonsDropsALabelRemovedOnTheTracker(t *testing.T) {
	fake := newHorizonTracker([]models.Task{
		{Key: "PE-1", Title: "Billing", Labels: []string{"domain-billing", "client-acme"}},
		{Key: "PE-2", Title: "Emptied", Labels: []string{"client-acme"}},
	})
	database, proj := jiraProjectWithTracker(t, fake)
	if _, err := database.ImportMacroHorizons(t.Context(), proj.ID); err != nil {
		t.Fatalf("first read refused: %v", err)
	}

	fake.epics[0].Labels = []string{"domain-billing"}
	fake.epics[1].Labels = nil
	if _, err := database.ImportMacroHorizons(t.Context(), proj.ID); err != nil {
		t.Fatalf("second read refused: %v", err)
	}

	byKey := macroByKey(t, database, proj.ID)
	if got := byKey["PE-1"].Labels; !reflect.DeepEqual(got, []string{"domain-billing"}) {
		t.Errorf("PE-1 labels = %v, want client-acme gone", got)
	}
	if got := byKey["PE-2"].Labels; len(got) != 0 {
		t.Errorf("PE-2 labels = %v, want none left", got)
	}
}

func TestSavingAMacroKeepsTheLabelsTheSyncRecorded(t *testing.T) {
	fake := newHorizonTracker([]models.Task{
		{Key: "PE-1", Title: "Billing", Labels: []string{"domain-billing"}},
	})
	database, proj := jiraProjectWithTracker(t, fake)
	if _, err := database.ImportMacroHorizons(t.Context(), proj.ID); err != nil {
		t.Fatalf("read refused: %v", err)
	}

	horizon := "later"
	framing := "Cadrage"
	if _, err := database.SaveMacroMeta(proj.ID, "PE-1", &horizon, nil, &framing, nil); err != nil {
		t.Fatalf("save refused: %v", err)
	}
	title := "Billing v2"
	if _, err := database.UpdateMacro(t.Context(), proj.ID, "PE-1", &title, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("update refused: %v", err)
	}

	if got := macroByKey(t, database, proj.ID)["PE-1"].Labels; !reflect.DeepEqual(got, []string{"domain-billing"}) {
		t.Errorf("labels = %v, want them kept across local saves", got)
	}
}

func TestAMacroCreatedLocallyCarriesNoLabel(t *testing.T) {
	database, proj := jiraProjectWithTracker(t, newHorizonTracker(nil))
	saved, err := database.SaveMacroMeta(proj.ID, "PE-9", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("save refused: %v", err)
	}
	if saved.Labels == nil || len(saved.Labels) != 0 {
		t.Errorf("saved labels = %#v, want an empty list", saved.Labels)
	}
}

func TestMigrateMacroCarriesItsLabels(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "test.db"), NewDB)
	if err != nil {
		t.Fatalf("database not initialised: %v", err)
	}
	defer database.Close()

	source, err := database.CreateProject(models.CreateProjectRequest{Name: "A", Slug: "a", IssueTracker: "local"})
	if err != nil {
		t.Fatalf("project A not created: %v", err)
	}
	target, err := database.CreateProject(models.CreateProjectRequest{Name: "B", Slug: "b", IssueTracker: "local"})
	if err != nil {
		t.Fatalf("project B not created: %v", err)
	}
	labels := []string{"client-acme", "roadmap:now"}
	if _, err := database.saveMacroMetaFull(source.ID, "M-1", nil, nil, nil, nil, nil, nil, nil, &labels); err != nil {
		t.Fatalf("macro not stored: %v", err)
	}

	if _, _, err := database.MigrateMacro(context.Background(), source.ID, "M-1", target.ID, false); err != nil {
		t.Fatalf("move refused: %v", err)
	}

	if got := macroByKey(t, database, target.ID)["M-1"].Labels; !reflect.DeepEqual(got, labels) {
		t.Errorf("moved labels = %v, want %v", got, labels)
	}
}

// syncedEpic records one epic of the fake tracker locally, as the sync would.
func syncedEpic(t *testing.T, labels ...string) (*DB, *models.Project, *horizonTracker) {
	t.Helper()
	fake := newHorizonTracker([]models.Task{{Key: "PE-1", Title: "Billing", Labels: labels}})
	database, proj := jiraProjectWithTracker(t, fake)
	if _, err := database.ImportMacroHorizons(t.Context(), proj.ID); err != nil {
		t.Fatalf("read refused: %v", err)
	}
	return database, proj, fake
}

func TestIsMacroAxisLabelRecognisesTheRoadmapPrefix(t *testing.T) {
	for label, want := range map[string]bool{
		"roadmap:now":    true,
		"Roadmap:Later":  true,
		"#roadmap:next":  true,
		" roadmap:x ":    true,
		"roadmap":        false,
		"client-acme":    false,
		"phase:discover": false,
	} {
		if got := IsMacroAxisLabel(label); got != want {
			t.Errorf("IsMacroAxisLabel(%q) = %v, want %v", label, got, want)
		}
	}
}

func TestValidateMacroLabelEditRefusesWhatCannotBeWritten(t *testing.T) {
	database, proj, _ := syncedEpic(t, "domain-billing")
	for _, tc := range []struct {
		name        string
		key         string
		add, remove []string
		want        string
	}{
		{"axis label added", "PE-1", []string{"roadmap:later"}, nil, "appartient à la roadmap"},
		{"axis label removed", "PE-1", nil, []string{"#Roadmap:now"}, "appartient à la roadmap"},
		{"empty label", "PE-1", []string{"  "}, nil, "vide"},
		{"label with a space", "PE-1", []string{"client acme"}, nil, "espace"},
		{"nothing to change", "PE-1", []string{"Domain-Billing"}, []string{"absent"}, "rien à modifier"},
		{"milestone", "M-3", []string{"client-acme"}, nil, "jalon"},
		{"epic of another project", "OPS-4", []string{"client-acme"}, nil, "autre projet"},
	} {
		if _, _, err := database.ValidateMacroLabelEdit(proj.ID, tc.key, tc.add, tc.remove); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want it to mention %q", tc.name, err, tc.want)
		}
	}
}

func TestValidateMacroLabelEditKeepsOnlyTheDelta(t *testing.T) {
	database, proj, _ := syncedEpic(t, "Domain-Billing", "roadmap:now")
	add, remove, err := database.ValidateMacroLabelEdit(proj.ID, "PE-1",
		[]string{" client-acme ", "client-acme", "domain-billing"},
		[]string{"domain-billing", "never-there"})
	if err != nil {
		t.Fatalf("edit refused: %v", err)
	}
	if !reflect.DeepEqual(add, []string{"client-acme"}) {
		t.Errorf("add = %v, want only the label the epic lacks", add)
	}
	// The tracker's spelling is what gets removed.
	if !reflect.DeepEqual(remove, []string{"Domain-Billing"}) {
		t.Errorf("remove = %v, want the stored spelling", remove)
	}
}

func TestPushMacroLabelsSendsTheDeltaThenRecordsIt(t *testing.T) {
	database, proj, fake := syncedEpic(t, "domain-billing", "roadmap:now")

	if _, err := database.PushMacroLabels(t.Context(), proj.ID, "PE-1", []string{"client-acme"}, []string{"domain-billing"}); err != nil {
		t.Fatalf("push refused: %v", err)
	}

	if len(fake.writes) != 1 {
		t.Fatalf("want one write, got %d", len(fake.writes))
	}
	write := fake.writes[0]
	if !reflect.DeepEqual(write.Labels, []string{"client-acme"}) || !reflect.DeepEqual(write.RemovedLabels, []string{"domain-billing"}) {
		t.Errorf("write = +%v -%v, want only the delta", write.Labels, write.RemovedLabels)
	}
	if write.Title != nil || write.Description != nil {
		t.Error("only the labels may travel")
	}
	want := []string{"roadmap:now", "client-acme"}
	if got := macroByKey(t, database, proj.ID)["PE-1"].Labels; !reflect.DeepEqual(got, want) {
		t.Errorf("stored labels = %v, want %v", got, want)
	}
}

func TestPushMacroLabelsChangesNothingWhenTheTrackerRefuses(t *testing.T) {
	database, proj, fake := syncedEpic(t, "domain-billing")
	fake.failKey = "PE-1"

	if _, err := database.PushMacroLabels(t.Context(), proj.ID, "PE-1", []string{"client-acme"}, nil); err == nil {
		t.Fatal("a refused write must fail the push")
	}

	if got := macroByKey(t, database, proj.ID)["PE-1"].Labels; !reflect.DeepEqual(got, []string{"domain-billing"}) {
		t.Errorf("stored labels = %v, want them untouched", got)
	}
	pending, err := database.PendingHorizonPushes(t.Context(), proj.ID)
	if err != nil {
		t.Fatalf("pending list unreadable: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("a failed label edit must leave nothing pending, got %v", pending)
	}
}
