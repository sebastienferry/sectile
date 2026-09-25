package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"tasks/internal/db"
)

// ServerTrackerCredentialsPath is where an admin manages the server credential
// of each tracker provider (#464, ADR 0028).
const ServerTrackerCredentialsPath = "/api/admin/tracker-credentials"

// HandleServerTrackerCredentials is the Administration page's view of the
// server tracker credentials:
//
//	GET    /api/admin/tracker-credentials                  every provider's state
//	POST   /api/admin/tracker-credentials/{tracker}/check  {email?, token?}
//	PUT    /api/admin/tracker-credentials/{tracker}        {email?, token}
//	DELETE /api/admin/tracker-credentials/{tracker}
//
// All of it is an admin's: the route table refuses members first, and the
// handler checks again so the rule does not depend on the table alone. No
// answer ever carries a token.
//
// A save is checked against the instance before anything is stored, like the
// tracker setup: a wrong token never replaces a working one. An empty token in
// a check means the credential already in use, stored or from the environment.
func (h *Handler) HandleServerTrackerCredentials(w http.ResponseWriter, r *http.Request) {
	caller, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, ServerTrackerCredentialsPath), "/")
	if rest == "" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		states, err := h.db.ServerTrackerCredentialStates()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, states)
		return
	}

	tracker, action, _ := strings.Cut(rest, "/")
	tracker = strings.ToLower(tracker)
	if !db.IsServerCredentialTracker(tracker) || (action != "" && action != "check") {
		writeError(w, http.StatusNotFound, "Aucun accès serveur pour ce tracker")
		return
	}

	var payload struct {
		Email string `json:"email"`
		Token string `json:"token"`
	}
	if r.Method == http.MethodPost || r.Method == http.MethodPut {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid payload: "+err.Error())
			return
		}
	}
	payload.Email, payload.Token = strings.TrimSpace(payload.Email), strings.TrimSpace(payload.Token)

	switch {
	case action == "check" && r.Method == http.MethodPost:
		account, err := h.db.CheckServerTrackerCredentials(r.Context(), tracker, "", payload.Email, payload.Token)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		// A successful check of the credential in use dates it, so the page
		// can say when it was last known to work.
		if payload.Token == "" {
			if err := h.db.RecordServerCredentialCheck(tracker, account); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "account": account})

	case action == "" && r.Method == http.MethodPut:
		if payload.Token == "" {
			writeError(w, http.StatusBadRequest, "Le jeton est requis")
			return
		}
		if tracker == "jira" && payload.Email == "" {
			writeError(w, http.StatusBadRequest, "L'e-mail du compte Jira est requis avec son jeton")
			return
		}
		account, err := h.db.CheckServerTrackerCredentials(r.Context(), tracker, "", payload.Email, payload.Token)
		if err != nil {
			// Nothing is stored on a failed check: the credential in use keeps
			// working.
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.db.SaveServerTrackerCredential(tracker, payload.Email, payload.Token, account, caller.UserID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.writeServerCredentialState(w, tracker)

	case action == "" && r.Method == http.MethodDelete:
		if err := h.db.ClearServerTrackerCredential(tracker); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.writeServerCredentialState(w, tracker)

	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *Handler) writeServerCredentialState(w http.ResponseWriter, tracker string) {
	state, err := h.db.ServerTrackerCredentialState(tracker)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, state)
}
