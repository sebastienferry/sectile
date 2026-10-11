package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"slices"
	"sort"
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
	ConnectionID       string
	inventory          agentprotocol.ProjectInventory
	inventoryUpdatedAt time.Time
	done               chan struct{}
	closeOnce          sync.Once
	UserID             string
	ProjectID          string
	DeviceID           string
	Conn               *websocket.Conn
	ConnectedAt        time.Time
	// Build is what the agent announced when it connected. It is set once at
	// registration and never changes; a rebind builds a new AgentConn.
	Build AgentBuild
	// lastSeen holds the Unix nanoseconds of the last frame received from the
	// agent. The read loop writes it while status handlers read it, so it is
	// atomic rather than guarded by mu, which Send may hold for seconds.
	lastSeen atomic.Int64
	mu       sync.Mutex
}

// AgentBuild is what an agent announced on its connection URL. An agent that
// predates the announcement leaves Announced false: it is still sent every
// operation, and its refusal of an unknown one is converted afterwards.
type AgentBuild struct {
	Announced  bool
	Version    string
	Commit     string
	Operations map[string]bool
}

// ParseAgentBuild reads the announcement parameters of an agent connection.
// The presence of operations, even empty, is what marks an announcing agent.
func ParseAgentBuild(query url.Values) AgentBuild {
	build := AgentBuild{Version: query.Get("agentVersion"), Commit: query.Get("agentCommit")}
	if !query.Has("operations") {
		return build
	}
	build.Announced = true
	build.Operations = map[string]bool{}
	for _, op := range strings.Split(query.Get("operations"), ",") {
		if op = strings.TrimSpace(op); op != "" {
			build.Operations[op] = true
		}
	}
	return build
}

// Supports reports whether the agent can run action. A legacy agent is given
// the benefit of the doubt, and so is an action this server does not list.
func (b AgentBuild) Supports(action string) bool {
	return !b.Announced || b.Operations[action] || !slices.Contains(agentprotocol.Operations, action)
}

// Outdated reports whether the agent lacks an operation this server may ask
// for. A legacy agent cannot say, and counts as outdated.
func (b AgentBuild) Outdated() bool {
	if !b.Announced {
		return true
	}
	for _, op := range agentprotocol.Operations {
		if !b.Operations[op] {
			return true
		}
	}
	return false
}

// Describe renders the build for an UnsupportedOperationError, "" when the
// agent announced none.
func (b AgentBuild) Describe() string {
	if !b.Announced {
		return ""
	}
	return agentprotocol.DescribeBuild(b.Version, b.Commit)
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
	UserID   string
	DeviceID string
}

// AgentDispatcher manages connected local agent daemons. It enforces a strict
// 1:1 mapping between (userID, deviceID) and an active WebSocket connection.
// When a user triggers a workflow action on the remote Web UI, the dispatcher
// routes the command to the matching local agent.
type AgentDispatcher struct {
	registrationMu sync.Mutex
	live           map[string]*AgentConn
	connections    map[*websocket.Conn]*AgentConn
	operations     map[string]*pendingOperation
	agents         map[agentKey]*AgentConn
	pending        map[string]*pendingAgentLaunch
	pendingPulls   map[string]*pendingTaskPull
	// lastConnected remembers when a slot last held an agent, so an absence
	// can be told apart from a project that never had one.
	lastConnected map[agentKey]time.Time
	lastProjects  map[agentKey]string
	runOwners     map[string]agentKey
	mu            sync.RWMutex
	// cluster forwards work for agents other server instances hold; nil when
	// this instance shares its store with nobody. See agent_cluster.go.
	cluster *agentCluster
}

// NewAgentDispatcher creates a dispatcher ready to accept agent connections.
func NewAgentDispatcher() *AgentDispatcher {
	return &AgentDispatcher{
		agents:        make(map[agentKey]*AgentConn),
		live:          make(map[string]*AgentConn),
		connections:   make(map[*websocket.Conn]*AgentConn),
		pending:       make(map[string]*pendingAgentLaunch),
		pendingPulls:  make(map[string]*pendingTaskPull),
		lastConnected: make(map[agentKey]time.Time),
		lastProjects:  make(map[agentKey]string),
		runOwners:     make(map[string]agentKey),
	}
}

// Register binds a local agent WebSocket to a (userID, deviceID) slot. If an
// existing connection occupies the slot, it is terminated with close code 4001
// (Session Rebound) and replaced by the new connection.
func (d *AgentDispatcher) Register(userID, projectID, deviceID string, conn *websocket.Conn) *AgentConn {
	return d.RegisterBuild(userID, projectID, deviceID, AgentBuild{}, conn)
}

// RegisterBuild is Register for an agent whose announced build is known.
func (d *AgentDispatcher) RegisterBuild(userID, projectID, deviceID string, build AgentBuild, conn *websocket.Conn) *AgentConn {
	d.registrationMu.Lock()
	defer d.registrationMu.Unlock()
	ac := d.registerLocal(userID, projectID, deviceID, build, conn)
	if err := d.cluster.agentConnected(ac); err != nil {
		log.Printf("[AgentDispatcher] Enregistrement de la session locale impossible : %v", err)
		ac.Close(websocket.CloseTryAgainLater, "Présence locale indisponible")
	}
	return ac
}

func (d *AgentDispatcher) registerLocal(userID, projectID, deviceID string, build AgentBuild, conn *websocket.Conn) *AgentConn {
	if deviceID == "" {
		deviceID = "unknown"
	}
	d.mu.Lock()
	key := agentKey{UserID: userID, DeviceID: deviceID}
	existing := d.agents[key]

	ac := &AgentConn{
		ConnectionID: uuid.NewString(),
		done:         make(chan struct{}),
		UserID:       userID,
		ProjectID:    projectID,
		DeviceID:     deviceID,
		Conn:         conn,
		ConnectedAt:  time.Now().UTC().Truncate(time.Microsecond),
		Build:        build,
	}
	ac.Touch()
	d.agents[key] = ac
	d.live[ac.ConnectionID] = ac
	d.connections[conn] = ac
	if existing != nil {
		delete(d.live, existing.ConnectionID)
	}
	if d.lastConnected == nil {
		d.lastConnected = make(map[agentKey]time.Time)
	}
	d.lastConnected[key] = time.Now()
	if d.lastProjects == nil {
		d.lastProjects = make(map[agentKey]string)
	}
	d.lastProjects[key] = projectID
	d.mu.Unlock()
	if existing != nil {
		existing.Close(4001, "Session Rebound")
	}

	log.Printf("[AgentDispatcher] Agent registered: user=%s project=%s device=%s", userID, projectID, deviceID)
	return ac
}

// Unregister removes an agent connection. It only removes the entry if the
// stored connection matches, to avoid racing with a rebind.
func (d *AgentDispatcher) Unregister(userID, deviceID string, conn *websocket.Conn) {
	d.mu.Lock()
	key := agentKey{UserID: userID, DeviceID: deviceID}
	existing := d.connections[conn]
	if existing != nil && existing.UserID == userID && existing.DeviceID == deviceID {
		delete(d.connections, conn)
		delete(d.live, existing.ConnectionID)
		existing.closeOnce.Do(func() { close(existing.done) })
		if d.agents[key] == existing {
			delete(d.agents, key)
		}
	} else {
		existing = nil
	}
	d.mu.Unlock()
	if existing != nil {
		d.cluster.sessionDisconnected(existing)
	}
}

// Lookup returns the connected agent for a given user and project, or nil if
// no agent is connected.
func (d *AgentDispatcher) Lookup(userID, projectID string, deviceID ...string) *AgentConn {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var selected *AgentConn
	for _, ac := range d.agents {
		if ac.UserID != userID || (firstDevice(deviceID) != "" && ac.DeviceID != firstDevice(deviceID)) {
			continue
		}
		if selected != nil {
			return nil
		}
		selected = ac
	}
	if selected != nil && projectEligible(selected.ProjectID, projectID) {
		return selected
	}
	return nil
}

// WaitForAgent returns the connected agent for a user and project, waiting up
// to grace for one to register. An agent that drops re-registers within
// seconds, and failing an operation during that window turns a hiccup into a
// broken stage transition. The wait also ends on the caller's context, so it
// never outlives the deadline the caller already chose.
func (d *AgentDispatcher) WaitForAgent(ctx context.Context, userID, projectID string, grace time.Duration) *AgentConn {
	route, err := d.waitForSelectedRoute(ctx, userID, projectID, "", grace)
	if err != nil {
		return nil
	}
	return route.local
}

// agentWasRecentlyConnected reports whether a slot held an agent recently
// enough for its absence to read as a reconnection. Register records the
// instant, and the lookup mirrors Lookup's fallbacks so a wildcard registration
// counts too.
func (d *AgentDispatcher) agentWasRecentlyConnected(userID, projectID string) bool {
	d.mu.RLock()
	for key, at := range d.lastConnected {
		if key.UserID == userID && projectEligible(d.lastProjects[key], projectID) && time.Since(at) < agentAbsenceIsRecent {
			d.mu.RUnlock()
			return true
		}
	}
	d.mu.RUnlock()
	// The agent may have been, or be reconnecting, on another instance.
	return d.cluster.recentlyConnected(userID, projectID)
}

// Dispatch sends a command message to the user's connected local agent. It
// returns an error if no agent is connected or the send fails.
func (d *AgentDispatcher) Dispatch(userID, projectID string, msgType string, taskID string, payload interface{}, deviceID ...string) error {
	route, err := d.SelectRoute(userID, projectID, firstDevice(deviceID))
	if err != nil {
		return err
	}
	return d.dispatchRoute(route, userID, projectID, msgType, taskID, payload)
}

func (d *AgentDispatcher) dispatchRoute(route *AgentRoute, userID, projectID, msgType, taskID string, payload any) error {
	if route.remote != nil {
		return d.cluster.dispatch(*route.remote, userID, projectID, msgType, taskID, payload)
	}
	return d.dispatchLocal(route.local, userID, msgType, taskID, payload)
}

// dispatchLocal sends a command to a connection held here.
func (d *AgentDispatcher) dispatchLocal(ac *AgentConn, userID, msgType, taskID string, payload interface{}) error {
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
		d.Unregister(ac.UserID, ac.DeviceID, ac.Conn)
		return fmt.Errorf("failed to send message to local agent: %w", err)
	}
	return nil
}

// ConnectedAgents returns a snapshot of all currently connected agents for
// status reporting (e.g. the Web UI connection indicator).
func (d *AgentDispatcher) ConnectedAgents() []AgentConnInfo {
	if d.cluster != nil {
		return d.clusterAgents()
	}
	d.mu.RLock()
	defer d.mu.RUnlock()

	out := make([]AgentConnInfo, 0, len(d.agents))
	for _, ac := range d.agents {
		info := AgentConnInfo{
			UserID:      ac.UserID,
			ProjectID:   ac.ProjectID,
			DeviceID:    ac.DeviceID,
			ConnectedAt: ac.ConnectedAt,
			LastPingAt:  ac.LastSeen(),
		}
		info.describeBuild(ac.Build)
		info.describeInventory(ac)
		out = append(out, info)
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
	ConnectionID        string     `json:"-"`
	InventoryState      string     `json:"inventoryState"`
	AvailableProjectIDs []string   `json:"availableProjectIds"`
	InventoryUpdatedAt  *time.Time `json:"inventoryUpdatedAt,omitempty"`
	UserID              string     `json:"userId"`
	ProjectID           string     `json:"projectId"`
	DeviceID            string     `json:"deviceId"`
	ConnectedAt         time.Time  `json:"connectedAt"`
	LastPingAt          time.Time  `json:"lastPingAt"`
	// InstanceID names the server instance holding the connection, when
	// several share the database.
	InstanceID string `json:"instanceId,omitempty"`
	// The build the agent announced, known only on the instance holding the
	// connection: an entry held elsewhere leaves them empty and Outdated nil.
	AgentVersion string   `json:"agentVersion,omitempty"`
	AgentCommit  string   `json:"agentCommit,omitempty"`
	Operations   []string `json:"operations,omitempty"`
	Outdated     *bool    `json:"outdated,omitempty"`
}

func (info *AgentConnInfo) describeBuild(build AgentBuild) {
	outdated := build.Outdated()
	info.Outdated = &outdated
	if !build.Announced {
		return
	}
	info.AgentVersion, info.AgentCommit = build.Version, build.Commit
	for op := range build.Operations {
		info.Operations = append(info.Operations, op)
	}
	sort.Strings(info.Operations)
}

// clusterAgents lists the agents of every live instance. A connection held
// here reports its last ping; one held elsewhere reports when it connected.
func (d *AgentDispatcher) clusterAgents() []AgentConnInfo {
	locations := d.cluster.locations()
	out := make([]AgentConnInfo, 0, len(locations))
	for _, l := range locations {
		info := AgentConnInfo{UserID: l.UserID, ProjectID: l.ProjectID, DeviceID: l.DeviceID, ConnectedAt: l.ConnectedAt, LastPingAt: l.ConnectedAt, InstanceID: l.InstanceID}
		info.ConnectionID, info.InventoryState, info.AvailableProjectIDs, info.InventoryUpdatedAt = l.ConnectionID, l.InventoryState, l.ProjectIDs, l.InventoryUpdatedAt
		if info.InventoryState == "" {
			info.InventoryState = "unknown"
		}
		if info.AvailableProjectIDs == nil {
			info.AvailableProjectIDs = []string{}
		}
		if !l.LastPingAt.IsZero() {
			info.LastPingAt = l.LastPingAt
		}
		d.mu.RLock()
		if ac, ok := d.agents[agentKey{UserID: l.UserID, DeviceID: l.DeviceID}]; ok && l.InstanceID == d.cluster.directory.InstanceID() && ac.ConnectedAt.Equal(l.ConnectedAt) {
			info.LastPingAt = ac.LastSeen()
			info.describeBuild(ac.Build)
			info.describeInventory(ac)
		}
		d.mu.RUnlock()
		out = append(out, info)
	}
	return out
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
var ErrLaunchUnconfirmed = agentprotocol.ErrLaunchUnconfirmed

// ErrRunNotOwned reports a non-owner acknowledgment. Cancellation promotes it
// to orphan evidence only after every candidate replies and no known owner
// is disconnected.
var ErrRunNotOwned = errors.New("agent does not own the run")

// ErrNoAgentConnected reports that nobody was there to ask.
var ErrNoAgentConnected = errors.New("no local agent connected")

// DispatchAndWait confirms a terminal launch before the HTTP caller reports
// success. Responses are bound to the connection that received the command.
func (d *AgentDispatcher) DispatchAndWait(ctx context.Context, userID, projectID, taskID string, payload any) error {
	dispatch, _ := payload.(agentconfig.Dispatch)
	if dispatch.Action == "cancel_run" {
		return d.CancelRun(ctx, userID, projectID, taskID, dispatch.RunID)
	}
	route, err := d.waitForSelectedRoute(ctx, userID, projectID, dispatch.DeviceID, defaultAgentReconnectGrace)
	if err != nil {
		return err
	}
	return d.dispatchAndWaitRoute(ctx, route, userID, projectID, taskID, payload)
}

func (d *AgentDispatcher) dispatchAndWaitRoute(ctx context.Context, route *AgentRoute, userID, projectID, taskID string, payload any) error {
	if route.taskLaunch {
		// Revalidate the same canonical session immediately before sending. A
		// displaced connection must not receive work after its replacement.
		valid := false
		for _, candidate := range d.taskRoutes(userID, projectID) {
			if candidate.DeviceID == route.DeviceID && candidate.local == route.local &&
				(candidate.remote == nil && route.remote == nil || candidate.remote != nil && route.remote != nil && candidate.remote.ConnectionID == route.remote.ConnectionID) {
				valid = true
			}
		}
		if !valid {
			return ErrNoAgentConnected
		}
	}
	dispatch, _ := payload.(agentconfig.Dispatch)
	if dispatch.RunID != "" && dispatch.Action != "cancel_run" {
		d.rememberRun(dispatch.RunID, userID, route.DeviceID)
	}
	var err error
	if route.remote != nil {
		err = d.cluster.dispatchAndWait(ctx, *route.remote, userID, projectID, taskID, payload, route.taskLaunch)
	} else {
		err = d.dispatchAndWaitLocal(ctx, route.local, userID, taskID, payload)
	}
	if err != nil && !errors.Is(err, ErrLaunchUnconfirmed) && dispatch.Action != "cancel_run" {
		d.ForgetRun(dispatch.RunID)
	}
	return err
}

// dispatchAndWaitLocal confirms a launch on a connection held here.
func (d *AgentDispatcher) dispatchAndWaitLocal(ctx context.Context, ac *AgentConn, userID, taskID string, payload any) error {
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
		return fmt.Errorf("%w: agent disconnected before confirming terminal launch of %s after %s; check the local agent (%s) before retrying",
			ErrLaunchUnconfirmed, action, waited(started), ac.DeviceID)
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
