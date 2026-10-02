package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"tasks/internal/db"
)

// workstationTestState is a state the broker accepts: what a workstation draws is 43 base64url characters.
const workstationTestState = "s7Hk2-Qx_9ZpLm3NvB8cR1tYw4EuJ6aFgDoKiVbXyTz"

// workstationSignIn calls the broker with a raw query and an optional session cookie or bearer key.
func workstationSignIn(t *testing.T, h *Handler, rawQuery string, cookie *http.Cookie, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/auth/workstation?"+rawQuery, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rr := httptest.NewRecorder()
	h.HandleWorkstationSignIn(rr, req)
	return rr
}

// signInRedirect checks the broker sent a stranger to the web sign-in and returns where that sign-in comes back to.
func signInRedirect(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d %s, want 302 to the sign-in", rr.Code, rr.Body.String())
	}
	location, err := url.Parse(rr.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if location.Host != "" || location.Path != "/auth/login" {
		t.Fatalf("Location = %q, want /auth/login", rr.Header().Get("Location"))
	}
	return location.Query().Get("redirect")
}

func TestWorkstationSignInSendsAStrangerToSignIn(t *testing.T) {
	h, _, cleanup := setupTestHandler(t)
	defer cleanup()

	rr := workstationSignIn(t, h, "port=49152&state="+workstationTestState, nil, "")
	want := "/auth/login?redirect=" + url.QueryEscape("/auth/workstation?port=49152&state="+workstationTestState)
	if got := rr.Header().Get("Location"); got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
	back := signInRedirect(t, rr)
	// The sign-in keeps the way back as it is, so the browser returns to the broker after signing in.
	if safeRedirect(back) != back {
		t.Fatalf("safeRedirect(%q) = %q: the sign-in would not come back to the broker", back, safeRedirect(back))
	}
	if rr.Header().Get("Cache-Control") != "no-store" || rr.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("headers = %v, want no-store and no-referrer", rr.Header())
	}
}

func TestWorkstationSignInHandsALoopbackACode(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	userID, cookie := account(t, database, "ada@example.com")

	rr := workstationSignIn(t, h, "port=49152&state="+workstationTestState, cookie, "")
	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d %s, want 302 to the loopback", rr.Code, rr.Body.String())
	}
	location, err := url.Parse(rr.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if location.Scheme != "http" || location.Host != "127.0.0.1:49152" || location.Path != "/callback" {
		t.Fatalf("Location = %q, want http://127.0.0.1:49152/callback", location)
	}
	if got := location.Query().Get("state"); got != workstationTestState {
		t.Fatalf("state = %q, want %q echoed", got, workstationTestState)
	}
	token, credential, err := database.RedeemPairingCode(location.Query().Get("code"), "laptop", "")
	if err != nil {
		t.Fatalf("the code handed to the loopback does not redeem: %v", err)
	}
	if credential.UserID != userID || database.UserForDeviceToken(token) != userID {
		t.Fatalf("the code paired %q, want %q", credential.UserID, userID)
	}
}

func TestWorkstationSignInRefusesBadInput(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	_, cookie := account(t, database, "ada@example.com")

	for _, rawQuery := range []string{
		"port=80&state=" + workstationTestState,
		"port=70000&state=" + workstationTestState,
		"port=abc&state=" + workstationTestState,
		"state=" + workstationTestState,
		"port=49152&state=short",
		"port=49152",
		"port=49152&state=" + workstationTestState + "/evil",
		"port=49152&state=" + workstationTestState + "%2Fevil",
		"port=49152&state=" + strings.Repeat("a", 129),
	} {
		for _, session := range []*http.Cookie{nil, cookie} {
			rr := workstationSignIn(t, h, rawQuery, session, "")
			if rr.Code != http.StatusBadRequest || rr.Header().Get("Location") != "" {
				t.Errorf("%q (signed in: %v): %d Location %q, want 400 and no Location", rawQuery, session != nil, rr.Code, rr.Header().Get("Location"))
			}
		}
	}
}

// A key must not mint keys: a bearer without a session is a stranger to the broker.
func TestWorkstationSignInIgnoresABearerKey(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	userID, _ := account(t, database, "ada@example.com")
	key, _, err := database.CreateAPIKey(userID, "laptop", db.DefaultAPIKeyTTL)
	if err != nil {
		t.Fatal(err)
	}

	signInRedirect(t, workstationSignIn(t, h, "port=49152&state="+workstationTestState, nil, key))
}

func TestWorkstationSignInRefusesABlockedAccount(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	// The first account takes the admin role; the second is the one blocked.
	account(t, database, "root@example.com")
	userID, _ := account(t, database, "bob@example.com")
	if _, err := database.SetUserBlocked(userID, true); err != nil {
		t.Fatal(err)
	}
	// Blocking may end the open sessions; one opened afterwards must still be refused by the broker itself.
	token, _, err := database.CreateWebSession(userID)
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: sessionCookie, Value: token}

	rr := workstationSignIn(t, h, "port=49152&state="+workstationTestState, cookie, "")
	if rr.Code != http.StatusForbidden || !strings.Contains(rr.Body.String(), msgBlocked) || rr.Header().Get("Location") != "" {
		t.Fatalf("blocked account: %d %s Location %q, want 403 and no Location", rr.Code, rr.Body.String(), rr.Header().Get("Location"))
	}
}

func TestWorkstationSignInDoesNotEchoExtraParameters(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	_, cookie := account(t, database, "ada@example.com")
	rawQuery := "port=49152&state=" + workstationTestState + "&next=https://evil.example&label=x"

	for _, session := range []*http.Cookie{nil, cookie} {
		rr := workstationSignIn(t, h, rawQuery, session, "")
		location := rr.Header().Get("Location")
		if rr.Code != http.StatusFound || location == "" {
			t.Fatalf("signed in: %v: %d %s, want a redirect", session != nil, rr.Code, rr.Body.String())
		}
		decoded, _ := url.QueryUnescape(location)
		for _, leaked := range []string{"evil", "next", "label"} {
			if strings.Contains(decoded, leaked) {
				t.Errorf("signed in: %v: Location %q carries %q", session != nil, location, leaked)
			}
		}
	}
}
