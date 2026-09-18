package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/db"
)

// credentialHandler is a handler and the session cookie of one signed-in
// account, so the routes act on one identity throughout, which is what these
// tests are about. Signing in is mandatory (ADR 0015), so there is no identity
// without a session any more.
func credentialHandler(t *testing.T) (*Handler, *http.Cookie) {
	t.Helper()
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
	return NewHandler(database), &http.Cookie{Name: sessionCookie, Value: token}
}

// signedRequest is a request carrying that session.
func signedRequest(cookie *http.Cookie, method, target string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, target, body)
	r.AddCookie(cookie)
	return r
}

// The routes never hand a token back, and they only ever act on the caller's
// own credentials: the user is taken from the session, never from the payload.
func TestPersonalCredentialRoutesNeverReturnAToken(t *testing.T) {
	h, session := credentialHandler(t)

	store := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.HandleUserTrackerCredentials(w, signedRequest(session, http.MethodPut, "/api/me/tracker-credentials", strings.NewReader(body)))
		return w
	}

	w := store(`{"tracker":"jira","siteUrl":"https://acme.atlassian.net","email":"ada@example.com","token":"ATATT-secret"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("store: %d %s", w.Code, w.Body)
	}
	if strings.Contains(w.Body.String(), "ATATT-secret") {
		t.Fatal("the answer must not carry the token")
	}
	var payload struct {
		Credentials []struct {
			Tracker  string `json:"tracker"`
			SiteURL  string `json:"siteUrl"`
			Email    string `json:"email"`
			Sealed   bool   `json:"sealed"`
			Unlocked bool   `json:"unlocked"`
		} `json:"credentials"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Credentials) != 1 || payload.Credentials[0].Tracker != "jira" || payload.Credentials[0].SiteURL != "https://acme.atlassian.net" || payload.Credentials[0].Email != "ada@example.com" || payload.Credentials[0].Sealed {
		t.Fatalf("stored credential: %+v", payload.Credentials)
	}

	// Sealing it changes what is reported, still without the token.
	if w := store(`{"tracker":"jira","siteUrl":"https://acme.atlassian.net","token":"ATATT-secret","passphrase":"open sesame"}`); w.Code != http.StatusOK || strings.Contains(w.Body.String(), "ATATT-secret") {
		t.Fatalf("seal: %d %s", w.Code, w.Body)
	}

	lock := httptest.NewRecorder()
	h.HandleUserTrackerCredentials(lock, signedRequest(session, http.MethodPost, "/api/me/tracker-credentials/lock", strings.NewReader(`{"tracker":"jira"}`)))
	if lock.Code != http.StatusOK || !strings.Contains(lock.Body.String(), `"unlocked":false`) {
		t.Fatalf("lock: %d %s", lock.Code, lock.Body)
	}

	wrong := httptest.NewRecorder()
	h.HandleUserTrackerCredentials(wrong, signedRequest(session, http.MethodPost, "/api/me/tracker-credentials/unlock", strings.NewReader(`{"tracker":"jira","passphrase":"nope"}`)))
	if wrong.Code != http.StatusForbidden {
		t.Fatalf("a wrong passphrase must be refused: %d %s", wrong.Code, wrong.Body)
	}

	right := httptest.NewRecorder()
	h.HandleUserTrackerCredentials(right, signedRequest(session, http.MethodPost, "/api/me/tracker-credentials/unlock", strings.NewReader(`{"tracker":"jira","passphrase":"open sesame"}`)))
	if right.Code != http.StatusOK || !strings.Contains(right.Body.String(), `"unlocked":true`) {
		t.Fatalf("unlock: %d %s", right.Code, right.Body)
	}

	gone := httptest.NewRecorder()
	h.HandleUserTrackerCredentials(gone, signedRequest(session, http.MethodDelete, "/api/me/tracker-credentials?tracker=jira", nil))
	if gone.Code != http.StatusOK || !strings.Contains(gone.Body.String(), `"credentials":[]`) {
		t.Fatalf("delete: %d %s", gone.Code, gone.Body)
	}
}
