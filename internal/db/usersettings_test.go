package db

import (
	"path/filepath"
	"testing"

	"tasks/internal/models"
)

func openUserSettingsDB(t *testing.T) *DB {
	t.Helper()
	database, err := NewDB(filepath.Join(t.TempDir(), "usersettings.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

// An account with no row of its own reads the deployment row, so an existing
// deployment shows what it always showed until someone saves a preference.
func TestUserSettingsSeedFromTheDeploymentRow(t *testing.T) {
	d := openUserSettingsDB(t)
	if _, err := d.UpdateSettings(models.Settings{Theme: "light", Language: "en", EditorCommand: "zed"}); err != nil {
		t.Fatal(err)
	}
	seeded, err := d.UserSettings("alice")
	if err != nil {
		t.Fatal(err)
	}
	if seeded.Theme != "light" || seeded.Language != "en" || seeded.EditorCommand != "zed" {
		t.Fatalf("seeded settings = %+v", seeded)
	}
}

// Two accounts on the same server keep their own preferences, which is the
// whole point of the split.
func TestUserSettingsAreIsolatedBetweenAccounts(t *testing.T) {
	d := openUserSettingsDB(t)
	if _, err := d.UpdateUserSettings("alice", models.Settings{Theme: "light", Density: "compact"}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.UpdateUserSettings("bob", models.Settings{Theme: "dark", Density: "comfortable"}); err != nil {
		t.Fatal(err)
	}
	alice, err := d.UserSettings("alice")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := d.UserSettings("bob")
	if err != nil {
		t.Fatal(err)
	}
	if alice.Theme != "light" || alice.Density != "compact" {
		t.Fatalf("alice = %+v", alice)
	}
	if bob.Theme != "dark" || bob.Density != "comfortable" {
		t.Fatalf("bob = %+v", bob)
	}
}

// A save carrying only the field it edits must not blank the others.
func TestUserSettingsOmittedKeyChangesNothing(t *testing.T) {
	d := openUserSettingsDB(t)
	if _, err := d.UpdateUserSettings("alice", models.Settings{Theme: "light", UserName: "Alice", ExternalTerminalCommand: "Ghostty"}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.UpdateUserSettings("alice", models.Settings{Theme: "dark"}); err != nil {
		t.Fatal(err)
	}
	alice, err := d.UserSettings("alice")
	if err != nil {
		t.Fatal(err)
	}
	if alice.Theme != "dark" {
		t.Fatalf("theme not updated: %+v", alice)
	}
	if alice.UserName != "Alice" || alice.ExternalTerminalCommand != "Ghostty" {
		t.Fatalf("omitted keys lost: %+v", alice)
	}
}

// The deployment row keeps the tracker and AI configuration whatever a member
// saves: a personal write never reaches it.
func TestUserSettingsWriteLeavesTheDeploymentRowAlone(t *testing.T) {
	d := openUserSettingsDB(t)
	if _, err := d.UpdateSettings(models.Settings{Theme: "light", IssueTracker: "github", GithubRepo: "acme/board"}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.UpdateUserSettings("alice", models.Settings{Theme: "dark"}); err != nil {
		t.Fatal(err)
	}
	deployment, err := d.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if deployment.Theme != "light" || deployment.IssueTracker != "github" || deployment.GithubRepo != "acme/board" {
		t.Fatalf("deployment row changed: %+v", deployment)
	}
}
