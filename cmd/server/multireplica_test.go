package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"tasks/internal/db"
	"tasks/internal/taskmcp"
)

// The multi-replica harness (#410) runs two real server processes on one
// PostgreSQL database, behind a load balancer the test plays, with a scripted
// local agent, and kills one of them. Everything #397 delivered in slices is
// checked here end to end, across process boundaries, which the in-process
// two-instance tests cannot do: a replica sharing the test's process cannot be
// killed, its heartbeat keeps it alive.
//
// It needs SECTILE_TEST_POSTGRES_DSN and skips without it, like every other
// PostgreSQL test; the test:postgres CI job sets it. It builds the server once.

const (
	harnessServerToken = "harness-server-token"
	harnessSecretKey   = "202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f"
	harnessBranch      = "feat/410-harness"
	harnessDeadAfter   = 4 * time.Second
	harnessGrace       = 3 * time.Second
)

func TestPostgresMultiReplicaHarness(t *testing.T) {
	dsn := os.Getenv("SECTILE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set SECTILE_TEST_POSTGRES_DSN to run the multi-replica harness")
	}
	bin := buildServer(t)
	a := startReplica(t, bin, dsn, "A", 0, 0)
	b := startReplica(t, bin, dsn, "B", 0, 0)
	lb := newBalancer(t, a, b)

	client := signIn(t, a)
	project := createProject(t, client, a)

	agent := startScriptedAgent(t, lb)
	agent.waitConnections(t, 1)

	t.Run("an agent operation crosses replicas", func(t *testing.T) {
		// The agent landed on A, the balancer's first choice; the operation
		// is asked of B.
		eventually(t, 15*time.Second, func() error { return gitBranchesThrough(client, b, project) })
	})

	t.Run("a live update crosses replicas", func(t *testing.T) {
		events := openEventStream(t, client, b)
		task := createTask(t, client, a, project, "harness task")
		// The stream sends nothing, headers included, before its first event,
		// so there is no telling when B has subscribed: the change is made
		// again until one reaches it.
		eventually(t, 20*time.Second, func() error {
			postJSON(t, client, a.url+"/api/tasks/postback", map[string]any{"taskId": task, "title": "harness task, renamed"}, nil)
			if events.saw(time.Second, func(event, data string) bool {
				return event == "task_updated" && strings.Contains(data, task)
			}) {
				return nil
			}
			return fmt.Errorf("no task_updated for %s on B's stream", task)
		})
	})

	var lostSession string
	t.Run("an MCP session crosses replicas", func(t *testing.T) {
		route := &mcpRoute{target: a.url}
		session := connectMCP(t, route)
		defer session.Close()
		if owner := taskmcp.SessionOwner(session.ID()); owner != a.instanceID(t) {
			t.Fatalf("the session is owned by %q, want A (%s)", owner, a.instanceID(t))
		}
		route.set(b.url)
		if _, err := callWithin(session, "list_projects"); err != nil {
			t.Fatalf("list_projects through B on A's session: %v", err)
		}
		lostSession = session.ID()
	})

	t.Run("a killed replica loses nothing it cannot recover", func(t *testing.T) {
		a.kill(t)
		agent.waitConnections(t, 2)
		eventually(t, 15*time.Second, func() error { return gitBranchesThrough(client, b, project) })
		eventually(t, 15*time.Second, func() error { return gitBranchesThrough(client, &replica{url: lb.URL}, project) })

		// A's session is gone with A: once A is dead, B answers 404 and the
		// client opens a new session, on B, which works.
		eventually(t, 3*harnessDeadAfter, func() error {
			status := mcpStatusForSession(t, b, lostSession)
			if status != http.StatusNotFound {
				return fmt.Errorf("B answers %d for the lost session, want 404", status)
			}
			return nil
		})
		session := connectMCP(t, &mcpRoute{target: b.url})
		defer session.Close()
		if _, err := callWithin(session, "list_projects"); err != nil {
			t.Fatalf("a new session on B: %v", err)
		}
	})

	t.Run("a stopped replica drains", func(t *testing.T) {
		a2 := startReplica(t, bin, dsn, "A2", a.port, a.internalPort)
		lb.replace(a, a2)
		stopped := b.instanceID(t)
		connectionsBefore := agent.connections.Load()

		b.terminate(t)
		eventually(t, harnessGrace, func() error {
			if status := getStatus(b.url + "/api/ready"); status != http.StatusServiceUnavailable {
				return fmt.Errorf("B's readiness answers %d while draining, want 503", status)
			}
			return nil
		})
		if status := getStatus(b.url + "/api/health"); status != http.StatusOK {
			t.Fatalf("B's liveness answers %d while draining, want 200", status)
		}
		if code := b.waitExit(t, harnessGrace+15*time.Second); code != 0 {
			t.Fatalf("B exited with %d, want 0", code)
		}

		agent.waitConnections(t, connectionsBefore+1)
		eventually(t, 15*time.Second, func() error { return gitBranchesThrough(client, a2, project) })

		// B removed itself: A2 does not have to wait for the dead-after bound.
		store, err := db.Open(db.Config{Driver: db.DriverPostgres, DSN: dsn})
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close()
		for _, instance := range store.LiveInstances() {
			if instance.ID == stopped {
				t.Fatalf("the drained instance %s is still listed", stopped)
			}
		}
	})
}

// ── Replicas ──────────────────────────────────────────────────────────────────

func buildServer(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "sectile-server")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the server: %v\n%s", err, out)
	}
	return bin
}

type replica struct {
	name         string
	url          string
	port         int
	internalPort int
	cmd          *exec.Cmd
	log          *bytes.Buffer
	exited       chan int
	id           string
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

// startReplica starts one server process on the shared database and waits for
// it to be ready. Zero ports are picked free. Its environment is built from
// nothing, so nothing of the developer's own configuration or agent leaks in.
func startReplica(t *testing.T, bin, dsn, name string, port, internalPort int) *replica {
	t.Helper()
	if port == 0 {
		port = freePort(t)
	}
	if internalPort == 0 {
		internalPort = freePort(t)
	}
	home := t.TempDir()
	r := &replica{name: name, url: fmt.Sprintf("http://127.0.0.1:%d", port), port: port, internalPort: internalPort,
		log: &bytes.Buffer{}, exited: make(chan int, 1)}
	r.cmd = exec.Command(bin)
	r.cmd.Dir = home
	r.cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
		"XDG_CONFIG_HOME=" + filepath.Join(home, ".config"),
		"TMPDIR=" + home,
		"DB_DRIVER=postgres",
		"DATABASE_URL=" + dsn,
		fmt.Sprintf("PORT=%d", port),
		fmt.Sprintf("SECTILE_INTERNAL_PORT=%d", internalPort),
		fmt.Sprintf("SECTILE_INTERNAL_URL=http://127.0.0.1:%d", internalPort),
		"SECTILE_SECRET_KEY=" + harnessSecretKey,
		"SECTILE_SERVER_TOKEN=" + harnessServerToken,
		"SECTILE_INSTANCE_HEARTBEAT=500ms",
		"SECTILE_INSTANCE_DEAD_AFTER=" + harnessDeadAfter.String(),
		"SECTILE_INSTANCE_RECLAIM=500ms",
		"SECTILE_SHUTDOWN_GRACE=" + harnessGrace.String(),
	}
	output := &lockedWriter{buf: r.log}
	r.cmd.Stdout, r.cmd.Stderr = output, output
	if err := r.cmd.Start(); err != nil {
		t.Fatalf("starting replica %s: %v", name, err)
	}
	go func() {
		err := r.cmd.Wait()
		code := 0
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else if err != nil {
			code = -1
		}
		r.exited <- code
	}()
	t.Cleanup(func() {
		_ = r.cmd.Process.Kill()
		if t.Failed() {
			t.Logf("── replica %s ──\n%s", name, output.String())
		}
	})
	eventually(t, 60*time.Second, func() error {
		select {
		case code := <-r.exited:
			r.exited <- code
			t.Fatalf("replica %s exited with %d before being ready:\n%s", name, code, output.String())
		default:
		}
		if status := getStatus(r.url + "/api/ready"); status != http.StatusOK {
			return fmt.Errorf("replica %s readiness answers %d", name, status)
		}
		return nil
	})
	return r
}

func (r *replica) instanceID(t *testing.T) string {
	t.Helper()
	if r.id != "" {
		return r.id
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Get(r.url + "/api/ready")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body struct {
		Instance string `json:"instance"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.Instance == "" {
		t.Fatalf("replica %s names no instance: %v", r.name, err)
	}
	r.id = body.Instance
	return r.id
}

// kill ends the process at once, as a node lost with its machine: no drain,
// no row removed, nothing flushed.
func (r *replica) kill(t *testing.T) {
	t.Helper()
	r.instanceID(t)
	if err := r.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	r.waitExit(t, 10*time.Second)
}

// terminate asks the process to stop, as an orchestrator does on a deploy.
func (r *replica) terminate(t *testing.T) {
	t.Helper()
	if err := r.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
}

func (r *replica) waitExit(t *testing.T, within time.Duration) int {
	t.Helper()
	select {
	case code := <-r.exited:
		r.exited <- code
		return code
	case <-time.After(within):
		t.Fatalf("replica %s did not exit within %s", r.name, within)
		return -1
	}
}

type lockedWriter struct {
	mu  sync.Mutex
	buf *bytes.Buffer
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *lockedWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// ── Load balancer ─────────────────────────────────────────────────────────────

// balancer sends each request, WebSocket upgrades included, to the first
// replica of its list whose readiness probe answers 200, which is what a load
// balancer honouring readiness does, and keeps placement predictable.
type balancer struct {
	*httptest.Server
	mu       sync.Mutex
	replicas []*replica
}

func newBalancer(t *testing.T, replicas ...*replica) *balancer {
	t.Helper()
	lb := &balancer{replicas: replicas}
	lb.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := lb.pick()
		if target == nil {
			http.Error(w, "no replica ready", http.StatusServiceUnavailable)
			return
		}
		to, _ := url.Parse(target.url)
		httputil.NewSingleHostReverseProxy(to).ServeHTTP(w, r)
	}))
	t.Cleanup(lb.Close)
	return lb
}

func (lb *balancer) pick() *replica {
	lb.mu.Lock()
	replicas := append([]*replica(nil), lb.replicas...)
	lb.mu.Unlock()
	for _, r := range replicas {
		if getStatusWithin(r.url+"/api/ready", 500*time.Millisecond) == http.StatusOK {
			return r
		}
	}
	return nil
}

func (lb *balancer) replace(old, replacement *replica) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	for i, r := range lb.replicas {
		if r == old {
			lb.replicas[i] = replacement
		}
	}
}

// ── Scripted agent ────────────────────────────────────────────────────────────

// scriptedAgent speaks just enough of the agent protocol: it answers the
// running-tasks pull and the git_branches operation, and reconnects through
// the balancer whenever its connection drops, as the real agent does.
type scriptedAgent struct {
	connections atomic.Int64
	stop        chan struct{}
}

func startScriptedAgent(t *testing.T, lb *balancer) *scriptedAgent {
	t.Helper()
	agent := &scriptedAgent{stop: make(chan struct{})}
	endpoint := strings.Replace(lb.URL, "http://", "ws://", 1) + "/ws/agent-connect?projectId=default&deviceId=harness"
	header := http.Header{"Authorization": []string{"Bearer " + harnessServerToken}}
	go func() {
		for {
			select {
			case <-agent.stop:
				return
			default:
			}
			conn, _, err := websocket.DefaultDialer.Dial(endpoint, header)
			if err != nil {
				time.Sleep(200 * time.Millisecond)
				continue
			}
			agent.connections.Add(1)
			agent.serve(conn)
			time.Sleep(200 * time.Millisecond)
		}
	}()
	t.Cleanup(func() { close(agent.stop) })
	return agent
}

type agentMessage struct {
	MsgID   string          `json:"msgId,omitempty"`
	Type    string          `json:"type"`
	TaskID  string          `json:"taskId,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

func (a *scriptedAgent) serve(conn *websocket.Conn) {
	defer conn.Close()
	for {
		var msg agentMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		var reply *agentMessage
		switch msg.Type {
		case "pull_tasks":
			reply = &agentMessage{MsgID: msg.MsgID, Type: "running_tasks", Payload: json.RawMessage(`[]`)}
		case "workspace_request":
			var op struct {
				Action string `json:"action"`
			}
			_ = json.Unmarshal(msg.Payload, &op)
			result := fmt.Sprintf(`{"error":"the harness agent does not do %s"}`, op.Action)
			if op.Action == "git_branches" {
				result = fmt.Sprintf(`{"value":{"repoPath":"/harness","currentBranch":%q,"branches":[]}}`, harnessBranch)
			}
			reply = &agentMessage{MsgID: msg.MsgID, Type: "workspace_result", Payload: json.RawMessage(result)}
		}
		if reply != nil {
			if err := conn.WriteJSON(reply); err != nil {
				return
			}
		}
	}
}

func (a *scriptedAgent) waitConnections(t *testing.T, want int64) {
	t.Helper()
	eventually(t, 20*time.Second, func() error {
		if got := a.connections.Load(); got < want {
			return fmt.Errorf("the agent connected %d time(s), want %d", got, want)
		}
		return nil
	})
}

// ── REST, SSE and MCP clients ─────────────────────────────────────────────────

// signIn opens a web session. Sessions live in the database, so the cookie
// works on every replica.
func signIn(t *testing.T, r *replica) *http.Client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 20 * time.Second}
	postJSON(t, client, r.url+"/auth/local", map[string]any{"email": "harness@example.com"}, nil)
	return client
}

func createProject(t *testing.T, client *http.Client, r *replica) string {
	t.Helper()
	var project struct {
		ID string `json:"id"`
	}
	postJSON(t, client, r.url+"/api/projects", map[string]any{
		"name": fmt.Sprintf("Harness %d", time.Now().UnixNano()), "repoPath": "/not-mounted", "issueTracker": "local", "useWorktrees": false,
	}, &project)
	if project.ID == "" {
		t.Fatal("the project has no id")
	}
	return project.ID
}

func createTask(t *testing.T, client *http.Client, r *replica, project, title string) string {
	t.Helper()
	var task struct {
		ID string `json:"id"`
	}
	postJSON(t, client, r.url+"/api/tasks", map[string]any{"projectId": project, "title": title}, &task)
	if task.ID == "" {
		t.Fatal("the task has no id")
	}
	return task.ID
}

func postJSON(t *testing.T, client *http.Client, target string, body any, out any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	resp, err := client.Post(target, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("POST %s: %v", target, err)
	}
	defer resp.Body.Close()
	answer, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		t.Fatalf("POST %s: %s %s", target, resp.Status, answer)
	}
	if out != nil {
		if err := json.Unmarshal(answer, out); err != nil {
			t.Fatalf("POST %s: unreadable answer %s: %v", target, answer, err)
		}
	}
}

func gitBranchesThrough(client *http.Client, r *replica, project string) error {
	resp, err := client.Get(r.url + "/api/git/branches?projectId=" + url.QueryEscape(project))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	answer, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("git branches through %s: %s %s", r.url, resp.Status, answer)
	}
	if !strings.Contains(string(answer), harnessBranch) {
		return fmt.Errorf("git branches through %s did not come from the agent: %s", r.url, answer)
	}
	return nil
}

type eventStream struct {
	events chan [2]string
}

func openEventStream(t *testing.T, client *http.Client, r *replica) *eventStream {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, r.url+"/api/events", nil)
	streaming := &http.Client{Jar: client.Jar}
	stream := &eventStream{events: make(chan [2]string, 64)}
	go func() {
		resp, err := streaming.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			stream.events <- [2]string{"status", resp.Status}
			return
		}
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		var event string
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				select {
				case stream.events <- [2]string{event, strings.TrimPrefix(line, "data: ")}:
				default:
				}
			}
		}
	}()
	return stream
}

// saw reports whether a matching event arrives within the bound.
func (s *eventStream) saw(within time.Duration, match func(event, data string) bool) bool {
	deadline := time.After(within)
	for {
		select {
		case got := <-s.events:
			if match(got[0], got[1]) {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

// mcpRoute sends MCP requests to a replica the test can change, so one session
// can be initialized on A and used through B.
type mcpRoute struct {
	mu     sync.Mutex
	target string
}

func (m *mcpRoute) set(target string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.target = target
}

func (m *mcpRoute) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	to, _ := url.Parse(m.target)
	m.mu.Unlock()
	out := req.Clone(req.Context())
	out.URL.Scheme, out.URL.Host, out.Host = to.Scheme, to.Host, to.Host
	out.Header.Set("Authorization", "Bearer "+harnessServerToken)
	return http.DefaultTransport.RoundTrip(out)
}

func connectMCP(t *testing.T, route *mcpRoute) *mcp.ClientSession {
	t.Helper()
	route.mu.Lock()
	endpoint := route.target + "/mcp"
	route.mu.Unlock()
	client := mcp.NewClient(&mcp.Implementation{Name: "multi-replica-harness", Version: "1"}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:   endpoint,
		HTTPClient: &http.Client{Transport: route},
	}, nil)
	if err != nil {
		t.Fatalf("initializing an MCP session on %s: %v", endpoint, err)
	}
	return session
}

func callWithin(session *mcp.ClientSession, tool string) (*mcp.CallToolResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return session.CallTool(ctx, &mcp.CallToolParams{Name: tool})
}

// mcpStatusForSession is the status a replica answers for a request on a given
// session, read raw: a client library would hide the 404 behind its own error.
func mcpStatusForSession(t *testing.T, r *replica, sessionID string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, r.url+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":99,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer "+harnessServerToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Session-Id", sessionID)
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return 0
	}
	resp.Body.Close()
	return resp.StatusCode
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func getStatus(target string) int { return getStatusWithin(target, 2*time.Second) }

func getStatusWithin(target string, within time.Duration) int {
	client := &http.Client{Timeout: within}
	resp, err := client.Get(target)
	if err != nil {
		return 0
	}
	resp.Body.Close()
	return resp.StatusCode
}

// eventually retries check until it succeeds or the bound passes.
func eventually(t *testing.T, within time.Duration, check func() error) {
	t.Helper()
	deadline := time.Now().Add(within)
	for {
		err := check()
		if err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("not within %s: %v", within, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
