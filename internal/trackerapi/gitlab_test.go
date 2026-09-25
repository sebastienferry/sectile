package trackerapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// gitlabSite is a fake GitLab instance, self-managed on purpose: the REST API
// under /api/v4, GraphQL under /api/graphql. Routes are keyed by
// "METHOD <escaped path>", so a project path that is not encoded as one
// segment does not match. Every request is recorded; the token is checked.
type gitlabSite struct {
	t        *testing.T
	mu       sync.Mutex
	routes   map[string]http.HandlerFunc
	graphql  func(query string, vars map[string]any) (int, string)
	requests []recordedRequest
	server   *httptest.Server
}

func newGitlabSite(t *testing.T) *gitlabSite {
	t.Helper()
	site := &gitlabSite{t: t, routes: map[string]http.HandlerFunc{}}
	site.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		site.mu.Lock()
		site.requests = append(site.requests, recordedRequest{r.Method, r.URL.EscapedPath(), r.URL.RawQuery, string(body)})
		site.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer gl-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"message":"401 Unauthorized"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && r.URL.Path == "/api/graphql" {
			var req struct {
				Query     string         `json:"query"`
				Variables map[string]any `json:"variables"`
			}
			_ = json.Unmarshal(body, &req)
			if site.graphql == nil {
				fmt.Fprint(w, `{"errors":[{"message":"Field 'iterationCadences' doesn't exist on type 'Group'"}]}`)
				return
			}
			status, answer := site.graphql(req.Query, req.Variables)
			w.WriteHeader(status)
			fmt.Fprint(w, answer)
			return
		}
		if handler, ok := site.routes[r.Method+" "+r.URL.EscapedPath()]; ok {
			r.Body = io.NopCloser(strings.NewReader(string(body)))
			handler(w, r)
			return
		}
		t.Errorf("unexpected request %s %s", r.Method, r.URL.EscapedPath())
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"404 Not Found"}`)
	}))
	t.Cleanup(site.server.Close)
	return site
}

func (s *gitlabSite) on(method, path string, handler http.HandlerFunc) {
	s.routes[method+" /api/v4"+path] = handler
}

func (s *gitlabSite) json(method, path, body string) {
	s.on(method, path, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) })
}

func (s *gitlabSite) client() *Client {
	return &Client{HTTP: s.server.Client(), GitlabURL: s.server.URL + "/api/v4", GitlabProject: "acme/app", GitlabToken: "gl-secret"}
}

func (s *gitlabSite) adapter() *GitlabAdapter { return NewGitlabAdapter(s.client()) }

func (s *gitlabSite) recorded(method, path string) []recordedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []recordedRequest
	for _, r := range s.requests {
		if r.Method == method && r.Path == "/api/v4"+path {
			out = append(out, r)
		}
	}
	return out
}

func (s *gitlabSite) graphqlCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, r := range s.requests {
		if r.Path == "/api/graphql" {
			n++
		}
	}
	return n
}

func decodeBody(t *testing.T, r recordedRequest) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal([]byte(r.Body), &body); err != nil {
		t.Fatalf("request body %q is not JSON: %v", r.Body, err)
	}
	return body
}

const gitlabNoBoards = `[]`

// Client plumbing.

func TestGitlabPagesFollowsNextPageAndEncodesSubgroups(t *testing.T) {
	site := newGitlabSite(t)
	site.on("GET", "/projects/acme%2Fplatform%2Fapp/issues", func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		if r.URL.Query().Get("per_page") != "100" {
			t.Errorf("per_page = %q", r.URL.Query().Get("per_page"))
		}
		next := map[string]string{"1": "2", "2": "3", "3": ""}[page]
		w.Header().Set("X-Next-Page", next)
		fmt.Fprintf(w, `[{"iid":%s0,"title":"t","state":"opened"}]`, page)
	})
	c := site.client()
	c.GitlabProject = "acme/platform/app"
	items, err := c.gitlabPages(context.Background(), "/projects/"+gitlabProjectSegment("acme/platform/app")+"/issues", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("three pages read, got %d items", len(items))
	}
}

func TestGitlabGraphQLEndpointFollowsTheInstance(t *testing.T) {
	c := &Client{GitlabURL: "https://gitlab.example.org/api/v4/"}
	if got := c.gitlabGraphQLEndpoint(); got != "https://gitlab.example.org/api/graphql" {
		t.Fatalf("GraphQL endpoint = %q", got)
	}
}

func TestGitlabErrorsQuoteGitLabInFrenchWithoutTheToken(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
		want   string
	}{
		"string":  {http.StatusNotFound, `{"message":"404 Project Not Found"}`, "projet ou ticket GitLab introuvable : 404 Project Not Found"},
		"list":    {http.StatusBadRequest, `{"message":["title is missing","labels is invalid"]}`, "GitLab a refusé la requête (HTTP 400) : title is missing | labels is invalid"},
		"byField": {http.StatusBadRequest, `{"message":{"title":["already being used"],"due_date":["is invalid"]}}`, "due_date : is invalid | title : already being used"},
		"error":   {http.StatusForbidden, `{"error":"insufficient_scope","error_description":"The request requires higher privileges"}`, "accès refusé par GitLab : The request requires higher privileges"},
		"401":     {http.StatusUnauthorized, `{"message":"401 Unauthorized"}`, "jeton GitLab refusé"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			site := newGitlabSite(t)
			site.on("GET", "/projects/acme%2Fapp/issues/1", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			})
			_, err := site.adapter().GetIssue(context.Background(), tracker.GetIssueRequest{Key: "#1"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.want)
			}
			if strings.Contains(err.Error(), "gl-secret") {
				t.Fatalf("the token leaked into the error: %v", err)
			}
		})
	}
}

func TestGitlabRateLimitIsRecognised(t *testing.T) {
	site := newGitlabSite(t)
	site.on("GET", "/projects/acme%2Fapp/issues", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"message":"Retry later"}`)
	})
	_, err := site.adapter().SyncIssues(context.Background(), tracker.SyncRequest{})
	if !IsRateLimited(err) {
		t.Fatalf("a 429 is a rate limit the loop backs off from: %v", err)
	}
}

func TestGitlabMissingConfigurationIsNamed(t *testing.T) {
	c := &Client{GitlabURL: "https://gitlab.com/api/v4", GitlabProject: "acme/app"}
	_, err := NewGitlabAdapter(c).SyncIssues(context.Background(), tracker.SyncRequest{})
	if err == nil || !strings.Contains(err.Error(), "GitLab") || !strings.Contains(err.Error(), GitlabTokenVar) {
		t.Fatalf("a missing server token names GitLab and its variable: %v", err)
	}
	c = &Client{GitlabURL: "https://gitlab.com/api/v4", GitlabToken: "t"}
	_, err = NewGitlabAdapter(c).SyncIssues(context.Background(), tracker.SyncRequest{})
	if err == nil || !strings.Contains(err.Error(), "projet GitLab") {
		t.Fatalf("a missing project path is named: %v", err)
	}
}

// Mapping.

func TestGitlabTaskMapping(t *testing.T) {
	stages := map[string]models.Status{
		"#new": models.StatusToClarify, "#clarified": models.StatusClarified, "#specified": models.StatusToImplement,
		"#implemented": models.StatusToTest, "#reviewed": models.StatusToClose, "#finished": models.StatusFinished,
	}
	for label, want := range stages {
		task, err := gitlabTask(GitlabIssueItem{IID: 3, State: "opened", Labels: []string{"bug", label}}, nil)
		if err != nil || task.Status != want {
			t.Errorf("%s: status %v, want %v (%v)", label, task.Status, want, err)
		}
	}
	task, _ := gitlabTask(GitlabIssueItem{IID: 3, State: "opened"}, nil)
	if task.Status != models.StatusToClarify {
		t.Errorf("no stage label is new: %v", task.Status)
	}
	task, _ = gitlabTask(GitlabIssueItem{IID: 3, State: "closed", Labels: []string{"#clarified"}}, nil)
	if task.Status != models.StatusFinished || task.TrackerStatus != "closed" {
		t.Errorf("closed is finished whatever the labels: %v %q", task.Status, task.TrackerStatus)
	}

	task, err := gitlabTask(GitlabIssueItem{
		IID: 12, State: "opened", Title: "T", Description: "D", WebURL: "https://gitlab.example.org/acme/app/-/issues/12",
		Labels:    []string{"macro:Core features", "parent:M-8", "team::platform", "team::data", "Doing"},
		Author:    &gitlabUser{Username: "ada", AvatarURL: "https://a/ada.png"},
		Assignees: []gitlabUser{{ID: 7, Username: "grace"}},
		Milestone: &gitlabMilestone{ID: 4, Title: "Release 2"},
		IssueType: "incident",
	}, map[string]bool{"Doing": true, "To Do": true})
	if err != nil {
		t.Fatal(err)
	}
	if task.Key != "#12" || task.Source != "gitlab" || *task.ExternalURL != "https://gitlab.example.org/acme/app/-/issues/12" {
		t.Errorf("identity: %q %q %v", task.Key, task.Source, task.ExternalURL)
	}
	if task.ParentKey != "M-8" || task.ParentTitle != "Core features" || task.ParentType != "macro" {
		t.Errorf("macro from labels: %q %q %q", task.ParentKey, task.ParentTitle, task.ParentType)
	}
	if task.Team != "platform" || task.TeamID != "platform" {
		t.Errorf("the first team:: label is the team: %q", task.Team)
	}
	if task.Sprint != "Release 2" {
		t.Errorf("a milestone is a sprint whatever its title, never a macro: %q", task.Sprint)
	}
	if task.TrackerStatus != "Doing" || task.Assignee != "grace" || task.Creator != "ada" || task.IssueType != "incident" {
		t.Errorf("fields: %q %q %q %q", task.TrackerStatus, task.Assignee, task.Creator, task.IssueType)
	}

	task, _ = gitlabTask(GitlabIssueItem{IID: 5, State: "opened", Milestone: &gitlabMilestone{ID: 4, Title: "Release 2"}, Iteration: &gitlabRESTIterate{ID: 9, StartDate: "2026-09-01", DueDate: "2026-09-14"}}, nil)
	if task.Sprint != "Itération 2026-09-01 au 2026-09-14" || task.ParentKey != "" {
		t.Errorf("the iteration wins over the milestone, and is named by its dates without a title: %q", task.Sprint)
	}
}

func TestGitlabFormatTaskIDKeepsProjectsApart(t *testing.T) {
	g := NewGitlabAdapter(&Client{})
	a, b := g.FormatTaskID("p1", "#12", ""), g.FormatTaskID("p2", "#12", "")
	if a != "gl-p1-12" || b != "gl-p2-12" || a == NewGithubAdapter(&Client{}).FormatTaskID("p1", "#12", "") {
		t.Fatalf("ids: %q %q", a, b)
	}
	if got := g.FormatTaskID("", "#12", ""); got != "gl-12" {
		t.Fatalf("default project id: %q", got)
	}
}

// Registry and credentials.

func TestGitlabIsRegisteredWithItsCapabilities(t *testing.T) {
	ts, ok := NewDefaultRegistry(&Client{}).Get("gitlab")
	if !ok {
		t.Fatal("gitlab is registered")
	}
	for _, c := range []tracker.Capability{tracker.CapCreate, tracker.CapGet, tracker.CapUpdate, tracker.CapDelete, tracker.CapSync,
		tracker.CapIncrementalSync, tracker.CapComment, tracker.CapLabels, tracker.CapAssign, tracker.CapTransition,
		tracker.CapSprint, tracker.CapSprintManage, tracker.CapTeam, tracker.CapBoard, tracker.CapPullRequests} {
		if !ts.Supports(c) {
			t.Errorf("gitlab supports %s", c)
		}
	}
	if ts.Supports(tracker.CapEpic) {
		t.Error("gitlab declares no epics")
	}
	if _, ok := ts.(tracker.PullRequestDiscoverer); !ok {
		t.Error("gitlab discovers merge requests")
	}
	if _, ok := ts.(tracker.SprintManager); !ok {
		t.Error("gitlab manages sprints")
	}
}

func TestGitlabCredentials(t *testing.T) {
	site := newGitlabSite(t)
	var tokens []string
	site.on("GET", "/projects/acme%2Fapp/issues", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `[]`) })
	site.json("GET", "/projects/acme%2Fapp/boards", gitlabNoBoards)
	site.on("POST", "/projects/acme%2Fapp/issues/1/notes", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{}`) })
	c := site.client()
	c.GitlabToken = "server-token"
	c.ResolveUser = func(userID, trackerName string) (string, string, string, error) {
		switch userID {
		case "u-ada":
			return "", "", "gl-secret", nil
		case "u-locked":
			return "", "", "", errors.New("credential sealed and locked")
		}
		return "", "", "", nil
	}
	// The fake checks the token; record which one each call carried.
	base := site.server.Config.Handler
	site.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokens = append(tokens, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		r.Header.Set("Authorization", "Bearer gl-secret")
		base.ServeHTTP(w, r)
	})
	g := NewGitlabAdapter(c)

	if _, err := g.SyncIssues(context.Background(), tracker.SyncRequest{}); err != nil {
		t.Fatal(err)
	}
	if tokens[0] != "server-token" {
		t.Fatalf("the synchronisation reads with the server credential: %q", tokens[0])
	}
	tokens = nil
	if err := g.AddComment(tracker.WithActingUser(context.Background(), "u-ada"), tracker.AddCommentRequest{Key: "#1", Body: "hi"}); err != nil {
		t.Fatal(err)
	}
	if tokens[0] != "gl-secret" {
		t.Fatalf("a person's note is posted with their own token: %q", tokens[0])
	}
	err := g.AddComment(tracker.WithActingUser(context.Background(), "u-grace"), tracker.AddCommentRequest{Key: "#1", Body: "hi"})
	var missing *MissingPersonalCredentialError
	if !errors.As(err, &missing) {
		t.Fatalf("a person without a GitLab token writes nothing (#482): %v", err)
	}
	if err := g.AddComment(tracker.WithActingUser(context.Background(), "u-locked"), tracker.AddCommentRequest{Key: "#1", Body: "hi"}); err == nil {
		t.Fatal("a sealed personal token refuses the write")
	}
	if _, err := g.SyncIssues(tracker.WithActingUser(context.Background(), "u-locked"), tracker.SyncRequest{}); err == nil {
		t.Fatal("a sealed personal token refuses the read too")
	}
}

// Issues.

func TestGitlabSyncReadsTheWindowAndMapsIssues(t *testing.T) {
	site := newGitlabSite(t)
	var queries []url.Values
	site.on("GET", "/projects/acme%2Fapp/issues", func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query())
		fmt.Fprint(w, `[{"iid":1,"title":"a","state":"opened","labels":["#specified","To Do"],"web_url":"https://gl/acme/app/-/issues/1"},{"iid":2,"title":"b","state":"closed","labels":["#clarified"]}]`)
	})
	site.json("GET", "/projects/acme%2Fapp/boards", `[{"id":1,"name":"Dev","lists":[{"id":1,"position":0,"label":{"name":"To Do"}}]}]`)
	g := site.adapter()

	tasks, err := g.SyncIssues(context.Background(), tracker.SyncRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 || tasks[0].Status != models.StatusToImplement || tasks[0].TrackerStatus != "To Do" || tasks[1].Status != models.StatusFinished {
		t.Fatalf("tasks: %+v", tasks)
	}
	if queries[0].Get("updated_after") != "" || queries[0].Get("state") != "all" || queries[0].Get("scope") != "all" {
		t.Fatalf("a full read has no window and reads every state: %v", queries[0])
	}
	if _, err := g.SyncIssues(context.Background(), tracker.SyncRequest{UpdatedWithinMin: 15}); err != nil {
		t.Fatal(err)
	}
	since, err := time.Parse(time.RFC3339, queries[1].Get("updated_after"))
	if err != nil || time.Since(since) < 14*time.Minute || time.Since(since) > 16*time.Minute {
		t.Fatalf("an incremental read narrows to the window: %q", queries[1].Get("updated_after"))
	}
}

func TestGitlabCreateIssuePayload(t *testing.T) {
	site := newGitlabSite(t)
	site.json("GET", "/projects/acme%2Fapp/members/all", `[{"id":41,"username":"grace-h"},{"id":42,"username":"grace"}]`)
	site.json("POST", "/projects/acme%2Fapp/issues", `{"iid":77,"title":"New","state":"opened","labels":["bug","#new"],"web_url":"https://gl/acme/app/-/issues/77"}`)
	task, err := site.adapter().CreateIssue(unattended(), tracker.CreateIssueRequest{
		Title: "New", Description: "Body", Labels: []string{"bug", " bug "}, IssueType: "Incident", Assignee: "grace", Team: "platform", Sprint: "milestone:5",
	})
	if err != nil {
		t.Fatal(err)
	}
	if task.Key != "#77" || task.Source != "gitlab" {
		t.Fatalf("created task: %+v", task)
	}
	body := decodeBody(t, site.recorded("POST", "/projects/acme%2Fapp/issues")[0])
	if body["labels"] != "bug,#new,team::platform" || body["issue_type"] != "incident" || body["description"] != "Body" {
		t.Fatalf("payload: %v", body)
	}
	if ids, _ := body["assignee_ids"].([]any); len(ids) != 1 || ids[0] != float64(42) {
		t.Fatalf("the assignee is resolved by exact username: %v", body["assignee_ids"])
	}
	if body["milestone_id"] != float64(5) {
		t.Fatalf("a milestone sprint goes with the creation: %v", body["milestone_id"])
	}

	if _, err := site.adapter().CreateIssue(unattended(), tracker.CreateIssueRequest{Title: "x", IssueType: "epic"}); err == nil {
		t.Fatal("an issue type GitLab does not create is refused")
	}
}

func TestGitlabUpdateIssueMergesLabelsAndSwapsTheStage(t *testing.T) {
	site := newGitlabSite(t)
	site.on("PUT", "/projects/acme%2Fapp/issues/3", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{}`) })
	g := site.adapter()
	title := "Renamed"
	specified := models.StatusToImplement
	err := g.UpdateIssue(unattended(), tracker.UpdateIssueRequest{Key: "#3", Title: &title, Status: &specified, Labels: []string{"bug", "#specified"}, RemovedLabels: []string{"#clarified", "Clarified"}})
	if err != nil {
		t.Fatal(err)
	}
	body := decodeBody(t, site.recorded("PUT", "/projects/acme%2Fapp/issues/3")[0])
	if body["title"] != "Renamed" || body["add_labels"] != "bug,#specified" || body["state_event"] != "reopen" {
		t.Fatalf("payload: %v", body)
	}
	removed := strings.Split(body["remove_labels"].(string), ",")
	for _, stale := range []string{"#clarified", "#new", "#implemented", "#reviewed", "#finished"} {
		if !containsString(removed, stale) {
			t.Errorf("stale stage label %s is removed: %v", stale, removed)
		}
	}
	if containsString(removed, "#specified") || containsString(removed, "bug") {
		t.Errorf("the posed labels are not removed: %v", removed)
	}

	finished := models.StatusFinished
	if err := g.UpdateIssue(unattended(), tracker.UpdateIssueRequest{Key: "#3", Status: &finished, Labels: []string{"#finished"}}); err != nil {
		t.Fatal(err)
	}
	if body := decodeBody(t, site.recorded("PUT", "/projects/acme%2Fapp/issues/3")[1]); body["state_event"] != "close" {
		t.Fatalf("finishing closes the issue: %v", body)
	}
}

func TestGitlabDeleteFallsBackToClose(t *testing.T) {
	site := newGitlabSite(t)
	site.on("DELETE", "/projects/acme%2Fapp/issues/3", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message":"403 Forbidden"}`)
	})
	site.on("PUT", "/projects/acme%2Fapp/issues/3", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{}`) })
	if err := site.adapter().DeleteIssue(unattended(), tracker.DeleteIssueRequest{Key: "#3"}); err != nil {
		t.Fatal(err)
	}
	if body := decodeBody(t, site.recorded("PUT", "/projects/acme%2Fapp/issues/3")[0]); body["state_event"] != "close" {
		t.Fatalf("a refused deletion closes: %v", body)
	}
	if err := site.adapter().DeleteIssue(unattended(), tracker.DeleteIssueRequest{Key: "#3", CloseOnly: true}); err != nil {
		t.Fatal(err)
	}
	if len(site.recorded("DELETE", "/projects/acme%2Fapp/issues/3")) != 1 {
		t.Fatal("CloseOnly never deletes")
	}
}

func TestGitlabNotesRoundTripWithoutSystemNotes(t *testing.T) {
	site := newGitlabSite(t)
	site.on("POST", "/projects/acme%2Fapp/issues/3/notes", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"id":1}`) })
	site.on("GET", "/projects/acme%2Fapp/issues/3/notes", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("sort") != "asc" {
			t.Errorf("notes are read oldest first: %v", r.URL.Query())
		}
		fmt.Fprint(w, `[{"id":1,"body":"first","system":false,"author":{"username":"ada"},"created_at":"2026-09-01T10:00:00Z"},{"id":2,"body":"added ~bug label","system":true,"author":{"username":"ada"}},{"id":3,"body":"second","system":false,"author":{"username":"grace"}}]`)
	})
	g := site.adapter()
	if err := g.AddComment(unattended(), tracker.AddCommentRequest{Key: "#3", Body: "hello"}); err != nil {
		t.Fatal(err)
	}
	if body := decodeBody(t, site.recorded("POST", "/projects/acme%2Fapp/issues/3/notes")[0]); body["body"] != "hello" {
		t.Fatalf("note body: %v", body)
	}
	comments, err := g.GetComments(context.Background(), tracker.GetCommentsRequest{Key: "#3"})
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 2 || comments[0].Body != "first" || comments[1].Author != "grace" || comments[0].Source != "gitlab" {
		t.Fatalf("comments: %+v", comments)
	}
}

func TestGitlabAssign(t *testing.T) {
	site := newGitlabSite(t)
	site.json("GET", "/projects/acme%2Fapp/members/all", `[{"id":42,"username":"grace","name":"Grace H","state":"active","avatar_url":"https://a/g.png"}]`)
	site.on("PUT", "/projects/acme%2Fapp/issues/3", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{}`) })
	g := site.adapter()
	for i, tc := range []struct {
		person string
		want   float64
	}{{"42", 42}, {"grace", 42}, {"", 0}} {
		if err := g.Assign(unattended(), "#3", tc.person); err != nil {
			t.Fatal(err)
		}
		ids := decodeBody(t, site.recorded("PUT", "/projects/acme%2Fapp/issues/3")[i])["assignee_ids"].([]any)
		if len(ids) != 1 || ids[0] != tc.want {
			t.Errorf("assign %q: %v", tc.person, ids)
		}
	}
	if err := g.Assign(unattended(), "#3", "nobody"); err == nil || !strings.Contains(err.Error(), "aucun membre GitLab") {
		t.Fatalf("an unknown username is refused: %v", err)
	}
	people, err := g.SearchAssignable(context.Background(), "#3", "gra", 10)
	if err != nil || len(people) != 1 || people[0].ID != "42" || people[0].DisplayName != "Grace H" || !people[0].Active {
		t.Fatalf("people: %+v %v", people, err)
	}
}

func TestGitlabRelatedMergeRequestsOldestFirst(t *testing.T) {
	site := newGitlabSite(t)
	site.json("GET", "/projects/acme%2Fapp/issues/3/related_merge_requests", `[
		{"web_url":"https://gl/acme/app/-/merge_requests/9","source_branch":"feat/3-b","created_at":"2026-09-10T10:00:00Z"},
		{"web_url":"https://gl/acme/app/-/merge_requests/5","source_branch":"feat/3","created_at":"2026-09-01T10:00:00Z"}]`)
	var ts tracker.TicketingSystem = site.adapter()
	links, err := ts.(tracker.PullRequestDiscoverer).IssuePullRequests(context.Background(), tracker.IssuePullRequestsRequest{Key: "#3"})
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 2 || links[0].Branch != "feat/3" || links[1].URL != "https://gl/acme/app/-/merge_requests/9" {
		t.Fatalf("links: %+v", links)
	}
}

// Boards.

const gitlabTwoBoards = `[
	{"id":1,"name":"Development","lists":[{"id":11,"position":1,"label":{"name":"Doing"}},{"id":10,"position":0,"label":{"name":"To Do"}},{"id":12,"position":2,"label":null}]},
	{"id":2,"name":"Support","lists":[{"id":20,"position":0,"label":{"name":"Triage"}}]}]`

func TestGitlabBoardsColumnsAndStatuses(t *testing.T) {
	site := newGitlabSite(t)
	site.json("GET", "/projects/acme%2Fapp/boards", gitlabTwoBoards)
	g := site.adapter()
	boards, err := g.ListBoards(context.Background(), tracker.BoardsRequest{})
	if err != nil || len(boards) != 2 || boards[1].Name != "Support" || boards[0].ID != "1" {
		t.Fatalf("boards: %+v %v", boards, err)
	}
	columns, err := g.ListBoardColumns(context.Background(), tracker.BoardRequest{BoardID: "1"})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range columns {
		names = append(names, c.Name+"="+strings.Join(c.Statuses, "|"))
	}
	if strings.Join(names, ",") != "Open=opened,To Do=To Do,Doing=Doing,Closed=closed" {
		t.Fatalf("columns in board order, non-label lists skipped: %v", names)
	}
	statuses, err := g.ListStatuses(context.Background(), tracker.ProjectRequest{})
	if err != nil || len(statuses) != 5 || statuses[0].Name != "opened" || statuses[4].Category != "done" {
		t.Fatalf("statuses: %+v %v", statuses, err)
	}
	types, _ := g.ListIssueTypes(context.Background(), tracker.ProjectRequest{})
	if strings.Join(types, ",") != "issue,incident,task,test_case" {
		t.Fatalf("issue types: %v", types)
	}
	fields, err := g.RequiredCreateFields(context.Background(), tracker.CreateMetaRequest{})
	if err != nil || fields == nil || len(fields) != 0 {
		t.Fatalf("no extra field on creation: %v %v", fields, err)
	}
}

func TestGitlabProjectWithoutBoards(t *testing.T) {
	site := newGitlabSite(t)
	site.json("GET", "/projects/acme%2Fapp/boards", gitlabNoBoards)
	boards, err := site.adapter().ListBoards(context.Background(), tracker.BoardsRequest{})
	if err != nil || boards == nil || len(boards) != 0 {
		t.Fatalf("no board is an empty list: %v %v", boards, err)
	}
	if _, err := site.adapter().ListBoardColumns(context.Background(), tracker.BoardRequest{}); err == nil || !strings.Contains(err.Error(), "aucun board") {
		t.Fatalf("columns of a project without board say so: %v", err)
	}
}

func TestGitlabColumnMoves(t *testing.T) {
	site := newGitlabSite(t)
	site.json("GET", "/projects/acme%2Fapp/boards", gitlabTwoBoards)
	site.on("PUT", "/projects/acme%2Fapp/issues/3", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{}`) })
	g := site.adapter()
	cases := []struct {
		status             string
		add, remove, state string
	}{
		{"Doing", "Doing", "To Do,Triage", "reopen"},
		{"closed", "", "", "close"},
		{"To Do", "To Do", "Doing,Triage", "reopen"},
		{"opened", "", "Doing,To Do,Triage", "reopen"},
	}
	for i, tc := range cases {
		if err := g.Transition(unattended(), "#3", tc.status); err != nil {
			t.Fatal(err)
		}
		body := decodeBody(t, site.recorded("PUT", "/projects/acme%2Fapp/issues/3")[i])
		add, _ := body["add_labels"].(string)
		remove, _ := body["remove_labels"].(string)
		if add != tc.add || remove != tc.remove || body["state_event"] != tc.state {
			t.Errorf("move to %s: %v", tc.status, body)
		}
		if strings.Contains(add+remove, "#") {
			t.Errorf("a column move never touches the stage label: %v", body)
		}
	}
	if err := g.Transition(unattended(), "#3", "Nowhere"); err == nil || !strings.Contains(err.Error(), "colonne GitLab inconnue") {
		t.Fatalf("an unknown column is refused: %v", err)
	}
}

// Sprints.

type gitlabPremium struct {
	mu        sync.Mutex
	probes    int
	failNext  bool
	automatic bool
	gone      bool
	mutations []string
}

func (p *gitlabPremium) answer(query string, vars map[string]any) (int, string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch {
	case strings.Contains(query, "iterationCadences"):
		p.probes++
		if p.failNext {
			p.failNext = false
			return http.StatusBadGateway, `{"message":"502"}`
		}
		return 200, `{"data":{"group":{"iterationCadences":{"nodes":[{"id":"gid://gitlab/Iterations::Cadence/1"}]}}}}`
	case strings.Contains(query, "iterations(includeAncestors"):
		return 200, `{"data":{"group":{"iterations":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[
			{"id":"gid://gitlab/Iteration/31","title":"Iteration 7","state":"current","startDate":"2026-09-20","dueDate":"2026-10-03","iterationCadence":{"id":"c1","automatic":false}},
			{"id":"gid://gitlab/Iteration/32","title":null,"state":"upcoming","startDate":"2026-10-04","dueDate":"2026-10-17","iterationCadence":{"id":"c2","automatic":true}}]}}}}`
	case strings.Contains(query, "iteration(id:"):
		if p.gone {
			return 200, `{"data":{"iteration":null}}`
		}
		return 200, fmt.Sprintf(`{"data":{"iteration":{"id":"%s","title":"Iteration 7","state":"current","startDate":"2026-09-20","dueDate":"2026-10-03","iterationCadence":{"id":"c1","automatic":%v}}}}`, vars["id"], p.automatic)
	case strings.Contains(query, "issueSetIteration"):
		input, _ := json.Marshal(vars["input"])
		p.mutations = append(p.mutations, "set "+string(input))
		return 200, `{"data":{"issueSetIteration":{"errors":[]}}}`
	case strings.Contains(query, "updateIteration"):
		input, _ := json.Marshal(vars["input"])
		p.mutations = append(p.mutations, "update "+string(input))
		return 200, `{"data":{"updateIteration":{"errors":[],"iteration":{"id":"gid://gitlab/Iteration/31","title":"Renamed","state":"current","startDate":"2026-09-20","dueDate":"2026-10-03"}}}}`
	case strings.Contains(query, "iterationDelete"):
		p.mutations = append(p.mutations, "delete")
		return 200, `{"data":{"iterationDelete":{"errors":[]}}}`
	}
	return 200, `{"errors":[{"message":"unexpected query"}]}`
}

const gitlabMilestones = `[
	{"id":4,"title":"Sprint 12","state":"active","start_date":"2020-01-01","due_date":"2020-01-14"},
	{"id":5,"title":"Release 3","state":"active","start_date":"2999-01-01","due_date":"2999-02-01"},
	{"id":6,"title":"Old","state":"closed"}]`

func TestGitlabListSprintsOnPremium(t *testing.T) {
	site := newGitlabSite(t)
	premium := &gitlabPremium{}
	site.graphql = premium.answer
	site.json("GET", "/projects/acme%2Fapp/milestones", gitlabMilestones)
	g := site.adapter()
	sprints, err := g.ListSprints(context.Background(), tracker.BoardRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range sprints {
		got = append(got, s.ID+"="+s.Name+"/"+s.State)
	}
	want := "milestone:4=Sprint 12/active,milestone:5=Release 3/future,milestone:6=Old/closed,iteration:31=Iteration 7/active,iteration:32=Itération 2026-10-04 au 2026-10-17/future"
	if strings.Join(got, ",") != want {
		t.Fatalf("sprints:\n got %s\nwant %s", strings.Join(got, ","), want)
	}
	if q := site.recorded("GET", "/projects/acme%2Fapp/milestones")[0].Query; !strings.Contains(q, "include_ancestors=true") {
		t.Fatalf("group milestones are included: %s", q)
	}
	if _, err := g.ListSprints(context.Background(), tracker.BoardRequest{}); err != nil {
		t.Fatal(err)
	}
	if premium.probes != 1 {
		t.Fatalf("the tier is probed once and cached: %d probes", premium.probes)
	}
}

func TestGitlabListSprintsOnFree(t *testing.T) {
	site := newGitlabSite(t) // graphql nil: the field does not exist
	site.json("GET", "/projects/acme%2Fapp/milestones", gitlabMilestones)
	sprints, err := site.adapter().ListSprints(context.Background(), tracker.BoardRequest{})
	if err != nil || len(sprints) != 3 {
		t.Fatalf("a Free instance lists its milestones and reports no error: %v %v", sprints, err)
	}
}

func TestGitlabTierProbeRetriesAfterATransportFailure(t *testing.T) {
	site := newGitlabSite(t)
	premium := &gitlabPremium{failNext: true}
	site.graphql = premium.answer
	g := site.adapter()
	c := site.client()
	if g.iterationsAvailable(context.Background(), c, "acme") {
		t.Fatal("a failed probe answers no for this call")
	}
	if !g.iterationsAvailable(context.Background(), c, "acme") {
		t.Fatal("and is not remembered: the next call asks again")
	}
	g.tiers.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	g.iterationsAvailable(context.Background(), c, "acme")
	if premium.probes != 3 {
		t.Fatalf("an answer expires after its TTL: %d probes", premium.probes)
	}
}

func TestGitlabSetSprint(t *testing.T) {
	site := newGitlabSite(t)
	premium := &gitlabPremium{}
	site.graphql = premium.answer
	site.on("PUT", "/projects/acme%2Fapp/issues/3", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{}`) })
	site.on("PUT", "/projects/acme%2Fapp/issues/4", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{}`) })
	g := site.adapter()

	if err := g.SetSprint(unattended(), "milestone:4", []string{"#3", "#4"}); err != nil {
		t.Fatal(err)
	}
	if body := decodeBody(t, site.recorded("PUT", "/projects/acme%2Fapp/issues/4")[0]); body["milestone_id"] != float64(4) {
		t.Fatalf("milestone set: %v", body)
	}
	if len(premium.mutations) != 2 || !strings.Contains(premium.mutations[0], `"iterationId":null`) {
		t.Fatalf("moving into a milestone clears the iteration: %v", premium.mutations)
	}

	premium.mutations = nil
	if err := g.SetSprint(unattended(), "iteration:31", []string{"#3"}); err != nil {
		t.Fatal(err)
	}
	if body := decodeBody(t, site.recorded("PUT", "/projects/acme%2Fapp/issues/3")[1]); body["milestone_id"] != float64(0) {
		t.Fatalf("moving into an iteration clears the milestone: %v", body)
	}
	if len(premium.mutations) != 1 || !strings.Contains(premium.mutations[0], `"iterationId":"gid://gitlab/Iteration/31"`) || !strings.Contains(premium.mutations[0], `"projectPath":"acme/app"`) {
		t.Fatalf("iteration set through GraphQL: %v", premium.mutations)
	}

	premium.mutations = nil
	if err := g.SetSprint(unattended(), "", []string{"#3"}); err != nil {
		t.Fatal(err)
	}
	if body := decodeBody(t, site.recorded("PUT", "/projects/acme%2Fapp/issues/3")[2]); body["milestone_id"] != float64(0) || len(premium.mutations) != 1 {
		t.Fatalf("out of any sprint clears both: %v %v", body, premium.mutations)
	}

	if err := g.SetSprint(unattended(), "12", []string{"#3"}); err == nil || !strings.Contains(err.Error(), "illisible") {
		t.Fatalf("a sprint id without its kind is refused, not guessed: %v", err)
	}
}

func TestGitlabIterationWritesOnFreeAreUnsupported(t *testing.T) {
	site := newGitlabSite(t)
	site.on("PUT", "/projects/acme%2Fapp/issues/3", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{}`) })
	g := site.adapter()
	err := g.SetSprint(unattended(), "iteration:31", []string{"#3"})
	if !tracker.IsUnsupported(err) || !strings.Contains(err.Error(), "itérations ne sont pas disponibles") {
		t.Fatalf("an iteration write on Free is refused as unsupported: %v", err)
	}
	if len(site.recorded("PUT", "/projects/acme%2Fapp/issues/3")) != 0 {
		t.Fatal("and nothing is sent as a milestone")
	}
	if err := g.SetSprint(unattended(), "milestone:4", []string{"#3"}); err != nil {
		t.Fatalf("a milestone write on Free works: %v", err)
	}
	if _, err := g.UpdateSprint(unattended(), nil, "iteration:31", models.SprintPatch{Name: strPtr("x")}); !tracker.IsUnsupported(err) {
		t.Fatalf("updating an iteration on Free is unsupported: %v", err)
	}
}

func strPtr(s string) *string { return &s }

func TestGitlabMilestoneManagement(t *testing.T) {
	site := newGitlabSite(t)
	site.json("POST", "/projects/acme%2Fapp/milestones", `{"id":8,"title":"Sprint 13","state":"active","start_date":"2999-03-01","due_date":"2999-03-14"}`)
	site.json("PUT", "/projects/acme%2Fapp/milestones/8", `{"id":8,"title":"Sprint 13b","state":"closed"}`)
	site.on("DELETE", "/projects/acme%2Fapp/milestones/8", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	site.on("DELETE", "/projects/acme%2Fapp/milestones/9", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"404 Not found"}`)
	})
	g := site.adapter()
	start := time.Date(2999, 3, 1, 9, 0, 0, 0, time.UTC)
	created, err := g.CreateSprint(unattended(), tracker.SprintCreateRequest{Name: "Sprint 13", Start: start, End: start.AddDate(0, 0, 14).Add(-time.Second)})
	if err != nil || created.ID != "milestone:8" || created.State != "future" {
		t.Fatalf("created: %+v %v", created, err)
	}
	if body := decodeBody(t, site.recorded("POST", "/projects/acme%2Fapp/milestones")[0]); body["start_date"] != "2999-03-01" || body["due_date"] != "2999-03-14" {
		t.Fatalf("dates as days: %v", body)
	}
	updated, err := g.UpdateSprint(unattended(), nil, "milestone:8", models.SprintPatch{Name: strPtr("Sprint 13b"), End: strPtr("2999-03-15"), State: strPtr("closed")})
	if err != nil || updated.State != "closed" {
		t.Fatalf("updated: %+v %v", updated, err)
	}
	if body := decodeBody(t, site.recorded("PUT", "/projects/acme%2Fapp/milestones/8")[0]); body["title"] != "Sprint 13b" || body["due_date"] != "2999-03-15" || body["state_event"] != "close" {
		t.Fatalf("update payload: %v", body)
	}
	if err := g.DeleteSprint(unattended(), nil, "milestone:8"); err != nil {
		t.Fatal(err)
	}
	if err := g.DeleteSprint(unattended(), nil, "milestone:9"); err != nil {
		t.Fatalf("a milestone GitLab no longer knows counts as deleted: %v", err)
	}
}

func TestGitlabIterationManagement(t *testing.T) {
	site := newGitlabSite(t)
	premium := &gitlabPremium{}
	site.graphql = premium.answer
	g := site.adapter()
	updated, err := g.UpdateSprint(unattended(), nil, "iteration:31", models.SprintPatch{Name: strPtr("Renamed"), Start: strPtr("2026-09-21")})
	if err != nil || updated.Name != "Renamed" || updated.ID != "iteration:31" {
		t.Fatalf("manual iteration updated: %+v %v", updated, err)
	}
	if m := premium.mutations[0]; !strings.Contains(m, `"groupPath":"acme"`) || !strings.Contains(m, `"startDate":"2026-09-21"`) || !strings.Contains(m, `"title":"Renamed"`) {
		t.Fatalf("update input: %s", m)
	}
	if _, err := g.UpdateSprint(unattended(), nil, "iteration:31", models.SprintPatch{State: strPtr("closed")}); err == nil {
		t.Fatal("an iteration's state follows its dates and is not set by hand")
	}
	if err := g.DeleteSprint(unattended(), nil, "iteration:31"); err != nil || premium.mutations[len(premium.mutations)-1] != "delete" {
		t.Fatalf("manual iteration deleted: %v %v", err, premium.mutations)
	}

	premium.automatic = true
	if _, err := g.UpdateSprint(unattended(), nil, "iteration:31", models.SprintPatch{Name: strPtr("x")}); err == nil || !strings.Contains(err.Error(), "planifiée automatiquement") {
		t.Fatalf("an automatic cadence's iteration is refused: %v", err)
	}
	if err := g.DeleteSprint(unattended(), nil, "iteration:31"); err == nil || !strings.Contains(err.Error(), "planifiée automatiquement") {
		t.Fatalf("and cannot be deleted: %v", err)
	}

	premium.gone = true
	if err := g.DeleteSprint(unattended(), nil, "iteration:31"); err != nil {
		t.Fatalf("an iteration GitLab no longer knows counts as deleted: %v", err)
	}
}

func TestGitlabNumericProjectResolvesItsGroup(t *testing.T) {
	site := newGitlabSite(t)
	site.json("GET", "/projects/1234", `{"path_with_namespace":"acme/platform/app","namespace":{"kind":"group","full_path":"acme/platform"}}`)
	loc, err := site.client().gitlabLocate(context.Background(), "1234")
	if err != nil || loc.group != "acme/platform" || loc.fullPath != "acme/platform/app" {
		t.Fatalf("numeric project: %+v %v", loc, err)
	}
	site.json("GET", "/projects/99", `{"path_with_namespace":"alice/app","namespace":{"kind":"user","full_path":"alice"}}`)
	if loc, _ := site.client().gitlabLocate(context.Background(), "99"); loc.group != "" {
		t.Fatalf("a personal namespace has no group: %+v", loc)
	}
}

// Teams.

func TestGitlabTeams(t *testing.T) {
	site := newGitlabSite(t)
	site.json("GET", "/projects/acme%2Fapp/labels", `[{"name":"team::platform"},{"name":"team::data"},{"name":"teamwork"},{"name":"team::Data"}]`)
	site.json("GET", "/projects/acme%2Fapp/issues/3", `{"iid":3,"state":"opened","labels":["bug","team::data"]}`)
	site.on("PUT", "/projects/acme%2Fapp/issues/3", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{}`) })
	site.json("GET", "/projects/acme%2Fapp/members/all", `[{"id":42,"username":"grace","name":"Grace H","state":"active"},{"id":43,"username":"bob","state":"blocked"}]`)
	g := site.adapter()

	teams, err := g.SearchTeams(context.Background(), tracker.TeamSearchRequest{})
	if err != nil || len(teams) != 2 || teams[0].Name != "data" || teams[1].ID != "platform" {
		t.Fatalf("teams: %+v %v", teams, err)
	}
	teams, _ = g.SearchTeams(context.Background(), tracker.TeamSearchRequest{Query: "plat"})
	if len(teams) != 1 || teams[0].Name != "platform" {
		t.Fatalf("a search narrows by name: %+v", teams)
	}

	if err := g.SetTeam(unattended(), "#3", "platform"); err != nil {
		t.Fatal(err)
	}
	if body := decodeBody(t, site.recorded("PUT", "/projects/acme%2Fapp/issues/3")[0]); body["add_labels"] != "team::platform" || body["remove_labels"] != "team::data" {
		t.Fatalf("team swap: %v", body)
	}
	if err := g.SetTeam(unattended(), "#3", ""); err != nil {
		t.Fatal(err)
	}
	if body := decodeBody(t, site.recorded("PUT", "/projects/acme%2Fapp/issues/3")[1]); body["add_labels"] != nil || body["remove_labels"] != "team::data" {
		t.Fatalf("clearing the team removes its label: %v", body)
	}

	members, err := g.TeamMembers(context.Background(), tracker.TeamRequest{TeamID: "platform"})
	if err != nil || len(members) != 2 || members[0].AccountID != "42" || !members[0].Active || members[1].Active || members[1].DisplayName != "bob" {
		t.Fatalf("members: %+v %v", members, err)
	}
}
