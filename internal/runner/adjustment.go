package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	"tasks/internal/agentexec"
)

// PullRequestEvidence is forge-confirmed identity and readiness, not agent prose.
type PullRequestEvidence struct {
	URL    string
	Branch string
	SHA    string
	Open   bool
	Draft  bool
	Merged bool
	// Forge names the forge that answered, "github" or "gitlab", so a caller
	// can word a refusal in the terms of the forge the user works with.
	Forge string
}

// The forge answered, but its answer is not usable evidence. Both are refusals,
// never lookup failures: the listing succeeded, so they must not be retried as
// an outage, and they must never be read as permission to create a PR.
var (
	ErrNoMatchingPullRequest = errors.New("no matching open or merged pull request")
	ErrAmbiguousPullRequest  = errors.New("several open pull requests on the branch")
)

// refusal words a refusal in the forge's own terms while still matching its kind.
type refusal struct {
	kind    error
	message string
}

func (e refusal) Error() string { return e.message }
func (e refusal) Unwrap() error { return e.kind }

func parsePullRequestEvidence(raw, branch string, gitlab bool) (PullRequestEvidence, error) {
	if gitlab {
		var rows []struct {
			URL      string     `json:"web_url"`
			Branch   string     `json:"source_branch"`
			SHA      string     `json:"sha"`
			State    string     `json:"state"`
			Draft    *bool      `json:"draft"`
			WIP      *bool      `json:"work_in_progress"`
			MergedAt *time.Time `json:"merged_at"`
		}
		if err := json.Unmarshal([]byte(raw), &rows); err != nil {
			return PullRequestEvidence{}, err
		}
		var open []PullRequestEvidence
		var fallback *PullRequestEvidence
		var fallbackAt *time.Time
		for i := range rows {
			p := rows[i]
			if p.Branch != branch || p.URL == "" {
				continue
			}
			if p.State == "opened" {
				if p.Draft == nil && p.WIP == nil {
					return PullRequestEvidence{}, fmt.Errorf("forge omitted MR readiness")
				}
				draft := (p.Draft != nil && *p.Draft) || (p.WIP != nil && *p.WIP)
				open = append(open, PullRequestEvidence{URL: p.URL, Branch: p.Branch, SHA: p.SHA, Open: true, Draft: draft})
				continue
			}
			// A merged MR is the same task MR; readiness no longer applies once it is
			// merged. The latest merge is the branch's state; without a merge date the
			// listing order, newest first, decides.
			if p.State == "merged" && (fallback == nil || (p.MergedAt != nil && (fallbackAt == nil || p.MergedAt.After(*fallbackAt)))) {
				fallback = &PullRequestEvidence{URL: p.URL, Branch: p.Branch, SHA: p.SHA, Merged: true}
				fallbackAt = p.MergedAt
			}
			// A closed-unmerged MR is abandoned work, never evidence.
		}
		switch {
		case len(open) == 1:
			return open[0], nil
		case len(open) > 1:
			// Which one is current cannot be guessed, as on the GitHub server path.
			urls := make([]string, len(open))
			for i, p := range open {
				urls[i] = p.URL
			}
			return PullRequestEvidence{}, refusal{ErrAmbiguousPullRequest, fmt.Sprintf("expected one open merge request on %s, got %d (%s)", branch, len(open), strings.Join(urls, ", "))}
		case fallback != nil:
			return *fallback, nil
		}
		return PullRequestEvidence{}, refusal{ErrNoMatchingPullRequest, "no matching open or merged merge request; recover through the configured creation owner"}
	}
	var p struct {
		URL    string `json:"url"`
		Branch string `json:"headRefName"`
		SHA    string `json:"headRefOid"`
		State  string `json:"state"`
		Draft  *bool  `json:"isDraft"`
	}
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return PullRequestEvidence{}, err
	}
	if p.URL == "" || p.Branch != branch || (p.State != "OPEN" && p.State != "MERGED") {
		return PullRequestEvidence{}, refusal{ErrNoMatchingPullRequest, "no matching open or merged pull request; recover through the configured creation owner"}
	}
	// A merged PR is the same task PR; readiness no longer applies once it is merged.
	if p.State == "MERGED" {
		return PullRequestEvidence{URL: p.URL, Branch: p.Branch, SHA: p.SHA, Merged: true}, nil
	}
	if p.Draft == nil {
		return PullRequestEvidence{}, fmt.Errorf("forge omitted PR readiness")
	}
	return PullRequestEvidence{URL: p.URL, Branch: p.Branch, SHA: p.SHA, Open: true, Draft: *p.Draft}, nil
}

// ErrNoOriginRemote is a lookup failure, never absence: without a remote the
// forge cannot be asked at all, and saying so names the way out instead of the
// exit status git happens to use.
var ErrNoOriginRemote = errors.New("project checkout has no origin remote; configure the project repository or pass the prUrl of the repository that carries the pull request")

// BranchPullRequest reports lookup failures distinctly; callers must never treat errors as permission to create.
func (r *Runner) BranchPullRequest(repoPath, branch string) (PullRequestEvidence, error) {
	if strings.TrimSpace(branch) == "" {
		return PullRequestEvidence{}, fmt.Errorf("task branch is missing")
	}
	remotes, err := agentexec.Hidden(exec.Command("git", "-C", repoPath, "remote")).Output()
	if err != nil {
		return PullRequestEvidence{}, fmt.Errorf("read repository remotes: %w", err)
	}
	if !slices.Contains(strings.Fields(string(remotes)), "origin") {
		return PullRequestEvidence{}, ErrNoOriginRemote
	}
	remote, err := agentexec.Hidden(exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")).Output()
	if err != nil {
		return PullRequestEvidence{}, fmt.Errorf("read repository remote: %w", err)
	}
	// Enterprise installations may use a hostname without the forge brand.
	firstGitLab := strings.Contains(strings.ToLower(string(remote)), "gitlab")
	return r.forgePullRequest(repoPath, branch, "", []bool{firstGitLab, !firstGitLab})
}

// RepositoryPullRequest looks the branch up in a repository other than the
// checkout's, named by its host/path identity, on the forge its pull request
// link names. dir only gives the CLI a working directory: it needs no remote.
// The selection and refusal rules are the ones of BranchPullRequest.
func (r *Runner) RepositoryPullRequest(dir, forge, repository, branch string) (PullRequestEvidence, error) {
	if strings.TrimSpace(branch) == "" {
		return PullRequestEvidence{}, fmt.Errorf("task branch is missing")
	}
	if (forge != "github" && forge != "gitlab") || strings.TrimSpace(repository) == "" {
		return PullRequestEvidence{}, fmt.Errorf("unsupported pull request repository %q on forge %q", repository, forge)
	}
	return r.forgePullRequest(dir, branch, repository, []bool{forge == "gitlab"})
}

// evidenceCommand builds the forge CLI call listing the branch's requests. An
// empty repository asks about the working directory's own remote; otherwise
// -R names the repository, as a URL for glab and as HOST/OWNER/REPO for gh.
func evidenceCommand(gitlab bool, branch, repository string) (string, []string) {
	if gitlab {
		// https://docs.gitlab.com/cli/mr/list/ : open MRs only unless --all, and a
		// merged MR is evidence, like a merged PR on GitHub.
		args := []string{"mr", "list", "--source-branch", branch, "--all", "--output", "json"}
		if repository != "" {
			args = append(args, "-R", "https://"+repository)
		}
		return "glab", args
	}
	args := []string{"pr", "view", branch, "--json", "url,state,headRefName,headRefOid,isDraft"}
	if repository != "" {
		args = append(args, "-R", repository)
	}
	return "gh", args
}

func (r *Runner) forgePullRequest(dir, branch, repository string, forges []bool) (PullRequestEvidence, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	var failures []string
	for _, gitlab := range forges {
		cli, args := evidenceCommand(gitlab, branch, repository)
		tool, err := FindCliTool(cli)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		run := func(args ...string) (string, error) { return r.runCommand(ctx, dir, tool, args...) }
		var out string
		if gitlab {
			out, err = gitLabEvidencePages(args, run)
		} else {
			out, err = run(args...)
		}
		if err != nil {
			failures = append(failures, cli+": "+err.Error())
			continue
		}
		// A successful response identifies the forge; invalid evidence is not a reason to substitute another PR.
		evidence, err := parsePullRequestEvidence(out, branch, gitlab)
		evidence.Forge = "github"
		if gitlab {
			evidence.Forge = "gitlab"
		}
		return evidence, err
	}
	return PullRequestEvidence{}, fmt.Errorf("PR lookup failed; retry or recover through the creation owner: %s", strings.Join(failures, "; "))
}

// --all selects states, not pages. Read the complete listing before choosing an
// MR: an older open request or the latest merge may be on a later page. All
// calls share the caller's deadline, and a partial listing is never evidence.
func gitLabEvidencePages(args []string, run func(...string) (string, error)) (string, error) {
	const pageSize = 100
	var rows []json.RawMessage
	for page := 1; ; page++ {
		pageArgs := append(append([]string(nil), args...), "--per-page", strconv.Itoa(pageSize), "--page", strconv.Itoa(page))
		out, err := run(pageArgs...)
		if err != nil {
			return "", fmt.Errorf("merge request page %d: %w", page, err)
		}
		var batch []json.RawMessage
		if err := json.Unmarshal([]byte(out), &batch); err != nil {
			return "", fmt.Errorf("decode merge request page %d: %w", page, err)
		}
		rows = append(rows, batch...)
		if len(batch) < pageSize {
			raw, err := json.Marshal(rows)
			return string(raw), err
		}
	}
}

// AdjustmentContract accompanies every native or managed customization.
const AdjustmentContract = `Mandatory adjustment contract: verify the existing matching task-branch PR before changes; it must be open, or already merged by the human. Never push onto a merged PR: review the final state and report it. Never create or replace a PR. Review the complete branch against the specification, reconcile the current remote default branch, retrieve available feedback and record dispositions; retrieval failure blocks completion. Run build/lint/test checks on final code, commit and push, update the same PR and verify readiness and its final commit. No human feedback is required. Preserve work on failure. Never merge, approve, close the task or remove its worktree. Missing PR recovery belongs to the configured earlier creation stage.`
