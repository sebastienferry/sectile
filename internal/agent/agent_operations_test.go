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
// locks it too, so it never returned. It backs the board's synchronous branch
// checkout, so it now answers as soon as the worktree exists and installs behind
// the answer. A launch that follows waits for that install instead of running a
// second one, and the main checkout's packages are left alone.
func TestPrepareWorkspaceAnswersBeforeTheInstallAndALaunchWaitsForIt(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	testhome.Temp(t)
	fake := useFakeInstall(t)
	fake.gate = make(chan struct{})
	fake.started = make(chan string, 4)
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
	worktree := filepath.Join(root, ".tasks", "worktrees", task.Key)
	web := filepath.Join(worktree, "web")

	// The operation answers while its install is still blocked.
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
		close(fake.gate)
		t.Fatal("prepare_workspace waited for the install")
	}
	if got.err != nil {
		close(fake.gate)
		t.Fatal(got.err)
	}
	info := got.value.(models.WorktreeInfo)
	if info.WorktreePath != worktree || info.Branch != branch {
		close(fake.gate)
		t.Fatalf("workspace: %+v", info)
	}
	select {
	case dir := <-fake.started:
		if dir != web {
			close(fake.gate)
			t.Fatalf("installed in %s, want %s", dir, web)
		}
	case <-time.After(30 * time.Second):
		close(fake.gate)
		t.Fatal("prepare_workspace never started the install")
	}
	if !daemon.prepareMu.TryLock() {
		close(fake.gate)
		t.Fatal("prepareMu is still held after the operation")
	}
	daemon.prepareMu.Unlock()

	// A launch of the same task blocks behind the running install.
	launched := make(chan error, 1)
	go func() {
		_, _, _, _, err := daemon.prepareDispatch(ctx, task.Key, true)
		launched <- err
	}()
	select {
	case err := <-launched:
		close(fake.gate)
		t.Fatalf("the launch did not wait for the install: %v", err)
	case <-time.After(500 * time.Millisecond):
	}
	close(fake.gate)
	select {
	case err := <-launched:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the launch never returned")
	}
	if ran := fake.ran(); len(ran) != 1 || ran[0] != web {
		t.Fatalf("installed in %v, want the worktree's web folder once", ran)
	}
	if !hasStamp(web) {
		t.Fatal("the worktree's web folder was not stamped")
	}
	if _, err := os.Lstat(filepath.Join(root, "web", "node_modules")); !os.IsNotExist(err) {
		t.Fatalf("the main checkout was provisioned: %v", err)
	}
}

// The launch path keeps waiting for the install, so the session starts with its
// dependencies in place.
func TestLaunchPreparationWaitsForTheInstall(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	testhome.Temp(t)
	fake := useFakeInstall(t)
	fake.gate = make(chan struct{})
	fake.started = make(chan string, 4)
	npmPackage(t, filepath.Join(root, "web"))
	for _, args := range [][]string{{"init", "-b", "main"}, {"add", "web"}, {"-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "Initial"}} {
		if _, err := gitLocal(ctx, root, args...); err != nil {
			t.Fatal(err)
		}
	}
	project := "project"
	branch := "feat/launched"
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
	web := filepath.Join(root, ".tasks", "worktrees", task.Key, "web")

	launched := make(chan error, 1)
	go func() {
		_, _, _, _, err := daemon.prepareDispatch(ctx, task.Key, true)
		launched <- err
	}()
	select {
	case <-fake.started:
	case err := <-launched:
		t.Fatalf("the launch returned before installing: %v", err)
	case <-time.After(30 * time.Second):
		t.Fatal("the launch never started the install")
	}
	select {
	case err := <-launched:
		close(fake.gate)
		t.Fatalf("the launch did not wait for the install: %v", err)
	case <-time.After(500 * time.Millisecond):
	}
	close(fake.gate)
	select {
	case err := <-launched:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the launch never returned")
	}
	if !hasStamp(web) {
		t.Fatal("the launch returned before the worktree was stamped")
	}
}
