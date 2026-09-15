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
	t.Setenv("SECTILE_GITHUB_API_URL", srv.URL)
	t.Setenv("SECTILE_GITHUB_TOKEN", "server-secret")
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

func TestTrackerFailureDoesNotCreatePhantomTaskOrCompleteSync(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	defer srv.Close()
	t.Setenv("SECTILE_GITHUB_API_URL", srv.URL)
	t.Setenv("SECTILE_GITHUB_TOKEN", "invalid")
	d, err := NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "API failure", IssueTracker: "github", GithubRepo: "acme/app"})
	if err != nil {
		t.Fatal(err)
	}
	if task, err := d.CreateTask(models.CreateTaskRequest{Title: "Must be remote", ProjectID: p.ID}); err == nil || task != nil {
		t.Fatalf("phantom task: %#v %v", task, err)
	}
	var count int
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM tasks WHERE project_id=?", p.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("phantom persisted: %d %v", count, err)
	}
	activity := models.TaskActivity{ID: "sync-failure", TaskID: "sync-all", SkillID: "sync_all", Status: "running", CreatedAt: time.Now()}
	if err := d.AddTaskActivity(activity); err != nil {
		t.Fatal(err)
	}
	settings, err := d.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	d.processSyncJob(context.Background(), SkillJob{SkillID: "sync_all", ActivityID: activity.ID}, settings)
	result, err := d.GetActivityByID(activity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "failed" {
		t.Fatalf("failed tracker sync reported success: %#v", result)
	}
}

func TestSkillEditorUsesAgentEvidenceAndFrameworkOverride(t *testing.T) {
	d, err := NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Skills", SpecFramework: "speckit"})
	if err != nil {
		t.Fatal(err)
	}
	config, err := d.AgentConfig(p.ID, "", "openspec")
	if err != nil {
		t.Fatal(err)
	}
	if config.SpecFramework != "openspec" {
		t.Fatalf("framework not honored: %#v", config)
	}
	if _, err := d.AgentConfig(p.ID, "", "unknown"); err == nil {
		t.Fatal("unknown framework accepted")
	}
	d.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		if op.Action != "skill_files" || op.ProjectID != p.ID {
			t.Fatalf("snapshot request: %#v", op)
		}
		return json.Marshal(map[string]agentprotocol.SkillFile{"clarify": {Content: "personal instructions", Paths: []string{"/agent/repository/clarify/SKILL.md"}}})
	})
	entries, err := d.ListProjectSkillEditor(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.ID == "clarify" {
			if !entry.Installed || !entry.Diverged || entry.RepoContent != "personal instructions" {
				t.Fatalf("agent divergence lost: %#v", entry)
			}
			return
		}
	}
	t.Fatal("clarify entry missing")
}

func TestServerRestartPreservesRemoteExecutionOwnership(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	task, err := d.CreateTask(models.CreateTaskRequest{Title: "Survive server restart", Source: "local"})
	if err != nil {
		t.Fatal(err)
	}
	remote, err := d.StartRemoteRun(task.ID, "pickup", "")
	if err != nil {
		t.Fatal(err)
	}
	agent, err := d.StartAgentRemoteRun(task.ID, "clarify")
	if err != nil {
		t.Fatal(err)
	}
	if err := d.AddTaskActivity(models.TaskActivity{ID: "interrupted-server-job", TaskID: task.ID, SkillID: "tracker_update", Status: "running", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	for _, id := range []string{remote.ID, agent.ID} {
		activity, err := restarted.GetActivityByID(id)
		if err != nil {
			t.Fatal(err)
		}
		if activity.Status != "running" {
			t.Fatalf("restart finished external execution: %#v", activity)
		}
		if _, err := restarted.FinishRemoteRun(task.ID, id, "completed", "External execution finished after restart"); err != nil {
			t.Fatal(err)
		}
	}
	local, err := restarted.GetActivityByID("interrupted-server-job")
	if err != nil {
		t.Fatal(err)
	}
	if local.Status != "failed" {
		t.Fatalf("server job falsely survived process loss: %#v", local)
	}
}

func TestSyncRemoteRunStatus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	project, err := d.CreateProject(models.CreateProjectRequest{Name: "Sync", IssueTracker: "local"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "Task", Priority: models.PriorityMedium})
	if err != nil {
		t.Fatal(err)
	}

	notifications := make(chan *models.TaskActivity, 10)
	d.RegisterPostBackListener(func(task *models.Task, act *models.TaskActivity, err error) {
		if act != nil {
			notifications <- act
		}
	})

	// 1. Sync a new queued task run
	actQueued, err := d.SyncRemoteRunStatus("run-1", task.ID, project.ID, task.Key, "clarify", "queued", "In local queue", nil)
	if err != nil {
		t.Fatal(err)
	}
	if actQueued.Status != "queued" || actQueued.SkillID != "remote_run" {
		t.Fatalf("expected queued status, got: %#v", actQueued)
	}
	if actQueued.StartedAt != nil {
		t.Fatalf("expected nil startedAt for queued, got: %v", actQueued.StartedAt)
	}

	// 2. Transition to running
	now := time.Now()
	actRunning, err := d.SyncRemoteRunStatus("run-1", task.ID, project.ID, task.Key, "clarify", "running", "Running now", &now)
	if err != nil {
		t.Fatal(err)
	}
	if actRunning.Status != "running" || actRunning.StartedAt == nil {
		t.Fatalf("expected running status with startedAt, got: %#v", actRunning)
	}

	// 3. Mark an activity as failed (simulating restart failure or timeout), then recover via sync
	if _, err := d.conn.Exec("UPDATE task_activities SET status='failed', error='timeout' WHERE id='run-1'"); err != nil {
		t.Fatal(err)
	}
	actRecovered, err := d.SyncRemoteRunStatus("run-1", task.ID, project.ID, task.Key, "clarify", "running", "Recovered running", &now)
	if err != nil {
		t.Fatal(err)
	}
	if actRecovered.Status != "running" || actRecovered.Error != "" {
		t.Fatalf("expected error cleared and status running, got: %#v", actRecovered)
	}
	select {
	case notified := <-notifications:
		if notified.ID != "run-1" {
			t.Fatalf("expected notification for run-1, got: %s", notified.ID)
		}
	case <-time.After(time.Second):
		t.Fatalf("expected postback listener notification")
	}
}

