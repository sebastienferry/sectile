package db

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"tasks/internal/agentprotocol"
	"tasks/internal/models"
)

func TestArchiveTaskWorkspaceDecidesTheBranchFromThePullRequests(t *testing.T) {
	d, task, _ := twoRepoTask(t)
	var asked []agentprotocol.Operation
	d.SetAgentOperations(func(_ context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		asked = append(asked, op)
		return json.Marshal(models.WorkspaceArchive{Repositories: []models.WorkspaceArchiveEntry{{Repository: "gitlab.com/g/a", Role: "code", Outcome: models.ArchiveRemoved}}})
	})
	links := func(raw string) {
		t.Helper()
		if _, err := d.conn.Exec("UPDATE tasks SET pr_links = ? WHERE id = ?", raw, task.ID); err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		links  string
		delete bool
	}{
		{`[]`, false},
		{`[{"url":"` + mrA + `","state":"merged"},{"url":"` + mrB + `","state":"open"}]`, false},
		{`[{"url":"` + mrA + `","state":"merged"},{"url":"` + mrB + `"}]`, false},
		{`[{"url":"` + mrA + `","state":"merged"},{"url":"` + mrB + `","state":"merged"}]`, true},
	}
	for _, c := range cases {
		links(c.links)
		asked = nil
		got, err := d.ArchiveTaskWorkspace(context.Background(), "usr_1", task.ID)
		if err != nil || !got.Archivable() {
			t.Fatalf("%s: %+v, %v", c.links, got, err)
		}
		if len(asked) != 1 {
			t.Fatalf("%s: operations %+v", c.links, asked)
		}
		op := asked[0]
		if op.Action != "archive_workspace" || op.UserID != "usr_1" || op.TaskID != task.ID || op.DeleteBranch != c.delete {
			t.Errorf("%s: operation %+v", c.links, op)
		}
		if !slices.Equal(op.Repositories, []string{"gitlab.com/g/a", "gitlab.com/g/b"}) {
			t.Errorf("repositories = %v", op.Repositories)
		}
	}
}

func TestArchiveTaskWorkspaceLeavesASharedBranchAlone(t *testing.T) {
	d, task, _ := twoRepoTask(t)
	d.SetAgentOperations(func(context.Context, agentprotocol.Operation) (json.RawMessage, error) {
		t.Error("a shared worktree was sent to the agent")
		return nil, errors.New("unexpected")
	})
	other, err := d.CreateTask(models.CreateTaskRequest{ProjectID: task.ProjectID, Title: "same batch"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.conn.Exec("UPDATE tasks SET branch_name = 'feat/12' WHERE id = ?", other.ID); err != nil {
		t.Fatal(err)
	}
	got, err := d.ArchiveTaskWorkspace(context.Background(), "", task.ID)
	if err != nil || !got.Archivable() || len(got.Repositories) != 2 {
		t.Fatalf("%+v, %v", got, err)
	}
	for _, entry := range got.Repositories {
		if entry.Outcome != models.ArchiveShared || entry.BranchOutcome != models.ArchiveBranchKept {
			t.Errorf("entry = %+v", entry)
		}
	}
}

func TestArchiveTaskWorkspaceRefusals(t *testing.T) {
	d, task, _ := twoRepoTask(t)
	if _, err := d.ArchiveTaskWorkspace(context.Background(), "", "missing"); !errors.Is(err, ErrArchiveTaskNotFound) {
		t.Errorf("unknown task: %v", err)
	}
	d.SetAgentOperations(func(context.Context, agentprotocol.Operation) (json.RawMessage, error) {
		return json.RawMessage(`null`), nil
	})
	if _, err := d.ArchiveTaskWorkspace(context.Background(), "", task.ID); err == nil || !strings.Contains(err.Error(), "update it") {
		t.Errorf("empty answer: %v", err)
	}
	d.SetAgentOperations(nil)
	if _, err := d.ArchiveTaskWorkspace(context.Background(), "", task.ID); err == nil {
		t.Error("archived with no agent")
	}
}
