package agent

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"tasks/internal/agentconfig"
	"tasks/internal/models"
	"tasks/internal/runner"
	"tasks/internal/terminal"
)

// Asking the engine for its reasoning stream.

// The streaming options reach the one command line Sectile builds for an
// autonomous Claude run, and nothing else: an engine whose stream format is not
// attested would be handed flags it does not have.
func TestOnlyTheAttestedEngineIsAskedForItsReasoning(t *testing.T) {
	cases := map[string]string{
		"claude": "claude -p --permission-mode bypassPermissions --output-format stream-json --verbose 'do it'",
		"codex":  "codex exec 'do it'",
		"vibe":   "vibe -p --auto-approve 'do it'",
	}
	for provider, want := range cases {
		got, err := modeCommandLine(provider, "", "", "do it", models.SkillModeAutonomous)
		if err != nil {
			t.Fatalf("%s: %v", provider, err)
		}
		if got != want {
			t.Errorf("%s: got %q, want %q", provider, got, want)
		}
	}
}

// A human is watching an interactive session and reading what it prints. The
// stream is for the runs nobody is watching.
func TestAnInteractiveLaunchIsNeverAskedForItsReasoning(t *testing.T) {
	for _, provider := range []string{"claude", "codex", "vibe", "agy", "gemini", "cursor"} {
		got, err := modeCommandLine(provider, "", "", "do it", models.SkillModeInteractive)
		if err != nil {
			t.Fatalf("%s: %v", provider, err)
		}
		if strings.Contains(got, "stream-json") {
			t.Errorf("%s: an interactive line asked for the stream: %q", provider, got)
		}
	}
}

// A template is its author's command line. Sectile adds nothing to it, so a
// project that configured one keeps exactly the launch it had.
func TestAConfiguredTemplateIsExpandedWithNothingAdded(t *testing.T) {
	line, err := modeCommandLine("claude", `claude {mode:-p|-i} '{prompt}'`, "", "do it", models.SkillModeAutonomous)
	if err != nil {
		t.Fatal(err)
	}
	if line != "claude -p 'do it'" {
		t.Fatalf("template was not left alone: %q", line)
	}
	if commandReadsReasoning(line) {
		t.Fatal("a template that does not ask for the stream must not be read as one")
	}

	config := agentconfig.Config{AIProvider: "claude", AICommandTemplateAutonomous: `claude -p '{prompt}'`}
	dedicated, err := launchCommandLine(config, "", "do it", models.SkillModeAutonomous)
	if err != nil {
		t.Fatal(err)
	}
	if dedicated != "claude -p 'do it'" {
		t.Fatalf("dedicated autonomous command was not left alone: %q", dedicated)
	}
}

// The command line is what says whether this run's output is a stream, because
// it is the line that will produce that output.
func TestACommandIsReadAsAStreamWhenItAsksForOne(t *testing.T) {
	if !commandReadsReasoning("claude -p --output-format stream-json --verbose 'x'") {
		t.Error("a line asking for the stream was not recognised")
	}
	if commandReadsReasoning("claude -p 'x'") {
		t.Error("a plain line was read as a stream")
	}
	// A template asking for the stream itself produces one, and is read as one.
	if !commandReadsReasoning("claude --output-format stream-json --verbose -p 'x'") {
		t.Error("a template asking for the stream was not recognised")
	}
}

// Holding the trace.

func TestAWatcherSeesWhatTheRunAlreadyShowedAndWhatComesNext(t *testing.T) {
	trace := newRunTrace()
	trace.publish(runner.ReasoningEvent{Kind: runner.ReasoningText, Text: "Reading the file."})
	trace.publish(runner.ReasoningEvent{Kind: runner.ReasoningTool, Text: "Bash", Detail: "go test ./..."})

	replay, lines := trace.attach()
	defer trace.detach(lines)
	if len(replay) != 2 {
		t.Fatalf("expected the two lines already shown, got %q", replay)
	}
	if !strings.Contains(replay[0], "Reading the file.") || !strings.HasSuffix(replay[0], "\r\n") {
		t.Errorf("prose is not rendered as a terminal line: %q", replay[0])
	}
	if !strings.Contains(replay[1], "▸ Bash · go test ./...") {
		t.Errorf("a tool call must name the tool and what it did: %q", replay[1])
	}

	trace.publish(runner.ReasoningEvent{Kind: runner.ReasoningText, Text: "Done."})
	select {
	case line := <-lines:
		if !strings.Contains(line, "Done.") {
			t.Errorf("the next line was not delivered: %q", line)
		}
	case <-time.After(time.Second):
		t.Fatal("a line published after attaching never arrived")
	}
}

// A run printing a large file must not grow the daemon without limit.
func TestTheTraceKeepsTheMostRecentLinesAndDropsTheOldest(t *testing.T) {
	trace := newRunTrace()
	for i := 0; i < traceRetained+50; i++ {
		trace.write("line\r\n")
	}
	trace.write("last\r\n")
	replay, lines := trace.attach()
	defer trace.detach(lines)
	if len(replay) != traceRetained {
		t.Fatalf("retained %d lines, expected %d", len(replay), traceRetained)
	}
	if replay[len(replay)-1] != "last\r\n" {
		t.Errorf("the most recent line was not kept: %q", replay[len(replay)-1])
	}
}

// The run is the work and the trace is a courtesy: a desktop that stopped
// reading is disconnected rather than allowed to hold up the run it watches.
func TestAWatcherThatStopsReadingIsDroppedAndNeverBlocksTheRun(t *testing.T) {
	trace := newRunTrace()
	_, lines := trace.attach()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < traceBacklog+20; i++ {
			trace.write("line\r\n")
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("publishing blocked on a watcher that was not reading")
	}

	// Draining what it was sent, the watcher then finds its stream closed.
	for range lines {
	}

	trace.mu.Lock()
	watchers := len(trace.clients)
	trace.mu.Unlock()
	if watchers != 0 {
		t.Fatalf("a dropped watcher is still registered: %d", watchers)
	}
}

func TestTheTraceEndsWithTheRunAndStaysReadable(t *testing.T) {
	trace := newRunTrace()
	trace.write("working\r\n")
	_, lines := trace.attach()

	trace.close()
	if _, open := <-lines; open {
		t.Fatal("a watcher was not told the run ended")
	}

	replay, closed := trace.attach()
	if len(replay) != 1 {
		t.Fatalf("what the run showed must stay readable, got %q", replay)
	}
	if _, open := <-closed; open {
		t.Fatal("attaching to a finished run must not leave a stream open")
	}
	// Nothing published after the end reaches anyone.
	trace.write("late\r\n")
	if again, _ := trace.attach(); len(again) != 1 {
		t.Fatalf("a line published after the end was retained: %q", again)
	}
}

// A run that was not traced has no trace, and publishing to it is a no-op
// rather than a panic on the path that finishes every headless run.
func TestARunWithoutATraceSwallowsEverything(t *testing.T) {
	var absent *runTrace
	absent.publish(runner.ReasoningEvent{Kind: runner.ReasoningText, Text: "x"})
	absent.write("x\r\n")
	absent.close()
}

func TestAnEventWithNothingToShowProducesNoLine(t *testing.T) {
	for _, event := range []runner.ReasoningEvent{
		{Kind: runner.ReasoningText, Text: "   "},
		{Kind: runner.ReasoningThinking, Text: ""},
		{Kind: "a_kind_nobody_planned_for", Text: "x"},
	} {
		if line := renderTraceEvent(event); line != "" {
			t.Errorf("%+v rendered %q, expected nothing", event, line)
		}
	}
}

// Prose arrives with plain newlines, and a terminal needs the carriage return
// or the second line starts where the first one ended.
func TestProseSpanningSeveralLinesIsRenderedForATerminal(t *testing.T) {
	line := renderTraceEvent(runner.ReasoningEvent{Kind: runner.ReasoningText, Text: "First.\nSecond."})
	if line != "First.\r\nSecond.\r\n" {
		t.Fatalf("got %q", line)
	}
}

// Reading a traced run's output.

func TestATracedRunShowsItsReasoningAndRecordsOnlyItsAnswer(t *testing.T) {
	trace := newRunTrace()
	var recorded strings.Builder
	stream := strings.Join([]string{
		`{"type":"system","subtype":"init","cwd":"/repo"}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Reading the file."}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"/repo/main.go"}}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","content":"package main"}]}}`,
		`{"type":"result","subtype":"success","is_error":false,"result":"The report."}`,
	}, "\n") + "\n"

	readTracedOutput(trace, strings.NewReader(stream), func(text string) { recorded.WriteString(text) })

	replay, lines := trace.attach()
	defer trace.detach(lines)
	if len(replay) != 2 {
		t.Fatalf("expected the prose and the tool call, got %q", replay)
	}
	if recorded.String() != "The report.\n" {
		t.Fatalf("the activity must receive the answer and nothing else, got %q", recorded.String())
	}
}

// Standard error shares the pipe, and that is where a failed run explains
// itself. Those lines keep reaching the task.
func TestADiagnosticBesideTheStreamStillReachesTheActivity(t *testing.T) {
	trace := newRunTrace()
	var recorded strings.Builder
	stream := "bash: claude: command not found\n" +
		`{"type":"assistant","message":{"content":[{"type":"text","text":"working"}]}}` + "\n" +
		"npm warn deprecated something@1.0.0\n"

	readTracedOutput(trace, strings.NewReader(stream), func(text string) { recorded.WriteString(text) })

	got := recorded.String()
	if !strings.Contains(got, "command not found") || !strings.Contains(got, "npm warn") {
		t.Fatalf("a diagnostic was swallowed: %q", got)
	}
	if strings.Contains(got, `"type"`) {
		t.Fatalf("a protocol line reached the activity: %q", got)
	}
}

// One object of the stream can carry a whole tool result, which is why the
// reading is not a bufio.Scanner: its 64 KB ceiling would end the trace for the
// size of what the engine said.
func TestALineLongerThanAScannerCeilingIsStillRead(t *testing.T) {
	trace := newRunTrace()
	var recorded strings.Builder
	huge := strings.Repeat("x", 200*1024)
	stream := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"echo ` + huge + `"}}]}}` + "\n" +
		`{"type":"result","result":"done"}` + "\n"

	readTracedOutput(trace, strings.NewReader(stream), func(text string) { recorded.WriteString(text) })

	replay, lines := trace.attach()
	defer trace.detach(lines)
	if len(replay) != 1 || !strings.Contains(replay[0], "▸ Bash") {
		t.Fatalf("the long line was not read: %q", replay)
	}
	if recorded.String() != "done\n" {
		t.Fatalf("the line after it was not read: %q", recorded.String())
	}
}

// The pipe delivers bytes, not lines: an object arriving in two reads is one
// object, and reading it in halves would lose it.
func TestALineSplitAcrossTwoReadsIsAssembledBeforeItIsRead(t *testing.T) {
	trace := newRunTrace()
	var recorded strings.Builder
	first := `{"type":"assistant","message":{"content":[{"type":"text",`
	second := `"text":"Split but whole."}]}}` + "\n"

	readTracedOutput(trace, io.MultiReader(strings.NewReader(first), slowReader{strings.NewReader(second)}), func(text string) { recorded.WriteString(text) })

	replay, lines := trace.attach()
	defer trace.detach(lines)
	if len(replay) != 1 || !strings.Contains(replay[0], "Split but whole.") {
		t.Fatalf("the split line was not assembled: %q", replay)
	}
	if recorded.String() != "" {
		t.Fatalf("nothing was due to the activity, got %q", recorded.String())
	}
}

// slowReader hands out one byte at a time, which is what a pipe does when the
// writer flushes mid-object.
type slowReader struct{ from io.Reader }

func (s slowReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return s.from.Read(p[:1])
}

// Serving the trace to the desktop.

func TestTheDesktopAttachesToATracedRunOnTheConsoleRoute(t *testing.T) {
	d := &agentDaemon{terminal: terminalChoice{manager: terminal.NewManager()}, loopback: loopbackServer{desktopToken: "private"}}
	trace := newRunTrace()
	trace.write("already shown\r\n")
	d.queue.runs = map[string]*controlledRun{
		"traced": {desktop: desktopRun{ID: "traced", Status: "running", Headless: true, Trace: true}, trace: trace, exited: make(chan struct{})},
		"blind":  {desktop: desktopRun{ID: "blind", Status: "running", Headless: true}, exited: make(chan struct{})},
	}
	server := httptest.NewServer(http.HandlerFunc(d.desktopHandler))
	defer server.Close()
	header := http.Header{"Authorization": []string{"Bearer private"}}

	connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/desktop/terminal?id=traced", header)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()

	read := func() string {
		t.Helper()
		_ = connection.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, chunk, err := connection.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		return string(chunk)
	}
	if got := read(); got != "already shown\r\n" {
		t.Fatalf("the watcher did not receive the replay: %q", got)
	}

	// Nobody is answering this run: what the pane sends is discarded, and the
	// trace keeps streaming.
	if err := connection.WriteJSON(map[string]string{"type": "input", "data": "hello\n"}); err != nil {
		t.Fatal(err)
	}
	trace.publish(runner.ReasoningEvent{Kind: runner.ReasoningText, Text: "still working"})
	if got := read(); !strings.Contains(got, "still working") {
		t.Fatalf("the trace stopped after an input frame: %q", got)
	}

	// A headless run with nothing to watch is still refused, which is what an
	// older agent's runs look like to a desktop.
	request := httptest.NewRequest("GET", "/desktop/terminal?id=blind", nil)
	request.Header.Set("Authorization", "Bearer private")
	response := httptest.NewRecorder()
	d.desktopHandler(response, request)
	if response.Code != 409 {
		t.Fatalf("a run with no console and no trace answered %d", response.Code)
	}

	// The run list says which runs have one.
	request = httptest.NewRequest("GET", "/desktop/runs", nil)
	request.Header.Set("Authorization", "Bearer private")
	response = httptest.NewRecorder()
	d.desktopHandler(response, request)
	var runs []desktopRun
	if err := json.Unmarshal(response.Body.Bytes(), &runs); err != nil {
		t.Fatal(err)
	}
	for _, run := range runs {
		if run.ID == "traced" && !run.Trace {
			t.Error("a traced run did not report its trace")
		}
		if run.ID == "blind" && run.Trace {
			t.Error("a run with no trace reported one")
		}
	}
}

func TestEveryWatcherGetsItsOwnReplay(t *testing.T) {
	d := &agentDaemon{terminal: terminalChoice{manager: terminal.NewManager()}, loopback: loopbackServer{desktopToken: "private"}}
	trace := newRunTrace()
	trace.write("shown\r\n")
	d.queue.runs = map[string]*controlledRun{"traced": {desktop: desktopRun{ID: "traced", Status: "running", Headless: true, Trace: true}, trace: trace, exited: make(chan struct{})}}
	server := httptest.NewServer(http.HandlerFunc(d.desktopHandler))
	defer server.Close()
	header := http.Header{"Authorization": []string{"Bearer private"}}
	endpoint := "ws" + strings.TrimPrefix(server.URL, "http") + "/desktop/terminal?id=traced"

	for _, name := range []string{"first", "second"} {
		connection, _, err := websocket.DefaultDialer.Dial(endpoint, header)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		_ = connection.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, chunk, err := connection.ReadMessage()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if string(chunk) != "shown\r\n" {
			t.Fatalf("%s: got %q", name, chunk)
		}
		connection.Close()
	}
}
