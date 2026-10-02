package taskmcp

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/db"
	"tasks/internal/models"
	"tasks/internal/testsqlite"
)

func TestRecordPullRequestNamesTheRepositoryOfEachLink(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "tasks.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "App", IssueTracker: "local", GitRemoteUrl: "git@github.com:o/app.git",
		Repositories: []string{"git@gitlab.com:g/deploy.git"}})
	if err != nil {
		t.Fatal(err)
	}
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "two repositories"})
	if err != nil {
		t.Fatal(err)
	}
	for _, url := range []string{"https://github.com/o/app/pull/1", "https://gitlab.com/g/deploy/-/merge_requests/2"} {
		if _, err := call(t, database, "record_pull_request", map[string]any{"taskKey": task.ID, "url": url}); err != nil {
			t.Fatalf("%s: %v", url, err)
		}
	}
	out, err := call(t, database, "record_pull_request", map[string]any{"taskKey": task.ID, "url": "https://github.com/o/app/pull/1"})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(out["prLinks"])
	var links []models.TaskPullRequest
	if err := json.Unmarshal(raw, &links); err != nil || len(links) != 2 {
		t.Fatalf("prLinks = %s %v", raw, err)
	}
	if links[0].Repository != "gitlab.com/g/deploy" || links[1].Repository != "github.com/o/app" {
		t.Fatalf("the primary repository's pull request must stay last, each naming its repository: %s", raw)
	}
	if _, err := call(t, database, "record_pull_request", map[string]any{"taskKey": task.ID, "url": "https://github.com/x/y/pull/3"}); err == nil || !strings.Contains(err.Error(), "not one of the task's repositories") {
		t.Fatalf("err = %v", err)
	}
	if _, err := callAs(t, database, nil, "record_pull_request", map[string]any{"taskKey": task.ID, "url": "https://github.com/o/app/pull/4"}); err == nil {
		t.Fatal("an anonymous caller recorded a pull request")
	}
}
