package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"tasks/internal/testhome"
	"testing"
	"time"
)

func TestWorkspaceOperationUsesLocalMappingAndAssignedCheckout(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, args := range [][]string{{"init", "-b", "feat/assigned"}, {"-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "Initial"}} {
		if _, err := gitLocal(ctx, root, args...); err != nil {
			t.Fatal(err)
		}
	}
	project := "project"
	branch := "feat/assigned"
	forbidden := "/server/must-not-be-used"
	task := models.Task{ID: "task", Key: "#54", ProjectID: project, BranchName: &branch, WorktreePath: &forbidden, RepoPath: &forbidden}
	config := agentconfig.Config{SchemaVersion: 1, ProjectID: project, UseWorktrees: true, AIProvider: "claude"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Error("credential missing")
		}
		if r.URL.Path == "/api/v1/agent/config" {
			_ = json.NewEncoder(w).Encode(config)
			return
		}
		_ = json.NewEncoder(w).Encode(task)
	}))
	defer srv.Close()
	daemon := &agentDaemon{repoRoot: root, link: serverLink{serverURL: srv.URL, token: "token", projectID: project}}
	op := agentprotocol.Operation{ProjectID: project, TaskID: task.ID, Action: "git_evidence"}
	value, err := daemon.executeOperation(ctx, op)
	if err != nil {
		t.Fatal(err)
	}
	evidence := value.(map[string]any)
	if evidence["branch"] != branch || evidence["clean"] != true || evidence["sha"] == "" {
		t.Fatalf("evidence: %#v", evidence)
	}
	if _, err := os.Stat(filepath.Join(root, ".tasks", "worktrees", task.Key)); !os.IsNotExist(err) {
		t.Fatal("read created another checkout")
	}
	task.ProjectID = "other"
	if _, err := daemon.executeOperation(ctx, op); err == nil {
		t.Fatal("foreign task accepted")
	}
	op.Action = "arbitrary_shell"
	if _, err := daemon.executeOperation(ctx, op); err == nil {
		t.Fatal("unknown capability accepted")
	}
}

// prepare_workspace used to lock prepareMu before calling prepareDispatch, which
// locks it too, so it never returned. It now answers, provisions the worktree it
// created, and leaves the main checkout's packages alone.
func TestPrepareWorkspaceProvisionsTheCreatedWorktree(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	testhome.Temp(t)
	fake := useFakeInstall(t)
	npmPackage(t, filepath.Join(root, "web"))
	for _, args := range [][]string{{"init", "-b", "main"}, {"add", "web"}, {"-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "Initial"}} {
		if _, err := gitLocal(ctx, root, args...); err != nil {
			t.Fatal(err)
		}
	}
	project := "project"
	branch := "feat/provisioned"
	task := models.Task{ID: "task", Key: "#232", ProjectID: project, BranchName: &branch}
	config := agentconfig.Config{SchemaVersion: 1, ProjectID: project, UseWorktrees: true, AIProvider: "claude"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/agent/config" {
			_ = json.NewEncoder(w).Encode(config)
			return
		}
		_ = json.NewEncoder(w).Encode(task)
	}))
	defer srv.Close()
	daemon := &agentDaemon{repoRoot: root, loopback: loopbackServer{url: "http://127.0.0.1:8091"}, link: serverLink{serverURL: srv.URL, token: "token", projectID: project}}

	type answer struct {
		value any
		err   error
	}
	done := make(chan answer, 1)
	go func() {
		value, err := daemon.executeOperation(ctx, agentprotocol.Operation{ProjectID: project, TaskID: task.ID, Action: "prepare_workspace"})
		done <- answer{value, err}
	}()
	var got answer
	select {
	case got = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("prepare_workspace never returned")
	}
	if got.err != nil {
		t.Fatal(got.err)
	}
	info := got.value.(models.WorktreeInfo)
	worktree := filepath.Join(root, ".tasks", "worktrees", task.Key)
	if info.WorktreePath != worktree || info.Branch != branch {
		t.Fatalf("workspace: %+v", info)
	}
	if ran := fake.ran(); len(ran) != 1 || ran[0] != filepath.Join(worktree, "web") {
		t.Fatalf("installed in %v, want the worktree's web folder only", ran)
	}
	if !hasStamp(filepath.Join(worktree, "web")) {
		t.Fatal("the worktree's web folder was not stamped")
	}
	if _, err := os.Lstat(filepath.Join(root, "web", "node_modules")); !os.IsNotExist(err) {
		t.Fatalf("the main checkout was provisioned: %v", err)
	}
	if !daemon.prepareMu.TryLock() {
		t.Fatal("prepareMu is still held after the operation")
	}
	daemon.prepareMu.Unlock()
}
