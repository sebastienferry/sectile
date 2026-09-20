package handlers

import (
	"net/http"
	"strings"
)

// HandleUserProjectBookmarks serves user-specific project bookmarks.
//
//	GET    /api/me/project-bookmarks               list bookmarked project IDs
//	PUT    /api/me/project-bookmarks/{id}          bookmark a project
//	DELETE /api/me/project-bookmarks/{id}          unbookmark a project
//	POST   /api/me/project-bookmarks/{id}/toggle   toggle bookmark
func (h *Handler) HandleUserProjectBookmarks(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requireSession(w, r)
	if !ok {
		return
	}

	rawPath := strings.TrimPrefix(r.URL.Path, "/api/me/project-bookmarks")
	rawPath = strings.Trim(rawPath, "/")

	if rawPath == "" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		bookmarks, err := h.db.GetUserProjectBookmarks(p.UserID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, bookmarks)
		return
	}

	parts := strings.Split(rawPath, "/")
	projectID := parts[0]
	if projectID == "" {
		writeError(w, http.StatusBadRequest, "Project ID is required")
		return
	}

	if len(parts) == 2 && parts[1] == "toggle" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		bookmarked, err := h.db.ToggleProjectBookmark(p.UserID, projectID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"bookmarked": bookmarked})
		return
	}

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodPut:
			if err := h.db.BookmarkProject(p.UserID, projectID); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]bool{"bookmarked": true})
			return
		case http.MethodDelete:
			if err := h.db.UnbookmarkProject(p.UserID, projectID); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]bool{"bookmarked": false})
			return
		case http.MethodPost:
			if r.URL.Query().Get("action") == "toggle" {
				bookmarked, err := h.db.ToggleProjectBookmark(p.UserID, projectID)
				if err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, map[string]bool{"bookmarked": bookmarked})
				return
			}
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		default:
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
	}

	writeError(w, http.StatusNotFound, "Not found")
}
