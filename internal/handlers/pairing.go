package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"tasks/internal/db"
)

// ImplicitUser is the single user a deployment has before an identity provider
// is configured. Paths that run without an HTTP request name it explicitly,
// so what still has to change for real multi-user is visible rather than
// spread through the code as a bare string.
const ImplicitUser = "default"

// webSessionUser identifies the person driving the web interface.
//
// With an OpenID Connect provider configured, that is whoever holds a valid
// session cookie. Without one the interface has a single implicit user, which
// is how a personal deployment runs.
//
// SECTILE_DEV_IDENTITY=1 lets a caller name itself through the X-Sectile-User
// header, so several users can be exercised before any provider exists. It is
// an impersonation switch, off unless the deployment sets it: leaving it on
// once real sign-in exists would let anyone claim any identity.
func (h *Handler) webSessionUser(r *http.Request) string {
	// A configured provider is the only authority: the development switch is
	// ignored rather than left as a way around real sign-in.
	if h.identityProvider != nil {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			return ""
		}
		return h.db.UserForWebSession(cookie.Value)
	}
	if os.Getenv("SECTILE_DEV_IDENTITY") == "1" {
		if claimed := strings.TrimSpace(r.Header.Get("X-Sectile-User")); claimed != "" {
			userID, err := h.db.UpsertUser("dev|"+claimed, "", claimed)
			if err != nil {
				log.Printf("[Identity] Development identity %q refused: %v", claimed, err)
				return ""
			}
			return userID
		}
	}
	return ImplicitUser
}

// HandlePairingCode issues a single-use pairing code for the signed-in user.
// The code travels to the desktop app by hand, so it is short lived and worth
// nothing on its own: it only buys a device credential, once.
func (h *Handler) HandlePairingCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	userID := h.webSessionUser(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "Sign in before pairing a workstation")
		return
	}
	if _, err := h.db.UpsertUser(userID, "", ""); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	code, expires, err := h.db.CreatePairingCode(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"code":      code,
		"expiresAt": expires,
	})
}

// HandleAgentPair exchanges a pairing code for a device credential. It is the
// only agent endpoint that is not itself authenticated: the code is the proof.
func (h *Handler) HandleAgentPair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		writeError(w, http.StatusForbidden, "Browser origins are not allowed on agent APIs")
		return
	}
	var payload struct {
		Code  string `json:"code"`
		Label string `json:"label"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid pairing request")
		return
	}
	if strings.TrimSpace(payload.Code) == "" {
		writeError(w, http.StatusBadRequest, "A pairing code is required")
		return
	}
	token, credential, err := h.db.RedeemPairingCode(payload.Code, strings.TrimSpace(payload.Label))
	if errors.Is(err, db.ErrPairingCode) {
		// One message for unknown, consumed and expired codes: which of the
		// three it is would tell an attacker whether a code ever existed.
		writeError(w, http.StatusUnauthorized, "Invalid or expired pairing code")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"token":    token,
		"deviceId": credential.ID,
		"userId":   credential.UserID,
	})
}

// HandleDeviceCredentials lists and revokes the signed-in user's workstations.
func (h *Handler) HandleDeviceCredentials(w http.ResponseWriter, r *http.Request) {
	userID := h.webSessionUser(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "Sign in to manage workstations")
		return
	}
	switch r.Method {
	case http.MethodGet:
		credentials, err := h.db.ListDeviceCredentials(userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if credentials == nil {
			credentials = []db.DeviceCredential{}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"devices": credentials})
	case http.MethodDelete:
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" {
			writeError(w, http.StatusBadRequest, "A device id is required")
			return
		}
		if err := h.db.RevokeDeviceCredential(userID, id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"revoked": id})
	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}
