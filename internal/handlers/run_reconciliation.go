package handlers

import (
	"log"
	"time"

	"tasks/internal/agentprotocol"
	"tasks/internal/db"
	"tasks/internal/models"
)

// reconnectCandidates snapshots only previously dispatched runs pinned to this
// exact owner/device before its new session connected. New admissions, legacy
// unknown assignments and in-flight launches are not orphan evidence.
func (h *Handler) reconnectCandidates(ownerID, deviceID string, connectedAt time.Time) []models.TaskActivity {
	activities, err := h.db.GetActivities("", "", "remote_run", "", "", 10000)
	if err != nil {
		return nil
	}
	var candidates []models.TaskActivity
	for _, run := range activities {
		if run.UserID != ownerID || run.DeviceID != deviceID || run.Action != db.RunActionAgent ||
			!run.CreatedAt.Before(connectedAt) || (run.Status != "running" && run.Status != "queued" && run.Status != "pending") {
			continue
		}
		state, err := h.db.RunDispatchState(run.ID)
		if err == nil && state == "dispatched" {
			candidates = append(candidates, run)
		}
	}
	return candidates
}

// A successful authenticated full supervisor snapshot is authoritative for its
// pinned device. Failed pulls never call this, and absence of a connection is
// never used as evidence that a process ended.
func (h *Handler) reconcileAssignedRuns(candidates []models.TaskActivity, reported []agentprotocol.RunningTask) {
	present := map[string]bool{}
	for _, run := range reported {
		present[run.ID] = true
	}
	for _, run := range candidates {
		if present[run.ID] {
			continue
		}
		// An orchestrator session has no task: it is closed as a session, which
		// lets the next one start.
		if h.db.IsOrchestratorSession(run.ID) {
			if _, err := h.db.FinishOrchestratorSession(db.Actor{}, true, "", run.ID, "canceled", "Exécution fermée : le poste attribué s’est reconnecté et ne détient plus cette exécution"); err != nil {
				log.Printf("[RemoteRun] Réconciliation de la session d’orchestration orpheline %s refusée : %v", run.ID, err)
				continue
			}
			h.agentDispatcher.ForgetRun(run.ID)
			h.BroadcastEvent(Event{Type: "agent_step_status"})
			continue
		}
		if _, err := h.db.FinishRemoteRun(run.TaskID, run.ID, "canceled", "Exécution fermée : le poste attribué s’est reconnecté et ne détient plus cette exécution"); err != nil {
			log.Printf("[RemoteRun] Réconciliation de l’exécution orpheline %s refusée : %v", run.ID, err)
			continue
		}
		h.agentDispatcher.ForgetRun(run.ID)
		h.BroadcastEvent(Event{Type: "agent_step_status"})
	}
}
