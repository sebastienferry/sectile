package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tasks/internal/models"
)

// fakeGithub answers GET /user for one good token, which is all a check asks.
func fakeGithub(t *testing.T) *httptest.Server {
	t.Helper()
	instance := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" || r.Header.Get("Authorization") != "Bearer good-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"login": "octocat"})
	}))
	t.Cleanup(instance.Close)
	return instance
}

// credentialServer serves the real handlers the server credentials touch,
// behind the guard main.go installs.
func credentialServer(t *testing.T, h *Handler) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(ServerTrackerCredentialsPath, h.HandleServerTrackerCredentials)
	mux.HandleFunc(ServerTrackerCredentialsPath+"/", h.HandleServerTrackerCredentials)
	mux.HandleFunc("/api/setup/tracker", h.HandleTrackerSetup)
	mux.HandleFunc("/api/setup/tracker/check", h.HandleTrackerSetup)
	mux.HandleFunc("/api/settings", h.HandleSettings)
	server := httptest.NewServer(h.EnableCORS(h.RequireSession(mux)))
	t.Cleanup(server.Close)
	return server
}

func githubState(t *testing.T, server *httptest.Server, admin *http.Cookie) map[string]any {
	t.Helper()
	status, body := call(t, server, admin, http.MethodGet, ServerTrackerCredentialsPath, "")
	if status != http.StatusOK {
		t.Fatalf("list: %d %s", status, body)
	}
	var states []map[string]any
	if err := json.Unmarshal([]byte(body), &states); err != nil {
		t.Fatalf("decoding %s: %v", body, err)
	}
	for _, state := range states {
		if state["tracker"] == "github" {
			return state
		}
	}
	t.Fatalf("no github state in %s", body)
	return nil
}

func TestOnlyAnAdminReachesTheServerCredentials(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := credentialServer(t, h)
	_, _ = account(t, database, "alice@example.com") // the first account is the admin
	_, bob := account(t, database, "bob@example.com")

	for _, route := range []struct{ method, path, body string }{
		{http.MethodGet, ServerTrackerCredentialsPath, ""},
		{http.MethodPut, ServerTrackerCredentialsPath + "/github", `{"token":"good-token"}`},
		{http.MethodDelete, ServerTrackerCredentialsPath + "/github", ""},
		{http.MethodPost, ServerTrackerCredentialsPath + "/github/check", `{}`},
	} {
		if status, body := call(t, server, nil, route.method, route.path, route.body); status != http.StatusUnauthorized {
			t.Errorf("anonymous %s %s: %d %s", route.method, route.path, status, body)
		}
		if status, body := call(t, server, bob, route.method, route.path, route.body); status != http.StatusForbidden {
			t.Errorf("member %s %s: %d %s", route.method, route.path, status, body)
		}
	}
	if !adminOnlyRoute(http.MethodGet, ServerTrackerCredentialsPath) || !adminOnlyRoute(http.MethodPut, ServerTrackerCredentialsPath+"/jira") {
		t.Fatal("the route table must reserve the server credentials to admins")
	}
}

func TestAnAdminSetsChecksAndClearsTheServerCredential(t *testing.T) {
	instance := fakeGithub(t)
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	if _, err := database.UpdateSettings(models.Settings{GithubApiUrl: instance.URL}); err != nil {
		t.Fatal(err)
	}
	server := credentialServer(t, h)
	_, alice := account(t, database, "alice@example.com")

	// A refused check stores nothing.
	if status, body := call(t, server, alice, http.MethodPut, ServerTrackerCredentialsPath+"/github", `{"token":"wrong-token"}`); status != http.StatusBadRequest {
		t.Fatalf("a refused credential must not be saved: %d %s", status, body)
	}
	if state := githubState(t, server, alice); state["source"] != "none" {
		t.Fatalf("a failed check stored something: %+v", state)
	}

	status, body := call(t, server, alice, http.MethodPut, ServerTrackerCredentialsPath+"/github", `{"token":"good-token"}`)
	if status != http.StatusOK || !strings.Contains(body, `"account":"octocat"`) || !strings.Contains(body, `"source":"database"`) {
		t.Fatalf("save: %d %s", status, body)
	}
	if strings.Contains(body, "good-token") {
		t.Fatalf("the answer carried the token: %s", body)
	}
	if status, body := call(t, server, alice, http.MethodGet, ServerTrackerCredentialsPath, ""); strings.Contains(body, "good-token") {
		t.Fatalf("the list carried the token: %d %s", status, body)
	}

	// An empty check checks the credential in use.
	if status, body := call(t, server, alice, http.MethodPost, ServerTrackerCredentialsPath+"/github/check", `{}`); status != http.StatusOK || !strings.Contains(body, "octocat") {
		t.Fatalf("check of the stored credential: %d %s", status, body)
	}

	// Jira takes its e-mail with its token.
	if status, body := call(t, server, alice, http.MethodPut, ServerTrackerCredentialsPath+"/jira", `{"token":"t"}`); status != http.StatusBadRequest {
		t.Fatalf("a Jira token without its e-mail must be refused: %d %s", status, body)
	}
	if status, _ := call(t, server, alice, http.MethodPut, ServerTrackerCredentialsPath+"/linear", `{"token":"t"}`); status != http.StatusNotFound {
		t.Fatalf("an unknown tracker: %d", status)
	}

	status, body = call(t, server, alice, http.MethodDelete, ServerTrackerCredentialsPath+"/github", "")
	if status != http.StatusOK || !strings.Contains(body, `"source":"none"`) {
		t.Fatalf("clear: %d %s", status, body)
	}
}

// A member points a project at its tracker, but no payload of theirs reaches a
// server credential, and what they may read of it is whether it is configured.
func TestAMemberCannotWriteTheServerCredentialThroughTheSettingsOrTheSetup(t *testing.T) {
	instance := fakeGithub(t)
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := credentialServer(t, h)
	_, alice := account(t, database, "alice@example.com")
	_, bob := account(t, database, "bob@example.com")

	// The credential keys are no member keys any more: the payload is refused
	// and names them.
	if status, body := call(t, server, bob, http.MethodPut, "/api/settings", `{"githubToken":"bob-token","jiraApiToken":"bob-jira"}`); status != http.StatusForbidden || !strings.Contains(body, "githubToken") {
		t.Fatalf("settings: %d %s", status, body)
	}
	// An admin's is accepted and still stores nothing: the admin page is the
	// one write path.
	if status, body := call(t, server, alice, http.MethodPut, "/api/settings", `{"githubToken":"alice-token"}`); status != http.StatusOK {
		t.Fatalf("admin settings: %d %s", status, body)
	}
	if state := githubState(t, server, alice); state["source"] != "none" {
		t.Fatalf("a member's settings payload stored a server credential: %+v", state)
	}

	setup := `{"tracker":"github","siteUrl":"` + instance.URL + `","token":"good-token"}`
	status, body := call(t, server, bob, http.MethodPost, "/api/setup/tracker", setup)
	if status != http.StatusForbidden || !strings.Contains(body, "Administration") {
		t.Fatalf("a member's token through the setup must be refused: %d %s", status, body)
	}

	// An admin's goes to the server credentials, checked as the server's.
	if status, body := call(t, server, alice, http.MethodPost, "/api/setup/tracker", setup); status != http.StatusOK || strings.Contains(body, "good-token") {
		t.Fatalf("admin setup: %d %s", status, body)
	}
	if state := githubState(t, server, alice); state["source"] != "database" || state["account"] != "octocat" {
		t.Fatalf("the admin's setup did not store the server credential: %+v", state)
	}

	// The member still points the tracker, without a token.
	if status, body := call(t, server, bob, http.MethodPost, "/api/setup/tracker", `{"tracker":"github","siteUrl":"`+instance.URL+`","project":"acme/app"}`); status != http.StatusOK {
		t.Fatalf("a member setting the tracker without a token: %d %s", status, body)
	}

	status, body = call(t, server, bob, http.MethodGet, "/api/settings", "")
	if status != http.StatusOK {
		t.Fatalf("settings: %d %s", status, body)
	}
	var settings models.Settings
	if err := json.Unmarshal([]byte(body), &settings); err != nil {
		t.Fatal(err)
	}
	if !settings.GithubTokenSet || settings.GithubRepo != "acme/app" || strings.Contains(body, "good-token") {
		t.Fatalf("a member reads whether it is configured, never the value: %s", body)
	}
}
