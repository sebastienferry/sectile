package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
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

	if !strings.HasSuffix(run.transcript, "the tail that explains the exit\n") {
		t.Fatal("the tail of the run was dropped instead of its head")
	}
	if len(run.transcript) > headlessTranscriptLimit {
		t.Fatalf("transcript = %d bytes, over the budget", len(run.transcript))
	}
	_, payload := runOutput(t, d, "?id=run")
	if payload["truncated"] != true {
		t.Fatalf("the route must report the truncation, got %v", payload["truncated"])
	}
	// The marker is served with the window, not stored in it: counting it as
	// output would shift every later position by its own length.
	if !strings.HasPrefix(payload["output"].(string), headlessTranscriptTruncated) {
		t.Fatalf("a replayed truncated window must open with its marker, got %q", payload["output"])
	}
	if want := run.dropped + len(run.transcript); payload["offset"].(float64) != float64(want) {
		t.Fatalf("offset = %v, want the position in the whole output (%d)", payload["offset"], want)
	}
}

// A pane polls with the position it has already read. Truncation slides the
// window under it, and an index into the kept bytes alone would then hand it
// output it has already shown: the position counts the run's whole output.
func TestDesktopRunOutputKeepsItsPlaceAcrossATruncation(t *testing.T) {
	d := &agentDaemon{loopback: loopbackServer{desktopToken: "private"}}
	run := &controlledRun{desktop: desktopRun{Status: "running", Headless: true}, exited: make(chan struct{})}
	d.queue.runs = map[string]*controlledRun{"run": run}

	d.appendHeadlessTranscript(run, strings.Repeat("a", headlessTranscriptLimit))
	_, payload := runOutput(t, d, "?id=run")
	read := int(payload["offset"].(float64))
	if read != headlessTranscriptLimit {
		t.Fatalf("offset = %d, want the whole output read so far", read)
	}

	// Enough to push the head out, so the kept window no longer starts at the
	// beginning of the run.
	d.appendHeadlessTranscript(run, "tail one\n")
	d.appendHeadlessTranscript(run, "tail two\n")
	if run.dropped == 0 {
		t.Fatal("the head was expected to be dropped")
	}

	_, payload = runOutput(t, d, "?id=run&offset="+strconv.Itoa(read))
	if got := payload["output"]; got != "tail one\ntail two\n" {
		t.Fatalf("output = %q, want only what was printed after the position already read", got)
	}
}
