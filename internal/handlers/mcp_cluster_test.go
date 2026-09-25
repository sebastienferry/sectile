package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"tasks/internal/db"
	"tasks/internal/models"
	"tasks/internal/taskmcp"
)

// These tests run #408 on two server instances in one process: each has its
// own store, public endpoint and internal listener, and they find each other
// through a directory the test controls, so an owner can be declared dead or
// pointed at an address that does not answer.

const (
	mcpClusterToken = "internal-token"
	mcpClientKey    = "client-key"
)

// mcpInstances is the liveness table the instances of a test share: instance
// id to internal address.
type mcpInstances struct {
	mu   sync.Mutex
	live map[string]string
}

func (s *mcpInstances) set(id, address string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.live[id] = address
}

func (s *mcpInstances) remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.live, id)
}

// mcpInstanceView is one instance's handle on the shared table.
type mcpInstanceView struct {
	shared *mcpInstances
	id     string
}

func (v *mcpInstanceView) InstanceID() string { return v.id }

func (v *mcpInstanceView) LiveInstance(id string) (db.InstanceLocation, bool) {
	v.shared.mu.Lock()
	defer v.shared.mu.Unlock()
	address, ok := v.shared.live[id]
	return db.InstanceLocation{ID: id, Address: address}, ok
}

func (v *mcpInstanceView) LiveInstances() []db.InstanceLocation {
	v.shared.mu.Lock()
	defer v.shared.mu.Unlock()
	var out []db.InstanceLocation
	for id, address := range v.shared.live {
		out = append(out, db.InstanceLocation{ID: id, Address: address})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// mcpNode is one server instance of a test.
type mcpNode struct {
	h        *Handler
	db       *db.DB
	public   *httptest.Server
	internal *httptest.Server
	// forwarded counts the requests the internal listener received, by method.
	forwarded sync.Map
}

func (n *mcpNode) id() string { return n.db.InstanceID() }

func (n *mcpNode) forwardedCount(method string) int64 {
	if v, ok := n.forwarded.Load(method); ok {
		return v.(*atomic.Int64).Load()
	}
	return 0
}

func newMCPNode(t *testing.T, shared *mcpInstances, database *db.DB) *mcpNode {
	t.Helper()
	if database == nil {
		var err error
		database, err = db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { database.Close() })
	}
	n := &mcpNode{h: NewHandler(database), db: database}
	t.Cleanup(n.h.mcpSessions.Stop)

	public := http.NewServeMux()
	public.Handle("/mcp", n.h.MCPHandler())
	public.HandleFunc("/api/mcp/sessions", n.h.HandleMCPSessions)
	n.public = httptest.NewServer(public)

	internal := http.NewServeMux()
	internal.Handle(internalMCPPath, n.h.InternalMCPHandler())
	internal.Handle(internalMCPSessionsPath, n.h.InternalMCPSessionsHandler())
	n.internal = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter, _ := n.forwarded.LoadOrStore(r.Method, &atomic.Int64{})
		counter.(*atomic.Int64).Add(1)
		internal.ServeHTTP(w, r)
	}))
	t.Cleanup(func() {
		n.public.CloseClientConnections()
		n.public.Close()
		n.internal.CloseClientConnections()
		n.internal.Close()
	})

	if shared != nil {
		shared.set(n.id(), n.internal.URL)
		n.h.setMCPCluster(&mcpInstanceView{shared: shared, id: n.id()}, mcpClusterToken, nil)
	}
	return n
}

// routeTransport sends each request of an MCP client to the public endpoint
// pick chooses, as a load balancer would, with the client's key.
type routeTransport struct {
	pick func(r *http.Request) *httptest.Server
}

func (tr routeTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	target := tr.pick(r)
	r = r.Clone(r.Context())
	r.URL.Host = strings.TrimPrefix(target.URL, "http://")
	r.Host = ""
	r.Header.Set("Authorization", "Bearer "+mcpClientKey)
	return http.DefaultTransport.RoundTrip(r)
}

// initializeOn sends an initialization to first and everything after it to
// rest: the session is created on first, and every later request reaches rest.
func initializeOn(first, rest *mcpNode) func(*http.Request) *httptest.Server {
	return func(r *http.Request) *httptest.Server {
		if r.Header.Get(mcpSessionHeader) == "" {
			return first.public
		}
		return rest.public
	}
}

func connectThrough(t *testing.T, pick func(*http.Request) *httptest.Server, opts *mcp.ClientOptions) *mcp.ClientSession {
	t.Helper()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "cluster-test", Version: "1"}, opts).Connect(context.Background(),
		&mcp.StreamableClientTransport{Endpoint: "http://placeholder/mcp", HTTPClient: &http.Client{Transport: routeTransport{pick: pick}}}, nil)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return session
}

func mcpTestTask(t *testing.T, d *db.DB, title string) *models.Task {
	t.Helper()
	task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: title, Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func startRunThrough(t *testing.T, session *mcp.ClientSession, taskID string) string {
	t.Helper()
	var run struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(callTool(t, session, "start_run", map[string]any{"taskKey": taskID, "skill": "implement"})), &run); err != nil || run.ID == "" {
		t.Fatalf("start_run answered no run id: %v", err)
	}
	return run.ID
}

func sessionRuns(n *mcpNode) map[string][]string {
	out := map[string][]string{}
	for _, view := range n.h.mcpSessions.Snapshot() {
		out[view.ID] = view.Runs
	}
	return out
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// rawMCP sends one request to an MCP endpoint and returns the status and body.
func rawMCP(t *testing.T, method, url string, headers map[string]string, body string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

const rawInitialize = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"raw","version":"1"}}}`
const rawPing = `{"jsonrpc":"2.0","id":2,"method":"ping"}`

// The whole life of a session held by A, every request after the first one
// reaching B: tool calls, runs, the event stream and the closing DELETE.
func TestAnMCPSessionIsServedByItsOwnerWhicheverInstanceReceivesIt(t *testing.T) {
	t.Setenv("SECTILE_SERVER_TOKEN", mcpClientKey)
	shared := &mcpInstances{live: map[string]string{}}
	a, b := newMCPNode(t, shared, nil), newMCPNode(t, shared, nil)
	task := mcpTestTask(t, a.db, "held by A")

	changed := make(chan struct{}, 16)
	session := connectThrough(t, initializeOn(a, b), &mcp.ClientOptions{
		ToolListChangedHandler: func(context.Context, *mcp.ToolListChangedRequest) { changed <- struct{}{} },
	})
	defer session.Close()
	if owner := taskmcp.SessionOwner(session.ID()); owner != a.id() {
		t.Fatalf("session %q is owned by %q, want A (%s)", session.ID(), owner, a.id())
	}

	// The task exists only in A's store: B answering from its own would fail.
	if got := callTool(t, session, "get_task", map[string]any{"taskKey": task.ID}); !strings.Contains(got, "held by A") {
		t.Fatalf("get_task through B = %s", got)
	}

	finished := startRunThrough(t, session, task.ID)
	if runs := sessionRuns(a)[session.ID()]; len(runs) != 1 || runs[0] != finished {
		t.Fatalf("A's session runs = %v, want the run started through B", runs)
	}
	callTool(t, session, "finish_run", map[string]any{"taskKey": task.ID, "runId": finished, "status": "completed", "note": "done"})
	if runs := sessionRuns(a)[session.ID()]; len(runs) != 0 {
		t.Fatalf("A still holds %v after finish_run", runs)
	}
	if run, _ := a.db.GetActivityByID(finished); run == nil || run.Status != "completed" {
		t.Fatalf("finished run = %+v, want completed", run)
	}

	// The event stream is opened through B and carries what A emits. A tool
	// added on A is announced to every session; the stream may still be
	// opening, so the announcement is repeated until one arrives.
	eventually(t, "the GET stream to reach A", func() bool { return a.forwardedCount(http.MethodGet) > 0 })
	delivered := false
	for i := 0; i < 50 && !delivered; i++ {
		mcp.AddTool(a.h.mcpServer, &mcp.Tool{Name: fmt.Sprintf("probe_%d", i)},
			func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
				return &mcp.CallToolResult{}, nil, nil
			})
		select {
		case <-changed:
			delivered = true
		case <-time.After(100 * time.Millisecond):
		}
	}
	if !delivered {
		t.Fatal("no notification emitted on A reached the client through B")
	}

	// The DELETE reaches A through B: A ends the session and cancels the run
	// it still owned.
	abandoned := startRunThrough(t, session, task.ID)
	if err := session.Close(); err != nil {
		t.Fatalf("closing: %v", err)
	}
	eventually(t, "A to end the session", func() bool { return len(a.h.mcpSessions.Snapshot()) == 0 })
	eventually(t, "A to cancel the run the session still owned", func() bool {
		run, _ := a.db.GetActivityByID(abandoned)
		return run != nil && run.Status == "canceled" && strings.Contains(run.Summary, models.RunDisconnectNote)
	})

	for _, method := range []string{http.MethodPost, http.MethodGet, http.MethodDelete} {
		if a.forwardedCount(method) == 0 {
			t.Errorf("no %s was forwarded to A", method)
		}
	}
	if got := b.h.mcpSessions.Snapshot(); len(got) != 0 {
		t.Errorf("B holds sessions of its own: %+v", got)
	}
}

// A session dies with its instance: the client is told it is not found, and a
// new session on a survivor works.
func TestASessionWhoseOwnerIsGoneIsNotFound(t *testing.T) {
	t.Setenv("SECTILE_SERVER_TOKEN", mcpClientKey)
	shared := &mcpInstances{live: map[string]string{}}
	a, b := newMCPNode(t, shared, nil), newMCPNode(t, shared, nil)
	session := connectThrough(t, initializeOn(a, b), nil)
	defer session.Close()
	callTool(t, session, "list_projects", nil)

	shared.remove(a.id())
	status, body := rawMCP(t, http.MethodPost, b.public.URL+"/mcp",
		map[string]string{"Authorization": "Bearer " + mcpClientKey, mcpSessionHeader: session.ID()}, rawPing)
	if status != http.StatusNotFound {
		t.Fatalf("a request for a dead owner's session answered %d %s, want 404", status, body)
	}
	if a.forwardedCount(http.MethodPost) != 2 {
		// The initialized notification and list_projects; nothing after A died.
		t.Errorf("A received %d forwarded POSTs, want 2", a.forwardedCount(http.MethodPost))
	}

	again := connectThrough(t, func(*http.Request) *httptest.Server { return b.public }, nil)
	defer again.Close()
	if owner := taskmcp.SessionOwner(again.ID()); owner != b.id() {
		t.Fatalf("new session owned by %q, want B", owner)
	}
	callTool(t, again, "list_projects", nil)
}

// An owner still listed as live but that cannot be reached is named in a 503,
// after exactly one attempt.
func TestAnUnreachableOwnerIsAnsweredOnceWith503(t *testing.T) {
	t.Setenv("SECTILE_SERVER_TOKEN", mcpClientKey)
	shared := &mcpInstances{live: map[string]string{}}
	a, b := newMCPNode(t, shared, nil), newMCPNode(t, shared, nil)
	session := connectThrough(t, initializeOn(a, b), nil)
	defer session.Close()

	// A listener that accepts and hangs up at once: reached, never answering.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	var accepted atomic.Int64
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			accepted.Add(1)
			conn.Close()
		}
	}()
	shared.set(a.id(), "http://"+listener.Addr().String())

	status, body := rawMCP(t, http.MethodPost, b.public.URL+"/mcp",
		map[string]string{"Authorization": "Bearer " + mcpClientKey, mcpSessionHeader: session.ID()}, rawPing)
	if status != http.StatusServiceUnavailable || !strings.Contains(body, a.id()) {
		t.Fatalf("answer = %d %s, want 503 naming %s", status, body, a.id())
	}
	if got := accepted.Load(); got != 1 {
		t.Errorf("the owner was tried %d times, want once", got)
	}
}

// The internal endpoint takes both the deployment's credential and a client
// credential the public endpoint would have taken.
func TestForwardedMCPRequestsNeedBothCredentials(t *testing.T) {
	t.Setenv("SECTILE_SERVER_TOKEN", mcpClientKey)
	shared := &mcpInstances{live: map[string]string{}}
	a := newMCPNode(t, shared, nil)
	url := a.internal.URL + internalMCPPath
	client := "Bearer " + mcpClientKey
	internal := "Bearer " + mcpClusterToken
	for name, c := range map[string]struct {
		headers map[string]string
		want    int
	}{
		"client credential only":    {map[string]string{"Authorization": client}, http.StatusUnauthorized},
		"wrong internal":            {map[string]string{"Authorization": client, internalAuthHeader: "Bearer nope"}, http.StatusUnauthorized},
		"internal as Authorization": {map[string]string{"Authorization": internal}, http.StatusUnauthorized},
		"no client credential":      {map[string]string{internalAuthHeader: internal}, http.StatusUnauthorized},
		"wrong client credential":   {map[string]string{"Authorization": "Bearer stolen", internalAuthHeader: internal}, http.StatusUnauthorized},
		"both":                      {map[string]string{"Authorization": client, internalAuthHeader: internal}, http.StatusOK},
	} {
		if status, body := rawMCP(t, http.MethodPost, url, c.headers, rawInitialize); status != c.want {
			t.Errorf("%s: answered %d %s, want %d", name, status, body, c.want)
		}
	}
	if status, _ := rawMCP(t, http.MethodGet, a.internal.URL+internalMCPSessionsPath, nil, ""); status != http.StatusUnauthorized {
		t.Errorf("the internal sessions endpoint answered %d without the internal credential", status)
	}

	// Without a server key there is no internal credential, and nothing opens.
	a.h.setMCPCluster(&mcpInstanceView{shared: shared, id: a.id()}, "", errors.New("no server key"))
	if status, _ := rawMCP(t, http.MethodPost, url, map[string]string{"Authorization": client, internalAuthHeader: "Bearer "}, rawInitialize); status != http.StatusUnauthorized {
		t.Errorf("without a server key the internal endpoint answered %d", status)
	}
}

// The routing decision for every shape of session id, with a peer that
// records what it receives.
func TestTheMCPRouterForwardsOnlyToALiveOtherInstance(t *testing.T) {
	type seen struct {
		path, internal, auth, session, body string
	}
	received := make(chan seen, 1)
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- seen{r.URL.Path, r.Header.Get(internalAuthHeader), r.Header.Get("Authorization"), r.Header.Get(mcpSessionHeader), string(body)}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"from":"peer"}`))
	}))
	defer peer.Close()

	shared := &mcpInstances{live: map[string]string{"self": "http://unused", "peer": peer.URL}}
	h := &Handler{}
	var localInternal string
	local := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		localInternal = r.Header.Get(internalAuthHeader)
		_, _ = w.Write([]byte("local"))
	})
	serve := func(sessionID string) string {
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(rawPing))
		req.Header.Set("Authorization", "Bearer client")
		req.Header.Set(internalAuthHeader, "Bearer forged")
		if sessionID != "" {
			req.Header.Set(mcpSessionHeader, sessionID)
		}
		rec := httptest.NewRecorder()
		h.mcpRouter(local).ServeHTTP(rec, req)
		return rec.Body.String()
	}

	// Off: a single instance serves everything itself.
	if got := serve("peer.abc"); got != "local" {
		t.Errorf("without a cluster, a peer's session was %q", got)
	}
	h.setMCPCluster(&mcpInstanceView{shared: shared, id: "self"}, mcpClusterToken, nil)
	for _, id := range []string{"", "self.abc", "ABCDEF", "dead.abc"} {
		localInternal = "unset"
		if got := serve(id); got != "local" {
			t.Errorf("session %q was %q, want served here", id, got)
		}
		if localInternal != "" {
			t.Errorf("session %q reached the local handler with the forged internal header %q", id, localInternal)
		}
	}
	if got := serve("peer.abc"); !strings.Contains(got, "peer") {
		t.Fatalf("a peer's session was %q, want forwarded", got)
	}
	got := <-received
	if got.path != internalMCPPath || got.internal != "Bearer "+mcpClusterToken || got.auth != "Bearer client" ||
		got.session != "peer.abc" || got.body != rawPing {
		t.Errorf("the peer received %+v", got)
	}

	// Without a server key nothing is forwarded.
	h.setMCPCluster(&mcpInstanceView{shared: shared, id: "self"}, "", errors.New("no server key"))
	if got := serve("peer.abc"); got != "local" {
		t.Errorf("without a server key, a peer's session was %q", got)
	}
}

type sessionsAnswer struct {
	Sessions    []taskmcp.SessionView `json:"sessions"`
	Unreachable []string              `json:"unreachable"`
}

func readSessions(t *testing.T, n *mcpNode) sessionsAnswer {
	t.Helper()
	status, body := rawMCP(t, http.MethodGet, n.public.URL+"/api/mcp/sessions", nil, "")
	if status != http.StatusOK {
		t.Fatalf("sessions view answered %d %s", status, body)
	}
	var answer sessionsAnswer
	if err := json.Unmarshal([]byte(body), &answer); err != nil {
		t.Fatal(err)
	}
	return answer
}

// Either instance lists the sessions of both, and an instance that does not
// answer in time is named rather than failing the view.
func TestTheSessionsViewListsEveryLiveInstance(t *testing.T) {
	t.Setenv("SECTILE_SERVER_TOKEN", mcpClientKey)
	bound := mcpPeerSessionsTimeout
	mcpPeerSessionsTimeout = 300 * time.Millisecond
	t.Cleanup(func() { mcpPeerSessionsTimeout = bound })

	shared := &mcpInstances{live: map[string]string{}}
	a, b := newMCPNode(t, shared, nil), newMCPNode(t, shared, nil)
	onA := connectThrough(t, func(*http.Request) *httptest.Server { return a.public }, nil)
	defer onA.Close()
	onB := connectThrough(t, func(*http.Request) *httptest.Server { return b.public }, nil)
	defer onB.Close()
	callTool(t, onA, "list_projects", nil)
	callTool(t, onB, "list_projects", nil)

	for _, n := range []*mcpNode{a, b} {
		answer := readSessions(t, n)
		instances := map[string]string{}
		for _, s := range answer.Sessions {
			instances[s.ID] = s.Instance
		}
		if len(answer.Sessions) != 2 || instances[onA.ID()] != a.id() || instances[onB.ID()] != b.id() || len(answer.Unreachable) != 0 {
			t.Errorf("view from %s = %+v", n.id(), answer)
		}
	}

	release := make(chan struct{})
	hung := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer hung.Close()
	defer close(release)
	shared.set("hung", hung.URL)
	started := time.Now()
	answer := readSessions(t, b)
	if len(answer.Sessions) != 2 || len(answer.Unreachable) != 1 || answer.Unreachable[0] != "hung" {
		t.Errorf("view with an instance that does not answer = %+v", answer)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Errorf("the view waited %s for an instance that does not answer", elapsed)
	}
}

// A single instance answers as before, with an empty unreachable list.
func TestTheSessionsViewOfASingleInstanceIsLocal(t *testing.T) {
	t.Setenv("SECTILE_SERVER_TOKEN", mcpClientKey)
	alone := newMCPNode(t, nil, nil)
	session := connectThrough(t, func(*http.Request) *httptest.Server { return alone.public }, nil)
	defer session.Close()
	callTool(t, session, "list_projects", nil)
	answer := readSessions(t, alone)
	if len(answer.Sessions) != 1 || answer.Sessions[0].Instance != alone.id() || answer.Unreachable == nil || len(answer.Unreachable) != 0 {
		t.Errorf("single-instance view = %+v", answer)
	}
}
