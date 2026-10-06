package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"tasks/internal/models"
)

// branchChangesAnswer says whether a task branch carries commits of its own in
// a repository other than the project's (#678). The server skips that
// repository's pull request only on found with ahead 0; any failure to
// answer is an error, never a found false or an ahead 0.
type branchChangesAnswer struct {
	Repository    string `json:"repository"`
	Found         bool   `json:"found"`
	DefaultBranch string `json:"defaultBranch,omitempty"`
	Exists        bool   `json:"exists"`
	Ahead         int    `json:"ahead"`
	// LazyCode says the task's launches here go without its code worktree
	// (#737), so its code repository needs no pull request when unchanged.
	LazyCode bool `json:"lazyCode,omitempty"`
}

// matchingCheckout says whether candidate is a usable directory whose own
// origin names repository. seen keeps a candidate listed twice from being
// asked twice. A candidate that is not a readable checkout with an origin is
// skipped, never an error: the candidates are hints only.
func matchingCheckout(ctx context.Context, repository, candidate string, seen map[string]bool) (string, bool) {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" || !filepath.IsAbs(candidate) {
		return "", false
	}
	candidate = filepath.Clean(candidate)
	if seen[candidate] {
		return "", false
	}
	seen[candidate] = true
	if fi, err := os.Stat(candidate); err != nil || !fi.IsDir() {
		return "", false
	}
	remote, err := gitLocal(ctx, candidate, "remote", "get-url", "origin")
	if err != nil || models.RepositoryIdentity(remote) != repository {
		return "", false
	}
	return candidate, true
}

// repositoryCheckout returns the first candidate whose own origin names
// repository, whatever branch it has checked out.
func repositoryCheckout(ctx context.Context, repository string, candidates []string) (string, bool) {
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if checkout, ok := matchingCheckout(ctx, repository, candidate, seen); ok {
			return checkout, true
		}
	}
	return "", false
}

// branchChanges counts the commits of branch that the default branch of
// checkout does not have, over every ref of the branch it can see: the local
// branch, the remote-tracking ref and the head origin reports now. Whether
// the branch was pushed plays no part; a branch seen nowhere has none. A head
// origin reports that is not a local commit cannot be counted, so it is an
// error, as is an origin/HEAD that was never set.
func branchChanges(ctx context.Context, checkout, branch string) (defaultBranch string, exists bool, ahead int, err error) {
	if branch == "" {
		return "", false, 0, errors.New("branch is required")
	}
	head, err := gitLocal(ctx, checkout, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD")
	if err != nil {
		return "", false, 0, fmt.Errorf("default branch of %s unknown (run git remote set-head origin --auto there): %w", checkout, err)
	}
	// The full ref, so a local branch named origin/main cannot shadow it.
	base := head
	defaultBranch = strings.TrimPrefix(head, "refs/remotes/origin/")
	if branch == defaultBranch {
		// Work committed straight on the default branch has nothing ahead of
		// it, yet it changed the repository.
		return "", false, 0, fmt.Errorf("%s is the default branch of %s: its commits cannot be told apart", branch, checkout)
	}

	var refs []string
	for _, ref := range []string{"refs/heads/" + branch, "refs/remotes/origin/" + branch} {
		sha, found, err := resolveRef(ctx, checkout, ref)
		if err != nil {
			return "", false, 0, err
		}
		if found {
			refs = append(refs, sha)
		}
	}
	listed, err := gitLocal(ctx, checkout, "ls-remote", "--heads", "origin", "refs/heads/"+branch)
	if err != nil {
		return "", false, 0, err
	}
	// A pattern matches any ref ending with it, so only the exact ref counts.
	if sha := listedHead(listed, "refs/heads/"+branch); sha != "" {
		if _, err := gitLocal(ctx, checkout, "cat-file", "-e", sha+"^{commit}"); err != nil {
			return "", false, 0, fmt.Errorf("origin holds %s at %s, which this checkout has not fetched: %w", branch, sha, err)
		}
		refs = append(refs, sha)
	}
	for _, ref := range refs {
		count, err := gitLocal(ctx, checkout, "rev-list", "--count", base+".."+ref)
		if err != nil {
			return "", false, 0, err
		}
		n, err := strconv.Atoi(count)
		if err != nil {
			return "", false, 0, fmt.Errorf("git rev-list --count answered %q", count)
		}
		ahead = max(ahead, n)
	}
	return defaultBranch, len(refs) > 0, ahead, nil
}

// listedHead returns the commit `git ls-remote` lists for exactly ref, "" when
// it lists none.
func listedHead(listed, ref string) string {
	for _, line := range strings.Split(listed, "\n") {
		if sha, name, ok := strings.Cut(strings.TrimSpace(line), "\t"); ok && name == ref {
			return sha
		}
	}
	return ""
}

// resolveRef reads ref in checkout. A missing ref is not an error; any other
// Git failure is.
func resolveRef(ctx context.Context, checkout, ref string) (string, bool, error) {
	sha, err := gitLocal(ctx, checkout, "rev-parse", "--verify", "--quiet", ref)
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return sha, true, nil
}
