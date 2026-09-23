package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
	"tasks/internal/terminal"
	"tasks/internal/testhome"

	"github.com/gorilla/websocket"
)

func TestDesktopConsoleAuthenticationAndReplay(t *testing.T) {
	d := &agentDaemon{terminal: terminalChoice{manager: terminal.NewManager()}, loopback: loopbackServer{desktopToken: "private"}}
	root := t.TempDir()
	if _, err := d.terminal.manager.GetOrCreateSession("run", root, nil); err != nil {
		t.Fatal(err)
	}
	defer d.terminal.manager.CloseSession("run")
	d.queue.runs = map[string]*controlledRun{"run": {sequence: 7, desktop: desktopRun{SessionID: "run", Directory: root, Status: "running"}, exited: make(chan struct{})}}
	server := httptest.NewServer(http.HandlerFunc(d.desktopHandler))
	defer server.Close()
	request := httptest.NewRequest("GET", "/desktop/runs", nil)
	response := httptest.NewRecorder()
	d.desktopHandler(response, request)
	if response.Code != 401 {
		t.Fatal("unauthenticated desktop access")
	}
	request.Header.Set("Authorization", "Bearer private")
	request.Header.Set("Origin", "http://localhost:8090")
	response = httptest.NewRecorder()
	d.desktopHandler(response, request)
	if response.Code != 401 {
		t.Fatal("web UI can access local consoles")
	}
	endpoint := "ws" + strings.TrimPrefix(server.URL, "http") + "/desktop/terminal?id=run"
	header := http.Header{"Authorization": []string{"Bearer private"}}
	connection, _, err := websocket.DefaultDialer.Dial(endpoint, header)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err := connection.WriteJSON(map[string]string{"type": "input", "data": "printf '%s%s\\n' DESKTOP_ REPLAY_OK\n"}); err != nil {
		t.Fatal(err)
	}
	readUntil := func(conn *websocket.Conn) {
		t.Helper()
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var output []byte
		for !bytes.Contains(output, []byte("DESKTOP_REPLAY_OK")) {
			_, chunk, err := conn.ReadMessage()
			if err != nil {
				t.Fatal(err)
			}
			output = append(output, chunk...)
		}
	}
	readUntil(connection)
	connection.Close()
	second, _, err := websocket.DefaultDialer.Dial(endpoint, header)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	readUntil(second)
	request = httptest.NewRequest("GET", "/desktop/runs", nil)
	request.Header.Set("Authorization", "Bearer private")
	response = httptest.NewRecorder()
	d.desktopHandler(response, request)
	var runs []desktopRun
	if err := json.Unmarshal(response.Body.Bytes(), &runs); err != nil || len(runs) != 1 {
		t.Fatalf("%s %v", response.Body.String(), err)
	}
	if runs[0].QueueSequence != 7 {
		t.Fatal("desktop response lost scheduler submission order")
	}
}

func TestDesktopRestartRequiresConfirmedExit(t *testing.T) {
	restarted := false
	run := &controlledRun{exited: make(chan struct{})}
	d := &agentDaemon{
		loopback:     loopbackServer{desktopToken: "private"},
		queue:        runQueue{runs: map[string]*controlledRun{"run": run}},
		restartAgent: func() { restarted = true },
	}
	call := func(token string) int {
		req := httptest.NewRequest(http.MethodPost, "/desktop/restart", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		d.desktopHandler(response, req)
		return response.Code
	}
	if got := call("wrong"); got != 401 || restarted {
		t.Fatalf("unauthorized restart: %d", got)
	}
	if got := call("private"); got != 409 || restarted {
		t.Fatalf("active run restart: %d", got)
	}
	close(run.exited)
	if got := call("private"); got != 204 || !restarted {
		t.Fatalf("idle restart: %d", got)
	}
	if _, err := d.wrapRun("task", "new-run", "echo unexpected"); err == nil {
		t.Fatal("accepted execution while restarting")
	}
	if got := call("private"); got != 409 {
		t.Fatalf("duplicate restart: %d", got)
	}
}

func TestDesktopShutdownDoesNotRelaunch(t *testing.T) {
	stopped := false
	d := &agentDaemon{restartAgent: func() { stopped = true }, loopback: loopbackServer{desktopToken: "private"}}
	req := httptest.NewRequest(http.MethodPost, "/desktop/shutdown", nil)
	req.Header.Set("Authorization", "Bearer private")
	response := httptest.NewRecorder()
	d.desktopHandler(response, req)
	if response.Code != 204 || !stopped || d.queue.restartRequested || !d.queue.shuttingDown {
		t.Fatalf("unexpected shutdown state: status=%d stopped=%v", response.Code, stopped)
	}
	if _, err := d.wrapRun("task", "new", "echo unexpected"); err == nil {
		t.Fatal("accepted execution during shutdown")
	}
}

func TestDesktopClearHistoryPreservesActiveRuns(t *testing.T) {
	finished := make(chan struct{})
	close(finished)
	d := &agentDaemon{loopback: loopbackServer{desktopToken: "private"}, queue: runQueue{runs: map[string]*controlledRun{
		"finished": {exited: finished},
		"active":   {exited: make(chan struct{})},
	}}}
	call := func(token string) int {
		req := httptest.NewRequest(http.MethodDelete, "/desktop/history", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res := httptest.NewRecorder()
		d.desktopHandler(res, req)
		return res.Code
	}
	if call("wrong") != 401 || len(d.queue.runs) != 2 {
		t.Fatal("unauthorized cleanup")
	}
	if call("private") != 200 || len(d.queue.runs) != 1 || d.queue.runs["active"] == nil {
		t.Fatal("cleanup must preserve active executions")
	}
	if call("private") != 200 {
		t.Fatal("cleanup must be idempotent")
	}
}

func TestLocalAgentDiscoveryWithoutDesktopMode(t *testing.T) {
	d := &agentDaemon{loopback: loopbackServer{desktopToken: "private", desktopInfo: filepath.Join(t.TempDir(), "nested", "agent-connection.json")}}
	server := httptest.NewServer(http.HandlerFunc(d.desktopHandler))
	defer server.Close()
	d.loopback.url = server.URL
	if err := d.writeDesktopInfo(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(d.loopback.desktopInfo)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("discovery file must be private")
	}
	if !localAgentAvailable(d.loopback.desktopInfo) {
		t.Fatal("standalone agent not discovered")
	}
	d.loopback.desktopToken = "rotated"
	if localAgentAvailable(d.loopback.desktopInfo) {
		t.Fatal("stale credential accepted")
	}
	if err := d.writeDesktopInfo(); err != nil {
		t.Fatal(err)
	}
	if !localAgentAvailable(d.loopback.desktopInfo) {
		t.Fatal("refreshed credential not discovered")
	}
}

func TestDesktopShutdownRefusesProjectDeployment(t *testing.T) {
	d := &agentDaemon{restartAgent: func() { t.Fatal("shutdown during deployment") }, loopback: loopbackServer{desktopToken: "private"}}
	d.prepareMu.Lock()
	defer d.prepareMu.Unlock()
	req := httptest.NewRequest(http.MethodPost, "/desktop/shutdown", nil)
	req.Header.Set("Authorization", "Bearer private")
	res := httptest.NewRecorder()
	d.desktopHandler(res, req)
	if res.Code != 409 {
		t.Fatalf("expected conflict, got %d", res.Code)
	}
}

func TestFinishedTasksCannotLaunch(t *testing.T) {
	for _, task := range []models.Task{{Status: models.StatusFinished}, {Status: models.StatusDone}, {Labels: []string{"#Finished"}}, {Labels: []string{"finished"}}} {
		if !desktopTaskFinished(task) {
			t.Fatalf("finished task accepted: %+v", task)
		}
	}
	if desktopTaskFinished(models.Task{Status: models.StatusToTest, Labels: []string{"#implemented"}}) {
		t.Fatal("unfinished task rejected")
	}
}

func TestDesktopRunStartTimestampLifecycle(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	created := time.Now().UTC().Add(-time.Hour)
	for _, status := range []string{"queued", "preparing", "failed", "canceled"} {
		raw, err := json.Marshal(desktopRun{CreatedAt: created, Status: status})
		if err != nil || bytes.Contains(raw, []byte(`"startedAt"`)) {
			t.Fatalf("unstarted %s exposes start: %s (%v)", status, raw, err)
		}
	}
	d := &agentDaemon{terminal: terminalChoice{manager: terminal.NewManager()}, loopback: loopbackServer{desktopToken: "private"}}
	d.queue.runs = map[string]*controlledRun{"run": {
		token: "control", exited: make(chan struct{}),
		desktop: desktopRun{CreatedAt: created, Status: "running", SessionID: "run"},
	}}
	before := time.Now().UTC()
	if err := d.runInPty("run", t.TempDir(), nil, "true"); err != nil {
		t.Fatal(err)
	}
	defer d.terminal.manager.CloseSession("run")
	started := d.queue.runs["run"].desktop.StartedAt
	if started.Before(before) || started.After(time.Now()) {
		t.Fatalf("start is not launch time: %v", started)
	}
	if err := d.runInPty("run", t.TempDir(), nil, "true"); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/control/runs/run", strings.NewReader(`{"status":"completed"}`))
	request.Header.Set("Authorization", "Bearer control")
	response := httptest.NewRecorder()
	d.handleRunControl(response, request)
	if response.Code != http.StatusOK {
		t.Fatal(response.Code)
	}
	request = httptest.NewRequest(http.MethodGet, "/desktop/runs", nil)
	request.Header.Set("Authorization", "Bearer private")
	response = httptest.NewRecorder()
	d.desktopHandler(response, request)
	var runs []desktopRun
	if err := json.Unmarshal(response.Body.Bytes(), &runs); err != nil || len(runs) != 1 {
		t.Fatalf("%s: %v", response.Body.String(), err)
	}
	if runs[0].Status != "completed" || !runs[0].CreatedAt.Equal(created) || !runs[0].StartedAt.Equal(started) {
		t.Fatalf("timestamps changed through completion/reuse: %+v", runs[0])
	}

	d.queue.runs["failed"] = &controlledRun{desktop: desktopRun{CreatedAt: created, Status: "preparing"}}
	invalidDir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(invalidDir, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := d.runInPty("failed", invalidDir, nil, "true"); err == nil {
		t.Fatal("launch with a file as working directory succeeded")
	}
	if !d.queue.runs["failed"].desktop.StartedAt.IsZero() {
		t.Fatal("failed launch recorded a start")
	}
}

func TestDesktopQueueCancellationMetadata(t *testing.T) {
	d := &agentDaemon{loopback: loopbackServer{desktopToken: "private"}, queue: runQueue{runs: map[string]*controlledRun{
		"waiting":  {sequence: 2, canceled: true, desktop: desktopRun{Status: "queued"}},
		"finished": {sequence: 1, canceled: true, desktop: desktopRun{Status: "canceled"}},
	}}}
	request := httptest.NewRequest("GET", "/desktop/runs", nil)
	request.Header.Set("Authorization", "Bearer private")
	response := httptest.NewRecorder()
	d.desktopHandler(response, request)
	var runs []desktopRun
	if err := json.Unmarshal(response.Body.Bytes(), &runs); err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected two executions, got %d", len(runs))
	}
	for _, run := range runs {
		if run.CancelRequested != (run.ID == "waiting") {
			t.Fatalf("incorrect cancellation metadata: %+v", run)
		}
	}
}

func TestLaunchAdmissionOfReservedSkills(t *testing.T) {
	config := agentconfig.Config{Skills: []agentconfig.Skill{{ID: "implement", Directory: "code-issue", Command: "/code-issue"}}}
	// A discussion needs no instructions; custom instructions still do.
	if !launchableSkill(config, "discuss", "") || !launchableSkill(config, "implement", "") {
		t.Fatal("legitimate launch rejected")
	}
	if !launchableSkill(config, "custom", "look at the failing test") {
		t.Fatal("custom instructions rejected")
	}
	for _, skillID := range []string{"custom", "discussion", "clarify", ""} {
		if launchableSkill(config, skillID, "") {
			t.Fatalf("accepted %q without instructions", skillID)
		}
	}
}

func TestDesktopProjectAIProviderAndModelOverrides(t *testing.T) {
	testhome.Temp(t)
	root := t.TempDir()
	for _, args := range [][]string{{"init"}, {"remote", "add", "origin", "https://example.test/project.git"}} {
		if _, err := gitLocal(context.Background(), root, args...); err != nil {
			t.Fatal(err)
		}
	}
	settings := agentconfig.Overrides{Projects: map[string]string{"p": root}}
	if err := agentconfig.WriteSettings(settings); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/agent/config") {
			json.NewEncoder(w).Encode(agentconfig.Config{
				SchemaVersion:     agentconfig.Version,
				ProjectID:         "p",
				GitRemoteURL:      "https://example.test/project.git",
				AIProvider:        "agy",
				AIModel:           "server-model",
				AICommandTemplate: "server-cmd {prompt}",
			})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/projects/") {
			json.NewEncoder(w).Encode(models.Project{
				ID:       "p",
				Name:     "Project P",
				MonoRepo: false,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	d := &agentDaemon{
		repoRoot: root,
		loopback: loopbackServer{desktopToken: "private"},
		link:     serverLink{serverURL: srv.URL, projectID: "p"},
	}

	doReq := func(method, path string, body any) *httptest.ResponseRecorder {
		var r *http.Request
		if body != nil {
			raw, _ := json.Marshal(body)
			r = httptest.NewRequest(method, path, bytes.NewReader(raw))
		} else {
			r = httptest.NewRequest(method, path, nil)
		}
		r.Header.Set("Authorization", "Bearer private")
		w := httptest.NewRecorder()
		d.desktopHandler(w, r)
		return w
	}

	// 1. Initial GET /desktop/project?id=p should return server defaults and false override flags
	w := doReq("GET", "/desktop/project?id=p", nil)
	if w.Code != 200 {
		t.Fatalf("GET /desktop/project returned %d: %s", w.Code, w.Body.String())
	}
	var projResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &projResp); err != nil {
		t.Fatal(err)
	}
	if projResp["aiProvider"] != "agy" || projResp["aiModel"] != "server-model" {
		t.Fatalf("unexpected provider/model: %v / %v", projResp["aiProvider"], projResp["aiModel"])
	}
	if projResp["aiProviderOverride"] != false || projResp["aiModelOverride"] != false {
		t.Fatalf("expected false override flags: %v / %v", projResp["aiProviderOverride"], projResp["aiModelOverride"])
	}

	// 2. Validation rejections
	// 2a. Invalid model
	w = doReq("POST", "/desktop/projects", map[string]any{
		"projectId":  "p",
		"path":       root,
		"aiModel":    "invalid model; rm -rf /",
		"aiProvider": "claude",
	})
	if w.Code != 400 {
		t.Fatalf("expected 400 for invalid model, got %d: %s", w.Code, w.Body.String())
	}

	// 2b. Invalid provider
	w = doReq("POST", "/desktop/projects", map[string]any{
		"projectId":  "p",
		"path":       root,
		"aiProvider": "unknown-provider",
	})
	if w.Code != 400 {
		t.Fatalf("expected 400 for invalid provider, got %d: %s", w.Code, w.Body.String())
	}

	// 2c. Custom provider without {prompt} in command
	w = doReq("POST", "/desktop/projects", map[string]any{
		"projectId":         "p",
		"path":              root,
		"aiProvider":        "custom",
		"aiCommandTemplate": "custom command without prompt slot",
	})
	if w.Code != 400 {
		t.Fatalf("expected 400 for custom provider without prompt, got %d: %s", w.Code, w.Body.String())
	}

	// Verify disk settings untouched after validation failures
	s, _ := agentconfig.ReadSettings(root)
	if len(s.AIProviders) != 0 || len(s.AIModels) != 0 {
		t.Fatalf("settings mutated after validation failure: %+v", s)
	}

	// 3. Valid POST /desktop/projects sets overrides
	w = doReq("POST", "/desktop/projects", map[string]any{
		"projectId":  "p",
		"path":       root,
		"aiProvider": "claude",
		"aiModel":    "claude-opus-5",
	})
	if w.Code != 204 {
		t.Fatalf("POST /desktop/projects returned %d: %s", w.Code, w.Body.String())
	}

	s, _ = agentconfig.ReadSettings(root)
	if s.AIProviders["p"] != "claude" || s.AIModels["p"] != "claude-opus-5" {
		t.Fatalf("overrides not persisted: %+v", s)
	}

	// GET /desktop/project?id=p should reflect overrides
	w = doReq("GET", "/desktop/project?id=p", nil)
	if w.Code != 200 {
		t.Fatalf("GET /desktop/project returned %d: %s", w.Code, w.Body.String())
	}
	projResp = nil
	if err := json.Unmarshal(w.Body.Bytes(), &projResp); err != nil {
		t.Fatal(err)
	}
	if projResp["aiProvider"] != "claude" || projResp["aiModel"] != "claude-opus-5" {
		t.Fatalf("unexpected provider/model after override: %v / %v", projResp["aiProvider"], projResp["aiModel"])
	}
	if projResp["aiProviderOverride"] != true || projResp["aiModelOverride"] != true {
		t.Fatalf("expected true override flags: %v / %v", projResp["aiProviderOverride"], projResp["aiModelOverride"])
	}

	// 4. Inherit resets provider and model to server defaults
	w = doReq("POST", "/desktop/projects", map[string]any{
		"projectId":         "p",
		"path":              root,
		"inheritAiProvider": true,
		"inheritAiModel":    true,
	})
	if w.Code != 204 {
		t.Fatalf("POST /desktop/projects inherit returned %d: %s", w.Code, w.Body.String())
	}

	s, _ = agentconfig.ReadSettings(root)
	if len(s.AIProviders) != 0 || len(s.AIModels) != 0 {
		t.Fatalf("overrides not deleted after inherit: %+v", s)
	}

	// GET /desktop/project?id=p should reflect server defaults again
	w = doReq("GET", "/desktop/project?id=p", nil)
	if w.Code != 200 {
		t.Fatalf("GET /desktop/project returned %d: %s", w.Code, w.Body.String())
	}
	projResp = nil
	if err := json.Unmarshal(w.Body.Bytes(), &projResp); err != nil {
		t.Fatal(err)
	}
	if projResp["aiProvider"] != "agy" || projResp["aiModel"] != "server-model" {
		t.Fatalf("unexpected provider/model after reset: %v / %v", projResp["aiProvider"], projResp["aiModel"])
	}
	if projResp["aiProviderOverride"] != false || projResp["aiModelOverride"] != false {
		t.Fatalf("expected false override flags after reset: %v / %v", projResp["aiProviderOverride"], projResp["aiModelOverride"])
	}
}

func TestDesktopTaskTransitionAndCapabilities(t *testing.T) {
	testhome.Temp(t)
	root := t.TempDir()

	var forwardedBody map[string]string
	var forwardedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/agent/config") {
			_ = json.NewEncoder(w).Encode(agentconfig.Config{
				SchemaVersion: agentconfig.Version,
				ProjectID:     "project-1",
			})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/tasks/") && strings.HasSuffix(r.URL.Path, "/stage") && r.Method == http.MethodPost {
			forwardedPath = r.URL.Path
			_ = json.NewDecoder(r.Body).Decode(&forwardedBody)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true,"stage":"reviewed"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	d := &agentDaemon{
		repoRoot: root,
		loopback: loopbackServer{desktopToken: "private"},
		link:     serverLink{serverURL: srv.URL, token: "device-token"},
	}

	doReq := func(method, path string, body any) *httptest.ResponseRecorder {
		var rdr io.Reader
		if body != nil {
			raw, _ := json.Marshal(body)
			rdr = bytes.NewReader(raw)
		}
		req := httptest.NewRequest(method, path, rdr)
		req.Header.Set("Authorization", "Bearer private")
		w := httptest.NewRecorder()
		d.desktopHandler(w, req)
		return w
	}

	// 1. GET /desktop/status includes "transition-stage" capability
	statusResp := doReq("GET", "/desktop/status", nil)
	if statusResp.Code != http.StatusOK {
		t.Fatalf("GET /desktop/status returned %d: %s", statusResp.Code, statusResp.Body.String())
	}
	var statusData struct {
		Capabilities []string `json:"capabilities"`
	}
	if err := json.Unmarshal(statusResp.Body.Bytes(), &statusData); err != nil {
		t.Fatal(err)
	}
	hasTransitionStage := false
	for _, cap := range statusData.Capabilities {
		if cap == "transition-stage" {
			hasTransitionStage = true
			break
		}
	}
	if !hasTransitionStage {
		t.Fatalf("expected transition-stage capability, got %v", statusData.Capabilities)
	}

	// 2. Validation failures (400 Bad Request)
	// Missing taskId
	w := doReq("POST", "/desktop/tasks/transition?projectId=project-1", map[string]string{
		"stage": "reviewed",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing taskId, got %d: %s", w.Code, w.Body.String())
	}

	// Missing stage
	w = doReq("POST", "/desktop/tasks/transition?projectId=project-1", map[string]string{
		"taskId": "task-abc",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing stage, got %d: %s", w.Code, w.Body.String())
	}

	// Missing projectId
	w = doReq("POST", "/desktop/tasks/transition", map[string]string{
		"taskId": "task-abc",
		"stage":  "reviewed",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing projectId, got %d: %s", w.Code, w.Body.String())
	}

	// 3. Valid transition request succeeds and forwards to server
	w = doReq("POST", "/desktop/tasks/transition?projectId=project-1", map[string]string{
		"taskId": "task-abc",
		"stage":  "reviewed",
		"note":   "Code declared as reviewed from desktop app",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid transition, got %d: %s", w.Code, w.Body.String())
	}
	if forwardedPath != "/api/tasks/task-abc/stage" {
		t.Fatalf("expected forwarded path /api/tasks/task-abc/stage, got %s", forwardedPath)
	}
	if forwardedBody["stage"] != "reviewed" || forwardedBody["note"] != "Code declared as reviewed from desktop app" {
		t.Fatalf("unexpected forwarded payload: %v", forwardedBody)
	}
}

func TestDesktopProjectTerminalSettings(t *testing.T) {
	root := t.TempDir()
	testhome.Set(t, root)
	for _, args := range [][]string{{"init"}, {"remote", "add", "origin", "https://example.test/project.git"}} {
		if _, err := gitLocal(context.Background(), root, args...); err != nil {
			t.Fatal(err)
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/agent/config") {
			json.NewEncoder(w).Encode(agentconfig.Config{
				SchemaVersion:           agentconfig.Version,
				ProjectID:               "p",
				GitRemoteURL:            "https://example.test/project.git",
				ExternalTerminalCommand: "terminal",
			})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/projects/") {
			json.NewEncoder(w).Encode(models.Project{
				ID:   "p",
				Name: "Project P",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	d := &agentDaemon{
		repoRoot: root,
		loopback: loopbackServer{desktopToken: "private"},
		link:     serverLink{serverURL: srv.URL, projectID: "p"},
	}

	doReq := func(method, path string, body any) *httptest.ResponseRecorder {
		var r *http.Request
		if body != nil {
			raw, _ := json.Marshal(body)
			r = httptest.NewRequest(method, path, bytes.NewReader(raw))
		} else {
			r = httptest.NewRequest(method, path, nil)
		}
		r.Header.Set("Authorization", "Bearer private")
		w := httptest.NewRecorder()
		d.desktopHandler(w, r)
		return w
	}

	// 1. Initial GET returns server default terminal and false override
	w := doReq("GET", "/desktop/project?id=p", nil)
	if w.Code != 200 {
		t.Fatalf("GET /desktop/project returned %d: %s", w.Code, w.Body.String())
	}
	var projResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &projResp); err != nil {
		t.Fatal(err)
	}
	if projResp["terminal"] != "terminal" || projResp["terminalOverride"] != false {
		t.Fatalf("unexpected terminal response: %+v", projResp)
	}

	// 2. Set terminal override via POST /desktop/projects
	w = doReq("POST", "/desktop/projects", map[string]any{
		"projectId": "p",
		"path":      root,
		"terminal":  "ghostty",
	})
	if w.Code != 204 {
		t.Fatalf("POST /desktop/projects returned %d: %s", w.Code, w.Body.String())
	}

	s, _ := agentconfig.ReadSettings(root)
	if s.Terminals["p"] != "ghostty" {
		t.Fatalf("expected Terminals[p] to be ghostty, got %+v", s.Terminals)
	}

	// 3. GET /desktop/project?id=p should reflect new terminal override
	w = doReq("GET", "/desktop/project?id=p", nil)
	if w.Code != 200 {
		t.Fatalf("GET /desktop/project returned %d: %s", w.Code, w.Body.String())
	}
	projResp = nil
	if err := json.Unmarshal(w.Body.Bytes(), &projResp); err != nil {
		t.Fatal(err)
	}
	if projResp["terminal"] != "ghostty" || projResp["terminalOverride"] != true {
		t.Fatalf("unexpected terminal after override: %v / %v", projResp["terminal"], projResp["terminalOverride"])
	}

	// 4. Reset via inheritTerminal
	w = doReq("POST", "/desktop/projects", map[string]any{
		"projectId":       "p",
		"path":            root,
		"inheritTerminal": true,
	})
	if w.Code != 204 {
		t.Fatalf("POST /desktop/projects inherit returned %d: %s", w.Code, w.Body.String())
	}

	w = doReq("GET", "/desktop/project?id=p", nil)
	if w.Code != 200 {
		t.Fatalf("GET /desktop/project returned %d: %s", w.Code, w.Body.String())
	}
	projResp = nil
	if err := json.Unmarshal(w.Body.Bytes(), &projResp); err != nil {
		t.Fatal(err)
	}
	if projResp["terminal"] != "terminal" || projResp["terminalOverride"] != false {
		t.Fatalf("expected reset terminal: %v / %v", projResp["terminal"], projResp["terminalOverride"])
	}
}

func TestDesktopTerminalDetach(t *testing.T) {
	var launchedApp, launchedSess string
	run := &controlledRun{
		desktop: desktopRun{
			ID:        "run-1",
			SessionID: "sess-1",
			ProjectID: "p1",
			Status:    "running",
		},
		exited: make(chan struct{}),
	}

	d := &agentDaemon{
		loopback: loopbackServer{desktopToken: "private"},
		queue: runQueue{
			runs: map[string]*controlledRun{"run-1": run},
		},
		launchTerminalFn: func(terminalApp, sessionID string) error {
			launchedApp = terminalApp
			launchedSess = sessionID
			return nil
		},
	}

	// 1. Unauthorized
	req := httptest.NewRequest("POST", "/desktop/terminal/detach", bytes.NewReader([]byte(`{"runId":"run-1"}`)))
	w := httptest.NewRecorder()
	d.desktopHandler(w, req)
	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	// 2. Not found
	req = httptest.NewRequest("POST", "/desktop/terminal/detach", bytes.NewReader([]byte(`{"runId":"unknown"}`)))
	req.Header.Set("Authorization", "Bearer private")
	w = httptest.NewRecorder()
	d.desktopHandler(w, req)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}

	// 3. Successful detach
	req = httptest.NewRequest("POST", "/desktop/terminal/detach", bytes.NewReader([]byte(`{"runId":"run-1","terminal":"ghostty"}`)))
	req.Header.Set("Authorization", "Bearer private")
	w = httptest.NewRecorder()
	d.desktopHandler(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if launchedApp != "ghostty" || launchedSess != "sess-1" {
		t.Fatalf("unexpected launch params: app=%s sess=%s", launchedApp, launchedSess)
	}
	if run.desktop.ExternalTerminal != "ghostty" {
		t.Fatalf("expected ExternalTerminal to be ghostty, got %s", run.desktop.ExternalTerminal)
	}

	// 4. Exited run
	close(run.exited)
	req = httptest.NewRequest("POST", "/desktop/terminal/detach", bytes.NewReader([]byte(`{"runId":"run-1","terminal":"ghostty"}`)))
	req.Header.Set("Authorization", "Bearer private")
	w = httptest.NewRecorder()
	d.desktopHandler(w, req)
	if w.Code != 409 {
		t.Fatalf("expected 409 for exited run, got %d", w.Code)
	}
}

func TestDesktopTasksTerminalExternal(t *testing.T) {
	root := t.TempDir()
	testhome.Set(t, root)
	for _, args := range [][]string{{"init"}, {"remote", "add", "origin", "https://example.test/project.git"}} {
		if _, err := gitLocal(context.Background(), root, args...); err != nil {
			t.Fatal(err)
		}
	}

	branchName := "feat/1"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/agent/config") {
			json.NewEncoder(w).Encode(agentconfig.Config{
				SchemaVersion:           agentconfig.Version,
				ProjectID:               "p",
				GitRemoteURL:            "https://example.test/project.git",
				ExternalTerminalCommand: "terminal",
			})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/projects/") {
			json.NewEncoder(w).Encode(models.Project{
				ID:   "p",
				Name: "Project P",
			})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/tasks/task-1") {
			json.NewEncoder(w).Encode(models.Task{
				ID:         "task-1",
				Key:        "#1",
				ProjectID:  "p",
				Title:      "Task 1",
				Status:     "in_progress",
				BranchName: &branchName,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	var launchedApp, launchedSess string
	d := &agentDaemon{
		repoRoot: root,
		loopback: loopbackServer{desktopToken: "private"},
		link:     serverLink{serverURL: srv.URL, projectID: "p"},
		terminal: terminalChoice{manager: terminal.NewManager()},
		launchTerminalFn: func(terminalApp, sessionID string) error {
			launchedApp = terminalApp
			launchedSess = sessionID
			return nil
		},
	}

	body, _ := json.Marshal(map[string]any{
		"projectId": "p",
		"taskId":    "task-1",
		"skillId":   "discuss",
		"terminal":  "ghostty",
	})
	req := httptest.NewRequest("POST", "/desktop/tasks/terminal-external", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer private")
	w := httptest.NewRecorder()
	d.desktopHandler(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var res map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res["success"] != true || res["terminal"] != "ghostty" || res["runId"] == "" {
		t.Fatalf("unexpected response: %+v", res)
	}
	runID := res["runId"].(string)
	if launchedApp != "ghostty" || launchedSess != runID {
		t.Fatalf("unexpected launch: app=%s sess=%s wantSess=%s", launchedApp, launchedSess, runID)
	}
}

func TestAdmitProjectRunValidatesAutonomousPreflight(t *testing.T) {
	d, config := disconnectFixture(t)
	config.AIProvider = "agy"
	config.AICommandTemplate = ""
	config.AICommandTemplateAutonomous = ""

	_ = agentconfig.WriteSettings(agentconfig.Overrides{
		Projects:   map[string]string{"p": d.repoRoot},
		AIProvider: "agy",
	})

	payload := agentconfig.Dispatch{
		RunID:   "run-auto-fail",
		SkillID: "implement",
		Mode:    models.SkillModeAutonomous,
	}
	run, err := d.admitProjectRun(context.Background(), "task-1", payload, config)
	if err == nil {
		t.Fatalf("expected autonomous execution to be rejected for provider %q", config.AIProvider)
	}
	if run != nil {
		t.Fatalf("expected run to be nil on rejection, got %+v", run)
	}
	if !strings.Contains(err.Error(), "headless mode") && !strings.Contains(err.Error(), "execution mode") {
		t.Fatalf("expected headless capability error, got: %v", err)
	}

	// When provider supports autonomous mode (e.g. claude), admission succeeds
	_ = agentconfig.WriteSettings(agentconfig.Overrides{
		Projects:   map[string]string{"p": d.repoRoot},
		AIProvider: "claude",
	})
	config.AIProvider = "claude"
	payload.RunID = "run-auto-pass"
	runPass, err := d.admitProjectRun(context.Background(), "task-1", payload, config)
	if err != nil {
		t.Fatalf("expected claude autonomous run to be admitted: %v", err)
	}
	if runPass == nil {
		t.Fatal("expected run to be non-nil on admission")
	}
}
