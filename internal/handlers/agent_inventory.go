package handlers

import (
	"cmp"
	"errors"
	"log"
	"slices"
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

func (d *AgentDispatcher) acceptInventory(ac *AgentConn, p agentprotocol.ProjectInventory) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.live[ac.ConnectionID] != ac {
		return errors.New("inactive agent session")
	}
	now := time.Now().UTC()
	if d.cluster != nil {
		if directory, ok := d.cluster.directory.(WorkstationDirectory); ok {
			if err := directory.WorkstationInventory(ac.ConnectionID, p.ProjectIDs, now); err != nil {
				return err
			}
		} else {
			return errors.New("workstation directory unavailable")
		}
	}
	ac.inventory = p
	ac.inventoryUpdatedAt = now
	return nil
}

type AvailableWorkstation struct {
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
	// A device's timestamps describe all its live scopes, even unknown inventories.
	for _, a := range agents {
		w, exists := devices[a.DeviceID]
		if a.UserID != userID || !exists {
			continue
		}
		if a.ConnectedAt.Before(w.ConnectedAt) {
			w.ConnectedAt = a.ConnectedAt
		}
		if a.LastPingAt.After(w.LastPingAt) {
			w.LastPingAt = a.LastPingAt
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
		log.Printf("[AgentConnect] Inventaire de projets refusé : %v", err)
		_ = ac.Send(AgentMessage{MsgID: msg.MsgID, Type: "error", Payload: []byte(`{"message":"Inventaire de projets refusé"}`)})
	}
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
		c.agentDisconnected(ac.UserID, ac.ProjectID, ac.DeviceID)
	}
}
func (d *AgentDispatcher) sessionPing(ac *AgentConn) {
	if d.cluster == nil {
		return
	}
	if directory, ok := d.cluster.directory.(WorkstationDirectory); ok {
		if err := directory.WorkstationPing(ac.ConnectionID, ac.LastSeen()); err != nil {
			log.Printf("[AgentDispatcher] Actualisation de la présence locale impossible : %v", err)
		}
	}
}
