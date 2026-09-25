package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"tasks/internal/agentconfig"
	"tasks/internal/db"
	"tasks/internal/models"
	"tasks/internal/skills"
)

// handleMacroRunSkill serves POST /api/projects/{id}/macros/{key}/run-skill:
// it launches a macro-scoped skill on the caller's local agent, the way a task
// skill is launched, with a run recorded on the macro rather than on a task.
func (h *Handler) handleMacroRunSkill(w http.ResponseWriter, r *http.Request, projectID, macroKey string) {
	var req models.RunSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid skill request: "+err.Error())
		return
	}
	skillID := models.NormalizeSkillID(req.SkillID)
	stage, ok := skills.StageSkillByID(skillID)
	if !ok || stage.Scope != "macro" {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("%q n'est pas une compétence de macro", req.SkillID))
		return
	}
	if err := agentconfig.ValidModel(req.Model); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	project, err := h.db.GetProjectByID(projectID)
	if err != nil || project == nil {
		writeError(w, http.StatusNotFound, "Project not found")
		return
	}
	title := ""
	macroFound := false
	if macros, err := h.db.GetProjectMacros(project.ID); err == nil {
		for _, macro := range macros {
			if strings.EqualFold(macro.Key, macroKey) {
				macroKey, title, macroFound = macro.Key, macro.Title, true
				break
			}
		}
	}
	if !macroFound {
		writeError(w, http.StatusNotFound, "Macro not found")
		return
	}
	if active, err := h.db.ActiveRunOnMacro(project.ID, macroKey); err != nil {
		writeError(w, http.StatusInternalServerError, "Cannot check the macro for an active run")
		return
	} else if active != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": describeActiveRun(active), "activeRunId": active.ID})
		return
	}

	userID := h.webSessionUser(r)
	ac := h.agentDispatcher.Route(userID, project.ID)
	if ac == nil {
		writeError(w, http.StatusFailedDependency, "Connectez l'agent local pour lancer cette compétence.")
		return
	}
	provider, model := h.db.ResolveTaskEngine(project.ID, skillID, req.Model)
	run, err := h.db.StartMacroRun(project.ID, macroKey, skillID, db.RunLaunch{Mode: models.SkillModeInteractive, Provider: provider, Model: model, UserID: userID})
	if errors.Is(err, db.ErrMacroRunBusy) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Cannot track remote execution: "+err.Error())
		return
	}
	log.Printf("🚀 [Dispatch] Delegating macro skill %s on %s (%s) to connected local agent (device=%s)", skillID, macroKey, project.ID, ac.DeviceID)
	launchCtx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	err = h.agentDispatcher.DispatchAndWait(launchCtx, ac.UserID, ac.ProjectID, "", agentconfig.Dispatch{
		SchemaVersion: agentconfig.Version, ProjectID: project.ID, MacroKey: macroKey, MacroTitle: title,
		SkillID: skillID, Action: skillID, Prompt: strings.TrimSpace(req.Prompt), RunID: run.ID,
		Mode: models.SkillModeInteractive, Model: strings.TrimSpace(req.Model),
	})
	if err != nil {
		// As for a task launch, an unconfirmed launch may be running: its run
		// stays open until the agent reports.
		if !errors.Is(err, ErrLaunchUnconfirmed) {
			_, _ = h.db.FinishMacroRunAs(db.Actor{ID: userID}, true, project.ID, macroKey, run.ID, "failed", err.Error())
		}
		writeError(w, http.StatusBadGateway, "Erreur lors de la délégation à l'agent local: "+err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"runId":    run.ID,
		"activity": run,
		"message":  fmt.Sprintf("Compétence %s déléguée à l'agent local (%s)", skillID, ac.DeviceID),
	})
}

// handleMacroRuns serves GET /api/projects/{id}/macros/{key}/runs: the macro's
// recent skill runs, most recent first, the running one included.
func (h *Handler) handleMacroRuns(w http.ResponseWriter, projectID, macroKey string) {
	// The route takes a slug as run-skill does; the runs are stored under the
	// project's primary key.
	project, err := h.db.GetProjectByID(projectID)
	if err != nil || project == nil {
		writeError(w, http.StatusNotFound, "Project not found")
		return
	}
	runs, err := h.db.MacroRuns(project.ID, macroKey, 10)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs})
}

// handleMacroCancelRun serves POST /api/projects/{id}/macros/{key}/cancel-run
// {runId, force}: it stops a macro skill run the way a task run is stopped
// (handleCancelRemoteRun). The owner's agent is asked to stop the process; an
// agent that no longer has the run closes it as orphaned; an unreachable agent
// needs force, and the note then says no local process was stopped.
func (h *Handler) handleMacroCancelRun(w http.ResponseWriter, r *http.Request, projectID, macroKey string) {
	var input struct {
		RunID string `json:"runId"`
		Force bool   `json:"force"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil || input.RunID == "" {
		writeError(w, http.StatusBadRequest, "runId is required")
		return
	}
	project, err := h.db.GetProjectByID(projectID)
	if err != nil || project == nil {
		writeError(w, http.StatusNotFound, "Project not found")
		return
	}
	active, err := h.db.ActiveRunOnMacro(project.ID, macroKey)
	if err != nil || active == nil || active.ID != input.RunID {
		writeError(w, http.StatusConflict, "Active remote execution not found on this macro")
		return
	}
	caller, ok := h.requireOwnerOrAdmin(w, r, active.UserID)
	if !ok {
		return
	}
	userID := active.UserID
	if userID == "" {
		userID = caller.UserID
	}
	note := "Execution stopped by the local agent"
	if active.Action == db.RunActionAgent {
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		dispatchErr := h.agentDispatcher.DispatchAndWait(ctx, userID, project.ID, "", agentconfig.Dispatch{
			SchemaVersion: agentconfig.Version, ProjectID: project.ID, RunID: active.ID, Action: "cancel_run",
		})
		switch {
		case dispatchErr == nil:
		case errors.Is(dispatchErr, ErrRunNotOwned):
			note = "Execution closed: the local agent restarted and no longer had this run"
		case input.Force:
			note = "Execution closed without reaching the local agent; any local process was left running"
		case errors.Is(dispatchErr, ErrNoAgentConnected):
			writeError(w, http.StatusBadGateway,
				"No local agent is connected, so the execution could not be stopped. Retry once the agent is running, or force the close to clear the run without stopping any local process.")
			return
		default:
			writeError(w, http.StatusBadGateway, dispatchErr.Error())
			return
		}
	} else if !input.Force {
		// A run a client reported by hand has no agent process to stop; closing
		// it is the client's own report, or an explicit force.
		writeError(w, http.StatusConflict, "This execution was reported by an agent session, not launched from Sectile: force the close to clear it.")
		return
	} else {
		note = "Execution closed from Sectile; the session that reported it was not stopped"
	}
	activity, err := h.db.FinishMacroRunAs(db.Actor{}, true, project.ID, macroKey, active.ID, "canceled", note)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, activity)
}

// slicingReadError turns the two failures of reaching the local agent into
// what the user can act on: the slicing import reads the specifications on
// their workstation, so a missing or outdated desktop app is the cause, not a
// configuration problem. Any other error is already written for the user.
func slicingReadError(err error) error {
	switch {
	case err == nil:
		return nil
	case strings.Contains(err.Error(), `unknown local operation "macro_spec_file"`):
		return errors.New("Votre app desktop Sectile est trop ancienne pour importer la découpe : mettez-la à jour puis réessayez.")
	case errors.Is(err, ErrNoAgentConnected), strings.Contains(err.Error(), "requires a connected agent"):
		return errors.New("L'import de la découpe lit les spécifications sur votre poste : connectez l'app desktop Sectile puis réessayez.")
	}
	// The agent's refusals are written for the user; the relay's prefix is
	// not, and it would open a French sentence with an English label.
	if refusal, ok := strings.CutPrefix(err.Error(), "local agent: "); ok {
		return errors.New(refusal)
	}
	return err
}
