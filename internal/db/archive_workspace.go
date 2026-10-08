package db

import (
	"context"
	"fmt"
	"strings"

	"tasks/internal/agentprotocol"
	"tasks/internal/models"
)

// ErrArchiveTaskNotFound refuses the archive of a task the server does not know.
var ErrArchiveTaskNotFound = fmt.Errorf("task not found")

// ArchiveTaskWorkspace cleans the worktrees of a task the caller archives in
// Desktop (#755), on the caller's own agent. The server decides what the agent
// may do, from what only it holds: a branch another task of the project
// records is shared by a batch, and is neither removed nor checked; the local
// branch may go only when the task records pull requests and every one of
// them is merged; a ticket that changed several repositories is cleaned in
// each of them (#456).
func (d *DB) ArchiveTaskWorkspace(ctx context.Context, userID, taskID string) (*models.WorkspaceArchive, error) {
	task, err := d.GetTaskByID(taskID)
	if err != nil || task == nil {
		return nil, ErrArchiveTaskNotFound
	}
	project, _ := d.GetProjectByID(task.ProjectID)
	var repositories []string
	if project != nil && multiRepoTask(project, task) {
		if primary := TaskPrimaryRepository(project, task); primary != "" {
			repositories = append(repositories, primary)
		}
		repositories = append(repositories, taskChangedRepositories(project, task)...)
	}
	branch := ""
	if task.BranchName != nil {
		branch = strings.TrimSpace(*task.BranchName)
	}
	shared, err := d.branchSharedWithAnotherTask(task.ProjectID, task.ID, branch)
	if err != nil {
		return nil, err
	}
	if shared {
		result := &models.WorkspaceArchive{}
		names := repositories
		if len(names) == 0 {
			names = []string{""}
		}
		for _, repository := range names {
			result.Repositories = append(result.Repositories, models.WorkspaceArchiveEntry{Repository: repository, Role: "code", Branch: branch,
				Outcome: models.ArchiveShared, BranchOutcome: models.ArchiveBranchKept, BranchReason: "shared"})
		}
		return result, nil
	}
	var result models.WorkspaceArchive
	op := agentprotocol.Operation{UserID: strings.TrimSpace(userID), ProjectID: task.ProjectID, TaskID: task.ID, Action: "archive_workspace",
		Repositories: repositories, DeleteBranch: pullRequestsAllMerged(task.PrLinks)}
	if err := d.callAgentContext(ctx, op, &result); err != nil {
		return nil, err
	}
	if len(result.Repositories) == 0 {
		return nil, fmt.Errorf("local agent is too old to clean a worktree on archive; update it")
	}
	return &result, nil
}

// branchSharedWithAnotherTask says another task of the project records branch,
// as the tickets of one batch do.
func (d *DB) branchSharedWithAnotherTask(projectID, taskID, branch string) (bool, error) {
	if branch == "" {
		return false, nil
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	// A ticket records its tracker rather than the project, so the project's
	// rows are its trackers' tickets (#741).
	scope, args := d.projectRowsScopeUnsafe(projectID)
	args = append(args, branch, taskID)
	var count int
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM tasks WHERE "+scope+" AND branch_name = ? AND id <> ?", args...).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// pullRequestsAllMerged says a task records at least one pull request and
// every one of them is merged. A state not refreshed yet is not merged.
func pullRequestsAllMerged(links []models.TaskPullRequest) bool {
	if len(links) == 0 {
		return false
	}
	for _, link := range links {
		if link.State != "merged" {
			return false
		}
	}
	return true
}
