package agent

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
	"tasks/internal/testhome"
)

// taskPageServer stands in for the server behind the task page routes: one
// task of project "p", t2 of project "other", and a record of every write.
type taskPageServer struct {
	mu        sync.Mutex
	puts      []map[string]json.RawMessage
	tokens    []string
	queries   []string
	putStatus int
}

func (s *taskPageServer) start(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.tokens = append(s.tokens, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/agent/config"):
			_ = json.NewEncoder(w).Encode(agentconfig.Config{SchemaVersion: agentconfig.Version, ProjectID: r.URL.Query().Get("projectId")})
		case r.URL.Path == "/api/tasks/t1/assignable":
			s.queries = append(s.queries, r.URL.Query().Get("q"))
			_, _ = w.Write([]byte(`[{"accountId":"42","displayName":"Jane Doe"}]`))
		case r.URL.Path == "/api/tasks/t1" && r.Method == http.MethodPut:
			var body map[string]json.RawMessage
			_ = json.NewDecoder(r.Body).Decode(&body)
			s.puts = append(s.puts, body)
			if s.putStatus != 0 {
				w.WriteHeader(s.putStatus)
				_, _ = w.Write([]byte(`{"error":"tracker refused"}`))
				return
			}
			var links []models.TaskPullRequest
			_ = json.Unmarshal(body["prLinks"], &links)
			current := ""
			if len(links) > 0 {
				current = links[len(links)-1].URL
			}
			_ = json.NewEncoder(w).Encode(models.Task{ID: "t1", ProjectID: "p", PrLinks: links, PrURL: &current})
		case r.URL.Path == "/api/tasks/t1":
			_ = json.NewEncoder(w).Encode(models.Task{ID: "t1", Key: "#1", ProjectID: "p", Title: "Loaded"})
		case r.URL.Path == "/api/tasks/t2":
			_ = json.NewEncoder(w).Encode(models.Task{ID: "t2", ProjectID: "other"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func taskPageRequest(d *agentDaemon, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer private")
	w := httptest.NewRecorder()
	d.desktopHandler(w, req)
	return w
}

func taskPageDaemon(t *testing.T, srv *httptest.Server) *agentDaemon {
	testhome.Temp(t)
	return &agentDaemon{repoRoot: t.TempDir(), loopback: loopbackServer{desktopToken: "private"}, link: serverLink{serverURL: srv.URL, token: "paired"}}
}

func TestDesktopStatusAnnouncesTheTaskPage(t *testing.T) {
	srv := (&taskPageServer{}).start(t)
	d := taskPageDaemon(t, srv)
	w := taskPageRequest(d, http.MethodGet, "/desktop/status", "")
	var status struct {
		Capabilities []string `json:"capabilities"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &status)
	for _, capability := range status.Capabilities {
		if capability == taskPageCapability {
			return
		}
	}
	t.Fatalf("/desktop/status must announce %q: %s", taskPageCapability, w.Body.String())
}

func TestDesktopTaskDetailReadsOnlyTheProjectsTasks(t *testing.T) {
	server := &taskPageServer{}
	d := taskPageDaemon(t, server.start(t))
	w := taskPageRequest(d, http.MethodGet, "/desktop/tasks/detail?projectId=p&taskId=t1", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"title":"Loaded"`) {
		t.Fatalf("the task of the project is read: %d %s", w.Code, w.Body.String())
	}
	w = taskPageRequest(d, http.MethodGet, "/desktop/tasks/detail?projectId=p&taskId=t2", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("a task of another project is refused: %d %s", w.Code, w.Body.String())
	}
	w = taskPageRequest(d, http.MethodPut, "/desktop/tasks/detail?projectId=p&taskId=t2", `{"title":"Moved"}`)
	if w.Code != http.StatusBadRequest || len(server.puts) != 0 {
		t.Fatalf("a task of another project is never written: %d %v", w.Code, server.puts)
	}
	w = taskPageRequest(d, http.MethodDelete, "/desktop/tasks/detail?projectId=p&taskId=t1", "")
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("delete is not a task page action: %d", w.Code)
	}
}

func TestDesktopTaskUpdateForwardsOnlyThePageFields(t *testing.T) {
	server := &taskPageServer{}
	d := taskPageDaemon(t, server.start(t))
	w := taskPageRequest(d, http.MethodPut, "/desktop/tasks/detail?projectId=p&taskId=t1", `{"title":" New title ","status":"finished","projectId":"other","stageProjectId":"x"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("update: %d %s", w.Code, w.Body.String())
	}
	if len(server.puts) != 1 {
		t.Fatalf("one update is forwarded: %v", server.puts)
	}
	put := server.puts[0]
	if string(put["title"]) != `"New title"` || len(put) != 1 {
		t.Fatalf("only the trimmed title is forwarded, unknown keys dropped: %v", put)
	}
	for _, token := range server.tokens {
		if token != "Bearer paired" {
			t.Fatalf("every server call carries the paired token: %v", server.tokens)
		}
	}

	server.puts = nil
	w = taskPageRequest(d, http.MethodPut, "/desktop/tasks/detail?projectId=p&taskId=t1", `{"description":"","assignee":"Jane Doe","assigneeAccountId":"42","assigneeAvatar":"","prLinks":[{"url":"https://github.com/o/r/pull/1","state":"merged","branch":"feat/1"},{"url":"https://github.com/o/r/pull/2"}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("update: %d %s", w.Code, w.Body.String())
	}
	put = server.puts[0]
	if string(put["description"]) != `""` || string(put["assignee"]) != `"Jane Doe"` || string(put["assigneeAccountId"]) != `"42"` || put["title"] != nil || put["prUrl"] != nil {
		t.Fatalf("an empty description is still a change, and prUrl is left to the server: %v", put)
	}
	var task models.Task
	_ = json.Unmarshal(w.Body.Bytes(), &task)
	if task.PrURL == nil || *task.PrURL != "https://github.com/o/r/pull/2" || len(task.PrLinks) != 2 || task.PrLinks[0].Branch != "feat/1" {
		t.Fatalf("the server's answer is relayed, its current link the last one: %+v", task)
	}

	server.puts = nil
	w = taskPageRequest(d, http.MethodPut, "/desktop/tasks/detail?projectId=p&taskId=t1", `{"prLinks":[]}`)
	if w.Code != http.StatusOK || string(server.puts[0]["prLinks"]) != `[]` {
		t.Fatalf("an empty set detaches every link: %d %v", w.Code, server.puts)
	}
}

func TestDesktopTaskUpdateRefusesBadInput(t *testing.T) {
	server := &taskPageServer{}
	d := taskPageDaemon(t, server.start(t))
	for _, body := range []string{
		`{"prLinks":[{"url":"javascript:alert(1)"}]}`,
		`{"prLinks":[{"url":"https://user:secret@github.com/o/r/pull/1"}]}`,
		`{"prLinks":[{"url":"github.com/o/r/pull/1"}]}`,
		`{"title":"   "}`,
		`not json`,
	} {
		w := taskPageRequest(d, http.MethodPut, "/desktop/tasks/detail?projectId=p&taskId=t1", body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s must be refused: %d %s", body, w.Code, w.Body.String())
		}
	}
	w := taskPageRequest(d, http.MethodPut, "/desktop/tasks/detail?projectId=p&taskId=t1", `{"prLinks":[{"url":"ftp://example.com/pull/1"}]}`)
	if !strings.Contains(w.Body.String(), "Invalid pull request link: ftp://example.com/pull/1") {
		t.Fatalf("the refusal names the link: %s", w.Body.String())
	}
	w = taskPageRequest(d, http.MethodPut, "/desktop/tasks/detail?projectId=p&taskId=t1", `{"description":"`+strings.Repeat("x", 129<<10)+`"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("a body over 128 KiB is refused: %d", w.Code)
	}
	if len(server.puts) != 0 {
		t.Fatalf("nothing is forwarded on a refusal: %v", server.puts)
	}

	server.putStatus = http.StatusBadGateway
	w = taskPageRequest(d, http.MethodPut, "/desktop/tasks/detail?projectId=p&taskId=t1", `{"title":"New"}`)
	body, _ := io.ReadAll(w.Body)
	if w.Code != http.StatusBadGateway || !strings.Contains(string(body), "tracker refused") {
		t.Fatalf("the server's refusal is relayed: %d %s", w.Code, body)
	}
}

func TestDesktopTaskAssignableRelaysTheSearch(t *testing.T) {
	server := &taskPageServer{}
	d := taskPageDaemon(t, server.start(t))
	w := taskPageRequest(d, http.MethodGet, "/desktop/tasks/assignable?projectId=p&taskId=t1&q=ja+ne", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Jane Doe") {
		t.Fatalf("assignable: %d %s", w.Code, w.Body.String())
	}
	if len(server.queries) != 1 || server.queries[0] != "ja ne" {
		t.Fatalf("the query is relayed: %v", server.queries)
	}
	w = taskPageRequest(d, http.MethodGet, "/desktop/tasks/assignable?projectId=p&taskId=t2", "")
	if w.Code != http.StatusBadRequest || len(server.queries) != 1 {
		t.Fatalf("a task of another project is not searched: %d %v", w.Code, server.queries)
	}
}
