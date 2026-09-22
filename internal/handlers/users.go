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
// PUT /api/users/{id} changes one role or opens and closes it, and DELETE
// /api/users/{id} removes it. They are admin-only; the guard refuses members
// before the handler runs, and the handler checks again so the rule does not
// depend on the route table alone.
func (h *Handler) HandleUsers(w http.ResponseWriter, r *http.Request) {
	caller, ok := h.requireAdmin(w, r)
	if !ok {
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
		// Both fields are pointers so that an absent one is left alone: the view
		// changes a role or a block, one at a time, and a zero value would
		// otherwise read as "demote" or "open".
		var payload struct {
			Role    *string `json:"role"`
			Blocked *bool   `json:"blocked"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid user request")
			return
		}
		if payload.Role == nil && payload.Blocked == nil {
			writeError(w, http.StatusBadRequest, "Name a role or a blocked state to change")
			return
		}
		var user *db.User
		if payload.Role != nil {
			if !db.ValidRole(*payload.Role) {
				writeError(w, http.StatusBadRequest, "role must be admin or member")
				return
			}
			updated, err := h.db.SetUserRole(id, *payload.Role)
			if !writeUserError(w, err, "This is the last admin: promote someone else first, or the board would have no admin") {
				return
			}
			user = updated
		}
		if payload.Blocked != nil {
			// Blocking yourself is refused rather than handled: it ends your own
			// session, and the account that could undo it is the one just closed.
			if *payload.Blocked && id == caller.UserID {
				writeError(w, http.StatusConflict, "You cannot block your own account")
				return
			}
			updated, err := h.db.SetUserBlocked(id, *payload.Blocked)
			if !writeUserError(w, err, "This is the last admin: promote someone else first, or the board would have no admin") {
				return
			}
			user = updated
		}
		writeJSON(w, http.StatusOK, user)
	case r.Method == http.MethodDelete && id != "":
		if id == caller.UserID {
			writeError(w, http.StatusConflict, "You cannot delete your own account")
			return
		}
		err := h.db.DeleteUser(id)
		if !writeUserError(w, err, "This is the last admin: promote someone else first, or the board would have no admin") {
			return
		}
		// What the account owned is not deleted with it: its tasks, comments and
		// past executions stay on the board and read as having no owner.
		writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// writeUserError answers the refusals the three account operations share and
// reports whether the caller may carry on. Each has a remedy the person can
// act on, which is why none of them is served as a bare 500.
func writeUserError(w http.ResponseWriter, err error, lastAdmin string) bool {
	switch {
	case err == nil:
		return true
	case errors.Is(err, db.ErrLastAdmin):
		writeError(w, http.StatusConflict, lastAdmin)
	case errors.Is(err, db.ErrImplicitUser):
		writeError(w, http.StatusConflict, "This account is not a person: it owns everything that runs without a session")
	case errors.Is(err, sql.ErrNoRows):
		writeError(w, http.StatusNotFound, "Unknown user")
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
	return false
}
