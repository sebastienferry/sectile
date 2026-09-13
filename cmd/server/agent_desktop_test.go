package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"tasks/internal/models"
	"tasks/internal/terminal"
)

func TestDesktopConsoleAuthenticationAndReplay(t *testing.T) {
	d := &agentDaemon{desktopToken: "private", terminalMgr: terminal.NewManager()}
	root := t.TempDir()
	if _, err := d.terminalMgr.GetOrCreateSession("run", root, nil); err != nil {
		t.Fatal(err)
	}
	defer d.terminalMgr.CloseSession("run")
	d.runs = map[string]*controlledRun{"run": {desktop: desktopRun{SessionID: "run", Directory: root, Status: "running"}, exited: make(chan struct{})}}
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
}

func TestDesktopRestartRequiresConfirmedExit(t *testing.T) {
	restarted := false
	run := &controlledRun{exited: make(chan struct{})}
	d := &agentDaemon{
		desktopToken: "private",
		runs:         map[string]*controlledRun{"run": run},
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
	d := &agentDaemon{desktopToken: "private", restartAgent: func() { stopped = true }}
	req := httptest.NewRequest(http.MethodPost, "/desktop/shutdown", nil)
	req.Header.Set("Authorization", "Bearer private")
	response := httptest.NewRecorder()
	d.desktopHandler(response, req)
	if response.Code != 204 || !stopped || d.restartRequested || !d.shuttingDown {
		t.Fatalf("unexpected shutdown state: status=%d stopped=%v", response.Code, stopped)
	}
	if _, err := d.wrapRun("task", "new", "echo unexpected"); err == nil {
		t.Fatal("accepted execution during shutdown")
	}
}

func TestDesktopClearHistoryPreservesActiveRuns(t *testing.T) {
	finished := make(chan struct{})
	close(finished)
	d := &agentDaemon{desktopToken: "private", runs: map[string]*controlledRun{
		"finished": {exited: finished},
		"active":   {exited: make(chan struct{})},
	}}
	call := func(token string) int {
		req := httptest.NewRequest(http.MethodDelete, "/desktop/history", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res := httptest.NewRecorder()
		d.desktopHandler(res, req)
		return res.Code
	}
	if call("wrong") != 401 || len(d.runs) != 2 {
		t.Fatal("unauthorized cleanup")
	}
	if call("private") != 200 || len(d.runs) != 1 || d.runs["active"] == nil {
		t.Fatal("cleanup must preserve active executions")
	}
	if call("private") != 200 {
		t.Fatal("cleanup must be idempotent")
	}
}

func TestLocalAgentDiscoveryWithoutDesktopMode(t *testing.T) {
	d := &agentDaemon{desktopToken: "private", desktopInfo: filepath.Join(t.TempDir(), "nested", "agent-connection.json")}
	server := httptest.NewServer(http.HandlerFunc(d.desktopHandler))
	defer server.Close()
	d.agentURL = server.URL
	if err := d.writeDesktopInfo(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(d.desktopInfo)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("discovery file must be private")
	}
	if !localAgentAvailable(d.desktopInfo) {
		t.Fatal("standalone agent not discovered")
	}
	d.desktopToken = "rotated"
	if localAgentAvailable(d.desktopInfo) {
		t.Fatal("stale credential accepted")
	}
	if err := d.writeDesktopInfo(); err != nil {
		t.Fatal(err)
	}
	if !localAgentAvailable(d.desktopInfo) {
		t.Fatal("refreshed credential not discovered")
	}
}

func TestDesktopShutdownRefusesProjectDeployment(t *testing.T) {
	d := &agentDaemon{desktopToken: "private", restartAgent: func() { t.Fatal("shutdown during deployment") }}
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
