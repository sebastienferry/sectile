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
	"time"
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
	tasks, err := c.SyncFromGithub("acme/app", "/nonexistent/server/checkout", 0)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(tasks) != 2 || tasks[0].Key != "#1" || tasks[0].Status != models.StatusToImplement || tasks[0].Assignee != "owner" || tasks[0].Sprint != "Sprint 1" || tasks[1].Status != models.StatusFinished {
		t.Fatalf("unexpected imported issues: %#v (%d calls)", tasks, calls)
	}
}

// GitHub filters the same read on the same idea under another name: `since`
// takes the instant, in UTC, which is the timezone its own `updated_at` is
// written in. A background pass asks for what moved; a read somebody asked for
// carries no parameter at all.
func TestGithubSyncBoundsAnIncrementalPassOnTheUpdateDate(t *testing.T) {
	var since string
	c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		since = r.URL.Query().Get("since")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"number":1,"title":"Moved","state":"open"}]`)
	})
	before := time.Now().UTC()
	if _, err := c.SyncFromGithub("acme/app", "", 20); err != nil {
		t.Fatal(err)
	}
	stamp, err := time.Parse(time.RFC3339, since)
	if err != nil {
		t.Fatalf("an incremental pass must carry a since parameter, got %q (%v)", since, err)
	}
	// Twenty minutes back from the call, with a second of slack for the clock
	// that moved between the two.
	if delta := before.Add(-20 * time.Minute).Sub(stamp); delta > time.Second || delta < -time.Second {
		t.Fatalf("since must be the window back from now, got %s (%s off)", stamp, delta)
	}

	since = "untouched"
	if _, err := c.SyncFromGithub("acme/app", "", 0); err != nil {
		t.Fatal(err)
	}
	if since != "" {
		t.Fatalf("a full read carries no since, got %q", since)
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
			if _, err := c.SyncFromGithub("acme/app", "", 0); err == nil || strings.Contains(err.Error(), "github-secret") {
				t.Fatalf("unsafe or missing error: %v", err)
			}
		})
	}
	calls := 0
	c := fixture(t, func(w http.ResponseWriter, r *http.Request) { calls++; fmt.Fprint(w, `[]`) })
	c.GithubToken = ""
	if _, err := c.SyncFromGithub("acme/app", "", 0); err == nil || calls != 0 {
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
		if _, err := c.SyncFromGithub("acme/app", "", 0); err == nil {
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

// Each provider reads its own variable and nothing else (#464): the generic
// name and the providers' conventions used to hand one provider's credential to
// another on a server driving several trackers.
func TestNewClientReadsOnlyEachProvidersOwnVariable(t *testing.T) {
	all := []string{"SECTILE_TRACKER_TOKEN", "GH_TOKEN", "GITHUB_TOKEN", "JIRA_API_TOKEN", "GITLAB_TOKEN",
		GithubTokenVar, GitlabTokenVar, JiraEmailVar, JiraTokenVar}
	cases := []struct {
		name                             string
		env                              map[string]string
		wantGithub, wantGitlab, wantJira string
	}{
		{
			name: "removed variables serve no provider",
			env: map[string]string{"SECTILE_TRACKER_TOKEN": "generic", "GH_TOKEN": "gh-cli", "GITHUB_TOKEN": "ci",
				"JIRA_API_TOKEN": "jira-conv", "GITLAB_TOKEN": "gl-conv"},
		},
		{
			name:       "each provider reads its own variable",
			env:        map[string]string{GithubTokenVar: "gh", GitlabTokenVar: "gl", JiraTokenVar: "jira"},
			wantGithub: "gh", wantGitlab: "gl", wantJira: "jira",
		},
		{
			name:       "one provider's variable reaches no other",
			env:        map[string]string{GithubTokenVar: "gh", "SECTILE_TRACKER_TOKEN": "generic"},
			wantGithub: "gh",
		},
		{name: "no credential at all"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			for _, name := range all {
				t.Setenv(name, "")
			}
			for name, value := range testCase.env {
				t.Setenv(name, value)
			}
			c := NewClient()
			if c.GithubToken != testCase.wantGithub || c.GitlabToken != testCase.wantGitlab || c.JiraToken != testCase.wantJira {
				t.Errorf("got github %q gitlab %q jira %q, want %q %q %q",
					c.GithubToken, c.GitlabToken, c.JiraToken, testCase.wantGithub, testCase.wantGitlab, testCase.wantJira)
			}
		})
	}
}

func TestRemovedTokenVariablesAreWarnedAboutOnlyWhenSet(t *testing.T) {
	env := map[string]string{"SECTILE_TRACKER_TOKEN": "x", "JIRA_API_TOKEN": "y", "GH_TOKEN": "  "}
	warnings := RemovedTokenVariableWarnings(func(name string) string { return env[name] })
	if len(warnings) != 2 {
		t.Fatalf("want one warning per set variable, got %q", warnings)
	}
	if !strings.HasPrefix(warnings[0], "SECTILE_TRACKER_TOKEN ") || !strings.Contains(warnings[0], GithubTokenVar) {
		t.Errorf("the generic variable's warning must name its replacements: %q", warnings[0])
	}
	if !strings.HasPrefix(warnings[1], "JIRA_API_TOKEN ") || !strings.Contains(warnings[1], JiraTokenVar) {
		t.Errorf("JIRA_API_TOKEN's warning must name %s: %q", JiraTokenVar, warnings[1])
	}
	if got := RemovedTokenVariableWarnings(func(string) string { return "" }); len(got) != 0 {
		t.Fatalf("nothing set, nothing to warn about: %q", got)
	}
}

// A call made for nobody, the synchronisation, only ever uses the server
// credential, so a missing one names the variable and the Administration page.
func TestMissingServerCredentialNamesTheVariableAndTheAdministrationPage(t *testing.T) {
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
		if !strings.Contains(err.Error(), GithubTokenVar) || !strings.Contains(err.Error(), "Administration") {
			t.Errorf("%s: %q must name %s and Administration", call.name, err, GithubTokenVar)
		}
		if strings.Contains(err.Error(), "Profile") {
			t.Errorf("%s: a synchronisation has nobody's profile to point at: %q", call.name, err)
		}
	}
	jira := &Client{JiraURL: "https://acme.atlassian.net", JiraToken: "t"}
	if err := jira.jiraConfigured(); err == nil || !strings.Contains(err.Error(), JiraEmailVar) {
		t.Errorf("half of the Jira server pair is none at all: %v", err)
	}
}

// A call made for somebody still points at the credential they can set: their
// own (ADR 0014).
func TestMissingCredentialForAPersonPointsAtTheirProfile(t *testing.T) {
	c := &Client{HTTP: http.DefaultClient, actingUser: "u1"}
	err := c.github(context.Background(), http.MethodGet, "/rate_limit", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "Profile") || strings.Contains(err.Error(), GithubTokenVar) {
		t.Fatalf("a person's missing credential must point at their profile: %v", err)
	}
	jira := &Client{JiraURL: "https://acme.atlassian.net", JiraToken: "t", actingUser: "u1"}
	if err := jira.jiraConfigured(); err == nil || err.Error() != "configure the Jira account e-mail" {
		t.Fatalf("a personal Jira token without its e-mail keeps its own message: %v", err)
	}
}

// A stored server credential the key does not open fails the call, even with
// an environment token at hand: the admin believes the stored one is in use.
func TestAnUnreadableServerCredentialIsNotReplacedByTheEnvironment(t *testing.T) {
	c := &Client{HTTP: http.DefaultClient, GithubToken: "env-token", JiraURL: "https://acme.atlassian.net", JiraEmail: "env@acme", JiraToken: "env-jira"}
	c.Resolve = func(string) Credentials { return Credentials{GithubUnreadable: true, JiraUnreadable: true} }
	resolved := c.For("p1")
	if resolved.GithubToken != "" || resolved.JiraToken != "" || resolved.JiraEmail != "" {
		t.Fatalf("the environment credential survived an unreadable stored one: %+v", resolved)
	}
	err := resolved.github(context.Background(), http.MethodGet, "/rate_limit", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "cannot be decrypted") {
		t.Fatalf("github: %v", err)
	}
	if err := resolved.jiraConfigured(); err == nil || !strings.Contains(err.Error(), "cannot be decrypted") {
		t.Fatalf("jira: %v", err)
	}
}

// ForActingUser must never mark the shared client: the next request, made for
// nobody, would report its missing credential as somebody's.
func TestForActingUserDoesNotMarkTheSharedClient(t *testing.T) {
	c := &Client{HTTP: http.DefaultClient}
	c.ResolveUser = func(string, string) (string, string, string, error) { return "", "", "", nil }
	c.Resolve = func(string) Credentials { return Credentials{} }
	resolved, personal, err := c.ForActingUser("u1", "github", "p1")
	if err != nil || personal {
		t.Fatalf("%v %v", personal, err)
	}
	if resolved == c || c.actingUser != "" || resolved.actingUser != "u1" {
		t.Fatalf("the shared client was marked: shared %q resolved %q", c.actingUser, resolved.actingUser)
	}
}

func TestGithubSyncMapsCreatorAndAvatar(t *testing.T) {
	c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{
			"number": 42,
			"title": "Bug report",
			"body": "Something is broken",
			"state": "open",
			"user": {
				"login": "octocat",
				"avatar_url": "https://avatars.githubusercontent.com/u/1"
			},
			"assignees": [{"login": "dev"}]
		}]`)
	})
	tasks, err := c.SyncFromGithub("acme/app", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1", len(tasks))
	}
	if tasks[0].Creator != "octocat" {
		t.Errorf("task.Creator = %q, want %q", tasks[0].Creator, "octocat")
	}
	if tasks[0].CreatorAvatar != "https://avatars.githubusercontent.com/u/1" {
		t.Errorf("task.CreatorAvatar = %q, want %q", tasks[0].CreatorAvatar, "https://avatars.githubusercontent.com/u/1")
	}
	if tasks[0].Assignee != "dev" {
		t.Errorf("task.Assignee = %q, want %q", tasks[0].Assignee, "dev")
	}
}
