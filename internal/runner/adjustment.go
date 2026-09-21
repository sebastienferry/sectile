package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
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
}

func parsePullRequestEvidence(raw, branch string, gitlab bool) (PullRequestEvidence, error) {
	if gitlab {
		var rows []struct {
			URL    string `json:"web_url"`
			Branch string `json:"source_branch"`
			SHA    string `json:"sha"`
			State  string `json:"state"`
			Draft  *bool  `json:"draft"`
			WIP    *bool  `json:"work_in_progress"`
		}
		if err := json.Unmarshal([]byte(raw), &rows); err != nil {
			return PullRequestEvidence{}, err
		}
		var fallback *PullRequestEvidence
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
				return PullRequestEvidence{p.URL, p.Branch, p.SHA, true, draft, false}, nil
			}
			// A merged MR is the same task MR; readiness no longer applies once it is merged.
			if p.State == "merged" && fallback == nil {
				fallback = &PullRequestEvidence{p.URL, p.Branch, p.SHA, false, false, true}
			}
		}
		if fallback != nil {
			return *fallback, nil
		}
		return PullRequestEvidence{}, fmt.Errorf("no matching open or merged merge request; recover through the configured creation owner")
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
		return PullRequestEvidence{}, fmt.Errorf("no matching open or merged pull request; recover through the configured creation owner")
	}
	// A merged PR is the same task PR; readiness no longer applies once it is merged.
	if p.State == "MERGED" {
		return PullRequestEvidence{p.URL, p.Branch, p.SHA, false, false, true}, nil
	}
	if p.Draft == nil {
		return PullRequestEvidence{}, fmt.Errorf("forge omitted PR readiness")
	}
	return PullRequestEvidence{p.URL, p.Branch, p.SHA, true, *p.Draft, false}, nil
}

// BranchPullRequest reports lookup failures distinctly; callers must never treat errors as permission to create.
func (r *Runner) BranchPullRequest(repoPath, branch string) (PullRequestEvidence, error) {
	if strings.TrimSpace(branch) == "" {
		return PullRequestEvidence{}, fmt.Errorf("task branch is missing")
	}
	remote, err := agentexec.Hidden(exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")).Output()
	if err != nil {
		return PullRequestEvidence{}, fmt.Errorf("read repository remote: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	// Enterprise installations may use a hostname without the forge brand.
	firstGitLab := strings.Contains(strings.ToLower(string(remote)), "gitlab")
	var failures []string
	for _, gitlab := range []bool{firstGitLab, !firstGitLab} {
		cli := "gh"
		args := []string{"pr", "view", branch, "--json", "url,state,headRefName,headRefOid,isDraft"}
		if gitlab {
			cli = "glab"
			args = []string{"mr", "list", "--source-branch", branch, "--output", "json"}
		}
		tool, err := FindCliTool(cli)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		out, err := r.runCommand(ctx, repoPath, tool, args...)
		if err != nil {
			failures = append(failures, cli+": "+err.Error())
			continue
		}
		// A successful response identifies the forge; invalid evidence is not a reason to substitute another PR.
		return parsePullRequestEvidence(out, branch, gitlab)
	}
	return PullRequestEvidence{}, fmt.Errorf("PR lookup failed; retry or recover through the creation owner: %s", strings.Join(failures, "; "))

}

// AdjustmentContract accompanies every native or managed customization.
const AdjustmentContract = `Mandatory adjustment contract: verify the existing matching task-branch PR before changes; it must be open, or already merged by the human. Never push onto a merged PR: review the final state and report it. Never create or replace a PR. Review the complete branch against the specification, reconcile the current remote default branch, retrieve available feedback and record dispositions; retrieval failure blocks completion. Run build/lint/test checks on final code, commit and push, update the same PR and verify readiness and its final commit. No human feedback is required. Preserve work on failure. Never merge, approve, close the task or remove its worktree. Missing PR recovery belongs to the configured earlier creation stage.`
