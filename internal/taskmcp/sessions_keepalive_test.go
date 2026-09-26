package taskmcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"tasks/internal/db"
	"tasks/internal/models"
)

const (
	testKeepalive = 50 * time.Millisecond
	testFailures  = 3
)

// keepaliveServer serves the MCP endpoint the way the server does, with a
// registry that pings every testKeepalive, or never when keepalive is false.
type keepaliveServer struct {
	database *db.DB
	registry *SessionRegistry
	url      string
	proxy    *idleProxy
	server   *httptest.Server
}

func newKeepaliveServer(t *testing.T, keepalive bool) *keepaliveServer {
	t.Helper()
	database, err := db.NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	// Silence and abandon bounds far beyond the test, so only the keepalive
	// can close a session here.
	registry := NewSessionRegistryBounded(database, database, time.Hour, time.Hour)
	t.Cleanup(registry.Stop)
	if keepalive {
		registry.SetKeepalive(testKeepalive, testFailures)
	}
	resolve := func(http.Header) (Caller, bool) { return tester, true }
	server := NewServerWithCallers(database, registry, resolve)
	proxy := &idleProxy{idle: 6 * testKeepalive, next: mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{JSONResponse: true, SessionTimeout: 0},
	)}
	srv := httptest.NewServer(proxy)
	t.Cleanup(srv.Close)
	return &keepaliveServer{database: database, registry: registry, url: srv.URL, proxy: proxy, server: srv}
}

// idleProxy stands for the ingress in front of the server: it cuts a GET
// stream that carried nothing for idle, as HAProxy's timeout-server does.
type idleProxy struct {
	next http.Handler
	idle time.Duration
	gets atomic.Int32
	cuts atomic.Int32
}

func (p *idleProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		p.next.ServeHTTP(w, r)
		return
	}
	p.gets.Add(1)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	written := make(chan struct{}, 1)
	go func() {
		timer := time.NewTimer(p.idle)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-written:
				timer.Reset(p.idle)
			case <-timer.C:
				p.cuts.Add(1)
				cancel()
				return
			}
		}
	}()
	p.next.ServeHTTP(&activityWriter{ResponseWriter: w, written: written}, r.WithContext(ctx))
}

type activityWriter struct {
	http.ResponseWriter
	written chan struct{}
}

func (w *activityWriter) Write(b []byte) (int, error) {
	select {
	case w.written <- struct{}{}:
	default:
	}
	return w.ResponseWriter.Write(b)
}

func (w *activityWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// liveSession reads one registered session under the lock.
func (s *keepaliveServer) session(id string) (liveSession, bool) {
	s.registry.mu.Lock()
	defer s.registry.mu.Unlock()
	entry := s.registry.live[id]
	if entry == nil {
		return liveSession{}, false
	}
	return *entry, true
}

func (s *keepaliveServer) count() int {
	s.registry.mu.Lock()
	defer s.registry.mu.Unlock()
	return len(s.registry.live)
}

// rawClient speaks MCP by hand and never reads a server-initiated message, so
// it stands for a client that went away without a word: it opens no stream and
// answers no ping.
type rawClient struct {
	t       *testing.T
	url     string
	id      string
	http    *http.Client
	counter int
}

func (c *rawClient) post(body map[string]any) *http.Response {
	c.t.Helper()
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, c.url, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if c.id != "" {
		req.Header.Set("Mcp-Session-Id", c.id)
		req.Header.Set("Mcp-Protocol-Version", "2025-06-18")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatalf("posting %v: %v", body["method"], err)
	}
	return resp
}

func (c *rawClient) request(method string, params map[string]any) map[string]any {
	c.t.Helper()
	c.counter++
	resp := c.post(map[string]any{"jsonrpc": "2.0", "id": c.counter, "method": method, "params": params})
	defer resp.Body.Close()
	var out map[string]any
	data, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(data, &out); err != nil {
		c.t.Fatalf("%s answered %d %q: %v", method, resp.StatusCode, data, err)
	}
	if resp.Header.Get("Mcp-Session-Id") != "" {
		c.id = resp.Header.Get("Mcp-Session-Id")
	}
	return out
}

func connectRaw(t *testing.T, url string) *rawClient {
	t.Helper()
	c := &rawClient{t: t, url: url, http: &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}}
	c.request("initialize", map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{},
		"clientInfo": map[string]any{"name": "gone", "version": "1"}})
	resp := c.post(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	resp.Body.Close()
	if c.id == "" {
		t.Fatal("the server gave no session id")
	}
	return c
}

func eventually(t *testing.T, within time.Duration, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s: still not true after %s", what, within)
}

// A connected client keeps one session across a silence longer than the
// proxy's idle timeout, because the pings keep its stream busy (#517). Without
// them the same proxy cuts the stream, which is what made clients open a new
// session every ~152s.
func TestAPingedStreamOutlivesTheProxyIdleTimeout(t *testing.T) {
	for _, keepalive := range []bool{true, false} {
		s := newKeepaliveServer(t, keepalive)
		ctx := context.Background()
		client, err := mcp.NewClient(&mcp.Implementation{Name: "claude-code", Version: "1"}, nil).
			Connect(ctx, &mcp.StreamableClientTransport{Endpoint: s.url}, nil)
		if err != nil {
			t.Fatal(err)
		}
		// Say nothing for several idle windows.
		time.Sleep(4 * s.proxy.idle)

		if keepalive {
			if cuts := s.proxy.cuts.Load(); cuts != 0 {
				t.Fatalf("the proxy cut %d pinged stream(s), want none", cuts)
			}
			if gets := s.proxy.gets.Load(); gets != 1 {
				t.Fatalf("the client opened %d streams, want one", gets)
			}
			if s.count() != 1 {
				t.Fatalf("%d sessions registered, want one", s.count())
			}
			if _, ok := s.session(client.ID()); !ok {
				t.Fatalf("the client's session %s is no longer registered", client.ID())
			}
		} else if s.proxy.cuts.Load() == 0 {
			t.Fatal("without the keepalive the proxy cut nothing: the test does not reproduce the idle cut")
		}
		client.Close()
	}
}

// A session whose client went away without a word, and which owns no run, is
// closed after the failure threshold, and what it held is released.
func TestAnOrphanedSessionIsClosedAndReleased(t *testing.T) {
	s := newKeepaliveServer(t, true)
	baseline := runtime.NumGoroutine()

	c := connectRaw(t, s.url)
	if _, ok := s.session(c.id); !ok {
		t.Fatalf("session %s is not registered", c.id)
	}
	eventually(t, 20*testKeepalive, "the orphaned session is closed", func() bool { return s.count() == 0 })
	c.http.CloseIdleConnections()
	eventually(t, 2*time.Second, "the goroutines return to their baseline", func() bool {
		return runtime.NumGoroutine() <= baseline
	})
}

// A client that keeps talking is alive even when its pings cannot reach it, as
// for a client that never opens the standalone stream.
func TestAClientThatTalksIsKeptWhateverItsPings(t *testing.T) {
	s := newKeepaliveServer(t, true)
	c := connectRaw(t, s.url)
	stop := time.Now().Add(10 * testKeepalive)
	for time.Now().Before(stop) {
		c.request("ping", map[string]any{})
		time.Sleep(testKeepalive / 2)
	}
	if _, ok := s.session(c.id); !ok {
		t.Fatal("a talking client's session was closed")
	}
}

// A session that owns a run is left to the silence and abandon bounds: the
// keepalive never closes it, and never ends its run.
func TestASessionOwningARunIsNotClosedByTheKeepalive(t *testing.T) {
	s := newKeepaliveServer(t, true)
	task, err := s.database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Owned", Source: "local"})
	if err != nil {
		t.Fatal(err)
	}
	c := connectRaw(t, s.url)
	out := c.request("tools/call", map[string]any{"name": "start_run", "arguments": map[string]any{"taskKey": task.Key, "skill": "pickup"}})
	result, _ := out["result"].(map[string]any)
	run, _ := result["structuredContent"].(map[string]any)
	runID, _ := run["id"].(string)
	if runID == "" {
		t.Fatalf("start_run answered %v", out)
	}

	eventually(t, 20*testKeepalive, "the pings fail past the threshold", func() bool {
		entry, ok := s.session(c.id)
		return ok && entry.pingFailures > testFailures
	})
	if _, ok := s.session(c.id); !ok {
		t.Fatal("the session owning a run was closed by the keepalive")
	}
	activity, err := s.database.GetActivityByID(runID)
	if err != nil || activity == nil || activity.Status != "running" {
		t.Fatalf("run = %+v (%v), want it still running", activity, err)
	}
}

// A ping reply is not the client speaking: it leaves the session's last
// activity where the client's own last message put it, so a silence is
// remarked upon at the usual bound.
func TestAPingReplyIsNotTheClientSpeaking(t *testing.T) {
	s := newKeepaliveServer(t, true)
	client, err := mcp.NewClient(&mcp.Implementation{Name: "claude-code", Version: "1"}, nil).
		Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: s.url}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	eventually(t, time.Second, "the session is registered", func() bool { _, ok := s.session(client.ID()); return ok })
	before, _ := s.session(client.ID())

	time.Sleep(5 * testKeepalive)
	after, ok := s.session(client.ID())
	if !ok {
		t.Fatal("an answering session was closed")
	}
	if !after.lastSeen.Equal(before.lastSeen) {
		t.Fatalf("lastSeen moved from %s to %s on ping replies alone", before.lastSeen, after.lastSeen)
	}
	if after.pingFailures != 0 {
		t.Fatalf("an answering client has %d failed pings, want none", after.pingFailures)
	}
}
