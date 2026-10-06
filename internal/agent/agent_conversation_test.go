package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
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
	// No real Claude is started to list commands; a test that wants a list sets its own.
	d.probeCommandsFn = func(*exec.Cmd) []conversationSlash { return nil }
	d.checkMCPFn = func(string, map[string]string) conversationMCP { return conversationMCP{Status: "unknown"} }
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
	// A message while Claude works, with no turn input to join, waits for the
	// next turn instead of starting a second one.
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"hello"}`, "private"); w.Code != 202 || !strings.Contains(w.Body.String(), `"joined":false`) {
		t.Fatalf("a message while busy was not kept: %d %s", w.Code, w.Body.String())
	}
	d.queue.mu.Lock()
	if next := d.queue.runs[id].conversation.next; len(next) != 1 || next[0] != "hello" {
		t.Errorf("next = %v", next)
	}
	d.queue.runs[id].conversation.busy = false
	d.queue.runs[id].conversation.next = nil
	d.queue.mu.Unlock()
	if w := conversationRequest(d, "POST", "/desktop/stop?id="+id, "", "private"); w.Code != 204 {
		t.Fatalf("stop idle: %d", w.Code)
	}
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"hello"}`, "private"); w.Code != 409 {
		t.Fatal("closed conversation resumed")
	}
}

func TestConversationCommandTakesTheMessageOnStdinOnly(t *testing.T) {
	cmd := claudeConversationCommand(t.TempDir(), "sonnet", "high", "", "session", "", nil, nil)
	got := strings.Join(cmd.Args, " ")
	for _, want := range []string{"--resume session", "--model sonnet", "--effort high", "--input-format stream-json", "--permission-prompt-tool stdio", "--allowedTools=mcp__sectile", "--include-partial-messages"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q: %s", want, got)
		}
	}
	if strings.Contains(got, "bypassPermissions") || cmd.Stdin != nil {
		t.Fatalf("unsafe command, or a message fixed before the turn: %s", got)
	}
}

// Each folder is one argument, whatever it holds, and the folder map reaches
// the session's environment (#676).
func TestConversationCommandCarriesTheProjectFolders(t *testing.T) {
	plain := claudeConversationCommand(t.TempDir(), "", "", "", "", "", nil, nil)
	cmd := claudeConversationCommand(t.TempDir(), "", "", "", "", "", []string{"/a", "/b c"}, map[string]string{"SECTILE_REPOSITORIES": `[{"path":"/a"}]`})
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
read -r init
read -r message
printf '%s' "$message" > prompt.txt
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
	raw, _ := os.ReadFile(filepath.Join(run.root, "prompt.txt"))
	var prompt struct {
		Type    string `json:"type"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal(raw, &prompt) != nil || prompt.Type != "user" || prompt.Message.Content != "second $(touch unsafe)" {
		t.Fatalf("prompt changed: %s", raw)
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

// A reply streams as a draft while Claude writes it; the trace keeps only the
// complete message, and a subagent's text never enters the draft.
func TestConversationStreamsTheReplyInProgress(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake CLI is a POSIX script")
	}
	testhome.Temp(t)
	bin := t.TempDir()
	script := `#!/bin/sh
read -r init
read -r message
printf '%s\n' '{"type":"stream_event","parent_tool_use_id":null,"event":{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}}'
printf '%s\n' '{"type":"stream_event","parent_tool_use_id":null,"event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello **wor"}}}'
printf '%s\n' '{"type":"stream_event","parent_tool_use_id":"toolu_9","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"subagent"}}}'
printf '%s\n' '{"type":"stream_event","parent_tool_use_id":null,"event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ld**"}}}'
while [ ! -f release ]; do sleep 0.02; done
printf '%s\n' '{"type":"assistant","parent_tool_use_id":null,"message":{"content":[{"type":"text","text":"Hello **world**"}]}}'
printf '%s\n' '{"type":"stream_event","parent_tool_use_id":null,"event":{"type":"content_block_stop","index":0}}'
printf '%s\n' '{"type":"result","is_error":false,"result":"Hello **world**","session_id":"11111111-1111-4111-8111-111111111111"}'
`
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	d, id := conversationFixture(t)
	run := d.queue.runs[id]
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"hi"}`, "private"); w.Code != 202 {
		t.Fatalf("send: %d %s", w.Code, w.Body.String())
	}
	read := func() (partial string, busy bool) {
		var body struct {
			Partial string `json:"partial"`
			Busy    bool   `json:"busy"`
		}
		_ = json.Unmarshal(conversationRequest(d, "GET", "/desktop/conversation?id="+id, "", "private").Body.Bytes(), &body)
		return body.Partial, body.Busy
	}
	deadline := time.Now().Add(5 * time.Second)
	for partial, _ := read(); partial != "Hello **world**"; partial, _ = read() {
		if time.Now().After(deadline) {
			t.Fatalf("the draft never reached the streamed text, last %q", partial)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if lines, _ := run.trace.snapshot(); strings.Contains(strings.Join(lines, "\n"), `"kind":"assistant"`) {
		t.Fatalf("a draft entered the trace: %v", lines)
	}
	if err := os.WriteFile(filepath.Join(run.root, "release"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	for partial, busy := read(); busy || partial != ""; partial, busy = read() {
		if time.Now().After(deadline) {
			t.Fatalf("the turn did not end or kept its draft: busy=%v partial=%q", busy, partial)
		}
		time.Sleep(10 * time.Millisecond)
	}
	lines, _ := run.trace.snapshot()
	text := strings.Join(lines, "\n")
	if strings.Count(text, `"kind":"assistant"`) != 1 || strings.Contains(text, "subagent") || strings.Contains(text, `"kind":"error"`) {
		t.Fatalf("unexpected transcript: %s", text)
	}
}

// conversationTurnTrace sends one message, waits for the turn to end and
// returns the trace it left.
func conversationTurnTrace(t *testing.T, script string) (*agentDaemon, *controlledRun, string) {
	t.Helper()
	testhome.Temp(t)
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	d, id := conversationFixture(t)
	run := d.queue.runs[id]
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"hi"}`, "private"); w.Code != 202 {
		t.Fatalf("send: %d %s", w.Code, w.Body.String())
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
	lines, _ := run.trace.snapshot()
	return d, run, strings.Join(lines, "\n")
}

// A JSON frame Sectile does not read, or reads under another shape, never
// reaches the chat as a raw error, and the fields Sectile does read survive it.
func TestConversationKeepsUnexpectedFramesOutOfTheChat(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake CLI is a POSIX script")
	}
	d, run, text := conversationTurnTrace(t, `#!/bin/sh
read -r init
read -r message
printf '%s\n' '{"type":"system","subtype":"ui_invalidate","event":"ui.render","uuid":"b1c51aa9-74b3-4db0-9710-8496455ac164","session_id":"209887cc-9814-45f6-a711-c66f1dde5d22"}'
printf '%s\n' '{"type":"system","subtype":"init","event":"ui.render","model":"claude-main","session_id":"209887cc-9814-45f6-a711-c66f1dde5d22"}'
printf '%s\n' '{"type":"stream_event","parent_tool_use_id":null,"event":"ui.render"}'
printf '%s\n' '{"foo":1}'
printf '%s\n' '[]'
printf '%s\n' '{"type":"assistant","parent_tool_use_id":null,"message":{"content":[{"type":"text","text":"Hello"}]}}'
printf '%s\n' '{"type":"result","is_error":false,"result":"Hello","session_id":"11111111-1111-4111-8111-111111111111","modelUsage":{"claude-main":{"contextWindow":200000},"claude-subagent":{"contextWindow":1000000}}}'
`)
	if strings.Contains(text, `"kind":"error"`) || strings.Count(text, `"kind":"assistant"`) != 1 {
		t.Fatalf("unexpected transcript: %s", text)
	}
	d.queue.mu.Lock()
	session, window, partial := run.conversation.session, run.conversation.contextWindow, run.conversation.partial
	d.queue.mu.Unlock()
	if session != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("session = %q, want the result's", session)
	}
	// The main model's window only wins when the init frame's model was read
	// despite its string event.
	if window != 200000 {
		t.Fatalf("context window = %d, want the main model's 200000", window)
	}
	if partial != "" {
		t.Fatalf("draft = %q, want none", partial)
	}
}

// A stdout line that is not JSON at all is still shown as an error.
func TestConversationShowsNonJSONLinesAsErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake CLI is a POSIX script")
	}
	_, _, text := conversationTurnTrace(t, `#!/bin/sh
read -r init
read -r message
printf '%s\n' 'plain diagnostic line'
printf '%s\n' '   '
printf '%s\n' '{"type":"result","is_error":false,"result":"Hello","session_id":"11111111-1111-4111-8111-111111111111"}'
`)
	if strings.Count(text, `"kind":"error"`) != 1 || !strings.Contains(text, "plain diagnostic line") {
		t.Fatalf("unexpected transcript: %s", text)
	}
}

// Interrupting stops the answer in progress and keeps the conversation open.
func TestConversationInterruptKeepsTheConversation(t *testing.T) {
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
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"interrupt":true}`, "private"); w.Code != http.StatusConflict {
		t.Fatalf("an idle conversation accepted an interrupt: %d", w.Code)
	}
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"wait"}`, "private"); w.Code != 202 {
		t.Fatal(w.Code)
	}
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"interrupt":true}`, "private"); w.Code != 202 {
		t.Fatalf("interrupt: %d %s", w.Code, w.Body.String())
	}
	run := d.queue.runs[id]
	deadline := time.Now().Add(10 * time.Second)
	for {
		d.queue.mu.Lock()
		busy, status := run.conversation.busy, run.desktop.Status
		d.queue.mu.Unlock()
		if !busy {
			if status != "running" {
				t.Fatalf("an interrupt ended the conversation: %s", status)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the interrupted turn never stopped")
		}
		time.Sleep(20 * time.Millisecond)
	}
	lines, _ := run.trace.snapshot()
	if text := strings.Join(lines, "\n"); !strings.Contains(text, `"text":"Interrupted"`) || strings.Contains(text, `"kind":"error"`) {
		t.Fatalf("unexpected transcript: %s", text)
	}
	if w := conversationRequest(d, "GET", "/desktop/conversation?id="+id, "", "private"); strings.Contains(w.Body.String(), `"readOnly":true`) {
		t.Fatalf("the conversation became read-only: %s", w.Body.String())
	}
}

// The terminal button opens a plain terminal on the conversation's directory,
// and nothing once the conversation has ended.
func TestConversationTerminalOpensItsDirectory(t *testing.T) {
	testhome.Temp(t)
	d, id := conversationFixture(t)
	var opened []string
	d.openTerminalFn = func(_, directory string) error { opened = append(opened, directory); return nil }
	if w := conversationRequest(d, "POST", "/desktop/conversation-terminal", `{"runId":"`+id+`"}`, "private"); w.Code != 200 {
		t.Fatalf("terminal: %d %s", w.Code, w.Body.String())
	}
	if len(opened) != 1 || opened[0] != d.queue.runs[id].desktop.Directory {
		t.Fatalf("the terminal did not open the conversation's directory: %v", opened)
	}
	if w := conversationRequest(d, "POST", "/desktop/stop?id="+id, "", "private"); w.Code != 204 {
		t.Fatalf("stop: %d", w.Code)
	}
	if w := conversationRequest(d, "POST", "/desktop/conversation-terminal", `{"runId":"`+id+`"}`, "private"); w.Code != http.StatusNotFound || len(opened) != 1 {
		t.Fatalf("an ended conversation opened a terminal: %d", w.Code)
	}
}

// A tool call Claude may not make on its own waits for the owner; the
// decision goes back to Claude on stdin and is kept in the trace.
func TestConversationAsksTheOwnerBeforeAToolCall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake CLI is a POSIX script")
	}
	testhome.Temp(t)
	bin := t.TempDir()
	script := `#!/bin/sh
read -r init
read -r message
printf '%s\n' '{"type":"assistant","parent_tool_use_id":null,"message":{"content":[{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"make deploy"}}]}}'
printf '%s\n' '{"type":"control_request","request_id":"req-1","request":{"subtype":"can_use_tool","tool_name":"Bash","tool_use_id":"toolu_1","description":"Deploy","input":{"command":"make deploy"},"permission_suggestions":[{"type":"addRules","rules":[{"toolName":"Bash","ruleContent":"make deploy"}],"behavior":"allow","destination":"localSettings"}]}}'
read -r answer
printf '%s' "$answer" > answer.txt
printf '%s\n' '{"type":"result","is_error":false,"result":"Deployed","session_id":"11111111-1111-4111-8111-111111111111"}'
`
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	d, id := conversationFixture(t)
	run := d.queue.runs[id]
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"deploy"}`, "private"); w.Code != 202 {
		t.Fatalf("send: %d", w.Code)
	}
	var body struct {
		Busy      bool                   `json:"busy"`
		Approvals []conversationApproval `json:"approvals"`
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(body.Approvals) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the tool call never waited for the owner")
		}
		time.Sleep(10 * time.Millisecond)
		_ = json.Unmarshal(conversationRequest(d, "GET", "/desktop/conversation?id="+id, "", "private").Body.Bytes(), &body)
	}
	if a := body.Approvals[0]; a.ID != "req-1" || a.ToolUseID != "toolu_1" || a.Tool != "Bash" || !strings.Contains(string(a.Input), "make deploy") {
		t.Fatalf("unexpected approval: %+v", a)
	}
	if !runWaiting(t, d, id) {
		t.Fatal("a tool call waiting for the owner does not mark the run waiting")
	}
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"approval":{"id":"req-1","decision":"maybe"}}`, "private"); w.Code != http.StatusBadRequest {
		t.Fatalf("an unknown decision was accepted: %d", w.Code)
	}
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"approval":{"id":"req-1","decision":"always"}}`, "private"); w.Code != 200 {
		t.Fatalf("decide: %d %s", w.Code, w.Body.String())
	}
	for body.Busy || len(body.Approvals) > 0 {
		if time.Now().After(deadline) {
			t.Fatalf("the turn did not end: %+v", body)
		}
		time.Sleep(10 * time.Millisecond)
		_ = json.Unmarshal(conversationRequest(d, "GET", "/desktop/conversation?id="+id, "", "private").Body.Bytes(), &body)
	}
	if runWaiting(t, d, id) {
		t.Fatal("the run still waits once the tool call is answered")
	}
	raw, _ := os.ReadFile(filepath.Join(run.root, "answer.txt"))
	var answer struct {
		Response struct {
			RequestID string `json:"request_id"`
			Response  struct {
				Behavior           string          `json:"behavior"`
				UpdatedInput       json.RawMessage `json:"updatedInput"`
				UpdatedPermissions json.RawMessage `json:"updatedPermissions"`
			} `json:"response"`
		} `json:"response"`
	}
	if json.Unmarshal(raw, &answer) != nil || answer.Response.RequestID != "req-1" || answer.Response.Response.Behavior != "allow" || !strings.Contains(string(answer.Response.Response.UpdatedPermissions), "make deploy") || !strings.Contains(string(answer.Response.Response.UpdatedInput), "make deploy") {
		t.Fatalf("Claude did not get the decision: %s", raw)
	}
	// "Always allow" names no persistent destination: the rule goes to the
	// project's allow rules instead of a file of the worktree (#700).
	if permissions := string(answer.Response.Response.UpdatedPermissions); strings.Contains(permissions, "localSettings") || !strings.Contains(permissions, `"destination":"session"`) {
		t.Fatalf("a persistent destination reached Claude: %s", permissions)
	}
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if sandbox := settings.Project("project").ClaudeSandbox; err != nil || sandbox == nil || !reflect.DeepEqual(sandbox.Allow, []string{"Bash(make deploy)"}) {
		t.Fatalf("the project's allow rules did not gain the rule: %+v %v", sandbox, err)
	}
	lines, _ := run.trace.snapshot()
	if text := strings.Join(lines, "\n"); !strings.Contains(text, `"kind":"approval","text":"always"`) || strings.Contains(text, `"kind":"error"`) || !strings.Contains(text, "Bash(make deploy)") {
		t.Fatalf("unexpected transcript: %s", text)
	}
}

func TestADeniedToolCallCarriesNoInput(t *testing.T) {
	data, _ := json.Marshal(approvalResponse(conversationApproval{ID: "r", Input: json.RawMessage(`{"command":"rm -rf /"}`)}, "deny", nil))
	if text := string(data); !strings.Contains(text, `"behavior":"deny"`) || strings.Contains(text, "rm -rf") {
		t.Fatalf("deny = %s", text)
	}
	data, _ = json.Marshal(approvalResponse(conversationApproval{ID: "r", Suggestions: json.RawMessage(`null`)}, "always", nil))
	if text := string(data); !strings.Contains(text, `"updatedInput":{}`) || strings.Contains(text, "updatedPermissions") {
		t.Fatalf("always without suggestions = %s", text)
	}
}

// A message sent while Claude works joins the running turn on its stdin.
func TestConversationMessageSentWhileClaudeWorksJoinsTheTurn(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake CLI is a POSIX script")
	}
	testhome.Temp(t)
	bin := t.TempDir()
	script := `#!/bin/sh
read -r init
read -r first
printf '%s\n' '{"type":"stream_event","parent_tool_use_id":null,"event":{"type":"message_start"}}'
read -r second
printf '%s' "$second" > second.txt
printf '%s\n' '{"type":"assistant","parent_tool_use_id":null,"message":{"content":[{"type":"text","text":"Both answered"}]}}'
printf '%s\n' '{"type":"result","is_error":false,"result":"Both answered","session_id":"11111111-1111-4111-8111-111111111111"}'
`
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	d, id := conversationFixture(t)
	run := d.queue.runs[id]
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"first"}`, "private"); w.Code != 202 {
		t.Fatalf("send: %d", w.Code)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		d.queue.mu.Lock()
		started := !run.conversation.requestAt.IsZero()
		d.queue.mu.Unlock()
		if started {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Claude never started its request")
		}
		time.Sleep(10 * time.Millisecond)
	}
	w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"and this"}`, "private")
	if w.Code != 202 || !strings.Contains(w.Body.String(), `"joined":true`) {
		t.Fatalf("a message while Claude works was not joined: %d %s", w.Code, w.Body.String())
	}
	waitConversationIdle(t, d, run)
	raw, _ := os.ReadFile(filepath.Join(run.root, "second.txt"))
	if !strings.Contains(string(raw), `"content":"and this"`) {
		t.Fatalf("Claude did not read the joined message: %s", raw)
	}
	lines, _ := run.trace.snapshot()
	if text := strings.Join(lines, "\n"); strings.Count(text, `"kind":"user"`) != 2 || strings.Count(text, `"kind":"assistant"`) != 1 {
		t.Fatalf("unexpected transcript: %s", text)
	}
}

// A message sent once the turn's stdin has closed starts the next turn.
func TestConversationMessageThatMissesTheTurnStartsTheNext(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake CLI is a POSIX script")
	}
	testhome.Temp(t)
	bin := t.TempDir()
	script := `#!/bin/sh
read -r init
read -r message
printf '%s\n' "$message" >> messages.txt
printf '%s\n' '{"type":"result","is_error":false,"result":"ok","session_id":"11111111-1111-4111-8111-111111111111"}'
sleep 1
`
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	d, id := conversationFixture(t)
	run := d.queue.runs[id]
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"first"}`, "private"); w.Code != 202 {
		t.Fatalf("send: %d", w.Code)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		d.queue.mu.Lock()
		closed := run.conversation.input != nil && run.conversation.input.closed
		d.queue.mu.Unlock()
		if closed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the turn's stdin never closed")
		}
		time.Sleep(5 * time.Millisecond)
	}
	w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"late"}`, "private")
	if w.Code != 202 || !strings.Contains(w.Body.String(), `"joined":false`) {
		t.Fatalf("a late message was not kept for the next turn: %d %s", w.Code, w.Body.String())
	}
	time.Sleep(50 * time.Millisecond)
	waitConversationIdle(t, d, run)
	raw, _ := os.ReadFile(filepath.Join(run.root, "messages.txt"))
	if text := string(raw); strings.Count(text, `"type":"user"`) != 2 || !strings.Contains(text, `"content":"late"`) {
		t.Fatalf("the late message did not start a turn: %s", text)
	}
}

func waitConversationIdle(t *testing.T, d *agentDaemon, run *controlledRun) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		d.queue.mu.Lock()
		busy := run.conversation.busy
		d.queue.mu.Unlock()
		if !busy {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the conversation never went idle")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// The model and the permission mode picked in the composer apply from the
// next turn; a value Claude would not take as one word is refused.
func TestConversationTurnTakesTheModelAndModePicked(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake CLI is a POSIX script")
	}
	testhome.Temp(t)
	bin := t.TempDir()
	script := `#!/bin/sh
read -r init
read -r message
printf '%s\n' "$@" > args.txt
printf '%s\n' '{"type":"result","is_error":false,"result":"ok","session_id":"11111111-1111-4111-8111-111111111111"}'
`
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	d, id := conversationFixture(t)
	run := d.queue.runs[id]
	for _, body := range []string{`{"message":"x","mode":"bypassPermissions"}`, `{"message":"x","model":"opus --dangerously-skip-permissions"}`} {
		if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, body, "private"); w.Code != http.StatusBadRequest {
			t.Fatalf("%s was accepted: %d", body, w.Code)
		}
	}
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"plan it","model":"opus","mode":"plan"}`, "private"); w.Code != 202 {
		t.Fatalf("send: %d %s", w.Code, w.Body.String())
	}
	waitConversationIdle(t, d, run)
	args, _ := os.ReadFile(filepath.Join(run.root, "args.txt"))
	if text := string(args); !strings.Contains(text, "--model\nopus") || !strings.Contains(text, "--permission-mode\nplan") {
		t.Fatalf("the picked model and mode did not reach Claude: %s", text)
	}
	w := conversationRequest(d, "GET", "/desktop/conversation?id="+id, "", "private")
	if body := w.Body.String(); !strings.Contains(body, `"model":"opus"`) || !strings.Contains(body, `"mode":"plan"`) {
		t.Fatalf("the conversation does not report its model and mode: %s", body)
	}
}

func TestAQuestionIsAnsweredWithItsInputAndTheAnswers(t *testing.T) {
	ask := conversationApproval{ID: "q", Tool: askUserQuestion, Input: json.RawMessage(`{"questions":[{"question":"Which color?","options":[{"label":"Red"},{"label":"Blue"}]},{"question":"Why?"}]}`)}
	data, _ := json.Marshal(approvalResponse(ask, "answer", map[string]string{"Which color?": "Blue", "Why?": "Calm"}))
	var sent struct {
		Response struct {
			Response struct {
				Behavior     string `json:"behavior"`
				UpdatedInput struct {
					Questions []json.RawMessage `json:"questions"`
					Answers   map[string]string `json:"answers"`
				} `json:"updatedInput"`
			} `json:"response"`
		} `json:"response"`
	}
	if json.Unmarshal(data, &sent) != nil || sent.Response.Response.Behavior != "allow" || len(sent.Response.Response.UpdatedInput.Questions) != 2 || sent.Response.Response.UpdatedInput.Answers["Which color?"] != "Blue" {
		t.Fatalf("answer = %s", data)
	}
	for name, tc := range map[string]struct {
		approval conversationApproval
		decision string
		answers  map[string]string
	}{
		"a question left unanswered": {ask, "answer", map[string]string{"Which color?": "Blue"}},
		"an answer to no question":   {ask, "answer", map[string]string{"Which color?": "Blue", "Why?": "x", "Who?": "y"}},
		"a blank answer":             {ask, "answer", map[string]string{"Which color?": " ", "Why?": "x"}},
		"answers to another tool":    {conversationApproval{Tool: "Bash"}, "answer", map[string]string{"x": "y"}},
		"answers with an allow":      {ask, "allow", map[string]string{"Which color?": "Blue"}},
		"answers past the bound":     {ask, "answer", map[string]string{"Which color?": strings.Repeat("x", conversationAnswersLimit), "Why?": "x"}},
	} {
		if checkAnswers(tc.approval, tc.decision, tc.answers) == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	if err := checkAnswers(ask, "answer", map[string]string{"Which color?": "Blue", "Why?": "Calm"}); err != nil {
		t.Errorf("full answers refused: %v", err)
	}
}

// Claude's question waits for the owner, whose answers reach Claude and stay
// in the trace.
func TestConversationAnswersClaudesQuestion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake CLI is a POSIX script")
	}
	testhome.Temp(t)
	bin := t.TempDir()
	script := `#!/bin/sh
read -r init
read -r message
printf '%s\n' '{"type":"control_request","request_id":"ask-1","request":{"subtype":"can_use_tool","tool_name":"AskUserQuestion","tool_use_id":"toolu_q","input":{"questions":[{"question":"Which color?","header":"Color","options":[{"label":"Red"},{"label":"Blue"}],"multiSelect":false}]}}}'
read -r answer
printf '%s' "$answer" > answer.txt
printf '%s\n' '{"type":"result","is_error":false,"result":"Blue it is","session_id":"11111111-1111-4111-8111-111111111111"}'
`
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	d, id := conversationFixture(t)
	run := d.queue.runs[id]
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"ask me"}`, "private"); w.Code != 202 {
		t.Fatalf("send: %d", w.Code)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		d.queue.mu.Lock()
		waiting := len(run.conversation.approvals) == 1
		d.queue.mu.Unlock()
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the question never waited for the owner")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"approval":{"id":"ask-1","decision":"answer","answers":{"Which color?":"Blue"}}}`, "private"); w.Code != 200 {
		t.Fatalf("answer: %d %s", w.Code, w.Body.String())
	}
	waitConversationIdle(t, d, run)
	raw, _ := os.ReadFile(filepath.Join(run.root, "answer.txt"))
	if !strings.Contains(string(raw), `"answers":{"Which color?":"Blue"}`) || !strings.Contains(string(raw), `"behavior":"allow"`) {
		t.Fatalf("Claude did not get the answers: %s", raw)
	}
	lines, _ := run.trace.snapshot()
	if text := strings.Join(lines, "\n"); !strings.Contains(text, `"kind":"approval","text":"answer","detail":"Which color? → Blue"`) {
		t.Fatalf("the answers are not in the trace: %s", text)
	}
}

// runWaiting reads the run list as Desktop does and says whether the run
// waits for its owner.
func runWaiting(t *testing.T, d *agentDaemon, id string) bool {
	t.Helper()
	var runs []desktopRun
	if err := json.Unmarshal(conversationRequest(d, "GET", "/desktop/runs", "", "private").Body.Bytes(), &runs); err != nil {
		t.Fatal(err)
	}
	for _, run := range runs {
		if run.ID == id {
			return !run.WaitingSince.IsZero()
		}
	}
	t.Fatalf("run %s is not listed", id)
	return false
}

// A skill asking its questions in a conversation declares a wait, as it does
// in a terminal; the owner's next message answers it.
func TestAConversationWaitIsAnsweredByTheNextMessage(t *testing.T) {
	testhome.Temp(t)
	d, id := conversationFixture(t)
	since := time.Now().Add(-time.Minute).UTC()
	payload, _ := json.Marshal(agentprotocol.RunWaiting{RunID: id, WaitingSince: &since})
	d.handleRunWaiting(agentprotocol.Message{Payload: payload})
	if !runWaiting(t, d, id) {
		t.Fatal("a conversation ignored the wait its session declared")
	}
	d.queue.mu.Lock()
	run := d.queue.runs[id]
	run.conversation.busy = true
	d.queue.mu.Unlock()
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"here is my answer"}`, "private"); w.Code != 202 {
		t.Fatalf("send: %d", w.Code)
	}
	if runWaiting(t, d, id) {
		t.Fatal("the owner's message did not answer the wait")
	}
	d.queue.mu.Lock()
	answered := run.answeredAt.Equal(since)
	run.conversation.busy, run.conversation.next = false, nil
	d.queue.mu.Unlock()
	if !answered {
		t.Fatal("the answer is not kept for the server")
	}
}

// A wait the session declared outlives the end of an approval wait.
func TestAnApprovalWaitLeavesTheSessionsWait(t *testing.T) {
	run := &controlledRun{conversation: &claudeConversation{}}
	since := time.Now().Add(-time.Minute).UTC()
	run.desktop.WaitingSince = since
	run.conversation.approvals = []conversationApproval{{ID: "a"}}
	markApprovalWaitLocked(run)
	run.conversation.approvals = nil
	markApprovalWaitLocked(run)
	if !run.desktop.WaitingSince.Equal(since) {
		t.Fatalf("the session's wait was cleared: %v", run.desktop.WaitingSince)
	}
}

func TestTheInitializeResponseListsTheSlashCommands(t *testing.T) {
	line := `{"type":"control_response","response":{"subtype":"success","request_id":"init-1","response":{"commands":[{"name":"clarify-issue","description":"Clarify a ticket","argumentHint":"<KEY>"},{"name":"bad name"},{"name":""},{"name":"long","description":"` + strings.Repeat("x", 400) + `"}]}}}`
	if _, ok := initializeCommands(line, "other"); ok {
		t.Fatal("another request's response was read as the command list")
	}
	commands, ok := initializeCommands(line, "init-1")
	if !ok || len(commands) != 2 || commands[0] != (conversationSlash{Name: "clarify-issue", Description: "Clarify a ticket", ArgumentHint: "<KEY>"}) {
		t.Fatalf("commands = %+v", commands)
	}
	if got := []rune(commands[1].Description); len(got) != conversationSlashDescription+1 {
		t.Errorf("a long description was not cut: %d", len(got))
	}
}

// The commands are listed before the first turn by a probe, and refreshed by
// each turn's own initialize.
func TestConversationListsItsSlashCommands(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake CLI is a POSIX script")
	}
	testhome.Temp(t)
	d, id := conversationFixture(t)
	probed := make(chan []string, 1)
	d.probeCommandsFn = func(cmd *exec.Cmd) []conversationSlash {
		probed <- cmd.Args
		return []conversationSlash{{Name: "from-probe"}}
	}
	conversationRequest(d, "GET", "/desktop/conversation?id="+id, "", "private")
	select {
	case args := <-probed:
		if strings.Contains(strings.Join(args, " "), "--resume") {
			t.Fatalf("the probe resumed the session: %q", args)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no probe listed the commands")
	}
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(conversationRequest(d, "GET", "/desktop/conversation?id="+id, "", "private").Body.String(), `"name":"from-probe"`) {
		if time.Now().After(deadline) {
			t.Fatal("the probed commands are not served")
		}
		time.Sleep(10 * time.Millisecond)
	}
	bin := t.TempDir()
	script := `#!/bin/sh
read -r init
id=$(printf '%s' "$init" | sed 's/.*"request_id":"\([^"]*\)".*/\1/')
printf '{"type":"control_response","response":{"subtype":"success","request_id":"%s","response":{"commands":[{"name":"from-turn"}]}}}\n' "$id"
read -r message
printf '%s\n' '{"type":"result","is_error":false,"result":"ok","session_id":"11111111-1111-4111-8111-111111111111"}'
`
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"hi"}`, "private"); w.Code != 202 {
		t.Fatalf("send: %d", w.Code)
	}
	waitConversationIdle(t, d, d.queue.runs[id])
	if body := conversationRequest(d, "GET", "/desktop/conversation?id="+id, "", "private").Body.String(); !strings.Contains(body, `"name":"from-turn"`) {
		t.Fatalf("the turn's initialize did not refresh the commands: %s", body)
	}
}

func TestTheCommandProbeStopsClaudeOnItsAnswer(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake CLI is a POSIX script")
	}
	bin := t.TempDir()
	script := `#!/bin/sh
read -r init
id=$(printf '%s' "$init" | sed 's/.*"request_id":"\([^"]*\)".*/\1/')
printf '{"type":"control_response","response":{"subtype":"success","request_id":"%s","response":{"commands":[{"name":"clarify-issue"}]}}}\n' "$id"
cat > /dev/null
`
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	d := &agentDaemon{}
	started := time.Now()
	commands := d.probeConversationCommands(claudeConversationCommand(t.TempDir(), "", "", "", "", "", nil, nil))
	if len(commands) != 1 || commands[0].Name != "clarify-issue" {
		t.Fatalf("commands = %+v", commands)
	}
	if time.Since(started) > 5*time.Second {
		t.Fatal("the probe waited past the answer")
	}
}

// A message starting with "!" runs in the shell without Claude, shows as a
// Bash card, and reaches Claude with the next message, as in Claude Code.
func TestConversationRunsABangCommandInTheShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake CLI is a POSIX script")
	}
	testhome.Temp(t)
	bin := t.TempDir()
	script := `#!/bin/sh
read -r init
read -r message
printf '%s' "$message" > message.txt
printf '%s\n' '{"type":"result","is_error":false,"result":"ok","session_id":"11111111-1111-4111-8111-111111111111"}'
`
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SHELL", "/bin/sh")
	d, id := conversationFixture(t)
	run := d.queue.runs[id]
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"!"}`, "private"); w.Code != http.StatusBadRequest {
		t.Fatalf("an empty command was accepted: %d", w.Code)
	}
	for _, line := range []string{`!printf 'bang-%s' "$SECTILE_PROJECT_ID"`, "!exit 3"} {
		body, _ := json.Marshal(map[string]string{"message": line})
		if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, string(body), "private"); w.Code != 202 || !strings.Contains(w.Body.String(), `"shell":true`) {
			t.Fatalf("%s: %d %s", line, w.Code, w.Body.String())
		}
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		lines, _ := run.trace.snapshot()
		if strings.Count(strings.Join(lines, "\n"), `"kind":"tool_result"`) == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the commands did not finish: %v", lines)
		}
		time.Sleep(10 * time.Millisecond)
	}
	d.queue.mu.Lock()
	busy := run.conversation.busy
	d.queue.mu.Unlock()
	if busy {
		t.Fatal("a shell command started Claude")
	}
	lines, _ := run.trace.snapshot()
	text := strings.Join(lines, "\n")
	if !strings.Contains(text, `"kind":"tool","text":"Bash"`) || !strings.Contains(text, `"text":"bang-project"`) || !strings.Contains(text, `Exit code 3.","toolId"`) || !strings.Contains(text, `"error":true`) {
		t.Fatalf("unexpected transcript: %s", text)
	}
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"what did it print?"}`, "private"); w.Code != 202 {
		t.Fatalf("send: %d", w.Code)
	}
	waitConversationIdle(t, d, run)
	raw, _ := os.ReadFile(filepath.Join(run.root, "message.txt"))
	var sent struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal(raw, &sent) != nil || !strings.Contains(sent.Message.Content, `<bash-input>printf 'bang-%s' "$SECTILE_PROJECT_ID"</bash-input>`) || !strings.Contains(sent.Message.Content, "<bash-stdout>bang-project</bash-stdout>") || !strings.HasSuffix(sent.Message.Content, "what did it print?") {
		t.Fatalf("Claude did not get the commands: %s", raw)
	}
	if lines, _ := run.trace.snapshot(); strings.Contains(strings.Join(lines, "\n"), `"kind":"user","text":"<bash-input>`) {
		t.Fatal("the shell context leaked into the owner's message in the transcript")
	}
}

// A local command answers without Claude: its output, laid out for a
// terminal, is kept as such rather than as a reply.
func TestALocalCommandsOutputIsKeptAsLaidOut(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake CLI is a POSIX script")
	}
	testhome.Temp(t)
	bin := t.TempDir()
	script := `#!/bin/sh
read -r init
read -r message
printf '%s\n' '{"type":"result","is_error":false,"result":"Current session: 5% used\n  65% of your usage was at >150k context","session_id":"11111111-1111-4111-8111-111111111111"}'
`
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	d, id := conversationFixture(t)
	run := d.queue.runs[id]
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"/usage"}`, "private"); w.Code != 202 {
		t.Fatalf("send: %d", w.Code)
	}
	waitConversationIdle(t, d, run)
	lines, _ := run.trace.snapshot()
	if text := strings.Join(lines, "\n"); !strings.Contains(text, `"kind":"command_output","text":"Current session: 5% used\n  65% of your usage`) || strings.Contains(text, `"kind":"assistant"`) {
		t.Fatalf("unexpected transcript: %s", text)
	}
}

// /mcp shows the CLI's health check of the MCP servers, without Claude.
func TestConversationMCPRunsTheCLIHealthCheck(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake CLI is a POSIX script")
	}
	testhome.Temp(t)
	bin := t.TempDir()
	script := `#!/bin/sh
printf '%s\n' "$*" > "$CLAUDE_ARGS"
printf 'Checking MCP server health…\n\nsectile: http://127.0.0.1/mcp - ✔ Connected\nwiz: https://wiz - ! Needs authentication\n'
`
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	args := filepath.Join(t.TempDir(), "args")
	t.Setenv("CLAUDE_ARGS", args)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SHELL", "/bin/sh")
	d, id := conversationFixture(t)
	run := d.queue.runs[id]
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":" /mcp "}`, "private"); w.Code != 202 || !strings.Contains(w.Body.String(), `"local":true`) {
		t.Fatalf("/mcp: %d %s", w.Code, w.Body.String())
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		lines, _ := run.trace.snapshot()
		text := strings.Join(lines, "\n")
		if strings.Contains(text, `"kind":"command_output"`) {
			if !strings.Contains(text, `"text":"sectile: http://127.0.0.1/mcp - ✔ Connected\nwiz: https://wiz - ! Needs authentication"`) {
				t.Fatalf("unexpected output: %s", text)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("/mcp never answered: %s", text)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if raw, _ := os.ReadFile(args); strings.TrimSpace(string(raw)) != "mcp list" {
		t.Fatalf("claude ran with %q", raw)
	}
	d.queue.mu.Lock()
	busy := run.conversation.busy
	d.queue.mu.Unlock()
	if busy {
		t.Fatal("/mcp started a turn")
	}
}

func TestSectileMCPStatusIsReadFromClaudeCode(t *testing.T) {
	got := parseMCPGet("sectile:\n  Scope: User config\n  Status: ✔ Connected\n  Type: http\n  URL: https://sectile.example/mcp\n")
	if got.Status != "connected" || got.Detail != "✔ Connected · https://sectile.example/mcp" {
		t.Errorf("connected = %+v", got)
	}
	for output, want := range map[string]string{
		"  Status: ! Needs authentication\n":                   "needs-auth",
		"  Status: ✗ Failed to connect\n":                      "failed",
		`No MCP server named "sectile". Configured servers: x`: "missing",
		"garbage": "unknown",
	} {
		if got := parseMCPGet(output).Status; got != want {
			t.Errorf("%q = %s, want %s", output, got, want)
		}
	}
	type server = struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	if got := initMCPServer([]server{{"other", "connected"}, {"sectile", "failed"}}); got.Status != "failed" {
		t.Errorf("init failed = %+v", got)
	}
	if got := initMCPServer([]server{{"other", "connected"}}); got.Status != "missing" {
		t.Errorf("init without sectile = %+v", got)
	}
}

// The status is checked before the first turn, checked again on request, and
// refreshed by each turn's init frame.
func TestConversationReportsSectileMCPConnectivity(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake CLI is a POSIX script")
	}
	testhome.Temp(t)
	d, id := conversationFixture(t)
	checks := make(chan struct{}, 4)
	d.checkMCPFn = func(string, map[string]string) conversationMCP {
		checks <- struct{}{}
		return conversationMCP{Status: "needs-auth"}
	}
	wait := func(want string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for !strings.Contains(conversationRequest(d, "GET", "/desktop/conversation?id="+id, "", "private").Body.String(), `"sectileMcp":{"status":"`+want+`"`) {
			if time.Now().After(deadline) {
				t.Fatalf("the status never became %s", want)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	wait("needs-auth")
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"checkMcp":true}`, "private"); w.Code != 202 {
		t.Fatalf("check: %d", w.Code)
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(checks) < 2 {
		if time.Now().After(deadline) {
			t.Fatal("checking again ran no check")
		}
		time.Sleep(10 * time.Millisecond)
	}
	bin := t.TempDir()
	script := `#!/bin/sh
read -r init
read -r message
printf '%s\n' '{"type":"system","subtype":"init","model":"m","session_id":"11111111-1111-4111-8111-111111111111","mcp_servers":[{"name":"sectile","status":"connected"}]}'
printf '%s\n' '{"type":"result","is_error":false,"result":"ok","session_id":"11111111-1111-4111-8111-111111111111"}'
`
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"hi"}`, "private"); w.Code != 202 {
		t.Fatalf("send: %d", w.Code)
	}
	waitConversationIdle(t, d, d.queue.runs[id])
	wait("connected")
}

// A poll that already shows the latest version is not sent the history again.
func TestAConversationPollSkipsAnUnchangedHistory(t *testing.T) {
	testhome.Temp(t)
	d, id := conversationFixture(t)
	var full struct {
		Version uint64            `json:"version"`
		Events  []json.RawMessage `json:"events"`
	}
	if err := json.Unmarshal(conversationRequest(d, "GET", "/desktop/conversation?id="+id, "", "private").Body.Bytes(), &full); err != nil || len(full.Events) == 0 {
		t.Fatalf("a first poll got no history: %v %+v", err, full)
	}
	since := strconv.FormatUint(full.Version, 10)
	body := conversationRequest(d, "GET", "/desktop/conversation?id="+id+"&since="+since, "", "private").Body.String()
	if strings.Contains(body, `"events"`) || !strings.Contains(body, `"version":`+since) || !strings.Contains(body, `"busy":false`) {
		t.Fatalf("an unchanged history was sent again, or the state was not: %s", body)
	}
	d.queue.mu.Lock()
	conversationWrite(d.queue.runs[id].trace, "notice", "something new", "")
	d.queue.mu.Unlock()
	if body := conversationRequest(d, "GET", "/desktop/conversation?id="+id+"&since="+since, "", "private").Body.String(); !strings.Contains(body, "something new") {
		t.Fatalf("a changed history was not sent: %s", body)
	}
}
