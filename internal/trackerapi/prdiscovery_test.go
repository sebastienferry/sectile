package trackerapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// The nominal read: the closing references of an issue, oldest first, with the
// branch each pull request was pushed from.
func TestIssuePullRequestsReadsClosingReferencesOldestFirst(t *testing.T) {
	var query string
	c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graphql" || r.Header.Get("Authorization") != "Bearer github-secret" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		query = string(buf)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"repository":{"issue":{"closedByPullRequestsReferences":{"nodes":[
			{"url":"https://github.com/acme/app/pull/9","headRefName":"feat/42-follow-up","createdAt":"2026-02-01T10:00:00Z","state":"OPEN","merged":false,"isDraft":true},
			{"url":"https://github.com/acme/app/pull/4","headRefName":"feat/42","createdAt":"2026-01-01T10:00:00Z","state":"MERGED","merged":true,"isDraft":false}
		]}}}}}`)
	})

	found, err := c.IssuePullRequests("acme/app", 42)
	if err != nil {
		t.Fatalf("discovery failed: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("expected two pull requests, got %+v", found)
	}
	// Oldest first: the set is ordered and its last link is the current PR.
	if found[0].URL != "https://github.com/acme/app/pull/4" || !found[0].Merged {
		t.Errorf("oldest pull request is not first: %+v", found)
	}
	if found[1].Branch != "feat/42-follow-up" || !found[1].Open || !found[1].Draft {
		t.Errorf("state of the newest pull request lost: %+v", found[1])
	}
	if !strings.Contains(query, "closedByPullRequestsReferences") || !strings.Contains(query, `"number":42`) {
		t.Errorf("query does not ask for the issue's closing references: %s", query)
	}
}

// An issue nothing closes is not an error: the task simply keeps no link.
func TestIssuePullRequestsWithoutAnyReference(t *testing.T) {
	c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"repository":{"issue":{"closedByPullRequestsReferences":{"nodes":[]}}}}}`)
	})
	found, err := c.IssuePullRequests("acme/app", 42)
	if err != nil || len(found) != 0 {
		t.Fatalf("an issue with no pull request must read as an empty set: %+v %v", found, err)
	}
}

// A node with no URL is dropped rather than recorded as an empty link, and a
// missing head branch only means the branch-based lookup cannot help later.
func TestIssuePullRequestsSkipsNodesWithoutURL(t *testing.T) {
	c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"repository":{"issue":{"closedByPullRequestsReferences":{"nodes":[
			{"url":"","headRefName":"gone","createdAt":"2026-01-01T10:00:00Z"},
			{"url":"https://github.com/acme/app/pull/4","headRefName":"","createdAt":"2026-01-02T10:00:00Z"}
		]}}}}}`)
	})
	found, err := c.IssuePullRequests("acme/app", 42)
	if err != nil || len(found) != 1 || found[0].Branch != "" {
		t.Fatalf("expected the single usable pull request, got %+v %v", found, err)
	}
}

// A rate limit must surface as such, so the caller can step back instead of
// hammering the instance once per ticket.
func TestIssuePullRequestsSurfacesRateLimit(t *testing.T) {
	c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"message":"API rate limit exceeded"}`)
	})
	_, err := c.IssuePullRequests("acme/app", 42)
	if err == nil {
		t.Fatal("a 429 must be an error")
	}
	if !IsRateLimited(err) && !strings.Contains(err.Error(), "429") {
		t.Fatalf("rate limit not recognisable: %v", err)
	}
}

// Discovery without a credential is refused before any request leaves.
func TestIssuePullRequestsRequiresACredential(t *testing.T) {
	c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request must be made without a token")
	})
	c.GithubToken = ""
	if _, err := c.IssuePullRequests("acme/app", 42); err == nil {
		t.Fatal("a missing credential must be reported")
	}
}

// The capability is what every call site asks before attempting anything:
// GitHub announces it and implements the interface, the others do neither.
func TestOnlyGithubAnnouncesPullRequestDiscovery(t *testing.T) {
	c := &Client{}
	gh := NewGithubAdapter(c)
	if !gh.Supports(tracker.CapPullRequests) {
		t.Error("GitHub must announce pull request discovery")
	}
	if _, ok := tracker.TicketingSystem(gh).(tracker.PullRequestDiscoverer); !ok {
		t.Error("GitHub must implement the discovery interface")
	}
	for name, ts := range map[string]tracker.TicketingSystem{"jira": NewJiraAdapter(c), "local": tracker.NewLocalAdapter()} {
		if ts.Supports(tracker.CapPullRequests) {
			t.Errorf("%s must not announce pull request discovery", name)
		}
		if _, ok := ts.(tracker.PullRequestDiscoverer); ok {
			t.Errorf("%s must not implement the discovery interface", name)
		}
	}
}

// The adapter maps what the client returns to the links a task records, and
// refuses a project with no repository rather than guessing one.
func TestGithubAdapterIssuePullRequests(t *testing.T) {
	c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"repository":{"issue":{"closedByPullRequestsReferences":{"nodes":[
			{"url":"https://github.com/acme/app/pull/4","headRefName":"feat/42","createdAt":"2026-01-01T10:00:00Z","state":"MERGED","merged":true}
		]}}}}}`)
	})
	gh := NewGithubAdapter(c)
	links, err := gh.IssuePullRequests(context.Background(), tracker.IssuePullRequestsRequest{
		Project: &models.Project{GithubRepo: "acme/app"},
		Key:     "#42",
	})
	if err != nil || len(links) != 1 || links[0].URL != "https://github.com/acme/app/pull/4" || links[0].Branch != "feat/42" {
		t.Fatalf("adapter did not map the discovery: %+v %v", links, err)
	}
	if _, err = gh.IssuePullRequests(context.Background(), tracker.IssuePullRequestsRequest{Project: &models.Project{}, Key: "#42"}); err == nil {
		t.Fatal("a project with no repository must be refused")
	}
}
