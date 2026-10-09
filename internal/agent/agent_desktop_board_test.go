package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
	"tasks/internal/testhome"
)

// boardTestDaemon answers /desktop requests against a fake server. project is
// what /api/projects/p answers, or nil for a server that fails to; update
// answers PUT /api/tasks/:id and records what it received.
type boardTestDaemon struct {
	daemon    *agentDaemon
	path      string
	method    string
	forwarded map[string]any
}

func newBoardTestDaemon(t *testing.T, project *models.Project, updateStatus int, updateBody string) *boardTestDaemon {
	t.Helper()
	root := t.TempDir()
	testhome.Set(t, root)
	for _, args := range [][]string{{"init"}, {"remote", "add", "origin", "https://example.test/project.git"}} {
		if _, err := gitLocal(context.Background(), root, args...); err != nil {
			t.Fatal(err)
		}
	}
	b := &boardTestDaemon{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/agent/config"):
			_ = json.NewEncoder(w).Encode(agentconfig.Config{SchemaVersion: agentconfig.Version, ProjectID: "p", GitRemoteURL: "https://example.test/project.git"})
		case strings.HasPrefix(r.URL.Path, "/api/projects/"):
			if project == nil {
				http.Error(w, "boom", 500)
				return
			}
			_ = json.NewEncoder(w).Encode(project)
		case strings.HasPrefix(r.URL.Path, "/api/tasks/"):
			b.path, b.method = r.URL.Path, r.Method
			b.forwarded = map[string]any{}
			_ = json.NewDecoder(r.Body).Decode(&b.forwarded)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(updateStatus)
			_, _ = w.Write([]byte(updateBody))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	b.daemon = &agentDaemon{
		repoRoot: root,
		loopback: loopbackServer{desktopToken: "private"},
		link:     serverLink{serverURL: srv.URL, projectID: "p", token: "device-token"},
	}
	return b
}

func (b *boardTestDaemon) do(method, path string, body any) *httptest.ResponseRecorder {
	var r *http.Request
	if raw, ok := body.(string); ok {
		r = httptest.NewRequest(method, path, strings.NewReader(raw))
	} else if body != nil {
		raw, _ := json.Marshal(body)
		r = httptest.NewRequest(method, path, bytes.NewReader(raw))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	r.Header.Set("Authorization", "Bearer private")
	w := httptest.NewRecorder()
	b.daemon.desktopHandler(w, r)
	return w
}

func TestDesktopProjectCarriesTheBoard(t *testing.T) {
	project := &models.Project{
		ID:             "p",
		EpicColors:     true,
		TrackerColumns: []models.TrackerColumn{{Name: "In Review", Statuses: []string{"Code Review"}}},
		StageColumns:   map[string][]string{"implemented": {"In Review"}},
		Trackers: []models.ProjectTracker{
			{TrackerID: "t1", Identity: "github|x", TrackerColumns: []models.TrackerColumn{{Name: "Doing", Statuses: []string{"WIP"}}}, StageColumns: map[string][]string{"specified": {"Doing"}}},
			{TrackerID: "t2"},
		},
	}
	b := newBoardTestDaemon(t, project, 200, "{}")
	w := b.do("GET", "/desktop/project?id=p", nil)
	if w.Code != 200 {
		t.Fatalf("GET /desktop/project returned %d: %s", w.Code, w.Body.String())
	}
	var answer struct {
		Board *desktopBoard `json:"board"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
		t.Fatal(err)
	}
	want := &desktopBoard{
		EpicColors:     true,
		TrackerColumns: project.TrackerColumns,
		StageColumns:   project.StageColumns,
		Trackers: []desktopBoardTracker{
			{TrackerID: "t1", TrackerColumns: project.Trackers[0].TrackerColumns, StageColumns: project.Trackers[0].StageColumns},
			{TrackerID: "t2", TrackerColumns: []models.TrackerColumn{}, StageColumns: map[string][]string{}},
		},
	}
	if !reflect.DeepEqual(answer.Board, want) {
		t.Fatalf("board = %+v, want %+v", answer.Board, want)
	}
	// An unmapped tracker answers empty values, never null, so the desktop
	// needs no guard per field.
	if !strings.Contains(w.Body.String(), `"trackerId":"t2","trackerColumns":[],"stageColumns":{}`) {
		t.Fatalf("unmapped tracker not answered empty: %s", w.Body.String())
	}
}

func TestDesktopProjectWithoutBoardWhenTheServerFails(t *testing.T) {
	b := newBoardTestDaemon(t, nil, 200, "{}")
	w := b.do("GET", "/desktop/project?id=p", nil)
	if w.Code != 200 {
		t.Fatalf("GET /desktop/project returned %d: %s", w.Code, w.Body.String())
	}
	var answer map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
		t.Fatal(err)
	}
	if _, ok := answer["board"]; ok {
		t.Fatalf("board answered although the server failed: %v", answer["board"])
	}
	if answer["configured"] != true {
		t.Fatalf("the rest of the payload is lost: %v", answer)
	}
}

func TestDesktopTaskStageMoveForwardsOnlyTheMove(t *testing.T) {
	b := newBoardTestDaemon(t, &models.Project{ID: "p"}, 200, `{"id":"task-1","status":"to_test"}`)
	w := b.do("POST", "/desktop/tasks/stage-move?projectId=p", map[string]any{
		"taskId":        "task-1",
		"labels":        []string{"bug", "#implemented"},
		"status":        "to_test",
		"trackerStatus": "Code Review",
		"title":         "not forwarded",
		"projectId":     "other",
	})
	if w.Code != 200 || w.Body.String() != `{"id":"task-1","status":"to_test"}` {
		t.Fatalf("stage move answered %d: %s", w.Code, w.Body.String())
	}
	if b.method != http.MethodPut || b.path != "/api/tasks/task-1" {
		t.Fatalf("forwarded %s %s", b.method, b.path)
	}
	want := map[string]any{"labels": []any{"bug", "#implemented"}, "status": "to_test", "trackerStatus": "Code Review", "stageProjectId": "p"}
	if !reflect.DeepEqual(b.forwarded, want) {
		t.Fatalf("forwarded %v, want %v", b.forwarded, want)
	}
}

func TestDesktopTaskStageMoveOmitsAnEmptyTrackerStatus(t *testing.T) {
	b := newBoardTestDaemon(t, &models.Project{ID: "p"}, 200, "{}")
	w := b.do("POST", "/desktop/tasks/stage-move?projectId=p", map[string]any{"taskId": "task-1", "status": "finished", "trackerStatus": "  "})
	if w.Code != 200 {
		t.Fatalf("stage move answered %d: %s", w.Code, w.Body.String())
	}
	want := map[string]any{"labels": []any{}, "status": "finished", "stageProjectId": "p"}
	if !reflect.DeepEqual(b.forwarded, want) {
		t.Fatalf("forwarded %v, want %v", b.forwarded, want)
	}
}

func TestDesktopTaskStageMoveRefusals(t *testing.T) {
	b := newBoardTestDaemon(t, &models.Project{ID: "p"}, 200, "{}")
	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   any
		code   int
	}{
		{"GET", "GET", "/desktop/tasks/stage-move?projectId=p", nil, 405},
		{"no JSON", "POST", "/desktop/tasks/stage-move?projectId=p", "not json", 400},
		{"no project", "POST", "/desktop/tasks/stage-move", map[string]any{"taskId": "task-1", "status": "to_test"}, 400},
		{"no task", "POST", "/desktop/tasks/stage-move?projectId=p", map[string]any{"taskId": " ", "status": "to_test"}, 400},
		{"unknown status", "POST", "/desktop/tasks/stage-move?projectId=p", map[string]any{"taskId": "task-1", "status": "done"}, 400},
		{"no status", "POST", "/desktop/tasks/stage-move?projectId=p", map[string]any{"taskId": "task-1"}, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b.path = ""
			w := b.do(tc.method, tc.path, tc.body)
			if w.Code != tc.code {
				t.Fatalf("answered %d, want %d: %s", w.Code, tc.code, w.Body.String())
			}
			if b.path != "" {
				t.Fatalf("a refused move reached the server: %s", b.path)
			}
		})
	}
}

func TestDesktopTaskStageMovePassesTheServerRefusalThrough(t *testing.T) {
	b := newBoardTestDaemon(t, &models.Project{ID: "p"}, 403, `{"error":"You cannot edit this task"}`)
	w := b.do("POST", "/desktop/tasks/stage-move?projectId=p", map[string]any{"taskId": "task-1", "status": "to_close"})
	if w.Code != 403 || w.Body.String() != `{"error":"You cannot edit this task"}` {
		t.Fatalf("answered %d: %s", w.Code, w.Body.String())
	}
}

func TestDesktopStatusListsStageMove(t *testing.T) {
	b := newBoardTestDaemon(t, &models.Project{ID: "p"}, 200, "{}")
	w := b.do("GET", "/desktop/status", nil)
	var status struct {
		Capabilities []string `json:"capabilities"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(status.Capabilities, stageMoveCapability) {
		t.Fatalf("stage-move missing from %v", status.Capabilities)
	}
}
