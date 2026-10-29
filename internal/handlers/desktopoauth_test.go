package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func startDesktopConsent(t *testing.T, h *Handler, session *http.Cookie, provider string) (string, string, *http.Cookie, string) {
	t.Helper()
	w := httptest.NewRecorder()
	h.HandleDesktopOAuth(w, signedRequest(session, "POST", DesktopOAuthPath, strings.NewReader(`{"provider":"`+provider+`"}`)))
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var flow struct {
		ID        string `json:"id"`
		StartPath string `json:"startPath"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &flow); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	h.HandleDesktopOAuthStart(w, httptest.NewRequest("GET", flow.StartPath, nil))
	if w.Code != 302 {
		t.Fatalf("start: %d %s", w.Code, w.Body.String())
	}
	target, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name == sessionCookie || !cookies[0].HttpOnly {
		t.Fatal("desktop consent must use a separate HTTP-only cookie")
	}
	return flow.ID, target.Query().Get("state"), cookies[0], flow.StartPath
}
func desktopStatus(t *testing.T, h *Handler, session *http.Cookie, id string) string {
	t.Helper()
	w := httptest.NewRecorder()
	h.HandleDesktopOAuth(w, signedRequest(session, "GET", DesktopOAuthPath+"?id="+id, nil))
	var result struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result.Status
}
func finishDesktopConsent(h *Handler, provider string, cookie *http.Cookie, state, code string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/auth/"+provider+"/callback?"+url.Values{"state": {state}, "code": {code}}.Encode(), nil)
	req.AddCookie(cookie)
	if provider == "jira" {
		h.HandleJiraOAuthCallback(w, req)
	} else {
		h.handleForgeOAuthCallback(provider)(w, req)
	}
	return w
}
func TestDesktopOAuthRoundTripForAllProviders(t *testing.T) {
	for _, provider := range []string{"github", "gitlab", "jira"} {
		t.Run(provider, func(t *testing.T) {
			var h *Handler
			var session *http.Cookie
			var consent func() string
			if provider == "jira" {
				handler, cookie, fake := oauthHandler(t)
				h, session = handler, cookie
				consent = func() string { return fake.Consent("Ada", "c-acme") }
			} else {
				handler, cookie, fake := forgeOAuthHandler(t, provider)
				h, session = handler, cookie
				consent = func() string { return fake.Consent("ada") }
			}
			id, state, cookie, start := startDesktopConsent(t, h, session, provider)
			replay := httptest.NewRecorder()
			h.HandleDesktopOAuthStart(replay, httptest.NewRequest("GET", start, nil))
			if replay.Code != 410 {
				t.Fatal("browser ticket replay accepted")
			}
			// An unrelated state does not consume or hijack the active desktop flow.
			finishDesktopConsent(h, provider, cookie, "wrong-state", consent())
			if desktopStatus(t, h, session, id) != "waiting" {
				t.Fatal("unrelated callback changed flow")
			}
			w := finishDesktopConsent(h, provider, cookie, state, consent())
			if w.Code != 200 || !strings.Contains(w.Body.String(), "Connexion réussie") {
				t.Fatalf("callback: %d %s", w.Code, w.Body.String())
			}
			if desktopStatus(t, h, session, id) != "connected" {
				t.Fatal("successful consent was not reported")
			}
			if h.db.UserForWebSession(cookie.Value) != "" {
				t.Fatal("temporary browser session was not revoked")
			}
			if h.db.UserForWebSession(session.Value) == "" {
				t.Fatal("ordinary web session was modified")
			}
			finishDesktopConsent(h, provider, cookie, state, consent())
			if desktopStatus(t, h, session, id) != "connected" {
				t.Fatal("replay changed terminal outcome")
			}
		})
	}
}
func TestDesktopOAuthOwnershipCancellationAndExpiry(t *testing.T) {
	h, session, _ := forgeOAuthHandler(t, "github")
	id, _, _, start := startDesktopConsent(t, h, session, "github")
	if err := h.db.EnsureUser("other"); err != nil {
		t.Fatal(err)
	}
	token, _, err := h.db.CreateWebSession("other")
	if err != nil {
		t.Fatal(err)
	}
	other := &http.Cookie{Name: sessionCookie, Value: token}
	if desktopStatus(t, h, other, id) != "expired" {
		t.Fatal("another user could inspect the flow")
	}
	w := httptest.NewRecorder()
	h.HandleDesktopOAuth(w, signedRequest(session, "DELETE", DesktopOAuthPath+"?id="+id, nil))
	if desktopStatus(t, h, session, id) != "cancelled" {
		t.Fatal("cancellation not recorded")
	}
	w = httptest.NewRecorder()
	h.HandleDesktopOAuthStart(w, httptest.NewRequest("GET", start, nil))
	if w.Code != 410 {
		t.Fatal("cancelled ticket accepted")
	}
	id, _, cookie, _ := startDesktopConsent(t, h, session, "github")
	h.desktopOAuth.flows[id].Expires = time.Now().Add(-time.Second)
	if desktopStatus(t, h, session, id) != "expired" {
		t.Fatal("flow did not expire")
	}
	if h.db.UserForWebSession(cookie.Value) != "" {
		t.Fatal("expired session retained")
	}
	w = httptest.NewRecorder()
	h.HandleDesktopOAuth(w, httptest.NewRequest("POST", DesktopOAuthPath, strings.NewReader(`{"provider":"github"}`)))
	if w.Code != 401 {
		t.Fatal("anonymous initiation accepted")
	}
}

func TestDesktopOAuthCookieDoesNotHijackAnOrdinaryWebConsent(t *testing.T) {
	h, session, fake := forgeOAuthHandler(t, "github")
	id, _, desktopCookie, _ := startDesktopConsent(t, h, session, "github")
	state := startForgeConsent(t, h, session, "github", fake)
	req := httptest.NewRequest("GET", GithubOAuthCallbackPath+"?"+url.Values{"state": {state}, "code": {fake.Consent("web-user")}}.Encode(), nil)
	req.AddCookie(session)
	req.AddCookie(desktopCookie)
	w := httptest.NewRecorder()
	h.HandleGithubOAuthCallback(w, req)
	location, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if location.Query().Get("oauth") != "connected" {
		t.Fatalf("web consent was intercepted: %d", w.Code)
	}
	if desktopStatus(t, h, session, id) != "waiting" {
		t.Fatal("web callback consumed desktop flow")
	}
}
