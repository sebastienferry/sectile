package handlers

import (
	"encoding/json"
	"errors"
	"log"
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
//	GET    /api/me/tracker-credentials           what this person stored
//	PUT    /api/me/tracker-credentials           store or replace one
//	DELETE /api/me/tracker-credentials?tracker=  forget one
//	POST   /api/me/tracker-credentials/unlock    supply the sealing passphrase
//	POST   /api/me/tracker-credentials/lock      forget the derived key
//	DELETE /api/me/tracker-credentials/orphaned  discard a leftover, admin only
//
// The GET also reports the credentials stored under an identity no account
// resolves, because that is the answer to "my tracker says to configure an
// e-mail I did configure". See internal/db/orphancredentials.go.
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
		h.listUserCredentials(w, r, userID)

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
		// The tracker is asked whom the credential belongs to, which is who
		// "me" is on it for My Tasks (#468). A failed answer keeps the save:
		// the form already checked it, and the profile can verify it again.
		if _, err := h.db.ConfirmUserTrackerCredential(r.Context(), userID, req.Tracker); err != nil {
			log.Printf("[TrackerCredentials] compte %s non confirmé : %v", req.Tracker, err)
		}
		h.listUserCredentials(w, r, userID)

	case action == "" && r.Method == http.MethodDelete:
		switch err := h.db.ClearUserTrackerCredential(userID, r.URL.Query().Get("tracker")); {
		case errors.Is(err, db.ErrNoUserCredential):
			writeError(w, http.StatusNotFound, "Aucun accès personnel enregistré pour ce tracker")
			return
		case err != nil:
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.listUserCredentials(w, r, userID)

	case action == "unlock" && r.Method == http.MethodPost:
		var req struct {
			Tracker    string `json:"tracker"`
			Passphrase string `json:"passphrase"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Payload illisible : "+err.Error())
			return
		}
		if strings.TrimSpace(req.Tracker) == "" || req.Tracker == "*" {
			creds, err := h.db.UserTrackerCredentials(userID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			unlockedCount := 0
			for _, c := range creds {
				if c.Sealed && !c.Unlocked {
					if err := h.db.UnlockUserTrackerCredential(userID, c.Tracker, req.Passphrase); err != nil {
						if errors.Is(err, secrets.ErrWrongKey) {
							writeError(w, http.StatusForbidden, "Phrase de scellement refusée.")
							return
						}
						writeError(w, http.StatusInternalServerError, err.Error())
						return
					}
					unlockedCount++
				}
			}
			h.listUserCredentials(w, r, userID)
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
		h.listUserCredentials(w, r, userID)

	case action == "orphaned" && r.Method == http.MethodDelete:
		// Removing a leftover is a deployment decision, not a personal one, so
		// it is an admin's. The storage layer refuses any target that still has
		// an owner, which is what stops this from being a way to delete a
		// colleague's token by naming their id.
		if _, ok := h.requireAdmin(w, r); !ok {
			return
		}
		switch err := h.db.DiscardOrphanedTrackerCredential(r.URL.Query().Get("userId"), r.URL.Query().Get("tracker")); {
		case errors.Is(err, db.ErrNoUserCredential):
			writeError(w, http.StatusNotFound, "Aucun accès enregistré sous cette identité pour ce tracker")
			return
		case errors.Is(err, db.ErrCredentialNotOrphaned):
			writeError(w, http.StatusConflict, "Cet accès appartient à un compte existant : son propriétaire est seul à pouvoir le supprimer.")
			return
		case err != nil:
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.listUserCredentials(w, r, userID)

	case action == "lock" && r.Method == http.MethodPost:
		var req struct {
			Tracker string `json:"tracker"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if strings.TrimSpace(req.Tracker) == "" || req.Tracker == "*" {
			creds, _ := h.db.UserTrackerCredentials(userID)
			for _, c := range creds {
				h.db.LockUserTrackerCredential(userID, c.Tracker)
			}
		} else {
			h.db.LockUserTrackerCredential(userID, req.Tracker)
		}
		h.listUserCredentials(w, r, userID)

	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *Handler) listUserCredentials(w http.ResponseWriter, r *http.Request, userID string) {
	credentials, err := h.db.UserTrackerCredentials(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	body := map[string]interface{}{"credentials": credentials}

	// Everyone signed in is told a leftover exists and for which tracker: it is
	// why their own access looks absent while the tracker behaves as if one was
	// configured, and they can act on it themselves by registering their token.
	// Who it is registered under, and for which account, is an admin's business,
	// as is discarding it.
	orphans, err := h.db.OrphanedTrackerCredentials()
	if err == nil && len(orphans) > 0 {
		trackers := make([]string, 0, len(orphans))
		for _, orphan := range orphans {
			trackers = append(trackers, orphan.Tracker)
		}
		body["orphanedCount"] = len(orphans)
		body["orphanedTrackers"] = trackers
		if h.webPrincipal(r).IsAdmin() {
			body["orphaned"] = orphans
		}
	}
	writeJSON(w, http.StatusOK, body)
}
