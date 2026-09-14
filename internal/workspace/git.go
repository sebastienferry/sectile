package workspace

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"tasks/internal/models"
	"tasks/internal/runner"
)

type Workspace struct {
	runner *runner.Runner
	ctx    context.Context
}

func New(ctx context.Context) *Workspace { return &Workspace{runner: runner.NewRunner(), ctx: ctx} }
func (d *Workspace) GetGitBranches(projectIDOrPath string) (*models.GitBranchesInfo, error) {
	repoPath := projectIDOrPath

	if repoPath == "" {
		return nil, fmt.Errorf("no Git repository configured")
	}

	if _, err := os.Stat(repoPath); err != nil {
		return nil, fmt.Errorf("directory %s does not exist", repoPath)
	}

	// Current active branch
	curCmd := exec.CommandContext(d.ctx, "git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	curOut, _ := curCmd.Output()
	currentBranch := strings.TrimSpace(string(curOut))

	// Get all branches (local + remote)
	format := "%(HEAD)|%(refname:short)|%(objectname:short)|%(contents:subject)"
	cmd := exec.CommandContext(d.ctx, "git", "-C", repoPath, "branch", "-a", "--format="+format)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list Git branches: %w", err)
	}

	var branches []models.GitBranchItem
	seen := make(map[string]bool)

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 2 {
			continue
		}
		isHead := strings.TrimSpace(parts[0]) == "*"
		rawName := strings.TrimSpace(parts[1])
		commit := ""
		if len(parts) >= 3 {
			commit = strings.TrimSpace(parts[2])
		}
		message := ""
		if len(parts) >= 4 {
			message = strings.TrimSpace(parts[3])
		}

		if rawName == "" || strings.HasPrefix(rawName, "origin/HEAD") || strings.HasPrefix(rawName, "remotes/origin/HEAD") {
			continue
		}

		isRemote := strings.HasPrefix(rawName, "origin/") || strings.HasPrefix(rawName, "remotes/")
		cleanName := strings.TrimPrefix(rawName, "remotes/")
		cleanName = strings.TrimPrefix(cleanName, "origin/")

		if seen[cleanName] {
			continue
		}
		seen[cleanName] = true

		isCurrent := isHead || cleanName == currentBranch

		branches = append(branches, models.GitBranchItem{
			Name:      cleanName,
			IsCurrent: isCurrent,
			IsRemote:  isRemote,
			Commit:    commit,
			Message:   message,
		})
	}

	if currentBranch != "" && !seen[currentBranch] {
		branches = append([]models.GitBranchItem{{
			Name:      currentBranch,
			IsCurrent: true,
		}}, branches...)
	}

	return &models.GitBranchesInfo{
		RepoPath:      repoPath,
		CurrentBranch: currentBranch,
		Branches:      branches,
	}, nil
}

func (d *Workspace) SwitchGitBranch(projectIDOrPath, targetBranch string, create bool) (*models.GitStatusInfo, error) {
	repoPath := projectIDOrPath

	if repoPath == "" {
		return nil, fmt.Errorf("no Git repository configured")
	}

	targetBranch = strings.TrimSpace(targetBranch)
	if targetBranch == "" {
		return nil, fmt.Errorf("target branch is required")
	}

	// Clean origin/ or remotes/ prefixes if user selected a remote branch
	targetBranch = strings.TrimPrefix(targetBranch, "remotes/")
	targetBranch = strings.TrimPrefix(targetBranch, "origin/")

	if err := exec.CommandContext(d.ctx, "git", "check-ref-format", "--branch", targetBranch).Run(); err != nil {
		return nil, fmt.Errorf("invalid branch name")
	}

	// 1. Get current branch
	curCmd := exec.CommandContext(d.ctx, "git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	curOut, _ := curCmd.Output()
	currentBranch := strings.TrimSpace(string(curOut))

	if currentBranch == targetBranch && !create {
		return d.runner.GetCwdGitStatus(repoPath)
	}

	// Let Git enforce dirty-checkout and occupied-worktree safety. Never reset an
	// existing branch or detach another checkout as a fallback.
	args := []string{"-C", repoPath, "switch"}
	if create {
		args = append(args, "-c")
	}
	args = append(args, targetBranch)
	if output, err := exec.CommandContext(d.ctx, "git", args...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("switch branch: %s (%w)", output, err)
	}

	return d.runner.GetCwdGitStatus(repoPath)
}

func (d *Workspace) CleanAllLocalBranches(projectIDOrPath string) (*models.CleanBranchesResult, error) {
	repoPath := projectIDOrPath

	if repoPath == "" {
		repoPath = "."
	}

	if fi, err := os.Stat(repoPath); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("directory not found: %s", repoPath)
	}

	// 1. Detect default branch (main or master)
	defaultBranch := "main"
	if err := exec.CommandContext(d.ctx, "git", "-C", repoPath, "rev-parse", "--verify", "main").Run(); err != nil {
		if err2 := exec.CommandContext(d.ctx, "git", "-C", repoPath, "rev-parse", "--verify", "master").Run(); err2 == nil {
			defaultBranch = "master"
		}
	}

	// 2. Remove any active worktrees under .tasks/worktrees/
	worktreesDir := filepath.Join(repoPath, ".tasks", "worktrees")
	if entries, err := os.ReadDir(worktreesDir); err == nil {
		for _, e := range entries {
			wtPath := filepath.Join(worktreesDir, e.Name())
			if err := exec.CommandContext(d.ctx, "git", "-C", repoPath, "worktree", "remove", wtPath).Run(); err != nil {
				return nil, fmt.Errorf("worktree removal failed; preserve local changes: %w", err)
			}
		}
	}
	_ = exec.CommandContext(d.ctx, "git", "-C", repoPath, "worktree", "prune").Run()

	// 3. Checkout default branch in main repo
	if out, err := exec.CommandContext(d.ctx, "git", "-C", repoPath, "checkout", defaultBranch).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("checkout default branch: %s (%w)", out, err)
	}

	// 4. List all local branches
	out, err := exec.CommandContext(d.ctx, "git", "-C", repoPath, "branch", "--format=%(refname:short)").Output()
	if err != nil {
		return nil, fmt.Errorf("list branches: %w", err)
	}

	var deleted []string
	lines := strings.Split(string(out), "\n")
	for _, l := range lines {
		branch := strings.TrimSpace(l)
		if branch == "" || branch == defaultBranch || branch == "main" || branch == "master" {
			continue
		}
		// Delete only merged local branches
		delCmd := exec.CommandContext(d.ctx, "git", "-C", repoPath, "branch", "-d", branch)
		if delOut, delErr := delCmd.CombinedOutput(); delErr == nil {
			deleted = append(deleted, branch)
		} else {
			return nil, fmt.Errorf("removed %d branches; could not delete %s: %s", len(deleted), branch, delOut)
		}
	}

	return &models.CleanBranchesResult{
		RepoPath:        repoPath,
		DefaultBranch:   defaultBranch,
		DeletedBranches: deleted,
		Message:         fmt.Sprintf("Removed %d merged local branches.", len(deleted)),
	}, nil
}

func (d *Workspace) DeleteGitBranch(projectIDOrPath string, branchName string, deleteRemote bool) error {
	branchName = strings.TrimSpace(branchName)
	if branchName == "" {
		return fmt.Errorf("branch name is required")
	}
	if branchName == "main" || branchName == "master" || branchName == "HEAD" {
		return fmt.Errorf("cannot delete primary branch '%s'", branchName)
	}

	if err := exec.CommandContext(d.ctx, "git", "check-ref-format", "--branch", branchName).Run(); err != nil {
		return fmt.Errorf("invalid branch name")
	}
	repoPath := projectIDOrPath

	if repoPath == "" {
		repoPath = "."
	}

	// 1. If a worktree is checked out on this branch, remove it
	worktreesDir := filepath.Join(repoPath, ".tasks", "worktrees")
	if entries, err := os.ReadDir(worktreesDir); err == nil {
		for _, e := range entries {
			wtPath := filepath.Join(worktreesDir, e.Name())
			curCmd := exec.CommandContext(d.ctx, "git", "-C", wtPath, "rev-parse", "--abbrev-ref", "HEAD")
			if curOut, curErr := curCmd.Output(); curErr == nil && strings.TrimSpace(string(curOut)) == branchName {
				if err := exec.CommandContext(d.ctx, "git", "-C", repoPath, "worktree", "remove", wtPath).Run(); err != nil {
					return fmt.Errorf("worktree removal failed; preserve local changes: %w", err)
				}
			}
		}
	}
	_ = exec.CommandContext(d.ctx, "git", "-C", repoPath, "worktree", "prune").Run()

	// 2. If main repo is currently on this branch, switch to main/master first
	curCmd := exec.CommandContext(d.ctx, "git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if curOut, curErr := curCmd.Output(); curErr == nil && strings.TrimSpace(string(curOut)) == branchName {
		defaultBranch := "main"
		if err := exec.CommandContext(d.ctx, "git", "-C", repoPath, "rev-parse", "--verify", "main").Run(); err != nil {
			defaultBranch = "master"
		}
		if out, err := exec.CommandContext(d.ctx, "git", "-C", repoPath, "checkout", defaultBranch).CombinedOutput(); err != nil {
			return fmt.Errorf("checkout default branch: %s (%w)", out, err)
		}
	}

	// 3. Delete local branch
	delCmd := exec.CommandContext(d.ctx, "git", "-C", repoPath, "branch", "-d", branchName)
	if out, err := delCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("delete local branch %s: %s", branchName, string(out))
	}

	// 4. Optionally delete remote branch
	if deleteRemote {
		if out, err := exec.CommandContext(d.ctx, "git", "-C", repoPath, "push", "origin", "--delete", branchName).CombinedOutput(); err != nil {
			return fmt.Errorf("remote branch deletion failed: %s (%w)", out, err)
		}
	}

	return nil
}
