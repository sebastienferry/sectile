package agentmcp

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDesktopProjectContextUsesBoundProjectWithoutChangingExplicitTargets(t *testing.T) {
	for _, tc := range []struct {
		name, tool, project string
		args                any
		want                any
	}{
		{"omitted", "get_project_context", "project-a", map[string]any{}, map[string]any{"projectId": "project-a"}},
		{"null", "get_project_context", "project-a", nil, map[string]any{"projectId": "project-a"}},
		{"explicit project", "get_project_context", "project-a", map[string]any{"projectId": "project-b"}, map[string]any{"projectId": "project-b"}},
		{"explicit task", "get_project_context", "project-a", map[string]any{"taskKey": "OTHER-1"}, map[string]any{"taskKey": "OTHER-1"}},
		{"unbound bridge", "get_project_context", "", map[string]any{}, map[string]any{}},
		{"other tool", "list_projects", "project-a", map[string]any{}, map[string]any{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := projectContextArguments(tc.tool, tc.args, tc.project)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("forwarded arguments = %#v, want %#v", got, tc.want)
			}
		})
	}
	got := projectContextArguments("get_project_context", json.RawMessage(`{"taskKey":"OTHER-1"}`), "project-a")
	if string(got.(json.RawMessage)) != `{"taskKey":"OTHER-1"}` {
		t.Fatalf("explicit task target changed: %s", got)
	}
}
