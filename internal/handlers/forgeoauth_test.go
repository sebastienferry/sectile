package handlers

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"

	"tasks/internal/db"
	"tasks/internal/forgeoauth"
	"tasks/internal/forgeoauth/forgeoauthtest"
	"tasks/internal/models"
)

// forgeOAuthTrackers are the forges a consent can connect (#804).
var forgeOAuthTrackers = []string{"github", "gitlab"}

// forgeOAuthEnvPrefix is where the environment configures each forge's app.
var forgeOAuthEnvPrefix = map[string]string{"github": "SECTILE_GITHUB_OAUTH_", "gitlab": "SECTILE_GITLAB_OAUTH_"}

func setForgeOAuthApp(t *testing.T, tracker, clientID string) {
	t.Helper()
	prefix := forgeOAuthEnvPrefix[tracker]
	t.Setenv(prefix+"CLIENT_ID", clientID)
	t.Setenv(prefix+"CLIENT_SECRET", forgeoauthtest.ClientSecret)
	t.Setenv(prefix+"REDIRECT_URL", "https://sectile.example.com/auth/"+tracker+"/callback")
}

// forgeProject creates a project on the forge, on its public instance when
// apiURL is empty.
func forgeProject(t *testing.T, h *Handler, tracker, apiURL string) {
	t.Helper()
	request := models.CreateProjectRequest{Name: "Forge " + apiURL, IssueTracker: tracker, GithubRepo: "acme/app", GithubApiUrl: apiURL}
	if tracker == "gitlab" {
		request = models.CreateProjectRequest{Name: "Forge " + apiURL, IssueTracker: tracker, GitlabProject: "acme/app", GitlabUrl: apiURL}
	}
	if _, err := h.db.CreateProject(request); err != nil {
		t.Fatal(err)
	}
}

// forgeOAuthHandler is credentialHandler on a deployment with the forge's
// OAuth app, a fake of that forge, and a project on its public instance.
func forgeOAuthHandler(t *testing.T, tracker string) (*Handler, *http.Cookie, *forgeoauthtest.Fake) {
	t.Helper()
	setForgeOAuthApp(t, tracker, forgeoauthtest.ClientID)
	h, session := credentialHandler(t)
	provider, _ := forgeoauth.ForTracker(tracker)
	fake := forgeoauthtest.New(t, provider)
	h.db.SetForgeEndpoints(tracker, fake.Endpoints(), nil)
	forgeProject(t, h, tracker, "")
	return h, session, fake
}

type forgeCredentialList struct {
	Credentials []struct {
		Tracker      string `json:"tracker"`
		Kind         string `json:"kind"`
		Account      string `json:"account"`
		Disconnected bool   `json:"disconnected"`
	} `json:"credentials"`
	GithubOAuth struct {
		Configured bool `json:"configured"`
	} `json:"githubOAuth"`
	GitlabOAuth struct {
		Configured bool `json:"configured"`
	} `json:"gitlabOAuth"`
}

func (l forgeCredentialList) offered(tracker string) bool {
	if tracker == "github" {
		return l.GithubOAuth.Configured
	}
	return l.GitlabOAuth.Configured
}

func listForgeCredentials(t *testing.T, h *Handler, session *http.Cookie) forgeCredentialList {
	t.Helper()
	w := httptest.NewRecorder()
	h.HandleUserTrackerCredentials(w, signedRequest(session, http.MethodGet, "/api/me/tracker-credentials", nil))
	var list forgeCredentialList
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	return list
}

func connectForgePath(tracker string) string {
	return "/api/me/tracker-credentials/" + tracker + "/connect"
}

func startForgeConsent(t *testing.T, h *Handler, session *http.Cookie, tracker string, fake *forgeoauthtest.Fake) string {
	t.Helper()
	w := httptest.NewRecorder()
	h.HandleUserTrackerCredentials(w, signedRequest(session, http.MethodPost, connectForgePath(tracker), nil))
	if w.Code != http.StatusOK {
		t.Fatalf("connect: %d %s", w.Code, w.Body)
	}
	var answer struct {
		AuthorizeURL string `json:"authorizeUrl"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &answer)
	parsed, err := url.Parse(answer.AuthorizeURL)
	if err != nil || parsed.Query().Get("state") == "" || !strings.HasPrefix(answer.AuthorizeURL, fake.Endpoints().AuthorizeEndpoint) {
		t.Fatalf("authorize URL: %q", answer.AuthorizeURL)
	}
	return parsed.Query().Get("state")
}

func forgeCallback(h *Handler, session *http.Cookie, tracker string, query url.Values) string {
	path, handle := GithubOAuthCallbackPath, h.HandleGithubOAuthCallback
	if tracker == "gitlab" {
		path, handle = GitlabOAuthCallbackPath, h.HandleGitlabOAuthCallback
	}
	r := httptest.NewRequest(http.MethodGet, path+"?"+query.Encode(), nil)
	if session != nil {
		r.AddCookie(session)
	}
	w := httptest.NewRecorder()
	handle(w, r)
	if w.Code != http.StatusFound {
		return "status " + w.Result().Status
	}
	location, _ := url.Parse(w.Header().Get("Location"))
	if location.Path != "/" || location.Query().Get("trackerCredentials") != tracker || location.Query().Has("jiraOAuth") {
		return "location " + location.String()
	}
	return location.Query().Get("oauth")
}

// A consent ends connected, every forged or stale callback stores nothing, and
// no log line carries the code, a token or the secret.
func TestTheForgeConsentRoundTrip(t *testing.T) {
	for _, tracker := range forgeOAuthTrackers {
		t.Run(tracker, func(t *testing.T) {
			h, session, fake := forgeOAuthHandler(t, tracker)
			var logs bytes.Buffer
			log.SetOutput(&logs)
			t.Cleanup(func() { log.SetOutput(os.Stderr) })

			if !listForgeCredentials(t, h, session).offered(tracker) {
				t.Fatal("the profile must offer the connection")
			}

			// Without a session, or without a state, or declined, nothing is stored.
			code := fake.Consent("ada")
			if got := forgeCallback(h, nil, tracker, url.Values{"state": {startForgeConsent(t, h, session, tracker, fake)}, "code": {code}}); got != db.ForgeOAuthInvalid {
				t.Errorf("no session: %s", got)
			}
			if got := forgeCallback(h, session, tracker, url.Values{"code": {fake.Consent("ada")}}); got != db.ForgeOAuthInvalid {
				t.Errorf("no state: %s", got)
			}
			if got := forgeCallback(h, session, tracker, url.Values{"state": {startForgeConsent(t, h, session, tracker, fake)}, "error": {"access_denied"}}); got != db.ForgeOAuthCancelled {
				t.Errorf("declined: %s", got)
			}
			if len(listForgeCredentials(t, h, session).Credentials) != 0 {
				t.Fatal("nothing may be stored yet")
			}

			state := startForgeConsent(t, h, session, tracker, fake)
			code = fake.Consent("ada")
			if got := forgeCallback(h, session, tracker, url.Values{"state": {state}, "code": {code}}); got != db.ForgeOAuthConnected {
				t.Fatalf("consent: %s", got)
			}
			stored := listForgeCredentials(t, h, session).Credentials
			if len(stored) != 1 || stored[0].Tracker != tracker || stored[0].Kind != "oauth" || stored[0].Account != "ada" || stored[0].Disconnected {
				t.Fatalf("stored: %+v", stored)
			}
			if got := forgeCallback(h, session, tracker, url.Values{"state": {state}, "code": {fake.Consent("eve")}}); got != db.ForgeOAuthInvalid {
				t.Errorf("a replayed state: %s", got)
			}
			if stored := listForgeCredentials(t, h, session).Credentials; stored[0].Account != "ada" {
				t.Fatalf("a replay replaced the grant: %+v", stored)
			}

			// The callback answers GET only.
			post := httptest.NewRecorder()
			h.handleForgeOAuthCallback(tracker)(post, signedRequest(session, http.MethodPost, "/auth/"+tracker+"/callback", nil))
			if post.Code != http.StatusMethodNotAllowed {
				t.Errorf("a POST callback: %d", post.Code)
			}

			if !strings.Contains(logs.String(), "[ForgeOAuth] "+tracker) {
				t.Errorf("the log must name the forge:\n%s", logs.String())
			}
			for _, secret := range []string{code, forgeoauthtest.ClientSecret, state} {
				if strings.Contains(logs.String(), secret) {
					t.Errorf("the log carries %q:\n%s", secret, logs.String())
				}
			}
		})
	}
}

// Only the web session of the person starts a consent; an unconfigured
// deployment says so with a code the web reads.
func TestConnectingAForgeNeedsASessionAndAConfiguredApp(t *testing.T) {
	for _, tracker := range forgeOAuthTrackers {
		t.Run(tracker, func(t *testing.T) {
			h, session, _ := forgeOAuthHandler(t, tracker)
			w := httptest.NewRecorder()
			h.HandleUserTrackerCredentials(w, httptest.NewRequest(http.MethodPost, connectForgePath(tracker), nil))
			if w.Code == http.StatusOK {
				t.Fatal("no session, no consent")
			}

			// A caller the session cookie does not name, as an agent's key is,
			// is refused.
			w = httptest.NewRecorder()
			h.connectForge(tracker)(w, signedRequest(session, http.MethodPost, connectForgePath(tracker), nil), "usr_someone_else")
			if w.Code != http.StatusForbidden {
				t.Fatalf("another caller: %d %s", w.Code, w.Body)
			}

			setForgeOAuthApp(t, tracker, "")
			w = httptest.NewRecorder()
			h.HandleUserTrackerCredentials(w, signedRequest(session, http.MethodPost, connectForgePath(tracker), nil))
			if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), tracker+"_oauth_not_configured") {
				t.Fatalf("not configured: %d %s", w.Code, w.Body)
			}
			if listForgeCredentials(t, h, session).offered(tracker) {
				t.Error("the profile must say it is not configured")
			}
		})
	}
}

// The forge's app is an admin's, and its secret never comes back.
func TestTheForgeOAuthAppIsAnAdminsAndItsSecretWriteOnly(t *testing.T) {
	for _, tracker := range forgeOAuthTrackers {
		t.Run(tracker, func(t *testing.T) {
			h, admin := credentialHandler(t)
			prefix := forgeOAuthEnvPrefix[tracker]
			t.Setenv(prefix+"CLIENT_ID", "")
			t.Setenv(prefix+"CLIENT_SECRET", "")
			t.Setenv(prefix+"REDIRECT_URL", "")
			path, handle := GithubOAuthAppPath, h.HandleGithubOAuthApp
			if tracker == "gitlab" {
				path, handle = GitlabOAuthAppPath, h.HandleGitlabOAuthApp
			}
			redirect := "https://sectile.example.com/auth/" + tracker + "/callback"
			call := func(session *http.Cookie, method, body string) *httptest.ResponseRecorder {
				w := httptest.NewRecorder()
				handle(w, signedRequest(session, method, path, strings.NewReader(body)))
				return w
			}

			if w := call(admin, http.MethodPut, `{"clientId":"cid","redirectUrl":"`+redirect+`"}`); w.Code != http.StatusBadRequest {
				t.Fatalf("a first save without a secret: %d %s", w.Code, w.Body)
			}
			w := call(admin, http.MethodPut, `{"clientId":"cid","clientSecret":"top-secret-value","redirectUrl":"`+redirect+`"}`)
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
			if h.db.JiraOAuthConfigured() {
				t.Error("the forge's app must not configure Jira's")
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
			if !adminOnlyRoute(http.MethodGet, path) {
				t.Error("the route table must reserve it to admins")
			}

			if w := call(admin, http.MethodDelete, ""); w.Code != http.StatusOK || strings.Contains(w.Body.String(), `"configured":true`) {
				t.Fatalf("clear: %d %s", w.Code, w.Body)
			}
		})
	}
}

// Connect is offered only when a tracker of the forge is on its public
// instance, which is all a grant serves.
func TestForgeOAuthIsNotOfferedForASelfHostedTracker(t *testing.T) {
	selfHosted := map[string]string{"github": "https://github.acme.io/api/v3", "gitlab": "https://gitlab.acme.io/api/v4"}
	for _, tracker := range forgeOAuthTrackers {
		t.Run(tracker, func(t *testing.T) {
			setForgeOAuthApp(t, tracker, forgeoauthtest.ClientID)
			h, session := credentialHandler(t)
			if listForgeCredentials(t, h, session).offered(tracker) {
				t.Fatal("no tracker, no connection")
			}

			forgeProject(t, h, tracker, selfHosted[tracker])
			if sites, err := h.db.ConfiguredForgeTrackers(tracker); err != nil || !slices.Equal(sites, []string{selfHosted[tracker]}) {
				t.Fatalf("configured trackers: %v %v", sites, err)
			}
			if listForgeCredentials(t, h, session).offered(tracker) {
				t.Fatal("a self-hosted tracker only must not offer the connection")
			}

			forgeProject(t, h, tracker, "")
			if sites, err := h.db.ConfiguredForgeTrackers(tracker); err != nil || len(sites) != 2 {
				t.Fatalf("configured trackers: %v %v", sites, err)
			}
			if !listForgeCredentials(t, h, session).offered(tracker) {
				t.Fatal("a tracker on the public instance must offer the connection")
			}
		})
	}
}

// Disconnecting deletes the grant and withdraws it at the forge.
func TestDisconnectingAForgeGrantRevokesIt(t *testing.T) {
	for _, tracker := range forgeOAuthTrackers {
		t.Run(tracker, func(t *testing.T) {
			h, session, fake := forgeOAuthHandler(t, tracker)
			state := startForgeConsent(t, h, session, tracker, fake)
			if got := forgeCallback(h, session, tracker, url.Values{"state": {state}, "code": {fake.Consent("ada")}}); got != db.ForgeOAuthConnected {
				t.Fatalf("consent: %s", got)
			}

			w := httptest.NewRecorder()
			h.HandleUserTrackerCredentials(w, signedRequest(session, http.MethodDelete, "/api/me/tracker-credentials?tracker="+tracker, nil))
			if w.Code != http.StatusOK {
				t.Fatalf("disconnect: %d %s", w.Code, w.Body)
			}
			if stored := listForgeCredentials(t, h, session).Credentials; len(stored) != 0 {
				t.Fatalf("the grant must be forgotten: %+v", stored)
			}
			if got := fake.Revocations(); !slices.Equal(got, []string{"ada"}) {
				t.Fatalf("the grant must be revoked at the forge: %v", got)
			}
		})
	}
}
