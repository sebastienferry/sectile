package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"tasks/internal/db"
	"tasks/internal/models"
)

// TrackersPath is where a member picks the trackers of a project, reads a
// tracker's backlog and labels from it, and asks for a synchronisation (#741).
const TrackersPath = "/api/trackers"

// AdminTrackersPath is where an admin records and configures the trackers:
// their source, board, column mapping, issue types and auto-sync (#741, D11).
const AdminTrackersPath = "/api/admin/trackers"

// trackerSummary is what a member sees of a tracker: enough to pick it.
type trackerSummary struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Site     string `json:"site"`
	Scope    string `json:"scope"`
	Identity string `json:"identity"`
}

// HandleTrackers serves the member routes:
//
//	GET  /api/trackers                                   the trackers to pick from
//	POST /api/trackers/{id}/sync                         a synchronisation of the tracker
//	GET  /api/trackers/{id}/backlog                      its tickets in no project
//	POST /api/trackers/{id}/backlog/{taskId}/project     {projectId}: label one into a project
//
// A member reaches a tracker's routes when a project selects it, as every
// member sees every project; an admin always does. Local boards are each one
// project's and are not listed.
func (h *Handler) HandleTrackers(w http.ResponseWriter, r *http.Request) {
	caller, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, TrackersPath), "/")
	if rest == "" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		trackers, err := h.db.GetTrackers()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		list := []trackerSummary{}
		for _, t := range trackers {
			if t.Provider == "local" {
				continue
			}
			list = append(list, trackerSummary{ID: t.ID, Name: t.Name, Provider: t.Provider, Site: t.Site, Scope: t.Scope, Identity: t.Identity})
		}
		writeJSON(w, http.StatusOK, list)
		return
	}

	parts := strings.Split(rest, "/")
	trackerID := parts[0]
	trk, err := h.db.GetTrackerByID(trackerID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if trk == nil {
		writeError(w, http.StatusNotFound, "Tracker non trouvé")
		return
	}
	if !caller.IsAdmin() && !h.db.TrackerHasProjects(trk.ID) {
		writeError(w, http.StatusForbidden, "Aucun de vos projets ne sélectionne ce tracker")
		return
	}

	switch {
	case len(parts) == 2 && parts[1] == "sync" && r.Method == http.MethodPost:
		activity, err := h.db.EnqueueTrackerSyncWith(caller.UserID, trk.ID, db.SyncOptions{})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"queued": true, "activity": activity})
	case len(parts) == 2 && parts[1] == "backlog" && r.Method == http.MethodGet:
		tasks, err := h.db.GetTrackerBacklog(trk.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, tasks)
	case len(parts) == 4 && parts[1] == "backlog" && parts[3] == "project" && r.Method == http.MethodPost:
		var payload struct {
			ProjectID string `json:"projectId"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid payload: "+err.Error())
			return
		}
		task, err := h.db.GetTaskByID(parts[2])
		if writeTaskKeyAmbiguous(w, err) {
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if task == nil || task.TrackerID != trk.ID {
			writeError(w, http.StatusNotFound, "Ticket non trouvé dans ce tracker")
			return
		}
		task, activity, err := h.db.AddTaskToProjectAs(h.actingContext(r), task.ID, payload.ProjectID)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, db.ErrProjectNotOnTracker) || errors.Is(err, db.ErrProjectWithoutLabel) {
				status = http.StatusConflict
			}
			writeError(w, status, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"task": task, "activity": activity})
	default:
		writeError(w, http.StatusNotFound, "Not found")
	}
}

// HandleAdminTrackers serves the admin routes:
//
//	GET    /api/admin/trackers                         every tracker
//	POST   /api/admin/trackers                         record one
//	GET    /api/admin/trackers/{id}                    one tracker
//	PUT    /api/admin/trackers/{id}                    rewrite it
//	DELETE /api/admin/trackers/{id}                    delete one no project or ticket uses
//	GET    /api/admin/trackers/{id}/boards             its boards
//	POST   /api/admin/trackers/{id}/board-columns      {boardId}: import a board's columns
//	GET    /api/admin/trackers/{id}/tracker-statuses   its workflow statuses
//	GET    /api/admin/trackers/{id}/issue-types        its work item types
//	GET    /api/admin/trackers/{id}/detected-statuses  the statuses seen on its tickets
//
// The route table refuses members first; the handler checks again so the rule
// does not depend on the table alone.
func (h *Handler) HandleAdminTrackers(w http.ResponseWriter, r *http.Request) {
	caller, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, AdminTrackersPath), "/")
	if rest == "" {
		switch r.Method {
		case http.MethodGet:
			trackers, err := h.db.GetTrackers()
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			list := []*models.Tracker{}
			for _, t := range trackers {
				if t.Provider != "local" {
					list = append(list, t)
				}
			}
			writeJSON(w, http.StatusOK, list)
		case http.MethodPost:
			var t models.Tracker
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&t); err != nil {
				writeError(w, http.StatusBadRequest, "Invalid payload: "+err.Error())
				return
			}
			created, err := h.db.CreateTrackerAs(caller.UserID, t)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, created)
		default:
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
		return
	}

	parts := strings.Split(rest, "/")
	trackerID := parts[0]
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			t, err := h.db.GetTrackerByID(trackerID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if t == nil {
				writeError(w, http.StatusNotFound, "Tracker non trouvé")
				return
			}
			writeJSON(w, http.StatusOK, t)
		case http.MethodPut:
			var t models.Tracker
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&t); err != nil {
				writeError(w, http.StatusBadRequest, "Invalid payload: "+err.Error())
				return
			}
			t.ID = trackerID
			updated, err := h.db.UpdateTrackerAs(caller.UserID, t)
			if err != nil {
				status := http.StatusBadRequest
				if errors.Is(err, db.ErrTrackerSourceInUse) {
					status = http.StatusConflict
				}
				writeError(w, status, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, updated)
		case http.MethodDelete:
			if err := h.db.DeleteTrackerAs(caller.UserID, trackerID); err != nil {
				status := http.StatusBadRequest
				if errors.Is(err, db.ErrTrackerInUse) {
					status = http.StatusConflict
				}
				writeError(w, status, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
		default:
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
		return
	}

	ctx := h.actingContext(r)
	var result any
	var err error
	switch {
	case parts[1] == "boards" && r.Method == http.MethodGet:
		result, err = h.db.ListTrackerBoardsAs(ctx, trackerID)
	case parts[1] == "board-columns" && r.Method == http.MethodPost:
		var payload struct {
			BoardID string `json:"boardId"`
		}
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&payload)
		result, err = h.db.ImportTrackerBoardColumns(ctx, trackerID, payload.BoardID)
	case parts[1] == "tracker-statuses" && r.Method == http.MethodGet:
		result, err = h.db.GetTrackerStatusesAs(ctx, trackerID)
	case parts[1] == "issue-types" && r.Method == http.MethodGet:
		result, err = h.db.ListTrackerIssueTypesAs(ctx, trackerID)
	case parts[1] == "detected-statuses" && r.Method == http.MethodGet:
		t, terr := h.db.GetTrackerByID(trackerID)
		if terr != nil || t == nil {
			writeError(w, http.StatusNotFound, "Tracker non trouvé")
			return
		}
		result, err = h.db.DetectStatusesForTracker(ctx, t.ID)
	default:
		writeError(w, http.StatusNotFound, "Not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}
