package agent

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"tasks/internal/agentconfig"
)

// waitingDaemon wires a daemon to a stub server so the relay can be observed,
// and returns the loopback the hook would post to.
func waitingDaemon(t *testing.T) (*agentDaemon, *httptest.Server, *atomic.Int32) {
	t.Helper()
	var relayed atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/activities/live/waiting" && r.Method == http.MethodPost {
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
