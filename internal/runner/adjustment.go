package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// PullRequestEvidence is forge-confirmed identity and readiness, not agent prose.
type PullRequestEvidence struct {
	URL    string
	Branch string
	SHA    string
	Open   bool
	Draft  bool
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
		for _, p := range rows {
			if p.Branch == branch && p.State == "opened" && p.URL != "" {
				if p.Draft == nil && p.WIP == nil {
					return PullRequestEvidence{}, fmt.Errorf("forge omitted PR readiness")
				}
				draft := (p.Draft != nil && *p.Draft) || (p.WIP != nil && *p.WIP)
				return PullRequestEvidence{p.URL, p.Branch, p.SHA, true, draft}, nil
			}
		}
		return PullRequestEvidence{}, fmt.Errorf("no matching open merge request; recover through the configured creation owner")
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
	if p.URL == "" || p.Branch != branch || p.State != "OPEN" {
		return PullRequestEvidence{}, fmt.Errorf("no matching open pull request; recover through the configured creation owner")
	}
	if p.Draft == nil {
		return PullRequestEvidence{}, fmt.Errorf("forge omitted PR readiness")
	}
	return PullRequestEvidence{p.URL, p.Branch, p.SHA, true, *p.Draft}, nil
}

// BranchPullRequest reports lookup failures distinctly; callers must never treat errors as permission to create.
func (r *Runner) BranchPullRequest(repoPath, branch string) (PullRequestEvidence, error) {
	if strings.TrimSpace(branch) == "" {
		return PullRequestEvidence{}, fmt.Errorf("task branch is missing")
	}
	remote, err := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin").Output()
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
const AdjustmentContract = `Mandatory adjustment contract: verify the existing matching open task-branch PR before changes. Never create or replace a PR. Review the complete branch against the specification, reconcile the current remote default branch, retrieve available feedback and record dispositions; retrieval failure blocks completion. Run build/lint/test checks on final code, commit and push, update the same PR and verify readiness and its final commit. No human feedback is required. Preserve work on failure. Never merge, approve, close the task or remove its worktree. Missing PR recovery belongs to the configured earlier creation stage.`
