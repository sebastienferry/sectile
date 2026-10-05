package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"sync"
	"testing"

	"tasks/internal/auth"
	"tasks/internal/db"
)

// oidcFixture models the provider's network boundary, including client-secret
// authentication and PKCE. All credentials and identities are synthetic.
type oidcFixture struct {
	server     *httptest.Server
	mu         sync.Mutex
	challenge  string
	tokenCalls int
	failure    string
}

func newOIDCFixture(t *testing.T, failure string) *oidcFixture {
	t.Helper()
	f := &oidcFixture{failure: failure}
	f.server = httptest.NewTLSServer(http.HandlerFunc(f.serveHTTP))
	t.Cleanup(f.server.Close)
	// Discover uses a default transport. Trust only the fixture certificate for
	// this serial test, then restore the original transport before other tests.
	previous := http.DefaultTransport
	http.DefaultTransport = f.server.Client().Transport
	t.Cleanup(func() { http.DefaultTransport = previous })
	for key, value := range map[string]string{
		"SECTILE_OIDC_ISSUER":        f.server.URL,
		"SECTILE_OIDC_CLIENT_ID":     "fixture-client",
		"SECTILE_OIDC_CLIENT_SECRET": "fixture-secret",
		"SECTILE_OIDC_REDIRECT_URL":  "https://sectile.example.com/auth/callback",
		"SECTILE_OIDC_ROLE_CLAIM":    "",
		"SECTILE_OIDC_ADMIN_GROUP":   "",
	} {
		t.Setenv(key, value)
	}
	return f
}

func (f *oidcFixture) serveHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/.well-known/openid-configuration":
		if f.failure == "metadata unavailable" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if f.failure == "invalid metadata" {
			_, _ = w.Write([]byte(`{`))
			return
		}
		issuer := f.server.URL
		if f.failure == "issuer mismatch" {
			issuer = "https://other.example.com"
		}
		metadata := map[string]string{"issuer": issuer, "authorization_endpoint": f.server.URL + "/authorize", "token_endpoint": f.server.URL + "/oauth/token", "userinfo_endpoint": f.server.URL + "/userinfo"}
		if f.failure == "missing endpoint" {
			delete(metadata, "token_endpoint")
		}
		_ = json.NewEncoder(w).Encode(metadata)
	case "/oauth/token":
		f.tokenCalls++
		_ = r.ParseForm()
		// Require client_secret_post to exercise oauth2's authentication fallback.
		if r.Form.Get("client_id") != "fixture-client" || r.Form.Get("client_secret") != "fixture-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid_client"}`))
			return
		}
		sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		challenge := base64.RawURLEncoding.EncodeToString(sum[:])
		if r.Method != http.MethodPost || r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code") != "fixture-code" || r.Form.Get("redirect_uri") != "https://sectile.example.com/auth/callback" || f.challenge == "" || challenge != f.challenge || f.failure == "token rejected" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "fixture-access", "token_type": "Bearer", "expires_in": 3600})
	case "/userinfo":
		if r.Header.Get("Authorization") != "Bearer fixture-access" || f.failure == "userinfo rejected" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if f.failure == "invalid userinfo" {
			_, _ = w.Write([]byte(`{`))
			return
		}
		subject := "auth0|fixture-user"
		if f.failure == "missing subject" {
			subject = ""
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"sub": subject, "email": "member@example.com", "name": "Fixture Member"})
	default:
		http.NotFound(w, r)
	}
}

func oidcHandler(t *testing.T) *Handler {
	t.Helper()
	provider, err := auth.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	database, err := db.NewDB(filepath.Join(t.TempDir(), "oidc.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	h := NewHandler(database)
	h.SetIdentityProvider(provider)
	return h
}

func startOIDCLogin(t *testing.T, h *Handler, f *oidcFixture, destination string) string {
	t.Helper()
	out := httptest.NewRecorder()
	h.HandleLogin(out, httptest.NewRequest(http.MethodGet, "/auth/login?redirect="+url.QueryEscape(destination), nil))
	if out.Code != http.StatusFound {
		t.Fatalf("login status = %d", out.Code)
	}
	location, err := url.Parse(out.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	q := location.Query()
	if location.Scheme+"://"+location.Host+location.Path != f.server.URL+"/authorize" {
		t.Fatalf("unexpected authorization endpoint: %s", location)
	}
	for key, want := range map[string]string{
		"response_type": "code", "client_id": "fixture-client",
		"redirect_uri": "https://sectile.example.com/auth/callback",
		"scope":        "openid profile email", "code_challenge_method": "S256",
	} {
		if got := q.Get(key); got != want {
			t.Fatalf("authorization %s = %q, want %q", key, got, want)
		}
	}
	for _, key := range []string{"nonce", "state", "code_challenge"} {
		if q.Get(key) == "" {
			t.Fatalf("authorization request is missing %s", key)
		}
	}
	f.mu.Lock()
	f.challenge = q.Get("code_challenge")
	f.mu.Unlock()
	return q.Get("state")
}

func oidcCallback(h *Handler, state string) *httptest.ResponseRecorder {
	out := httptest.NewRecorder()
	h.HandleAuthCallback(out, httptest.NewRequest(http.MethodGet, "https://sectile.example.com/auth/callback?code=fixture-code&state="+url.QueryEscape(state), nil))
	return out
}

func TestOIDCBrowserSignInAndLogout(t *testing.T) {
	f := newOIDCFixture(t, "")
	h := oidcHandler(t)
	// An established administrator must not make every new provider user admin.
	if _, err := h.db.SignInLocal("admin@example.com"); err != nil {
		t.Fatal(err)
	}
	var userID string
	for _, destination := range []string{"/board", "https://outside.example.com"} {
		state := startOIDCLogin(t, h, f, destination)
		out := oidcCallback(h, state)
		want := destination
		if destination != "/board" {
			want = "/"
		}
		if out.Code != http.StatusFound || out.Header().Get("Location") != want {
			t.Fatalf("callback: %d %s", out.Code, out.Body.String())
		}
		cookies := out.Result().Cookies()
		if len(cookies) != 1 || !cookies[0].HttpOnly || !cookies[0].Secure {
			t.Fatalf("invalid session cookie: %v", cookies)
		}
		cookie := cookies[0]
		id := h.db.UserForWebSession(cookie.Value)
		if id == "" || (userID != "" && id != userID) {
			t.Fatalf("identity changed: previous %q, current %q", userID, id)
		}
		userID = id
		if role := h.db.UserRole(id); role != db.RoleMember {
			t.Fatalf("new member role = %q", role)
		}
		me := httptest.NewRecorder()
		h.HandleCurrentUser(me, signedRequest(cookie, http.MethodGet, "/api/me", nil))
		var body struct {
			SignedIn bool   `json:"signedIn"`
			UserID   string `json:"userId"`
		}
		if err := json.Unmarshal(me.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if !body.SignedIn || body.UserID != id {
			t.Fatalf("session not recognized: %s", me.Body.String())
		}
		replay := oidcCallback(h, state)
		if replay.Code != http.StatusBadRequest || len(replay.Result().Cookies()) != 0 {
			t.Fatal("replayed callback granted a session")
		}
		logout := httptest.NewRecorder()
		h.HandleLogout(logout, signedRequest(cookie, http.MethodPost, "/auth/logout", nil))
		if logout.Code != http.StatusOK || h.db.UserForWebSession(cookie.Value) != "" {
			t.Fatal("logout did not revoke the session")
		}
	}
	local := httptest.NewRecorder()
	h.HandleLocalSignIn(local, httptest.NewRequest(http.MethodPost, "/auth/local", nil))
	if local.Code != http.StatusNotFound || len(local.Result().Cookies()) != 0 {
		t.Fatal("local sign-in remains available with a provider")
	}
}

func TestOIDCCallbackFailuresNeverCreateSessions(t *testing.T) {
	for _, failure := range []string{"unknown state", "provider denial", "token rejected", "userinfo rejected", "invalid userinfo", "missing subject"} {
		t.Run(failure, func(t *testing.T) {
			f := newOIDCFixture(t, failure)
			h := oidcHandler(t)
			state := startOIDCLogin(t, h, f, "/board")
			var out *httptest.ResponseRecorder
			want := http.StatusUnauthorized
			if failure == "unknown state" {
				state = "unknown"
				want = http.StatusBadRequest
			}
			if failure == "provider denial" {
				out = httptest.NewRecorder()
				h.HandleAuthCallback(out, httptest.NewRequest(http.MethodGet, "/auth/callback?error=access_denied&state="+url.QueryEscape(state), nil))
			} else {
				out = oidcCallback(h, state)
			}
			if out.Code != want || len(out.Result().Cookies()) != 0 {
				t.Fatalf("failure granted a session or wrong status: %d", out.Code)
			}
			if failure == "unknown state" || failure == "provider denial" {
				f.mu.Lock()
				calls := f.tokenCalls
				f.mu.Unlock()
				if calls != 0 {
					t.Fatal("invalid callback reached the token endpoint")
				}
			}
		})
	}
}

func TestOIDCDiscoveryRejectsInvalidProviders(t *testing.T) {
	for _, failure := range []string{"metadata unavailable", "invalid metadata", "issuer mismatch", "missing endpoint"} {
		t.Run(failure, func(t *testing.T) {
			newOIDCFixture(t, failure)
			if provider, err := auth.Discover(context.Background()); err == nil || provider != nil {
				t.Fatal("discovery accepted invalid provider metadata")
			}
		})
	}
}
