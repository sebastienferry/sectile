package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"

	"tasks/internal/auth"
	"tasks/internal/db"
)

const sessionCookie = "sectile_session"

// HealthPath is the server's liveness route. The route registration, the
// session guard and the "am I already serving?" probe all read it here: the
// three held the same literal separately once, drifted, and a load balancer
// polling a guarded path took the whole deployment out.
const HealthPath = "/api/health"

// SetIdentityProvider installs the provider the server signs people in
// against. Without one the local e-mail sign-in identifies people.
func (h *Handler) SetIdentityProvider(provider *auth.Provider) {
	h.identityProvider = provider
}

// safeRedirect keeps a sign-in from being used to bounce someone to another
// site: only a path within this interface is accepted.
func safeRedirect(target string) string {
	target = strings.TrimSpace(target)
	if !strings.HasPrefix(target, "/") || strings.HasPrefix(target, "//") {
		return "/"
	}
	return target
}

func (h *Handler) sessionCookieFor(r *http.Request, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:  sessionCookie,
		Value: value,
		Path:  "/",
		// The cookie is the session: script must not be able to read it, and
		// it must not travel in clear when the interface is served over TLS.
		HttpOnly: true,
		Secure:   r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"),
		// Lax still sends the cookie on the provider's top-level redirect
		// back to us, which Strict would drop.
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	}
}

// HandleLogin starts a sign-in and sends the browser to the provider, or, on
// a deployment without one, to the interface's local sign-in screen, so the one
// URL the interface links to works in both modes.
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if h.identityProvider == nil {
		http.Redirect(w, r, "/signin?redirect="+url.QueryEscape(safeRedirect(r.URL.Query().Get("redirect"))), http.StatusFound)
		return
	}
	verifier, challenge, err := auth.NewVerifier()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	nonce, err := auth.NewNonce()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	state, err := h.db.StartLoginFlow(db.LoginFlow{
		Nonce:        nonce,
		CodeVerifier: verifier,
		Redirect:     safeRedirect(r.URL.Query().Get("redirect")),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	http.Redirect(w, r, h.identityProvider.AuthCodeURL(state, nonce, challenge), http.StatusFound)
}

// HandleAuthCallback completes a sign-in and opens a browser session.
func (h *Handler) HandleAuthCallback(w http.ResponseWriter, r *http.Request) {
	if h.identityProvider == nil {
		writeError(w, http.StatusNotFound, "No identity provider is configured")
		return
	}
	query := r.URL.Query()
	if providerError := query.Get("error"); providerError != "" {
		// The provider's own wording is not shown back: it is attacker
		// controlled in a redirect.
		log.Printf("[Identity] Provider refused the sign-in: %s", providerError)
		writeError(w, http.StatusUnauthorized, "The identity provider refused the sign-in")
		return
	}
	flow, err := h.db.ConsumeLoginFlow(query.Get("state"))
	if errors.Is(err, db.ErrLoginFlow) {
		writeError(w, http.StatusBadRequest, "This sign-in attempt is unknown or has expired. Start again.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	identity, err := h.identityProvider.Exchange(r.Context(), query.Get("code"), flow.CodeVerifier)
	if err != nil {
		log.Printf("[Identity] Sign-in failed: %v", err)
		writeError(w, http.StatusUnauthorized, "The sign-in could not be completed")
		return
	}
	// The provider's claim, when configured, is the authority on the role and
	// overwrites any manual change; without a claim the stored role stands and
	// the first person to sign in while no admin exists becomes one.
	user, err := h.db.SignInProvider(identity.Subject, identity.Email, identity.DisplayName, identity.Role, identity.RoleFromClaim)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	token, _, err := h.db.CreateWebSession(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	http.SetCookie(w, h.sessionCookieFor(r, token, int(db.WebSessionTTL.Seconds())))
	http.Redirect(w, r, safeRedirect(flow.Redirect), http.StatusFound)
}

// HandleLocalSignIn is the temporary local sign-in of a deployment without a
// provider: an e-mail address and nothing else. It identifies without
// authenticating, which is why it does not exist once a provider is configured
// and why the interface says what it is. The first account created is admin.
func (h *Handler) HandleLocalSignIn(w http.ResponseWriter, r *http.Request) {
	if h.identityProvider != nil {
		writeError(w, http.StatusNotFound, "Sign-in goes through the identity provider on this deployment")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var payload struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid sign-in request")
		return
	}
	user, err := h.db.SignInLocal(payload.Email)
	if errors.Is(err, db.ErrInvalidEmail) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	token, _, err := h.db.CreateWebSession(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	http.SetCookie(w, h.sessionCookieFor(r, token, int(db.WebSessionTTL.Seconds())))
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"userId":      user.ID,
		"email":       user.Email,
		"displayName": user.DisplayName,
		"role":        user.Role,
		"mode":        h.signInMode(),
	})
}

// HandleLogout ends the browser session on the server, not only in the browser.
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		if err := h.db.RevokeWebSession(cookie.Value); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	http.SetCookie(w, h.sessionCookieFor(r, "", -1))
	writeJSON(w, http.StatusOK, map[string]interface{}{"signedOut": true})
}

// HandleCurrentUser tells the interface who is signed in, and whether signing
// in is even possible on this deployment.
func (h *Handler) HandleCurrentUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	caller := h.webPrincipal(r)
	body := map[string]interface{}{
		"userId":           caller.UserID,
		"signedIn":         !caller.Anonymous(),
		"identityProvider": h.identityProvider != nil,
		// mode says how people sign in here: oidc or local. role is empty for
		// an anonymous caller.
		"mode": caller.Mode,
		"role": caller.Role,
		// The profile shows a deprecation notice while the shared credential
		// is configured, so the operator moves to API keys before it goes.
		"sharedServerToken": sharedServerTokenConfigured(),
	}
	if user, err := h.db.GetUser(caller.UserID); err == nil && user != nil {
		body["email"] = user.Email
		body["displayName"] = user.DisplayName
	}
	writeJSON(w, http.StatusOK, body)
}

// publicPaths are reachable without a browser session. Agent APIs carry their
// own device credential, sign-in cannot require being signed in, and the
// interface itself must load in order to offer the sign-in button.
func publicPath(path string) bool {
	// Two probes have to answer before anyone is signed in: /api/me, which the
	// interface asks to know who it is talking to, and /api/health, which is
	// what a load balancer polls and which holds no session. What hangs below
	// either is not public, personal tracker credentials least of all, so both
	// are matched exactly rather than as a prefix.
	switch strings.TrimSuffix(path, "/") {
	case "/api/me", HealthPath:
		return true
	}
	// "/health" is not listed here: it is the agent's own route, on the agent's
	// own mux, and naming it on this side once made /api/health look covered
	// when it was not. A path outside /api/ and /ws/ is public by the rule
	// below anyway.
	for _, prefix := range []string{
		"/auth/",
		"/api/v1/agent/",
		"/mcp",
		"/ws/agent-connect",
	} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	// Anything that is not an API call is the interface's own static files.
	return !strings.HasPrefix(path, "/api/") && !strings.HasPrefix(path, "/ws/")
}

// RequireSession guards the interface API: everything but publicPath needs a
// session, on every deployment and from the first visit (ADR 0015). Past the
// sign-in check it also refuses members on the routes adminOnlyRoute names.
func (h *Handler) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if publicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		caller := h.webPrincipal(r)
		if caller.Anonymous() {
			writeError(w, http.StatusUnauthorized, msgSignIn)
			return
		}
		if adminOnlyRoute(r.Method, r.URL.Path) && !caller.IsAdmin() {
			writeError(w, http.StatusForbidden, msgAdminOnly)
			return
		}
		next.ServeHTTP(w, r)
	})
}
