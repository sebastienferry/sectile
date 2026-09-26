package db

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tasks/internal/models"
	"tasks/internal/secrets"
	"tasks/internal/trackerapi"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

// The URLs still resolve project override, then user configuration, then
// environment; the token only ever comes from the server credential, stored
// first, then the environment (#464).
func TestTrackerCredentialsResolutionOrder(t *testing.T) {
	database := testDB(t)
	database.trackers.GithubURL = "https://env.example/api"
	database.trackers.GithubToken = "env-token"

	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Resolution", IssueTracker: "github", GithubRepo: "acme/app"})
	if err != nil {
		t.Fatal(err)
	}

	// Nothing stored: the environment answers.
	if got := database.tracker(project.ID); got.GithubToken != "env-token" || got.GithubURL != "https://env.example/api" {
		t.Fatalf("environment fallback: %q %q", got.GithubToken, got.GithubURL)
	}

	if _, err = database.UpdateSettings(models.Settings{GithubApiUrl: "https://settings.example/api"}); err != nil {
		t.Fatal(err)
	}
	if err = database.SaveServerTrackerCredential("github", "", "stored-token", "octocat", "usr_admin"); err != nil {
		t.Fatal(err)
	}
	if got := database.tracker(project.ID); got.GithubToken != "stored-token" || got.GithubURL != "https://settings.example/api" {
		t.Fatalf("the stored credential and the settings URL win: %q %q", got.GithubToken, got.GithubURL)
	}

	url := "https://project.example/api"
	if _, err = database.UpdateProject(project.ID, models.UpdateProjectRequest{GithubApiUrl: &url}); err != nil {
		t.Fatal(err)
	}
	if got := database.tracker(project.ID); got.GithubToken != "stored-token" || got.GithubURL != "https://project.example/api" {
		t.Fatalf("a project overrides its URL, never the credential: %q %q", got.GithubToken, got.GithubURL)
	}

	// Clearing the stored credential hands the provider back to the environment.
	if err = database.ClearServerTrackerCredential("github"); err != nil {
		t.Fatal(err)
	}
	if got := database.tracker(project.ID); got.GithubToken != "env-token" {
		t.Fatalf("clear falls back to the environment: %q", got.GithubToken)
	}
}

// A trailing slash on a stored URL must not produce a double slash in the
// request path, exactly as NewClient guarantees for the environment value.
func TestTrackerCredentialsTrimStoredURL(t *testing.T) {
	database := testDB(t)
	if _, err := database.UpdateSettings(models.Settings{GitlabUrl: "https://gitlab.example/api/v4/"}); err != nil {
		t.Fatal(err)
	}
	if got := database.tracker("").GitlabURL; got != "https://gitlab.example/api/v4" {
		t.Fatalf("got %q", got)
	}
}

// The settings payload writes no credential, whoever sends it: the server
// credentials are an admin's, through SaveServerTrackerCredential.
func TestSettingsWriteNoTrackerCredential(t *testing.T) {
	database := testDB(t)
	saved, err := database.UpdateSettings(models.Settings{GithubToken: "gh-secret", GitlabToken: "gl-secret", JiraAPIToken: "jira-secret", JiraEmail: "ada@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if saved.GithubToken != "" || saved.GitlabToken != "" || saved.JiraAPIToken != "" || saved.JiraEmail != "" {
		t.Fatalf("a credential was returned to the client: %+v", saved)
	}
	if saved.GithubTokenSet || saved.GitlabTokenSet || saved.JiraAPITokenSet {
		t.Fatalf("a settings payload stored a credential: %+v", saved)
	}
	if got := database.tracker(""); got.GithubToken != "" || got.GitlabToken != "" || got.JiraToken != "" || got.JiraEmail != "" {
		t.Fatalf("a settings payload reached the tracker client: %+v", got)
	}
	var raw string
	if err := database.conn.QueryRow(`SELECT github_token || gitlab_token || jira_api_token || jira_email FROM settings WHERE id = 1`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if raw != "" {
		t.Fatalf("the settings columns were written: %q", raw)
	}
}

// The settings answer says whether each provider is configured, and where
// from, and nothing else: that is all a member may know of it.
func TestSettingsReportWhereTheServerCredentialComesFrom(t *testing.T) {
	database := testDB(t)
	database.trackers.GithubToken = "env-token"
	database.trackers.JiraToken = "env-jira" // no e-mail: half a pair is none
	if err := database.SaveServerTrackerCredential("gitlab", "", "gl-stored", "", ""); err != nil {
		t.Fatal(err)
	}
	read, err := database.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if read.GithubTokenSet || !read.GithubTokenFromEnv {
		t.Fatalf("an environment credential must be reported as such: %+v", read)
	}
	if !read.GitlabTokenSet || read.GitlabTokenFromEnv {
		t.Fatalf("a stored credential must be reported as such: %+v", read)
	}
	if read.JiraAPITokenSet || read.JiraAPITokenFromEnv {
		t.Fatalf("a Jira token without its e-mail is no credential: %+v", read)
	}
	if read.GitlabToken != "" || read.GithubToken != "" {
		t.Fatalf("a token was returned: %+v", read)
	}
}

// A stored Jira credential wins as a whole: its e-mail is never replaced by
// the environment's, and an environment e-mail never completes it.
func TestAStoredJiraCredentialIsNeverMixedWithTheEnvironment(t *testing.T) {
	database := testDB(t)
	database.trackers.JiraEmail, database.trackers.JiraToken = "env@example.com", "env-jira"
	if err := database.SaveServerTrackerCredential("jira", "sync@example.com", "stored-jira", "Sync", "usr_admin"); err != nil {
		t.Fatal(err)
	}
	got := database.tracker("")
	if got.JiraEmail != "sync@example.com" || got.JiraToken != "stored-jira" {
		t.Fatalf("the stored pair must win whole: %q %q", got.JiraEmail, got.JiraToken)
	}
	if err := database.SaveServerTrackerCredential("jira", "", "no-email", "", ""); err == nil {
		t.Fatal("a Jira server credential without its e-mail must be refused")
	}
}

// A stored credential the key does not open fails the call rather than reach
// the tracker under the environment's account.
func TestAnUnreadableServerCredentialDoesNotFallBackToTheEnvironment(t *testing.T) {
	database := testDB(t)
	database.trackers.GithubToken = "env-token"
	if err := database.SaveServerTrackerCredential("github", "", "stored-token", "", ""); err != nil {
		t.Fatal(err)
	}
	other, err := secrets.ServerKey(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	database.serverKey = other

	if got := database.tracker(""); got.GithubToken != "" {
		t.Fatalf("the environment token replaced an unreadable stored one: %q", got.GithubToken)
	}
	_, err = database.tracker("").GithubGraphQL("{viewer{login}}")
	if err == nil || !strings.Contains(err.Error(), "cannot be decrypted") {
		t.Fatalf("the call must say the stored credential cannot be read: %v", err)
	}
	state, err := database.ServerTrackerCredentialState("github")
	if err != nil {
		t.Fatal(err)
	}
	if state.Source != ServerCredentialStored || !state.Unreadable {
		t.Fatalf("the page must show it stored and unreadable: %+v", state)
	}
	// It can still be replaced.
	if err := database.SaveServerTrackerCredential("github", "", "new-token", "", ""); err != nil {
		t.Fatal(err)
	}
	if got := database.tracker(""); got.GithubToken != "new-token" {
		t.Fatalf("replacing it must work: %q", got.GithubToken)
	}
}

// A credential saved through one handle is used by another on the same
// database without a restart: nothing caches it, so every replica sees it.
func TestAServerCredentialTakesEffectOnEveryInstance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.db")
	first, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if err := first.SaveServerTrackerCredential("github", "", "shared-token", "", ""); err != nil {
		t.Fatal(err)
	}
	if got := second.tracker("").GithubToken; got != "shared-token" {
		t.Fatalf("the second instance does not see the credential: %q", got)
	}
}

// The token is sealed at rest: the row holds no clear text.
func TestAServerCredentialIsSealedAtRest(t *testing.T) {
	database := testDB(t)
	if err := database.SaveServerTrackerCredential("github", "", "ghp-clear-text", "", ""); err != nil {
		t.Fatal(err)
	}
	var record []byte
	if err := database.conn.QueryRow(`SELECT record FROM server_tracker_credentials WHERE tracker = 'github'`).Scan(&record); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(record), "ghp-clear-text") {
		t.Fatal("the token is stored in clear text")
	}
	if _, err := secrets.Open(database.serverKey, secrets.Binding{UserID: "usr_admin", Tracker: "github"}, record); err == nil {
		t.Fatal("a server record opened as a personal one")
	}
}

func TestCheckTrackerCredentials(t *testing.T) {
	var seen string
	instance := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Authorization")
		if r.URL.Path == "/rest/api/3/myself" {
			// Jira authenticates the account: Basic e-mail:token.
			if seen != "Basic "+base64.StdEncoding.EncodeToString([]byte("ada@example.com:good-token")) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"accountId": "a1", "displayName": "Ada"})
			return
		}
		if seen != "Bearer good-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"login": "octocat", "username": "octocat"})
	}))
	defer instance.Close()

	database := testDB(t)
	database.trackers.HTTP = instance.Client()

	account, err := database.CheckTrackerCredentials(context.Background(), "github", instance.URL, "", "good-token")
	if err != nil || account != "octocat" {
		t.Fatalf("check: %q %v", account, err)
	}
	if account, err = database.CheckTrackerCredentials(context.Background(), "gitlab", instance.URL, "", "good-token"); err != nil || account != "octocat" {
		t.Fatalf("gitlab check: %q %v", account, err)
	}
	if _, err = database.CheckTrackerCredentials(context.Background(), "github", instance.URL, "", "wrong-token"); err == nil {
		t.Fatal("a wrong credential must be refused")
	}
	if account, err = database.CheckTrackerCredentials(context.Background(), "jira", instance.URL, "ada@example.com", "good-token"); err != nil || account != "Ada" {
		t.Fatalf("jira check: %q %v", account, err)
	}
	if _, err = database.CheckTrackerCredentials(context.Background(), "jira", instance.URL, "ada@example.com", "wrong-token"); err == nil {
		t.Fatal("a wrong Jira credential must be refused")
	}
}

// Saving a tracker's connection parameters keeps the rest of the
// configuration, and stores no credential.
func TestSaveTrackerCredentialsKeepsTheRestOfTheConfiguration(t *testing.T) {
	database := testDB(t)
	if _, err := database.UpdateSettings(models.Settings{Theme: "light"}); err != nil {
		t.Fatal(err)
	}
	saved, err := database.SaveTrackerCredentials("gitlab", "https://gitlab.example/api/v4", "group/app")
	if err != nil {
		t.Fatal(err)
	}
	if saved.GitlabProject != "group/app" || saved.GitlabUrl != "https://gitlab.example/api/v4" || saved.GitlabTokenSet {
		t.Fatalf("parameters not persisted, or a credential appeared: %+v", saved)
	}
	if saved.Theme != "light" {
		t.Fatalf("an unrelated preference was overwritten: %+v", saved)
	}
	jira, err := database.SaveTrackerCredentials("jira", "https://acme.atlassian.net", "pe")
	if err != nil {
		t.Fatal(err)
	}
	if jira.JiraUrl != "https://acme.atlassian.net" || jira.JiraProject != "PE" || jira.GitlabProject != "group/app" || jira.Theme != "light" {
		t.Fatalf("jira parameters not persisted, or the rest touched: %+v", jira)
	}
}

// A GitLab project resolves to the GitLab adapter (#398), and the stored
// parameters are the ones its calls carry: the instance and project of the
// settings, the server credential.
func TestStoredGitlabParametersReachTheGitlabAdapter(t *testing.T) {
	database := testDB(t)
	if _, err := database.UpdateSettings(models.Settings{GitlabUrl: "https://gitlab.example/api/v4", GitlabProject: "group/app"}); err != nil {
		t.Fatal(err)
	}
	if err := database.SaveServerTrackerCredential("gitlab", "", "gl-token", "", ""); err != nil {
		t.Fatal(err)
	}
	ts, err := database.trackerRegistry.ForProject(&models.Project{ID: "p", IssueTracker: "gitlab"})
	if err != nil || ts.Name() != "gitlab" {
		t.Fatalf("a GitLab project resolves to the GitLab adapter: %v %v", ts, err)
	}
	cred := database.trackerCredentials("")
	if cred.GitlabURL != "https://gitlab.example/api/v4" || cred.GitlabProject != "group/app" || cred.GitlabToken != "gl-token" {
		t.Fatalf("stored parameters: %+v", cred)
	}
}

// The agent configuration is a secret-free contract; the new credentials must
// not be what breaks it.
func TestAgentConfigCarriesNoTrackerToken(t *testing.T) {
	database := testDB(t)
	if _, err := database.UpdateSettings(models.Settings{IssueTracker: "github"}); err != nil {
		t.Fatal(err)
	}
	for tracker, token := range map[string]string{"github": "gh-secret", "gitlab": "gl-secret"} {
		if err := database.SaveServerTrackerCredential(tracker, "", token, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := database.SaveServerTrackerCredential("jira", "ada@example.com", "jira-secret", "", ""); err != nil {
		t.Fatal(err)
	}
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Secret free", GithubRepo: "acme/app"})
	if err != nil {
		t.Fatal(err)
	}
	config, err := database.AgentConfig(project.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"gh-secret", "gl-secret", "jira-secret"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("the agent configuration leaked %s: %s", secret, raw)
		}
	}
}

// A client with no resolver behaves as it did before the configuration existed.
func TestClientWithoutResolverIsUnchanged(t *testing.T) {
	client := &trackerapi.Client{GithubURL: "https://api.github.com", GithubToken: "env"}
	if client.For("any") != client {
		t.Fatal("For must return the client itself when nothing resolves")
	}
}

// A check is interactive: somebody is waiting for this one answer. A site that
// never answers must not hold them for the sixty seconds the tracker client
// allows its other calls.
func TestACredentialCheckGivesUpAfterFiveSeconds(t *testing.T) {
	if credentialCheckTimeout > 10*time.Second {
		t.Fatalf("a person waits for this answer: %v is too long", credentialCheckTimeout)
	}
	silent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer silent.Close()

	database := testDB(t)
	database.trackers.HTTP = silent.Client()

	started := time.Now()
	_, err := database.CheckTrackerCredentials(context.Background(), "jira", silent.URL, "ada@example.com", "token")
	elapsed := time.Since(started)

	if err == nil {
		t.Fatal("a site that never answers must be reported, not awaited")
	}
	if !strings.Contains(err.Error(), "secondes") {
		t.Errorf("the message must say the instance did not answer in time: %v", err)
	}
	if elapsed > credentialCheckTimeout+2*time.Second {
		t.Errorf("the check waited %v, past its own deadline", elapsed)
	}
}

// The evidence lookup used to resolve with neither a project nor a person, so a
// deployment whose only GitHub credential was the one somebody stored in their
// profile had none on this path: the stage transition failed with "configure
// SECTILE_TRACKER_TOKEN on the server" while every other tracker call worked.
func TestStagePRLookupUsesTheCallersOwnToken(t *testing.T) {
	database := testDB(t)
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Evidence", IssueTracker: "github", GithubRepo: "acme/app"})
	if err != nil {
		t.Fatal(err)
	}
	if err = database.EnsureUser("usr_alice"); err != nil {
		t.Fatal(err)
	}
	if err = database.SetUserTrackerCredential("usr_alice", "github", "", "", "alice-pat", ""); err != nil {
		t.Fatal(err)
	}

	// The person who asked travels under their own token.
	if got := database.trackerAs("usr_alice", "github", project.ID); got.GithubToken != "alice-pat" {
		t.Fatalf("the caller's own token is used: %q", got.GithubToken)
	}
	// Unattended work names nobody and uses the server credential.
	if err = database.SaveServerTrackerCredential("github", "", "server-token", "", ""); err != nil {
		t.Fatal(err)
	}
	if got := database.trackerAs("", "github", project.ID); got.GithubToken != "server-token" {
		t.Fatalf("no caller uses the server credential: %q", got.GithubToken)
	}
	// A person without a stored credential is not refused on GitHub: the
	// server's still answers, as it did before personal credentials existed.
	if got := database.trackerAs("usr_bob", "github", project.ID); got.GithubToken != "server-token" {
		t.Fatalf("a caller without a credential falls back: %q", got.GithubToken)
	}
}
