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
		t.Fatalf("database not initialised: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	proj, err := database.CreateProject(models.CreateProjectRequest{
		Name:         "Platform",
		Slug:         "platform",
		IssueTracker: "jira",
		JiraProject:  "PE",
	})
	if err != nil {
		t.Fatalf("project not created: %v", err)
	}
	database.TrackerRegistry().Register("jira", fake)
	return database, proj
}

func TestIsMilestoneKeyRecognisesOnlyNumberedMilestones(t *testing.T) {
	for _, key := range []string{"M-1", "m-42", " M-7 "} {
		if !isMilestoneKey(key) {
			t.Errorf("%q should be a milestone", key)
		}
	}
	for _, key := range []string{"PE-1141", "M-", "MOVE-2", "M-abc", ""} {
		if isMilestoneKey(key) {
			t.Errorf("%q should not be a milestone", key)
		}
	}
}

func TestBelongsToProjectComparesTheTrackerPrefix(t *testing.T) {
	proj := &models.Project{JiraProject: "PE"}
	if !belongsToProject("pe-1141", proj) {
		t.Error("case should not matter")
	}
	if belongsToProject("DS-12", proj) {
		t.Error("an epic of another project should not pass")
	}
	if belongsToProject("PEDANT-3", proj) {
		t.Error("the prefix must be followed by a dash")
	}
	// With no project key there is nothing to compare against, so nothing is
	// ruled out here and the tracker's capability refuses if anything must.
	if !belongsToProject("DS-12", &models.Project{}) {
		t.Error("with no project key the comparison should rule nothing out")
	}
}

func TestPushMacroHorizonLabelWritesTheAxisExclusively(t *testing.T) {
	fake := newHorizonTracker(nil)
	database, proj := jiraProjectWithTracker(t, fake)

	note, err := database.PushMacroHorizonLabel(t.Context(), proj.ID, "PE-1141", "next")
	if err != nil {
		t.Fatalf("push refused: %v", err)
	}
	if len(fake.writes) != 1 {
		t.Fatalf("want one write, got %d", len(fake.writes))
	}
	write := fake.writes[0]
	if write.Key != "PE-1141" {
		t.Errorf("written key = %q", write.Key)
	}
	if len(write.Labels) != 1 || write.Labels[0] != "roadmap:next" {
		t.Errorf("labels added = %v, want [roadmap:next]", write.Labels)
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
	// Nothing but the labels travels: an axis is not a ticket update.
	if write.Title != nil || write.Description != nil || write.Status != nil || write.Assignee != nil {
		t.Error("the write should carry labels and nothing else")
	}
	if note == "" {
		t.Error("the push should return a report")
	}
}

func TestPushMacroHorizonLabelClearsTheAxisWhenUnclassified(t *testing.T) {
	fake := newHorizonTracker(nil)
	database, proj := jiraProjectWithTracker(t, fake)

	if _, err := database.PushMacroHorizonLabel(t.Context(), proj.ID, "PE-1141", ""); err != nil {
		t.Fatalf("removal refused: %v", err)
	}
	write := fake.writes[0]
	if len(write.Labels) != 0 {
		t.Errorf("no label should be added, got %v", write.Labels)
	}
	if len(write.RemovedLabels) != 4 {
		t.Errorf("the four labels of the axis should be removed, got %v", write.RemovedLabels)
	}
}

func TestPushMacroHorizonLabelRefusesWhatCannotCarryALabel(t *testing.T) {
	fake := newHorizonTracker(nil)
	database, proj := jiraProjectWithTracker(t, fake)

	if _, err := database.PushMacroHorizonLabel(t.Context(), proj.ID, "M-3", "now"); err == nil {
		t.Error("a milestone should be refused")
	}
	if _, err := database.PushMacroHorizonLabel(t.Context(), proj.ID, "DS-12", "now"); err == nil {
		t.Error("an epic of another project should be refused")
	}
	if len(fake.writes) != 0 {
		t.Errorf("no write should leave, got %d", len(fake.writes))
	}
}

func TestImportMacroHorizonsReadsTheLabelsBack(t *testing.T) {
	fake := newHorizonTracker([]models.Task{
		{Key: "PE-1", Title: "Refonte API", Labels: []string{"platform", "roadmap:later"}, Status: models.StatusToClarify, TrackerStatus: "Open"},
		{Key: "PE-2", Title: "SSO", Labels: []string{"roadmap:now"}, Status: models.StatusFinished, TrackerStatus: "Done"},
		{Key: "PE-3", Title: "Sans axe", Labels: []string{"platform"}, Status: models.StatusToClarify},
	})
	database, proj := jiraProjectWithTracker(t, fake)

	// PE-3 is classified here and carries no label: the read knows nothing
	// about it and must not wipe that classification.
	local := "next"
	if _, err := database.SaveMacroMeta(proj.ID, "PE-3", &local, nil, nil, nil); err != nil {
		t.Fatalf("local classification not stored: %v", err)
	}

	if _, err := database.ImportMacroHorizons(t.Context(), proj.ID); err != nil {
		t.Fatalf("read refused: %v", err)
	}

	metas, err := database.GetProjectMacros(proj.ID)
	if err != nil {
		t.Fatalf("cannot read back: %v", err)
	}
	byKey := map[string]models.MacroMeta{}
	for _, m := range metas {
		byKey[m.Key] = m
	}
	if got := byKey["PE-1"].Horizon; got != "later" {
		t.Errorf("PE-1 horizon = %q, want later", got)
	}
	if got := byKey["PE-1"].Title; got != "Refonte API" {
		t.Errorf("PE-1 title = %q", got)
	}
	if !byKey["PE-2"].Closed {
		t.Error("PE-2 should be marked closed")
	}
	if got := byKey["PE-3"].Horizon; got != "next" {
		t.Errorf("PE-3 horizon = %q, want next kept", got)
	}
}

func TestPendingHorizonPushesListsOnlyWhatIsBehind(t *testing.T) {
	fake := newHorizonTracker([]models.Task{
		{Key: "PE-1", Labels: []string{"roadmap:now"}},
		{Key: "PE-2", Labels: []string{"roadmap:later"}},
	})
	database, proj := jiraProjectWithTracker(t, fake)

	now, later, unclassified := "now", "later", ""
	// Up to date on the tracker.
	mustSaveMacro(t, database, proj.ID, "PE-1", &now)
	// Classified NOW here, the tracker says LATER: behind.
	mustSaveMacro(t, database, proj.ID, "PE-2", &now)
	// Classified here, absent from the tracker: behind.
	mustSaveMacro(t, database, proj.ID, "PE-3", &later)
	// Unclassified: nothing to push.
	mustSaveMacro(t, database, proj.ID, "PE-4", &unclassified)
	// A milestone and a foreign epic: never pushable, so never behind.
	mustSaveMacro(t, database, proj.ID, "M-9", &now)
	mustSaveMacro(t, database, proj.ID, "DS-7", &now)

	pending, err := database.PendingHorizonPushes(t.Context(), proj.ID)
	if err != nil {
		t.Fatalf("listing refused: %v", err)
	}
	keys := []string{}
	for _, m := range pending {
		keys = append(keys, m.Key)
	}
	sort.Strings(keys)
	if len(keys) != 2 || keys[0] != "PE-2" || keys[1] != "PE-3" {
		t.Fatalf("behind = %v, want [PE-2 PE-3]", keys)
	}
}

func TestPendingHorizonPushesSkipsTheTrackerWhenNothingIsClassified(t *testing.T) {
	// A tracker that cannot read epics: the list must still answer an empty
	// list, not an error, on a roadmap where nothing is classified.
	fake := &horizonTracker{BaseTicketingSystem: tracker.BaseTicketingSystem{TrackerName: "jira"}}
	database, proj := jiraProjectWithTracker(t, fake)

	unclassified := ""
	mustSaveMacro(t, database, proj.ID, "PE-1", &unclassified)

	pending, err := database.PendingHorizonPushes(t.Context(), proj.ID)
	if err != nil {
		t.Fatalf("listing refused: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("want an empty list, got %v", pending)
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
		t.Fatalf("push refused: %v", err)
	}
	if pushed != 2 {
		t.Errorf("pushed = %d, want 2", pushed)
	}
	if len(failures) != 1 {
		t.Fatalf("failures = %v, want exactly one", failures)
	}
	if failures[0][:5] != "PE-2 " {
		t.Errorf("the failure should name the key, got %q", failures[0])
	}
}

func mustSaveMacro(t *testing.T, database *DB, projectID string, key string, horizon *string) {
	t.Helper()
	if _, err := database.SaveMacroMeta(projectID, key, horizon, nil, nil, nil); err != nil {
		t.Fatalf("macro %s not stored: %v", key, err)
	}
}
