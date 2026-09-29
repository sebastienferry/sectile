package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"tasks/internal/agentconfig"
	"tasks/internal/terminal"
)

// The run store (#588): what the sidebar lists survives the agent.

func testRunStore(t *testing.T) *runStore {
	t.Helper()
	store := openRunStore(filepath.Join(t.TempDir(), "state", "runs"))
	if store == nil {
		t.Fatal("the run store did not open")
	}
	return store
}

func storedFiles(t *testing.T, store *runStore) []string {
	t.Helper()
	entries, err := os.ReadDir(store.dir)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestAStoredRunIsReadBackAsItWasWrittenAndStaysPrivate(t *testing.T) {
	store := testRunStore(t)
	created := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	finished := created.Add(time.Minute)
	record := storedRun{
		Run: desktopRun{ID: "run-1", TaskID: "task-1", TaskKey: "#1", ProjectID: "project", Skill: "discuss", Directory: "/work",
			Branch: "feat/1", Status: "completed", Provider: "claude", Model: "opus", Prompt: "hello", CreatedAt: created, StartedAt: created},
		Console:    []byte("\x1b[1mprompt\x1b[0m $ echo hi\r\nhi\r\n"),
		FinishedAt: finished,
	}
	if err := store.save(record); err != nil {
		t.Fatal(err)
	}
	if got := storedFiles(t, store); len(got) != 1 || got[0] != "run-1.json" {
		t.Fatalf("the store holds %v, want the run's file alone and no temporary file", got)
	}
	if runtime.GOOS != "windows" {
		directory, _ := os.Stat(store.dir)
		file, _ := os.Stat(store.path("run-1"))
		if directory.Mode().Perm() != 0700 || file.Mode().Perm() != 0600 {
			t.Fatalf("the store is %v and its file %v, want 0700 and 0600", directory.Mode().Perm(), file.Mode().Perm())
		}
	}

	loaded := openRunStore(store.dir).load()
	if len(loaded) != 1 {
		t.Fatalf("loaded %d runs, want 1", len(loaded))
	}
	got := loaded[0]
	if got.Run != record.Run || string(got.Console) != string(record.Console) || !got.FinishedAt.Equal(finished) {
		t.Fatalf("the run came back as %+v", got)
	}
}

func TestAnExistingStoreDirectoryIsMadePrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX modes")
	}
	dir := filepath.Join(t.TempDir(), "runs")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	openRunStore(dir)
	if info, _ := os.Stat(dir); info.Mode().Perm() != 0700 {
		t.Fatalf("the store directory stayed %v", info.Mode().Perm())
	}
}

func TestAFileThatIsNotAStoredRunIsSkippedAndKept(t *testing.T) {
	store := testRunStore(t)
	if err := store.save(storedRun{Run: desktopRun{ID: "../escape", Status: "completed"}}); err == nil {
		t.Fatal("a run id with a separator was written")
	}
	good := storedRun{Run: desktopRun{ID: "good", Status: "failed"}, FinishedAt: time.Now().UTC()}
	if err := store.save(good); err != nil {
		t.Fatal(err)
	}
	future, _ := json.Marshal(storedRun{Version: runStoreVersion + 1, Run: desktopRun{ID: "future", Status: "completed"}})
	other, _ := json.Marshal(storedRun{Version: runStoreVersion, Run: desktopRun{ID: "someone-else", Status: "completed"}})
	files := map[string]string{"corrupt.json": "{not json", "future.json": string(future), "named.json": string(other), "a.b.json": "{}", "notes.txt": "hi"}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(store.dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}

	loaded := openRunStore(store.dir).load()
	if len(loaded) != 1 || loaded[0].Run.ID != "good" {
		t.Fatalf("loaded %+v, want the valid run alone", loaded)
	}
	for name := range files {
		if _, err := os.Stat(filepath.Join(store.dir, name)); err != nil {
			t.Errorf("%s was not kept: %v", name, err)
		}
	}
}

func TestARunKilledWithTheAgentComesBackCanceledAndAnUnstartedOneIsDropped(t *testing.T) {
	store := testRunStore(t)
	for _, run := range []desktopRun{{ID: "running", Status: "running"}, {ID: "queued", Status: "queued"}, {ID: "preparing", Status: "preparing"}} {
		if err := store.save(storedRun{Run: run, Console: []byte("last write")}); err != nil {
			t.Fatal(err)
		}
	}

	loaded := openRunStore(store.dir).load()
	if len(loaded) != 1 || loaded[0].Run.ID != "running" {
		t.Fatalf("loaded %+v, want the running run alone", loaded)
	}
	if loaded[0].Run.Status != "canceled" || string(loaded[0].Console) != "last write" || loaded[0].FinishedAt.IsZero() {
		t.Fatalf("the interrupted run came back as %+v", loaded[0])
	}
	if got := storedFiles(t, store); len(got) != 1 || got[0] != "running.json" {
		t.Fatalf("the store holds %v, want the unstarted runs' files deleted", got)
	}
	// Written back: the next start reads the status the run was shown with.
	again := openRunStore(store.dir).load()
	if len(again) != 1 || again[0].Run.Status != "canceled" || !again[0].FinishedAt.Equal(loaded[0].FinishedAt) {
		t.Fatalf("the second start read %+v", again)
	}
}

func TestRetentionKeepsTheMostRecentFinishedRunsAndNeverALiveOne(t *testing.T) {
	store := testRunStore(t)
	base := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	for i := range runStoreRetained + 3 {
		record := storedRun{Run: desktopRun{ID: fmt.Sprintf("finished-%03d", i), Status: "completed"}, FinishedAt: base.Add(time.Duration(i) * time.Minute)}
		if err := store.save(record); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.save(storedRun{Run: desktopRun{ID: "live", Status: "running"}}); err != nil {
		t.Fatal(err)
	}

	// finished-000 finished first but is running again under the same id
	// here: a live run is never deleted, whatever its file says.
	store.prune(runStoreRetained, map[string]bool{"live": true, "finished-000": true})

	for _, kept := range []string{"live", "finished-000", "finished-003", fmt.Sprintf("finished-%03d", runStoreRetained+2)} {
		if _, err := os.Stat(store.path(kept)); err != nil {
			t.Errorf("%s was deleted", kept)
		}
	}
	for _, dropped := range []string{"finished-001", "finished-002"} {
		if _, err := os.Stat(store.path(dropped)); err == nil {
			t.Errorf("%s was kept past the cap", dropped)
		}
	}
	if got := len(storedFiles(t, store)); got != runStoreRetained+2 {
		t.Fatalf("the store holds %d files, want %d finished runs, the spared one and the live one", got, runStoreRetained+2)
	}
}

func TestAClearedRunIsNeverWrittenBack(t *testing.T) {
	store := testRunStore(t)
	record := storedRun{Run: desktopRun{ID: "cleared", Status: "completed"}, FinishedAt: time.Now().UTC()}
	if err := store.save(record); err != nil {
		t.Fatal(err)
	}
	store.remove("cleared")
	if err := store.save(record); err != nil {
		t.Fatal(err)
	}
	if got := storedFiles(t, store); len(got) != 0 {
		t.Fatalf("a write racing the clear brought back %v", got)
	}
}

func TestADaemonWithoutAStoreBehavesAsBefore(t *testing.T) {
	var store *runStore
	if err := store.save(storedRun{Run: desktopRun{ID: "x"}}); err != nil {
		t.Fatal(err)
	}
	store.remove("x")
	store.prune(0, nil)
	if store.load() != nil {
		t.Fatal("a disabled store loaded runs")
	}
	d := &agentDaemon{}
	d.restoreRuns()
	d.persistRuns()
	d.pruneRuns()
	d.tapConsole("x")
	if len(d.queue.runs) != 0 {
		t.Fatal("a daemon without a store restored runs")
	}
}

func TestAConsoleTapIsBoundedAsTheSessionHistoryIs(t *testing.T) {
	tap := &consoleTap{}
	tap.write([]byte(strings.Repeat("a", terminal.HistoryLimit)))
	tap.write([]byte("tail"))
	got, version := tap.snapshot()
	if len(got) != terminal.HistoryLimit || !strings.HasSuffix(string(got), "tail") || version != 2 {
		t.Fatalf("the tap holds %d bytes at version %d", len(got), version)
	}
}

func TestARunKeepsItsConsoleOutputAfterItsSessionCloses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell")
	}
	d := &agentDaemon{terminal: terminalChoice{manager: terminal.NewManager()}, store: testRunStore(t)}
	d.queue.runs = map[string]*controlledRun{"tapped": {desktop: desktopRun{ID: "tapped", Status: "running"}, exited: make(chan struct{})}}
	if _, err := d.terminal.manager.GetOrCreateSession("tapped", t.TempDir(), nil); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.terminal.manager.CloseSession("tapped") }()
	time.Sleep(time.Second)
	d.tapConsole("tapped")
	d.tapConsole("tapped") // A run given its console twice is tapped once.
	if err := d.terminal.manager.SendInput("tapped", "echo SEC\"-\"588\n"); err != nil {
		t.Fatal(err)
	}
	tap := d.queue.runs["tapped"].console
	deadline := time.Now().Add(10 * time.Second)
	for {
		output, _ := tap.snapshot()
		if strings.Contains(string(output), "SEC-588") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the tap never saw the command's output: %q", output)
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = d.terminal.manager.CloseSession("tapped")
	if output, _ := tap.snapshot(); !strings.Contains(string(output), "SEC-588") {
		t.Fatal("the run lost its output with its session")
	}
	if strings.Count(func() string { o, _ := tap.snapshot(); return string(o) }(), "SEC-588") != 1 {
		t.Fatal("the output was copied twice")
	}
}

func TestAFinishedRunIsWrittenAtOnceAndAQuietOneIsNotRewritten(t *testing.T) {
	d := &agentDaemon{store: testRunStore(t)}
	run, err := d.enqueueRun("task-1", agentconfig.Dispatch{RunID: "finishing", TaskKey: "#1", SkillID: "implement"}, "project", "/work", 1, false)
	if err != nil {
		t.Fatal(err)
	}
	// Not started: nothing to show, nothing written.
	d.persistRuns()
	if got := storedFiles(t, d.store); len(got) != 0 {
		t.Fatalf("a queued run was written: %v", got)
	}

	tap := &consoleTap{}
	tap.write([]byte("working\r\n"))
	d.queue.read("finishing", func(run *controlledRun) { run.desktop.Status = "running"; run.console = tap })
	d.persistRuns()
	if _, err := os.Stat(d.store.path("finishing")); err != nil {
		t.Fatalf("a running run was not written: %v", err)
	}
	// Unchanged since: the flush leaves it alone.
	_ = os.Remove(d.store.path("finishing"))
	d.persistRuns()
	if _, err := os.Stat(d.store.path("finishing")); err == nil {
		t.Fatal("a quiet run was written again")
	}
	tap.write([]byte("more\r\n"))
	d.persistRuns()
	if _, err := os.Stat(d.store.path("finishing")); err != nil {
		t.Fatal("new output was not written")
	}

	d.queue.mu.Lock()
	run.desktop.Status = "completed"
	run.desktop.WaitingSince = time.Now()
	run.once.Do(func() { close(run.exited) })
	d.queue.mu.Unlock()
	deadline := time.Now().Add(5 * time.Second)
	for {
		// Read as a file: loading would treat the store as a previous agent's.
		var stored storedRun
		raw, _ := os.ReadFile(d.store.path("finishing"))
		_ = json.Unmarshal(raw, &stored)
		if stored.Run.Status == "completed" {
			if stored.FinishedAt.IsZero() || !stored.Run.WaitingSince.IsZero() || string(stored.Console) != "working\r\nmore\r\n" {
				t.Fatalf("the finished run was written as %+v", stored)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the finished run was not written at once: %+v", stored)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// restoredDaemon is an agent started on a store left by a previous one.
func restoredDaemon(t *testing.T) *agentDaemon {
	t.Helper()
	store := testRunStore(t)
	at := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	for _, record := range []storedRun{
		{Run: desktopRun{ID: "discussion", TaskID: "task-1", TaskKey: "#1", ProjectID: "p", Skill: "discuss", SessionID: "discussion", Directory: "/work", Status: "completed", CreatedAt: at, WaitingSince: at, ExternalTerminal: "iterm"}, Console: []byte("asked\r\nanswered\r\n"), FinishedAt: at},
		{Run: desktopRun{ID: "headless", TaskID: "task-2", TaskKey: "#2", ProjectID: "p", Skill: "implement", Status: "failed", Headless: true, Trace: true, CreatedAt: at.Add(time.Second)}, Trace: []string{"first\r\n", "second\r\n"}, FinishedAt: at},
		{Run: desktopRun{ID: "macro", TaskKey: "M-1", MacroKey: "M-1", ProjectID: "p", Skill: "refine_macro", SessionID: "macro", Status: "running", CreatedAt: at.Add(2 * time.Second)}, Console: []byte("refining")},
	} {
		if err := store.save(record); err != nil {
			t.Fatal(err)
		}
	}
	d := &agentDaemon{terminal: terminalChoice{manager: terminal.NewManager()}, loopback: loopbackServer{desktopToken: "private"}, store: openRunStore(store.dir)}
	d.restoreRuns()
	return d
}

func TestRestoredRunsAreListedWithoutASession(t *testing.T) {
	d := restoredDaemon(t)
	request := httptest.NewRequest("GET", "/desktop/runs", nil)
	request.Header.Set("Authorization", "Bearer private")
	response := httptest.NewRecorder()
	d.desktopHandler(response, request)
	var runs []desktopRun
	if err := json.Unmarshal(response.Body.Bytes(), &runs); err != nil {
		t.Fatal(err)
	}
	byID := map[string]desktopRun{}
	for _, run := range runs {
		byID[run.ID] = run
	}
	if len(byID) != 3 {
		t.Fatalf("listed %v", runs)
	}
	for id, run := range byID {
		if !run.Restored || run.SessionID != "" || !run.WaitingSince.IsZero() || run.CancelRequested || run.ExternalTerminal != "" {
			t.Errorf("%s is listed as %+v", id, run)
		}
	}
	if byID["discussion"].TaskKey != "#1" || byID["discussion"].Skill != "discuss" || byID["discussion"].Directory != "/work" {
		t.Errorf("the discussion lost its record: %+v", byID["discussion"])
	}
	if byID["macro"].Status != "canceled" || byID["macro"].MacroKey != "M-1" {
		t.Errorf("the macro run killed with the agent is %+v", byID["macro"])
	}
	if !byID["headless"].Headless || !byID["headless"].Trace || byID["headless"].Status != "failed" {
		t.Errorf("the headless run is %+v", byID["headless"])
	}
}

func TestARestoredRunReplaysWhatItShowedAndEnds(t *testing.T) {
	d := restoredDaemon(t)
	server := httptest.NewServer(http.HandlerFunc(d.desktopHandler))
	defer server.Close()
	header := http.Header{"Authorization": []string{"Bearer private"}}
	replay := func(id string) []string {
		t.Helper()
		connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/desktop/terminal?id="+id, header)
		if err != nil {
			t.Fatal(err)
		}
		defer connection.Close()
		// Nobody is answering a restored run: the frame is discarded.
		_ = connection.WriteJSON(map[string]string{"type": "input", "data": "hello\n"})
		chunks := []string{}
		for {
			_ = connection.SetReadDeadline(time.Now().Add(5 * time.Second))
			_, chunk, err := connection.ReadMessage()
			if err != nil {
				if _, closed := err.(*websocket.CloseError); !closed && !strings.Contains(err.Error(), "EOF") {
					t.Fatalf("%s: the replay did not end: %v", id, err)
				}
				return chunks
			}
			chunks = append(chunks, string(chunk))
		}
	}
	if got := replay("discussion"); strings.Join(got, "") != "asked\r\nanswered\r\n" {
		t.Fatalf("the discussion replayed %q", got)
	}
	if got := replay("headless"); strings.Join(got, "") != "first\r\nsecond\r\n" {
		t.Fatalf("the headless run replayed %q", got)
	}
	if got := replay("macro"); strings.Join(got, "") != "refining" {
		t.Fatalf("the macro run replayed %q", got)
	}
	if sessions := d.terminal.manager.ListSessions(); len(sessions) != 0 {
		t.Fatalf("replaying spawned sessions: %v", sessions)
	}
}

func TestClearingFinishedConsolesDeletesTheirStoredRuns(t *testing.T) {
	d := restoredDaemon(t)
	live, err := d.enqueueRun("task-3", agentconfig.Dispatch{RunID: "live", TaskKey: "#3", SkillID: "implement"}, "p", "/work", 1, false)
	if err != nil {
		t.Fatal(err)
	}
	d.queue.read("live", func(run *controlledRun) { run.desktop.Status = "running" })
	d.persistRuns()
	defer live.once.Do(func() { close(live.exited) })

	request := httptest.NewRequest(http.MethodDelete, "/desktop/history", nil)
	request.Header.Set("Authorization", "Bearer private")
	response := httptest.NewRecorder()
	d.desktopHandler(response, request)
	if response.Code != 200 {
		t.Fatalf("clearing answered %d", response.Code)
	}
	if got := storedFiles(t, d.store); len(got) != 1 || got[0] != "live.json" {
		t.Fatalf("the store holds %v after clearing, want the live run alone", got)
	}
	if len(d.queue.runs) != 1 || d.queue.runs["live"] == nil {
		t.Fatalf("clearing left %v", d.queue.runs)
	}
}

func TestARestoredRunIdCannotBeRegisteredAgain(t *testing.T) {
	d := restoredDaemon(t)
	if _, err := d.enqueueRun("task-1", agentconfig.Dispatch{RunID: "discussion"}, "p", "/work", 1, false); err == nil {
		t.Fatal("a restored run id was queued again")
	}
	if _, err := d.wrapRun("task-1", "discussion", "true"); err == nil {
		t.Fatal("a restored run id was given a new control token")
	}
	// A stop on a restored run finds it exited, as any finished run.
	request := httptest.NewRequest(http.MethodPost, "/desktop/stop?id=discussion", nil)
	request.Header.Set("Authorization", "Bearer private")
	response := httptest.NewRecorder()
	d.desktopHandler(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("stopping a restored run answered %d", response.Code)
	}
}
