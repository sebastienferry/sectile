package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// handleProjectSprints serves the sprint management routes of a project:
//
//	POST   /api/projects/{id}/sprints              {name, count, start, weeks}
//	PATCH  /api/projects/{id}/sprints/{sprintId}   SprintPatch (PUT accepted)
//	DELETE /api/projects/{id}/sprints/{sprintId}
//
// Each call writes to the tracker before it answers. A tracker that does not
// manage its sprints is a 409, any other refusal a 400 carrying its reason.
func (h *Handler) handleProjectSprints(w http.ResponseWriter, r *http.Request, projectID string, parts []string) {
	ctx := tracker.WithProject(h.actingContext(r), projectID)
	sprintID := ""
	if len(parts) >= 3 {
		sprintID = parts[2]
		if decoded, err := url.PathUnescape(sprintID); err == nil {
			sprintID = decoded
		}
	}
	switch {
	case sprintID == "" && r.Method == http.MethodPost:
		var req struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
			Start string `json:"start"`
			Weeks int    `json:"weeks"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid payload: "+err.Error())
			return
		}
		// A day is read at 09:00 local time, which is when a sprint starts in
		// practice; no day means today.
		start := time.Now()
		if day := strings.TrimSpace(req.Start); day != "" {
			parsed, err := time.ParseInLocation("2006-01-02", day, time.Local)
			if err != nil {
				writeError(w, http.StatusBadRequest, "date de début invalide : attendu AAAA-MM-JJ")
				return
			}
			start = parsed
		}
		start = time.Date(start.Year(), start.Month(), start.Day(), 9, 0, 0, 0, time.Local)
		created, err := h.db.CreateProjectSprints(ctx, projectID, req.Name, req.Count, start, req.Weeks)
		if err != nil && len(created) > 0 {
			writeJSON(w, http.StatusMultiStatus, map[string]any{"created": created, "error": err.Error()})
			return
		}
		if err != nil {
			writeSprintError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"created": created})
	case sprintID != "" && (r.Method == http.MethodPatch || r.Method == http.MethodPut):
		var patch models.SprintPatch
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid payload: "+err.Error())
			return
		}
		updated, err := h.db.UpdateProjectSprint(ctx, projectID, sprintID, patch)
		if err != nil {
			writeSprintError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)
	case sprintID != "" && r.Method == http.MethodDelete:
		if err := h.db.DeleteProjectSprint(ctx, projectID, sprintID); err != nil {
			writeSprintError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"deleted": sprintID})
	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func writeSprintError(w http.ResponseWriter, err error) {
	var unsupported *tracker.ErrUnsupported
	if errors.As(err, &unsupported) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeTrackerError(w, http.StatusBadRequest, err)
}
