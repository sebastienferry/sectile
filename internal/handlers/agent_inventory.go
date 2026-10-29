package handlers

import (
	"cmp"
	"errors"
	"log"
	"slices"
	"sync"
	"time"

	"tasks/internal/agentprotocol"
	"tasks/internal/db"
)

// WorkstationDirectory is the live session directory used by upgraded replicas.
type WorkstationDirectory interface {
	WorkstationConnected(db.AgentLocation) (db.AgentLocation, bool, error)
	WorkstationDisconnected(string) error
	WorkstationInventory(string, []string, time.Time) error
	WorkstationPing(string, time.Time) error
	ConnectedWorkstations() []db.AgentLocation
}

func (info *AgentConnInfo) describeInventory(ac *AgentConn) {
	info.ConnectionID = ac.ConnectionID
	info.InventoryState = "unknown"
	info.AvailableProjectIDs = []string{}
	if !ac.inventoryUpdatedAt.IsZero() {
		info.InventoryState = "reported"
		info.AvailableProjectIDs = slices.Clone(ac.inventory.ProjectIDs)
		updated := ac.inventoryUpdatedAt
		info.InventoryUpdatedAt = &updated
	}
}

// acceptInventory records what a connection says it can serve. The directory
// write happens outside the dispatcher lock: a slow database used to hold that
// lock, and with it every routing decision for every agent of the instance.
// The session is checked again before the snapshot is published, so one that
// was replaced meanwhile keeps nothing.
func (d *AgentDispatcher) acceptInventory(ac *AgentConn, p agentprotocol.ProjectInventory) error {
	d.mu.Lock()
	live := d.live[ac.ConnectionID] == ac
	cluster := d.cluster
	d.mu.Unlock()
	if !live {
		return errors.New("inactive agent session")
	}
	now := time.Now().UTC()
	if cluster != nil {
		if directory, ok := cluster.directory.(WorkstationDirectory); ok {
			if err := directory.WorkstationInventory(ac.ConnectionID, p.ProjectIDs, now); err != nil {
				return err
			}
		} else {
			return errors.New("workstation directory unavailable")
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.live[ac.ConnectionID] != ac {
		return errors.New("inactive agent session")
	}
	ac.inventory = p
	ac.inventoryUpdatedAt = now
	return nil
}

type AvailableWorkstation struct {
	Label       string    `json:"label"`
	DeviceID    string    `json:"deviceId"`
	ConnectedAt time.Time `json:"connectedAt"`
	LastPingAt  time.Time `json:"lastPingAt"`
}

func workstations(agents []AgentConnInfo, userID, projectID string) []AvailableWorkstation {
	devices := map[string]AvailableWorkstation{}
	for _, a := range agents {
		if a.UserID != userID || a.InventoryState != "reported" || !slices.Contains(a.AvailableProjectIDs, projectID) {
			continue
		}
		w, exists := devices[a.DeviceID]
		if !exists {
			w = AvailableWorkstation{DeviceID: a.DeviceID, ConnectedAt: a.ConnectedAt, LastPingAt: a.LastPingAt}
		}
		devices[a.DeviceID] = w
	}

	out := make([]AvailableWorkstation, 0, len(devices))
	for _, w := range devices {
		out = append(out, w)
	}
	slices.SortFunc(out, func(a, b AvailableWorkstation) int { return cmp.Compare(a.DeviceID, b.DeviceID) })
	return out
}

func (d *AgentDispatcher) AvailableWorkstations(userID, projectID string) []AvailableWorkstation {
	return workstations(d.ConnectedAgents(), userID, projectID)
}

// inventoryProjects uses the same shared-project visibility as agent discovery.
func (h *Handler) inventoryProjects(userID string) (map[string]bool, error) {
	caller := h.principalFor(userID)
	if caller.Anonymous() || caller.Blocked {
		return nil, errors.New("project access refused")
	}
	projects, err := h.db.AgentProjects()
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]bool, len(projects.Projects))
	for _, p := range projects.Projects {
		allowed[p.ID] = true
	}
	return allowed, nil
}

func (h *Handler) inventoryProjectAllowed(userID, projectID string) bool {
	allowed, err := h.inventoryProjects(userID)
	return err == nil && allowed[projectID]
}

func (h *Handler) receiveProjectInventory(ac *AgentConn, msg AgentMessage, size int) {
	p, err := agentprotocol.ParseProjectInventory(msg.Payload)
	if size > agentprotocol.InventoryMaxBytes {
		err = errors.New("invalid project inventory")
	}
	if err == nil {
		var allowed map[string]bool
		allowed, err = h.inventoryProjects(ac.UserID)
		for _, id := range p.ProjectIDs {
			if err != nil || !allowed[id] || (ac.ProjectID != "" && ac.ProjectID != "default" && ac.ProjectID != "all" && ac.ProjectID != id) {
				err = errors.New("invalid project inventory")
				break
			}
		}
	}
	if err == nil {
		err = h.agentDispatcher.acceptInventory(ac, p)
	}
	if err != nil {
		if errors.Is(err, db.ErrWorkstationSessionGone) {
			closeGoneSession(ac)
			return
		}
		log.Printf("[AgentConnect] Inventaire de projets refusé : %v", err)
		_ = ac.Send(AgentMessage{MsgID: msg.MsgID, Type: "error", Payload: []byte(`{"message":"Inventaire de projets refusé"}`)})
	}
}

// closeGoneSession hangs up on a connection whose session another instance
// forgot. Refusing its writes one by one left the agent connected, serving
// nothing and told nothing, for as long as it stayed up; once hung up, it
// reconnects and registers a session the directory knows.
func closeGoneSession(ac *AgentConn) {
	log.Printf("[AgentConnect] Session de device=%s oubliée par une autre instance : déconnexion pour réenregistrement", ac.DeviceID)
	ac.Close(agentCloseSessionGone, "workstation session forgotten by another server instance")
}

func (c *agentCluster) locations() []db.AgentLocation {
	if directory, ok := c.directory.(WorkstationDirectory); ok {
		return directory.ConnectedWorkstations()
	}
	return c.directory.ConnectedAgentLocations()
}
func (c *agentCluster) sessionDisconnected(ac *AgentConn) {
	if c == nil {
		return
	}
	if directory, ok := c.directory.(WorkstationDirectory); ok {
		if err := directory.WorkstationDisconnected(ac.ConnectionID); err != nil {
			log.Printf("[AgentDispatcher] Retrait de la session locale impossible : %v", err)
		}
	} else {
		c.agentDisconnected(ac)
	}
}
func (d *AgentDispatcher) sessionPing(ac *AgentConn) {
	if d.cluster == nil {
		return
	}
	if directory, ok := d.cluster.directory.(WorkstationDirectory); ok {
		err := directory.WorkstationPing(ac.ConnectionID, ac.LastSeen())
		if errors.Is(err, db.ErrWorkstationSessionGone) {
			closeGoneSession(ac)
		} else if err != nil {
			log.Printf("[AgentDispatcher] Actualisation de la présence locale impossible : %v", err)
		}
	}
}

// agentPresence carries the presence writes of one agent connection, its
// heartbeats and its project inventories, off the connection's read loop.
// Written from the loop, a database that took longer than the read deadline
// to answer stopped the loop from reading, and the agent was dropped for a
// silence it never kept. Only the latest of each is kept while one is being
// written: an older heartbeat or inventory has nothing left to say.
type agentPresence struct {
	mu        sync.Mutex
	ping      bool
	inventory *pendingInventory
	wake      chan struct{}
}

type pendingInventory struct {
	msg  AgentMessage
	size int
}

func newAgentPresence() *agentPresence {
	return &agentPresence{wake: make(chan struct{}, 1)}
}

func (p *agentPresence) notePing() {
	p.mu.Lock()
	p.ping = true
	p.mu.Unlock()
	p.signal()
}

func (p *agentPresence) noteInventory(msg AgentMessage, size int) {
	p.mu.Lock()
	p.inventory = &pendingInventory{msg: msg, size: size}
	p.mu.Unlock()
	p.signal()
}

func (p *agentPresence) signal() {
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

// runAgentPresence writes what the read loop noted until stop is closed.
func (h *Handler) runAgentPresence(ac *AgentConn, p *agentPresence, stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case <-p.wake:
		}
		p.mu.Lock()
		ping, inventory := p.ping, p.inventory
		p.ping, p.inventory = false, nil
		p.mu.Unlock()
		if ping {
			h.agentDispatcher.sessionPing(ac)
		}
		if inventory != nil {
			h.receiveProjectInventory(ac, inventory.msg, inventory.size)
		}
	}
}
