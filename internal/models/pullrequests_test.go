package models

import (
	"slices"
	"strings"
	"testing"
)

const (
	appPR    = "https://github.com/o/app/pull/1"
	appPR2   = "https://github.com/o/app/pull/2"
	deployMR = "https://gitlab.com/g/deploy/-/merge_requests/7"
)

func urls(links []TaskPullRequest) []string {
	var out []string
	for _, link := range links {
		out = append(out, link.URL)
	}
	return out
}

func TestNormalizePullRequestLinksDerivesTheRepository(t *testing.T) {
	links := NormalizePullRequestLinks([]TaskPullRequest{
		{URL: " " + deployMR + " ", MissingToken: "gitlab"},
		{URL: "https://example.org/not-a-pr"},
	})
	if links[0].Repository != "gitlab.com/g/deploy" || links[0].MissingToken != "gitlab" {
		t.Fatalf("first link = %+v", links[0])
	}
	if links[1].Repository != "" {
		t.Fatalf("an unrecognized URL names no repository: %+v", links[1])
	}
}

func TestAcceptRepositoryPullRequest(t *testing.T) {
	recorded := []TaskPullRequest{{URL: appPR, Branch: "feat/1"}}
	allowed := []string{"github.com/o/app", "gitlab.com/g/deploy"}
	cases := []struct {
		name    string
		links   []TaskPullRequest
		url     string
		branch  string
		allowed []string
		refused string
	}{
		{name: "another changed repository on another branch is an addition", links: recorded, url: deployMR, branch: "other", allowed: allowed},
		{name: "a follow-up on the same branch", links: recorded, url: appPR2, branch: "feat/1", allowed: allowed},
		{name: "a substitution within the repository", links: recorded, url: appPR2, branch: "other", allowed: allowed, refused: `unrelated to the recorded feat/1`},
		{name: "a repository outside the task", links: recorded, url: "https://github.com/x/y/pull/3", branch: "feat/1", allowed: allowed, refused: "not one of the task's repositories"},
		{name: "a repository outside the task already recorded", links: append(slices.Clone(recorded), TaskPullRequest{URL: "https://github.com/x/y/pull/3"}), url: "https://github.com/x/y/pull/3", allowed: allowed},
		{name: "no allowed set scopes without the repository check", links: recorded, url: "https://github.com/x/y/pull/3", branch: "other"},
		{name: "an unrecognized URL keeps the whole-set rule", links: recorded, url: "https://example.org/pr/9", branch: "other", allowed: allowed, refused: "unrelated"},
		{name: "an unrecognized recorded link still vetoes", links: []TaskPullRequest{{URL: "https://example.org/pr/9", Branch: "feat/1"}}, url: deployMR, branch: "other", allowed: allowed, refused: "unrelated"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := AcceptRepositoryPullRequest(c.links, c.url, c.branch, c.allowed)
			if c.refused == "" && err != nil {
				t.Fatalf("refused: %v", err)
			}
			if c.refused != "" && (err == nil || !strings.Contains(err.Error(), c.refused)) {
				t.Fatalf("err = %v, want %q", err, c.refused)
			}
		})
	}
}

func TestAddPullRequestLinkKeepsThePrimaryCurrent(t *testing.T) {
	primary := []string{"github.com/o/app"}
	links := AddPullRequestLink(nil, appPR, "feat/1", primary)
	links = AddPullRequestLink(links, deployMR, "feat/1", primary)
	if got := urls(links); !slices.Equal(got, []string{deployMR, appPR}) || CurrentPullRequest(links) != appPR {
		t.Fatalf("a secondary link displaced the primary one: %v", got)
	}
	links = AddPullRequestLink(links, appPR2, "feat/1", primary)
	if got := urls(links); !slices.Equal(got, []string{deployMR, appPR, appPR2}) {
		t.Fatalf("a follow-up of the primary one is the current one: %v", got)
	}
	if got := urls(AddPullRequestLink(links, deployMR, "feat/1", primary)); !slices.Equal(got, []string{deployMR, appPR, appPR2}) {
		t.Fatalf("a recorded link was moved: %v", got)
	}
	if got := urls(AddPullRequestLink([]TaskPullRequest{{URL: appPR}}, deployMR, "", nil)); !slices.Equal(got, []string{appPR, deployMR}) {
		t.Fatalf("an unknown primary must leave the order alone: %v", got)
	}
	if got := urls(KeepPrimaryLast([]TaskPullRequest{{URL: deployMR}}, primary)); !slices.Equal(got, []string{deployMR}) {
		t.Fatalf("no primary link to move: %v", got)
	}
}
