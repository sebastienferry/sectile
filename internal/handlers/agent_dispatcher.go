package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type AgentMessage = agentprotocol.Message

const (
	// defaultAgentPingInterval is how often the server pings a connected agent.
	defaultAgentPingInterval = 10 * time.Second
	// defaultAgentReadTimeout drops a connection that has produced neither a
	// message nor a pong within this window. It must leave room for several
	// missed pings so a briefly stalled network does not unregister a healthy
	// agent. At one server ping every 10s and one agent heartbeat every 10s
	// (agentHeartbeatInterval in internal/agent), four consecutive keepalives have
	// to be lost before this fires. It is deliberately not equal to the agent
	// heartbeat period: when the two matched at 30s, a heartbeat arrived
	// exactly on the deadline and whether the connection survived came down to
	// scheduling order.
	defaultAgentReadTimeout = 45 * time.Second
	// agentCloseKeepaliveTimeout is the close code sent to an agent dropped for
	// silence. It sits next to 4001 ("Session Rebound") in the private range and
	// lets the agent log say which of the two happened.
	agentCloseKeepaliveTimeout = 4002
	// defaultAgentReconnectGrace is how long an operation waits for an agent to
	// come back before giving up. It is short by design: it has to fit inside
	// the 15s budget the caller gives a local inspection and still leave the
	// operation itself room to run, so it covers a reconnection hiccup and not
	// an agent that is simply not running.
	defaultAgentReconnectGrace = 4 * time.Second
	// agentAbsenceIsRecent bounds how long after a disconnection an absent
	// agent is still treated as reconnecting. Past it, waiting would only slow
	// down a server that has no agent at all, which is a legitimate setup and
	// deserves its immediate answer.
	agentAbsenceIsRecent = 5 * time.Minute
)

// AgentConn represents a single connected local agent daemon. The connection
// belongs to the user who authenticated and covers one project workspace.
type AgentConn struct {
	done        chan struct{}
	closeOnce   sync.Once
	UserID      string
	ProjectID   string
	DeviceID    string
	Conn        *websocket.Conn
	ConnectedAt time.Time
	// lastSeen holds the Unix nanoseconds of the last frame received from the
	// agent. The read loop writes it while status handlers read it, so it is
	// atomic rather than guarded by mu, which Send may hold for seconds.
	lastSeen atomic.Int64
	mu       sync.Mutex
}

// Touch records that a frame just arrived from the agent.
func (ac *AgentConn) Touch() { ac.lastSeen.Store(time.Now().UnixNano()) }

// LastSeen reports when the agent last produced a message or a pong.
func (ac *AgentConn) LastSeen() time.Time { return time.Unix(0, ac.lastSeen.Load()) }

// Send writes a JSON message to the agent WebSocket. It serialises access to
// the underlying connection so multiple goroutines can call it safely.
func (ac *AgentConn) Send(msg AgentMessage) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if err := ac.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	return ac.Conn.WriteJSON(msg)
}

// Close terminates the WebSocket connection with an optional close code.
func (ac *AgentConn) Close(code int, reason string) {
	ac.closeOnce.Do(func() {
		if ac.done != nil {
			close(ac.done)
		}
	})
	ac.mu.Lock()
	defer ac.mu.Unlock()
	_ = ac.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	closeMsg := websocket.FormatCloseMessage(code, reason)
	_ = ac.Conn.WriteMessage(websocket.CloseMessage, closeMsg)
	_ = ac.Conn.Close()
}

// Keepalive pings the agent at a fixed interval until the connection ends. A
// WebSocket that dies silently — a sleeping laptop, a dropped VPN, an expired
// NAT binding — produces neither an error nor a close frame, so without this
// the server keeps the slot registered and every operation pays its full
// timeout before failing. Pings force the peer to answer, and a write failure
// or a missed pong closes the connection, which unblocks the read loop and
// unregisters the agent. Ping frames are written with WriteControl, which is
// safe to call concurrently with Send and so never waits on mu.
func (ac *AgentConn) Keepalive(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ac.done:
			return
		case <-ticker.C:
			if err := ac.Conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(interval)); err != nil {
				log.Printf("[AgentDispatcher] Keepalive ping failed for device=%s: %v", ac.DeviceID, err)
				_ = ac.Conn.Close()
				return
			}
		}
	}
}

// agentKey uniquely identifies a connected agent slot.
type agentKey struct {
	UserID    string
	ProjectID string
}

// AgentDispatcher manages connected local agent daemons. It enforces a strict
// 1:1 mapping between (userID, projectID) and an active WebSocket connection.
// When a user triggers a workflow action on the remote Web UI, the dispatcher
// routes the command to the matching local agent.
type AgentDispatcher struct {
	operations   map[string]*pendingOperation
	agents       map[agentKey]*AgentConn
	pending      map[string]*pendingAgentLaunch
	pendingPulls map[string]*pendingTaskPull
	// lastConnected remembers when a slot last held an agent, so an absence
	// can be told apart from a project that never had one.
	lastConnected map[agentKey]time.Time
	mu            sync.RWMutex
}

// NewAgentDispatcher creates a dispatcher ready to accept agent connections.
func NewAgentDispatcher() *AgentDispatcher {
	return &AgentDispatcher{
		agents:        make(map[agentKey]*AgentConn),
		pending:       make(map[string]*pendingAgentLaunch),
		pendingPulls:  make(map[string]*pendingTaskPull),
		lastConnected: make(map[agentKey]time.Time),
	}
}

// Register binds a local agent WebSocket to a (userID, projectID) slot. If an
// existing connection occupies the slot, it is terminated with close code 4001
// (Session Rebound) and replaced by the new connection.
func (d *AgentDispatcher) Register(userID, projectID, deviceID string, conn *websocket.Conn) *AgentConn {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := agentKey{UserID: userID, ProjectID: projectID}

	if existing, ok := d.agents[key]; ok {
		log.Printf("[AgentDispatcher] Rebinding agent for user=%s project=%s (old device=%s, new device=%s)",
			userID, projectID, existing.DeviceID, deviceID)
		existing.Close(4001, "Session Rebound")
	}

	ac := &AgentConn{
		done:        make(chan struct{}),
		UserID:      userID,
		ProjectID:   projectID,
		DeviceID:    deviceID,
		Conn:        conn,
		ConnectedAt: time.Now(),
	}
	ac.Touch()
	d.agents[key] = ac
	if d.lastConnected == nil {
		d.lastConnected = make(map[agentKey]time.Time)
	}
	d.lastConnected[key] = time.Now()

	log.Printf("[AgentDispatcher] Agent registered: user=%s project=%s device=%s", userID, projectID, deviceID)
	return ac
}

// Unregister removes an agent connection. It only removes the entry if the
// stored connection matches, to avoid racing with a rebind.
func (d *AgentDispatcher) Unregister(userID, projectID string, conn *websocket.Conn) {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := agentKey{UserID: userID, ProjectID: projectID}
	if existing, ok := d.agents[key]; ok && existing.Conn == conn {
		existing.closeOnce.Do(func() { close(existing.done) })
		delete(d.agents, key)
		log.Printf("[AgentDispatcher] Agent unregistered: user=%s project=%s", userID, projectID)
	}
}

// Lookup returns the connected agent for a given user and project, or nil if
// no agent is connected.
func (d *AgentDispatcher) Lookup(userID, projectID string) *AgentConn {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// 1. Exact match
	if ac, ok := d.agents[agentKey{UserID: userID, ProjectID: projectID}]; ok {
		return ac
	}

	// 2. Wildcard / default project registered by the agent
	if ac, ok := d.agents[agentKey{UserID: userID, ProjectID: "default"}]; ok {
		return ac
	}
	if ac, ok := d.agents[agentKey{UserID: userID, ProjectID: "all"}]; ok {
		return ac
	}
	if ac, ok := d.agents[agentKey{UserID: userID, ProjectID: ""}]; ok {
		return ac
	}

	return nil
}

// WaitForAgent returns the connected agent for a user and project, waiting up
// to grace for one to register. An agent that drops re-registers within
// seconds, and failing an operation during that window turns a hiccup into a
// broken stage transition. The wait also ends on the caller's context, so it
// never outlives the deadline the caller already chose.
func (d *AgentDispatcher) WaitForAgent(ctx context.Context, userID, projectID string, grace time.Duration) *AgentConn {
	if ac := d.Lookup(userID, projectID); ac != nil {
		return ac
	}
	if !d.agentWasRecentlyConnected(userID, projectID) {
		// Nothing has ever answered here, or not for a long time. Waiting
		// would make a server with no agent slow instead of clear.
		return nil
	}
	deadline := time.NewTimer(grace)
	defer deadline.Stop()
	// Polling is enough for a wait measured in seconds and keeps the registry
	// free of per-waiter subscriptions.
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-deadline.C:
			return nil
		case <-ticker.C:
			if ac := d.Lookup(userID, projectID); ac != nil {
				return ac
			}
		}
	}
}

// agentWasRecentlyConnected reports whether a slot held an agent recently
// enough for its absence to read as a reconnection. Register records the
// instant, and the lookup mirrors Lookup's fallbacks so a wildcard registration
// counts too.
func (d *AgentDispatcher) agentWasRecentlyConnected(userID, projectID string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, key := range []agentKey{
		{UserID: userID, ProjectID: projectID},
		{UserID: userID, ProjectID: "default"},
		{UserID: userID, ProjectID: "all"},
		{UserID: userID, ProjectID: ""},
	} {
		if at, ok := d.lastConnected[key]; ok && time.Since(at) < agentAbsenceIsRecent {
			return true
		}
	}
	return false
}

// Dispatch sends a command message to the user's connected local agent. It
// returns an error if no agent is connected or the send fails.
func (d *AgentDispatcher) Dispatch(userID, projectID string, msgType string, taskID string, payload interface{}) error {
	ac := d.Lookup(userID, projectID)
	if ac == nil {
		return fmt.Errorf("no local agent connected for user %s on project %s", userID, projectID)
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal dispatch payload: %w", err)
	}

	msg := AgentMessage{
		MsgID:   uuid.New().String(),
		Type:    msgType,
		TaskID:  taskID,
		UserID:  userID,
		Payload: raw,
	}

	if err := ac.Send(msg); err != nil {
		// Connection broken: clean up and report.
		d.Unregister(ac.UserID, ac.ProjectID, ac.Conn)
		return fmt.Errorf("failed to send message to local agent: %w", err)
	}
	return nil
}

// ConnectedAgents returns a snapshot of all currently connected agents for
// status reporting (e.g. the Web UI connection indicator).
func (d *AgentDispatcher) ConnectedAgents() []AgentConnInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()

	out := make([]AgentConnInfo, 0, len(d.agents))
	for _, ac := range d.agents {
		out = append(out, AgentConnInfo{
			UserID:      ac.UserID,
			ProjectID:   ac.ProjectID,
			DeviceID:    ac.DeviceID,
			ConnectedAt: ac.ConnectedAt,
			LastPingAt:  ac.LastSeen(),
		})
	}
	return out
}

// ActiveConnections returns a snapshot of active agent connections.
func (d *AgentDispatcher) ActiveConnections() []*AgentConn {
	d.mu.RLock()
	defer d.mu.RUnlock()

	out := make([]*AgentConn, 0, len(d.agents))
	for _, ac := range d.agents {
		out = append(out, ac)
	}
	return out
}

// AgentConnInfo is a read-only snapshot of an agent connection for API responses.

type AgentConnInfo struct {
	UserID      string    `json:"userId"`
	ProjectID   string    `json:"projectId"`
	DeviceID    string    `json:"deviceId"`
	ConnectedAt time.Time `json:"connectedAt"`
	LastPingAt  time.Time `json:"lastPingAt"`
}

type agentLaunchStatus struct {
	Status  string `json:"status"`
	Summary string `json:"summary"`
}
type pendingAgentLaunch struct {
	agent  *AgentConn
	result chan agentLaunchStatus
}

// ErrLaunchUnconfirmed reports a launch whose confirmation never arrived. The
// agent may well be running the skill, so the caller must not record the run as
// failed: only the agent knows how it ends.
var ErrLaunchUnconfirmed = errors.New("launch not confirmed")

// ErrRunNotOwned reports an agent that answered it does not have the run it
// was asked to cancel. Unlike every other failure here it is good news: the
// agent is reachable and states it is running nothing, so the recorded run is
// an orphan the server can close.
var ErrRunNotOwned = errors.New("agent does not own the run")

// ErrNoAgentConnected reports that nobody was there to ask.
var ErrNoAgentConnected = errors.New("no local agent connected")

// DispatchAndWait confirms a terminal launch before the HTTP caller reports
// success. Responses are bound to the connection that received the command.
func (d *AgentDispatcher) DispatchAndWait(ctx context.Context, userID, projectID, taskID string, payload any) error {
	ac := d.Lookup(userID, projectID)
	if ac == nil {
		return ErrNoAgentConnected
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	action := launchAction(payload)
	id := uuid.NewString()
	pending := &pendingAgentLaunch{agent: ac, result: make(chan agentLaunchStatus, 1)}
	d.mu.Lock()
	d.pending[id] = pending
	d.mu.Unlock()
	defer func() { d.mu.Lock(); delete(d.pending, id); d.mu.Unlock() }()
	if err := ac.Send(AgentMessage{MsgID: id, Type: "dispatch_step", TaskID: taskID, UserID: userID, Payload: raw}); err != nil {
		return err
	}
	started := time.Now()
	select {
	case result := <-pending.result:
		if result.Status == "failed" {
			if strings.HasPrefix(result.Summary, agentprotocol.RunNotOwned) {
				return fmt.Errorf("%w: %s", ErrRunNotOwned, result.Summary)
			}
			return fmt.Errorf("local terminal launch failed: %s", result.Summary)
		}
		return nil
	case <-ac.done:
		return fmt.Errorf("agent disconnected before confirming terminal launch of %s after %s; check the local agent (%s) before retrying",
			action, waited(started), ac.DeviceID)
	case <-ctx.Done():
		log.Printf("[AgentDispatcher] Launch of %s on device=%s not confirmed after %s: %v", action, ac.DeviceID, waited(started), ctx.Err())
		return fmt.Errorf("%w: %s was not confirmed after %s; the local agent (%s) may still be running it, so its run stays open: %w",
			ErrLaunchUnconfirmed, action, waited(started), ac.DeviceID, ctx.Err())
	}
}

// waited reports the time actually spent waiting, so a caller that cancelled
// early is distinguishable from one that ran its deadline out.
func waited(since time.Time) time.Duration { return time.Since(since).Round(time.Millisecond) }

// launchAction names the dispatch for a reader of the error, falling back to a
// generic label rather than failing over the diagnostics.
func launchAction(payload any) string {
	dispatch, ok := payload.(agentconfig.Dispatch)
	if !ok {
		return "terminal launch"
	}
	for _, name := range []string{dispatch.SkillID, dispatch.Action} {
		if strings.TrimSpace(name) != "" {
			return "skill " + name
		}
	}
	return "terminal launch"
}

func (d *AgentDispatcher) ReportLaunchStatus(ac *AgentConn, msg AgentMessage) {
	var status agentLaunchStatus
	if json.Unmarshal(msg.Payload, &status) != nil || (status.Status != "completed" && status.Status != "failed") {
		return
	}
	d.mu.RLock()
	pending := d.pending[msg.MsgID]
	d.mu.RUnlock()
	if pending == nil || pending.agent != ac {
		return
	}
	select {
	case pending.result <- status:
	default:
	}
}

type pendingTaskPull struct {
	agent  *AgentConn
	result chan []agentprotocol.RunningTask
}

// PullTasks requests the connected agent to report its active and queued tasks.
func (d *AgentDispatcher) PullTasks(ctx context.Context, ac *AgentConn) ([]agentprotocol.RunningTask, error) {
	if ac == nil {
		return nil, fmt.Errorf("no agent connection provided")
	}
	id := uuid.NewString()
	pending := &pendingTaskPull{agent: ac, result: make(chan []agentprotocol.RunningTask, 1)}
	d.mu.Lock()
	if d.pendingPulls == nil {
		d.pendingPulls = make(map[string]*pendingTaskPull)
	}
	d.pendingPulls[id] = pending
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		delete(d.pendingPulls, id)
		d.mu.Unlock()
	}()

	msg := AgentMessage{
		MsgID:  id,
		Type:   "pull_tasks",
		UserID: ac.UserID,
	}
	if err := ac.Send(msg); err != nil {
		return nil, err
	}

	select {
	case tasks := <-pending.result:
		return tasks, nil
	case <-ac.done:
		return nil, fmt.Errorf("agent disconnected before returning running tasks")
	case <-ctx.Done():
		return nil, fmt.Errorf("timed out waiting for agent running tasks: %w", ctx.Err())
	}
}

// ReportRunningTasks handles incoming "running_tasks" messages from an agent.
func (d *AgentDispatcher) ReportRunningTasks(ac *AgentConn, msg AgentMessage) {
	var tasks []agentprotocol.RunningTask
	if err := json.Unmarshal(msg.Payload, &tasks); err != nil {
		log.Printf("[AgentDispatcher] Failed to unmarshal running_tasks payload: %v", err)
		return
	}
	d.mu.RLock()
	pending := d.pendingPulls[msg.MsgID]
	d.mu.RUnlock()
	if pending == nil || pending.agent != ac {
		return
	}
	select {
	case pending.result <- tasks:
	default:
	}
}
