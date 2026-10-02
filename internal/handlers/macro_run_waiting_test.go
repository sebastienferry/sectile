package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// A macro skill reports its wait the way it starts and finishes its run, by
// project and macro key, and its next call ends the wait (#648).
func TestMacroRunReportsWaitingThroughMCP(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	key := agentTestKey(t, h)
	if _, err := database.SaveMacroMeta("default", "M-1", nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h.MCPHandler())
	t.Cleanup(srv.Close)
	session := mcpSession(t, srv.URL, key)
	t.Cleanup(func() { session.Close() })

	var run struct {
		ID string `json:"id"`
	}
	raw := callTool(t, session, "start_run", map[string]any{"projectId": "default", "macroKey": "M-1", "skill": "refine-macro"})
	if err := json.Unmarshal([]byte(raw), &run); err != nil || run.ID == "" {
		t.Fatalf("start_run answered %s (%v)", raw, err)
	}
	callTool(t, session, "report_waiting", map[string]any{"projectId": "default", "macroKey": "M-1", "runId": run.ID, "waiting": true})
	if waitingSinceOf(t, database, run.ID) == nil {
		t.Fatal("report_waiting did not mark the macro run")
	}
	callTool(t, session, "list_projects", map[string]any{})
	if waitingSinceOf(t, database, run.ID) != nil {
		t.Fatal("the session's next call left the macro run waiting")
	}
}
