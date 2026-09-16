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
// user on purpose. Both must still land on the same value until a provider is
// configured, or this change would break single-user deployments.
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
}

// The development switch names a user, so a dispatch made on its behalf must
// follow it rather than fall back to the implicit one: two people exercising
// the switch must not share one agent.
func TestDevelopmentIdentityIsNotTheImplicitUser(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "identity.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer func() { _ = database.Close() }()
	h := NewHandler(database)

	t.Setenv("SECTILE_DEV_IDENTITY", "1")
	resolve := func(name string) string {
		request, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:8090/api/tasks/x/run-skill", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("X-Sectile-User", name)
		return h.webSessionUser(request)
	}

	alice, bob := resolve("alice"), resolve("bob")
	if alice == ImplicitUser || bob == ImplicitUser {
		t.Fatalf("a named development identity resolved to the implicit user: %q, %q", alice, bob)
	}
	if alice == bob {
		t.Fatalf("alice and bob both resolved to %q", alice)
	}
	if again := resolve("alice"); again != alice {
		t.Fatalf("alice resolved to %q then %q", alice, again)
	}
}
