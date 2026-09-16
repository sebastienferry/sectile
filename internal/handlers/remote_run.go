package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"tasks/internal/agentconfig"
	"tasks/internal/db"
	"time"
)

// handleCancelRemoteRun stops an execution the server believes is running.
//
// Stopping used to require the agent to confirm, and treated every other
// answer as a failure. An agent that restarted no longer knows the run, so it
// answered that it does not own it, the request failed, and the run stayed
// open for good: the task became impossible to stop, and each retry repeated
// the same exchange.
//
// The agent's answers are now read for what they say. One of them, "I do not
// have this run", comes from a reachable agent stating it is running nothing,
// which is exactly what identifies an orphan; the run is then closed. An agent
// that cannot be reached says nothing at all, and closing the run there would
// claim a stop nobody witnessed, so it takes an explicit force.
func (h *Handler) handleCancelRemoteRun(w http.ResponseWriter, r *http.Request, taskID string) {
	var input struct {
		RunID string `json:"runId"`
		// Force closes the run when the agent cannot be reached. The process,
		// if any, is left alone, so the note says so rather than claiming a
		// stop that was never observed.
		Force bool `json:"force"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil || input.RunID == "" {
		writeError(w, 400, "runId is required")
		return
	}
	task, err := h.db.GetTaskByID(taskID)
	if err != nil || task == nil {
		writeError(w, 404, "Task not found")
		return
	}
	run, err := h.db.GetActivityByID(input.RunID)
	if err != nil || run == nil || run.TaskID != task.ID || run.SkillID != "remote_run" || run.Status != "running" || run.Action != db.RunActionAgent {
		writeError(w, 409, "Active remote execution not found")
		return
	}

	// The agent registers under the user its credential resolves to, so the
	// caller's own identity is what finds it. Without an identity provider
	// this is the single implicit user, as before.
	userID := h.webSessionUser(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "Sign in to stop an execution")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	dispatchErr := h.agentDispatcher.DispatchAndWait(ctx, userID, task.ProjectID, task.ID, agentconfig.Dispatch{
		SchemaVersion: agentconfig.Version, TaskID: task.ID, RunID: run.ID, Action: "cancel_run",
	})

	note := "Execution stopped by the local agent"
	switch {
	case dispatchErr == nil:
	case errors.Is(dispatchErr, ErrRunNotOwned):
		log.Printf("[RemoteRun] Closing orphaned run %s on task %s: the agent does not have it", run.ID, task.ID)
		note = "Execution closed: the local agent restarted and no longer had this run"
	case input.Force:
		log.Printf("[RemoteRun] Force closing run %s on task %s: %v", run.ID, task.ID, dispatchErr)
		note = "Execution closed without reaching the local agent; any local process was left running"
	default:
		// Unreachable is not the same as stopped. Say which it is, and what
		// closing anyway would mean.
		if errors.Is(dispatchErr, ErrNoAgentConnected) {
			writeError(w, http.StatusBadGateway,
				"No local agent is connected, so the execution could not be stopped. Retry once the agent is running, or force the close to clear the run without stopping any local process.")
			return
		}
		writeError(w, http.StatusBadGateway, dispatchErr.Error())
		return
	}

	activity, err := h.db.FinishRemoteRun(task.ID, run.ID, "canceled", note)
	if err != nil {
		writeError(w, 409, err.Error())
		return
	}
	writeJSON(w, 200, activity)
}
