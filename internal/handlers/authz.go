package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"tasks/internal/auth"
	"tasks/internal/db"
	"tasks/internal/models"
)

// modeImplicit is the sign-in mode of a deployment without a provider and
// without a local account yet: the single implicit user of ADR 0008, who holds
// the admin role so a personal deployment is never locked out of anything.
const modeImplicit = "implicit"

// principal is who a request comes from, resolved once and read by every
// authorization check. No handler compares user ids by hand or reads the role
// column itself: the two roles and the one ownership rule live here.
type principal struct {
	UserID string
	Role   string
	// Mode is the deployment's sign-in mode: oidc, local or implicit.
	Mode string
	Name string
}

// Anonymous reports a request with no identity at all.
func (p principal) Anonymous() bool { return p.UserID == "" }

// IsAdmin reports the admin role.
func (p principal) IsAdmin() bool { return p.Role == db.RoleAdmin }

// Actor is the principal as the storage layer records it.
func (p principal) Actor() db.Actor { return db.Actor{ID: p.UserID, Name: p.Name} }

// Messages of the three refusals, stable so the interface can tell them apart.
const (
	msgSignIn    = "Sign in to use this interface"
	msgAdminOnly = "This action is reserved to admins"
	msgNotOwner  = "Only the owner of this execution or an admin can act on it"
)

// signInMode is the deployment's mode. A configured provider is the only
// authority; without one the local e-mail sign-in identifies people, except
// while nobody has an account yet, where the implicit user keeps the board open.
func (h *Handler) signInMode() string {
	if h.identityProvider != nil {
		return auth.ModeOIDC
	}
	if h.db != nil && h.db.HasLocalAccounts() {
		return auth.ModeLocal
	}
	return modeImplicit
}

// principalFor resolves a user id to its principal. The implicit user is
// always an admin; an unknown id is a member, the safe reading of a bad row.
func (h *Handler) principalFor(userID string) principal {
	p := principal{UserID: strings.TrimSpace(userID), Mode: h.signInMode()}
	if p.UserID == "" {
		return p
	}
	if p.UserID == ImplicitUser {
		p.Role = db.RoleAdmin
		return p
	}
	p.Role = db.RoleMember
	if h.db != nil {
		if user, err := h.db.GetUser(p.UserID); err == nil && user != nil {
			p.Role = user.Role
			p.Name = user.Name()
		}
	}
	return p
}

// webPrincipal is the principal of a browser request.
func (h *Handler) webPrincipal(r *http.Request) principal {
	return h.principalFor(h.webSessionUser(r))
}

// requireSession refuses an anonymous request with 401.
func (h *Handler) requireSession(w http.ResponseWriter, r *http.Request) (principal, bool) {
	p := h.webPrincipal(r)
	if p.Anonymous() {
		writeError(w, http.StatusUnauthorized, msgSignIn)
		return p, false
	}
	return p, true
}

// requireAdmin refuses an anonymous request with 401 and a member with 403.
func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) (principal, bool) {
	p, ok := h.requireSession(w, r)
	if !ok {
		return p, false
	}
	if !p.IsAdmin() {
		writeError(w, http.StatusForbidden, msgAdminOnly)
		return p, false
	}
	return p, true
}

// requireOwnerOrAdmin lets the owner of a record and admins through. A record
// without owner, written before ownership existed, belongs to no one and is an
// admin's to act on.
func (h *Handler) requireOwnerOrAdmin(w http.ResponseWriter, r *http.Request, ownerID string) (principal, bool) {
	p, ok := h.requireSession(w, r)
	if !ok {
		return p, false
	}
	if p.IsAdmin() || (ownerID != "" && ownerID == p.UserID) {
		return p, true
	}
	writeError(w, http.StatusForbidden, msgNotOwner)
	return p, false
}

// adminOnlyRoute names the interface mutations reserved to admins. It is the
// table the guard enforces and the test reads; a mutation that needs the
// request body to decide, settings and dispatch, is checked in its handler.
func adminOnlyRoute(method, path string) bool {
	switch {
	case path == "/api/users" || strings.HasPrefix(path, "/api/users/"):
		return true
	case strings.HasPrefix(path, "/api/setup/tracker"):
		return method == http.MethodPost || method == http.MethodPut
	case path == "/api/projects":
		return method == http.MethodPost
	case strings.HasPrefix(path, "/api/projects/"):
		return method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete
	}
	return false
}

// memberSettingsKeys are the settings a member may write: the personal
// preferences the profile edits, none of which configures the deployment.
// Everything else in the shared settings row is an admin's.
var memberSettingsKeys = map[string]bool{
	"theme": true, "accentColor": true, "language": true, "density": true,
	"defaultView": true, "uiScale": true, "detailMode": true,
	"userName": true, "userEmail": true, "userAvatar": true,
}

// memberSettingsViolations names the admin-only keys a member's payload would
// change. The interface always posts the whole row, so a key is only offending
// when its value differs from what is stored; an unchanged key is not a change.
func memberSettingsViolations(current models.Settings, sent map[string]json.RawMessage) []string {
	stored, err := json.Marshal(current)
	if err != nil {
		return nil
	}
	var storedKeys map[string]json.RawMessage
	_ = json.Unmarshal(stored, &storedKeys)
	var offending []string
	for key, value := range sent {
		if memberSettingsKeys[key] || key == "id" || key == "updatedAt" {
			continue
		}
		if bytes.Equal(canonicalJSON(value), canonicalJSON(storedKeys[key])) {
			continue
		}
		offending = append(offending, key)
	}
	sort.Strings(offending)
	return offending
}

// memberSettingsPayload is what a member's save actually writes: the stored
// row, overlaid with the preference keys their request carried. Comparing
// values is not enough on its own, because an omitted key is a zero value to
// the decoder and would switch off a boolean like autoSyncEnabled; taking
// everything else from storage makes an omission change nothing.
func memberSettingsPayload(current models.Settings, sent map[string]json.RawMessage) (models.Settings, error) {
	stored, err := json.Marshal(current)
	if err != nil {
		return current, err
	}
	var merged map[string]json.RawMessage
	if err := json.Unmarshal(stored, &merged); err != nil {
		return current, err
	}
	for key, value := range sent {
		if memberSettingsKeys[key] {
			merged[key] = value
		}
	}
	body, err := json.Marshal(merged)
	if err != nil {
		return current, err
	}
	var effective models.Settings
	if err := json.Unmarshal(body, &effective); err != nil {
		return current, err
	}
	return effective, nil
}

// canonicalJSON re-encodes a value so two spellings of the same thing compare
// equal, and so an absent key compares equal to its zero value.
func canonicalJSON(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return []byte("null")
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return raw
	}
	if value == nil || value == "" || value == false || value == 0.0 {
		return []byte("null")
	}
	out, err := json.Marshal(value)
	if err != nil {
		return raw
	}
	return out
}
