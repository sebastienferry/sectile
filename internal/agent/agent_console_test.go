package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"tasks/internal/agentexec"
	"testing"
	"time"

	"tasks/internal/agentconfig"
	"tasks/internal/terminal"
)

// Exercise the real supervisor when a test-owned PTY launches this executable.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "agent-exec" {
		if err := agentexec.Run(os.Args[2:]); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestConsoleCommandsHaveNoPromptOrFlags(t *testing.T) {
	for _, provider := range []string{"codex", "claude"} {
		command, err := consoleCommand(provider, "")
		if err != nil || command != "exec "+provider {
			t.Fatalf("%q: %q %v", provider, command, err)
		}
	}
	for _, provider := range []string{"", "custom", "codex --prompt hi", "claude; touch /tmp/unexpected"} {
		if _, err := consoleCommand(provider, ""); err == nil {
			t.Fatalf("accepted %q", provider)
		}
	}
}

// A free console runs against the model the project resolves, and carries no
// model flag when none is configured.
func TestConsoleCommandCarriesTheResolvedModel(t *testing.T) {
	for _, provider := range []string{"codex", "claude"} {
		command, err := consoleCommand(provider, "M")
		if err != nil || command != "exec "+provider+" --model M" {
			t.Fatalf("%q: %q %v", provider, command, err)
		}
	}
}

func TestConsoleAdmissionValidation(t *testing.T) {
	d := &agentDaemon{loopback: loopbackServer{desktopToken: "private"}}
	for _, tt := range []struct {
		method, body string
		code         int
	}{
		{"GET", "", 405}, {"POST", "{}", 400}, {"POST", `{"projectId":"p","provider":"sh"}`, 400},
		{"POST", `{"projectId":"p","provider":"codex"`, 400},
	} {
		rec := disconnectRequest(d, tt.method, "/desktop/consoles", tt.body)
		if rec.Code != tt.code {
			t.Fatalf("%s %s: %d", tt.method, tt.body, rec.Code)
		}
	}
	req := httptest.NewRequest("POST", "/desktop/consoles", strings.NewReader(`{"projectId":"p","provider":"codex"}`))
	rec := httptest.NewRecorder()
	d.desktopHandler(rec, req)
	if rec.Code != 401 {
		t.Fatal(rec.Code)
	}
	req.Header.Set("Authorization", "Bearer private")
	req.Header.Set("Origin", "http://localhost")
	rec = httptest.NewRecorder()
	d.desktopHandler(rec, req)
	if rec.Code != 401 {
		t.Fatal(rec.Code)
	}
	if len(d.queue.runs) != 0 {
		t.Fatal("invalid request registered a console")
	}
}

func TestFreeConsolePTYLifecycle(t *testing.T) {
	t.Setenv("SECTILE_TASK_ID", "inherited-task")
	t.Setenv("SECTILE_RUN_ID", "inherited-run")
	var remoteRequests atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteRequests.Add(1)
		http.Error(w, "unexpected task access", 500)
	}))
	defer remote.Close()
	d := &agentDaemon{serverURL: remote.URL, terminalMgr: terminal.NewManager(), loopback: loopbackServer{desktopToken: "private"}}
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/control/") {
			d.handleRunControl(w, r)
		} else {
			d.desktopHandler(w, r)
		}
	}))
	defer local.Close()
	d.loopback.url = local.URL
	root := t.TempDir()
	script := filepath.Join(root, "fake-agent")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n[ -t 0 ] || exit 20\n[ \"$#\" -eq 0 ] || exit 21\n[ -z \"$SECTILE_TASK_ID$SECTILE_RUN_ID\" ] || exit 22\nprintf 'READY\\n'\nread answer\nprintf 'ANSWER:%s\\n' \"$answer\"\nsleep 60\n"), 0700); err != nil {
		t.Fatal(err)
	}
	run, err := d.enqueueRun("", agentconfig.Dispatch{RunID: "free"}, "project", root, 2, false)
	if err != nil {
		t.Fatal(err)
	}
	run.desktop.Kind = "console"
	run.desktop.Provider = "codex"
	d.launchConsole(run, "exec "+quoteShell(script))
	defer d.terminalMgr.CloseSession("free")
	session, err := d.terminalMgr.GetOrCreateSession("free", root, nil)
	if err != nil {
		t.Fatal(err)
	}
	output := make(chan string, 20)
	session.AddOutputListener(func(chunk []byte) {
		select {
		case output <- string(chunk):
		default:
		}
	})
	if err := d.terminalMgr.SendInput("free", "hello console\n"); err != nil {
		t.Fatal(err)
	}
	var captured strings.Builder
	timer := time.NewTimer(8 * time.Second)
	defer timer.Stop()
	for !strings.Contains(captured.String(), "ANSWER:hello console") {
		select {
		case chunk := <-output:
			captured.WriteString(chunk)
		case <-timer.C:
			t.Fatalf("no interactive response: %s", captured.String())
		}
	}
	rec := disconnectRequest(d, "GET", "/desktop/runs", "")
	var entries []desktopRun
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Kind != "console" || entries[0].TaskID != "" || entries[0].Skill != "" || entries[0].Prompt != "" || entries[0].Directory != root {
		t.Fatalf("unexpected console: %+v", entries)
	}
	rec = disconnectRequest(d, "GET", "/desktop/run-result?id=free", "")
	if rec.Code != 404 {
		t.Fatal(rec.Code)
	}
	rec = disconnectRequest(d, "POST", "/desktop/stop?id=free", "")
	if rec.Code != 204 {
		t.Fatalf("stop: %d %s", rec.Code, rec.Body.String())
	}
	if remoteRequests.Load() != 0 {
		t.Fatal("free console contacted task tracker")
	}
	d.queue.mu.Lock()
	status := run.desktop.Status
	d.queue.mu.Unlock()
	if status != "canceled" {
		t.Fatal(status)
	}
	rec = disconnectRequest(d, "DELETE", "/desktop/history", "")
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte("free")) {
		t.Fatal(rec.Body.String())
	}
}

func TestFreeConsoleQueueCancellation(t *testing.T) {
	d := &agentDaemon{}
	// A shared-checkout execution owns the mapped repository, where a console runs too.
	first, _ := d.enqueueRun("task", agentconfig.Dispatch{RunID: "task"}, "project", "/repo", 2, false)
	if err := d.awaitRunSlot(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	run, _ := d.enqueueRun("", agentconfig.Dispatch{RunID: "console"}, "project", "/repo", 2, false)
	run.desktop.Kind = consoleRunKind
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if err := d.awaitRunSlot(ctx, run); err == nil {
		t.Fatal("free console bypassed checkout ownership")
	}
	d.queue.mu.Lock()
	run.canceled = true
	d.queue.mu.Unlock()
	d.launchConsole(run, "exec codex")
	select {
	case <-run.exited:
	default:
		t.Fatal("queued cancellation did not release run")
	}
	if run.desktop.Status != "canceled" {
		t.Fatal(run.desktop.Status)
	}
}

func TestConsoleAdmissionUsesLocalMappingAndQueue(t *testing.T) {
	d, config := disconnectFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/v1/agent/config" {
			t.Errorf("unexpected server request: %s %s", r.Method, r.URL)
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(config)
	}))
	defer server.Close()
	d.serverURL = server.URL
	// A shared-checkout execution keeps the mapped repository busy, so the console queues.
	first, err := d.enqueueRun("task", agentconfig.Dispatch{RunID: "active"}, "p", d.repoRoot, 3, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = d.awaitRunSlot(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	for _, provider := range []string{"codex", "claude"} {
		rec := disconnectRequest(d, "POST", "/desktop/consoles", `{"projectId":"p","provider":"`+provider+`"}`)
		var entry desktopRun
		if rec.Code != 202 || json.Unmarshal(rec.Body.Bytes(), &entry) != nil {
			t.Fatalf("admission: %d %s", rec.Code, rec.Body.String())
		}
		if entry.Kind != "console" || entry.Provider != provider || entry.TaskID != "" || entry.Prompt != "" || entry.Skill != "" || entry.Directory != d.repoRoot || entry.Status != "queued" {
			t.Fatalf("unexpected entry: %+v", entry)
		}
		d.queue.mu.Lock()
		run := d.queue.runs[entry.ID]
		isolated := run.isolated
		d.queue.mu.Unlock()
		if isolated {
			t.Fatal("console must reserve mapped checkout")
		}
		rec = disconnectRequest(d, "POST", "/desktop/stop?id="+entry.ID, "")
		if rec.Code != 204 {
			t.Fatalf("queue cancellation: %d", rec.Code)
		}
	}
	settings, err := agentconfig.ReadSettings(d.repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	settings.DisconnectedProjects = map[string]bool{"p": true}
	if err = agentconfig.WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	rec := disconnectRequest(d, "POST", "/desktop/consoles", `{"projectId":"p","provider":"codex"}`)
	if rec.Code != 409 {
		t.Fatalf("disconnected project: %d %s", rec.Code, rec.Body.String())
	}
}

func TestFreeConsoleExitStatus(t *testing.T) {
	for _, tt := range []struct{ command, status string }{{"exit 0", "completed"}, {"exec /nonexistent/sectile-test-cli", "failed"}} {
		t.Run(tt.status, func(t *testing.T) {
			d := &agentDaemon{terminalMgr: terminal.NewManager(), loopback: loopbackServer{desktopToken: "private"}}
			local := httptest.NewServer(http.HandlerFunc(d.handleRunControl))
			defer local.Close()
			d.loopback.url = local.URL
			run, err := d.enqueueRun("", agentconfig.Dispatch{RunID: "console"}, "p", t.TempDir(), 1, false)
			if err != nil {
				t.Fatal(err)
			}
			run.desktop.Kind = "console"
			d.launchConsole(run, tt.command)
			defer d.terminalMgr.CloseSession("console")
			select {
			case <-run.exited:
			case <-time.After(5 * time.Second):
				t.Fatal("console exit was not reported")
			}
			d.queue.mu.Lock()
			status, session := run.desktop.Status, run.desktop.SessionID
			d.queue.mu.Unlock()
			if status != tt.status || session != "console" {
				t.Fatalf("%s: %s %s", tt.command, status, session)
			}
		})
	}
}
