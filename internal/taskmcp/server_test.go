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
)

// call runs one tool against an in-process server and returns its structured result.
func call(t *testing.T, database *db.DB, name string, args map[string]any) (map[string]any, error) {
	t.Helper()
	ctx := context.Background()
	client, server := mcp.NewInMemoryTransports()
	go func() { _ = NewServer(database).Run(ctx, server) }()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, client, nil)
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
	t.Setenv("TASKFLOW_GITHUB_API_URL", tracker.URL)
	t.Setenv("TASKFLOW_GITHUB_TOKEN", "server-secret")
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Tracker backed", IssueTracker: "github", GithubRepo: "acme/app"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := database.CreateTask(models.CreateTaskRequest{Title: "Remote issue", ProjectID: project.ID})
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
