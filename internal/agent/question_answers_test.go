package agent

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"tasks/internal/agentprotocol"
)

// skippedByOwner is what the Tauri question form sends for a question the
// owner skipped on its own (#904).
const skippedByOwner = "Skipped by the owner"

// stepServer stands in for the server end of the agent link and hands over
// every run_step it receives.
func stepServer(t *testing.T) (*websocket.Conn, <-chan agentprotocol.RunStep) {
	t.Helper()
	steps := make(chan agentprotocol.RunStep, 8)
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			var msg agentprotocol.Message
			if err := conn.ReadJSON(&msg); err != nil {
				return
			}
			var payload agentprotocol.RunStep
			if msg.Type == agentprotocol.RunStepType && json.Unmarshal(msg.Payload, &payload) == nil {
				steps <- payload
			}
		}
	}))
	t.Cleanup(server.Close)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn, steps
}

// stdinBuffer records what the agent writes to the engine's stdin.
type stdinBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *stdinBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *stdinBuffer) Close() error { return nil }

func (b *stdinBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// questionRun is a conversation run waiting on one question request.
func questionRun(taskID, provider string, approval conversationApproval) (*controlledRun, *stdinBuffer) {
	stdin := &stdinBuffer{}
	run := &controlledRun{
		desktop:      desktopRun{ID: "run-1", TaskID: taskID, Provider: provider},
		trace:        newRunTrace(),
		conversation: &providerConversation{input: &conversationInput{w: stdin}, approvals: []conversationApproval{approval}},
	}
	return run, stdin
}

func traceText(run *controlledRun) string {
	lines, _ := run.trace.snapshot()
	return strings.Join(lines, "\n")
}

func expectStep(t *testing.T, steps <-chan agentprotocol.RunStep) agentprotocol.RunStep {
	t.Helper()
	select {
	case step := <-steps:
		return step
	case <-time.After(3 * time.Second):
		t.Fatal("the server was never sent the answers as a run step")
	}
	return agentprotocol.RunStep{}
}

func expectNoStep(t *testing.T, steps <-chan agentprotocol.RunStep) {
	t.Helper()
	select {
	case step := <-steps:
		t.Fatalf("a run step was sent that should not have been: %+v", step)
	case <-time.After(150 * time.Millisecond):
	}
}

var claudeQuestions = conversationApproval{ID: "q", Tool: askUserQuestion, Input: json.RawMessage(`{"questions":[{"question":"Which color?","options":[{"label":"Red (Recommended)"},{"label":"Blue"}]},{"question":"Token?","isSecret":true},{"question":"Why?"}]}`)}

func TestClaudeAnswersMaskSecretsAndReachTheRunActivity(t *testing.T) {
	conn, steps := stepServer(t)
	d := &agentDaemon{}
	d.link.conn = conn
	run, stdin := questionRun("task-1", "", claudeQuestions)
	answers := map[string]string{"Which color?": "Red (Recommended)", "Token?": "secret-fixture", "Why?": skippedByOwner}
	if err := d.decideApprovalLocked(run, "q", "answer", answers); err != nil {
		t.Fatalf("answers refused: %v", err)
	}
	if sent := stdin.String(); !strings.Contains(sent, "secret-fixture") || !strings.Contains(sent, skippedByOwner) || !strings.Contains(sent, "Red (Recommended)") {
		t.Fatalf("Claude did not receive the answers as given: %s", sent)
	}
	if trace := traceText(run); strings.Contains(trace, "secret-fixture") || !strings.Contains(trace, "Token? → [hidden]") {
		t.Fatalf("the transcript does not mask the secret answer: %s", trace)
	}
	step := expectStep(t, steps)
	want := "Owner answers:\nToken? → [hidden]\nWhich color? → Red (Recommended)\nWhy? → " + skippedByOwner
	if step.RunID != "run-1" || step.Text != want {
		t.Fatalf("step = %+v, want run-1 with %q", step, want)
	}
}

func TestQuestionAnswersAddNoStepOutsideATaskOrOnASkippedRequest(t *testing.T) {
	conn, steps := stepServer(t)
	d := &agentDaemon{}
	d.link.conn = conn
	free, _ := questionRun("", "", claudeQuestions)
	if err := d.decideApprovalLocked(free, "q", "answer", map[string]string{"Which color?": "Blue", "Token?": "x", "Why?": "y"}); err != nil {
		t.Fatalf("answers refused: %v", err)
	}
	if !strings.Contains(traceText(free), "Which color? → Blue") {
		t.Fatalf("the transcript lost the answers: %s", traceText(free))
	}
	denied, _ := questionRun("task-1", "", claudeQuestions)
	if err := d.decideApprovalLocked(denied, "q", "deny", nil); err != nil {
		t.Fatalf("skip refused: %v", err)
	}
	expectNoStep(t, steps)
}

func TestQuestionAnswersReachTheAgentWithoutAServerLink(t *testing.T) {
	d := &agentDaemon{}
	run, stdin := questionRun("task-1", "", claudeQuestions)
	if err := d.decideApprovalLocked(run, "q", "answer", map[string]string{"Which color?": "Blue", "Token?": "x", "Why?": "y"}); err != nil {
		t.Fatalf("answers refused without a server link: %v", err)
	}
	if !strings.Contains(stdin.String(), `"Which color?":"Blue"`) {
		t.Fatalf("Claude did not receive the answers: %s", stdin.String())
	}
}

func TestSkippedQuestionPassesTheAnswerCheck(t *testing.T) {
	ask := conversationApproval{Tool: askUserQuestion, Input: json.RawMessage(`{"questions":[{"question":"A?"},{"question":"B?"}]}`)}
	if err := checkAnswers(ask, "answer", map[string]string{"A?": skippedByOwner, "B?": skippedByOwner}); err != nil {
		t.Fatalf("every question skipped was refused: %v", err)
	}
}

func TestCodexSkippedAndSecretAnswersAreSentAndMasked(t *testing.T) {
	conn, steps := stepServer(t)
	d := &agentDaemon{}
	d.link.conn = conn
	approval := conversationApproval{
		ID: "codex-q", Tool: askUserQuestion, Method: "item/tool/requestUserInput", RequestID: json.RawMessage(`"question-1"`),
		Input:           json.RawMessage(`{"questions":[{"question":"Scope?"},{"question":"Key?","isSecret":true}]}`),
		QuestionIDs:     map[string]string{"Scope?": "q_1", "Key?": "q_2"},
		QuestionSecrets: map[string]bool{"Key?": true},
	}
	run, stdin := questionRun("task-1", "codex", approval)
	if err := d.decideApprovalLocked(run, "codex-q", "answer", map[string]string{"Scope?": skippedByOwner, "Key?": "secret-fixture"}); err != nil {
		t.Fatalf("answers refused: %v", err)
	}
	var sent struct {
		Result struct {
			Answers map[string]struct {
				Answers []string `json:"answers"`
			} `json:"answers"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdin.String())), &sent); err != nil {
		t.Fatalf("Codex reply unreadable: %v %s", err, stdin.String())
	}
	if got := sent.Result.Answers["q_1"].Answers; len(got) != 1 || got[0] != skippedByOwner {
		t.Fatalf("a skipped question reached Codex as %v", got)
	}
	if got := sent.Result.Answers["q_2"].Answers; len(got) != 1 || got[0] != "secret-fixture" {
		t.Fatalf("the secret answer reached Codex as %v", got)
	}
	if trace := traceText(run); strings.Contains(trace, "secret-fixture") || !strings.Contains(trace, "Key? → [hidden]") {
		t.Fatalf("the transcript does not mask the secret answer: %s", trace)
	}
	step := expectStep(t, steps)
	if step.Text != "Owner answers:\nKey? → [hidden]\nScope? → "+skippedByOwner {
		t.Fatalf("step = %q", step.Text)
	}
}
