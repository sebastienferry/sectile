package agent

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"tasks/internal/agentconfig"
)

// relayedBodies records what reached the stub server, in order.
var relayedBodies = struct {
	sync.Mutex
	bodies []string
}{}

// waitingDaemon wires a daemon to a stub server so the relay can be observed,
// and returns the loopback the hook would post to.
func waitingDaemon(t *testing.T) (*agentDaemon, *httptest.Server, *atomic.Int32) {
	t.Helper()
	var relayed atomic.Int32
	relayedBodies.Lock()
	relayedBodies.bodies = nil
	relayedBodies.Unlock()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/activities/live/waiting" && r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			relayedBodies.Lock()
			relayedBodies.bodies = append(relayedBodies.bodies, strings.TrimSpace(string(body)))
			relayedBodies.Unlock()
			relayed.Add(1)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	d := &agentDaemon{link: serverLink{serverURL: server.URL, token: "workstation-key"}}
	if _, err := d.enqueueRun("task", agentconfig.Dispatch{RunID: "live"}, "project", t.TempDir(), 1, false); err != nil {
		t.Fatal(err)
	}
	loopback := httptest.NewServer(http.HandlerFunc(d.handleRunControl))
	t.Cleanup(loopback.Close)
	d.loopback.url = loopback.URL
	// The handler checks the host against the port it believes it listens on.
	if _, port, err := net.SplitHostPort(strings.TrimPrefix(loopback.URL, "http://")); err == nil {
		d.loopback.port, _ = strconv.Atoi(port)
	}
	return d, loopback, &relayed
}

func waitingRequest(t *testing.T, loopback *httptest.Server, runID, token, body string, origin bool) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, loopback.URL+"/control/runs/"+runID+"/waiting", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if origin {
		req.Header.Set("Origin", "http://evil.example")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func TestWaitingReportIsAcceptedAndRelayed(t *testing.T) {
	_, loopback, relayed := waitingDaemon(t)

	resp := waitingRequest(t, loopback, "live", "workstation-key", `{"waiting":true}`, false)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("a valid report answered %d", resp.StatusCode)
	}
	// The relay is deliberately off the response path: the hook is already gone.
	for range 50 {
		if relayed.Load() == 1 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the report never reached the server")
}

func TestWaitingReportIsRefusedWithoutTheWorkstationCredential(t *testing.T) {
	_, loopback, relayed := waitingDaemon(t)

	for _, test := range []struct {
		name   string
		token  string
		origin bool
		want   int
	}{
		{name: "no token", want: http.StatusUnauthorized},
		{name: "wrong token", token: "guess", want: http.StatusUnauthorized},
		{name: "browser origin", token: "workstation-key", origin: true, want: http.StatusForbidden},
	} {
		resp := waitingRequest(t, loopback, "live", test.token, `{"waiting":true}`, test.origin)
		if resp.StatusCode != test.want {
			t.Fatalf("%s answered %d instead of %d", test.name, resp.StatusCode, test.want)
		}
	}
	time.Sleep(100 * time.Millisecond)
	if relayed.Load() != 0 {
		t.Fatal("a refused report was relayed anyway, so a run would change state")
	}
}

func TestWaitingReportIsRefusedForAnUnknownRunOrMalformedBody(t *testing.T) {
	_, loopback, _ := waitingDaemon(t)

	if resp := waitingRequest(t, loopback, "ghost", "workstation-key", `{"waiting":true}`, false); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("an unknown run answered %d instead of 404", resp.StatusCode)
	}
	for _, body := range []string{``, `{}`, `nonsense`} {
		if resp := waitingRequest(t, loopback, "live", "workstation-key", body, false); resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("body %q answered %d instead of 400", body, resp.StatusCode)
		}
	}
}

// waitingMark reads the stamp the desktop will see on its next poll.
func waitingMark(d *agentDaemon, runID string) time.Time {
	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()
	return d.queue.runs[runID].desktop.WaitingSince
}

// awaitRelays waits until the stub server has seen the expected number of
// relays, then a little longer to make sure no extra one follows.
func awaitRelays(t *testing.T, relayed *atomic.Int32, want int32) {
	t.Helper()
	for range 50 {
		if relayed.Load() >= want {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	if got := relayed.Load(); got != want {
		t.Fatalf("%d relays reached the server, expected %d", got, want)
	}
}

// Every tool call reports "working" again, and a prompt left unanswered may be
// reported more than once. Only the transitions reach the server, and a repeated
// wait keeps the stamp of its first report: the wait started then.
func TestOnlyATransitionIsRelayedAndTheFirstStampIsKept(t *testing.T) {
	d, loopback, relayed := waitingDaemon(t)

	waitingRequest(t, loopback, "live", "workstation-key", `{"waiting":false}`, false)
	awaitRelays(t, relayed, 0)

	waitingRequest(t, loopback, "live", "workstation-key", `{"waiting":true}`, false)
	first := waitingMark(d, "live")
	if first.IsZero() {
		t.Fatal("the run is not marked as waiting")
	}
	time.Sleep(10 * time.Millisecond)
	waitingRequest(t, loopback, "live", "workstation-key", `{"waiting":true}`, false)
	if again := waitingMark(d, "live"); !again.Equal(first) {
		t.Fatalf("a repeated report moved the start of wait from %v to %v", first, again)
	}
	for range 3 {
		waitingRequest(t, loopback, "live", "workstation-key", `{"waiting":false}`, false)
	}
	if !waitingMark(d, "live").IsZero() {
		t.Fatal("the run is still marked as waiting after it resumed")
	}
	awaitRelays(t, relayed, 2)
	relayedBodies.Lock()
	defer relayedBodies.Unlock()
	if len(relayedBodies.bodies) != 2 || !strings.Contains(relayedBodies.bodies[0], "true") || !strings.Contains(relayedBodies.bodies[1], "false") {
		t.Fatalf("the server saw %v, expected the wait then the resume", relayedBodies.bodies)
	}
}

// An autonomous run has nobody to wait for: its Stop hook fires as the process
// ends, and a waiting mark there would raise a false banner in the poll before
// the exit is observed. The report is accepted and changes nothing.
func TestAWaitingReportOnAnAutonomousRunChangesNothing(t *testing.T) {
	d, loopback, relayed := waitingDaemon(t)
	d.queue.mu.Lock()
	d.queue.runs["live"].desktop.Headless = true
	d.queue.mu.Unlock()

	if resp := waitingRequest(t, loopback, "live", "workstation-key", `{"waiting":true}`, false); resp.StatusCode != http.StatusAccepted {
		t.Fatalf("the report answered %d; a hook must never be told it failed", resp.StatusCode)
	}
	if !waitingMark(d, "live").IsZero() {
		t.Fatal("an autonomous run was marked as waiting")
	}
	awaitRelays(t, relayed, 0)
}
