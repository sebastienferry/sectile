package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"tasks/internal/db"
	"tasks/internal/secrets"
)

// A tracker credential is personal wherever the tracker attributes a write to
// the account its token belongs to, which Jira does. These routes let each
// person hold their own, and never hand one back: the answer carries what is
// stored about a credential, never the credential.
//
//	GET    /api/me/tracker-credentials          what this person stored
//	PUT    /api/me/tracker-credentials          store or replace one
//	DELETE /api/me/tracker-credentials?tracker= forget one
//	POST   /api/me/tracker-credentials/unlock   supply the sealing passphrase
//	POST   /api/me/tracker-credentials/lock     forget the derived key
func (h *Handler) HandleUserTrackerCredentials(w http.ResponseWriter, r *http.Request) {
	userID := h.webSessionUser(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "Connectez-vous avant d'enregistrer un accès personnel")
		return
	}

	action := strings.TrimPrefix(strings.TrimSuffix(r.URL.Path, "/"), "/api/me/tracker-credentials")
	action = strings.Trim(action, "/")

	switch {
	case action == "" && r.Method == http.MethodGet:
		h.listUserCredentials(w, userID)

	case action == "" && (r.Method == http.MethodPut || r.Method == http.MethodPost):
		var req struct {
			Tracker string `json:"tracker"`
			// SiteURL belongs here because an Atlassian account belongs to a
			// site: one person's instance is not a server-wide setting.
			SiteURL string `json:"siteUrl"`
			Email   string `json:"email"`
			Token   string `json:"token"`
			// Passphrase seals the credential. Empty leaves it openable by the
			// server, which is what lets it serve a request the owner did not
			// start. The passphrase itself is never stored.
			Passphrase string `json:"passphrase"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Payload illisible : "+err.Error())
			return
		}
		if err := h.db.SetUserTrackerCredential(userID, req.Tracker, req.SiteURL, req.Email, req.Token, req.Passphrase); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.listUserCredentials(w, userID)

	case action == "" && r.Method == http.MethodDelete:
		switch err := h.db.ClearUserTrackerCredential(userID, r.URL.Query().Get("tracker")); {
		case errors.Is(err, db.ErrNoUserCredential):
			writeError(w, http.StatusNotFound, "Aucun accès personnel enregistré pour ce tracker")
			return
		case err != nil:
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.listUserCredentials(w, userID)

	case action == "unlock" && r.Method == http.MethodPost:
		var req struct {
			Tracker    string `json:"tracker"`
			Passphrase string `json:"passphrase"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Payload illisible : "+err.Error())
			return
		}
		switch err := h.db.UnlockUserTrackerCredential(userID, req.Tracker, req.Passphrase); {
		case errors.Is(err, secrets.ErrWrongKey):
			// The same answer whatever is wrong: an attacker learns nothing
			// from the difference between a bad phrase and a missing record.
			writeError(w, http.StatusForbidden, "Phrase de scellement refusée. Si vous l'avez perdue, enregistrez de nouveau votre jeton : cela remplace la phrase.")
			return
		case errors.Is(err, db.ErrNoUserCredential):
			writeError(w, http.StatusNotFound, "Aucun accès personnel enregistré pour ce tracker")
			return
		case errors.Is(err, db.ErrNotSealed):
			writeError(w, http.StatusConflict, "Ce jeton n'est pas scellé : il n'y a rien à desceller.")
			return
		case err != nil:
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.listUserCredentials(w, userID)

	case action == "lock" && r.Method == http.MethodPost:
		var req struct {
			Tracker string `json:"tracker"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		h.db.LockUserTrackerCredential(userID, req.Tracker)
		h.listUserCredentials(w, userID)

	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *Handler) listUserCredentials(w http.ResponseWriter, userID string) {
	credentials, err := h.db.UserTrackerCredentials(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"credentials": credentials})
}
