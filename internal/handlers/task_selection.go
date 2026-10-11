package handlers

import (
	"context"
	"slices"
	"sort"
	"time"
)

// taskRoutes reads inventory from the exact canonical session, rather than
// treating its registration scope as proof that the project is available.
func (d *AgentDispatcher) taskRoutes(userID, projectID string) []*AgentRoute {
	routes := d.userRoutes(userID)
	eligible := make([]*AgentRoute, 0, len(routes))
	for _, route := range routes {
		qualifies := false
		if route.remote != nil {
			qualifies = route.remote.InventoryState == "reported" && slices.Contains(route.remote.ProjectIDs, projectID)
			route.connectedAt = route.remote.ConnectedAt
		} else {
			d.mu.RLock()
			ac := route.local
			qualifies = d.live[ac.ConnectionID] == ac && !ac.inventoryUpdatedAt.IsZero() && slices.Contains(ac.inventory.ProjectIDs, projectID)
			route.connectedAt = ac.ConnectedAt
			d.mu.RUnlock()
		}
		if qualifies {
			route.taskLaunch = true
			eligible = append(eligible, route)
		}
	}
	sort.Slice(eligible, func(i, j int) bool {
		if eligible[i].connectedAt.Equal(eligible[j].connectedAt) {
			return eligible[i].DeviceID < eligible[j].DeviceID
		}
		return eligible[i].connectedAt.After(eligible[j].connectedAt)
	})
	return eligible
}

type taskSelectionError struct {
	cause   error
	message string
}

func (e *taskSelectionError) Error() string { return e.message }
func (e *taskSelectionError) Unwrap() error { return e.cause }

func taskTargetError(explicit bool, affinity string) error {
	if explicit {
		return &taskSelectionError{ErrInvalidDevice, "Le poste demandé est indisponible ou ne rapporte pas ce projet"}
	}
	if affinity != "" {
		return &taskSelectionError{ErrDeviceRequired, "Le poste du dernier lancement est indisponible ; indiquez targetDeviceId pour choisir explicitement un autre poste"}
	}
	return &taskSelectionError{ErrDeviceRequired, "Aucun poste connecté ne rapporte ce projet"}
}

func (d *AgentDispatcher) selectTaskRoute(ctx context.Context, userID, projectID, target, affinity string, grace time.Duration) (*AgentRoute, error) {
	device := target
	if device == "" {
		device = affinity
	}
	find := func() *AgentRoute {
		for _, route := range d.taskRoutes(userID, projectID) {
			if device == "" || route.DeviceID == device {
				return route
			}
		}
		return nil
	}
	if route := find(); route != nil {
		return route, nil
	}
	refusal := taskTargetError(target != "", affinity)
	// Only an explicit or persisted device may wait. Automatic routing must
	// never resurrect a disconnected device merely from in-memory presence.
	if device == "" {
		return nil, refusal
	}
	// An offline unknown device or an already reported wrong inventory is
	// invalid immediately. Only recent disconnections and initial inventory
	// reporting can use reconnect grace.
	wait := false
	connected := false
	for _, route := range d.userRoutes(userID) {
		if route.DeviceID != device {
			continue
		}
		connected = true
		if route.remote != nil {
			wait = route.remote.InventoryState != "reported"
		} else {
			d.mu.RLock()
			wait = route.local.inventoryUpdatedAt.IsZero()
			d.mu.RUnlock()
		}
	}
	if !connected {
		d.mu.RLock()
		at := d.lastConnected[agentKey{UserID: userID, DeviceID: device}]
		wait = !at.IsZero() && time.Since(at) < agentAbsenceIsRecent
		d.mu.RUnlock()
		if d.cluster != nil {
			if directory, ok := d.cluster.directory.(interface {
				RecentAgentDevices(string, string, time.Time) []string
			}); ok {
				wait = wait || slices.Contains(directory.RecentAgentDevices(userID, projectID, time.Now().UTC().Add(-agentAbsenceIsRecent)), device)
			}
		}
	}
	if !wait {
		return nil, refusal
	}
	timer := time.NewTimer(grace)
	defer timer.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, refusal
		case <-timer.C:
			return nil, refusal
		case <-ticker.C:
			if route := find(); route != nil {
				return route, nil
			}
		}
	}
}

func (h *Handler) selectTaskLaunchRoute(ctx context.Context, taskID, userID, projectID, target string) (*AgentRoute, error) {
	affinity := ""
	if target == "" {
		var err error
		affinity, err = h.db.TaskLaunchDevice(taskID, projectID, userID)
		if err != nil {
			return nil, err
		}
	}
	return h.agentDispatcher.selectTaskRoute(ctx, userID, projectID, target, affinity, defaultAgentReconnectGrace)
}
