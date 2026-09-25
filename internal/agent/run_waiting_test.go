package agent

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"tasks/internal/agentprotocol"
)

// waitingMessage builds the message the server sends when a run's waiting mark
// changes; a nil instant clears it.
func waitingMessage(t *testing.T, runID string, since *time.Time) agentprotocol.Message {
	t.Helper()
	raw, err := json.Marshal(agentprotocol.RunWaiting{RunID: runID, WaitingSince: since})
	if err != nil {
		t.Fatal(err)
	}
	return agentprotocol.Message{MsgID: "m", Type: agentprotocol.RunWaitingType, Payload: raw}
}

// desktopRuns reads the list the desktop polls, which is what its banner is
// raised from.
func desktopRuns(t *testing.T, d *agentDaemon) []map[string]any {
	t.Helper()
	request := httptest.NewRequest("GET", "/desktop/runs", nil)
	request.Header.Set("Authorization", "Bearer private")
	response := httptest.NewRecorder()
	d.desktopHandler(response, request)
	if response.Code != 200 {
		t.Fatalf("GET /desktop/runs = %d %s", response.Code, response.Body.String())
	}
	var runs []map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &runs); err != nil {
		t.Fatal(err)
	}
	return runs
}

func waitingOf(runs []map[string]any, id string) (string, bool) {
	for _, run := range runs {
		if run["id"] == id {
			value, ok := run["waitingSince"].(string)
			return value, ok
		}
	}
	return "", false
}

func TestRunWaitingReachesTheDesktopRunList(t *testing.T) {
	d := &agentDaemon{loopback: loopbackServer{desktopToken: "private"}}
	d.queue.runs = map[string]*controlledRun{
		"live":     {desktop: desktopRun{Status: "running"}, exited: make(chan struct{})},
		"headless": {desktop: desktopRun{Status: "running", Headless: true}, exited: make(chan struct{})},
	}
	since := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)

	d.handleMessage(context.Background(), nil, waitingMessage(t, "live", &since))
	d.handleMessage(context.Background(), nil, waitingMessage(t, "headless", &since))
	// A run this agent does not hold belongs to another workstation.
	d.handleMessage(context.Background(), nil, waitingMessage(t, "elsewhere", &since))

	runs := desktopRuns(t, d)
	if got, ok := waitingOf(runs, "live"); !ok || got != since.Format(time.RFC3339) {
		t.Fatalf("live run waitingSince = %q, %v; want the server's instant", got, ok)
	}
	if _, ok := waitingOf(runs, "headless"); ok {
		t.Fatal("a headless run was marked waiting, which would raise a false banner")
	}
	if len(runs) != 2 {
		t.Fatalf("an unknown run was added to the list: %v", runs)
	}

	d.handleMessage(context.Background(), nil, waitingMessage(t, "live", nil))
	if _, ok := waitingOf(desktopRuns(t, d), "live"); ok {
		t.Fatal("the cleared wait is still on the desktop run list")
	}
}
