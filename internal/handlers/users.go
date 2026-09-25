package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

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
		rows, err := h.usersWithActivity(users)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"users": rows,
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

// AdminStatsPath is the admin page's summary of the board.
const AdminStatsPath = "/api/admin/stats"

// userRow is an account as the admin's users view lists it: the stored row,
// plus when it last used a browser session and whether that was recent enough
// to count as active.
type userRow struct {
	db.User
	LastActiveAt *time.Time `json:"lastActiveAt,omitempty"`
	Active       bool       `json:"active"`
}

// usersWithActivity joins each account to its last browser activity. A blocked
// account is never active: its sessions were revoked with the block.
func (h *Handler) usersWithActivity(users []db.User) ([]userRow, error) {
	activity, err := h.db.UserActivity()
	if err != nil {
		return nil, err
	}
	cutoff := time.Now().UTC().Add(-db.ActiveUserWindow)
	rows := make([]userRow, 0, len(users))
	for _, user := range users {
		row := userRow{User: user}
		if seen, ok := activity[user.ID]; ok {
			row.LastActiveAt = &seen
			row.Active = !user.Blocked && !seen.Before(cutoff)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// HandleAdminStats answers GET /api/admin/stats: the roster at a glance, how
// many people are using the board right now, and how many runs are not over.
// Every figure is read from the database, so each instance sharing it answers
// the same.
func (h *Handler) HandleAdminStats(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	users, err := h.db.CountUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	active, err := h.db.ActiveUserCount(db.ActiveUserWindow)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	runs, err := h.db.ActiveRunCounts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	total := 0
	for _, count := range runs {
		total += count
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"users": map[string]any{
			"total":   users.Total,
			"admins":  users.Admins,
			"blocked": users.Blocked,
			"active":  active,
		},
		"runs": map[string]any{
			"active":   total,
			"byStatus": runs,
		},
		"activeWindowSeconds": int(db.ActiveUserWindow / time.Second),
		"generatedAt":         time.Now().UTC(),
	})
}
