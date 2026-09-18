package handlers

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"tasks/internal/db"
)

// Signing in is mandatory (ADR 0015): an anonymous interface request names
// nobody, on a fresh deployment as on an established one. ImplicitUser only
// survives on the paths that run without an HTTP request.
func TestAnonymousRequestNamesNobody(t *testing.T) {
	if ImplicitUser != "default" {
		t.Fatalf("ImplicitUser = %q; existing agents register under \"default\"", ImplicitUser)
	}

	database, err := db.NewDB(filepath.Join(t.TempDir(), "identity.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer func() { _ = database.Close() }()
	h := NewHandler(database)

	anonymous, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8090/api/tasks", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := h.webSessionUser(anonymous); got != "" {
		t.Fatalf("anonymous request on an empty deployment resolved to %q", got)
	}
	if mode := h.signInMode(); mode != "local" {
		t.Fatalf("signInMode = %q, want local; the implicit mode is gone", mode)
	}
	if !h.webPrincipal(anonymous).Anonymous() {
		t.Fatal("an anonymous request produced an identified principal")
	}
}

// The "default" account is an ordinary one: it keeps the role its row stores
// instead of being an admin by construction. An existing deployment where it
// is an admin stays administrable, and one where it is not is not locked out
// either, since the first sign-in still takes the admin role.
func TestDefaultAccountKeepsItsStoredRole(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "identity.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer func() { _ = database.Close() }()
	h := NewHandler(database)

	if err := database.EnsureUser(db.ImplicitUserID); err != nil {
		t.Fatal(err)
	}
	if p := h.principalFor(db.ImplicitUserID); p.IsAdmin() {
		t.Fatalf("a member default resolved as %q, want member", p.Role)
	}
	// Nobody is an admin yet, so the first person to sign in becomes one.
	alice, err := database.SignInLocal("alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if p := h.principalFor(alice.ID); !p.IsAdmin() {
		t.Fatalf("first sign-in beside a default row = %q, want admin", p.Role)
	}

	if _, err := database.SetUserRole(db.ImplicitUserID, db.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if p := h.principalFor(db.ImplicitUserID); !p.IsAdmin() {
		t.Fatalf("a stored admin default resolved as %q", p.Role)
	}

	bob, err := database.SignInLocal("bob@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if p := h.principalFor(bob.ID); p.IsAdmin() {
		t.Fatalf("a later account is %q, want member", p.Role)
	}
}

// The first account on an empty deployment is admin, the unchanged ADR 0013
// rule: nobody is locked out by sign-in becoming mandatory.
func TestFirstAccountOnAnEmptyDeploymentIsAdmin(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "identity.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer func() { _ = database.Close() }()
	h := NewHandler(database)

	alice, err := database.SignInLocal("alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := database.CreateWebSession(alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	signed, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:8090/api/tasks", nil)
	signed.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	if p := h.webPrincipal(signed); !p.IsAdmin() || p.Name != "alice@example.com" {
		t.Fatalf("first account principal = %+v, want an admin named by her address", p)
	}
}

// RequireSession answers 401 on an interface route and keeps serving every
// public path, on a deployment where nobody has an account yet.
func TestRequireSessionRefusesAnonymousAndServesPublicPaths(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "identity.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer func() { _ = database.Close() }()
	h := NewHandler(database)

	served := false
	guard := h.RequireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served = true
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tasks", nil))
	if rec.Code != http.StatusUnauthorized || served {
		t.Fatalf("anonymous /api/tasks = %d (served=%v), want 401", rec.Code, served)
	}

	for _, path := range []string{"/api/me", "/auth/signin", "/api/v1/agent/poll", "/mcp", "/health", "/index.html"} {
		served = false
		rec := httptest.NewRecorder()
		guard.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if !served || rec.Code != http.StatusOK {
			t.Fatalf("public path %s = %d (served=%v), want it served", path, rec.Code, served)
		}
	}
}

// A workstation API key still names its user: the machine surfaces reach the
// same routes with a key rather than a cookie.
func TestAPIKeyStillResolvesItsUser(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "identity.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer func() { _ = database.Close() }()
	h := NewHandler(database)

	alice, err := database.SignInLocal("alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	key, _, err := database.CreateAPIKey(alice.ID, "laptop", db.DefaultAPIKeyTTL)
	if err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:8090/api/tasks", nil)
	request.Header.Set("Authorization", "Bearer "+key)
	if got := h.webSessionUser(request); got != alice.ID {
		t.Fatalf("API key resolved to %q, want %s", got, alice.ID)
	}
}
