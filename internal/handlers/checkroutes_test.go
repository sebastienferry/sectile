package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tasks/internal/models"
	"tasks/internal/trackerapi"
)

// forgeAccounts answers GET /user for two tokens: the service account's and a
// person's, so a check says whose credential it actually used.
func forgeAccounts(t *testing.T, field string) *httptest.Server {
	t.Helper()
	accounts := map[string]string{"Bearer good-token": "service-bot", "Bearer alice-token": "alice"}
	instance := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, ok := accounts[r.Header.Get("Authorization")]
		if !strings.HasSuffix(r.URL.Path, "/user") || !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{field: name})
	}))
	t.Cleanup(instance.Close)
	return instance
}

// The Administration check of a server credential names the service account,
// even for an admin who stored a token of their own; the check of the personal
// form names the caller's own account and never falls back to the service
// account (ADR 0058).
func TestEachCheckNamesTheAccountItBelongsTo(t *testing.T) {
	github := forgeAccounts(t, "login")
	gitlab := forgeAccounts(t, "username")
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	if _, err := database.UpdateSettings(models.Settings{GithubApiUrl: github.URL, GitlabUrl: gitlab.URL}); err != nil {
		t.Fatal(err)
	}
	server := credentialServer(t, h)
	aliceID, alice := account(t, database, "alice@example.com")

	for _, provider := range []struct{ name, url string }{{"github", github.URL}, {"gitlab", gitlab.URL}} {
		if status, body := call(t, server, alice, http.MethodPut, ServerTrackerCredentialsPath+"/"+provider.name, `{"token":"good-token"}`); status != http.StatusOK {
			t.Fatalf("%s: saving the server credential: %d %s", provider.name, status, body)
		}
		personalCheck := `{"tracker":"` + provider.name + `","siteUrl":"` + provider.url + `"}`

		// No personal token yet: the personal check refuses rather than
		// answering with the service account.
		status, body := call(t, server, alice, http.MethodPost, "/api/setup/tracker/check", personalCheck)
		if status != http.StatusBadRequest || strings.Contains(body, "service-bot") {
			t.Fatalf("%s: a personal check without a personal token: %d %s", provider.name, status, body)
		}

		if err := database.SetUserTrackerCredential(aliceID, provider.name, provider.url, "", "alice-token", ""); err != nil {
			t.Fatal(err)
		}
		if status, body := call(t, server, alice, http.MethodPost, ServerTrackerCredentialsPath+"/"+provider.name+"/check", `{}`); status != http.StatusOK || !strings.Contains(body, "service-bot") || strings.Contains(body, `"alice"`) {
			t.Fatalf("%s: the Administration check must name the service account: %d %s", provider.name, status, body)
		}
		if status, body := call(t, server, alice, http.MethodPost, "/api/setup/tracker/check", personalCheck); status != http.StatusOK || !strings.Contains(body, `"alice"`) {
			t.Fatalf("%s: the personal check must name the caller's own account: %d %s", provider.name, status, body)
		}
	}
}

// A write refused for want of the server credential says so with its own code:
// the caller has nothing to add, an administrator has to set it (ADR 0058).
func TestAMissingServerCredentialAnswersItsOwnCode(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeTrackerError(recorder, http.StatusInternalServerError, &trackerapi.MissingServerCredentialError{Tracker: "gitlab"})
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusServiceUnavailable || body["code"] != trackerapi.ServerCredentialMissingCode || body["tracker"] != "gitlab" || !strings.Contains(body["error"], "Administration") {
		t.Fatalf("got %d %v", recorder.Code, body)
	}
	recorder = httptest.NewRecorder()
	writeTrackerError(recorder, http.StatusInternalServerError, &trackerapi.MissingPersonalCredentialError{Tracker: "gitlab"})
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), TrackerCredentialMissingCode) {
		t.Fatalf("a missing personal credential keeps its 403: %d %s", recorder.Code, recorder.Body.String())
	}
}
