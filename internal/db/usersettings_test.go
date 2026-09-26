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
	if _, err := d.UpdateSettings(models.Settings{Theme: "light", Language: "en"}); err != nil {
		t.Fatal(err)
	}
	// Written before #305; still read, for the seed of a workstation.
	setLegacySettings(t, d, map[string]any{"editor_command": "zed"})
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
	if _, err := d.UpdateUserSettings("alice", models.Settings{Theme: "light", UserName: "Alice"}); err != nil {
		t.Fatal(err)
	}
	setLegacyColumns(t, d, "user_settings", "user_id", "alice", map[string]any{"external_terminal_command": "Ghostty"})
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

// Les deux échelles, celle du serveur et celle de l'interface, doivent offrir
// les mêmes crans : c'est le serveur qui borne ce qui est enregistré, et un
// cran offert par l'interface mais refusé ici s'appliquerait puis disparaîtrait
// au rechargement.
func TestUIScaleOptionsMatchTheInterfaceLadder(t *testing.T) {
	expected := []int{80, 90, 100, 112, 125, 150, 175}
	if len(UIScaleOptions) != len(expected) {
		t.Fatalf("%d crans attendus, %d obtenus : %v", len(expected), len(UIScaleOptions), UIScaleOptions)
	}
	for i, want := range expected {
		if UIScaleOptions[i] != want {
			t.Errorf("cran %d : %d attendu, %d obtenu", i, want, UIScaleOptions[i])
		}
	}
	// Les quatre crans historiques ne bougent pas : un réglage déjà choisi par
	// quelqu'un ne se déplace pas pour faire une plus jolie suite.
	for _, historical := range []int{90, 100, 112, 125} {
		if NormalizeUIScale(historical) != historical {
			t.Errorf("le cran historique %d a bougé : %d", historical, NormalizeUIScale(historical))
		}
	}
}

func TestNormalizeUIScaleSnapsRatherThanRefuses(t *testing.T) {
	// Une base plus ancienne ne porte pas la colonne : zéro vaut cent.
	if got := NormalizeUIScale(0); got != 100 {
		t.Errorf("100 attendu pour une valeur absente, obtenu %d", got)
	}
	if got := NormalizeUIScale(-10); got != 100 {
		t.Errorf("100 attendu pour une valeur négative, obtenu %d", got)
	}
	// Une valeur hors liste s'accroche au cran le plus proche : un réglage écrit
	// par une autre version ne doit pas rendre l'interface inutilisable.
	if got := NormalizeUIScale(111); got != 112 {
		t.Errorf("112 attendu pour 111, obtenu %d", got)
	}
	if got := NormalizeUIScale(10000); got != 175 {
		t.Errorf("175 attendu pour une valeur démesurée, obtenu %d", got)
	}
	if got := NormalizeUIScale(1); got != 80 {
		t.Errorf("80 attendu pour une valeur minuscule, obtenu %d", got)
	}
}
