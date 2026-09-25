package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"tasks/internal/agentprotocol"
	"tasks/internal/terminal"
)

// answerServer stands in for the server end of the agent link and hands over
// every run_answered it receives.
func answerServer(t *testing.T) (*websocket.Conn, <-chan agentprotocol.RunAnswered) {
	t.Helper()
	answers := make(chan agentprotocol.RunAnswered, 8)
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
			if msg.Type != agentprotocol.RunAnsweredType {
				continue
			}
			var payload agentprotocol.RunAnswered
			if err := json.Unmarshal(msg.Payload, &payload); err == nil {
				answers <- payload
			}
		}
	}))
	t.Cleanup(server.Close)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn, answers
}

func expectAnswer(t *testing.T, answers <-chan agentprotocol.RunAnswered, runID string, since time.Time) {
	t.Helper()
	select {
	case got := <-answers:
		if got.RunID != runID || !got.WaitingSince.Equal(since) {
			t.Fatalf("the server was told %+v, want run %s answered at %v", got, runID, since)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("the server was never told run %s was answered", runID)
	}
}

func expectNoAnswer(t *testing.T, answers <-chan agentprotocol.RunAnswered) {
	t.Helper()
	select {
	case got := <-answers:
		t.Fatalf("the server was told of an answer nobody gave: %+v", got)
	case <-time.After(150 * time.Millisecond):
	}
}

// waitingDaemon holds one interactive and one headless run, both marked
// waiting by the server.
func waitingDaemon(t *testing.T, since time.Time) *agentDaemon {
	t.Helper()
	d := &agentDaemon{loopback: loopbackServer{desktopToken: "private"}}
	d.queue.runs = map[string]*controlledRun{
		"live":     {desktop: desktopRun{Status: "running"}, exited: make(chan struct{})},
		"idle":     {desktop: desktopRun{Status: "running"}, exited: make(chan struct{})},
		"headless": {desktop: desktopRun{Status: "running", Headless: true}, exited: make(chan struct{})},
	}
	d.handleMessage(context.Background(), nil, waitingMessage(t, "live", &since))
	d.handleMessage(context.Background(), nil, waitingMessage(t, "headless", &since))
	return d
}

// Enter in the run's console answers its question: the glyph drops on the
// desktop at once and the server is told which wait was answered (#475).
func TestEnterInTheConsoleAnswersTheWait(t *testing.T) {
	since := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	d := waitingDaemon(t, since)
	conn, answers := answerServer(t)
	d.link.conn = conn
	manager := terminal.NewManager()
	d.terminal.manager = manager
	for _, id := range []string{"live", "idle", "headless"} {
		if _, err := manager.GetOrCreateSession(id, t.TempDir(), nil); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = manager.CloseSession(id) }()
		d.watchAnswers(id)
		// A run given its console again is not watched twice.
		d.watchAnswers(id)
	}

	// Arrows, plain characters and Escape answer nothing. Ctrl-C is left out: a
	// shell still starting up exits on it.
	for _, keys := range []string{"\x1b[A", "yes", "\x1b"} {
		if err := manager.SendViewerInput("live", keys); err != nil {
			t.Fatal(err)
		}
	}
	expectNoAnswer(t, answers)
	if _, ok := waitingOf(desktopRuns(t, d), "live"); !ok {
		t.Fatal("a key other than Enter cleared the wait")
	}

	if err := manager.SendViewerInput("live", "\r"); err != nil {
		t.Fatal(err)
	}
	expectAnswer(t, answers, "live", since)
	if _, ok := waitingOf(desktopRuns(t, d), "live"); ok {
		t.Fatal("the answered run still shows waiting on the desktop")
	}
	// Only one answer per wait, however many lines follow.
	if err := manager.SendViewerInput("live", "\r"); err != nil {
		t.Fatal(err)
	}
	// Enter on a run that waits for nothing, or on a headless run, says nothing.
	if err := manager.SendViewerInput("idle", "\r"); err != nil {
		t.Fatal(err)
	}
	if err := manager.SendViewerInput("headless", "\r"); err != nil {
		t.Fatal(err)
	}
	expectNoAnswer(t, answers)
}

// A push that crossed the answer still carries the answered mark: it must not
// bring the glyph back, and the answer is repeated. A newer mark is a new
// question and is shown; a clear forgets the answer.
func TestAnAnsweredMarkIsNotRestoredByAnEcho(t *testing.T) {
	since := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	d := waitingDaemon(t, since)
	conn, answers := answerServer(t)
	d.link.conn = conn

	d.answerRun("live")
	expectAnswer(t, answers, "live", since)

	d.handleMessage(context.Background(), nil, waitingMessage(t, "live", &since))
	if _, ok := waitingOf(desktopRuns(t, d), "live"); ok {
		t.Fatal("an echo of the answered wait raised the glyph again")
	}
	expectAnswer(t, answers, "live", since)

	newer := since.Add(time.Minute)
	d.handleMessage(context.Background(), nil, waitingMessage(t, "live", &newer))
	if got, ok := waitingOf(desktopRuns(t, d), "live"); !ok || got != newer.Format(time.RFC3339) {
		t.Fatalf("a new question after the answer is not shown: %q %v", got, ok)
	}
	d.handleMessage(context.Background(), nil, waitingMessage(t, "live", nil))
	d.queue.read("live", func(run *controlledRun) {
		if !run.answeredAt.IsZero() {
			t.Fatal("a clear from the server left the answer pending")
		}
	})
}

// An answer given while the agent is disconnected is kept, and sent as soon
// as it reconnects; one the server confirmed is not.
func TestAnAnswerGivenOfflineIsSentOnReconnection(t *testing.T) {
	since := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	d := waitingDaemon(t, since)

	d.answerRun("live")
	if _, ok := waitingOf(desktopRuns(t, d), "live"); ok {
		t.Fatal("the glyph stays while the agent is offline")
	}

	conn, answers := answerServer(t)
	d.resendAnswers(conn)
	expectAnswer(t, answers, "live", since)

	d.handleMessage(context.Background(), nil, waitingMessage(t, "live", nil))
	d.resendAnswers(conn)
	expectNoAnswer(t, answers)
}
