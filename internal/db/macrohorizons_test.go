package db

import (
	"context"
	"path/filepath"
	"sort"
	"testing"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// horizonTracker records the label writes it is asked for and serves a fixed list
// of epics, so the horizon mirroring can be exercised without a tracker.
type horizonTracker struct {
	tracker.BaseTicketingSystem
	epics   []models.Task
	writes  []tracker.UpdateIssueRequest
	failKey string
}

func (f *horizonTracker) UpdateIssue(ctx context.Context, req tracker.UpdateIssueRequest) error {
	f.writes = append(f.writes, req)
	if f.failKey != "" && req.Key == f.failKey {
		return context.DeadlineExceeded
	}
	return nil
}

func (f *horizonTracker) ListEpics(ctx context.Context, req tracker.ProjectRequest) ([]models.Task, error) {
	return f.epics, nil
}

func newHorizonTracker(epics []models.Task) *horizonTracker {
	return &horizonTracker{
		BaseTicketingSystem: tracker.BaseTicketingSystem{
			TrackerName: "jira",
			Capabilities: []tracker.Capability{
				tracker.CapUpdate, tracker.CapLabels, tracker.CapEpic,
			},
		},
		epics: epics,
	}
}

// jiraProjectWithTracker opens a database holding one Jira project whose
// tracker is the fake, which is what every test here starts from.
func jiraProjectWithTracker(t *testing.T, fake *horizonTracker) (*DB, *models.Project) {
	t.Helper()
	database, err := NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("base non initialisée : %v", err)
	}
	t.Cleanup(func() { database.Close() })

	proj, err := database.CreateProject(models.CreateProjectRequest{
		Name:         "Platform",
		Slug:         "platform",
		IssueTracker: "jira",
		JiraProject:  "PE",
	})
	if err != nil {
		t.Fatalf("projet non créé : %v", err)
	}
	database.TrackerRegistry().Register("jira", fake)
	return database, proj
}

func TestIsMilestoneKeyRecognisesOnlyNumberedMilestones(t *testing.T) {
	for _, key := range []string{"M-1", "m-42", " M-7 "} {
		if !isMilestoneKey(key) {
			t.Errorf("%q devrait être un jalon", key)
		}
	}
	for _, key := range []string{"PE-1141", "M-", "MOVE-2", "M-abc", ""} {
		if isMilestoneKey(key) {
			t.Errorf("%q ne devrait pas être un jalon", key)
		}
	}
}

func TestBelongsToProjectComparesTheTrackerPrefix(t *testing.T) {
	proj := &models.Project{JiraProject: "PE"}
	if !belongsToProject("pe-1141", proj) {
		t.Error("la casse ne devrait pas compter")
	}
	if belongsToProject("DS-12", proj) {
		t.Error("un épic d'un autre projet ne devrait pas passer")
	}
	if belongsToProject("PEDANT-3", proj) {
		t.Error("le préfixe doit être suivi d'un tiret")
	}
	// Sans clé de projet, rien ne permet de trancher : tout passe, et c'est la
	// capacité du tracker qui refusera si elle doit refuser.
	if !belongsToProject("DS-12", &models.Project{}) {
		t.Error("sans clé de projet, la comparaison ne devrait rien écarter")
	}
}

func TestPushMacroHorizonLabelWritesTheAxisExclusively(t *testing.T) {
	fake := newHorizonTracker(nil)
	database, proj := jiraProjectWithTracker(t, fake)

	note, err := database.PushMacroHorizonLabel(t.Context(), proj.ID, "PE-1141", "next")
	if err != nil {
		t.Fatalf("poussée refusée : %v", err)
	}
	if len(fake.writes) != 1 {
		t.Fatalf("attendu une écriture, obtenu %d", len(fake.writes))
	}
	write := fake.writes[0]
	if write.Key != "PE-1141" {
		t.Errorf("clé écrite = %q", write.Key)
	}
	if len(write.Labels) != 1 || write.Labels[0] != "roadmap:next" {
		t.Errorf("labels posés = %v, attendu [roadmap:next]", write.Labels)
	}
	removed := append([]string{}, write.RemovedLabels...)
	sort.Strings(removed)
	want := []string{"roadmap:hidden", "roadmap:later", "roadmap:now"}
	if len(removed) != len(want) {
		t.Fatalf("labels retirés = %v, attendu %v", removed, want)
	}
	for i := range want {
		if removed[i] != want[i] {
			t.Fatalf("labels retirés = %v, attendu %v", removed, want)
		}
	}
	// Rien d'autre que les labels ne part : un axe n'est pas une mise à jour de
	// ticket.
	if write.Title != nil || write.Description != nil || write.Status != nil || write.Assignee != nil {
		t.Error("l'écriture devrait ne porter que des labels")
	}
	if note == "" {
		t.Error("la poussée devrait rendre un compte rendu")
	}
}

func TestPushMacroHorizonLabelClearsTheAxisWhenUnclassified(t *testing.T) {
	fake := newHorizonTracker(nil)
	database, proj := jiraProjectWithTracker(t, fake)

	if _, err := database.PushMacroHorizonLabel(t.Context(), proj.ID, "PE-1141", ""); err != nil {
		t.Fatalf("retrait refusé : %v", err)
	}
	write := fake.writes[0]
	if len(write.Labels) != 0 {
		t.Errorf("aucun label ne devrait être posé, obtenu %v", write.Labels)
	}
	if len(write.RemovedLabels) != 4 {
		t.Errorf("les quatre labels de l'axe devraient être retirés, obtenu %v", write.RemovedLabels)
	}
}

func TestPushMacroHorizonLabelRefusesWhatCannotCarryALabel(t *testing.T) {
	fake := newHorizonTracker(nil)
	database, proj := jiraProjectWithTracker(t, fake)

	if _, err := database.PushMacroHorizonLabel(t.Context(), proj.ID, "M-3", "now"); err == nil {
		t.Error("un jalon devrait être refusé")
	}
	if _, err := database.PushMacroHorizonLabel(t.Context(), proj.ID, "DS-12", "now"); err == nil {
		t.Error("un épic d'un autre projet devrait être refusé")
	}
	if len(fake.writes) != 0 {
		t.Errorf("aucune écriture ne devrait partir, obtenu %d", len(fake.writes))
	}
}

func TestImportMacroHorizonsReadsTheLabelsBack(t *testing.T) {
	fake := newHorizonTracker([]models.Task{
		{Key: "PE-1", Title: "Refonte API", Labels: []string{"platform", "roadmap:later"}, Status: models.StatusToClarify, TrackerStatus: "Open"},
		{Key: "PE-2", Title: "SSO", Labels: []string{"roadmap:now"}, Status: models.StatusFinished, TrackerStatus: "Done"},
		{Key: "PE-3", Title: "Sans axe", Labels: []string{"platform"}, Status: models.StatusToClarify},
	})
	database, proj := jiraProjectWithTracker(t, fake)

	// PE-3 est classé ici et ne porte aucun label : la lecture ne doit pas
	// effacer ce classement, elle n'en sait rien.
	local := "next"
	if _, err := database.SaveMacroMeta(proj.ID, "PE-3", &local, nil, nil, nil); err != nil {
		t.Fatalf("classement local non enregistré : %v", err)
	}

	if _, err := database.ImportMacroHorizons(t.Context(), proj.ID); err != nil {
		t.Fatalf("lecture refusée : %v", err)
	}

	metas, err := database.GetProjectMacros(proj.ID)
	if err != nil {
		t.Fatalf("relecture impossible : %v", err)
	}
	byKey := map[string]models.MacroMeta{}
	for _, m := range metas {
		byKey[m.Key] = m
	}
	if got := byKey["PE-1"].Horizon; got != "later" {
		t.Errorf("PE-1 horizon = %q, attendu later", got)
	}
	if got := byKey["PE-1"].Title; got != "Refonte API" {
		t.Errorf("PE-1 titre = %q", got)
	}
	if !byKey["PE-2"].Closed {
		t.Error("PE-2 devrait être marqué terminé")
	}
	if got := byKey["PE-3"].Horizon; got != "next" {
		t.Errorf("PE-3 horizon = %q, attendu next conservé", got)
	}
}

func TestPendingHorizonPushesListsOnlyWhatIsBehind(t *testing.T) {
	fake := newHorizonTracker([]models.Task{
		{Key: "PE-1", Labels: []string{"roadmap:now"}},
		{Key: "PE-2", Labels: []string{"roadmap:later"}},
	})
	database, proj := jiraProjectWithTracker(t, fake)

	now, later, unclassified := "now", "later", ""
	// À jour sur le tracker.
	mustSaveMacro(t, database, proj.ID, "PE-1", &now)
	// Classé ici en NOW, le tracker dit LATER : en retard.
	mustSaveMacro(t, database, proj.ID, "PE-2", &now)
	// Classé ici, absent du tracker : en retard.
	mustSaveMacro(t, database, proj.ID, "PE-3", &later)
	// Non classé : rien à pousser.
	mustSaveMacro(t, database, proj.ID, "PE-4", &unclassified)
	// Jalon et épic étranger : jamais poussables, donc jamais en retard.
	mustSaveMacro(t, database, proj.ID, "M-9", &now)
	mustSaveMacro(t, database, proj.ID, "DS-7", &now)

	pending, err := database.PendingHorizonPushes(t.Context(), proj.ID)
	if err != nil {
		t.Fatalf("liste refusée : %v", err)
	}
	keys := []string{}
	for _, m := range pending {
		keys = append(keys, m.Key)
	}
	sort.Strings(keys)
	if len(keys) != 2 || keys[0] != "PE-2" || keys[1] != "PE-3" {
		t.Fatalf("en retard = %v, attendu [PE-2 PE-3]", keys)
	}
}

func TestPendingHorizonPushesSkipsTheTrackerWhenNothingIsClassified(t *testing.T) {
	// Un tracker qui ne sait pas lire d'épics : la liste doit quand même rendre
	// une liste vide, et non une erreur, sur une roadmap où rien n'est classé.
	fake := &horizonTracker{BaseTicketingSystem: tracker.BaseTicketingSystem{TrackerName: "jira"}}
	database, proj := jiraProjectWithTracker(t, fake)

	unclassified := ""
	mustSaveMacro(t, database, proj.ID, "PE-1", &unclassified)

	pending, err := database.PendingHorizonPushes(t.Context(), proj.ID)
	if err != nil {
		t.Fatalf("liste refusée : %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("attendu une liste vide, obtenu %v", pending)
	}
}

func TestPushPendingHorizonsNamesEachFailureAndKeepsGoing(t *testing.T) {
	fake := newHorizonTracker(nil)
	fake.failKey = "PE-2"
	database, proj := jiraProjectWithTracker(t, fake)

	now := "now"
	mustSaveMacro(t, database, proj.ID, "PE-1", &now)
	mustSaveMacro(t, database, proj.ID, "PE-2", &now)
	mustSaveMacro(t, database, proj.ID, "PE-3", &now)

	pushed, failures, err := database.PushPendingHorizons(t.Context(), proj.ID)
	if err != nil {
		t.Fatalf("poussée refusée : %v", err)
	}
	if pushed != 2 {
		t.Errorf("poussés = %d, attendu 2", pushed)
	}
	if len(failures) != 1 {
		t.Fatalf("échecs = %v, attendu un seul", failures)
	}
	if failures[0][:5] != "PE-2 " {
		t.Errorf("l'échec devrait nommer la clé, obtenu %q", failures[0])
	}
}

func mustSaveMacro(t *testing.T, database *DB, projectID string, key string, horizon *string) {
	t.Helper()
	if _, err := database.SaveMacroMeta(projectID, key, horizon, nil, nil, nil); err != nil {
		t.Fatalf("macro %s non enregistrée : %v", key, err)
	}
}
