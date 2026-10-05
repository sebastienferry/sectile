package handlers

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"tasks/internal/agentprotocol"
)

// skillRefreshTimeout bounds the whole fan-out of one override change.
const skillRefreshTimeout = 30 * time.Second

// refreshSkillCopies asks every connected workstation that can see the project to rewrite the direct skill copies it already manages (#732).
//
// It is best-effort and runs in the background: the save already succeeded,
// and a workstation that is offline catches up at its next initialization or
// configuration sync. Sectile has no per-user project access: every agent is
// served every project (HandleAgentProjects), so "can see the project" means
// an agent connected for that project or for every project, which is what
// the operation's routing reaches. ConnectedAgents lists the agents of every
// instance sharing the database, and CallOperation forwards to the instance
// holding the connection, so a multi-instance deployment is covered. An agent
// that predates the operation, or one gone meanwhile, is skipped silently.
func (h *Handler) refreshSkillCopies(projectID string) {
	if h.agentDispatcher == nil || projectID == "" {
		return
	}
	// An operation is routed by user and project, so one call per user
	// reaches the device that serves the project; a second device of that
	// user would only be asked the same thing on the same connection.
	type target struct{ userID, deviceID string }
	seen := map[string]bool{}
	var targets []target
	for _, a := range h.agentDispatcher.ConnectedAgents() {
		if !servesProject(a.ProjectID, projectID) || seen[a.UserID] {
			continue
		}
		seen[a.UserID] = true
		targets = append(targets, target{a.UserID, a.DeviceID})
	}
	if len(targets) == 0 {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), skillRefreshTimeout)
		defer cancel()
		var wg sync.WaitGroup
		for _, t := range targets {
			wg.Add(1)
			go func(t target) {
				defer wg.Done()
				_, err := h.agentDispatcher.CallOperation(ctx, agentprotocol.Operation{UserID: t.userID, ProjectID: projectID, Action: "refresh_skills"})
				if err != nil && !errors.Is(err, ErrNoAgentConnected) && !errors.Is(err, agentprotocol.ErrUnsupportedOperation) {
					log.Printf("[SkillRefresh] project=%s device=%s: %v", projectID, t.deviceID, err)
				}
			}(t)
		}
		wg.Wait()
	}()
}

// servesProject says whether an agent connected for registered is the one an
// operation on projectID reaches: the same project, or the wildcard keys the
// dispatcher falls back to (Lookup).
func servesProject(registered, projectID string) bool {
	switch registered {
	case projectID, "", "default", "all":
		return true
	}
	return false
}
