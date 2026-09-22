package trackerapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"tasks/internal/models"
	"testing"
)

func fixture(t *testing.T, handle http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handle)
	t.Cleanup(server.Close)
	return &Client{HTTP: server.Client(), GithubURL: server.URL, GithubToken: "github-secret"}
}

func TestGithubSyncUsesServerHTTPAndPaginatesIssues(t *testing.T) {
	calls := 0
	c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer github-secret" || r.URL.Path != "/repos/acme/app/issues" {
			t.Errorf("unexpected request: %s", r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "2" {
			fmt.Fprint(w, `[{"number":3,"title":"Closed","state":"closed","html_url":"https://github.com/acme/app/issues/3"}]`)
			return
		}
		w.Header().Set("Link", `</repos/acme/app/issues?page=2>; rel="next"`)
		fmt.Fprint(w, `[{"number":1,"title":"Build","body":"Details","state":"open","labels":[{"name":"#specified"}],"assignees":[{"login":"owner"}],"milestone":{"title":"Sprint 1","number":1}},{"number":2,"pull_request":{"url":"ignored"}}]`)
	})
	t.Setenv("PATH", t.TempDir())
	tasks, err := c.SyncFromGithub("acme/app", "/nonexistent/server/checkout")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(tasks) != 2 || tasks[0].Key != "#1" || tasks[0].Status != models.StatusToImplement || tasks[0].Assignee != "owner" || tasks[0].Sprint != "Sprint 1" || tasks[1].Status != models.StatusFinished {
		t.Fatalf("unexpected imported issues: %#v (%d calls)", tasks, calls)
	}
}

func TestGithubWritesPreserveLabelsAndUseExplicitFields(t *testing.T) {
	var patch map[string]any
	c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/app/issues/7" {
			t.Errorf("path: %s", r.URL)
		}
		if r.Method == "GET" {
			fmt.Fprint(w, `{"number":7,"labels":[{"name":"personal"},{"name":"#clarified"}]}`)
			return
		}
		if r.Method != "PATCH" {
			t.Errorf("method: %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			t.Error(err)
		}
		fmt.Fprint(w, `{}`)
	})
	status := models.StatusToImplement
	if err := c.UpdateGithubIssue("acme/app", "", "#7", nil, nil, &status, []string{"#specified"}, []string{"#clarified"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := patch["body"]; ok {
		t.Fatal("untouched description sent")
	}
	if patch["state"] != "open" || !reflect.DeepEqual(patch["labels"], []any{"personal", "#specified"}) {
		t.Fatalf("patch: %#v", patch)
	}
}

func TestTrackerFailuresNeverSucceedOrExposeSecrets(t *testing.T) {
	for _, status := range []int{401, 403, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			c := fixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status); fmt.Fprint(w, "github-secret") })
			if _, err := c.SyncFromGithub("acme/app", ""); err == nil || strings.Contains(err.Error(), "github-secret") {
				t.Fatalf("unsafe or missing error: %v", err)
			}
		})
	}
	calls := 0
	c := fixture(t, func(w http.ResponseWriter, r *http.Request) { calls++; fmt.Fprint(w, `[]`) })
	c.GithubToken = ""
	if _, err := c.SyncFromGithub("acme/app", ""); err == nil || calls != 0 {
		t.Fatalf("missing credentials sent request: %v", err)
	}
}

func TestGithubCreationFollowsRepositoryRedirects(t *testing.T) {
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			calls := 0
			c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path == "/repos/acme/old/issues" {
					http.Redirect(w, r, "/repositories/123/issues", status)
					return
				}
				if r.URL.Path != "/repositories/123/issues" || r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer github-secret" {
					t.Errorf("incorrect redirected request: %s %s", r.Method, r.URL.Path)
				}
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if payload["title"] != "New task" || payload["body"] != "Description" || !reflect.DeepEqual(payload["labels"], []any{"#new"}) {
					t.Errorf("redirect changed creation payload: %v", payload)
				}
				w.WriteHeader(http.StatusCreated)
				fmt.Fprint(w, `{"number":42,"title":"New task","html_url":"https://github.com/acme/new/issues/42"}`)
			})
			task, err := c.CreateGithubIssue("acme/old", "", "New task", "Description", []string{"#new"})
			if err != nil || task == nil || task.Key != "#42" || calls != 2 {
				t.Fatalf("redirected creation failed: task=%+v calls=%d err=%v", task, calls, err)
			}
		})
	}
}

func TestTrackerBoundsRedirectsAndPreservesMethods(t *testing.T) {
	for _, status := range []int{http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			calls := 0
			c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				http.Redirect(w, r, "/loop", status)
			})
			if _, err := c.CreateGithubIssue("acme/app", "", "Task", "", nil); err == nil {
				t.Fatal("accepted redirect loop or method change")
			}
			want := 1
			if status == http.StatusTemporaryRedirect {
				want = 10
			}
			if calls != want {
				t.Fatalf("made %d requests, want %d", calls, want)
			}
		})
	}
}

func TestTrackerRejectsForeignRedirectsAndPagination(t *testing.T) {
	leaked := false
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked = true }))
	defer other.Close()
	for _, redirect := range []bool{false, true} {
		c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
			if redirect {
				http.Redirect(w, r, other.URL, http.StatusFound)
				return
			}
			w.Header().Set("Link", "<"+other.URL+">; rel=\"next\"")
			fmt.Fprint(w, `[]`)
		})
		if _, err := c.SyncFromGithub("acme/app", ""); err == nil {
			t.Fatal("accepted foreign response")
		}
	}
	if leaked {
		t.Fatal("tracker credentials sent to another origin")
	}
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, other.URL, status)
		})
		if _, err := c.CreateGithubIssue("acme/app", "", "Task", "", nil); err == nil || leaked {
			t.Fatalf("foreign mutation redirect was not rejected: %v", err)
		}
	}
}

func TestGraphQLErrorsRejectPartialData(t *testing.T) {
	c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"repository":null},"errors":[{"message":"github-secret"}]}`)
	})
	if _, err := c.GithubGraphQL(`query{viewer{login}}`); err == nil || strings.Contains(err.Error(), "github-secret") {
		t.Fatalf("partial response: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := c.request(ctx, "GET", c.GithubURL, "token", nil); err == nil {
		t.Fatal("canceled request succeeded")
	}
}

func TestBranchPullRequestPrefersOpenThenMerged(t *testing.T) {
	for _, tc := range []struct {
		name   string
		body   string
		url    string
		sha    string
		merged bool
	}{
		{"open wins", `[{"html_url":"https://forge/pull/2","state":"closed","merged_at":"2026-01-01T00:00:00Z","head":{"ref":"ticket","sha":"old"}},{"html_url":"https://forge/pull/3","state":"open","head":{"ref":"ticket","sha":"tip"}}]`, "https://forge/pull/3", "tip", false},
		// The human merge boundary must not strand the task before reviewed.
		{"merged accepted", `[{"html_url":"https://forge/pull/2","state":"closed","merged_at":"2026-01-01T00:00:00Z","head":{"ref":"ticket","sha":"tip"}}]`, "https://forge/pull/2", "tip", true},
		{"closed unmerged rejected", `[{"html_url":"https://forge/pull/2","state":"closed","head":{"ref":"ticket","sha":"tip"}}]`, "", "", false},
		// A branch that produced several merged PRs is not ambiguous: the branch
		// moved on and the latest merge is its state.
		{"latest merge wins", `[{"html_url":"https://forge/pull/2","state":"closed","merged_at":"2026-01-01T00:00:00Z","head":{"ref":"ticket","sha":"a"}},{"html_url":"https://forge/pull/4","state":"closed","merged_at":"2026-01-02T00:00:00Z","head":{"ref":"ticket","sha":"b"}}]`, "https://forge/pull/4", "b", true},
		{"latest merge wins whatever the forge order", `[{"html_url":"https://forge/pull/4","state":"closed","merged_at":"2026-01-02T00:00:00Z","head":{"ref":"ticket","sha":"b"}},{"html_url":"https://forge/pull/2","state":"closed","merged_at":"2026-01-01T00:00:00Z","head":{"ref":"ticket","sha":"a"}}]`, "https://forge/pull/4", "b", true},
		// Two open PRs is a real ambiguity: which one is current cannot be guessed.
		{"ambiguous open rejected", `[{"html_url":"https://forge/pull/2","state":"open","head":{"ref":"ticket","sha":"a"}},{"html_url":"https://forge/pull/4","state":"open","head":{"ref":"ticket","sha":"b"}}]`, "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("state") != "all" {
					t.Errorf("merged PRs are unreachable with state=%q", r.URL.Query().Get("state"))
				}
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Query().Get("page") == "2" {
					_, _ = w.Write([]byte(`[]`))
					return
				}
				_, _ = w.Write([]byte(tc.body))
			})
			pr, err := c.BranchPullRequest("acme/app", "ticket")
			if tc.url == "" {
				if err == nil {
					t.Fatalf("accepted %+v", pr)
				}
				return
			}
			if err != nil || pr.URL != tc.url || pr.Merged != tc.merged || pr.Open == tc.merged || pr.SHA != tc.sha {
				t.Fatalf("%+v %v", pr, err)
			}
		})
	}
}

func TestNewClientResolvesTrackerCredentials(t *testing.T) {
	cases := []struct {
		name       string
		env        map[string]string
		wantGithub string
	}{
		{
			name:       "generic name alone serves every provider",
			env:        map[string]string{"SECTILE_TRACKER_TOKEN": "generic"},
			wantGithub: "generic",
		},
		{
			name:       "provider-specific names keep working on their own",
			env:        map[string]string{"SECTILE_GITHUB_TOKEN": "gh"},
			wantGithub: "gh",
		},
		{
			name:       "provider-specific name overrides the generic one",
			env:        map[string]string{"SECTILE_TRACKER_TOKEN": "generic", "SECTILE_GITHUB_TOKEN": "gh"},
			wantGithub: "gh",
		},
		{
			name:       "generic name outranks the environment conventions",
			env:        map[string]string{"SECTILE_TRACKER_TOKEN": "generic", "GH_TOKEN": "gh-cli"},
			wantGithub: "generic",
		},
		{
			name:       "environment conventions remain the last resort",
			env:        map[string]string{"GITHUB_TOKEN": "ci"},
			wantGithub: "ci",
		},
		{name: "no credential at all"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			for _, name := range []string{"SECTILE_TRACKER_TOKEN", "SECTILE_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"} {
				t.Setenv(name, "")
			}
			for name, value := range testCase.env {
				t.Setenv(name, value)
			}
			c := NewClient()
			if c.GithubToken != testCase.wantGithub {
				t.Errorf("got github %q, want %q", c.GithubToken, testCase.wantGithub)
			}
		})
	}
}

// Every call that needs a credential and has none must say the same thing, and
// it must point at the credential the person can actually set. It used to name
// SECTILE_TRACKER_TOKEN, which sends them to a deployment they usually cannot
// reach to fix something they can: this product attributes a tracker write to
// whoever made it, so each person stores their own token (ADR 0014). The
// server-wide variable survives for work nobody asked for, and is deliberately
// not advertised here.
func TestMissingCredentialErrorPointsAtThePersonalToken(t *testing.T) {
	c := &Client{HTTP: http.DefaultClient}
	for _, call := range []struct {
		name string
		run  func() error
	}{
		{"github", func() error { return c.github(context.Background(), http.MethodGet, "/rate_limit", nil, nil) }},
		{"githubPages", func() error { _, err := c.githubPages(context.Background(), "repos/acme/app/issues"); return err }},
		{"githubGraphQL", func() error { _, err := c.GithubGraphQL("{viewer{login}}"); return err }},
	} {
		err := call.run()
		if err == nil {
			t.Errorf("%s: a call with no credential must fail", call.name)
			continue
		}
		if got, want := err.Error(), missingCredential("GitHub"); got != want {
			t.Errorf("%s: got %q, want %q", call.name, got, want)
		}
		if strings.Contains(err.Error(), genericTokenVar) {
			t.Errorf("%s: the error still names the server variable: %v", call.name, err)
		}
	}
}

// The message a person sees when nothing could be resolved must point at the
// credential they can actually set. Sending them to a server environment
// variable asks them to change a deployment they usually cannot reach, to fix
// something they can.
func TestMissingCredentialPointsAtThePersonalToken(t *testing.T) {
	msg := missingCredential("GitHub")
	if strings.Contains(msg, genericTokenVar) || strings.Contains(strings.ToLower(msg), "server") {
		t.Fatalf("the message sends the user to the server credential: %q", msg)
	}
	if !strings.Contains(msg, "GitHub") {
		t.Fatalf("the message does not name the tracker: %q", msg)
	}
}
