package db

import (
	"context"
	"path/filepath"
	"reflect"
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
