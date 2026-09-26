package handlers

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/db"
	"tasks/internal/secrets"
)

// sealedCredentialUnlocked stores a sealed Jira credential through the route,
// which leaves it unlocked, and fails unless the profile says so.
func sealedCredentialUnlocked(t *testing.T, h *Handler, session *http.Cookie) {
	t.Helper()
	save := httptest.NewRecorder()
	h.HandleUserTrackerCredentials(save, signedRequest(session, http.MethodPut, "/api/me/tracker-credentials",
		strings.NewReader(`{"tracker":"jira","siteUrl":"https://acme.atlassian.net","email":"ada@example.com","token":"sealed-token","passphrase":"open sesame"}`)))
	if save.Code != http.StatusOK || !strings.Contains(save.Body.String(), `"unlocked":true`) {
		t.Fatalf("storing a sealed credential: %d %s", save.Code, save.Body)
	}
}

// signOut posts to the sign-out route with a session.
func signOut(t *testing.T, h *Handler, session *http.Cookie) {
	t.Helper()
	out := httptest.NewRecorder()
	h.HandleLogout(out, signedRequest(session, http.MethodPost, "/auth/logout", nil))
	if out.Code != http.StatusOK {
		t.Fatalf("sign-out: %d %s", out.Code, out.Body)
	}
}

// unlockedNow reports whether the person's Jira credential is unlocked.
func unlockedNow(t *testing.T, h *Handler, userID string) bool {
	t.Helper()
	credentials, err := h.db.UserTrackerCredentials(userID)
	if err != nil || len(credentials) != 1 {
		t.Fatalf("reading the credentials back: %+v %v", credentials, err)
	}
	return credentials[0].Unlocked
}

// Signing out of the only session, with no agent connected, locks the sealed
// credential at once (US5).
func TestSigningOutOfTheLastSessionLocksSealedTokens(t *testing.T) {
	h, session := credentialHandler(t)
	userID := h.db.WebSessionOwner(session.Value)
	sealedCredentialUnlocked(t, h, session)
	signOut(t, h, session)
	if unlockedNow(t, h, userID) {
		t.Fatal("signing out of the last session must lock the credential")
	}
}

// Another open tab keeps the credential unlocked when one session signs out.
func TestSigningOutOfOneOfTwoSessionsKeepsTheUnlock(t *testing.T) {
	h, session := credentialHandler(t)
	userID := h.db.WebSessionOwner(session.Value)
	sealedCredentialUnlocked(t, h, session)
	other, _, err := h.db.CreateWebSession(userID)
	if err != nil {
		t.Fatal(err)
	}
	if h.db.UserForWebSession(other) != userID {
		t.Fatal("the second session must resolve")
	}
	signOut(t, h, session)
	if !unlockedNow(t, h, userID) {
		t.Fatal("a second live session must keep the credential unlocked")
	}
}

// A connected local agent keeps the credential unlocked through the sign-out of
// the last browser session (US4, US5).
func TestSigningOutWithAConnectedAgentKeepsTheUnlock(t *testing.T) {
	h, session := credentialHandler(t)
	userID := h.db.WebSessionOwner(session.Value)
	sealedCredentialUnlocked(t, h, session)
	stop, err := h.db.StartInstance()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stop)
	if _, _, err := h.db.AgentConnected(userID, "default", "device-1"); err != nil {
		t.Fatal(err)
	}
	signOut(t, h, session)
	if !unlockedNow(t, h, userID) {
		t.Fatal("a connected agent must keep the credential unlocked")
	}
}

// Without a server key an unlock cannot be kept, and the route says so rather
// than answering "unlocked" for as long as one process lives (FR-9).
func TestUnlockingWithoutAServerKeyAnswers503(t *testing.T) {
	t.Setenv(secrets.KeyEnvVar, "not-a-key")
	database, err := db.NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	user, err := database.SignInLocal("ada@example.com")
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := database.CreateWebSession(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	h, session := NewHandler(database), &http.Cookie{Name: sessionCookie, Value: token}
	if err := database.SetUserTrackerCredential(user.ID, "jira", "https://acme.atlassian.net", "", "sealed-token", "open sesame"); err != nil {
		t.Fatal(err)
	}

	for _, body := range []string{`{"tracker":"jira","passphrase":"open sesame"}`, `{"passphrase":"open sesame"}`} {
		unlock := httptest.NewRecorder()
		h.HandleUserTrackerCredentials(unlock, signedRequest(session, http.MethodPost, "/api/me/tracker-credentials/unlock", strings.NewReader(body)))
		if unlock.Code != http.StatusServiceUnavailable || !strings.Contains(unlock.Body.String(), secrets.KeyEnvVar) {
			t.Fatalf("%s: %d %s", body, unlock.Code, unlock.Body)
		}
	}
}
