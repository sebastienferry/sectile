package trackerapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"tasks/internal/models"
	"tasks/internal/tracker"
	"testing"
)

// jiraSite is a fake Jira Cloud site: handlers keyed by "METHOD /path", every
// request recorded, Basic auth checked on every call.
type jiraSite struct {
	t        *testing.T
	mu       sync.Mutex
	routes   map[string]http.HandlerFunc
	requests []recordedRequest
	server   *httptest.Server
}

type recordedRequest struct {
	Method string
	Path   string
	Query  string
	Body   string
}

func newJiraSite(t *testing.T) *jiraSite {
	t.Helper()
	resetJiraFieldCache()
	resetJiraPriorityCache()
	resetJiraCreatePriorityCache()
	site := &jiraSite{t: t, routes: map[string]http.HandlerFunc{}}
	site.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		site.mu.Lock()
		site.requests = append(site.requests, recordedRequest{r.Method, r.URL.Path, r.URL.RawQuery, string(body)})
		site.mu.Unlock()
		want := "Basic " + base64.StdEncoding.EncodeToString([]byte("ada@example.com:jira-secret"))
		if r.Header.Get("Authorization") != want {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"errorMessages":["bad credentials"]}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if handler, ok := site.routes[r.Method+" "+r.URL.Path]; ok {
			r.Body = io.NopCloser(strings.NewReader(string(body)))
			handler(w, r)
			return
		}
		// Field discovery is asked by most reads; a site with no custom field
		// answers an empty list.
		if r.Method == "GET" && r.URL.Path == "/rest/api/3/field" {
			fmt.Fprint(w, `[]`)
			return
		}
		// So is the priority scheme, on every read and on every write that
		// carries one. Unless a test says otherwise, the site runs Atlassian's
		// default scheme.
		if r.Method == "GET" && r.URL.Path == "/rest/api/3/priority/search" {
			fmt.Fprint(w, jiraDefaultPriorities)
			return
		}
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(site.server.Close)
	return site
}

// jiraDefaultPriorities is what an untouched Jira Cloud site serves, most
// urgent first.
const jiraDefaultPriorities = `{"isLast":true,"values":[
	{"id":"1","name":"Highest"},{"id":"2","name":"High"},{"id":"3","name":"Medium"},
	{"id":"4","name":"Low"},{"id":"5","name":"Lowest"}
]}`

func (s *jiraSite) on(method, path string, handler http.HandlerFunc) {
	s.routes[method+" "+path] = handler
}

func (s *jiraSite) reply(method, path, body string) {
	s.on(method, path, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) })
}

func (s *jiraSite) client() *Client {
	return &Client{HTTP: s.server.Client(), JiraURL: s.server.URL, JiraEmail: "ada@example.com", JiraToken: "jira-secret"}
}

func (s *jiraSite) adapter() *JiraAdapter { return NewJiraAdapter(s.client()) }

func (s *jiraSite) calls(method, path string) []recordedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []recordedRequest
	for _, r := range s.requests {
		if r.Method == method && r.Path == path {
			out = append(out, r)
		}
	}
	return out
}

func jiraProject() *models.Project {
	return &models.Project{ID: "p1", Slug: "pe", IssueTracker: "jira", JiraProject: "PE"}
}

func TestJiraAdapterSatisfiesTheAbstraction(t *testing.T) {
	var ts tracker.TicketingSystem = NewJiraAdapter(&Client{})
	for _, c := range []tracker.Capability{tracker.CapCreate, tracker.CapGet, tracker.CapUpdate, tracker.CapDelete, tracker.CapSync, tracker.CapComment, tracker.CapLabels, tracker.CapAssign, tracker.CapTransition, tracker.CapSprint, tracker.CapTeam, tracker.CapEpic, tracker.CapBoard} {
		if !ts.Supports(c) {
			t.Errorf("jira should support %q", c)
		}
	}
	reg := NewDefaultRegistry(&Client{})
	if resolved, err := reg.ForTask(&models.Task{Source: "jira", Key: "PE-7"}, nil); err != nil || resolved.Name() != "jira" {
		t.Fatalf("a jira-sourced task must resolve to the jira adapter: %v %v", resolved, err)
	}
	if resolved, err := reg.ForProject(jiraProject()); err != nil || resolved.Name() != "jira" {
		t.Fatalf("a jira project must resolve to the jira adapter: %v %v", resolved, err)
	}
	// GitHub gained nothing it does not have.
	gh, _ := reg.Get("github")
	if gh.Supports(tracker.CapBoard) {
		t.Error("github must not claim boards")
	}
	if _, err := gh.ListBoards(context.Background(), tracker.BoardsRequest{}); !tracker.IsUnsupported(err) {
		t.Errorf("github boards: %v", err)
	}
}

func TestJiraFormatTaskID(t *testing.T) {
	j := NewJiraAdapter(&Client{})
	for in, want := range map[string]string{"PE-12": "jira-p1-PE-12", "pe-12": "jira-p1-PE-12", "jira-p1-PE-12": "jira-p1-PE-12"} {
		if got := j.FormatTaskID("p1", in, ""); got != want {
			t.Errorf("FormatTaskID(%q) = %q, want %q", in, got, want)
		}
	}
	if got := j.FormatTaskID("", "PE-3", ""); got != "jira-PE-3" {
		t.Errorf("no project: %q", got)
	}
}

func TestJiraRefusesToWorkWithoutCredentials(t *testing.T) {
	site := newJiraSite(t)
	c := site.client()
	c.JiraToken = ""
	_, err := NewJiraAdapter(c).GetIssue(context.Background(), tracker.GetIssueRequest{Project: jiraProject(), Key: "PE-1"})
	// The caller wraps it, so the guidance has to be contained rather than equal.
	want := c.missingCredential("Jira").Error()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("expected the credential error %q, got %v", want, err)
	}
	if len(site.requests) != 0 {
		t.Fatalf("no network call is allowed without credentials: %v", site.requests)
	}
}

func TestJiraSyncPaginatesAndMapsWorkItems(t *testing.T) {
	site := newJiraSite(t)
	site.reply("GET", "/rest/api/3/field", `[
		{"id":"customfield_10020","name":"Sprint","schema":{"type":"array","custom":"com.pyxis.greenhopper.jira:gh-sprint"}},
		{"id":"customfield_10001","name":"Team","schema":{"type":"team","custom":"com.atlassian.jira.plugin.system.customfieldtypes:atlassian-team"}}
	]`)
	site.on("GET", "/rest/api/3/search/jql", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if !strings.Contains(q.Get("jql"), `project = "PE" AND issuetype IN ("Story", "Bug") ORDER BY updated DESC`) {
			t.Errorf("jql: %s", q.Get("jql"))
		}
		if !strings.Contains(q.Get("fields"), "customfield_10020") || !strings.Contains(q.Get("fields"), "parent") {
			t.Errorf("fields: %s", q.Get("fields"))
		}
		if q.Get("nextPageToken") == "page2" {
			fmt.Fprint(w, `{"issues":[{"key":"PE-3","fields":{"summary":"Closed one","status":{"name":"Done","statusCategory":{"key":"done"}},"labels":[]}}],"isLast":true}`)
			return
		}
		fmt.Fprint(w, `{"issues":[
			{"key":"PE-1","fields":{"summary":"Build it","description":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Body"}]}]},
			 "status":{"name":"In Progress","statusCategory":{"key":"indeterminate"}},"priority":{"name":"Highest"},
			 "assignee":{"accountId":"acc-1","displayName":"Ada"},"labels":["specified","team-x"],"issuetype":{"name":"Story"},
			 "parent":{"key":"PE-10","fields":{"summary":"Big epic","issuetype":{"name":"Epic"}}},
			 "created":"2026-08-25T09:12:33.000+0200","updated":"2026-09-01T10:00:00.000+0200",
			 "customfield_10020":[{"name":"Sprint 1","state":"closed"},{"name":"Sprint 2","state":"active"}],
			 "customfield_10001":{"id":"team-uuid-67","name":"Platform"}}},
			{"key":"PE-2","fields":{"summary":"No label","status":{"name":"To Do","statusCategory":{"key":"new"}},"priority":{"name":"Low"},"labels":[]}}
		],"nextPageToken":"page2","isLast":false}`)
	})
	proj := jiraProject()
	proj.IssueTypes = []string{"Story", "Bug"}
	tasks, err := site.adapter().SyncIssues(context.Background(), tracker.SyncRequest{Project: proj})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 3 || len(site.calls("GET", "/rest/api/3/search/jql")) != 2 {
		t.Fatalf("expected 3 tasks over 2 pages, got %d tasks, %d calls", len(tasks), len(site.calls("GET", "/rest/api/3/search/jql")))
	}
	first := tasks[0]
	if first.Key != "PE-1" || first.Source != "jira" || first.Status != models.StatusToImplement || first.TrackerStatus != "In Progress" ||
		first.Priority != models.PriorityUrgent || first.Assignee != "Ada" || first.Description != "Body" || first.IssueType != "Story" ||
		first.ParentKey != "PE-10" || first.ParentTitle != "Big epic" || first.ParentType != "Epic" || first.Sprint != "Sprint 2" ||
		first.Team != "Platform" || first.TeamID != "team-uuid-67" || first.ExternalURL == nil || !strings.HasSuffix(*first.ExternalURL, "/browse/PE-1") ||
		first.TrackerCreatedAt == nil || first.TrackerUpdatedAt == nil {
		t.Fatalf("first task mapped wrong: %+v", first)
	}
	if tasks[1].Status != models.StatusToClarify || tasks[1].Priority != models.PriorityLow {
		t.Fatalf("unlabelled open item: %+v", tasks[1])
	}
	if tasks[2].Status != models.StatusFinished {
		t.Fatalf("done category must be finished: %+v", tasks[2])
	}
	if id := site.adapter().FormatTaskID(proj.ID, tasks[0].Key, tasks[0].ID); id != "jira-p1-PE-1" {
		t.Fatalf("identity: %s", id)
	}
}

func TestJiraSyncWorksOnASiteWithoutSprintAndTeamFields(t *testing.T) {
	site := newJiraSite(t)
	site.reply("GET", "/rest/api/3/search/jql", `{"issues":[{"key":"PE-1","fields":{"summary":"Only","status":{"name":"To Do","statusCategory":{"key":"new"}}}}],"isLast":true}`)
	tasks, err := site.adapter().SyncIssues(context.Background(), tracker.SyncRequest{Project: jiraProject()})
	if err != nil || len(tasks) != 1 || tasks[0].Sprint != "" || tasks[0].Team != "" {
		t.Fatalf("sync without custom fields: %v %+v", err, tasks)
	}
}

func TestJiraCreateUsesTheTypeFallbackAndQuotesRefusals(t *testing.T) {
	site := newJiraSite(t)
	var created map[string]any
	site.on("POST", "/rest/api/3/issue", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&created)
		fields := created["fields"].(map[string]any)
		if _, ok := fields["customfield_10011"]; !ok {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"errorMessages":[],"errors":{"customfield_10011":"Epic Type is required."}}`)
			return
		}
		fmt.Fprint(w, `{"id":"1","key":"PE-42"}`)
	})
	site.reply("GET", "/rest/api/3/issue/PE-42", `{"key":"PE-42","fields":{"summary":"New","status":{"name":"To Do","statusCategory":{"key":"new"}},"labels":["new"]}}`)
	// What the site makes mandatory, and of which shape: a select list takes an
	// option id, a free text field takes the text.
	site.reply("GET", "/rest/api/3/issue/createmeta/PE/issuetypes", `{"total":2,"issueTypes":[{"id":"10001","name":"Story"},{"id":"10002","name":"Task"}]}`)
	mandatory := `{"total":2,"fields":[
		{"fieldId":"customfield_10011","name":"Epic Type","required":true,"allowedValues":[{"id":"10200","value":"Feature"}]},
		{"fieldId":"customfield_10050","name":"Cost centre","required":true}
	]}`
	site.reply("GET", "/rest/api/3/issue/createmeta/PE/issuetypes/10001", mandatory)
	site.reply("GET", "/rest/api/3/issue/createmeta/PE/issuetypes/10002", mandatory)

	proj := jiraProject()
	proj.IssueTypes = []string{"Story"}
	_, err := site.adapter().CreateIssue(context.Background(), tracker.CreateIssueRequest{Project: proj, Title: "New", Description: "# Heading\n\nBody", ParentKey: "pe-10"})
	if err == nil || !strings.Contains(err.Error(), "Epic Type") {
		t.Fatalf("the site's refusal must be quoted: %v", err)
	}
	// Jira names the field by id only; the refusal is completed with what the
	// creation screen requires and the request left out.
	if !strings.Contains(err.Error(), "fields this project requires on creation for Story: Cost centre (customfield_10050), Epic Type (customfield_10011)") {
		t.Fatalf("the refusal must list the mandatory fields: %v", err)
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusBadRequest {
		t.Fatalf("the completed refusal must keep the HTTP error: %v", err)
	}
	fields := created["fields"].(map[string]any)
	if fields["issuetype"].(map[string]any)["name"] != "Story" || fields["project"].(map[string]any)["key"] != "PE" || fields["parent"].(map[string]any)["key"] != "PE-10" {
		t.Fatalf("creation fields: %#v", fields)
	}
	if fields["description"].(map[string]any)["type"] != "doc" {
		t.Fatalf("description must be ADF: %#v", fields["description"])
	}

	// A field the request gave is not listed as missing.
	_, err = site.adapter().CreateIssue(context.Background(), tracker.CreateIssueRequest{Project: proj, Title: "New", Fields: map[string]string{"customfield_10050": "R&D"}})
	if err == nil || !strings.Contains(err.Error(), "for Story: Epic Type (customfield_10011)") || strings.Contains(err.Error(), "Cost centre") {
		t.Fatalf("only the fields left out are listed: %v", err)
	}

	task, err := site.adapter().CreateIssue(context.Background(), tracker.CreateIssueRequest{Project: proj, Title: "New", Fields: map[string]string{"customfield_10011": "10200", "customfield_10050": "R&D"}})
	if err != nil || task.Key != "PE-42" || task.Source != "jira" || task.Status != models.StatusToClarify {
		t.Fatalf("created task: %+v %v", task, err)
	}
	fields = created["fields"].(map[string]any)
	if fields["customfield_10011"].(map[string]any)["id"] != "10200" {
		t.Fatalf("a select list takes an option id: %#v", fields)
	}
	// Wrapping this one too made creation impossible on any project with a
	// mandatory text field: Jira answers that the value must be a string.
	if fields["customfield_10050"] != "R&D" {
		t.Fatalf("a free text field takes its text: %#v", fields["customfield_10050"])
	}

	// No configured type and no request type: Task.
	proj.IssueTypes = nil
	_, _ = site.adapter().CreateIssue(context.Background(), tracker.CreateIssueRequest{Project: proj, Title: "Bare", Fields: map[string]string{"customfield_10011": "10200"}})
	if created["fields"].(map[string]any)["issuetype"].(map[string]any)["name"] != "Task" {
		t.Fatalf("type fallback: %#v", created["fields"])
	}
}

func TestJiraCreateRefusalStaysAsIsWhenTheScreenCannotBeRead(t *testing.T) {
	site := newJiraSite(t)
	site.on("POST", "/rest/api/3/issue", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"errorMessages":[],"errors":{"customfield_10011":"Epic Type is required."}}`)
	})
	site.on("GET", "/rest/api/3/issue/createmeta/PE/issuetypes", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := site.adapter().CreateIssue(context.Background(), tracker.CreateIssueRequest{Project: jiraProject(), Title: "New"})
	if err == nil || !strings.Contains(err.Error(), "customfield_10011: Epic Type is required.") {
		t.Fatalf("Jira's refusal must be returned: %v", err)
	}
	if strings.Contains(err.Error(), "fields this project requires") {
		t.Fatalf("an unreadable creation screen must not complete the refusal: %v", err)
	}
}

func TestJiraCreateRefusalDoesNotListFieldsTheBodyCarried(t *testing.T) {
	site := newJiraSite(t)
	site.on("POST", "/rest/api/3/issue", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"errorMessages":[],"errors":{"parent":"Issue 'PE-999' does not exist."}}`)
	})
	site.reply("GET", "/rest/api/3/issue/createmeta/PE/issuetypes", `{"total":1,"issueTypes":[{"id":"10002","name":"Task"}]}`)
	// A screen that makes the description and the labels mandatory: the
	// adapter sends both from the request, outside its custom fields.
	site.reply("GET", "/rest/api/3/issue/createmeta/PE/issuetypes/10002", `{"total":3,"fields":[
		{"fieldId":"description","name":"Description","required":true},
		{"fieldId":"labels","name":"Labels","required":true},
		{"fieldId":"customfield_10050","name":"Cost centre","required":true}
	]}`)

	_, err := site.adapter().CreateIssue(context.Background(), tracker.CreateIssueRequest{Project: jiraProject(), Title: "New", Description: "Body", Labels: []string{"ops"}, ParentKey: "PE-999"})
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("Jira's refusal must be quoted: %v", err)
	}
	if !strings.Contains(err.Error(), "for Task: Cost centre (customfield_10050)") || strings.Contains(err.Error(), "Description (") || strings.Contains(err.Error(), "Labels (") {
		t.Fatalf("only the fields the body left out are listed: %v", err)
	}
}

func TestJiraCreateThatSucceedsReadsNoCreationScreen(t *testing.T) {
	site := newJiraSite(t)
	site.reply("POST", "/rest/api/3/issue", `{"id":"1","key":"PE-42"}`)
	site.reply("GET", "/rest/api/3/issue/PE-42", `{"key":"PE-42","fields":{"summary":"New","status":{"name":"To Do","statusCategory":{"key":"new"}}}}`)

	task, err := site.adapter().CreateIssue(context.Background(), tracker.CreateIssueRequest{Project: jiraProject(), Title: "New"})
	if err != nil || task.Key != "PE-42" {
		t.Fatalf("created task: %+v %v", task, err)
	}
	for _, r := range site.requests {
		if strings.Contains(r.Path, "/createmeta/") {
			t.Fatalf("a creation that succeeds must not read the creation screen: %s %s", r.Method, r.Path)
		}
	}
}

func TestJiraUpdateMovesLabelsWithoutTouchingTheStatus(t *testing.T) {
	site := newJiraSite(t)
	var put map[string]any
	site.on("PUT", "/rest/api/3/issue/PE-7", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&put)
		w.WriteHeader(http.StatusNoContent)
	})
	status := models.StatusToImplement
	err := site.adapter().UpdateIssue(context.Background(), tracker.UpdateIssueRequest{
		Project: jiraProject(), Key: "PE-7", Status: &status,
		Labels: []string{"specified", "keep"}, RemovedLabels: []string{"clarified"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := put["fields"]; ok {
		t.Fatalf("an untouched title and description must not travel: %#v", put)
	}
	ops, _ := json.Marshal(put["update"].(map[string]any)["labels"])
	for _, want := range []string{`{"add":"keep"}`, `{"add":"specified"}`, `{"remove":"clarified"}`} {
		if !strings.Contains(string(ops), want) {
			t.Errorf("label ops %s miss %s", ops, want)
		}
	}
	if calls := site.calls("GET", "/rest/api/3/issue/PE-7/transitions"); len(calls) != 0 {
		t.Fatal("a stage change must not look at transitions")
	}
}

func TestJiraFinishingRunsTheDoneTransition(t *testing.T) {
	site := newJiraSite(t)
	site.reply("GET", "/rest/api/3/issue/PE-7/transitions", `{"transitions":[
		{"id":"11","name":"Review","to":{"name":"In Review","statusCategory":{"key":"indeterminate"}}},
		{"id":"31","name":"Close Issue","to":{"name":"Done","statusCategory":{"key":"done"}}}
	]}`)
	var posted map[string]any
	site.on("POST", "/rest/api/3/issue/PE-7/transitions", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&posted)
		w.WriteHeader(http.StatusNoContent)
	})
	site.on("PUT", "/rest/api/3/issue/PE-7", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })

	finished := models.StatusFinished
	if err := site.adapter().UpdateIssue(context.Background(), tracker.UpdateIssueRequest{Project: jiraProject(), Key: "PE-7", Status: &finished, Labels: []string{"finished"}}); err != nil {
		t.Fatal(err)
	}
	if posted["transition"].(map[string]any)["id"] != "31" {
		t.Fatalf("the done-category transition must run: %#v", posted)
	}

	posted = nil
	if err := site.adapter().DeleteIssue(context.Background(), tracker.DeleteIssueRequest{Project: jiraProject(), Key: "PE-7", CloseOnly: true}); err != nil {
		t.Fatal(err)
	}
	if posted["transition"].(map[string]any)["id"] != "31" {
		t.Fatalf("deleting closes: %#v", posted)
	}

	// A named target picks by status name, case-insensitively.
	posted = nil
	if err := site.adapter().Transition(context.Background(), "PE-7", "in review"); err != nil {
		t.Fatal(err)
	}
	if posted["transition"].(map[string]any)["id"] != "11" {
		t.Fatalf("named transition: %#v", posted)
	}
}

func TestJiraFinishingWithoutADoneTransitionFails(t *testing.T) {
	site := newJiraSite(t)
	site.reply("GET", "/rest/api/3/issue/PE-8/transitions", `{"transitions":[{"id":"21","name":"Start","to":{"name":"In Progress","statusCategory":{"key":"indeterminate"}}}]}`)
	site.reply("GET", "/rest/api/3/issue/PE-8", `{"key":"PE-8","fields":{"status":{"name":"To Do","statusCategory":{"key":"new"}}}}`)
	err := site.adapter().DeleteIssue(context.Background(), tracker.DeleteIssueRequest{Project: jiraProject(), Key: "PE-8"})
	if err == nil || !strings.Contains(err.Error(), "PE-8") || !strings.Contains(err.Error(), "In Progress") {
		t.Fatalf("the refusal must name the work item and the available transitions: %v", err)
	}
	if len(site.calls("POST", "/rest/api/3/issue/PE-8/transitions")) != 0 {
		t.Fatal("nothing must be transitioned")
	}
}

func TestJiraCommentsRoundTripThroughADF(t *testing.T) {
	site := newJiraSite(t)
	var posted map[string]any
	site.on("POST", "/rest/api/3/issue/PE-1/comment", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&posted)
		fmt.Fprint(w, `{"id":"100"}`)
	})
	site.reply("GET", "/rest/api/3/issue/PE-1/comment", `{"total":1,"comments":[{"id":"100","author":{"displayName":"Ada"},"created":"2026-09-18T06:00:00.000+0200",
		"body":{"type":"doc","version":1,"content":[{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Report"}]},{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"one"}]}]}]}]}}]}`)

	j := site.adapter()
	if err := j.AddComment(context.Background(), tracker.AddCommentRequest{Project: jiraProject(), Key: "PE-1", Body: "## Report\n\n- one"}); err != nil {
		t.Fatal(err)
	}
	body := posted["body"].(map[string]any)
	if body["type"] != "doc" {
		t.Fatalf("comment body must be ADF: %#v", body)
	}
	comments, err := j.GetComments(context.Background(), tracker.GetCommentsRequest{Project: jiraProject(), Key: "PE-1"})
	if err != nil || len(comments) != 1 {
		t.Fatalf("comments: %v %+v", err, comments)
	}
	if comments[0].Author != "Ada" || comments[0].Source != "jira" || comments[0].CreatedAt == nil || comments[0].Body != "## Report\n\n- one" {
		t.Fatalf("comment mapped wrong: %+v", comments[0])
	}
	if err := j.AddComment(context.Background(), tracker.AddCommentRequest{Key: "PE-1", Body: "  "}); err == nil {
		t.Fatal("an empty comment is refused before any call")
	}
}

func TestJiraSprintMovesInBatchesOfFifty(t *testing.T) {
	site := newJiraSite(t)
	var sizes []int
	site.on("POST", "/rest/agile/1.0/sprint/77/issue", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Issues []string `json:"issues"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		sizes = append(sizes, len(body.Issues))
		w.WriteHeader(http.StatusNoContent)
	})
	site.on("POST", "/rest/agile/1.0/backlog/issue", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	keys := make([]string, 60)
	for i := range keys {
		keys[i] = fmt.Sprintf("pe-%d", i+1)
	}
	if err := site.adapter().SetSprint(context.Background(), "77", keys); err != nil {
		t.Fatal(err)
	}
	if len(sizes) != 2 || sizes[0] != 50 || sizes[1] != 10 {
		t.Fatalf("batches: %v", sizes)
	}
	if err := site.adapter().SetSprint(context.Background(), "", []string{"PE-1"}); err != nil {
		t.Fatal(err)
	}
	if len(site.calls("POST", "/rest/agile/1.0/backlog/issue")) != 1 {
		t.Fatal("an empty sprint id moves to the backlog")
	}
}

func TestJiraTeamsSearchMembersAndWrite(t *testing.T) {
	site := newJiraSite(t)
	site.reply("GET", "/rest/api/3/jql/autocompletedata/suggestions", `{"results":[{"value":"team-1","displayName":"<b>Plat</b>form"},{"value":"","displayName":"ghost"}]}`)
	site.reply("GET", "/_edge/tenant_info", `{"cloudId":"cloud-9"}`)
	site.on("GET", "/gateway/api/v4/teams/team-1/members", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("siteId") != "cloud-9" {
			t.Errorf("siteId: %s", r.URL.RawQuery)
		}
		if r.URL.Query().Get("after") == "c1" {
			fmt.Fprint(w, `{"entities":[{"membershipId":{"memberId":"acc-2"},"state":"FULL_MEMBER"}],"cursor":null}`)
			return
		}
		fmt.Fprint(w, `{"entities":[{"membershipId":{"memberId":"acc-1"},"state":"FULL_MEMBER"},{"membershipId":{"memberId":"acc-9"},"state":"INVITED"}],"cursor":"c1"}`)
	})
	site.on("GET", "/rest/api/3/user/bulk", func(w http.ResponseWriter, r *http.Request) {
		if ids := r.URL.Query()["accountId"]; len(ids) != 2 {
			t.Errorf("accountIds: %v", ids)
		}
		fmt.Fprint(w, `{"values":[{"accountId":"acc-1","displayName":"Ada","emailAddress":"ada@example.com","active":true,"accountType":"atlassian","avatarUrls":{"48x48":"https://a/48"}},
			{"accountId":"acc-2","displayName":"Bot","active":true,"accountType":"app"}]}`)
	})
	site.reply("GET", "/rest/api/3/field", `[{"id":"customfield_10001","name":"Team","schema":{"type":"team","custom":"x:atlassian-team"}}]`)
	var puts []map[string]any
	site.on("PUT", "/rest/api/3/issue/PE-1", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		puts = append(puts, body)
		if len(puts) == 1 {
			// The bare id is refused on this site; the object form is accepted.
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"errors":{"customfield_10001":"expected an object"}}`)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	j := site.adapter()
	teams, err := j.SearchTeams(context.Background(), tracker.TeamSearchRequest{Project: jiraProject(), Query: "plat"})
	if err != nil || len(teams) != 1 || teams[0].ID != "team-1" || teams[0].Name != "Platform" {
		t.Fatalf("teams: %v %+v", err, teams)
	}
	members, err := j.TeamMembers(context.Background(), tracker.TeamRequest{Project: jiraProject(), TeamID: "team-1"})
	if err != nil || len(members) != 1 || members[0].AccountID != "acc-1" || members[0].DisplayName != "Ada" || members[0].TeamID != "team-1" || members[0].AvatarURL != "https://a/48" {
		t.Fatalf("members (invited and app accounts dropped): %v %+v", err, members)
	}
	if err := j.SetTeam(context.Background(), "PE-1", "team-1"); err != nil {
		t.Fatal(err)
	}
	if len(puts) != 2 || puts[1]["fields"].(map[string]any)["customfield_10001"].(map[string]any)["id"] != "team-1" {
		t.Fatalf("team write must retry with the object form: %#v", puts)
	}
}

func TestJiraTeamMembersFailureIsAnError(t *testing.T) {
	site := newJiraSite(t)
	site.on("GET", "/_edge/tenant_info", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) })
	if _, err := site.adapter().TeamMembers(context.Background(), tracker.TeamRequest{TeamID: "team-1"}); err == nil {
		t.Fatal("an unreachable members endpoint must be reported, the caller decides to degrade")
	}
}

func TestJiraReadSideBoardsColumnsSprintsStatusesTypes(t *testing.T) {
	site := newJiraSite(t)
	site.reply("GET", "/rest/agile/1.0/board", `{"values":[{"id":5,"name":"PE board","type":"scrum"}],"isLast":true}`)
	site.reply("GET", "/rest/agile/1.0/board/5/configuration", `{"columnConfig":{"columns":[{"name":"To Do","statuses":[{"id":"1"}]},{"name":"Empty","statuses":[]},{"name":"Done","statuses":[{"id":"3"},{"id":"4"}]}]}}`)
	site.reply("GET", "/rest/api/3/status", `[{"id":"1","name":"To Do"},{"id":"3","name":"Done"},{"id":"4","name":"Closed"}]`)
	site.reply("GET", "/rest/agile/1.0/board/5/sprint", `{"values":[{"id":9,"name":"Sprint 9","state":"active","startDate":"2026-09-01","endDate":"2026-09-14"}],"isLast":true}`)
	site.reply("GET", "/rest/api/3/project/PE/statuses", `[{"statuses":[{"id":"1","name":"To Do","statusCategory":{"key":"new"}},{"id":"3","name":"Done","statusCategory":{"key":"done"}}]},{"statuses":[{"id":"1","name":"To Do","statusCategory":{"key":"new"}}]}]`)
	// The create-metadata envelope, verbatim: its own array name and a total,
	// with none of the "values"/"isLast" the Agile pages carry. Stubbing it as
	// an Agile page hid an adapter that decoded nothing from a real site.
	site.reply("GET", "/rest/api/3/issue/createmeta/PE/issuetypes", `{"maxResults":50,"startAt":0,"total":3,"issueTypes":[{"id":"10001","name":"Story","subtask":false},{"id":"10003","name":"Sub-task","subtask":true},{"id":"10000","name":"Epic","subtask":false}]}`)
	site.reply("GET", "/rest/api/3/issue/createmeta/PE/issuetypes/10000", `{"maxResults":50,"startAt":0,"total":3,"fields":[
		{"fieldId":"summary","name":"Summary","required":true},
		{"fieldId":"customfield_10011","name":"Epic Type","required":true,"hasDefaultValue":false,"allowedValues":[{"id":"1","value":"Feature"},{"id":"2","value":"Tech"}]},
		{"fieldId":"customfield_10099","name":"Defaulted","required":true,"hasDefaultValue":true}
	]}`)
	site.reply("GET", "/rest/api/3/search/jql", `{"issues":[{"key":"PE-10","fields":{"summary":"Big epic","issuetype":{"name":"Epic"},"status":{"name":"To Do","statusCategory":{"key":"new"}}}}],"isLast":true}`)

	j := site.adapter()
	ctx := context.Background()
	proj := jiraProject()
	boards, err := j.ListBoards(ctx, tracker.BoardsRequest{Project: proj})
	if err != nil || len(boards) != 1 || boards[0].ID != "5" || boards[0].Type != "scrum" {
		t.Fatalf("boards: %v %+v", err, boards)
	}
	columns, err := j.ListBoardColumns(ctx, tracker.BoardRequest{Project: proj, BoardID: "5"})
	if err != nil || len(columns) != 2 || columns[1].Name != "Done" || len(columns[1].Statuses) != 2 || columns[1].Statuses[1] != "Closed" {
		t.Fatalf("columns (empty one dropped, ids resolved): %v %+v", err, columns)
	}
	proj.BoardID = "5"
	sprints, err := j.ListSprints(ctx, tracker.BoardRequest{Project: proj})
	if err != nil || len(sprints) != 1 || sprints[0].ID != "9" || sprints[0].State != "active" {
		t.Fatalf("sprints: %v %+v", err, sprints)
	}
	statuses, err := j.ListStatuses(ctx, tracker.ProjectRequest{Project: proj})
	if err != nil || len(statuses) != 2 || statuses[1].Category != "done" {
		t.Fatalf("statuses (deduplicated): %v %+v", err, statuses)
	}
	types, err := j.ListIssueTypes(ctx, tracker.ProjectRequest{Project: proj})
	if err != nil || strings.Join(types, ",") != "Epic,Story" {
		t.Fatalf("issue types (sub-tasks out, sorted): %v %v", err, types)
	}
	required, err := j.RequiredCreateFields(ctx, tracker.CreateMetaRequest{Project: proj, IssueType: "Epic"})
	if err != nil || len(required) != 1 || required[0].Name != "Epic Type" || len(required[0].Options) != 2 || required[0].Options[1].Value != "Tech" {
		t.Fatalf("required fields: %v %+v", err, required)
	}
	epics, err := j.ListEpics(ctx, tracker.ProjectRequest{Project: proj})
	if err != nil || len(epics) != 1 || epics[0].Key != "PE-10" {
		t.Fatalf("epics: %v %+v", err, epics)
	}
	if jql := site.calls("GET", "/rest/api/3/search/jql")[0].Query; !strings.Contains(jql, "issuetype+%3D+Epic") {
		t.Fatalf("epic query: %s", jql)
	}
}

func TestJiraAssignAndParentAndLabels(t *testing.T) {
	site := newJiraSite(t)
	var assignee map[string]any
	site.on("PUT", "/rest/api/3/issue/PE-1/assignee", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&assignee)
		w.WriteHeader(http.StatusNoContent)
	})
	var put map[string]any
	site.on("PUT", "/rest/api/3/issue/PE-1", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&put)
		w.WriteHeader(http.StatusNoContent)
	})
	site.reply("GET", "/rest/api/3/user/assignable/search", `[{"accountId":"acc-1","displayName":"Ada","active":true,"accountType":"atlassian"}]`)

	j := site.adapter()
	ctx := context.Background()
	if err := j.Assign(ctx, "PE-1", "acc-1"); err != nil || assignee["accountId"] != "acc-1" {
		t.Fatalf("assign: %v %#v", err, assignee)
	}
	if err := j.Assign(ctx, "PE-1", ""); err != nil || assignee["accountId"] != nil {
		t.Fatalf("unassign sends null: %v %#v", err, assignee)
	}
	if err := j.SetParent(ctx, "PE-1", "pe-10"); err != nil || put["fields"].(map[string]any)["parent"].(map[string]any)["key"] != "PE-10" {
		t.Fatalf("parent: %v %#v", err, put)
	}
	if err := j.SetParent(ctx, "PE-1", ""); err != nil || put["fields"].(map[string]any)["parent"] != nil {
		t.Fatalf("detach sends null: %v %#v", err, put)
	}
	if err := j.UpdateLabels(ctx, "PE-1", []string{"a"}, []string{"b"}); err != nil || len(put["update"].(map[string]any)["labels"].([]any)) != 2 {
		t.Fatalf("labels: %v %#v", err, put)
	}
	people, err := j.SearchAssignable(ctx, "PE-1", "ad", 5)
	if err != nil || len(people) != 1 || people[0].ID != "acc-1" {
		t.Fatalf("assignable: %v %+v", err, people)
	}
	if err := j.Assign(ctx, "#12", "acc-1"); err == nil {
		t.Fatal("a GitHub-shaped key must be refused")
	}
}

func TestJiraRateLimitAndCredentialRefusalsAreReadable(t *testing.T) {
	site := newJiraSite(t)
	site.on("GET", "/rest/api/3/issue/PE-1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	_, err := site.adapter().GetIssue(context.Background(), tracker.GetIssueRequest{Project: jiraProject(), Key: "PE-1"})
	if !IsRateLimited(err) {
		t.Fatalf("429 must be recognised as a rate limit: %v", err)
	}
	c := site.client()
	c.JiraToken = "wrong"
	_, err = NewJiraAdapter(c).GetIssue(context.Background(), tracker.GetIssueRequest{Project: jiraProject(), Key: "PE-1"})
	// The refusal says what to check and never echoes the credential itself.
	if err == nil || !strings.Contains(err.Error(), "e-mail et jeton") || strings.Contains(err.Error(), "wrong") {
		t.Fatalf("a 401 must name what to check without echoing the credential: %v", err)
	}
}

func TestCheckJiraAnswersWithTheAccount(t *testing.T) {
	site := newJiraSite(t)
	site.reply("GET", "/rest/api/3/myself", `{"accountId":"acc-1","displayName":"Ada Lovelace"}`)
	name, err := site.client().CheckJira(context.Background(), site.server.URL+"/rest/api/3/", "ada@example.com", "jira-secret")
	if err != nil || name != "Ada Lovelace" {
		t.Fatalf("check: %q %v", name, err)
	}
	if _, err := site.client().CheckJira(context.Background(), site.server.URL, "ada@example.com", "nope"); err == nil {
		t.Fatal("a wrong token must be refused")
	}
}

func TestJiraBaseURLNormalisation(t *testing.T) {
	for in, want := range map[string]string{
		"acme.atlassian.net":                          "https://acme.atlassian.net",
		"https://acme.atlassian.net/":                 "https://acme.atlassian.net",
		"https://acme.atlassian.net/browse/PE-1":      "https://acme.atlassian.net",
		"https://acme.atlassian.net/rest/api/3/issue": "https://acme.atlassian.net",
		"":                           "",
		"http://localhost:8080/jira": "http://localhost:8080",
	} {
		if got := jiraBaseURL(in); got != want {
			t.Errorf("jiraBaseURL(%q) = %q, want %q", in, got, want)
		}
	}
}

// A headless deployment configures Jira through the environment rather than the
// setup screen, and the interface has to say so instead of showing an empty
// field on a server that is in fact configured.
func TestJiraCredentialsComeFromTheEnvironmentWhenNothingIsStored(t *testing.T) {
	for _, name := range []string{"SECTILE_JIRA_URL", "SECTILE_JIRA_EMAIL", "SECTILE_JIRA_TOKEN", "SECTILE_TRACKER_TOKEN", "JIRA_API_TOKEN"} {
		t.Setenv(name, "")
	}
	t.Setenv("SECTILE_JIRA_URL", "acme.atlassian.net")
	t.Setenv("SECTILE_JIRA_EMAIL", "ada@example.com")
	t.Setenv("SECTILE_JIRA_TOKEN", "jira-specific")
	c := NewClient()
	if c.JiraURL != "https://acme.atlassian.net" || c.JiraEmail != "ada@example.com" || c.JiraToken != "jira-specific" {
		t.Fatalf("environment not read: %q %q %q", c.JiraURL, c.JiraEmail, c.JiraToken)
	}

	// Only the Jira variable is read (#464): neither the tracker-agnostic name
	// nor the provider convention reaches Jira any more.
	t.Setenv("SECTILE_JIRA_TOKEN", "")
	t.Setenv("SECTILE_TRACKER_TOKEN", "generic")
	t.Setenv("JIRA_API_TOKEN", "from-ci")
	if got := NewClient().JiraToken; got != "" {
		t.Errorf("a removed variable reached Jira: %q", got)
	}
	t.Setenv("SECTILE_JIRA_TOKEN", "jira-specific")

	// A stored credential still wins over the environment.
	resolved := NewClient()
	resolved.Resolve = func(string) Credentials {
		return Credentials{JiraURL: "https://other.atlassian.net", JiraEmail: "stored@example.com", JiraToken: "stored"}
	}
	got := resolved.For("p1")
	if got.JiraURL != "https://other.atlassian.net" || got.JiraEmail != "stored@example.com" || got.JiraToken != "stored" {
		t.Fatalf("stored configuration must win: %q %q %q", got.JiraURL, got.JiraEmail, got.JiraToken)
	}
}

// A Jira write is attributed to the account its token belongs to. So the token
// that leaves must be the acting user's own when they stored one, and the
// server's when nobody is acting, which is what background work does.
func TestTheActingUsersOwnTokenIsWhatReachesJira(t *testing.T) {
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/rest/api/3/field" {
			// Field discovery rides along on every read; the search is what
			// this test watches.
			fmt.Fprint(w, `[]`)
			return
		}
		if r.URL.Path == "/rest/api/3/priority/search" {
			// So does the priority scheme, and it is cached per site rather
			// than per credential.
			fmt.Fprint(w, jiraDefaultPriorities)
			return
		}
		seen = append(seen, r.Header.Get("Authorization"))
		fmt.Fprint(w, `{"issues":[],"isLast":true}`)
	}))
	t.Cleanup(server.Close)
	resetJiraFieldCache()
	resetJiraPriorityCache()

	c := &Client{HTTP: server.Client(), JiraURL: server.URL, JiraEmail: "service@example.com", JiraToken: "service-token"}
	c.ResolveUser = func(userID, tracker string) (string, string, string, error) {
		switch {
		case tracker != "jira":
			return "", "", "", nil
		case userID == "ada":
			// Ada's account lives on her own site, which travels with her token.
			return server.URL, "ada@example.com", "ada-token", nil
		case userID == "locked":
			return "", "", "", fmt.Errorf("credential is sealed: its owner must unlock it")
		}
		return "", "", "", nil
	}
	adapter := NewJiraAdapter(c)
	project := jiraProject()

	// Nobody acting: the server credential, as every queued job will use.
	if _, err := adapter.SyncIssues(context.Background(), tracker.SyncRequest{Project: project}); err != nil {
		t.Fatal(err)
	}
	// Somebody acting, with a token of their own.
	ctx := tracker.WithActingUser(context.Background(), "ada")
	if _, err := adapter.SyncIssues(ctx, tracker.SyncRequest{Project: project}); err != nil {
		t.Fatal(err)
	}
	// Somebody acting, with no token of their own: refused rather than written
	// under the server account, which would put a name on it nobody chose.
	if _, err := adapter.SyncIssues(tracker.WithActingUser(context.Background(), "someone-else"), tracker.SyncRequest{Project: project}); err == nil || !strings.Contains(err.Error(), "personal Jira token") {
		t.Fatalf("a named user without a token must be refused: %v", err)
	}

	service := "Basic " + base64.StdEncoding.EncodeToString([]byte("service@example.com:service-token"))
	personal := "Basic " + base64.StdEncoding.EncodeToString([]byte("ada@example.com:ada-token"))
	if len(seen) != 2 || seen[0] != service || seen[1] != personal {
		t.Fatalf("wrong credential used: %v", seen)
	}

	// A sealed credential its owner has not unlocked fails loudly. Falling back
	// to the service account would write under a name nobody chose.
	before := len(seen)
	_, err := adapter.SyncIssues(tracker.WithActingUser(context.Background(), "locked"), tracker.SyncRequest{Project: project})
	if err == nil || !strings.Contains(err.Error(), "sealed") {
		t.Fatalf("a locked credential must stop the call: %v", err)
	}
	if len(seen) != before {
		t.Fatal("nothing must reach Jira when the acting user's credential is locked")
	}
}

// A team-managed project has one board, and the Agile API types it "simple".
// Filtering it out left such a project with no board at all, so its column
// detection failed with "no board on project <KEY>" even though the board
// exposes the very same columnConfig as a scrum or kanban one.
func TestJiraListBoardsKeepsTheSimpleBoardOfATeamManagedProject(t *testing.T) {
	site := newJiraSite(t)
	site.reply("GET", "/rest/agile/1.0/board", `{"values":[{"id":549,"name":"SFE board","type":"simple"}],"isLast":true}`)
	site.reply("GET", "/rest/agile/1.0/board/549/configuration", `{"columnConfig":{"columns":[{"name":"To Do","statuses":[{"id":"1"}]},{"name":"Done","statuses":[{"id":"3"}]}]}}`)
	site.reply("GET", "/rest/api/3/status", `[{"id":"1","name":"To Do"},{"id":"3","name":"Done"}]`)

	j := site.adapter()
	ctx := context.Background()
	proj := jiraProject()
	boards, err := j.ListBoards(ctx, tracker.BoardsRequest{Project: proj})
	if err != nil || len(boards) != 1 || boards[0].ID != "549" || boards[0].Type != "simple" {
		t.Fatalf("a simple board is a board: %v %+v", err, boards)
	}
	calls := site.calls("GET", "/rest/agile/1.0/board")
	if len(calls) != 1 || !strings.Contains(calls[0].Query, "type=scrum%2Ckanban%2Csimple") {
		t.Fatalf("the board filter must ask for the three kinds carrying columns: %+v", calls)
	}
	columns, err := j.ListBoardColumns(ctx, tracker.BoardRequest{Project: proj, BoardID: "549"})
	if err != nil || len(columns) != 2 || columns[0].Name != "To Do" || columns[1].Statuses[0] != "Done" {
		t.Fatalf("columns of a simple board: %v %+v", err, columns)
	}
}

// The background loop re-reads a project every few minutes. Asking for all of
// it costs one request per hundred work items, and asking for each work item
// one by one costs one per ticket: an incremental read asks the site what it
// has touched since the previous pass, and pays for that answer only.
//
// The clause is relative on purpose. JQL dates `-15m` with the site's own
// clock, so nothing has to agree on a timezone, and no drift between Sectile
// and Atlassian can shift the window.
func TestJiraSyncBoundsAnIncrementalPassOnTheUpdateDate(t *testing.T) {
	site := newJiraSite(t)
	site.on("GET", "/rest/api/3/search/jql", func(w http.ResponseWriter, r *http.Request) {
		jql := r.URL.Query().Get("jql")
		if !strings.Contains(jql, "updated >= -18m") {
			t.Errorf("an incremental pass must bound the search: %s", jql)
		}
		if !strings.HasPrefix(jql, `project = "PE"`) {
			t.Errorf("the project clause must survive: %s", jql)
		}
		fmt.Fprint(w, `{"issues":[{"key":"PE-1","fields":{"summary":"Moved","status":{"name":"To Do","statusCategory":{"key":"new"}}}}],"isLast":true}`)
	})
	tasks, err := site.adapter().SyncIssues(context.Background(), tracker.SyncRequest{Project: jiraProject(), UpdatedWithinMin: 18})
	if err != nil || len(tasks) != 1 {
		t.Fatalf("incremental sync: %v %+v", err, tasks)
	}
}

// A synchronisation somebody asked for reads the whole project, and so does a
// pass the loop decided to make full: no window, no clause.
func TestJiraSyncWithoutAWindowAsksForTheWholeProject(t *testing.T) {
	site := newJiraSite(t)
	site.on("GET", "/rest/api/3/search/jql", func(w http.ResponseWriter, r *http.Request) {
		if jql := r.URL.Query().Get("jql"); strings.Contains(jql, "updated >=") {
			t.Errorf("a full read carries no window: %s", jql)
		}
		fmt.Fprint(w, `{"issues":[],"isLast":true}`)
	})
	if _, err := site.adapter().SyncIssues(context.Background(), tracker.SyncRequest{Project: jiraProject()}); err != nil {
		t.Fatal(err)
	}
}

func TestJiraSyncMapsCreatorAndReporterFallback(t *testing.T) {
	site := newJiraSite(t)
	site.reply("GET", "/rest/api/3/search/jql", `{
		"issues": [
			{
				"key": "PE-1",
				"fields": {
					"summary": "Creator present",
					"status": {"name": "To Do", "statusCategory": {"key": "new"}},
					"creator": {
						"displayName": "Ada Lovelace",
						"accountId": "ada-123",
						"avatarUrls": {"48x48": "https://jira.example.com/avatar/ada.png"}
					},
					"reporter": {
						"displayName": "Reporter Name",
						"accountId": "rep-123",
						"avatarUrls": {"48x48": "https://jira.example.com/avatar/rep.png"}
					}
				}
			},
			{
				"key": "PE-2",
				"fields": {
					"summary": "Reporter fallback",
					"status": {"name": "To Do", "statusCategory": {"key": "new"}},
					"creator": null,
					"reporter": {
						"displayName": "Charles Babbage",
						"accountId": "charles-456",
						"avatarUrls": {"48x48": "https://jira.example.com/avatar/charles.png"}
					}
				}
			},
			{
				"key": "PE-3",
				"fields": {
					"summary": "Creator accountId fallback when displayName empty",
					"status": {"name": "To Do", "statusCategory": {"key": "new"}},
					"creator": {
						"displayName": "",
						"accountId": "alan-turing",
						"avatarUrls": {"32x32": "https://jira.example.com/avatar/alan.png"}
					}
				}
			}
		],
		"isLast": true
	}`)
	tasks, err := site.adapter().SyncIssues(context.Background(), tracker.SyncRequest{Project: jiraProject()})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 3 {
		t.Fatalf("len(tasks) = %d, want 3", len(tasks))
	}
	// Task 1: Creator used
	if tasks[0].Creator != "Ada Lovelace" {
		t.Errorf("task[0].Creator = %q, want %q", tasks[0].Creator, "Ada Lovelace")
	}
	if tasks[0].CreatorAvatar != "https://jira.example.com/avatar/ada.png" {
		t.Errorf("task[0].CreatorAvatar = %q, want %q", tasks[0].CreatorAvatar, "https://jira.example.com/avatar/ada.png")
	}
	// Task 2: Reporter fallback
	if tasks[1].Creator != "Charles Babbage" {
		t.Errorf("task[1].Creator = %q, want %q", tasks[1].Creator, "Charles Babbage")
	}
	if tasks[1].CreatorAvatar != "https://jira.example.com/avatar/charles.png" {
		t.Errorf("task[1].CreatorAvatar = %q, want %q", tasks[1].CreatorAvatar, "https://jira.example.com/avatar/charles.png")
	}
	// Task 3: AccountId fallback and non-48x48 avatar fallback
	if tasks[2].Creator != "alan-turing" {
		t.Errorf("task[2].Creator = %q, want %q", tasks[2].Creator, "alan-turing")
	}
	if tasks[2].CreatorAvatar != "https://jira.example.com/avatar/alan.png" {
		t.Errorf("task[2].CreatorAvatar = %q, want %q", tasks[2].CreatorAvatar, "https://jira.example.com/avatar/alan.png")
	}
}
