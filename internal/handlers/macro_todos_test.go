package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"tasks/internal/agentprotocol"
	"tasks/internal/models"
)

// A macro skill deposits its slicing through update_macro_todos and reads it
// back from prepare_macro_worktree, which always says how many lines there
// are, none included (#647).
func TestMacroSkillWritesAndReadsItsTodos(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	key := agentTestKey(t, h)
	description := "Shaping written by a person."
	if _, err := database.SaveMacroMeta("default", "M-1", nil, &description, nil, nil); err != nil {
		t.Fatal(err)
	}
	database.SetAgentOperations(func(_ context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		return json.RawMessage(`{"path":"/work/macro-m-1","branch":"macro/m-1","worktree":true}`), nil
	})
	srv := httptest.NewServer(h.MCPHandler())
	t.Cleanup(srv.Close)
	session := mcpSession(t, srv.URL, key)
	t.Cleanup(func() { session.Close() })

	if got := callTool(t, session, "prepare_macro_worktree", map[string]any{"projectId": "default", "macroKey": "M-1"}); !strings.Contains(got, `"todos":[]`) {
		t.Fatalf("a macro without todos must answer an empty list: %s", got)
	}

	var written struct {
		Todos []models.MacroTodo `json:"todos"`
	}
	raw := callTool(t, session, "update_macro_todos", map[string]any{"projectId": "default", "macroKey": "M-1",
		"todos": []any{map[string]any{"text": "[US-1] First slice"}, map[string]any{"text": "[US-2] Second slice"}}})
	if err := json.Unmarshal([]byte(raw), &written); err != nil || len(written.Todos) != 2 || written.Todos[0].ID == "" {
		t.Fatalf("update_macro_todos answered %s (%v), want the two lines with ids", raw, err)
	}

	var read struct {
		Todos []models.MacroTodo `json:"todos"`
	}
	raw = callTool(t, session, "prepare_macro_worktree", map[string]any{"projectId": "default", "macroKey": "M-1"})
	if err := json.Unmarshal([]byte(raw), &read); err != nil || len(read.Todos) != 2 || read.Todos[1].ID != written.Todos[1].ID {
		t.Fatalf("prepare_macro_worktree answered %s (%v), want the written lines", raw, err)
	}
	macros, err := database.GetProjectMacros("default")
	if err != nil {
		t.Fatal(err)
	}
	for _, macro := range macros {
		if macro.Key == "M-1" && macro.Description != description {
			t.Errorf("the shaping changed: %q", macro.Description)
		}
	}
}
