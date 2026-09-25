package handlers

import (
	"errors"
	"net/http"
	"slices"
	"strings"

	"tasks/internal/db"
)

// myTasks is who "me" is for the request's caller (#468): the confirmed
// account of each personal tracker credential, and the account's name and
// e-mail for everything else. Signed out, the name and e-mail are the local
// profile's. Nothing here reaches a tracker.
func (h *Handler) myTasks(r *http.Request) (*db.MyTasks, error) {
	userID := h.webSessionUser(r)
	settings, err := h.composedSettings(userID)
	if err != nil {
		return nil, err
	}
	mine := &db.MyTasks{ByTracker: map[string]string{}}
	for _, identity := range []string{settings.UserName, settings.UserEmail} {
		if strings.TrimSpace(identity) != "" && !slices.ContainsFunc(mine.Fallback, func(kept string) bool {
			return strings.EqualFold(strings.TrimSpace(kept), strings.TrimSpace(identity))
		}) {
			mine.Fallback = append(mine.Fallback, identity)
		}
	}
	if userID != "" {
		if mine.ByTracker, err = h.db.TrackerAccounts(userID); err != nil {
			return nil, err
		}
	}
	return mine, nil
}

// HandleAssigneeIdentities says who My Tasks takes the caller to be on each
// tracker of a scope, so the button can tell when it had to fall back on the
// account's name and e-mail.
//
//	GET /api/me/assignee-identities?projectId=|viewId=
//
// It answers a signed-out caller too, with the local profile: the board works
// without an account, and My Tasks with it.
func (h *Handler) HandleAssigneeIdentities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	userID := h.webSessionUser(r)
	mine, err := h.myTasks(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	scope := db.TaskScope{UserID: userID, ProjectID: r.URL.Query().Get("projectId"), ViewID: r.URL.Query().Get("viewId")}
	sources, err := h.db.TaskSourcesInScope(scope)
	if errors.Is(err, db.ErrBoardViewNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type trackerIdentity struct {
		Tracker  string `json:"tracker"`
		Identity string `json:"identity,omitempty"`
		Known    bool   `json:"known"`
	}
	trackers := []trackerIdentity{}
	for _, source := range sources {
		if source == "local" {
			continue
		}
		identity := mine.ByTracker[source]
		trackers = append(trackers, trackerIdentity{Tracker: source, Identity: identity, Known: identity != ""})
	}
	fallback := mine.Fallback
	if fallback == nil {
		fallback = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"signedIn": userID != "",
		"fallback": fallback,
		"trackers": trackers,
	})
}
