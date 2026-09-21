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

// The resolution order is the whole point of the feature: what the user typed
// for this project wins over what they typed globally, which wins over what the
// server's shell exports.
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

	if _, err = database.UpdateSettings(models.Settings{GithubApiUrl: "https://settings.example/api", GithubToken: "settings-token"}); err != nil {
		t.Fatal(err)
	}
	if got := database.tracker(project.ID); got.GithubToken != "settings-token" || got.GithubURL != "https://settings.example/api" {
		t.Fatalf("settings win over environment: %q %q", got.GithubToken, got.GithubURL)
	}

	override, url := "project-token", "https://project.example/api"
	if _, err = database.UpdateProject(project.ID, models.UpdateProjectRequest{GithubToken: &override, GithubApiUrl: &url}); err != nil {
		t.Fatal(err)
	}
	if got := database.tracker(project.ID); got.GithubToken != "project-token" || got.GithubURL != "https://project.example/api" {
		t.Fatalf("project override wins: %q %q", got.GithubToken, got.GithubURL)
	}
	// A project with no override of its own still reads the user configuration.
	if got := database.tracker(""); got.GithubToken != "settings-token" {
		t.Fatalf("no project: %q", got.GithubToken)
	}
}

// A trailing slash on a stored URL must not produce a double slash in the
// request path, exactly as NewClient guarantees for the environment value.
func TestTrackerCredentialsTrimStoredURL(t *testing.T) {
	database := testDB(t)
	if _, err := database.UpdateSettings(models.Settings{GitlabUrl: "https://gitlab.example/api/v4/", GitlabToken: "t"}); err != nil {
		t.Fatal(err)
	}
	if got := database.tracker("").GitlabURL; got != "https://gitlab.example/api/v4" {
		t.Fatalf("got %q", got)
	}
}

func TestTrackerTokensAreWriteOnly(t *testing.T) {
	database := testDB(t)
	saved, err := database.UpdateSettings(models.Settings{GithubToken: "gh-secret", GitlabToken: "gl-secret", JiraAPIToken: "jira-secret"})
	if err != nil {
		t.Fatal(err)
	}
	if saved.GithubToken != "" || saved.GitlabToken != "" || saved.JiraAPIToken != "" {
		t.Fatalf("a token was returned to the client: %+v", saved)
	}
	if !saved.GithubTokenSet || !saved.GitlabTokenSet || !saved.JiraAPITokenSet {
		t.Fatalf("the Set flags do not report the stored tokens: %+v", saved)
	}
	if saved.GithubTokenFromEnv || saved.GitlabTokenFromEnv {
		t.Fatalf("a stored token must not be reported as coming from the environment: %+v", saved)
	}

	// An empty token keeps the stored one: the interface never received it back
	// and cannot resend it.
	if _, err = database.UpdateSettings(models.Settings{Theme: "light"}); err != nil {
		t.Fatal(err)
	}
	if got := database.tracker("").GithubToken; got != "gh-secret" {
		t.Fatalf("empty token did not keep the stored one: %q", got)
	}

	if _, err = database.UpdateSettings(models.Settings{GithubToken: TrackerTokenClearSentinel}); err != nil {
		t.Fatal(err)
	}
	read, err := database.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if read.GithubTokenSet || database.tracker("").GithubToken != "" {
		t.Fatalf("the clear sentinel did not delete the token: %+v", read)
	}
	if !read.GitlabTokenSet {
		t.Fatalf("clearing one token cleared another: %+v", read)
	}
}

func TestSettingsReportATokenSuppliedByTheEnvironment(t *testing.T) {
	database := testDB(t)
	database.trackers.GithubToken = "env-token"
	read, err := database.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if read.GithubTokenSet || !read.GithubTokenFromEnv {
		t.Fatalf("an environment token must be reported as such: %+v", read)
	}
}

func TestProjectTokensAreWriteOnly(t *testing.T) {
	database := testDB(t)
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Write only", GithubToken: "project-secret"})
	if err != nil {
		t.Fatal(err)
	}
	if project.GithubToken != "" || !project.GithubTokenSet {
		t.Fatalf("create leaked or lost the token: %+v", project)
	}
	read, err := database.GetProjectByID(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if read.GithubToken != "" || !read.GithubTokenSet {
		t.Fatalf("read leaked or lost the token: %+v", read)
	}
	// Saving an unrelated field must not wipe the credential.
	name := "Renamed"
	if _, err = database.UpdateProject(project.ID, models.UpdateProjectRequest{Name: &name}); err != nil {
		t.Fatal(err)
	}
	if got := database.tracker(project.ID).GithubToken; got != "project-secret" {
		t.Fatalf("update dropped the stored project token: %q", got)
	}
	empty := ""
	if _, err = database.UpdateProject(project.ID, models.UpdateProjectRequest{GithubToken: &empty}); err != nil {
		t.Fatal(err)
	}
	if got := database.tracker(project.ID).GithubToken; got != "project-secret" {
		t.Fatalf("an empty token must mean unchanged: %q", got)
	}
	sentinel := TrackerTokenClearSentinel
	if _, err = database.UpdateProject(project.ID, models.UpdateProjectRequest{GithubToken: &sentinel}); err != nil {
		t.Fatal(err)
	}
	if got := database.tracker(project.ID).GithubToken; got != "" {
		t.Fatalf("the clear sentinel must delete the project token: %q", got)
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

// A refused check leaves the user configuration untouched: that is what
// "check before save" buys.
func TestSaveTrackerCredentialsKeepsTheRestOfTheConfiguration(t *testing.T) {
	database := testDB(t)
	if _, err := database.UpdateSettings(models.Settings{Theme: "light", JiraAPIToken: "jira-secret"}); err != nil {
		t.Fatal(err)
	}
	saved, err := database.SaveTrackerCredentials("gitlab", "https://gitlab.example/api/v4", "group/app", "", "gl-token")
	if err != nil {
		t.Fatal(err)
	}
	if saved.GitlabProject != "group/app" || saved.GitlabUrl != "https://gitlab.example/api/v4" || !saved.GitlabTokenSet {
		t.Fatalf("parameters not persisted: %+v", saved)
	}
	if saved.Theme != "light" {
		t.Fatalf("an unrelated preference was overwritten: %+v", saved)
	}
	if !saved.JiraAPITokenSet {
		t.Fatal("saving GitLab parameters dropped the Jira token")
	}
	jira, err := database.SaveTrackerCredentials("jira", "https://acme.atlassian.net", "pe", "ada@example.com", "jira-token")
	if err != nil {
		t.Fatal(err)
	}
	if jira.JiraUrl != "https://acme.atlassian.net" || jira.JiraEmail != "ada@example.com" || jira.JiraProject != "PE" || !jira.JiraAPITokenSet {
		t.Fatalf("jira parameters not persisted: %+v", jira)
	}
	if !jira.GitlabTokenSet || jira.Theme != "light" {
		t.Fatalf("saving Jira parameters touched the rest: %+v", jira)
	}
}

// GitLab parameters are configuration, not a tracker: the registry still has no
// gitlab adapter, and saying so is more honest than half a synchronisation.
func TestStoredGitlabParametersDoNotRegisterATracker(t *testing.T) {
	database := testDB(t)
	if _, err := database.UpdateSettings(models.Settings{GitlabUrl: "https://gitlab.example/api/v4", GitlabProject: "group/app", GitlabToken: "gl-token"}); err != nil {
		t.Fatal(err)
	}
	_, err := database.trackerRegistry.ForProject(&models.Project{ID: "p", IssueTracker: "gitlab"})
	if err == nil || !strings.Contains(err.Error(), "aucun tracker distant configuré") {
		t.Fatalf("expected the registry's unconfigured-tracker error, got %v", err)
	}
}

// The agent configuration is a secret-free contract; the new credentials must
// not be what breaks it.
func TestAgentConfigCarriesNoTrackerToken(t *testing.T) {
	database := testDB(t)
	if _, err := database.UpdateSettings(models.Settings{GithubToken: "gh-secret", GitlabToken: "gl-secret", JiraAPIToken: "jira-secret", IssueTracker: "github"}); err != nil {
		t.Fatal(err)
	}
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Secret free", GithubRepo: "acme/app", GithubToken: "project-gh-secret", GitlabToken: "project-gl-secret"})
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
	for _, secret := range []string{"gh-secret", "gl-secret", "jira-secret", "project-gh-secret", "project-gl-secret"} {
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
	// Unattended work names nobody and keeps the project credential.
	override := "project-token"
	if _, err = database.UpdateProject(project.ID, models.UpdateProjectRequest{GithubToken: &override}); err != nil {
		t.Fatal(err)
	}
	if got := database.trackerAs("", "github", project.ID); got.GithubToken != "project-token" {
		t.Fatalf("no caller falls back to the project: %q", got.GithubToken)
	}
	// A person without a stored credential is not refused: the project's own
	// still answers, as it did before personal credentials existed.
	if got := database.trackerAs("usr_bob", "github", project.ID); got.GithubToken != "project-token" {
		t.Fatalf("a caller without a credential falls back: %q", got.GithubToken)
	}
}
