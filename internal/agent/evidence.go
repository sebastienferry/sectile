package agent

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"tasks/internal/models"
	"tasks/internal/runner"
)

// checkoutEvidence is the head of a verified checkout: a local checkout of the
// repository a pull request lives in, other than the project checkout.
type checkoutEvidence struct {
	Path   string
	SHA    string
	Branch string
	Clean  bool
}

// verifiedCheckout looks for a checkout of repository with branch checked out.
// The candidates come from the server, so they are hints only: one is used
// when its own origin names repository, whatever the server believes it is.
// A candidate that is not a readable checkout with an origin is skipped; a Git
// failure once a candidate is known to be the repository is an error, so an
// unreadable checkout is never reported as a missing one.
func verifiedCheckout(ctx context.Context, repository, branch string, candidates []string) (checkoutEvidence, bool, error) {
	seen := map[string]bool{}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || !filepath.IsAbs(candidate) {
			continue
		}
		candidate = filepath.Clean(candidate)
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		if fi, err := os.Stat(candidate); err != nil || !fi.IsDir() {
			continue
		}
		remote, err := gitLocal(ctx, candidate, "remote", "get-url", "origin")
		if err != nil || models.RepositoryIdentity(remote) != repository {
			continue
		}
		worktrees, err := gitLocal(ctx, candidate, "worktree", "list", "--porcelain")
		if err != nil {
			return checkoutEvidence{}, false, err
		}
		path := worktreeOnBranch(worktrees, branch)
		if path == "" {
			continue
		}
		sha, err := gitLocal(ctx, path, "rev-parse", "HEAD")
		if err != nil {
			return checkoutEvidence{}, false, err
		}
		status, err := gitLocal(ctx, path, "status", "--porcelain")
		if err != nil {
			return checkoutEvidence{}, false, err
		}
		return checkoutEvidence{Path: path, SHA: strings.TrimSpace(sha), Branch: branch, Clean: strings.TrimSpace(status) == ""}, true, nil
	}
	return checkoutEvidence{}, false, nil
}

// worktreeOnBranch reads `git worktree list --porcelain` and returns the
// worktree, the main checkout included, that has branch checked out.
func worktreeOnBranch(porcelain, branch string) string {
	path := ""
	for _, line := range strings.Split(porcelain, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "worktree "):
			path = strings.TrimPrefix(line, "worktree ")
		case line == "branch refs/heads/"+branch && path != "":
			return path
		case line == "":
			path = ""
		}
	}
	return ""
}

// checkoutCandidates lists where a checkout of another repository may be: the
// repository the task pins first, then the ones the project has known.
func (d *agentDaemon) checkoutCandidates(ctx context.Context, task models.Task, projectID string) ([]string, error) {
	var candidates []string
	if task.RepoPath != nil {
		candidates = append(candidates, *task.RepoPath)
	}
	var project models.Project
	if err := d.readAPI(ctx, "/api/projects/"+url.PathEscape(projectID), &project); err != nil {
		return nil, err
	}
	candidates = append(candidates, project.RepoPath)
	return append(candidates, project.RepoPaths...), nil
}

// adjustmentPullRequest pre-checks the PR an adjustment will update. When the
// task's current PR lives in a repository other than the checkout's, the
// branch is looked up there, as the server does; otherwise, and when the task
// has no PR yet, in the checkout's own repository.
func adjustmentPullRequest(ctx context.Context, workDir, branch, current string) (runner.PullRequestEvidence, error) {
	r := runner.NewRunner()
	if link, ok := models.ParsePullRequestLink(current, ""); ok {
		remote, err := gitLocal(ctx, workDir, "remote", "get-url", "origin")
		if err != nil || models.RepositoryIdentity(remote) != link.Identity() {
			return r.RepositoryPullRequest(workDir, link.Forge, link.Identity(), branch)
		}
	}
	return r.BranchPullRequest(workDir, branch)
}
