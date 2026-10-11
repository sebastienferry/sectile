package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"tasks/internal/agentconfig"
)

var ErrDeviceRequired = errors.New("workstation selection required")
var ErrInvalidDevice = errors.New("invalid workstation selection")

type deviceSelectionError struct {
	cause   error
	devices []string
}

func (e *deviceSelectionError) Error() string {
	message := "Plusieurs postes sont connectés"
	if errors.Is(e.cause, ErrInvalidDevice) {
		message = "Le poste demandé est inconnu ou ne couvre pas ce projet"
	}
	return fmt.Sprintf("%s : indiquez deviceId pour choisir un poste connecté (postes disponibles : %s)", message, strings.Join(e.devices, ", "))
}
func (e *deviceSelectionError) Unwrap() error { return e.cause }
func firstDevice(devices []string) string {
	if len(devices) > 0 {
		return devices[0]
	}
	return ""
}
func projectEligible(registered, requested string) bool {
	return registered == requested || registered == "" || registered == "all" || registered == "default"
}

// userRoutes deduplicates the live directory by device. A directory record
// supersedes a local connection displaced by a connection on another instance.
func (d *AgentDispatcher) userRoutes(userID string) []*AgentRoute {
	routes := map[string]*AgentRoute{}
	d.mu.RLock()
	for _, ac := range d.agents {
		if ac.UserID == userID {
			routes[ac.DeviceID] = &AgentRoute{UserID: userID, ProjectID: ac.ProjectID, DeviceID: ac.DeviceID, local: ac}
		}
	}
	d.mu.RUnlock()
	if d.cluster != nil {
		self := d.cluster.directory.InstanceID()
		for _, location := range d.cluster.locations() {
			if location.UserID != userID {
				continue
			}
			if location.InstanceID != self {
				l := location
				routes[l.DeviceID] = &AgentRoute{UserID: userID, ProjectID: l.ProjectID, DeviceID: l.DeviceID, remote: &l}
			}
		}
	}
	out := make([]*AgentRoute, 0, len(routes))
	for _, route := range routes {
		out = append(out, route)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DeviceID < out[j].DeviceID })
	return out
}

// SelectRoute validates once, before side effects, and returns the exact route
// on which the caller must send. Activity never chooses between workstations.
func (d *AgentDispatcher) SelectRoute(userID, projectID, deviceID string) (*AgentRoute, error) {
	routes := d.userRoutes(userID)
	eligible := []string{}
	for _, route := range routes {
		if projectEligible(route.ProjectID, projectID) {
			eligible = append(eligible, route.DeviceID)
		}
	}
	if deviceID == "" && len(routes) > 1 {
		return nil, &deviceSelectionError{ErrDeviceRequired, eligible}
	}
	for _, route := range routes {
		if (deviceID == "" || route.DeviceID == deviceID) && projectEligible(route.ProjectID, projectID) {
			return route, nil
		}
	}
	if deviceID != "" {
		return nil, &deviceSelectionError{ErrInvalidDevice, eligible}
	}
	return nil, ErrNoAgentConnected
}

func writeDeviceSelectionError(w http.ResponseWriter, err error) bool {
	status := 0
	switch {
	case errors.Is(err, ErrDeviceRequired):
		status = http.StatusPreconditionRequired
	case errors.Is(err, ErrInvalidDevice):
		status = http.StatusBadRequest
	}
	if status == 0 {
		return false
	}
	writeError(w, status, err.Error())
	return true
}

func (d *AgentDispatcher) waitForSelectedRoute(ctx context.Context, userID, projectID, deviceID string, grace time.Duration) (*AgentRoute, error) {
	route, err := d.SelectRoute(userID, projectID, deviceID)
	if err == nil || errors.Is(err, ErrDeviceRequired) {
		return route, err
	}
	// An invalid live target is a selection error, not a reconnect.
	for _, route := range d.userRoutes(userID) {
		if route.DeviceID == deviceID {
			return nil, err
		}
	}
	recent := map[string]bool{}
	d.mu.RLock()
	for key, at := range d.lastConnected {
		if key.UserID == userID && projectEligible(d.lastProjects[key], projectID) && time.Since(at) < agentAbsenceIsRecent {
			recent[key.DeviceID] = true
		}
	}
	d.mu.RUnlock()
	if d.cluster != nil {
		if directory, ok := d.cluster.directory.(interface {
			RecentAgentDevices(string, string, time.Time) []string
		}); ok {
			for _, device := range directory.RecentAgentDevices(userID, projectID, time.Now().UTC().Add(-agentAbsenceIsRecent)) {
				recent[device] = true
			}
		}
	}
	if deviceID == "" {
		if len(recent) != 1 {
			return nil, err
		}
		for device := range recent {
			deviceID = device
		}
	} else if !recent[deviceID] {
		return nil, err
	}
	// Pin the historical device for the entire grace; a new workstation cannot
	// receive a command intended for the device that just disconnected.
	timer := time.NewTimer(grace)
	defer timer.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ErrNoAgentConnected
		case <-timer.C:
			return nil, ErrNoAgentConnected
		case <-ticker.C:
			route, err = d.SelectRoute(userID, projectID, deviceID)
			if err == nil {
				return route, nil
			}
			if !errors.Is(err, ErrNoAgentConnected) && !errors.Is(err, ErrInvalidDevice) {
				return nil, err
			}
		}
	}
}

func (d *AgentDispatcher) rememberRun(runID, userID, deviceID string) {
	if runID == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.runOwners == nil {
		d.runOwners = map[string]agentKey{}
	}
	if owner, exists := d.runOwners[runID]; exists && owner.UserID != userID {
		return
	}
	d.runOwners[runID] = agentKey{UserID: userID, DeviceID: deviceID}
}
func (d *AgentDispatcher) ForgetRun(runID string) {
	d.mu.Lock()
	delete(d.runOwners, runID)
	d.mu.Unlock()
}

// CancelRun asks all fallback candidates concurrently within the caller's
// deadline. Only complete non-ownership evidence establishes an orphan.
func (d *AgentDispatcher) CancelRun(ctx context.Context, userID, projectID, taskID, runID string) error {
	routes := d.userRoutes(userID)
	d.mu.RLock()
	owner, known := d.runOwners[runID]
	d.mu.RUnlock()
	known = known && owner.UserID == userID
	var preferred *AgentRoute
	var candidates []*AgentRoute
	for _, route := range routes {
		if !projectEligible(route.ProjectID, projectID) {
			continue
		}
		candidates = append(candidates, route)
		if known && route.DeviceID == owner.DeviceID {
			preferred = route
		}
	}
	if len(candidates) == 0 {
		return ErrNoAgentConnected
	}
	payload := agentconfig.Dispatch{SchemaVersion: agentconfig.Version, TaskID: taskID, ProjectID: projectID, RunID: runID, Action: "cancel_run"}
	send := func(ctx context.Context, route *AgentRoute) error {
		return d.dispatchAndWaitRoute(ctx, route, userID, projectID, taskID, payload)
	}
	// Keep the owner's acknowledgment alive while fallback starts after a
	// short grace. Its process stop may legitimately take several seconds.
	attemptCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan error, len(candidates))
	count := 0
	incomplete := known && preferred == nil
	var failure error
	if preferred != nil {
		count++
		go func() { results <- send(attemptCtx, preferred) }()
		grace := time.NewTimer(time.Second)
		select {
		case err := <-results:
			count--
			if err == nil {
				grace.Stop()
				d.ForgetRun(runID)
				return nil
			}
			if !errors.Is(err, ErrRunNotOwned) {
				incomplete = true
				failure = err
			}
		case <-grace.C:
		case <-ctx.Done():
			grace.Stop()
			return fmt.Errorf("cancellation not confirmed: %w", ctx.Err())
		}
		grace.Stop()
	}
	for _, route := range candidates {
		if route == preferred {
			continue
		}
		count++
		go func(route *AgentRoute) { results <- send(attemptCtx, route) }(route)
	}
	for i := 0; i < count; i++ {
		select {
		case err := <-results:
			if err == nil {
				d.ForgetRun(runID)
				return nil
			}
			if !errors.Is(err, ErrRunNotOwned) {
				incomplete = true
				failure = err
			}
		case <-ctx.Done():
			return fmt.Errorf("cancellation not confirmed: %w", ctx.Err())
		}
	}
	if incomplete {
		if failure == nil {
			failure = ErrNoAgentConnected
		}
		return fmt.Errorf("cancellation not confirmed by the run owner: %w", failure)
	}
	d.ForgetRun(runID)
	return ErrRunNotOwned
}

func (d *AgentDispatcher) runDevice(runID, userID string) string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	owner := d.runOwners[runID]
	if owner.UserID == userID {
		return owner.DeviceID
	}
	return ""
}
