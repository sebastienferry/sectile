package models

import (
	"fmt"
	"strings"
)

// A task holds an ordered set of pull request links rather than a single URL:
// one ticket routinely produces several PRs, a first one merged and a follow-up
// pushed on the same branch. The rules below are shared by the server, which
// validates a stage transition, and the agent, which pre-checks its own
// adjustment before dispatching one — the two must not disagree on what counts
// as the task's own pull request.

// NormalizePullRequestLinks trims every entry, drops the ones with no URL and
// keeps the first occurrence of a URL, so the order stays the order the links
// were recorded in.
func NormalizePullRequestLinks(links []TaskPullRequest) []TaskPullRequest {
	var out []TaskPullRequest
	seen := map[string]bool{}
	for _, link := range links {
		url := strings.TrimSpace(link.URL)
		if url == "" || seen[url] {
			continue
		}
		seen[url] = true
		out = append(out, TaskPullRequest{URL: url, Branch: strings.TrimSpace(link.Branch), State: NormalizePullRequestState(link.State)})
	}
	return out
}

// AppendPullRequestLink records url once. An already recorded URL leaves the set
// untouched, including its position: re-running a stage must not reorder the
// history of a task.
func AppendPullRequestLink(links []TaskPullRequest, url, branch string) []TaskPullRequest {
	url = strings.TrimSpace(url)
	links = NormalizePullRequestLinks(links)
	if url == "" {
		return links
	}
	for _, link := range links {
		if link.URL == url {
			return links
		}
	}
	return append(links, TaskPullRequest{URL: url, Branch: strings.TrimSpace(branch)})
}

// CurrentPullRequest is the value of the task's single PR link: the last of the
// set, or the empty string once every link has been detached.
func CurrentPullRequest(links []TaskPullRequest) string {
	links = NormalizePullRequestLinks(links)
	if len(links) == 0 {
		return ""
	}
	return links[len(links)-1].URL
}

// PullRequestBranches lists the branches the set mentions, for an error that
// says which branch was expected instead of only that the PR was refused.
func PullRequestBranches(links []TaskPullRequest) []string {
	var out []string
	seen := map[string]bool{}
	for _, link := range links {
		branch := strings.TrimSpace(link.Branch)
		if branch == "" || seen[branch] {
			continue
		}
		seen[branch] = true
		out = append(out, branch)
	}
	return out
}

// AcceptPullRequest is the swap guard, read over the whole set rather than over
// one recorded URL. A PR already recorded, or a newer one on a branch the task
// has already used, is the task's own work — a merged recorded link never vetoes
// it. A PR on a branch no link mentions is a substitution and is refused: that is
// the case the guard exists for, and the recorded branches say so in the error.
func AcceptPullRequest(links []TaskPullRequest, url, branch string) error {
	if len(links) == 0 {
		return nil
	}
	url = strings.TrimSpace(url)
	for _, link := range links {
		if strings.TrimSpace(link.URL) == url {
			return nil
		}
	}
	branch = strings.TrimSpace(branch)
	for _, link := range links {
		if branch != "" && strings.TrimSpace(link.Branch) == branch {
			return nil
		}
	}
	recorded := PullRequestBranches(links)
	if len(recorded) == 0 {
		// Links recorded before branches were kept carry no branch to compare
		// against; the lineage cannot be checked, so the forge evidence stands.
		return nil
	}
	return fmt.Errorf("pull request %s is on branch %q, unrelated to the recorded %s", url, branch, strings.Join(recorded, ", "))
}

// NormalizePullRequestState keeps legacy and unrecognized states unknown.
func NormalizePullRequestState(state string) string {
	switch state {
	case "open", "conflicting", "merged", "closed":
		return state
	}
	return ""
}
