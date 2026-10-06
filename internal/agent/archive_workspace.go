package agent

import (
	"context"
	"os"
	"strings"

	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
)

// archiveWorkspace answers archive_workspace (#755): the Desktop archives a
// ticket task only once each of its worktrees is gone. Each worktree of the
// task's branch is removed with a plain "git worktree remove", so Git keeps
// one holding uncommitted or untracked changes and the entry fails. The
// task's specifications worktree, in a distinct Issue folder, is removed the
// same way. Where its worktree is gone, the local branch is deleted when the
// server found every pull request of the task merged and the branch holds
// nothing its upstream lacks; a branch kept never fails an entry.
func archiveWorkspace(ctx context.Context, config agentconfig.Config, overrides agentconfig.Settings, projectRoot string, task models.Task, op agentprotocol.Operation) models.WorkspaceArchive {
	result := models.WorkspaceArchive{Repositories: []models.WorkspaceArchiveEntry{}}
	if !config.UseWorktrees {
		// The task works in the checkout itself, which is never removed.
		result.Repositories = append(result.Repositories, models.WorkspaceArchiveEntry{Role: "code", Outcome: models.ArchiveDisabled})
		return result
	}
	branch, _ := taskWorktreeBranch(task, config.BranchNameFormat)
	var roots []string
	archive := func(identity, role, root string) {
		roots = append(roots, root)
		entry := archiveWorktree(ctx, root, branch)
		entry.Repository, entry.Role = identity, role
		if entry.Outcome != models.ArchiveFailed {
			entry.BranchOutcome, entry.BranchReason = archiveBranch(ctx, root, branch, op.DeleteBranch)
		}
		result.Repositories = append(result.Repositories, entry)
	}
	if len(op.Repositories) > 0 {
		code := codeIdentity(config)
		for _, identity := range op.Repositories {
			root, ok := repositoryFolder(ctx, overrides, config.ProjectID, projectRoot, code, identity)
			if !ok {
				result.Repositories = append(result.Repositories, models.WorkspaceArchiveEntry{Repository: identity, Role: "code", Branch: branch,
					Outcome: models.ArchiveFailed, Error: "not found on this workstation"})
				continue
			}
			archive(identity, "code", root)
		}
	} else {
		root, identity, err := primaryRoot(ctx, config, overrides, projectRoot, task)
		if err != nil {
			result.Repositories = append(result.Repositories, models.WorkspaceArchiveEntry{Repository: strings.TrimSpace(task.Repository), Role: "code", Branch: branch,
				Outcome: models.ArchiveFailed, Error: err.Error()})
		} else {
			archive(identity, "code", root)
		}
	}
	if overrides.IssueSpecPath(config.ProjectID) == "" {
		return result
	}
	issue, err := localIssueSpecRepo(overrides, config.ProjectID, projectRoot)
	if err != nil {
		result.Repositories = append(result.Repositories, models.WorkspaceArchiveEntry{Role: "specifications", Branch: branch, Outcome: models.ArchiveFailed, Error: err.Error()})
		return result
	}
	for _, root := range roots {
		if sameDirectory(root, issue) {
			return result
		}
	}
	if _, err := gitLocal(ctx, issue, "rev-parse", "--git-dir"); err != nil {
		// A plain Issue folder is written in place: it has no worktree.
		return result
	}
	archive("", "specifications", issue)
	return result
}

// archiveWorktree removes the worktree of branch in the repository at root.
// No worktree on the branch, or the branch checked out in root itself, is
// absent: there is nothing of the task's own to remove there.
func archiveWorktree(ctx context.Context, root, branch string) models.WorkspaceArchiveEntry {
	entry := models.WorkspaceArchiveEntry{Branch: branch, Outcome: models.ArchiveAbsent}
	if branch == "" {
		return entry
	}
	worktree, err := worktreeForBranch(ctx, root, branch)
	if err != nil {
		entry.Outcome, entry.Error = models.ArchiveFailed, err.Error()
		return entry
	}
	if worktree == "" || sameDirectory(worktree, root) {
		return entry
	}
	entry.Path = worktree
	if _, err := os.Stat(worktree); os.IsNotExist(err) {
		// A folder deleted by hand leaves Git's record behind, which "worktree
		// remove" refuses: pruning it is all that is left to do.
		if _, err := gitLocal(ctx, root, "worktree", "prune"); err != nil {
			entry.Outcome, entry.Error = models.ArchiveFailed, err.Error()
		}
		return entry
	}
	if _, err := gitLocal(ctx, root, "worktree", "remove", worktree); err != nil {
		entry.Outcome, entry.Error = models.ArchiveFailed, err.Error()
		return entry
	}
	entry.Outcome = models.ArchiveRemoved
	return entry
}

// archiveBranch deletes the task's local branch in the repository at root
// when deleteBranch says its pull requests are merged, and says why it is
// kept otherwise. "git branch -D" is needed: a squash merge leaves the branch
// unmerged for Git, which is why the upstream comparison guards it instead.
func archiveBranch(ctx context.Context, root, branch string, deleteBranch bool) (outcome, reason string) {
	if branch == "" {
		return "", ""
	}
	if !deleteBranch {
		return models.ArchiveBranchKept, "no-merged-pr"
	}
	if _, err := gitLocal(ctx, root, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); err != nil {
		return models.ArchiveBranchKept, "missing"
	}
	if worktree, err := worktreeForBranch(ctx, root, branch); err != nil {
		return models.ArchiveBranchKept, err.Error()
	} else if worktree != "" {
		return models.ArchiveBranchKept, "checked-out"
	}
	if _, err := gitLocal(ctx, root, "rev-parse", "--verify", "--quiet", branch+"@{upstream}"); err != nil {
		return models.ArchiveBranchKept, "no-upstream"
	}
	ahead, err := gitLocal(ctx, root, "rev-list", "--count", branch+"@{upstream}.."+branch)
	if err != nil {
		return models.ArchiveBranchKept, err.Error()
	}
	if strings.TrimSpace(ahead) != "0" {
		return models.ArchiveBranchKept, "unpushed"
	}
	if _, err := gitLocal(ctx, root, "branch", "-D", branch); err != nil {
		return models.ArchiveBranchKept, err.Error()
	}
	return models.ArchiveBranchDeleted, ""
}
