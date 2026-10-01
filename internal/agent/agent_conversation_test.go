package agent

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"tasks/internal/agentconfig"
	"tasks/internal/models"
	"tasks/internal/testhome"
	"testing"
	"time"
)

func conversationRequest(d *agentDaemon, method, path, body, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	d.desktopHandler(w, r)
	return w
}

func conversationFixture(t *testing.T) (*agentDaemon, string) {
	t.Helper()
	d := &agentDaemon{loopback: loopbackServer{desktopToken: "private"}}
	d.queue.runs = map[string]*controlledRun{"source": {desktop: desktopRun{ProjectID: "project", Provider: "claude", Directory: t.TempDir(), Branch: "feat/test"}}}
	w := conversationRequest(d, "POST", "/desktop/conversation", `{"sourceRunId":"source"}`, "private")
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var entry desktopRun
	if err := json.Unmarshal(w.Body.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.stopConversations)
	return d, entry.ID
}

func TestConversationRequiresDesktopAuthenticationAndOwnedDirectory(t *testing.T) {
	d, id := conversationFixture(t)
	if w := conversationRequest(d, "GET", "/desktop/conversation?id="+id, "", "wrong"); w.Code != 401 {
		t.Fatal("conversation bypassed authentication")
	}
	if w := conversationRequest(d, "POST", "/desktop/conversation", `{"sourceRunId":"unknown","directory":"/tmp"}`, "private"); w.Code != 404 {
		t.Fatal("unregistered directory admitted")
	}
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":" "}`, "private"); w.Code != 400 {
		t.Fatal("empty turn admitted")
	}
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"hello","effort":"--dangerously-skip-permissions"}`, "private"); w.Code != 400 {
		t.Fatal("unknown effort admitted")
	}
	d.queue.mu.Lock()
	d.queue.runs[id].conversation.busy = true
	d.queue.mu.Unlock()
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"hello"}`, "private"); w.Code != 409 {
		t.Fatal("concurrent turn admitted")
	}
	d.queue.mu.Lock()
	d.queue.runs[id].conversation.busy = false
	d.queue.mu.Unlock()
	if w := conversationRequest(d, "POST", "/desktop/stop?id="+id, "", "private"); w.Code != 204 {
		t.Fatalf("stop idle: %d", w.Code)
	}
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"hello"}`, "private"); w.Code != 409 {
		t.Fatal("closed conversation resumed")
	}
}

func TestConversationCommandPassesPromptWithoutShellInterpretation(t *testing.T) {
	prompt := "--help\n$(touch should-not-exist) `echo unsafe`"
	cmd := claudeConversationCommand(t.TempDir(), "sonnet", "high", "session", prompt, nil, nil)
	if got := strings.Join(cmd.Args, " "); strings.Contains(got, "touch") || !strings.Contains(got, "--resume session") || !strings.Contains(got, "--model sonnet") || !strings.Contains(got, "--effort high") || strings.Contains(got, "bypassPermissions") {
		t.Fatalf("unsafe or incomplete command: %s", got)
	}
	data, err := io.ReadAll(cmd.Stdin)
	if err != nil || string(data) != prompt {
		t.Fatal("prompt was not preserved on stdin")
	}
}

// Each folder is one argument, whatever it holds, and the folder map reaches
// the session's environment (#676).
func TestConversationCommandCarriesTheProjectFolders(t *testing.T) {
	plain := claudeConversationCommand(t.TempDir(), "", "", "", "hello", nil, nil)
	cmd := claudeConversationCommand(t.TempDir(), "", "", "", "hello", []string{"/a", "/b c"}, map[string]string{"SECTILE_REPOSITORIES": `[{"path":"/a"}]`})
	if got := cmd.Args[len(plain.Args):]; len(got) != 2 || got[0] != "--add-dir=/a" || got[1] != "--add-dir=/b c" {
		t.Fatalf("folder arguments = %q", got)
	}
	for _, arg := range plain.Args {
		if strings.HasPrefix(arg, "--add-dir") {
			t.Fatalf("a turn without folders changed: %q", plain.Args)
		}
	}
	found := false
	for _, entry := range cmd.Env {
		found = found || entry == `SECTILE_REPOSITORIES=[{"path":"/a"}]`
	}
	if !found {
		t.Fatal("the folder map is not in the environment")
	}
}

// A folder attached between two turns is given to the second one, which also
// receives the folder map (#676).
func TestConversationTurnReadsTheProjectFoldersEachTime(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake CLI is a POSIX script")
	}
	testhome.Temp(t)
	bin := t.TempDir()
	script := `#!/bin/sh
{ printf '%s\n' "$@"; printf 'REPOS=%s\n---\n' "$SECTILE_REPOSITORIES"; } >> "$CONVERSATION_ARGS"
printf '%s\n' '{"type":"result","is_error":false,"result":"ok","session_id":"11111111-1111-4111-8111-111111111111"}'
`
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	argsFile := filepath.Join(t.TempDir(), "args.txt")
	t.Setenv("CONVERSATION_ARGS", argsFile)
	root := checkoutOf(t, "git@github.com:o/a.git")
	notes := t.TempDir()
	if err := agentconfig.WriteSettings(agentconfig.Settings{ProjectSettings: map[string]agentconfig.ProjectSettings{"p": {Path: root}}}); err != nil {
		t.Fatal(err)
	}
	d, _, do := desktopAgent(t, root, models.Task{})
	d.queue.runs = map[string]*controlledRun{"source": {desktop: desktopRun{ProjectID: "p", Provider: "claude", Directory: root}}}
	w := do("POST", "/desktop/conversation", map[string]string{"sourceRunId": "source"})
	var entry desktopRun
	if w.Code != http.StatusCreated || json.Unmarshal(w.Body.Bytes(), &entry) != nil {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	t.Cleanup(d.stopConversations)
	run := d.queue.runs[entry.ID]
	turn := func() {
		t.Helper()
		if w := do("POST", "/desktop/conversation?id="+entry.ID, map[string]string{"message": "list"}); w.Code != 202 {
			t.Fatalf("send: %d %s", w.Code, w.Body.String())
		}
		deadline := time.Now().Add(10 * time.Second)
		for {
			d.queue.mu.Lock()
			busy := run.conversation.busy
			d.queue.mu.Unlock()
			if !busy {
				return
			}
			if time.Now().After(deadline) {
				t.Fatal("Claude turn did not finish")
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	turn()
	if w := do("POST", "/desktop/folders", map[string]any{"projectId": "p", "path": notes}); w.Code != 204 {
		t.Fatalf("attach: %d %s", w.Code, w.Body.String())
	}
	turn()
	raw, _ := os.ReadFile(argsFile)
	turns := strings.Split(string(raw), "---\n")
	if len(turns) < 2 {
		t.Fatalf("turns = %q", raw)
	}
	// o/b and o/c have no folder here: the first turn has nothing to add.
	if strings.Contains(turns[0], "--add-dir") {
		t.Fatalf("first turn had a folder: %s", turns[0])
	}
	if !strings.Contains(turns[1], "--add-dir="+notes+"\n") || !strings.Contains(turns[1], notes) || !strings.Contains(turns[1], `"attached":true`) {
		t.Fatalf("second turn misses the attached folder or its map: %s", turns[1])
	}
	lines, _ := run.trace.snapshot()
	if text := strings.Join(lines, "\n"); strings.Contains(text, "could not be read") {
		t.Fatalf("a readable project was reported unreadable: %s", text)
	}
}

func TestConversationOutputHandlesSplitLargeAndUnterminatedFrames(t *testing.T) {
	var lines []string
	o := &conversationOutput{line: func(s string) { lines = append(lines, s) }}
	frame := strings.Repeat("x", 100000)
	_, _ = o.Write([]byte(frame[:100]))
	_, _ = o.Write([]byte(frame[100:] + "\r\nnext"))
	if len(lines) != 1 || lines[0] != frame || string(o.buffer) != "next" {
		t.Fatal("fragmented large frame lost")
	}
	_, _ = o.Write([]byte(strings.Repeat("x", 2*1024*1024) + "\nlast\n"))
	if len(lines) != 3 || lines[2] != "last" || len(o.buffer) != 0 {
		t.Fatal("oversized frame prevented recovery")
	}
}

func TestConversationRunsAndResumesClaudeWithoutPTY(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake CLI is a POSIX script; Windows launch flags have a separate test")
	}
	testhome.Temp(t)
	bin := t.TempDir()
	script := `#!/bin/sh
cat > prompt.txt
printf '%s\n' "$@" >> args.txt
printf '%s\n' '{"type":"system","subtype":"init","model":"claude-main","session_id":"11111111-1111-4111-8111-111111111111"}'
printf '%s\n' '{"type":"assistant","parent_tool_use_id":null,"message":{"usage":{"input_tokens":10,"cache_creation_input_tokens":2000,"cache_read_input_tokens":40000,"output_tokens":90},"content":[{"type":"text","text":"Hello"},{"type":"tool_use","name":"Read","input":{"file_path":"file.go"}}]}}'
printf '%s\n' '{"type":"assistant","parent_tool_use_id":"toolu_1","message":{"usage":{"input_tokens":900000,"output_tokens":1},"content":[]}}'
printf '%s\n' '{"type":"result","is_error":false,"result":"Hello","session_id":"11111111-1111-4111-8111-111111111111","modelUsage":{"claude-main":{"contextWindow":200000},"claude-subagent":{"contextWindow":1000000}}}'
`
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	d, id := conversationFixture(t)
	run := d.queue.runs[id]
	for _, prompt := range []string{"first", "second $(touch unsafe)"} {
		body, _ := json.Marshal(map[string]string{"message": prompt, "effort": "xhigh"})
		w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, string(body), "private")
		if w.Code != 202 {
			t.Fatalf("send: %d %s", w.Code, w.Body.String())
		}
		if !json.Valid(w.Body.Bytes()) {
			t.Fatal("Electron requires a JSON acknowledgement for a 202 response")
		}
		deadline := time.Now().Add(5 * time.Second)
		for {
			d.queue.mu.Lock()
			busy := run.conversation.busy
			d.queue.mu.Unlock()
			if !busy {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("Claude turn did not finish")
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	args, err := os.ReadFile(filepath.Join(run.root, "args.txt"))
	if err != nil || !strings.Contains(string(args), "--resume\n11111111-1111-4111-8111-111111111111") || strings.Count(string(args), "--effort\nxhigh") != 2 {
		t.Fatalf("session not resumed: %s %v", args, err)
	}
	prompt, _ := os.ReadFile(filepath.Join(run.root, "prompt.txt"))
	if string(prompt) != "second $(touch unsafe)" {
		t.Fatalf("prompt changed: %s", prompt)
	}
	if _, err := os.Stat(filepath.Join(run.root, "unsafe")); !os.IsNotExist(err) {
		t.Fatal("prompt interpreted by shell")
	}
	lines, _ := run.trace.snapshot()
	text := strings.Join(lines, "\n")
	if strings.Count(text, `"kind":"assistant"`) != 2 || !strings.Contains(text, `"kind":"tool"`) || strings.Contains(text, `"kind":"error"`) {
		t.Fatalf("unexpected transcript: %s", text)
	}
	// This agent has no server to read the project from: each turn says it
	// runs without the attached folders, and still runs.
	if strings.Count(text, "Attached folders could not be read for this message") != 2 || strings.Contains(string(args), "--add-dir") {
		t.Fatalf("an unreadable project must be reported and carry no folder: %s %s", text, args)
	}
	w := conversationRequest(d, "GET", "/desktop/conversation?id="+id, "", "private")
	if body := w.Body.String(); !strings.Contains(body, `"context":{"used":42100,"window":200000}`) || !strings.Contains(body, `"effort":"xhigh"`) {
		t.Fatalf("context or effort not reported, or a subagent counted: %s", body)
	}
	if run.desktop.SessionID != "" || !run.desktop.Headless {
		t.Fatal("conversation acquired a PTY")
	}
}

func TestConversationHistoryRestoresReadOnly(t *testing.T) {
	store := testRunStore(t)
	entry := desktopRun{ID: "chat", Conversation: true, Headless: true, Kind: consoleRunKind, Provider: "claude", ProjectID: "project", Status: "canceled", CreatedAt: time.Now().UTC()}
	if err := store.save(storedRun{Run: entry, Trace: []string{`{"kind":"assistant","text":"Saved answer"}`}, FinishedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	d := &agentDaemon{loopback: loopbackServer{desktopToken: "private"}, store: openRunStore(store.dir)}
	d.restoreRuns()
	w := conversationRequest(d, "GET", "/desktop/conversation?id=chat", "", "private")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"readOnly":true`) || !strings.Contains(w.Body.String(), "Saved answer") {
		t.Fatalf("history missing: %d %s", w.Code, w.Body.String())
	}
	if w := conversationRequest(d, "POST", "/desktop/conversation?id=chat", `{"message":"resume"}`, "private"); w.Code != 409 {
		t.Fatal("restored chat launched a process")
	}
}

func TestConversationStopTerminatesActiveProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake CLI is a POSIX script")
	}
	testhome.Temp(t)
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\nexec sleep 300\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	d, id := conversationFixture(t)
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"wait"}`, "private"); w.Code != 202 {
		t.Fatal(w.Code)
	}
	if w := conversationRequest(d, "POST", "/desktop/stop?id="+id, "", "private"); w.Code != 204 {
		t.Fatalf("stop busy: %d %s", w.Code, w.Body.String())
	}
	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()
	if d.queue.runs[id].desktop.Status != "canceled" || d.queue.runs[id].conversation.busy {
		t.Fatal("stop did not finish conversation")
	}
}
