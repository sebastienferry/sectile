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

// principal is who a request comes from, resolved once and read by every
// authorization check. No handler compares user ids by hand or reads the role
// column itself: the two roles and the one ownership rule live here.
type principal struct {
	UserID string
	Role   string
	// Mode is the deployment's sign-in mode: oidc or local.
	Mode string
	Name string
	// Blocked is an account an admin has closed. It is resolved here rather
	// than checked at each door so that a block takes hold everywhere at once,
	// including on the sessions that were already open.
	Blocked bool
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
	msgBlocked   = "This account is blocked: ask an admin to open it again"
)

// signInMode is the deployment's mode. A configured provider is the only
// authority; without one the local e-mail sign-in identifies people. Signing
// in is mandatory either way (ADR 0015): there is no mode that opens the board
// to an anonymous visitor.
func (h *Handler) signInMode() string {
	if h.identityProvider != nil {
		return auth.ModeOIDC
	}
	return auth.ModeLocal
}

// principalFor resolves a user id to its principal. Every id, "default"
// included, resolves through the users table and keeps its stored role; an
// unknown id is a member, the safe reading of a bad row.
func (h *Handler) principalFor(userID string) principal {
	p := principal{UserID: strings.TrimSpace(userID), Mode: h.signInMode()}
	if p.UserID == "" {
		return p
	}
	p.Role = db.RoleMember
	if h.db != nil {
		if user, err := h.db.GetUser(p.UserID); err == nil && user != nil {
			p.Role = user.Role
			p.Name = user.Name()
			p.Blocked = user.Blocked
		}
	}
	return p
}

// webPrincipal is the principal of a browser request.
func (h *Handler) webPrincipal(r *http.Request) principal {
	return h.principalFor(h.webSessionUser(r))
}

// requireSession refuses an anonymous request with 401, and a blocked account
// with 403. The two answers differ because the remedies do: one is a sign-in,
// the other is a word with an admin.
func (h *Handler) requireSession(w http.ResponseWriter, r *http.Request) (principal, bool) {
	p := h.webPrincipal(r)
	if p.Anonymous() {
		writeError(w, http.StatusUnauthorized, msgSignIn)
		return p, false
	}
	if p.Blocked {
		writeError(w, http.StatusForbidden, msgBlocked)
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
//
// Only the accounts themselves are on it. The board is a shared workspace: a
// member creates, renames and deletes a project and configures the tracker it
// reads from, because a board where only an admin can open a project is a board
// that waits on one person. What stays an admin's is the roster, who exists,
// what role they hold, and whether their account still opens.
func adminOnlyRoute(_ string, path string) bool {
	return path == "/api/users" || strings.HasPrefix(path, "/api/users/")
}

// personalSettingsKeys is the routing table between the two settings stores
// (ADR 0015): a key named here belongs to the caller and is written to their
// user_settings row; every other key configures the deployment, lives in the
// single settings row and is an admin's to change. It is therefore also the
// list a member may write, which is what it was before the split.
var personalSettingsKeys = map[string]bool{
	"theme": true, "accentColor": true, "language": true, "density": true,
	"defaultView": true, "uiScale": true, "detailMode": true,
	"userName": true, "userEmail": true, "userAvatar": true,
	"editorCommand": true, "externalTerminalCommand": true,
}

// trackerSettingsKeys are deployment keys a member may nonetheless write. They
// describe which tracker the board reads from and the credential it reads with,
// and they are the settings half of the same rule as the route table: opening a
// project and pointing it at its tracker are one act, so refusing the second to
// a member who may do the first would only make the board unusable in a
// different place. They stay in the shared row: there is one tracker per
// deployment, not one per person (a *personal* credential is another mechanism,
// ADR 0014). The token fields are included because a credential is what makes
// the configuration work; the Set / FromEnv flags are projections the API
// answers rather than values anyone writes, and are listed so a whole-row post
// carrying them is not read as an offence.
var trackerSettingsKeys = map[string]bool{
	"issueTracker": true,
	"githubRepo":   true, "githubApiUrl": true,
	"githubToken": true, "githubTokenSet": true, "githubTokenFromEnv": true,
	"jiraUrl": true, "jiraProject": true, "jiraEmail": true,
	"jiraApiToken": true, "jiraApiTokenSet": true, "jiraApiTokenFromEnv": true,
}

// retiredSettingsKeys are keys the settings no longer have. A tab opened before
// the upgrade still holds them and posts them back with the whole row; the
// decoder drops them, so they change nothing and must not be refused as an
// admin-only change either. The GitLab tracker settings went with #251.
var retiredSettingsKeys = map[string]bool{
	"gitlabUrl": true, "gitlabProject": true,
	"gitlabToken": true, "gitlabTokenSet": true, "gitlabTokenFromEnv": true,
}

// memberSettingsKeys is the authorization rule: what a member may change, in
// either store. It is no longer the personal table alone, which is why the two
// are now named apart.
func memberSettingsKeys(key string) bool {
	return personalSettingsKeys[key] || trackerSettingsKeys[key]
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
		if memberSettingsKeys(key) || retiredSettingsKeys[key] || key == "id" || key == "updatedAt" {
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
	return settingsOverlay(current, sent, func(key string) bool { return personalSettingsKeys[key] })
}

// deploymentSettingsPayload is the other half of the routing: the stored
// deployment row overlaid with the deployment keys the request carried, so a
// personal key in the same payload never reaches the shared row. A member
// reaches the row too, for the tracker keys only; everything else on it, the AI
// configuration, the prompts, the repository path and the sync loop, answers to
// an admin.
func deploymentSettingsPayload(current models.Settings, sent map[string]json.RawMessage, admin bool) (models.Settings, error) {
	return settingsOverlay(current, sent, func(key string) bool {
		if personalSettingsKeys[key] {
			return false
		}
		return admin || trackerSettingsKeys[key]
	})
}

// settingsOverlay merges the keys wanted names over the stored row, through
// JSON so the payload's own spelling of a field decides whether it was sent.
func settingsOverlay(current models.Settings, sent map[string]json.RawMessage, wanted func(string) bool) (models.Settings, error) {
	stored, err := json.Marshal(current)
	if err != nil {
		return current, err
	}
	var merged map[string]json.RawMessage
	if err := json.Unmarshal(stored, &merged); err != nil {
		return current, err
	}
	for key, value := range sent {
		if wanted(key) {
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
