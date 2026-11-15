package agent

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/testhome"
)

func formerLines(events ...conversationEvent) []string {
	lines := make([]string, 0, len(events))
	for _, event := range events {
		data, _ := json.Marshal(event)
		lines = append(lines, string(data))
	}
	return lines
}

// Only what the owner and the assistant said goes into the former context,
// in order; tool activity is left out and said to be.
func TestFormerContextKeepsUserAndAssistantMessagesInOrder(t *testing.T) {
	lines := formerLines(
		conversationEvent{Kind: "notice", Text: "Claude Code conversation"},
		conversationEvent{Kind: "user", Text: "Explain the store"},
		conversationEvent{Kind: "tool", Text: "Read", Tool: "Read"},
		conversationEvent{Kind: "tool_result", Text: "file body"},
		conversationEvent{Kind: "assistant", Text: "The store keeps runs."},
		conversationEvent{Kind: "user", Text: "  "},
	)
	lines = append(lines, "not json")
	got, err := buildFormerContext("source", "Store questions", lines)
	if err != nil {
		t.Fatal(err)
	}
	if got.Messages != 2 || got.Omitted != 0 || !got.ToolsOmitted {
		t.Fatalf("context = %+v", got)
	}
	user, assistant := strings.Index(got.Text, "User: Explain the store"), strings.Index(got.Text, "Assistant: The store keeps runs.")
	if user < 0 || assistant < user {
		t.Fatalf("messages missing or out of order:\n%s", got.Text)
	}
	for _, unwanted := range []string{"file body", "Claude Code conversation"} {
		if strings.Contains(got.Text, unwanted) {
			t.Fatalf("former context carries %q:\n%s", unwanted, got.Text)
		}
	}
	for _, wanted := range []string{`<former-context source="Store questions">`, "background only", "Tool calls, tool results and notices", "</former-context>"} {
		if !strings.Contains(got.Text, wanted) {
			t.Fatalf("former context lacks %q:\n%s", wanted, got.Text)
		}
	}
}

// Past the budget, the most recent messages are kept whole and the earlier
// ones are counted, not cut.
func TestFormerContextKeepsTheMostRecentMessagesWithinTheBudget(t *testing.T) {
	block := strings.Repeat("a", formerContextBudget/3)
	lines := formerLines(
		conversationEvent{Kind: "user", Text: "first " + block},
		conversationEvent{Kind: "assistant", Text: "second " + block},
		conversationEvent{Kind: "user", Text: "third " + block},
		conversationEvent{Kind: "assistant", Text: "fourth"},
	)
	got, err := buildFormerContext("source", "", lines)
	if err != nil {
		t.Fatal(err)
	}
	if got.Messages != 3 || got.Omitted != 1 || got.ToolsOmitted {
		t.Fatalf("context = %+v", got)
	}
	if strings.Contains(got.Text, "first ") || !strings.Contains(got.Text, "second ") || !strings.Contains(got.Text, "fourth") {
		t.Fatalf("wrong messages kept")
	}
	if !strings.Contains(got.Text, "1 earlier messages were left out") || strings.Contains(got.Text, "Tool calls") {
		t.Fatalf("notices wrong")
	}
	if !strings.Contains(got.Text, `source="Archived conversation"`) {
		t.Fatalf("an untitled source has no fallback title")
	}
}

// A last message longer than the whole budget is still given, cut and marked.
func TestFormerContextCutsASingleOversizedMessage(t *testing.T) {
	lines := formerLines(
		conversationEvent{Kind: "user", Text: "earlier"},
		conversationEvent{Kind: "assistant", Text: strings.Repeat("é", formerContextBudget+10)},
	)
	got, err := buildFormerContext("source", "Long", lines)
	if err != nil {
		t.Fatal(err)
	}
	if got.Messages != 1 || got.Omitted != 1 || got.Characters != formerContextBudget {
		t.Fatalf("context = %+v", got)
	}
	if !strings.Contains(got.Text, formerContextCutMarker) {
		t.Fatal("cut message is not marked")
	}
}

func TestFormerContextRefusesASourceWithoutMessages(t *testing.T) {
	for name, lines := range map[string][]string{
		"empty":     nil,
		"tool only": formerLines(conversationEvent{Kind: "notice", Text: "hello"}, conversationEvent{Kind: "tool", Text: "Bash"}),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := buildFormerContext("source", "", lines); !errors.Is(err, errFormerContextEmpty) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func formerContextDaemon(t *testing.T) *agentDaemon {
	t.Helper()
	testhome.Temp(t)
	d := &agentDaemon{loopback: loopbackServer{desktopToken: "private"}}
	d.probeCommandsFn = func(*exec.Cmd) []conversationSlash { return nil }
	d.checkMCPFn = func(string, map[string]string) conversationMCP { return conversationMCP{Status: "unknown"} }
	t.Cleanup(d.stopConversations)
	return d
}

// addFormerSource admits a finished conversation of project p holding events.
func addFormerSource(t *testing.T, d *agentDaemon, id, projectID string, events ...conversationEvent) *controlledRun {
	t.Helper()
	run, err := d.enqueueRun("", agentconfig.Dispatch{RunID: id}, projectID, t.TempDir(), 1, false)
	if err != nil {
		t.Fatal(err)
	}
	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()
	startConversationLocked(run, "", "", "Old.")
	run.desktop.Title = "Old chat"
	run.restored = true
	for _, event := range events {
		conversationWriteEvent(run.trace, event)
	}
	return run
}

func sourceState(d *agentDaemon, run *controlledRun) (desktopRun, []string) {
	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()
	lines, _ := run.trace.snapshot()
	return run.desktop, lines
}

func TestFormerContextReadsOnlyAConversationOfTheProject(t *testing.T) {
	d := formerContextDaemon(t)
	source := addFormerSource(t, d, "source", "p", conversationEvent{Kind: "user", Text: "hello"})
	terminal, err := d.enqueueRun("", agentconfig.Dispatch{RunID: "terminal"}, "p", t.TempDir(), 1, false)
	if err != nil {
		t.Fatal(err)
	}
	_ = terminal
	beforeRun, beforeLines := sourceState(d, source)
	for name, test := range map[string]struct {
		project, id string
		want        error
	}{
		"unknown":          {"p", "missing", errFormerContextUnavailable},
		"unsafe id":        {"p", "../source", errFormerContextUnavailable},
		"not conversation": {"p", "terminal", errFormerContextUnavailable},
		"other project":    {"other", "source", errFormerContextProject},
	} {
		t.Run(name, func(t *testing.T) {
			d.queue.mu.Lock()
			_, err := d.formerContextLocked(test.project, test.id, "")
			d.queue.mu.Unlock()
			if !errors.Is(err, test.want) {
				t.Fatalf("err = %v, want %v", err, test.want)
			}
		})
	}
	d.queue.mu.Lock()
	got, err := d.formerContextLocked("p", "source", "")
	d.queue.mu.Unlock()
	if err != nil || got.SourceTitle != "Old chat" || got.Messages != 1 {
		t.Fatalf("context = %+v, %v", got, err)
	}
	afterRun, afterLines := sourceState(d, source)
	if !reflect.DeepEqual(beforeRun, afterRun) || !reflect.DeepEqual(beforeLines, afterLines) {
		t.Fatal("reading the former context changed its source")
	}
}

func TestFormerContextPreviewAnswersWithoutTheText(t *testing.T) {
	d := formerContextDaemon(t)
	addFormerSource(t, d, "source", "p", conversationEvent{Kind: "user", Text: "secret plan"}, conversationEvent{Kind: "tool", Text: "Bash"})
	addFormerSource(t, d, "empty", "p")
	for target, want := range map[string]int{
		"/desktop/conversation/former-context?projectId=p&id=source":     200,
		"/desktop/conversation/former-context?projectId=p&id=missing":    404,
		"/desktop/conversation/former-context?projectId=other&id=source": 400,
		"/desktop/conversation/former-context?projectId=p&id=empty":      422,
	} {
		rec := disconnectRequest(d, "GET", target, "")
		if rec.Code != want {
			t.Fatalf("%s = %d %s, want %d", target, rec.Code, rec.Body.String(), want)
		}
		if want == 200 {
			var body formerContext
			if json.Unmarshal(rec.Body.Bytes(), &body) != nil || body.Messages != 1 || !body.ToolsOmitted || body.SourceTitle != "Old chat" {
				t.Fatalf("preview = %s", rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "secret plan") {
				t.Fatal("the preview carries the history itself")
			}
		}
	}
	if rec := disconnectRequest(d, "POST", "/desktop/conversation/former-context?projectId=p&id=source", ""); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST = %d", rec.Code)
	}
}

// fakeRecordingClaude is a Claude that writes the message of each turn to
// message.txt in its working folder.
func fakeRecordingClaude(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake CLI is a POSIX script")
	}
	bin := t.TempDir()
	script := `#!/bin/sh
read -r init
read -r message
printf '%s\n' "$message" >> message.txt
printf '%s\n' '{"type":"result","is_error":false,"result":"ok","session_id":"11111111-1111-4111-8111-111111111111"}'
`
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func recordedMessages(t *testing.T, directory string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(directory, "message.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var messages []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var frame struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			t.Fatalf("turn message %q: %v", line, err)
		}
		messages = append(messages, frame.Message.Content)
	}
	return messages
}

func userEvents(lines []string) []string {
	var texts []string
	for _, line := range lines {
		var event conversationEvent
		if json.Unmarshal([]byte(line), &event) == nil && event.Kind == "user" {
			texts = append(texts, event.Text)
		}
	}
	return texts
}

// A conversation started over from an archived one sends its former context
// with the first message only, shows where it came from, and leaves the
// source as it was.
func TestAConversationStartedOverSendsItsFormerContextOnce(t *testing.T) {
	d, config := disconnectFixture(t)
	fakeRecordingClaude(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/api/v1/agent/config" {
			_ = json.NewEncoder(w).Encode(config)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	d.link.serverURL = server.URL
	d.probeCommandsFn = func(*exec.Cmd) []conversationSlash { return nil }
	d.checkMCPFn = func(string, map[string]string) conversationMCP { return conversationMCP{Status: "unknown"} }
	t.Cleanup(d.stopConversations)
	source := addFormerSource(t, d, "source", "p", conversationEvent{Kind: "user", Text: "Design the cache"}, conversationEvent{Kind: "assistant", Text: "Use an LRU."})
	beforeRun, beforeLines := sourceState(d, source)

	if rec := disconnectRequest(d, "POST", "/desktop/consoles", `{"projectId":"p","provider":"claude","contextRunId":"source"}`); rec.Code != 400 {
		t.Fatalf("former context outside the conversation view = %d %s", rec.Code, rec.Body.String())
	}
	d.queue.mu.Lock()
	runsBefore := len(d.queue.runs)
	d.queue.mu.Unlock()
	if rec := disconnectRequest(d, "POST", "/desktop/consoles", `{"projectId":"p","provider":"claude","view":"conversation","contextRunId":"missing"}`); rec.Code != 404 {
		t.Fatalf("missing source = %d %s", rec.Code, rec.Body.String())
	}
	d.queue.mu.Lock()
	runsAfter := len(d.queue.runs)
	d.queue.mu.Unlock()
	if runsAfter != runsBefore {
		t.Fatal("a refused start over created a run")
	}

	rec := disconnectRequest(d, "POST", "/desktop/consoles", `{"projectId":"p","provider":"claude","view":"conversation","contextRunId":"source"}`)
	var entry desktopRun
	if rec.Code != 202 || json.Unmarshal(rec.Body.Bytes(), &entry) != nil {
		t.Fatalf("start over = %d %s", rec.Code, rec.Body.String())
	}
	if entry.ID == "source" || entry.ContextRunID != "source" || entry.ContextTitle != "Old chat" || entry.ContextOmitted != 0 {
		t.Fatalf("new run = %+v", entry)
	}
	d.queue.mu.Lock()
	run := d.queue.runs[entry.ID]
	d.queue.mu.Unlock()
	for _, message := range []string{"Now add metrics", "And tests"} {
		if w := conversationRequest(d, "POST", "/desktop/conversation?id="+entry.ID, `{"message":"`+message+`"}`, "private"); w.Code != http.StatusAccepted {
			t.Fatalf("send: %d %s", w.Code, w.Body.String())
		}
		waitConversationIdle(t, d, run)
	}
	sent := recordedMessages(t, run.desktop.Directory)
	if len(sent) != 2 {
		t.Fatalf("turns = %q", sent)
	}
	if !strings.Contains(sent[0], "former-context") || !strings.Contains(sent[0], "Use an LRU.") || strings.Index(sent[0], "Now add metrics") < strings.Index(sent[0], "former-context") {
		t.Fatalf("first turn = %s", sent[0])
	}
	if strings.Contains(sent[1], "former-context") {
		t.Fatalf("second turn repeats the former context: %s", sent[1])
	}
	_, lines := sourceState(d, run)
	if got := userEvents(lines); !reflect.DeepEqual(got, []string{"Now add metrics", "And tests"}) {
		t.Fatalf("journaled user messages = %q", got)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "Started from Old chat") {
		t.Fatal("the new conversation does not say where it started from")
	}
	afterRun, afterLines := sourceState(d, source)
	if !reflect.DeepEqual(beforeRun, afterRun) || !reflect.DeepEqual(beforeLines, afterLines) {
		t.Fatal("starting over changed the source")
	}
}

// A skill started over keeps its command first: the former context follows
// it in the provider's message, and stays out of the journal.
func TestASkillStartedOverKeepsItsCommandFirst(t *testing.T) {
	d := formerContextDaemon(t)
	fakeRecordingClaude(t)
	directory := t.TempDir()
	run, err := d.enqueueRun("task", agentconfig.Dispatch{RunID: "skill-run"}, "p", directory, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	former := formerContext{SourceID: "source", SourceTitle: "Old chat", Messages: 3, Omitted: 2, Text: "<former-context>past</former-context>"}
	d.startLaunchConversationWithContext(run, desktopRun{ID: "skill-run", TaskID: "task", ProjectID: "p", Skill: "clarify", Directory: directory}, "", "", "It runs the clarify skill.", "/clarify T-1", &former, nil, nil)
	waitConversationIdle(t, d, run)
	sent := recordedMessages(t, directory)
	if len(sent) != 1 || sent[0] != "/clarify T-1\n\n<former-context>past</former-context>" {
		t.Fatalf("first turn = %q", sent)
	}
	entry, lines := sourceState(d, run)
	if got := userEvents(lines); !reflect.DeepEqual(got, []string{"/clarify T-1"}) {
		t.Fatalf("journaled user messages = %q", got)
	}
	if entry.ContextRunID != "source" || entry.ContextTitle != "Old chat" || entry.ContextOmitted != 2 {
		t.Fatalf("run = %+v", entry)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "2 earlier messages left out") {
		t.Fatal("the omitted count is not shown")
	}
}

// Only an interactive launch in the conversation view can carry former
// context; the refusal comes before the server is asked anything.
func TestATaskLaunchWithFormerContextNeedsAnInteractiveConversation(t *testing.T) {
	d, config := disconnectFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/agent/config" {
			_ = json.NewEncoder(w).Encode(config)
			return
		}
		t.Errorf("unexpected server call %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}))
	defer server.Close()
	d.link.serverURL = server.URL
	for name, body := range map[string]string{
		"terminal":   `{"taskId":"t","skillId":"clarify","view":"terminal","contextRunId":"source"}`,
		"no view":    `{"taskId":"t","skillId":"clarify","contextRunId":"source"}`,
		"autonomous": `{"taskId":"t","skillId":"clarify","mode":"autonomous","view":"conversation","contextRunId":"source"}`,
	} {
		t.Run(name, func(t *testing.T) {
			rec := disconnectRequest(d, "POST", "/desktop/tasks?projectId=p", body)
			if rec.Code != 400 || !strings.Contains(rec.Body.String(), "conversation view") {
				t.Fatalf("launch = %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}
