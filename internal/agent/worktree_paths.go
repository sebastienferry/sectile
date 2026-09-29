package agent

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"tasks/internal/models"
)

const worktreeNameLimit = 120

// safeWorktreeName keeps identity in the digest, independent of normalization.
func safeWorktreeName(key string) (string, error) {
	if key == "" || key == "." || key == ".." || strings.ContainsAny(key, "/\\") {
		return "", fmt.Errorf("invalid task key for worktree")
	}
	if strings.HasPrefix(key, "#") && len(key) > 1 && key[1] >= '1' && key[1] <= '9' {
		numeric := true
		for _, c := range key[1:] {
			numeric = numeric && c >= '0' && c <= '9'
		}
		if numeric && len(key)+5 <= worktreeNameLimit {
			return "issue-" + key[1:], nil
		}
	}
	var slug strings.Builder
	separator := false
	for _, c := range strings.ToLower(key) {
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			if separator && slug.Len() > 0 {
				slug.WriteByte('-')
			}
			slug.WriteRune(c)
			separator = false
		} else {
			separator = true
		}
		if slug.Len() >= 50 {
			break
		}
	}
	s := strings.Trim(slug.String(), "-")
	if s == "" {
		s = "item"
	}
	return fmt.Sprintf("key-%s-%x", s, sha256.Sum256([]byte(key))), nil
}

// taskWorktreeBranch is the branch a task's worktree carries: the one assigned
// to the task, which a format never renames, else the project's branch name
// format rendered for it (#621); an empty format gives feat/<key slug>.
func taskWorktreeBranch(task models.Task, format string) (string, error) {
	if _, err := safeWorktreeName(task.Key); err != nil {
		return "", err
	}
	if task.BranchName != nil && strings.TrimSpace(*task.BranchName) != "" {
		return strings.TrimSpace(*task.BranchName), nil
	}
	return models.TaskBranchName(format, task.Key, task.Title)
}

// localTaskPath uses Git's branch inventory, including arbitrary and legacy paths.
// An occupied prediction is never returned as a usable checkout.
func localTaskPath(ctx context.Context, root string, task models.Task, branchFormat string) (string, error) {
	name, err := safeWorktreeName(task.Key)
	if err != nil {
		return "", err
	}
	branch, err := taskWorktreeBranch(task, branchFormat)
	if err != nil {
		return "", err
	}
	if _, err := gitLocal(ctx, root, "check-ref-format", "--branch", branch); err != nil {
		return "", err
	}
	existing, err := worktreeForBranch(ctx, root, branch)
	if err != nil {
		return "", err
	}
	if existing != "" {
		if sameDirectory(existing, root) {
			return root, nil
		}
		return existing, nil
	}
	return availableTaskWorktreePath(root, name, branch)
}

// Lstat treats dangling symlinks as occupied. Git creation errors (including
// races after this selection) propagate without destructive retries.
func availableTaskWorktreePath(root, name, branch string) (string, error) {
	for i := 0; i < 100; i++ {
		candidate := name
		if i > 0 {
			candidate = occupiedWorktreeName(name, branch, i)
		}
		target := filepath.Join(root, ".tasks", "worktrees", candidate)
		if _, err := os.Lstat(target); os.IsNotExist(err) {
			return target, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("no free worktree destination for branch %s", branch)
}

func occupiedWorktreeName(name, branch string, counter int) string {
	suffix := fmt.Sprintf("-%x-%d", sha256.Sum256([]byte(branch)), counter)
	if len(name)+len(suffix) > worktreeNameLimit {
		name = name[:worktreeNameLimit-len(suffix)]
	}
	return name + suffix
}
