package taskmcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tasks/internal/models"
	sectiletracker "tasks/internal/tracker"
)

// A comment a person posts from an interactive session is theirs and goes out
// with their own credential. The same comment posted from a skill run, a
// console Sectile launched or a session holding an unfinished run, is the
// run's: it goes out with the server credential, still recorded as the
// person's (ADR 0058).
func TestACommentFromASkillRunUsesTheServerCredential(t *testing.T) {
	site := newFakeJira(t)
	database, project := jiraDatabase(t, site)
	task, err := database.CreateTaskAs(sectiletracker.WithUnattended(context.Background()), models.CreateTaskRequest{ProjectID: project.ID, Title: "Follow-up", RequireRemoteCreation: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.EnsureUser("usr_ada"); err != nil {
		t.Fatal(err)
	}
	if err := database.SetUserTrackerCredential("usr_ada", "jira", site.server.URL, "ada@example.com", "ada-token", ""); err != nil {
		t.Fatal(err)
	}
	caller := Caller{UserID: "usr_ada", Name: "Ada"}
	personal, server := basic("ada@example.com", "ada-token"), basic("server@example.com", "server-secret")
	const path = "/rest/api/3/issue/PE-42/comment"
	posted := func() []string { return site.authenticatedAs(http.MethodPost, path) }
	expectLast := func(t *testing.T, want, why string) {
		t.Helper()
		got := posted()
		if len(got) == 0 || got[len(got)-1] != want {
			t.Fatalf("%s: got %v, want %s last", why, got, want)
		}
	}
	comment := map[string]any{"taskKey": task.ID, "body": "Hello"}

	if _, err := callAs(t, database, &caller, "add_comment", comment); err != nil {
		t.Fatal(err)
	}
	expectLast(t, personal, "an interactive comment carries the person's credential")

	if _, err := callAsInRun(t, database, &caller, "run-launched", "add_comment", comment); err != nil {
		t.Fatal(err)
	}
	expectLast(t, server, "a comment from a launched console carries the server credential")

	// One session that starts a run, comments, finishes it and comments again.
	registry := NewSessionRegistry(database)
	defer registry.Stop()
	resolve := func(http.Header) (Caller, bool) { return caller, true }
	srv := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return NewServerWithCallers(database, registry, resolve) },
		&mcp.StreamableHTTPOptions{JSONResponse: true},
	))
	defer srv.Close()
	ctx := context.Background()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, &mcp.StreamableClientTransport{Endpoint: srv.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	tool := func(name string, args map[string]any) map[string]any {
		t.Helper()
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || result.IsError {
			t.Fatalf("%s: %+v %v", name, result, err)
		}
		out, _ := result.StructuredContent.(map[string]any)
		return out
	}

	started := tool("start_run", map[string]any{"taskKey": task.ID, "skill": "clarify-issue"})
	runID, _ := started["id"].(string)
	if runID == "" {
		t.Fatalf("start_run returned no id: %v", started)
	}
	tool("add_comment", comment)
	expectLast(t, server, "a comment while the session holds a run carries the server credential")

	tool("finish_run", map[string]any{"taskKey": task.ID, "runId": runID, "status": "completed", "note": "done"})
	tool("add_comment", comment)
	expectLast(t, personal, "once the run is finished, a comment is the person's again")
}

// An edit a skill run makes through update_task travels through the queued
// field sync: the run's marker must survive the queue, so the issue is updated
// with the server credential, while the same edit made interactively carries
// the person's own (ADR 0058).
func TestAnUpdateFromASkillRunUsesTheServerCredential(t *testing.T) {
	site := newFakeJira(t)
	database, project := jiraDatabase(t, site)
	task, err := database.CreateTaskAs(sectiletracker.WithUnattended(context.Background()), models.CreateTaskRequest{ProjectID: project.ID, Title: "Follow-up", RequireRemoteCreation: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.EnsureUser("usr_ada"); err != nil {
		t.Fatal(err)
	}
	if err := database.SetUserTrackerCredential("usr_ada", "jira", site.server.URL, "ada@example.com", "ada-token", ""); err != nil {
		t.Fatal(err)
	}
	caller := Caller{UserID: "usr_ada", Name: "Ada"}
	const path = "/rest/api/3/issue/PE-42"
	for _, round := range []struct {
		runID, title, want string
	}{
		{"", "Edited by Ada", basic("ada@example.com", "ada-token")},
		{"run-launched", "Edited by a run", basic("server@example.com", "server-secret")},
	} {
		before := len(site.authenticatedAs(http.MethodPut, path))
		if _, err := callAsInRun(t, database, &caller, round.runID, "update_task", map[string]any{"taskKey": task.ID, "title": round.title}); err != nil {
			t.Fatal(err)
		}
		// The field sync is queued: wait for its write to reach Jira.
		deadline := time.Now().Add(10 * time.Second)
		for len(site.authenticatedAs(http.MethodPut, path)) == before && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		// And for the sync to go quiet, so a late write is not counted in the
		// next round.
		for seen := -1; seen != len(site.authenticatedAs(http.MethodPut, path)) && time.Now().Before(deadline); {
			seen = len(site.authenticatedAs(http.MethodPut, path))
			time.Sleep(100 * time.Millisecond)
		}
		got := site.authenticatedAs(http.MethodPut, path)[before:]
		if len(got) == 0 {
			t.Fatalf("%q: the edit did not reach Jira", round.runID)
		}
		for _, auth := range got {
			if auth != round.want {
				t.Fatalf("%q: got %v, want %s", round.runID, got, round.want)
			}
		}
	}
}
