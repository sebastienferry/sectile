package trackerapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPullRequestStatesGithubBatchesAndMapsStates(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/graphql" || r.Method != "POST" || r.Header.Get("Authorization") != "Bearer gh-token" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		var payload struct{ Query string }
		json.NewDecoder(r.Body).Decode(&payload)
		count := strings.Count(payload.Query, "pullRequest(number:")
		if count > 50 {
			t.Errorf("unbounded batch: %d", count)
		}
		data := map[string]any{}
		for i := 0; i < count; i++ {
			state, mergeable := "OPEN", "MERGEABLE"
			switch i % 5 {
			case 1:
				mergeable = "CONFLICTING"
			case 2:
				state = "MERGED"
			case 3:
				state = "CLOSED"
			case 4:
				mergeable = "UNKNOWN"
			}
			data[fmt.Sprintf("p%d", i)] = map[string]any{"pullRequest": map[string]string{"state": state, "mergeable": mergeable}}
		}
		json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer server.Close()
	c := &Client{GithubURL: server.URL, GithubToken: "gh-token", HTTP: server.Client()}
	var links []string
	for i := 1; i <= 51; i++ {
		links = append(links, fmt.Sprintf("%s/acme/app/pull/%d", server.URL, i))
	}
	links = append(links, links[0], "https://untrusted.example/acme/app/pull/1")
	states, err := c.PullRequestStates(context.Background(), "github", links)
	if err != nil || len(states) != 51 || calls != 2 {
		t.Fatalf("states=%d calls=%d err=%v", len(states), calls, err)
	}
	for i, want := range []string{"open", "conflicting", "merged", "closed", "open"} {
		if states[links[i]] != want {
			t.Errorf("%d: %q, want %q", i, states[links[i]], want)
		}
	}
}

func TestPullRequestStatesGitlabGroupsAndMapsStates(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" || !strings.Contains(r.URL.Path, "/merge_requests") || r.Header.Get("Authorization") != "Bearer gl-token" {
			t.Errorf("unexpected request %s", r.URL)
		}
		if r.URL.Query().Get("scope") != "all" || r.URL.Query().Get("state") != "all" {
			t.Errorf("missing filters: %s", r.URL)
		}
		ids := r.URL.Query()["iids[]"]
		if len(ids) > 50 {
			t.Errorf("unbounded batch: %d", len(ids))
		}
		fmt.Fprint(w, `[{"iid":1,"state":"opened","detailed_merge_status":"conflict"},{"iid":2,"state":"merged","has_conflicts":true},{"iid":3,"state":"closed"},{"iid":4,"state":"opened","detailed_merge_status":"checking"},{"iid":5,"state":"opened","has_conflicts":true}]`)
	}))
	defer server.Close()
	c := &Client{GitlabURL: server.URL + "/api/v4", GitlabToken: "gl-token", HTTP: server.Client()}
	var links []string
	for i := 1; i <= 5; i++ {
		links = append(links, fmt.Sprintf("%s/group/sub/app/-/merge_requests/%d", server.URL, i))
	}
	links = append(links, server.URL+"/group/other/-/merge_requests/1")
	states, err := c.PullRequestStates(context.Background(), "gitlab", links)
	if err != nil || calls != 2 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	for i, want := range []string{"conflicting", "merged", "closed", "open", "conflicting", "conflicting"} {
		if states[links[i]] != want {
			t.Errorf("%d: %q, want %q", i, states[links[i]], want)
		}
	}
}

func TestPullRequestStatesErrorsNeverInventStates(t *testing.T) {
	for _, body := range []string{`{"errors":[{"message":"denied"}]}`, `{"data":{"p0":null}}`, `invalid`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer server.Close()
			c := &Client{GithubURL: server.URL, GithubToken: "token", HTTP: server.Client()}
			states, err := c.PullRequestStates(context.Background(), "github", []string{server.URL + "/a/b/pull/1"})
			if len(states) != 0 {
				t.Fatalf("invented states: %v", states)
			}
			if body != `{"data":{"p0":null}}` && err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestPullRequestForgeValidatesConfiguredHost(t *testing.T) {
	c := &Client{GithubURL: DefaultGithubURL, GitlabURL: "https://git.example/api/v4"}
	for _, raw := range []string{"https://github.com.evil/a/b/pull/1", "http://github.com/a/b/pull/1", "https://user@github.com/a/b/pull/1", "https://github.com/a/b/pull/1?redirect=x", "https://git.example/a/b/-/merge_requests/0"} {
		if c.PullRequestForge(raw) != "" {
			t.Errorf("accepted %s", raw)
		}
	}
	if c.PullRequestForge("https://github.com/a/b/pull/1") != "github" || c.PullRequestForge("https://git.example/a/b/-/merge_requests/1") != "gitlab" {
		t.Fatal("valid link refused")
	}
}

func TestPullRequestStatesKeepTheAliasesThatResolved(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"p0":{"pullRequest":null},"p1":{"pullRequest":{"state":"MERGED","mergeable":"UNKNOWN"}}},"errors":[{"message":"Could not resolve to a PullRequest","path":["p0","pullRequest"]}]}`)
	}))
	defer server.Close()
	c := &Client{GithubURL: server.URL, GithubToken: "token", HTTP: server.Client()}
	links := []string{server.URL + "/gone/repo/pull/1", server.URL + "/acme/app/pull/2"}
	states, err := c.PullRequestStates(context.Background(), "github", links)
	if err == nil || len(states) != 1 || states[links[1]] != "merged" {
		t.Fatalf("states=%v err=%v", states, err)
	}
}

func TestPullRequestStatesReadTheBatchesAfterAFailedOne(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		fmt.Fprint(w, `{"data":{"p0":{"pullRequest":{"state":"CLOSED","mergeable":"UNKNOWN"}}}}`)
	}))
	defer server.Close()
	c := &Client{GithubURL: server.URL, GithubToken: "token", HTTP: server.Client()}
	var links []string
	for i := 1; i <= 51; i++ {
		links = append(links, fmt.Sprintf("%s/acme/app/pull/%d", server.URL, i))
	}
	states, err := c.PullRequestStates(context.Background(), "github", links)
	if err == nil || calls != 2 || len(states) != 1 || states[links[50]] != "closed" {
		t.Fatalf("calls=%d states=%v err=%v", calls, states, err)
	}
}

func TestPullRequestStatesReadTheGitlabProjectsAfterAFailedOne(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.EscapedPath(), "a%2Fgone") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Fprint(w, `[{"iid":1,"state":"merged"}]`)
	}))
	defer server.Close()
	c := &Client{GitlabURL: server.URL + "/api/v4", GitlabToken: "gl-token", HTTP: server.Client()}
	links := []string{server.URL + "/a/gone/-/merge_requests/1", server.URL + "/b/app/-/merge_requests/1"}
	states, err := c.PullRequestStates(context.Background(), "gitlab", links)
	if err == nil || len(states) != 1 || states[links[1]] != "merged" {
		t.Fatalf("states=%v err=%v", states, err)
	}
}

func TestPullRequestStatesStopOnARateLimit(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	c := &Client{GithubURL: server.URL, GithubToken: "token", HTTP: server.Client()}
	var links []string
	for i := 1; i <= 101; i++ {
		links = append(links, fmt.Sprintf("%s/acme/app/pull/%d", server.URL, i))
	}
	if _, err := c.PullRequestStates(context.Background(), "github", links); !IsRateLimited(err) || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}
