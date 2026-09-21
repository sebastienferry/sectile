package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// runOutput asks the route the desktop console pane polls.
func runOutput(t *testing.T, d *agentDaemon, query string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/desktop/run-output"+query, nil)
	request.Header.Set("Authorization", "Bearer private")
	response := httptest.NewRecorder()
	d.desktopHandler(response, request)
	payload := map[string]any{}
	if response.Code == http.StatusOK {
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decoding the transcript: %v", err)
		}
	}
	return response, payload
}

// A headless run has no console to attach to, so the pane reads its transcript
// here. An offset is what keeps the pane from reprinting what it already shows.
func TestDesktopRunOutputServesWhatFollowsTheOffset(t *testing.T) {
	d := &agentDaemon{loopback: loopbackServer{desktopToken: "private"}}
	d.queue.runs = map[string]*controlledRun{"run": {
		transcript: "first\nsecond\n",
		desktop:    desktopRun{Status: "running", Headless: true},
		exited:     make(chan struct{}),
	}}

	response, payload := runOutput(t, d, "?id=run")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if payload["output"] != "first\nsecond\n" {
		t.Fatalf("output = %q, want the whole transcript", payload["output"])
	}
	if payload["offset"].(float64) != 13 || payload["headless"] != true || payload["status"] != "running" {
		t.Fatalf("payload = %v", payload)
	}

	_, payload = runOutput(t, d, "?id=run&offset=6")
	if payload["output"] != "second\n" {
		t.Fatalf("output = %q, want only what follows the offset", payload["output"])
	}

	// An offset past the end is what a truncated transcript leaves behind:
	// rewinding beats reporting nothing at all.
	_, payload = runOutput(t, d, "?id=run&offset=9000")
	if payload["output"] != "first\nsecond\n" {
		t.Fatalf("output = %q, want the transcript replayed from the start", payload["output"])
	}
}

func TestDesktopRunOutputReportsAnUnknownRun(t *testing.T) {
	d := &agentDaemon{loopback: loopbackServer{desktopToken: "private"}}
	d.queue.runs = map[string]*controlledRun{}
	response, _ := runOutput(t, d, "?id=gone")
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
}

// The transcript is bounded like the activity record, and says so where it cuts:
// a user reading a shortened run must not take the truncation for the run.
func TestHeadlessTranscriptTruncatesItsHeadAndSaysSo(t *testing.T) {
	d := &agentDaemon{loopback: loopbackServer{desktopToken: "private"}}
	run := &controlledRun{desktop: desktopRun{Status: "running", Headless: true}, exited: make(chan struct{})}
	d.queue.runs = map[string]*controlledRun{"run": run}

	d.appendHeadlessTranscript(run, strings.Repeat("a", headlessTranscriptLimit))
	d.appendHeadlessTranscript(run, "the tail that explains the exit\n")

	if !strings.HasPrefix(run.transcript, headlessTranscriptTruncated) {
		t.Fatalf("a truncated transcript must open with its marker, got %q", run.transcript[:80])
	}
	if !strings.HasSuffix(run.transcript, "the tail that explains the exit\n") {
		t.Fatal("the tail of the run was dropped instead of its head")
	}
	if len(run.transcript) > headlessTranscriptLimit+len(headlessTranscriptTruncated) {
		t.Fatalf("transcript = %d bytes, over the budget", len(run.transcript))
	}
	_, payload := runOutput(t, d, "?id=run")
	if payload["truncated"] != true {
		t.Fatalf("the route must report the truncation, got %v", payload["truncated"])
	}
}
