package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"tasks/internal/agentexec"
)

// OpenBranchMergeRequestURL confirms an open PR for the current commit, rather
// than accepting an old/closed PR or a URL mentioned in the agent's prose.
func (r *Runner) OpenBranchMergeRequestURL(repoPath, branch string) string {
	head, err := agentexec.Hidden(exec.Command("git", "-C", repoPath, "rev-parse", "HEAD")).Output()
	if err != nil || strings.TrimSpace(branch) == "" {
		return ""
	}
	sha := strings.TrimSpace(string(head))
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if gh, err := FindCliTool("gh"); err == nil {
		out, err := r.runCommand(ctx, repoPath, gh, "pr", "view", branch, "--json", "url,state,headRefName,headRefOid")
		if err == nil {
			if url := openPRFromJSON(out, branch, sha, false); url != "" {
				return url
			}
		}
	}
	if glab, err := FindCliTool("glab"); err == nil {
		// https://docs.gitlab.com/cli/mr/list/ : defaults to open MRs.
		out, err := r.runCommand(ctx, repoPath, glab, "mr", "list", "--source-branch", branch, "--output", "json")
		if err == nil {
			return openPRFromJSON(out, branch, sha, true)
		}
	}
	return ""
}

func openPRFromJSON(raw, branch, sha string, gitlab bool) string {
	if gitlab {
		var prs []struct {
			URL    string `json:"web_url"`
			State  string `json:"state"`
			Branch string `json:"source_branch"`
			SHA    string `json:"sha"`
		}
		if json.Unmarshal([]byte(raw), &prs) == nil {
			for _, pr := range prs {
				if pr.State == "opened" && pr.Branch == branch && pr.SHA == sha {
					return pr.URL
				}
			}
		}
		return ""
	}
	var pr struct {
		URL    string `json:"url"`
		State  string `json:"state"`
		Branch string `json:"headRefName"`
		SHA    string `json:"headRefOid"`
	}
	if json.Unmarshal([]byte(raw), &pr) == nil && pr.State == "OPEN" && pr.Branch == branch && pr.SHA == sha {
		return pr.URL
	}
	return ""
}

// Récupération de la merge request ouverte par une skill.
//
// Avant, Taskacao fabriquait l'URL : hôte GitHub codé en dur et numéro du ticket
// à la place du numéro de la PR. Elle était donc fausse dès que le dépôt vivait
// sur GitLab, et pointait sur une PR sans rapport sur GitHub.
//
// Deux sources, dans cet ordre : la forge elle-même, qui sait quelle MR porte la
// branche, puis à défaut le compte-rendu de l'agent, où l'URL figure presque
// toujours.

var (
	githubPRPattern = regexp.MustCompile(`https?://[^\s"'<>)\]]+/pull/\d+`)
	gitlabMRPattern = regexp.MustCompile(`https?://[^\s"'<>)\]]+/-/merge_requests/\d+`)
)

// DetectMergeRequestURL extracts the last pull or merge request URL of a text.
// The last one, because a report often cites the branch's previous ones before
// announcing the one it just opened.
func DetectMergeRequestURL(text string) string {
	best := ""
	bestAt := -1
	for _, pattern := range []*regexp.Regexp{githubPRPattern, gitlabMRPattern} {
		for _, loc := range pattern.FindAllStringIndex(text, -1) {
			if loc[0] > bestAt {
				bestAt = loc[0]
				best = strings.TrimRight(text[loc[0]:loc[1]], ".,;")
			}
		}
	}
	return best
}

// BranchMergeRequestURL asks the forge which merge request carries a branch.
// Empty when there is none, or when no CLI is available for that forge.
func (r *Runner) BranchMergeRequestURL(repoPath, branch string) string {
	branch = strings.TrimSpace(branch)
	if branch == "" || strings.TrimSpace(repoPath) == "" {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	if gh, err := FindCliTool("gh"); err == nil && gh != "" {
		out, err := r.runCommand(ctx, repoPath, gh, "pr", "view", branch, "--json", "url")
		if err == nil {
			var payload struct {
				URL string `json:"url"`
			}
			if json.Unmarshal([]byte(strings.TrimSpace(out)), &payload) == nil && payload.URL != "" {
				return payload.URL
			}
		}
	}

	if glab, err := FindCliTool("glab"); err == nil && glab != "" {
		// glab n'expose pas --json sur toutes les versions : on lit l'URL dans la
		// sortie plutôt que de dépendre d'un format.
		out, err := r.runCommand(ctx, repoPath, glab, "mr", "list", "--source-branch", branch)
		if err == nil {
			if url := DetectMergeRequestURL(out); url != "" {
				return url
			}
		}
		out, err = r.runCommand(ctx, repoPath, glab, "mr", "view", branch)
		if err == nil {
			if url := DetectMergeRequestURL(out); url != "" {
				return url
			}
		}
	}

	return ""
}

// MergeRequestForStep returns the merge request of a workflow step: what the
// forge reports for the branch, and failing that what the agent wrote.
func (r *Runner) MergeRequestForStep(repoPath, branch, agentOutput string) (string, string) {
	if url := r.BranchMergeRequestURL(repoPath, branch); url != "" {
		return url, "forge"
	}
	if url := DetectMergeRequestURL(agentOutput); url != "" {
		return url, "compte-rendu"
	}
	return "", ""
}

// MergeRequestStep describes what was found, for the activity log.
func MergeRequestStep(url, source string) string {
	if url == "" {
		return "🔗 Aucune merge request détectée pour cette branche"
	}
	return fmt.Sprintf("🔗 Merge request rattachée depuis %s : %s", source, url)
}
