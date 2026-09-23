package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"tasks/internal/db"
	"tasks/internal/models"
)

// maxBoardViewBody bounds what a view definition may weigh: a name, a few
// dozen labels and the board's project ids fit many times over.
const maxBoardViewBody = 64 << 10

// HandleBoardViews serves the signed-in user's saved board views (#387).
//
//	GET    /api/me/board-views        list the user's views
//	POST   /api/me/board-views        create one: {name, projectIds, labels}
//	GET    /api/me/board-views/{id}   read one
//	PATCH  /api/me/board-views/{id}   change the fields sent
//	DELETE /api/me/board-views/{id}   delete one
//
// A view belongs to its owner alone: another user's view answers 404, the same
// answer as a view that does not exist.
func (h *Handler) HandleBoardViews(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requireSession(w, r)
	if !ok {
		return
	}

	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/me/board-views"), "/")
	if strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "Not found")
		return
	}

	if id == "" {
		switch r.Method {
		case http.MethodGet:
			views, err := h.db.ListBoardViews(p.UserID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, views)
		case http.MethodPost:
			var req models.BoardViewRequest
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBoardViewBody)).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "Invalid JSON body")
				return
			}
			view, err := h.db.CreateBoardView(p.UserID, req)
			if err != nil {
				writeBoardViewError(w, err)
				return
			}
			writeJSON(w, http.StatusCreated, view)
		default:
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
		return
	}

	switch r.Method {
	case http.MethodGet:
		view, err := h.db.GetBoardView(p.UserID, id)
		if err != nil {
			writeBoardViewError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, view)
	case http.MethodPatch:
		var req models.BoardViewRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBoardViewBody)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid JSON body")
			return
		}
		view, err := h.db.UpdateBoardView(p.UserID, id, req)
		if err != nil {
			writeBoardViewError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, view)
	case http.MethodDelete:
		if err := h.db.DeleteBoardView(p.UserID, id); err != nil {
			writeBoardViewError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// writeBoardViewError maps the storage errors of a view to their status. The
// messages are the ones the interface shows as they are.
func writeBoardViewError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, db.ErrBoardViewNotFound):
		writeError(w, http.StatusNotFound, db.ErrBoardViewNotFound.Error())
	case errors.Is(err, db.ErrBoardViewNameTaken):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, db.ErrBoardViewNameRequired),
		errors.Is(err, db.ErrBoardViewNoProject),
		errors.Is(err, db.ErrBoardViewTooLarge),
		errors.Is(err, db.ErrBoardViewUnknownProject):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
