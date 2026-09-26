package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"tasks/internal/agentconfig"
)

// A server that predates the contract routes answers the catch-all 404. Read as
// a plain HTTP failure it looks like a blip and the agent retries forever, so
// the agent has to name it as the build problem it is.
func TestMissingContractRouteIsReportedAsMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Route API non trouvée: " + r.URL.Path})
	}))
	defer srv.Close()
	d := &agentDaemon{link: serverLink{serverURL: srv.URL, token: "token"}}

	_, err := d.discoverProjects(context.Background())
	if !agentconfig.IsMismatch(err) {
		t.Fatalf("missing discovery route not reported as a contract mismatch: %v", err)
	}
	for _, want := range []string{srv.URL, "/api/v1/agent/projects", "HTTP 404", "older than this agent"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message misses %q: %s", want, err)
		}
	}
	if d.contract.current() != err.Error() {
		t.Errorf("mismatch not exposed to the desktop: %q", d.contract.current())
	}

	// The same server also fails a launch, and that path must say the same thing
	// rather than relaying a bare 404 into the desktop.
	if _, err := d.fetchConfig(context.Background(), "project", ""); !agentconfig.IsMismatch(err) {
		t.Fatalf("missing config route not reported as a contract mismatch: %v", err)
	}
}

// A 404 outside the contract prefix is ordinary API traffic: an absent task is
// not a reason to declare the whole server incompatible.
func TestNonContractNotFoundStaysAPlainFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Task not found"})
	}))
	defer srv.Close()
	d := &agentDaemon{link: serverLink{serverURL: srv.URL, token: "token"}}
	var ignored any
	err := d.readAPI(context.Background(), "/api/tasks/missing", &ignored)
	if err == nil || agentconfig.IsMismatch(err) {
		t.Fatalf("plain 404 reported as a contract mismatch: %v", err)
	}
	if d.contract.current() != "" {
		t.Errorf("unrelated 404 recorded as a mismatch: %q", d.contract.current())
	}
}

// A rejected credential is not a build problem either: the route is there.
func TestUnauthorizedIsNotAMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Valid agent bearer token required"})
	}))
	defer srv.Close()
	d := &agentDaemon{link: serverLink{serverURL: srv.URL, token: "token"}}
	if _, err := d.discoverProjects(context.Background()); err == nil || agentconfig.IsMismatch(err) {
		t.Fatalf("401 reported as a contract mismatch: %v", err)
	}
}

// A served route carrying an unsupported version is the other half of the same
// failure, and has to read the same way.
func TestUnsupportedSchemaVersionIsReportedAsMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(agentconfig.Projects{SchemaVersion: agentconfig.Version + 1})
	}))
	defer srv.Close()
	d := &agentDaemon{link: serverLink{serverURL: srv.URL, token: "token"}}
	_, err := d.discoverProjects(context.Background())
	if !agentconfig.IsMismatch(err) {
		t.Fatalf("unsupported version not reported as a contract mismatch: %v", err)
	}
	if !strings.Contains(err.Error(), "schemaVersion 2") {
		t.Errorf("message does not name the served version: %s", err)
	}
}

// The diagnosis is worth nothing if it scrolls past. A standing mismatch is one
// episode however many calls hit it, and the failing route alternating between
// the connection loop and a desktop call must not reprint the banner.
func TestStandingMismatchIsAnnouncedOnce(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Route API non trouvée: " + r.URL.Path})
	}))
	defer srv.Close()
	// The logger is global: a PTY reader left by an earlier test may still
	// write to it, so the capture has to be safe for concurrent use.
	var announcements lockedBuffer
	log.SetOutput(&announcements)
	defer log.SetOutput(os.Stderr)

	d := &agentDaemon{link: serverLink{serverURL: srv.URL, token: "token"}}
	for range 3 {
		_, _ = d.discoverProjects(context.Background())
		_, _ = d.fetchConfig(context.Background(), "project", "")
	}
	if got := strings.Count(announcements.String(), "Server contract mismatch"); got != 1 {
		t.Fatalf("announced %d times, want once:\n%s", got, announcements.String())
	}
}

// lockedBuffer is a bytes.Buffer that several goroutines may write to.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// Updating and restarting the server is what clears the mismatch, so a contract
// route answering correctly has to retire the warning the desktop is showing.
func TestUpdatedServerClearsTheMismatch(t *testing.T) {
	updated := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !updated {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Route API non trouvée: " + r.URL.Path})
			return
		}
		_ = json.NewEncoder(w).Encode(agentconfig.Projects{SchemaVersion: agentconfig.Version})
	}))
	defer srv.Close()
	d := &agentDaemon{link: serverLink{serverURL: srv.URL, token: "token"}}
	if _, err := d.discoverProjects(context.Background()); !agentconfig.IsMismatch(err) {
		t.Fatalf("setup: %v", err)
	}
	updated = true
	if _, err := d.discoverProjects(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d.contract.current() != "" {
		t.Errorf("mismatch survived an updated server: %q", d.contract.current())
	}
}
