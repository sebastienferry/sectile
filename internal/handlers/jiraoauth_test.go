package handlers

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"tasks/internal/atlassian/atlassiantest"
	"tasks/internal/db"
	"tasks/internal/models"
)

// oauthHandler is credentialHandler on a deployment with a Jira OAuth app,
// a fake Atlassian and a Jira project on its first site.
func oauthHandler(t *testing.T) (*Handler, *http.Cookie, *atlassiantest.Fake) {
	t.Helper()
	t.Setenv(db.JiraOAuthClientIDVar, atlassiantest.ClientID)
	t.Setenv(db.JiraOAuthClientSecretVar, atlassiantest.ClientSecret)
	t.Setenv(db.JiraOAuthRedirectURLVar, "https://sectile.example.com/auth/jira/callback")
	h, session := credentialHandler(t)
	fake := atlassiantest.New(t, atlassiantest.FakeSite{CloudID: "c-acme", URL: "https://acme.atlassian.net"})
	h.db.SetAtlassianEndpoints(fake.Endpoints(), nil)
	if _, err := h.db.CreateProject(models.CreateProjectRequest{Name: "Jira", IssueTracker: "jira", JiraProject: "PE", TrackerUrl: "https://acme.atlassian.net"}); err != nil {
		t.Fatal(err)
	}
	return h, session, fake
}

type credentialList struct {
	Credentials []struct {
		Tracker      string   `json:"tracker"`
		Kind         string   `json:"kind"`
		Account      string   `json:"account"`
		Disconnected bool     `json:"disconnected"`
		GrantedSites []string `json:"grantedSites"`
	} `json:"credentials"`
	JiraOAuth struct {
		Configured bool     `json:"configured"`
		Sites      []string `json:"sites"`
	} `json:"jiraOAuth"`
}

func listCredentials(t *testing.T, h *Handler, session *http.Cookie) credentialList {
	t.Helper()
	w := httptest.NewRecorder()
	h.HandleUserTrackerCredentials(w, signedRequest(session, http.MethodGet, "/api/me/tracker-credentials", nil))
	var list credentialList
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	return list
}

func startConsent(t *testing.T, h *Handler, session *http.Cookie) string {
	t.Helper()
	w := httptest.NewRecorder()
	h.HandleUserTrackerCredentials(w, signedRequest(session, http.MethodPost, "/api/me/tracker-credentials/jira/connect", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("connect: %d %s", w.Code, w.Body)
	}
	var answer struct {
		AuthorizeURL string `json:"authorizeUrl"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &answer)
	parsed, err := url.Parse(answer.AuthorizeURL)
	if err != nil || parsed.Query().Get("state") == "" {
		t.Fatalf("authorize URL: %q", answer.AuthorizeURL)
	}
	return parsed.Query().Get("state")
}

func callback(h *Handler, session *http.Cookie, query url.Values) string {
	r := httptest.NewRequest(http.MethodGet, JiraOAuthCallbackPath+"?"+query.Encode(), nil)
	if session != nil {
		r.AddCookie(session)
	}
	w := httptest.NewRecorder()
	h.HandleJiraOAuthCallback(w, r)
	if w.Code != http.StatusFound {
		return "status " + w.Result().Status
	}
	location, _ := url.Parse(w.Header().Get("Location"))
	if location.Path != "/" || location.Query().Get("trackerCredentials") != "jira" {
		return "location " + location.String()
	}
	return location.Query().Get("jiraOAuth")
}

// US1, AC6, AC9: a consent ends connected, every forged or stale callback
// stores nothing, and no log line carries the code, a token or the secret.
func TestTheJiraConsentRoundTrip(t *testing.T) {
	h, session, fake := oauthHandler(t)
	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	list := listCredentials(t, h, session)
	if !list.JiraOAuth.Configured || len(list.JiraOAuth.Sites) == 0 || list.JiraOAuth.Sites[0] != "https://acme.atlassian.net" {
		t.Fatalf("jiraOAuth: %+v", list.JiraOAuth)
	}

	// Without a session, or with a used state, nothing is stored.
	code := fake.Consent("Ada", "c-acme")
	if got := callback(h, nil, url.Values{"state": {startConsent(t, h, session)}, "code": {code}}); got != db.JiraOAuthInvalid {
		t.Errorf("no session: %s", got)
	}
	if got := callback(h, session, url.Values{"code": {fake.Consent("Ada", "c-acme")}}); got != db.JiraOAuthInvalid {
		t.Errorf("no state: %s", got)
	}
	if got := callback(h, session, url.Values{"state": {startConsent(t, h, session)}, "error": {"access_denied"}}); got != db.JiraOAuthCancelled {
		t.Errorf("declined: %s", got)
	}
	if len(listCredentials(t, h, session).Credentials) != 0 {
		t.Fatal("nothing may be stored yet")
	}

	state := startConsent(t, h, session)
	code = fake.Consent("Ada", "c-acme")
	if got := callback(h, session, url.Values{"state": {state}, "code": {code}}); got != db.JiraOAuthConnected {
		t.Fatalf("consent: %s", got)
	}
	stored := listCredentials(t, h, session).Credentials
	if len(stored) != 1 || stored[0].Kind != "oauth" || stored[0].Account != "Ada" || len(stored[0].GrantedSites) != 1 {
		t.Fatalf("stored: %+v", stored)
	}
	if got := callback(h, session, url.Values{"state": {state}, "code": {fake.Consent("Eve", "c-acme")}}); got != db.JiraOAuthInvalid {
		t.Errorf("a replayed state: %s", got)
	}
	if stored := listCredentials(t, h, session).Credentials; stored[0].Account != "Ada" {
		t.Fatalf("a replay replaced the grant: %+v", stored)
	}

	for _, secret := range []string{code, atlassiantest.ClientSecret, state} {
		if strings.Contains(logs.String(), secret) {
			t.Errorf("the log carries %q:\n%s", secret, logs.String())
		}
	}
}

// FR5: only a web session starts a consent; an unconfigured deployment says
// so with a code the web reads.
func TestConnectingNeedsASessionAndAConfiguredApp(t *testing.T) {
	h, session, _ := oauthHandler(t)
	w := httptest.NewRecorder()
	h.HandleUserTrackerCredentials(w, httptest.NewRequest(http.MethodPost, "/api/me/tracker-credentials/jira/connect", nil))
	if w.Code == http.StatusOK {
		t.Fatal("no session, no consent")
	}

	t.Setenv(db.JiraOAuthClientIDVar, "")
	w = httptest.NewRecorder()
	h.HandleUserTrackerCredentials(w, signedRequest(session, http.MethodPost, "/api/me/tracker-credentials/jira/connect", nil))
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "jira_oauth_not_configured") {
		t.Fatalf("not configured: %d %s", w.Code, w.Body)
	}
	if list := listCredentials(t, h, session); list.JiraOAuth.Configured {
		t.Error("the profile must say it is not configured")
	}
}

// US6, AC9: the app is an admin's, and its secret never comes back.
func TestTheJiraOAuthAppIsAnAdminsAndItsSecretWriteOnly(t *testing.T) {
	h, admin := credentialHandler(t)
	t.Setenv(db.JiraOAuthClientIDVar, "")
	t.Setenv(db.JiraOAuthClientSecretVar, "")
	t.Setenv(db.JiraOAuthRedirectURLVar, "")
	call := func(session *http.Cookie, method, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.HandleJiraOAuthApp(w, signedRequest(session, method, JiraOAuthAppPath, strings.NewReader(body)))
		return w
	}

	if w := call(admin, http.MethodPut, `{"clientId":"cid","redirectUrl":"https://sectile.example.com/auth/jira/callback"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("a first save without a secret: %d %s", w.Code, w.Body)
	}
	w := call(admin, http.MethodPut, `{"clientId":"cid","clientSecret":"top-secret-value","redirectUrl":"https://sectile.example.com/auth/jira/callback"}`)
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "top-secret-value") {
		t.Fatalf("save: %d %s", w.Code, w.Body)
	}
	var state db.OAuthAppState
	_ = json.Unmarshal(w.Body.Bytes(), &state)
	if !state.Configured || !state.SecretSet || state.Source != db.ServerCredentialStored || state.ClientID != "cid" {
		t.Fatalf("state: %+v", state)
	}
	if w := call(admin, http.MethodGet, ""); strings.Contains(w.Body.String(), "top-secret-value") {
		t.Fatal("the secret came back")
	}

	user, err := h.db.SignInLocal("bob@example.com")
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := h.db.CreateWebSession(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	member := &http.Cookie{Name: sessionCookie, Value: token}
	if w := call(member, http.MethodGet, ""); w.Code != http.StatusForbidden {
		t.Errorf("a member: %d", w.Code)
	}
	if !adminOnlyRoute(http.MethodGet, JiraOAuthAppPath) {
		t.Error("the route table must reserve it to admins")
	}

	if w := call(admin, http.MethodDelete, ""); w.Code != http.StatusOK || strings.Contains(w.Body.String(), `"configured":true`) {
		t.Fatalf("clear: %d %s", w.Code, w.Body)
	}
}
