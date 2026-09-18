package handlers

import (
	"context"

	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"tasks/internal/tracker"
	"time"

	"tasks/internal/db"
)

// apiKeyRequest is the body of a key creation or renewal. TTLDays absent means
// the default period; zero means a key without expiry.
type apiKeyRequest struct {
	Label   string `json:"label"`
	TTLDays *int   `json:"ttlDays"`
}

// ttl turns the requested period into a duration.
func (r apiKeyRequest) ttl() time.Duration {
	if r.TTLDays == nil {
		return db.DefaultAPIKeyTTL
	}
	if *r.TTLDays <= 0 {
		return 0
	}
	return time.Duration(*r.TTLDays) * 24 * time.Hour
}

// ImplicitUser owns what runs without an HTTP request, the local agent's own
// operations and the launches started outside a browser session. No HTTP path
// resolves to it any more (ADR 0015): it is an ordinary account, named here so
// those requestless entry points do not carry a bare string.
const ImplicitUser = "default"

// webSessionUser identifies the caller of an interface request: whoever holds
// a valid session cookie, opened by the identity provider's callback or by the
// local e-mail sign-in.
//
// A workstation API key answers for the machine surfaces that reach the same
// routes: the agent gateway forwards the consoles' `/api/` calls with the key
// rather than with a cookie, and that key names a user just as a session does.
// Only a real key counts here, never the deprecated shared token nor the legacy
// open mode where any value named the implicit user: those would hand anyone a
// way past sign-in by setting one header.
//
// Without either, the caller is a stranger and the answer is empty: signing in
// is mandatory on every deployment (ADR 0015), including a fresh one, whose
// board only opens once someone has signed in.
// actingContext is the request's context marked with whoever is making it, for
// the store calls that reach a tracker: on a tracker that attributes its writes
// to the account behind the token, this is what puts the right name on them.
func (h *Handler) actingContext(r *http.Request) context.Context {
	return tracker.WithActingUser(r.Context(), h.webSessionUser(r))
}

func (h *Handler) webSessionUser(r *http.Request) string {
	if cookie, err := r.Cookie(sessionCookie); err == nil && h.db != nil {
		if userID := h.db.UserForWebSession(cookie.Value); userID != "" {
			return userID
		}
	}
	if token := bearerToken(r); token != "" && h.db != nil {
		if credential, err := h.resolveAgentCredential(token); err == nil && credential.Device != nil {
			return credential.UserID
		}
	}
	return ""
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
	// The row exists for anyone who signed in; the implicit user has none, and
	// the pairing code's foreign key needs one.
	if err := h.db.EnsureUser(userID); err != nil {
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

// HandleDeviceCredentials manages the signed-in user's API keys: GET lists
// them, POST creates one and returns its plaintext exactly once, PUT renews or
// clears the expiry of one, DELETE revokes one. An admin may name another user
// with ?userId= to list or revoke that user's workstations.
func (h *Handler) HandleDeviceCredentials(w http.ResponseWriter, r *http.Request) {
	caller := h.webPrincipal(r)
	if caller.Anonymous() {
		writeError(w, http.StatusUnauthorized, "Sign in to manage workstations")
		return
	}
	userID := caller.UserID
	if other := strings.TrimSpace(r.URL.Query().Get("userId")); other != "" && other != caller.UserID {
		if !caller.IsAdmin() {
			writeError(w, http.StatusForbidden, msgAdminOnly)
			return
		}
		// An admin sees and revokes someone else's workstations. Minting or
		// renewing a key in their name is another thing entirely: it would hand
		// the admin a durable credential that acts as that person, which no
		// part of this feature asks for.
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			writeError(w, http.StatusForbidden, "A workstation key can only be created or renewed by its own owner")
			return
		}
		userID = other
	}
	switch r.Method {
	case http.MethodPost:
		if err := h.db.EnsureUser(userID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		var payload apiKeyRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid API key request")
			return
		}
		token, credential, err := h.db.CreateAPIKey(userID, payload.Label, payload.ttl())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"token":  token,
			"device": credential,
		})
	case http.MethodPut:
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" {
			writeError(w, http.StatusBadRequest, "A device id is required")
			return
		}
		var payload apiKeyRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid API key request")
			return
		}
		expires, err := h.db.RenewDeviceCredential(userID, id, payload.ttl())
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"renewed": id, "expiresAt": expires})
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
