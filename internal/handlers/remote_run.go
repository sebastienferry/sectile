package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"tasks/internal/agentconfig"
	"tasks/internal/db"
	"tasks/internal/models"
	"time"
)

// handleCancelRemoteRun stops an execution the server believes is running.
// A single non-owner reply is insufficient with concurrent workstations.
// Cancellation searches the owner's devices within one bounded deadline;
// only complete non-ownership evidence permits closing an orphan. Incomplete
// evidence requires an explicit force and says the process may remain running.
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
	if writeTaskKeyAmbiguous(w, err) {
		return
	}
	if err != nil || task == nil {
		writeError(w, 404, "Task not found")
		return
	}
	run, err := h.db.GetActivityByID(input.RunID)
	if err != nil || run == nil || run.TaskID != task.ID || run.SkillID != "remote_run" || run.Status != "running" {
		writeError(w, 409, "Active remote execution not found")
		return
	}
	if run.Action == db.RunActionClient {
		h.closeClientRun(w, r, task, run)
		return
	}
	if run.Action != db.RunActionAgent {
		writeError(w, 409, "Active remote execution not found")
		return
	}

	// Only the owner or an admin stops an execution. The stop goes to the
	// owner's agent, which is the one that has the process: routing it to the
	// caller's own agent would make that agent answer "not mine", and the run
	// would be closed as orphaned while it keeps running on another machine.
	// A run without recorded owner predates ownership; it is an admin's to
	// close, through the caller's agent as before.
	caller, ok := h.requireOwnerOrAdmin(w, r, run.UserID)
	if !ok {
		return
	}
	userID := run.UserID
	if userID == "" {
		userID = caller.UserID
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
	h.agentDispatcher.ForgetRun(run.ID)
	writeJSON(w, 200, activity)
}

// closeClientRun closes a run a connected client created, from the board (#319).
// No agent holds such a run, so there is nothing to stop: its client is an MCP
// session, alive or long gone. The run is closed the way a disconnection closes
// it, with the disconnect note, so its owner can still report the real outcome
// through finish_run, and it goes through FinishRemoteRunAs so the workflow is
// handed back exactly as a reported end would. Its session then forgets it,
// rather than closing it a second time when it ends.
func (h *Handler) closeClientRun(w http.ResponseWriter, r *http.Request, task *models.Task, run *models.TaskActivity) {
	caller, ok := h.requireOwnerOrAdmin(w, r, run.UserID)
	if !ok {
		return
	}
	activity, err := h.db.FinishRemoteRunAs(caller.Actor(), caller.IsAdmin(), task.ID, run.ID, "canceled", models.RunDisconnectNote)
	if errors.Is(err, db.ErrRunNotYours) {
		writeError(w, http.StatusForbidden, msgNotOwner)
		return
	}
	if err != nil {
		writeError(w, 409, err.Error())
		return
	}
	h.mcpSessions.ReleaseRun(run.ID)
	log.Printf("[RemoteRun] Closed client run %s on task %s from the board", run.ID, task.ID)
	h.agentDispatcher.ForgetRun(run.ID)
	writeJSON(w, 200, activity)
}
