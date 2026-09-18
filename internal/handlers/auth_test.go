package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// An open redirect in a sign-in flow turns the interface into a bounce to
// whatever the caller names.
func TestSignInRedirectStaysInsideTheInterface(t *testing.T) {
	for _, testCase := range []struct{ given, want string }{
		{"/board", "/board"},
		{"/projects?id=1", "/projects?id=1"},
		{"", "/"},
		{"   ", "/"},
		{"//evil.example/path", "/"},
		{"https://evil.example", "/"},
		{"http://evil.example", "/"},
		{"javascript:alert(1)", "/"},
		{"../../etc/passwd", "/"},
	} {
		if got := safeRedirect(testCase.given); got != testCase.want {
			t.Errorf("safeRedirect(%q) = %q, want %q", testCase.given, got, testCase.want)
		}
	}
}

func TestSessionCookieIsHiddenFromScript(t *testing.T) {
	h := &Handler{}
	request, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8090/auth/callback", nil)
	if err != nil {
		t.Fatal(err)
	}
	cookie := h.sessionCookieFor(request, "value", 3600)
	if !cookie.HttpOnly {
		t.Error("session cookie is readable by script")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax so the provider redirect keeps the cookie", cookie.SameSite)
	}
	if cookie.Path != "/" {
		t.Errorf("Path = %q, want /", cookie.Path)
	}
}

// Behind a TLS-terminating proxy the cookie must still be marked Secure, or it
// travels in clear on the next plain request.
func TestSessionCookieIsSecureBehindTLS(t *testing.T) {
	h := &Handler{}
	request, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8090/auth/callback", nil)
	if err != nil {
		t.Fatal(err)
	}
	if h.sessionCookieFor(request, "value", 3600).Secure {
		t.Error("cookie marked Secure on a plain request")
	}
	request.Header.Set("X-Forwarded-Proto", "https")
	if !h.sessionCookieFor(request, "value", 3600).Secure {
		t.Error("cookie not marked Secure behind a TLS proxy")
	}
}

func TestSignOutCookieIsExpired(t *testing.T) {
	h := &Handler{}
	request, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:8090/auth/logout", nil)
	if err != nil {
		t.Fatal(err)
	}
	cookie := h.sessionCookieFor(request, "", -1)
	if cookie.Value != "" || cookie.MaxAge != -1 {
		t.Fatalf("sign-out cookie is %q with MaxAge %d", cookie.Value, cookie.MaxAge)
	}
}

// The guard decides what an unauthenticated caller can reach. A path wrongly
// listed as public is an authentication bypass.
func TestOnlyIntendedPathsBypassTheSessionGuard(t *testing.T) {
	public := []string{
		"/auth/login", "/auth/callback", "/auth/logout", "/auth/local",
		"/api/me",
		"/api/v1/agent/pair", "/api/v1/agent/config", "/api/v1/agent/projects",
		"/mcp",
		"/ws/agent-connect",
		// The server's liveness route, the one cmd/server registers and the one
		// a load balancer polls. "/health" is the agent's, on another mux.
		"/api/health",
		"/health",
		"/", "/index.html", "/assets/app.js",
	}
	for _, path := range public {
		if !publicPath(path) {
			t.Errorf("publicPath(%q) = false, want true", path)
		}
	}
	guarded := []string{
		"/api/tasks", "/api/projects", "/api/settings",
		"/api/devices", "/api/pairing-codes",
		"/api/agent/dispatch", "/api/agent/status",
		"/ws/terminal",
		// The identity probe is public; what hangs below it is not. A personal
		// tracker credential is the last thing that should answer without a
		// session.
		"/api/me/tracker-credentials", "/api/me/tracker-credentials/unlock",
		// Both probes are matched exactly: nothing below them is public.
		"/api/health/details",
	}
	for _, path := range guarded {
		if publicPath(path) {
			t.Errorf("publicPath(%q) = true, want false", path)
		}
	}
}

// The rename takes its account from the session and never from the body, so the
// route that lets somebody choose their own name is not also a route that lets
// them choose somebody else's.
func TestRenameTheCurrentUser(t *testing.T) {
	h, session := credentialHandler(t)

	patch := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.HandleCurrentUser(w, signedRequest(session, http.MethodPatch, "/api/me", strings.NewReader(body)))
		return w
	}

	w := patch(`{"displayName":"Ada Lovelace"}`)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"displayName":"Ada Lovelace"`) {
		t.Fatalf("rename = %d %s", w.Code, w.Body.String())
	}
	// The answer is the profile itself, and a following read agrees with it.
	read := httptest.NewRecorder()
	h.HandleCurrentUser(read, signedRequest(session, http.MethodGet, "/api/me", nil))
	if !strings.Contains(read.Body.String(), `"displayName":"Ada Lovelace"`) {
		t.Fatalf("GET after rename = %s", read.Body.String())
	}

	if w = patch(`{"displayName":"` + strings.Repeat("e", 81) + `"}`); w.Code != http.StatusBadRequest ||
		!strings.Contains(w.Body.String(), "at most 80 characters") {
		t.Fatalf("over-long name = %d %s", w.Code, w.Body.String())
	}
	if w = patch(`{"displayName":"Ada\nLovelace"}`); w.Code != http.StatusBadRequest ||
		!strings.Contains(w.Body.String(), "cannot contain line breaks") {
		t.Fatalf("line break = %d %s", w.Code, w.Body.String())
	}
	// A refused rename left the stored name alone.
	read = httptest.NewRecorder()
	h.HandleCurrentUser(read, signedRequest(session, http.MethodGet, "/api/me", nil))
	if !strings.Contains(read.Body.String(), `"displayName":"Ada Lovelace"`) {
		t.Fatalf("a refused rename changed the name: %s", read.Body.String())
	}

	// Clearing hands the account back to the name its sign-in supplies.
	if w = patch(`{"displayName":""}`); w.Code != http.StatusOK ||
		!strings.Contains(w.Body.String(), `"displayName":"ada@example.com"`) {
		t.Fatalf("cleared name = %d %s", w.Code, w.Body.String())
	}
}

// /api/me is public for the read alone: it is what the interface asks before
// anyone is signed in. Writing through it requires the session all the same.
func TestRenameWithoutASessionIsRefused(t *testing.T) {
	h, _ := credentialHandler(t)

	w := httptest.NewRecorder()
	h.HandleCurrentUser(w, httptest.NewRequest(http.MethodPatch, "/api/me", strings.NewReader(`{"displayName":"Stranger"}`)))
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "Sign in to use this interface") {
		t.Fatalf("anonymous rename = %d %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	h.HandleCurrentUser(w, httptest.NewRequest(http.MethodDelete, "/api/me", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unsupported method = %d", w.Code)
	}
}
