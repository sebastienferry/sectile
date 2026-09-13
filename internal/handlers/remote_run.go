package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"tasks/internal/agentconfig"
	"time"
)

func (h *Handler) handleCancelRemoteRun(w http.ResponseWriter, r *http.Request, taskID string) {
	var input struct {
		RunID string `json:"runId"`
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
	if err != nil || run == nil || run.TaskID != task.ID || run.SkillID != "remote_run" || run.Status != "running" || run.Action != "Agent-owned remote execution" {
		writeError(w, 409, "Active remote execution not found")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if err := h.agentDispatcher.DispatchAndWait(ctx, "default", task.ProjectID, task.ID, agentconfig.Dispatch{SchemaVersion: agentconfig.Version, TaskID: task.ID, RunID: run.ID, Action: "cancel_run"}); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	activity, err := h.db.FinishRemoteRun(task.ID, run.ID, "canceled", "Execution stopped by the local agent")
	if err != nil {
		writeError(w, 409, err.Error())
		return
	}
	writeJSON(w, 200, activity)
}
