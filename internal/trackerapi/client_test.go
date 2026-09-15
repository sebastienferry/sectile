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
	return &Client{HTTP: server.Client(), GithubURL: server.URL, GithubToken: "github-secret", LinearURL: server.URL, LinearToken: "linear-secret"}
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
	c.LinearToken = ""
	if _, err := c.SyncFromLinear("APP"); err == nil || calls != 0 {
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

func TestLinearSyncPaginatesAndPreservesMapping(t *testing.T) {
	calls := 0
	c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "linear-secret" {
			t.Error("wrong authorization")
		}
		var request struct {
			Query     string
			Variables map[string]any
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if !strings.Contains(request.Query, "issues(first:100") {
			t.Errorf("query: %s", request.Query)
		}
		filter := request.Variables["filter"].(map[string]any)["team"].(map[string]any)["key"].(map[string]any)["eq"]
		if filter != "APP" {
			t.Errorf("team filter: %v", filter)
		}
		if calls == 1 {
			fmt.Fprint(w, `{"data":{"issues":{"nodes":[{"id":"one","identifier":"APP-1","title":"First","priority":1,"state":{"name":"In Progress"},"labels":{"nodes":[{"id":"l","name":"#specified"}]}}],"pageInfo":{"hasNextPage":true,"endCursor":"cursor"}}}}`)
			return
		}
		if request.Variables["after"] != "cursor" {
			t.Errorf("cursor: %v", request.Variables)
		}
		fmt.Fprint(w, `{"data":{"issues":{"nodes":[{"id":"two","identifier":"APP-2","title":"Second"}],"pageInfo":{"hasNextPage":false}}}}`)
	})
	t.Setenv("PATH", t.TempDir())
	tasks, err := c.SyncFromLinear("APP")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 || tasks[0].Priority != models.PriorityUrgent || tasks[0].Status != models.StatusToImplement || tasks[1].Key != "APP-2" {
		t.Fatalf("mapping: %#v", tasks)
	}
}

func TestLinearMutationRequiresAcknowledgement(t *testing.T) {
	var input map[string]any
	c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query     string
			Variables map[string]any
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if strings.Contains(req.Query, "issue(id:") {
			fmt.Fprint(w, `{"data":{"issue":{"id":"uuid","identifier":"APP-1","team":{"id":"team"}}}}`)
			return
		}
		input = req.Variables["input"].(map[string]any)
		if req.Variables["id"] != "uuid" {
			t.Errorf("mutation identity: %v", req.Variables)
		}
		fmt.Fprint(w, `{"data":{"issueUpdate":{"success":false}}}`)
	})
	title := "Updated"
	if err := c.UpdateLinearIssue("APP-1", &title, nil, nil, nil, nil); err == nil {
		t.Fatal("unconfirmed mutation succeeded")
	}
	if !reflect.DeepEqual(input, map[string]any{"title": "Updated"}) {
		t.Fatalf("unexpected changed fields: %#v", input)
	}
}

func TestGraphQLErrorsRejectPartialData(t *testing.T) {
	c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"issues":{"nodes":[]}},"errors":[{"message":"linear-secret"}]}`)
	})
	if _, err := c.SyncFromLinear("APP"); err == nil || strings.Contains(err.Error(), "linear-secret") {
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
		merged bool
	}{
		{"open wins", `[{"html_url":"https://forge/pull/2","state":"closed","merged_at":"2026-01-01T00:00:00Z","head":{"ref":"ticket","sha":"old"}},{"html_url":"https://forge/pull/3","state":"open","head":{"ref":"ticket","sha":"tip"}}]`, "https://forge/pull/3", false},
		// The human merge boundary must not strand the task before reviewed.
		{"merged accepted", `[{"html_url":"https://forge/pull/2","state":"closed","merged_at":"2026-01-01T00:00:00Z","head":{"ref":"ticket","sha":"tip"}}]`, "https://forge/pull/2", true},
		{"closed unmerged rejected", `[{"html_url":"https://forge/pull/2","state":"closed","head":{"ref":"ticket","sha":"tip"}}]`, "", false},
		{"ambiguous merged rejected", `[{"html_url":"https://forge/pull/2","state":"closed","merged_at":"2026-01-01T00:00:00Z","head":{"ref":"ticket","sha":"a"}},{"html_url":"https://forge/pull/4","state":"closed","merged_at":"2026-01-02T00:00:00Z","head":{"ref":"ticket","sha":"b"}}]`, "", false},
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
			if err != nil || pr.URL != tc.url || pr.Merged != tc.merged || pr.Open == tc.merged || pr.SHA != "tip" {
				t.Fatalf("%+v %v", pr, err)
			}
		})
	}
}
