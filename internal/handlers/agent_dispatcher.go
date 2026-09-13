package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"tasks/internal/agentprotocol"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type AgentMessage = agentprotocol.Message

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
	LastPingAt  time.Time
	mu          sync.Mutex
}

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
	operations map[string]*pendingOperation
	agents     map[agentKey]*AgentConn
	pending    map[string]*pendingAgentLaunch
	mu         sync.RWMutex
}

// NewAgentDispatcher creates a dispatcher ready to accept agent connections.
func NewAgentDispatcher() *AgentDispatcher {
	return &AgentDispatcher{
		agents:  make(map[agentKey]*AgentConn),
		pending: make(map[string]*pendingAgentLaunch),
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
		LastPingAt:  time.Now(),
	}
	d.agents[key] = ac

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
			LastPingAt:  ac.LastPingAt,
		})
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

// DispatchAndWait confirms a terminal launch before the HTTP caller reports
// success. Responses are bound to the connection that received the command.
func (d *AgentDispatcher) DispatchAndWait(ctx context.Context, userID, projectID, taskID string, payload any) error {
	ac := d.Lookup(userID, projectID)
	if ac == nil {
		return fmt.Errorf("no local agent connected")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	id := uuid.NewString()
	pending := &pendingAgentLaunch{agent: ac, result: make(chan agentLaunchStatus, 1)}
	d.mu.Lock()
	d.pending[id] = pending
	d.mu.Unlock()
	defer func() { d.mu.Lock(); delete(d.pending, id); d.mu.Unlock() }()
	if err := ac.Send(AgentMessage{MsgID: id, Type: "dispatch_step", TaskID: taskID, UserID: userID, Payload: raw}); err != nil {
		return err
	}
	select {
	case result := <-pending.result:
		if result.Status == "failed" {
			return fmt.Errorf("local terminal launch failed: %s", result.Summary)
		}
		return nil
	case <-ac.done:
		return fmt.Errorf("agent disconnected before confirming terminal launch; check the local agent before retrying")
	case <-ctx.Done():
		return fmt.Errorf("terminal launch was not confirmed before timeout or cancellation; check the local agent before retrying: %w", ctx.Err())
	}
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
