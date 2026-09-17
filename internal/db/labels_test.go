package db

import (
	"testing"

	"tasks/internal/models"
)

// Poser une étape doit retirer toutes les autres côté tracker : Jira ne fait pas
// le remplacement tout seul, et un ticket accumulait clarified, specified,
// implemented… alors que Taskacao n'en montrait qu'un.
func TestStaleWorkflowLabelsExcludesTargetOnly(t *testing.T) {
	stale := StaleWorkflowLabels("implemented")

	for _, label := range stale {
		if label == "implemented" || label == "Implemented" {
			t.Fatalf("le label visé ne doit pas être retiré, trouvé %q", label)
		}
	}

	// Les deux graphies des autres étapes doivent être visées, Jira distinguant
	// la casse.
	for _, want := range []string{"clarified", "Clarified", "reviewed", "Reviewed", "new", "New"} {
		found := false
		for _, label := range stale {
			if label == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%q attendu dans les labels à retirer", want)
		}
	}
}

// SetWorkflowLabel, lui, remplace bien en local.
func TestSetWorkflowLabelReplacesLocally(t *testing.T) {
	out := SetWorkflowLabel([]string{"ai:tech:autonomous", "clarified"}, "specified")
	if len(out) != 2 || out[0] != "ai:tech:autonomous" || out[1] != "#specified" {
		t.Fatalf("remplacement attendu, obtenu %v", out)
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func createTestProject(t *testing.T, database *DB) *models.Project {
	t.Helper()
	proj, err := database.CreateProject(models.CreateProjectRequest{
		Name:         "Test Project",
		Slug:         "test-proj",
		IssueTracker: "local",
		RepoPath:     t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}
	return proj
}

func TestCreateTaskLowercaseDefaultWorkflowLabel(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	proj := createTestProject(t, database)

	// Scenario 1: Create without supplied labels
	t.Run("Create without supplied labels", func(t *testing.T) {
		created, err := database.CreateTask(models.CreateTaskRequest{
			ProjectID: proj.ID,
			Title:     "Task without labels",
		})
		if err != nil {
			t.Fatalf("CreateTask failed: %v", err)
		}
		expectedLabels := []string{"#new"}
		if !equalStringSlices(created.Labels, expectedLabels) {
			t.Errorf("Returned labels = %v, want %v", created.Labels, expectedLabels)
		}
		if created.Status != models.StatusToClarify {
			t.Errorf("Returned status = %v, want %v", created.Status, models.StatusToClarify)
		}

		reloaded, err := database.GetTaskByID(created.ID)
		if err != nil {
			t.Fatalf("GetTaskByID failed: %v", err)
		}
		if !equalStringSlices(reloaded.Labels, expectedLabels) {
			t.Errorf("Reloaded labels = %v, want %v", reloaded.Labels, expectedLabels)
		}
		if reloaded.Status != models.StatusToClarify {
			t.Errorf("Reloaded status = %v, want %v", reloaded.Status, models.StatusToClarify)
		}
	})

	// Scenario 2: Replace legacy workflow variants and retain custom labels
	t.Run("Replace legacy workflow variants and retain custom labels", func(t *testing.T) {
		created, err := database.CreateTask(models.CreateTaskRequest{
			ProjectID: proj.ID,
			Title:     "Task with legacy labels",
			Labels:    []string{"New", "#New", "#Specified", "CustomerCase", "#TeamTag"},
		})
		if err != nil {
			t.Fatalf("CreateTask failed: %v", err)
		}
		expectedLabels := []string{"CustomerCase", "#TeamTag", "#new"}
		if !equalStringSlices(created.Labels, expectedLabels) {
			t.Errorf("Returned labels = %v, want %v", created.Labels, expectedLabels)
		}

		reloaded, err := database.GetTaskByID(created.ID)
		if err != nil {
			t.Fatalf("GetTaskByID failed: %v", err)
		}
		if !equalStringSlices(reloaded.Labels, expectedLabels) {
			t.Errorf("Reloaded labels = %v, want %v", reloaded.Labels, expectedLabels)
		}
	})

	// Scenario 5 (Creation): Default and explicit status
	t.Run("Create with explicit status", func(t *testing.T) {
		created, err := database.CreateTask(models.CreateTaskRequest{
			ProjectID: proj.ID,
			Title:     "Task with explicit status",
			Status:    models.StatusToImplement,
		})
		if err != nil {
			t.Fatalf("CreateTask failed: %v", err)
		}
		if created.Status != models.StatusToImplement {
			t.Errorf("Returned status = %v, want %v", created.Status, models.StatusToImplement)
		}
		expectedLabels := []string{"#new"}
		if !equalStringSlices(created.Labels, expectedLabels) {
			t.Errorf("Returned labels = %v, want %v", created.Labels, expectedLabels)
		}

		reloaded, err := database.GetTaskByID(created.ID)
		if err != nil {
			t.Fatalf("GetTaskByID failed: %v", err)
		}
		if reloaded.Status != models.StatusToImplement {
			t.Errorf("Reloaded status = %v, want %v", reloaded.Status, models.StatusToImplement)
		}
		if !equalStringSlices(reloaded.Labels, expectedLabels) {
			t.Errorf("Reloaded labels = %v, want %v", reloaded.Labels, expectedLabels)
		}
	})
}

func TestCloneTaskLowercaseDefaultWorkflowLabel(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	proj := createTestProject(t, database)

	// Create source task and transition it to specified
	sourceTask, err := database.CreateTask(models.CreateTaskRequest{
		ProjectID: proj.ID,
		Title:     "Original Task",
		Labels:    []string{"CustomerCase", "#TeamTag"},
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	updatedSource, _, err := database.TransitionTaskStage(sourceTask.ID, "specified", "Spec completed", "", "")
	if err != nil {
		t.Fatalf("TransitionTaskStage failed: %v", err)
	}
	expectedSourceLabels := []string{"CustomerCase", "#TeamTag", "#specified"}
	if !equalStringSlices(updatedSource.Labels, expectedSourceLabels) {
		t.Fatalf("Source labels before clone = %v, want %v", updatedSource.Labels, expectedSourceLabels)
	}

	// Scenario 3: Clone with source labels included (default and explicit true)
	t.Run("Clone with source labels included", func(t *testing.T) {
		incLabels := true
		cloned, err := database.CloneTask(sourceTask.ID, models.CloneTaskRequest{
			Title:         "Cloned Task with labels",
			IncludeLabels: &incLabels,
		})
		if err != nil {
			t.Fatalf("CloneTask failed: %v", err)
		}

		expectedCloneLabels := []string{"CustomerCase", "#TeamTag", "#new"}
		if !equalStringSlices(cloned.Labels, expectedCloneLabels) {
			t.Errorf("Cloned returned labels = %v, want %v", cloned.Labels, expectedCloneLabels)
		}
		if cloned.Status != models.StatusToClarify {
			t.Errorf("Cloned returned status = %v, want %v", cloned.Status, models.StatusToClarify)
		}

		reloadedClone, err := database.GetTaskByID(cloned.ID)
		if err != nil {
			t.Fatalf("GetTaskByID failed: %v", err)
		}
		if !equalStringSlices(reloadedClone.Labels, expectedCloneLabels) {
			t.Errorf("Cloned reloaded labels = %v, want %v", reloadedClone.Labels, expectedCloneLabels)
		}

		// Verify source task remains unchanged
		reloadedSource, err := database.GetTaskByID(sourceTask.ID)
		if err != nil {
			t.Fatalf("GetTaskByID source failed: %v", err)
		}
		if !equalStringSlices(reloadedSource.Labels, expectedSourceLabels) {
			t.Errorf("Source task labels changed! Got %v, want %v", reloadedSource.Labels, expectedSourceLabels)
		}
		if reloadedSource.Status != models.StatusToImplement {
			t.Errorf("Source task status changed! Got %v, want %v", reloadedSource.Status, models.StatusToImplement)
		}
	})

	// Scenario 4: Clone without source labels
	t.Run("Clone without source labels", func(t *testing.T) {
		incLabels := false
		cloned, err := database.CloneTask(sourceTask.ID, models.CloneTaskRequest{
			Title:         "Cloned Task without labels",
			IncludeLabels: &incLabels,
		})
		if err != nil {
			t.Fatalf("CloneTask failed: %v", err)
		}

		expectedCloneLabels := []string{"#new"}
		if !equalStringSlices(cloned.Labels, expectedCloneLabels) {
			t.Errorf("Cloned returned labels = %v, want %v", cloned.Labels, expectedCloneLabels)
		}

		reloadedClone, err := database.GetTaskByID(cloned.ID)
		if err != nil {
			t.Fatalf("GetTaskByID failed: %v", err)
		}
		if !equalStringSlices(reloadedClone.Labels, expectedCloneLabels) {
			t.Errorf("Cloned reloaded labels = %v, want %v", reloadedClone.Labels, expectedCloneLabels)
		}

		// Verify source task remains unchanged
		reloadedSource, err := database.GetTaskByID(sourceTask.ID)
		if err != nil {
			t.Fatalf("GetTaskByID source failed: %v", err)
		}
		if !equalStringSlices(reloadedSource.Labels, expectedSourceLabels) {
			t.Errorf("Source task labels changed! Got %v, want %v", reloadedSource.Labels, expectedSourceLabels)
		}
	})

	// Scenario 5 (Clone): Clone with explicit status
	t.Run("Clone with explicit status", func(t *testing.T) {
		incLabels := true
		cloned, err := database.CloneTask(sourceTask.ID, models.CloneTaskRequest{
			Title:         "Cloned Task with explicit status",
			Status:        models.StatusClarified,
			IncludeLabels: &incLabels,
		})
		if err != nil {
			t.Fatalf("CloneTask failed: %v", err)
		}

		if cloned.Status != models.StatusClarified {
			t.Errorf("Cloned status = %v, want %v", cloned.Status, models.StatusClarified)
		}
		expectedCloneLabels := []string{"CustomerCase", "#TeamTag", "#new"}
		if !equalStringSlices(cloned.Labels, expectedCloneLabels) {
			t.Errorf("Cloned labels = %v, want %v", cloned.Labels, expectedCloneLabels)
		}
	})
}

// Scenario 6: Read an existing task with legacy label
func TestReadHistoricalTaskRetainsLegacyLabel(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	proj := createTestProject(t, database)

	// Insert task with legacy "New" label directly into the database
	taskID := "legacy-task-1"
	_, err := database.conn.Exec(`
		INSERT INTO tasks (id, project_id, key, title, status, priority, labels, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, taskID, proj.ID, "#999", "Historical Task", models.StatusToClarify, models.PriorityMedium, `["New", "CustomerCase"]`)
	if err != nil {
		t.Fatalf("Failed to insert historical task: %v", err)
	}

	task, err := database.GetTaskByID(taskID)
	if err != nil {
		t.Fatalf("GetTaskByID failed: %v", err)
	}

	expectedLabels := []string{"New", "CustomerCase"}
	if !equalStringSlices(task.Labels, expectedLabels) {
		t.Errorf("Historical task labels = %v, want %v", task.Labels, expectedLabels)
	}
}

// La graphie du label d'étape est décidée par SetWorkflowLabel, pas par
// l'appelant : c'est ce qui empêche un futur site d'appel de réintroduire la
// forme nue « new » à côté de « #new ».
func TestSetWorkflowLabelNormalisesSpelling(t *testing.T) {
	cases := []struct {
		name   string
		target string
	}{
		{"forme nue", "new"},
		{"forme préfixée", "#new"},
		{"casse mixte", "#New"},
		{"casse mixte sans préfixe", "New"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := SetWorkflowLabel([]string{"CustomerCase", "#TeamTag"}, tc.target)
			want := []string{"CustomerCase", "#TeamTag", "#new"}
			if !equalStringSlices(out, want) {
				t.Errorf("SetWorkflowLabel(_, %q) = %v, want %v", tc.target, out, want)
			}
		})
	}
}

// Les tâches historiques portent encore le label nu. Elles doivent continuer à
// résoudre leur étape, et la prochaine transition doit les normaliser.
func TestLegacyBareWorkflowLabelStillResolvesAndIsCleaned(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	proj := createTestProject(t, database)

	taskID := "legacy-bare-label"
	_, err := database.conn.Exec(`
		INSERT INTO tasks (id, project_id, key, title, status, priority, labels, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, taskID, proj.ID, "#998", "Legacy bare label", models.StatusToClarify, models.PriorityMedium, `["new", "CustomerCase"]`)
	if err != nil {
		t.Fatalf("insertion de la tâche héritée impossible: %v", err)
	}

	task, err := database.GetTaskByID(taskID)
	if err != nil {
		t.Fatalf("GetTaskByID failed: %v", err)
	}
	if stage := database.StageOfTask(task); stage != "new" {
		t.Fatalf("StageOfTask = %q, want %q", stage, "new")
	}

	updated, _, err := database.TransitionTaskStage(taskID, "clarified", "Clarified", "", "")
	if err != nil {
		t.Fatalf("TransitionTaskStage failed: %v", err)
	}
	want := []string{"CustomerCase", "#clarified"}
	if !equalStringSlices(updated.Labels, want) {
		t.Errorf("labels après transition = %v, want %v", updated.Labels, want)
	}
}
