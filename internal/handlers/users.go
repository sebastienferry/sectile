package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"tasks/internal/db"
)

// HandleUsers is the admin's users view: GET /api/users lists every account,
// PUT /api/users/{id} changes one role. Both are admin-only; the guard refuses
// members before the handler runs, and the handler checks again so the rule
// does not depend on the route table alone.
func (h *Handler) HandleUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/users"), "/")
	switch {
	case r.Method == http.MethodGet && id == "":
		users, err := h.db.ListUsers()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"users": users,
			// The provider's claim overrides a manual change at the user's next
			// sign-in; the view says so rather than letting the change look final.
			"rolesFromProvider": h.identityProvider != nil && h.identityProvider.SuppliesRoles(),
		})
	case (r.Method == http.MethodPut || r.Method == http.MethodPatch) && id != "":
		var payload struct {
			Role string `json:"role"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid role request")
			return
		}
		if !db.ValidRole(payload.Role) {
			writeError(w, http.StatusBadRequest, "role must be admin or member")
			return
		}
		user, err := h.db.SetUserRole(id, payload.Role)
		switch {
		case errors.Is(err, db.ErrLastAdmin):
			writeError(w, http.StatusConflict, "This is the last admin: promote someone else first, or the board would have no admin")
			return
		case errors.Is(err, sql.ErrNoRows):
			writeError(w, http.StatusNotFound, "Unknown user")
			return
		case err != nil:
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, user)
	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}
