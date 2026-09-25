package db

import (
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/secrets"
)

// writeLegacyServerTokens puts clear-text server tokens where a version before
// #464 kept them, as an upgraded database carries them.
func writeLegacyServerTokens(t *testing.T, d *DB, github, jiraEmail, jiraToken string) {
	t.Helper()
	if _, err := d.conn.Exec(`UPDATE settings SET github_token = ?, jira_email = ?, jira_api_token = ? WHERE id = 1`,
		github, jiraEmail, jiraToken); err != nil {
		t.Fatal(err)
	}
}

func legacyColumns(t *testing.T, d *DB) string {
	t.Helper()
	var raw string
	if err := d.conn.QueryRow(`SELECT github_token || '|' || gitlab_token || '|' || jira_api_token || '|' || jira_email FROM settings WHERE id = 1`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestUpgradeSealsTheClearTextServerTokensOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	writeLegacyServerTokens(t, d, "ghp-legacy", "ada@example.com", "jira-legacy")
	d.Close()

	upgraded, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := legacyColumns(t, upgraded); got != "|||" {
		t.Fatalf("the clear-text columns must be blank after the upgrade, got %q", got)
	}
	client := upgraded.tracker("")
	if client.GithubToken != "ghp-legacy" || client.JiraEmail != "ada@example.com" || client.JiraToken != "jira-legacy" {
		t.Fatalf("the adopted credentials must be the ones in use: %q %q %q", client.GithubToken, client.JiraEmail, client.JiraToken)
	}
	state, err := upgraded.ServerTrackerCredentialState("github")
	if err != nil {
		t.Fatal(err)
	}
	if state.Source != ServerCredentialStored || state.Account != "" || state.CheckedAt != nil {
		t.Fatalf("an adopted credential is stored and not yet checked: %+v", state)
	}
	var record []byte
	if err := upgraded.conn.QueryRow(`SELECT record FROM server_tracker_credentials WHERE tracker = 'github'`).Scan(&record); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(record), "ghp-legacy") {
		t.Fatal("the adopted token is still in clear text")
	}
	upgraded.Close()

	// A second start has nothing to do and changes nothing.
	again, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	var againRecord []byte
	if err := again.conn.QueryRow(`SELECT record FROM server_tracker_credentials WHERE tracker = 'github'`).Scan(&againRecord); err != nil {
		t.Fatal(err)
	}
	if string(againRecord) != string(record) {
		t.Fatal("a second start sealed the token again")
	}
}

// A run interrupted between the insert and the blanking, or another instance
// that got there first, leaves a row and a column: the row wins, the column is
// blanked.
func TestUpgradeBlanksAColumnWhoseProviderIsAlreadyStored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SaveServerTrackerCredential("github", "", "stored-token", "octocat", "usr_admin"); err != nil {
		t.Fatal(err)
	}
	writeLegacyServerTokens(t, d, "ghp-legacy", "", "")
	d.Close()

	reopened, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if got := legacyColumns(t, reopened); got != "|||" {
		t.Fatalf("the column must be blanked, got %q", got)
	}
	if got := reopened.tracker("").GithubToken; got != "stored-token" {
		t.Fatalf("the stored row must win over the leftover column: %q", got)
	}
}

// With a clear-text token to seal and no key to seal it with, the server does
// not start, and the token is left where it was.
func TestUpgradeRefusesToLoseATokenItCannotSeal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	writeLegacyServerTokens(t, d, "ghp-legacy", "", "")
	d.Close()

	t.Setenv(secrets.KeyEnvVar, "not-a-key")
	if _, err := NewDB(path); err == nil || !strings.Contains(err.Error(), secrets.KeyEnvVar) {
		t.Fatalf("the start must be refused, naming %s: %v", secrets.KeyEnvVar, err)
	}

	t.Setenv(secrets.KeyEnvVar, "")
	reopened, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if got := reopened.tracker("").GithubToken; got != "ghp-legacy" {
		t.Fatalf("the token must have survived the refused start: %q", got)
	}
}

// Without anything to seal, a missing key is no reason not to start.
func TestUpgradeWithNothingToSealNeedsNoKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
	t.Setenv(secrets.KeyEnvVar, "not-a-key")
	reopened, err := NewDB(path)
	if err != nil {
		t.Fatalf("nothing to seal, the start must go on: %v", err)
	}
	reopened.Close()
}

// Migration 17 creates the store and removes the per-project tokens.
func TestMigrationSeventeenDropsTheProjectTokens(t *testing.T) {
	d := testDB(t)
	for _, probe := range []string{"SELECT github_token FROM projects", "SELECT gitlab_token FROM projects"} {
		if _, err := d.conn.Exec(probe); err == nil {
			t.Errorf("%s: the column must be gone", probe)
		}
	}
	if _, err := d.conn.Exec("SELECT tracker, email, record, account, checked_at, updated_at, updated_by FROM server_tracker_credentials"); err != nil {
		t.Fatalf("the store is missing: %v", err)
	}
}

func TestServerCredentialStatesListEveryProvider(t *testing.T) {
	d := testDB(t)
	d.trackers.GithubToken = "env-token"
	if err := d.SaveServerTrackerCredential("jira", "sync@example.com", "jira-token", "Sync", "usr_admin"); err != nil {
		t.Fatal(err)
	}
	states, err := d.ServerTrackerCredentialStates()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]ServerCredentialState{}
	for _, state := range states {
		got[state.Tracker] = state
	}
	if len(states) != 3 || got["github"].Source != ServerCredentialEnvironment || got["gitlab"].Source != ServerCredentialNone {
		t.Fatalf("states: %+v", states)
	}
	jira := got["jira"]
	if jira.Source != ServerCredentialStored || jira.Email != "sync@example.com" || jira.Account != "Sync" || jira.CheckedAt == nil || jira.UpdatedAt == nil {
		t.Fatalf("jira: %+v", jira)
	}
	if err := d.ClearServerTrackerCredential("jira"); err != nil {
		t.Fatal(err)
	}
	if state, _ := d.ServerTrackerCredentialState("jira"); state.Source != ServerCredentialNone {
		t.Fatalf("a cleared credential with no environment is none: %+v", state)
	}
}
