package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"tasks/internal/runner"
	"tasks/internal/trackerapi"
)

// configurationTask is SFE-367 of #584: a task at specified on a project that
// opens its pull request at implemented and whose checkout has no origin
// remote, so every pull request lookup fails with the explicit #392 error.
func configurationTask(t *testing.T) (*DB, *models.Task, *atomic.Int64) {
	t.Helper()
	d := testDB(t)
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Coordination", IssueTracker: "local", PRCreationStage: "implemented"})
	if err != nil {
		t.Fatal(err)
	}
	setLegacyProject(t, d, p.ID, map[string]any{"repo_path": "/not-mounted-on-server"})
	task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: p.ID, Title: "Fix the webhook URL", Labels: []string{"#specified"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.conn.Exec("UPDATE tasks SET branch_name='feat/sfe-367' WHERE id=?", task.ID); err != nil {
		t.Fatal(err)
	}
	d.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		return nil, fmt.Errorf("unexpected local operation %q", op.Action)
	})
	lookups := new(atomic.Int64)
	d.prEvidenceLookup = func(string, string, string) (trackerapi.PullRequest, error) {
		lookups.Add(1)
		return trackerapi.PullRequest{}, fmt.Errorf("pull request lookup failed on the local agent: local agent: %w", runner.ErrNoOriginRemote)
	}
	task, err = d.GetTaskByID(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	return d, task, lookups
}

// A task that changed no repository reaches implemented, then reviewed, on
// the statement alone, and its report says that no pull request was expected.
func TestNoRepositoryChangeStandsInForThePullRequest(t *testing.T) {
	d, task, lookups := configurationTask(t)

	if _, _, err := d.TransitionTaskStage(task.ID, "implemented", "webhook fixed through the API", "", "feat/sfe-367"); err == nil ||
		!strings.Contains(err.Error(), runner.ErrNoOriginRemote.Error()) {
		t.Fatalf("without the statement the transition must keep the explicit #392 error, got %v", err)
	}

	if lookups.Load() != 1 {
		t.Errorf("pull request lookups = %d, want only the one of the call without the statement", lookups.Load())
	}
	set, err := d.noRepositoryChangeEvidence(task, "implement", "feat/sfe-367")
	if err != nil || set.notice != noRepositoryChangeNotice || len(set.urls) != 0 {
		t.Fatalf("the report must say no pull request was expected: %+v %v", set, err)
	}

	for _, stage := range []string{"implemented", "reviewed"} {
		got, _, err := d.TransitionTaskStageWithoutRepositoryChange("", task.ID, stage, "webhook fixed through the API, delivery checked", "feat/sfe-367")
		if err != nil {
			t.Fatalf("%s with the statement: %v", stage, err)
		}
		if d.StageOfTask(got) != stage || len(got.PrLinks) != 0 {
			t.Fatalf("%s: stage %q, links %v; want the stage and no pull request", stage, d.StageOfTask(got), got.PrLinks)
		}
	}

}

// The statement is refused when the task shows that it changed a repository.
func TestNoRepositoryChangeIsRefusedForATaskWithCode(t *testing.T) {
	t.Run("pull request on its branch", func(t *testing.T) {
		d, task, _ := configurationTask(t)
		links, _ := json.Marshal([]models.TaskPullRequest{{URL: "https://github.com/o/r/pull/9", Branch: "feat/sfe-367"}})
		if _, err := d.conn.Exec("UPDATE tasks SET pr_links=? WHERE id=?", string(links), task.ID); err != nil {
			t.Fatal(err)
		}
		_, _, err := d.TransitionTaskStageWithoutRepositoryChange("", task.ID, "implemented", "done", "feat/sfe-367")
		if err == nil || !strings.Contains(err.Error(), "pull/9") {
			t.Fatalf("a task with a pull request on its branch must be refused, got %v", err)
		}
	})
	t.Run("pull request on another branch", func(t *testing.T) {
		d, task, _ := configurationTask(t)
		links, _ := json.Marshal([]models.TaskPullRequest{{URL: "https://github.com/o/r/pull/3", Branch: "feat/earlier"}})
		if _, err := d.conn.Exec("UPDATE tasks SET pr_links=? WHERE id=?", string(links), task.ID); err != nil {
			t.Fatal(err)
		}
		if _, _, err := d.TransitionTaskStageWithoutRepositoryChange("", task.ID, "implemented", "done", "feat/sfe-367"); err != nil {
			t.Fatalf("a pull request of another branch says nothing about this one: %v", err)
		}
	})
	t.Run("changed repository", func(t *testing.T) {
		d, task, _ := configurationTask(t)
		if _, err := d.conn.Exec(`UPDATE tasks SET changed_repositories='["gitlab.com/group/infra"]' WHERE id=?`, task.ID); err != nil {
			t.Fatal(err)
		}
		_, _, err := d.TransitionTaskStageWithoutRepositoryChange("", task.ID, "implemented", "done", "feat/sfe-367")
		if err == nil || !strings.Contains(err.Error(), "gitlab.com/group/infra") {
			t.Fatalf("a task with a changed repository must be refused, got %v", err)
		}
	})
}
