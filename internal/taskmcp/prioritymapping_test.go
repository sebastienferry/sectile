package taskmcp

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/db"
	"tasks/internal/models"
	"tasks/internal/testsqlite"
	"tasks/internal/tracker"
)

// guessingTracker lists a scheme whose every option the mapping guesses.
type guessingTracker struct {
	tracker.BaseTicketingSystem
}

func (guessingTracker) PriorityScheme(ctx context.Context, project *models.Project, fresh bool) ([]models.PriorityOption, error) {
	return []models.PriorityOption{{ID: "1", Name: "P1"}, {ID: "2", Name: "P2"}, {ID: "3", Name: "P3"}, {ID: "4", Name: "P4"}}, nil
}

func (guessingTracker) ClassifyPriority(name string, rank, n int) (models.Priority, bool) {
	return models.PriorityLevels[rank], false
}

// US3.3: an agent's update_task carrying a priority the project's mapping only
// guessed is refused with the sentence, and writes nothing (#679).
func TestUpdateTaskRefusesAGuessedPriority(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "tasks.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.TrackerRegistry().Register("jira", &guessingTracker{tracker.BaseTicketingSystem{TrackerName: "jira"}})
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Ops", IssueTracker: "jira", JiraProject: "OPS"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "Guarded", Priority: models.PriorityMedium, Source: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.RefreshPriorityMapping(context.Background(), project.ID, false); err != nil {
		t.Fatal(err)
	}

	_, err = call(t, database, "update_task", map[string]any{"taskKey": task.ID, "title": "Renamed", "priority": "high"})
	if err == nil || !strings.Contains(err.Error(), `Priority "high" is not mapped with certainty`) {
		t.Fatalf("err = %v", err)
	}
	reread, _ := database.GetTaskByID(task.ID)
	if reread.Title != "Guarded" || reread.Priority != models.PriorityMedium {
		t.Fatalf("the refused update wrote the task: %+v", reread)
	}
}
