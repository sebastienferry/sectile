package handlers

import (
	"net/http"
	"path/filepath"
	"testing"

	"tasks/internal/db"
)

// Every dispatch used to name "default" directly, which stops finding the
// agent as soon as one workstation is paired to a real user. The paths that
// have a request now resolve the caller; those that do not name the implicit
// user on purpose. Both must still land on the same value until someone can
// sign in, or this change would break single-user deployments.
func TestImplicitUserMatchesAnUnauthenticatedSession(t *testing.T) {
	if ImplicitUser != "default" {
		t.Fatalf("ImplicitUser = %q; existing agents register under \"default\"", ImplicitUser)
	}

	h := &Handler{}
	request, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:8090/api/tasks/x/run-skill", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := h.webSessionUser(request); got != ImplicitUser {
		t.Fatalf("webSessionUser without a provider = %q, want %q", got, ImplicitUser)
	}
	if mode := h.signInMode(); mode != modeImplicit {
		t.Fatalf("signInMode without a database = %q, want implicit", mode)
	}
	if p := h.principalFor(ImplicitUser); !p.IsAdmin() {
		t.Fatalf("the implicit user is %q, want admin: a personal deployment must never be locked out", p.Role)
	}
}

// The implicit user lasts until the first local account. From then on an
// anonymous request is a stranger, and the cookie is the only way in.
func TestFirstLocalAccountEndsTheImplicitUser(t *testing.T) {
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
	if got := h.webSessionUser(anonymous); got != ImplicitUser {
		t.Fatalf("before any account: %q, want the implicit user", got)
	}

	alice, err := database.SignInLocal("alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got := h.webSessionUser(anonymous); got != "" {
		t.Fatalf("after the first account an anonymous request resolved to %q", got)
	}
	if mode := h.signInMode(); mode != "local" {
		t.Fatalf("signInMode = %q, want local", mode)
	}

	token, _, err := database.CreateWebSession(alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	signed, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:8090/api/tasks", nil)
	signed.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	if got := h.webSessionUser(signed); got != alice.ID {
		t.Fatalf("cookie resolved to %q, want %s", got, alice.ID)
	}
	if p := h.webPrincipal(signed); !p.IsAdmin() || p.Name != "alice@example.com" {
		t.Fatalf("first account principal = %+v, want an admin named by her address", p)
	}
}
