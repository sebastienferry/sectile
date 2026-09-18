package handlers

import (
	"net/http"
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
