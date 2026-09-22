package db

import (
	"path/filepath"
	"testing"

	"tasks/internal/models"
)

func TestEscapeLikeNeutralisesWildcards(t *testing.T) {
	if got := escapeLike("team_alpha"); got != `team\_alpha` {
		t.Errorf("expected the underscore to be escaped, got %q", got)
	}
	if got := escapeLike("100%"); got != `100\%` {
		t.Errorf("expected the percent sign to be escaped, got %q", got)
	}
	if got := escapeLike(`a\b`); got != `a\\b` {
		t.Errorf("expected the backslash to be escaped, got %q", got)
	}
	if got := escapeLike("team-alpha"); got != "team-alpha" {
		t.Errorf("expected a plain value to be left alone, got %q", got)
	}
}

func TestHasAndAddProjectLabel(t *testing.T) {
	if !hasProjectLabel([]string{"bug", "Team-Alpha"}, "team-alpha") {
		t.Error("expected the comparison to ignore case")
	}
	if hasProjectLabel([]string{"team-alphabet"}, "team-alpha") {
		t.Error("expected a whole-label match, not a prefix one")
	}
	if hasProjectLabel([]string{"bug"}, "") {
		t.Error("an empty project label belongs to nobody")
	}

	proj := &models.Project{ProjectLabel: "team-alpha"}
	added := addProjectLabel([]string{"bug"}, proj)
	if len(added) != 2 || added[1] != "team-alpha" {
		t.Errorf("expected the label to be stamped, got %v", added)
	}
	if again := addProjectLabel([]string{"bug", "TEAM-ALPHA"}, proj); len(again) != 2 {
		t.Errorf("expected no duplicate whatever the case, got %v", again)
	}
	if none := addProjectLabel([]string{"bug"}, &models.Project{}); len(none) != 1 {
		t.Errorf("expected a project without a label to stamp nothing, got %v", none)
	}
	if nilProj := addProjectLabel([]string{"bug"}, nil); len(nilProj) != 1 {
		t.Errorf("expected a nil project to stamp nothing, got %v", nilProj)
	}
}

// membershipTestDB builds a store holding two projects — one filtering on
// "team-alpha", one filtering on nothing — with a handful of tasks each.
func membershipTestDB(t *testing.T) (*DB, *models.Project, *models.Project) {
	t.Helper()

	database, err := NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("failed to initialize database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	// The label is configured after the tasks exist: creating a task in a labelled
	// project stamps it, which is the behaviour of US5 and would leave the fixture
	// with no unattributed ticket to filter out.
	labelled, err := database.CreateProject(models.CreateProjectRequest{
		Name:         "Shared board",
		Slug:         "shared-board",
		IssueTracker: "local",
	})
	if err != nil {
		t.Fatalf("failed to create the labelled project: %v", err)
	}
	open, err := database.CreateProject(models.CreateProjectRequest{
		Name:         "Open board",
		Slug:         "open-board",
		IssueTracker: "local",
	})
	if err != nil {
		t.Fatalf("failed to create the unlabelled project: %v", err)
	}

	mk := func(projectID, title string, labels []string) {
		t.Helper()
		if _, err := database.CreateTask(models.CreateTaskRequest{
			ProjectID: projectID,
			Title:     title,
			Status:    models.StatusToClarify,
			Priority:  models.PriorityMedium,
			Labels:    labels,
			Source:    "local",
		}); err != nil {
			t.Fatalf("failed to create task %q: %v", title, err)
		}
	}

	mk(labelled.ID, "carries the label", []string{"team-alpha"})
	mk(labelled.ID, "carries it in another case", []string{"Team-Alpha"})
	mk(labelled.ID, "carries a look-alike", []string{"team-alphabet"})
	mk(labelled.ID, "carries nothing", []string{"bug"})
	mk(open.ID, "belongs to a project without a label", []string{"bug"})

	teamAlpha := "team-alpha"
	labelled, err = database.UpdateProject(labelled.ID, models.UpdateProjectRequest{ProjectLabel: &teamAlpha})
	if err != nil {
		t.Fatalf("failed to configure the membership label: %v", err)
	}

	return database, labelled, open
}

func titlesOf(tasks []models.Task) map[string]bool {
	out := map[string]bool{}
	for _, task := range tasks {
		out[task.Title] = true
	}
	return out
}

func TestMembershipFilterOnALabelledProject(t *testing.T) {
	database, labelled, _ := membershipTestDB(t)

	tasks, err := database.GetTasks("", "", "", "", labelled.ID, "", "", "", "", nil, nil, false)
	if err != nil {
		t.Fatalf("GetTasks failed: %v", err)
	}
	got := titlesOf(tasks)
	if !got["carries the label"] || !got["carries it in another case"] {
		t.Errorf("expected the carriers to be shown, got %v", got)
	}
	if got["carries a look-alike"] {
		t.Error("team-alphabet must not match team-alpha")
	}
	if got["carries nothing"] {
		t.Error("expected a ticket without the label to be hidden")
	}
	if got["belongs to a project without a label"] {
		t.Error("another project's task leaked into this project's scope")
	}
}

func TestMembershipFilterLiftedByMembershipAll(t *testing.T) {
	database, labelled, _ := membershipTestDB(t)

	tasks, err := database.GetTasksForUser("", "", "", "", "", labelled.ID, "", "", "", "", nil, nil, false, true)
	if err != nil {
		t.Fatalf("GetTasksForUser failed: %v", err)
	}
	got := titlesOf(tasks)
	for _, want := range []string{"carries the label", "carries it in another case", "carries a look-alike", "carries nothing"} {
		if !got[want] {
			t.Errorf("expected %q to be shown with membership=all, got %v", want, got)
		}
	}
}

func TestMembershipFilterLeavesAnUnlabelledProjectAlone(t *testing.T) {
	database, _, open := membershipTestDB(t)

	tasks, err := database.GetTasks("", "", "", "", open.ID, "", "", "", "", nil, nil, false)
	if err != nil {
		t.Fatalf("GetTasks failed: %v", err)
	}
	if got := titlesOf(tasks); !got["belongs to a project without a label"] || len(got) != 1 {
		t.Errorf("expected the whole board of an unlabelled project, got %v", got)
	}
}

func TestMembershipFilterAcrossEveryProject(t *testing.T) {
	database, _, _ := membershipTestDB(t)

	// No project selected: each project answers for its own tasks — the labelled
	// one with its carriers, the unlabelled one with everything.
	tasks, err := database.GetTasks("", "", "", "", "", "", "", "", "", nil, nil, false)
	if err != nil {
		t.Fatalf("GetTasks failed: %v", err)
	}
	got := titlesOf(tasks)
	if !got["carries the label"] || !got["belongs to a project without a label"] {
		t.Errorf("expected both projects to contribute, got %v", got)
	}
	if got["carries nothing"] || got["carries a look-alike"] {
		t.Errorf("expected the labelled project to contribute its slice only, got %v", got)
	}
}

func TestMembershipFilterWithAWildcardInTheLabel(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("failed to initialize database: %v", err)
	}
	defer database.Close()

	proj, err := database.CreateProject(models.CreateProjectRequest{
		Name:         "Underscored",
		Slug:         "underscored",
		IssueTracker: "local",
	})
	if err != nil {
		t.Fatalf("failed to create the project: %v", err)
	}
	for _, labels := range [][]string{{"team_alpha"}, {"teamXalpha"}} {
		if _, err := database.CreateTask(models.CreateTaskRequest{
			ProjectID: proj.ID,
			Title:     labels[0],
			Status:    models.StatusToClarify,
			Priority:  models.PriorityMedium,
			Labels:    labels,
			Source:    "local",
		}); err != nil {
			t.Fatalf("failed to create task: %v", err)
		}
	}
	underscored := "team_alpha"
	if _, err := database.UpdateProject(proj.ID, models.UpdateProjectRequest{ProjectLabel: &underscored}); err != nil {
		t.Fatalf("failed to configure the membership label: %v", err)
	}

	tasks, err := database.GetTasks("", "", "", "", proj.ID, "", "", "", "", nil, nil, false)
	if err != nil {
		t.Fatalf("GetTasks failed: %v", err)
	}
	got := titlesOf(tasks)
	if !got["team_alpha"] {
		t.Errorf("expected the exact carrier to be shown, got %v", got)
	}
	if got["teamXalpha"] {
		t.Error("the underscore was treated as a LIKE wildcard")
	}
}

func TestFacetsFollowTheMembershipScope(t *testing.T) {
	database, labelled, _ := membershipTestDB(t)

	narrowed, err := database.GetTaskFacets(labelled.ID)
	if err != nil {
		t.Fatalf("GetTaskFacets failed: %v", err)
	}
	if narrowed.Total != 2 {
		t.Errorf("expected the two carriers to be counted, got %d", narrowed.Total)
	}

	widened, err := database.GetTaskFacetsForUser("", labelled.ID, true)
	if err != nil {
		t.Fatalf("GetTaskFacetsForUser failed: %v", err)
	}
	if widened.Total != 4 {
		t.Errorf("expected the whole board to be counted with membership=all, got %d", widened.Total)
	}
}

func TestCreateTaskStampsTheProjectLabel(t *testing.T) {
	database, labelled, open := membershipTestDB(t)

	stamped, err := database.CreateTask(models.CreateTaskRequest{
		ProjectID: labelled.ID,
		Title:     "created in a labelled project",
		Status:    models.StatusToClarify,
		Priority:  models.PriorityMedium,
		Labels:    []string{"bug"},
		Source:    "local",
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	if !hasProjectLabel(stamped.Labels, "team-alpha") {
		t.Errorf("expected the membership label to be stamped locally, got %v", stamped.Labels)
	}

	already, err := database.CreateTask(models.CreateTaskRequest{
		ProjectID: labelled.ID,
		Title:     "already carries it",
		Status:    models.StatusToClarify,
		Priority:  models.PriorityMedium,
		Labels:    []string{"TEAM-ALPHA"},
		Source:    "local",
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	count := 0
	for _, l := range already.Labels {
		if cleanLabel(l) == "team-alpha" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected the label not to be duplicated, got %v", already.Labels)
	}

	untouched, err := database.CreateTask(models.CreateTaskRequest{
		ProjectID: open.ID,
		Title:     "created in an unlabelled project",
		Status:    models.StatusToClarify,
		Priority:  models.PriorityMedium,
		Labels:    []string{"bug"},
		Source:    "local",
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	for _, l := range untouched.Labels {
		if cleanLabel(l) == "team-alpha" {
			t.Errorf("an unlabelled project stamped a label: %v", untouched.Labels)
		}
	}
}

func TestProjectLabelSurvivesASaveAndRefusesWhitespace(t *testing.T) {
	database, labelled, _ := membershipTestDB(t)

	reread, err := database.GetProjectByID(labelled.ID)
	if err != nil {
		t.Fatalf("GetProjectByID failed: %v", err)
	}
	if reread.ProjectLabel != "team-alpha" {
		t.Errorf("expected the stored label to be read back, got %q", reread.ProjectLabel)
	}

	bad := "team alpha"
	if _, err := database.UpdateProject(labelled.ID, models.UpdateProjectRequest{ProjectLabel: &bad}); err == nil {
		t.Error("expected a label with a space to be refused")
	}

	good := "  team-beta  "
	updated, err := database.UpdateProject(labelled.ID, models.UpdateProjectRequest{ProjectLabel: &good})
	if err != nil {
		t.Fatalf("UpdateProject failed: %v", err)
	}
	if updated.ProjectLabel != "team-beta" {
		t.Errorf("expected the surrounding whitespace to be trimmed, got %q", updated.ProjectLabel)
	}
}
