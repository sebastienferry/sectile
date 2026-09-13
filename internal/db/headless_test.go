package db

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"testing"
	"time"
)

func TestHeadlessTrackerReadsWithoutAgentOrCLI(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer server-secret" {
			t.Error("server credential missing")
		}
		if r.URL.Path != "/repos/acme/app/issues/1" {
			t.Errorf("unexpected tracker route: %s", r.URL)
		}
		fmt.Fprint(w, `{"number":1,"title":"Remote issue","state":"open","labels":[{"name":"#specified"}]}`)
	}))
	defer srv.Close()
	t.Setenv("TASKFLOW_GITHUB_API_URL", srv.URL)
	t.Setenv("TASKFLOW_GITHUB_TOKEN", "server-secret")
	d, err := NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Headless", IssueTracker: "github", GithubRepo: "acme/app", RepoPath: "/no/server/checkout"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := d.CreateTask(models.CreateTaskRequest{Title: "Local seed", Source: "local", ProjectID: p.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.conn.Exec("UPDATE tasks SET source='github',key='#1' WHERE id=?", task.ID); err != nil {
		t.Fatal(err)
	}
	updated, err := d.SyncSingleTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Remote issue" || d.StageOfTask(updated) != "specified" {
		t.Fatalf("remote result not persisted: %#v", updated)
	}
	if _, err := d.GetGitStatus(p.ID); err == nil {
		t.Fatal("headless Git unexpectedly available")
	}
}

func TestWorkerLaunchDoesNotLockOrAdvanceWorkflow(t *testing.T) {
	d, err := NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	task, err := d.CreateTask(models.CreateTaskRequest{Title: "Queued skill", Source: "local"})
	if err != nil {
		t.Fatal(err)
	}
	activity := models.TaskActivity{ID: "launch", TaskID: task.ID, ProjectID: task.ProjectID, SkillID: "clarify", Status: "queued", CreatedAt: time.Now()}
	if err := d.AddTaskActivity(activity); err != nil {
		t.Fatal(err)
	}
	runID := ""
	d.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		if op.Action != "execute_skill" || op.RunID == "" {
			t.Fatalf("launch contract: %#v", op)
		}
		runID = op.RunID
		fresh, _ := d.GetTaskByID(task.ID)
		if d.StageOfTask(fresh) != "new" {
			t.Fatal("dispatch advanced stage")
		}
		// A fast native client can report before the launch acknowledgement returns.
		if _, _, err := d.TransitionTaskStage(task.ID, "clarified", "Verified native report", "", ""); err != nil {
			t.Fatalf("launch blocked native transition: %v", err)
		}
		return json.RawMessage(`null`), nil
	})
	d.processSkillJob(SkillJob{ActivityID: activity.ID, TaskID: task.ID, ProjectID: task.ProjectID, SkillID: "clarify"})
	got, err := d.GetActivityByID(activity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SkillID != "agent_launch" || got.Status != "completed" {
		t.Fatalf("launch result: %#v", got)
	}
	run, err := d.GetActivityByID(runID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "running" {
		t.Fatalf("launch ended skill execution: %#v", run)
	}
}
