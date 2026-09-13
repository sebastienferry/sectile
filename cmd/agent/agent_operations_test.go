package main

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
	"testing"
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
	daemon := &agentDaemon{serverURL: srv.URL, token: "token", repoRoot: root, projectID: project}
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
