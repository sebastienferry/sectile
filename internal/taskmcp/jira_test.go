package taskmcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"tasks/internal/db"
	"tasks/internal/models"
)

// fakeJira is a Jira Cloud site serving one project, PE, that records every
// request and the account each one authenticated as.
type fakeJira struct {
	mu       sync.Mutex
	server   *httptest.Server
	posts    []map[string]any
	accounts []string
}

func basic(email, token string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(email+":"+token))
}

func newFakeJira(t *testing.T) *fakeJira {
	t.Helper()
	site := &fakeJira{}
	site.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		site.mu.Lock()
		site.accounts = append(site.accounts, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
		site.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/rest/api/3/issue":
			var payload map[string]any
			_ = json.Unmarshal(body, &payload)
			site.mu.Lock()
			site.posts = append(site.posts, payload)
			site.mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"id":"10042","key":"PE-42"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/issue/PE-42":
			fmt.Fprint(w, `{"key":"PE-42","fields":{"summary":"Follow-up","status":{"name":"To Do","statusCategory":{"key":"new"}},"issuetype":{"name":"Task"},"labels":["CustomerCase"]}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/issue/PE-42/comment":
			fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":1,"comments":[{"id":"1","author":{"displayName":"Ada"},"body":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Looks good"}]}]},"created":"2026-09-25T10:00:00.000+0000"}]}`)
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/field":
			fmt.Fprint(w, `[]`)
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/priority/search":
			fmt.Fprint(w, `{"isLast":true,"values":[{"id":"2","name":"High"},{"id":"3","name":"Medium"}]}`)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/api/3/issue/createmeta/PE/issuetypes"):
			// A creation screen without a priority: the level is put on
			// afterwards, which the PUT below accepts.
			fmt.Fprint(w, `{"total":1,"issueTypes":[{"id":"10002","name":"Task"}],"fields":[]}`)
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(site.server.Close)
	return site
}

func (s *fakeJira) created() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]map[string]any{}, s.posts...)
}

// authenticatedAs reports the Authorization header of every request to path.
func (s *fakeJira) authenticatedAs(method, path string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, line := range s.accounts {
		if rest, ok := strings.CutPrefix(line, method+" "+path+" "); ok {
			out = append(out, rest)
		}
	}
	return out
}

// jiraDatabase gives a database whose server Jira credential reaches the fake
// site, with a Jira-backed project on it.
func jiraDatabase(t *testing.T, site *fakeJira) (*db.DB, *models.Project) {
	t.Helper()
	t.Setenv("SECTILE_JIRA_URL", site.server.URL)
	t.Setenv("SECTILE_JIRA_EMAIL", "server@example.com")
	t.Setenv("SECTILE_JIRA_TOKEN", "server-secret")
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Platform", IssueTracker: "jira", JiraProject: "PE"})
	if err != nil {
		t.Fatal(err)
	}
	return database, project
}

// callAs runs one tool over HTTP, for a host that resolves every request to
// caller.
func callAs(t *testing.T, database *db.DB, caller Caller, name string, args map[string]any) (map[string]any, error) {
	t.Helper()
	ctx := context.Background()
	srv := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server {
			return NewServerWithCallers(database, nil, func(http.Header) (Caller, bool) { return caller, true })
		},
		&mcp.StreamableHTTPOptions{JSONResponse: true},
	))
	defer srv.Close()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, &mcp.StreamableClientTransport{Endpoint: srv.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return nil, err
	}
	if result.IsError {
		return nil, errorOf(result)
	}
	var out map[string]any
	if err := json.Unmarshal(mustJSON(t, result.StructuredContent), &out); err != nil {
		t.Fatal(err)
	}
	return out, nil
}

func TestCreateTaskFilesAJiraIssue(t *testing.T) {
	site := newFakeJira(t)
	database, project := jiraDatabase(t, site)

	result, err := call(t, database, "create_task", map[string]any{
		"projectId": project.ID, "title": "Follow-up", "description": "# Heading\n\nLeft over from **the handoff**",
		"issueType": "Task", "parentKey": "PE-10", "priority": "high", "labels": []any{"CustomerCase"},
	})
	if err != nil {
		t.Fatalf("creation failed: %v", err)
	}
	task, _ := result["task"].(map[string]any)
	if task == nil || task["key"] != "PE-42" {
		t.Fatalf("created task not returned with its key: %v", result)
	}
	if url, _ := task["externalUrl"].(string); url != site.server.URL+"/browse/PE-42" {
		t.Fatalf("browse URL not carried back: %v", task["externalUrl"])
	}

	posts := site.created()
	if len(posts) != 1 {
		t.Fatalf("Jira issue not created exactly once: %d", len(posts))
	}
	fields := posts[0]["fields"].(map[string]any)
	if fields["issuetype"].(map[string]any)["name"] != "Task" {
		t.Fatalf("issue type not sent: %#v", fields["issuetype"])
	}
	if fields["parent"].(map[string]any)["key"] != "PE-10" {
		t.Fatalf("parent epic not sent: %#v", fields["parent"])
	}
	if description, _ := fields["description"].(map[string]any); description["type"] != "doc" {
		t.Fatalf("description must be sent as ADF: %#v", fields["description"])
	}
	// Nobody is named on this transport, so the server credential files it.
	if got := site.authenticatedAs(http.MethodPost, "/rest/api/3/issue"); len(got) != 1 || got[0] != basic("server@example.com", "server-secret") {
		t.Fatalf("creation must use the server credential when nobody is named: %v", got)
	}

	reread, err := database.GetTaskByID(task["id"].(string))
	if err != nil || reread == nil {
		t.Fatal(err)
	}
	if reread.Status != models.StatusToClarify {
		t.Fatalf("created outside the first stage: %s", reread.Status)
	}
	workflow, custom := 0, false
	for _, label := range reread.Labels {
		switch {
		case label == "CustomerCase":
			custom = true
		case label == "new" || strings.HasPrefix(label, "#"):
			workflow++
		}
	}
	if workflow != 1 || !custom {
		t.Fatalf("workflow or custom labels not preserved: %v", reread.Labels)
	}
}

func TestCreateTaskFilesUnderTheCallersOwnJiraAccount(t *testing.T) {
	site := newFakeJira(t)
	database, project := jiraDatabase(t, site)
	caller := Caller{UserID: "usr_ada", Name: "Ada"}

	// Without a personal token the call fails before reaching Jira: the issue
	// would otherwise carry the server account's name.
	_, err := callAs(t, database, caller, "create_task", map[string]any{"projectId": project.ID, "title": "Unattributed"})
	if err == nil || !strings.Contains(err.Error(), "no personal Jira token for this user") {
		t.Fatalf("a caller without a personal token must be refused: %v", err)
	}
	if posts := site.created(); len(posts) != 0 {
		t.Fatalf("Jira must not be asked to create: %d", len(posts))
	}
	tasks, err := database.GetTasks("", "", "", "", project.ID, "", "", "", "", nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Fatalf("no row may be written on a refusal: %d", len(tasks))
	}

	if err := database.SetUserTrackerCredential("usr_ada", "jira", site.server.URL, "ada@example.com", "ada-token", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := callAs(t, database, caller, "create_task", map[string]any{"projectId": project.ID, "title": "Follow-up"}); err != nil {
		t.Fatalf("creation with a personal token failed: %v", err)
	}
	if got := site.authenticatedAs(http.MethodPost, "/rest/api/3/issue"); len(got) != 1 || got[0] != basic("ada@example.com", "ada-token") {
		t.Fatalf("creation must authenticate with the caller's token: %v", got)
	}
}

func TestGetTaskReadsJiraCommentsAsTheCaller(t *testing.T) {
	site := newFakeJira(t)
	database, project := jiraDatabase(t, site)
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "Follow-up", RequireRemoteCreation: true})
	if err != nil {
		t.Fatal(err)
	}
	caller := Caller{UserID: "usr_ada", Name: "Ada"}

	// A caller with no personal token keeps the ticket and learns why its
	// discussion is missing; the server credential is not used in their name.
	result, err := callAs(t, database, caller, "get_task", map[string]any{"taskKey": task.ID})
	if err != nil {
		t.Fatalf("get_task failed: %v", err)
	}
	if result["task"] == nil {
		t.Fatalf("the task must be returned: %v", result)
	}
	if msg, _ := result["commentsError"].(string); !strings.Contains(msg, "no personal Jira token for this user") {
		t.Fatalf("commentsError must say why: %v", result["commentsError"])
	}
	if got := site.authenticatedAs(http.MethodGet, "/rest/api/3/issue/PE-42/comment"); len(got) != 0 {
		t.Fatalf("comments must not be read with the server credential for a named caller: %v", got)
	}

	if err := database.SetUserTrackerCredential("usr_ada", "jira", site.server.URL, "ada@example.com", "ada-token", ""); err != nil {
		t.Fatal(err)
	}
	result, err = callAs(t, database, caller, "get_task", map[string]any{"taskKey": task.ID})
	if err != nil {
		t.Fatalf("get_task failed: %v", err)
	}
	comments, _ := result["comments"].([]any)
	if len(comments) != 1 || result["commentsError"] != nil {
		t.Fatalf("the Jira comments must be returned: %v", result)
	}
	if got := site.authenticatedAs(http.MethodGet, "/rest/api/3/issue/PE-42/comment"); len(got) != 1 || got[0] != basic("ada@example.com", "ada-token") {
		t.Fatalf("comments must be read with the caller's token: %v", got)
	}
}
