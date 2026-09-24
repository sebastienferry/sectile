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
	runs, err := h.db.MacroRuns(projectID, macroKey, 10)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs})
}
