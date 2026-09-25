package taskmcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"tasks/internal/db"
	"tasks/internal/models"
	sectiletracker "tasks/internal/tracker"
)

// tester is the person behind the calls of call: a write tool refuses a caller
// that names nobody (#482).
var tester = Caller{UserID: "usr_tester", Name: "Tester"}

// asTester stores a personal GitHub token for tester and returns a context
// acting as them, for the tests whose writes reach a tracker.
func asTester(t *testing.T, database *db.DB) context.Context {
	t.Helper()
	if err := database.EnsureUser(tester.UserID); err != nil {
		t.Fatal(err)
	}
	if err := database.SetUserTrackerCredential(tester.UserID, "github", "", "", "tester-token", ""); err != nil {
		t.Fatal(err)
	}
	return sectiletracker.WithActingUser(context.Background(), tester.UserID)
}

// call runs one tool against an in-process server, as tester, and returns its
// structured result.
func call(t *testing.T, database *db.DB, name string, args map[string]any) (map[string]any, error) {
	t.Helper()
	return callAs(t, database, &tester, name, args)
}

// callAs runs one tool as caller, over HTTP: the caller is resolved from the
// request headers, which an in-memory transport does not carry. A nil caller is
// a request that names nobody.
func callAs(t *testing.T, database *db.DB, caller *Caller, name string, args map[string]any) (map[string]any, error) {
	t.Helper()
	ctx := context.Background()
	resolve := func(http.Header) (Caller, bool) {
		if caller == nil {
			return Caller{}, false
		}
		return *caller, true
	}
	srv := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return NewServerWithCallers(database, nil, resolve) },
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

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func errorOf(result *mcp.CallToolResult) error {
	var b strings.Builder
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			b.WriteString(text.Text)
		}
	}
	return &toolError{message: b.String()}
}

type toolError struct{ message string }

func (e *toolError) Error() string { return e.message }

// A session must keep its ticket when the tracker cannot be reached: the task is
// already known locally, and losing it leaves the session unable to work at all.
func TestTaskReadSurvivesUnreachableTracker(t *testing.T) {
	// The ticket is created through the tracker, then its comment endpoint stops
	// answering: exactly the shape of a session whose server lost its credential.
	tracker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/comments") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"number":1,"title":"Remote issue","state":"open"}`))
	}))
	defer tracker.Close()
	t.Setenv("SECTILE_GITHUB_API_URL", tracker.URL)
	t.Setenv("SECTILE_GITHUB_TOKEN", "server-secret")
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Tracker backed", IssueTracker: "github", GithubRepo: "acme/app"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := database.CreateTaskAs(asTester(t, database), models.CreateTaskRequest{Title: "Remote issue", ProjectID: project.ID})
	if err != nil {
		t.Fatal(err)
	}

	result, err := call(t, database, "get_task", map[string]any{"taskKey": task.ID})
	if err != nil {
		t.Fatalf("task read failed instead of degrading: %v", err)
	}
	read, _ := result["task"].(map[string]any)
	if read == nil || read["id"] != task.ID {
		t.Fatalf("task not returned: %v", result)
	}
	if message, _ := result["commentsError"].(string); strings.TrimSpace(message) == "" {
		t.Fatalf("comment retrieval failure not reported: %v", result)
	}
	if _, present := result["comments"]; present {
		t.Fatal("unreadable comments presented as the ticket discussion")
	}

	if _, err := call(t, database, "get_task", map[string]any{"taskKey": "#does-not-exist"}); err == nil {
		t.Fatal("unknown task accepted")
	}
}

// A local task reads its comments without any tracker involved.
func TestTaskReadReturnsCommentsWhenAvailable(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Local task", Source: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.PostTaskComment(task.ID, "First note"); err != nil {
		t.Fatal(err)
	}
	result, err := call(t, database, "get_task", map[string]any{"taskKey": task.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, present := result["commentsError"]; present {
		t.Fatalf("retrieval error reported on a readable task: %v", result)
	}
	comments, _ := result["comments"].([]any)
	if len(comments) != 1 {
		t.Fatalf("comments not returned: %v", result["comments"])
	}
}

// The context a session is told to read must fit in what a session can read.
func TestProjectContextOmitsSkillBodies(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	result, err := call(t, database, "get_project_context", map[string]any{"projectId": "default"})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"projectId", "specFramework", "prCreationStage", "issueTracker", "skills"} {
		if _, present := result[key]; !present {
			t.Fatalf("%s missing from the session context: %v", key, result)
		}
	}
	raw := mustJSON(t, result)
	for _, forbidden := range []string{"\"content\"", "\"commandContent\"", "## Goal"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("skill body leaked into the session context: %s", forbidden)
		}
	}
	if len(raw) > 8000 {
		t.Fatalf("session context too large to consume: %d bytes", len(raw))
	}
	for _, entry := range result["skills"].([]any) {
		skill := entry.(map[string]any)
		if skill["id"] == "" || skill["directory"] == "" {
			t.Fatalf("skill reference is not resolvable: %v", skill)
		}
	}
}

// A session that files a ticket must reach the tracker, not just the local
// board: the whole point of the tool is that a human sees the result.
func TestCreateTaskReachesTheTracker(t *testing.T) {
	var created int
	var signedBy string
	tracker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			created++
			signedBy = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"number":42,"title":"Follow-up","state":"open","html_url":"https://github.com/acme/app/issues/42"}`))
			return
		}
		_, _ = w.Write([]byte(`{"number":42,"title":"Follow-up","state":"open"}`))
	}))
	defer tracker.Close()
	t.Setenv("SECTILE_GITHUB_API_URL", tracker.URL)
	t.Setenv("SECTILE_GITHUB_TOKEN", "server-secret")
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Tracker backed", IssueTracker: "github", GithubRepo: "acme/app"})
	if err != nil {
		t.Fatal(err)
	}

	asTester(t, database)

	result, err := call(t, database, "create_task", map[string]any{
		"projectId": project.ID, "title": "Follow-up", "description": "Left over from the handoff",
		"issueType": "Task", "priority": "high", "labels": []any{"CustomerCase"},
	})
	if err != nil {
		t.Fatalf("creation failed: %v", err)
	}
	task, _ := result["task"].(map[string]any)
	if task == nil {
		t.Fatalf("created task not returned: %v", result)
	}
	if created != 1 {
		t.Fatalf("tracker issue not created exactly once: %d", created)
	}
	// The issue is created as the caller, exactly as from the web, never under
	// the server account (#482).
	if signedBy != "Bearer tester-token" {
		t.Fatalf("the issue must be created with the caller's own token, got %q", signedBy)
	}
	if task["key"] != "#42" {
		t.Fatalf("tracker key not carried back: %v", task["key"])
	}
	if url, _ := task["externalUrl"].(string); !strings.Contains(url, "/issues/42") {
		t.Fatalf("external link not carried back: %v", task["externalUrl"])
	}
	if task["projectId"] != project.ID {
		t.Fatalf("task filed on another project: %v", task["projectId"])
	}
	for field, want := range map[string]any{"description": "Left over from the handoff", "issueType": "Task", "priority": "high"} {
		if task[field] != want {
			t.Fatalf("%s not applied: %v", field, task[field])
		}
	}

	// The board owns the workflow: a created ticket enters at the first stage
	// carrying the single creation label, whatever else the caller supplied.
	reread, err := database.GetTaskByID(task["id"].(string))
	if err != nil || reread == nil {
		t.Fatal(err)
	}
	if reread.Status != models.StatusToClarify {
		t.Fatalf("created outside the first stage: %s", reread.Status)
	}
	workflow := 0
	custom := false
	for _, label := range reread.Labels {
		switch label {
		case "new":
			workflow++
		case "CustomerCase":
			custom = true
		default:
			if strings.HasPrefix(label, "#") {
				workflow++
			}
		}
	}
	if workflow != 1 || !custom {
		t.Fatalf("workflow or custom labels not preserved: %v", reread.Labels)
	}
}

// Naming the project is the guard-rail. CreateTask falls back to the first
// project when the identifier does not resolve, so an unknown one must be
// refused here rather than filed on someone else's board.
func TestCreateTaskRefusesAnUnnamedOrUnknownProject(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	for name, args := range map[string]map[string]any{
		"missing project": {"title": "Orphan"},
		"blank project":   {"projectId": "   ", "title": "Orphan"},
		"unknown project": {"projectId": "no-such-project", "title": "Orphan"},
		"missing title":   {"projectId": "default"},
		"blank title":     {"projectId": "default", "title": "  "},
	} {
		if _, err := call(t, database, "create_task", args); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	tasks, err := database.GetTasks("", "", "", "", "", "", "", "", "", nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range tasks {
		if task.Title == "Orphan" {
			t.Fatalf("rejected request still created a task on %s", task.ProjectID)
		}
	}
}

// A tracker that cannot create remotely must fail the call. Falling back to a
// local-only ticket would leave the agent believing it filed something the
// reviewer will never find.
func TestCreateTaskFailsRatherThanFilingLocally(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Jira backed", IssueTracker: "jira", JiraProject: "OPS"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := call(t, database, "create_task", map[string]any{"projectId": project.ID, "title": "Unsupported"}); err == nil {
		t.Fatal("unsupported remote creation accepted")
	}
	tasks, err := database.GetTasks("", "", "", "", project.ID, "", "", "", "", nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Fatalf("task persisted despite an unsupported tracker: %d", len(tasks))
	}

	// A project with no remote tracker at all stays creatable: local boards are
	// a supported deployment, not a degraded one.
	local, err := database.CreateProject(models.CreateProjectRequest{Name: "Local only", IssueTracker: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := call(t, database, "create_task", map[string]any{"projectId": local.ID, "title": "Local follow-up"}); err != nil {
		t.Fatalf("local project creation refused: %v", err)
	}
}

func TestUpdateTaskFields(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	task, err := database.CreateTask(models.CreateTaskRequest{
		ProjectID:   "default",
		Title:       "Initial title",
		Description: "Initial description",
		Priority:    models.PriorityLow,
		IssueType:   "Task",
		Labels:      []string{"init"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clarifiedStatus := models.StatusClarified
	task, err = database.UpdateTask(task.ID, models.UpdateTaskRequest{
		Status: &clarifiedStatus,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 1. Update title and description
	res, err := call(t, database, "update_task", map[string]any{
		"taskKey":     task.ID,
		"title":       "Updated title",
		"description": "Updated description",
	})
	if err != nil {
		t.Fatalf("update_task failed: %v", err)
	}
	taskMap, _ := res["task"].(map[string]any)
	if taskMap == nil || taskMap["title"] != "Updated title" || taskMap["description"] != "Updated description" {
		t.Fatalf("unexpected task result: %v", res)
	}
	reread, err := database.GetTaskByID(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reread.Title != "Updated title" || reread.Description != "Updated description" {
		t.Fatalf("database not updated: title=%q desc=%q", reread.Title, reread.Description)
	}
	if reread.Status != models.StatusClarified {
		t.Fatalf("status unexpectedly changed: %s", reread.Status)
	}

	// 2. Clear description with empty string
	res, err = call(t, database, "update_task", map[string]any{
		"taskKey":     task.ID,
		"description": "",
	})
	if err != nil {
		t.Fatalf("update_task clear desc failed: %v", err)
	}
	taskMap, _ = res["task"].(map[string]any)
	if taskMap == nil || taskMap["description"] != "" {
		t.Fatalf("description not cleared in result: %v", res)
	}
	reread, err = database.GetTaskByID(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reread.Description != "" {
		t.Fatalf("description not cleared in db: %q", reread.Description)
	}

	// 3. Update priority, issueType, and custom labels
	res, err = call(t, database, "update_task", map[string]any{
		"taskKey":   task.ID,
		"priority":  "urgent",
		"issueType": "Bug",
		"labels":    []any{"backend", "db"},
	})
	if err != nil {
		t.Fatalf("update_task priority/issueType/labels failed: %v", err)
	}
	reread, err = database.GetTaskByID(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reread.Priority != models.PriorityUrgent || reread.IssueType != "Bug" {
		t.Fatalf("priority or issueType not updated: prio=%s issueType=%s", reread.Priority, reread.IssueType)
	}
	hasBackend, hasDb, hasClarified := false, false, false
	for _, l := range reread.Labels {
		if l == "backend" {
			hasBackend = true
		}
		if l == "db" {
			hasDb = true
		}
		if l == "#clarified" {
			hasClarified = true
		}
	}
	if !hasBackend || !hasDb || !hasClarified {
		t.Fatalf("labels not updated properly with workflow stage retention: %v", reread.Labels)
	}

	// 4. Custom labels update strips foreign workflow stage label (#specified) and retains current (#clarified)
	res, err = call(t, database, "update_task", map[string]any{
		"taskKey": task.Key,
		"labels":  []any{"frontend", "urgent", "#specified"},
	})
	if err != nil {
		t.Fatalf("update_task foreign stage labels failed: %v", err)
	}
	reread, err = database.GetTaskByID(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	hasFrontend, hasUrgent, hasSpecified := false, false, false
	hasClarified = false
	for _, l := range reread.Labels {
		if l == "frontend" {
			hasFrontend = true
		}
		if l == "urgent" {
			hasUrgent = true
		}
		if l == "#clarified" {
			hasClarified = true
		}
		if l == "#specified" || l == "specified" {
			hasSpecified = true
		}
	}
	if !hasFrontend || !hasUrgent || !hasClarified || hasSpecified {
		t.Fatalf("stage label was not filtered or current stage was not preserved: %v", reread.Labels)
	}
	if reread.Status != models.StatusClarified {
		t.Fatalf("status was modified: %s", reread.Status)
	}
}

func TestUpdateTaskValidation(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	task, err := database.CreateTask(models.CreateTaskRequest{
		ProjectID: "default",
		Title:     "Validation task",
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		args    map[string]any
		wantErr string
	}{
		{
			name:    "missing taskKey",
			args:    map[string]any{"title": "Valid"},
			wantErr: "taskKey",
		},
		{
			name:    "blank taskKey",
			args:    map[string]any{"taskKey": "   ", "title": "Valid"},
			wantErr: "taskKey is required",
		},
		{
			name:    "no mutable fields provided",
			args:    map[string]any{"taskKey": task.ID},
			wantErr: "at least one mutable field (title, description, priority, issueType, labels) must be provided",
		},
		{
			name:    "blank title",
			args:    map[string]any{"taskKey": task.ID, "title": "   "},
			wantErr: "title cannot be empty",
		},
		{
			name:    "empty title",
			args:    map[string]any{"taskKey": task.ID, "title": ""},
			wantErr: "title",
		},
		{
			name:    "non-existent taskKey",
			args:    map[string]any{"taskKey": "#99999", "title": "Valid"},
			wantErr: "task not found: #99999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := call(t, database, "update_task", tt.args)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestUpdateTaskCallerAttributionAndTrackerSync(t *testing.T) {
	tracker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"number":42,"title":"Remote task","state":"open","html_url":"https://github.com/acme/app/issues/42"}`))
	}))
	defer tracker.Close()
	t.Setenv("SECTILE_GITHUB_API_URL", tracker.URL)
	t.Setenv("SECTILE_GITHUB_TOKEN", "server-secret")

	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	project, err := database.CreateProject(models.CreateProjectRequest{
		Name:         "Tracker project",
		IssueTracker: "github",
		GithubRepo:   "acme/app",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.EnsureUser("usr_alice"); err != nil {
		t.Fatal(err)
	}
	if err := database.SetUserTrackerCredential("usr_alice", "github", "", "", "alice-token", ""); err != nil {
		t.Fatal(err)
	}
	task, err := database.CreateTaskAs(sectiletracker.WithActingUser(context.Background(), "usr_alice"), models.CreateTaskRequest{
		ProjectID: project.ID,
		Title:     "Remote task",
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	caller := Caller{UserID: "usr_alice", Name: "Alice"}
	srv := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server {
			return NewServerWithCallers(database, nil, func(header http.Header) (Caller, bool) {
				return caller, true
			})
		},
		&mcp.StreamableHTTPOptions{JSONResponse: true},
	))
	defer srv.Close()

	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, &mcp.StreamableClientTransport{Endpoint: srv.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "update_task",
		Arguments: map[string]any{
			"taskKey": task.ID,
			"title":   "Title updated by Alice",
		},
	})
	if err != nil || res.IsError {
		t.Fatalf("update_task failed: %v, %v", res, err)
	}

	activities, err := database.GetTaskActivities(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range activities {
		t.Logf("Activity: id=%s skill=%s action=%s userId=%q", a.ID, a.SkillID, a.Action, a.UserID)
	}
	var trackerUpdate *models.TaskActivity
	for i := len(activities) - 1; i >= 0; i-- {
		if activities[i].SkillID == "tracker_update" {
			trackerUpdate = &activities[i]
			break
		}
	}
	if trackerUpdate == nil {
		t.Fatal("no tracker_update activity queued")
	}
	if trackerUpdate.UserID != "usr_alice" {
		t.Fatalf("expected tracker_update activity attributed to usr_alice, got %q", trackerUpdate.UserID)
	}
}

// A caller Sectile cannot tie to a person writes nothing: every write tool is
// refused before any change, while reads still answer (#482). The shared server
// key is one such caller; a transport that names nobody is another.
func TestWriteToolsRefuseACallerThatNamesNobody(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Untouched"})
	if err != nil {
		t.Fatal(err)
	}
	shared := &Caller{UserID: "default", Name: "Default", Anonymous: true}

	for _, caller := range []*Caller{nil, shared} {
		for name, args := range map[string]map[string]any{
			"create_task":      {"projectId": "default", "title": "Anonymous"},
			"update_task":      {"taskKey": task.ID, "title": "Anonymous"},
			"add_comment":      {"taskKey": task.ID, "body": "Anonymous"},
			"transition_stage": {"taskKey": task.ID, "stage": "clarified", "note": "Anonymous"},
		} {
			if _, err := callAs(t, database, caller, name, args); err == nil || !strings.Contains(err.Error(), "not tied to a user") {
				t.Fatalf("%s by %+v must be refused by name, got %v", name, caller, err)
			}
		}
		if _, err := callAs(t, database, caller, "get_task", map[string]any{"taskKey": task.ID}); err != nil {
			t.Fatalf("a read must still answer %+v: %v", caller, err)
		}
	}
	unchanged, err := database.GetTaskByID(task.ID)
	if err != nil || unchanged.Title != "Untouched" || unchanged.Status != models.StatusToClarify {
		t.Fatalf("a refused write changed the task: %+v %v", unchanged, err)
	}
	if tasks, _ := database.GetTasks("", "", "", "", "default", "", "", "", "", nil, nil, false); len(tasks) != 1 {
		t.Fatalf("a refused creation filed a task: %d", len(tasks))
	}
}

// A person without a GitHub token of their own creates nothing through an
// agent session: the server token is not theirs to sign with (#482).
func TestCreateTaskRefusesAPersonWithoutATrackerCredential(t *testing.T) {
	requests := 0
	tracker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"number":42,"title":"Follow-up","state":"open"}`))
	}))
	defer tracker.Close()
	t.Setenv("SECTILE_GITHUB_API_URL", tracker.URL)
	t.Setenv("SECTILE_GITHUB_TOKEN", "server-secret")
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Tracker backed", IssueTracker: "github", GithubRepo: "acme/app"})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.EnsureUser(tester.UserID); err != nil {
		t.Fatal(err)
	}

	_, err = call(t, database, "create_task", map[string]any{"projectId": project.ID, "title": "Follow-up"})
	if err == nil || !strings.Contains(err.Error(), "no personal GitHub token") || !strings.Contains(err.Error(), "Profile → Tracker credentials") {
		t.Fatalf("the creation must be refused with the missing-credential message, got %v", err)
	}
	if requests != 0 {
		t.Fatalf("a refused creation must reach nothing, GitHub received %d request(s)", requests)
	}
}
