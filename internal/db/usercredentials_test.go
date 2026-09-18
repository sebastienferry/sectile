package db

import (
	"errors"
	"strings"
	"tasks/internal/secrets"
	"testing"
)

func TestAPersonalTokenIsStoredEncryptedAndComesBack(t *testing.T) {
	database := testDB(t)
	if err := database.SetUserTrackerCredential("u1", "jira", "https://acme.atlassian.net", "ada@example.com", "ATATT-secret", ""); err != nil {
		t.Fatal(err)
	}

	// Nothing readable survives in the row itself.
	var record []byte
	if err := database.conn.QueryRow(`SELECT record FROM user_tracker_credentials WHERE user_id = ? AND tracker = ?`, "u1", "jira").Scan(&record); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(record), "ATATT-secret") {
		t.Fatal("the token must not be stored in clear")
	}

	site, email, token, err := database.userTrackerCredential("u1", "jira")
	if err != nil || site != "https://acme.atlassian.net" || email != "ada@example.com" || token != "ATATT-secret" {
		t.Fatalf("round trip: %q %q %q %v", site, email, token, err)
	}
}

// The property the whole design is for: a row copied into somebody else's name
// hands over nothing.
func TestAStolenRowDoesNotServeAnotherUser(t *testing.T) {
	database := testDB(t)
	if err := database.SetUserTrackerCredential("victim", "jira", "https://acme.atlassian.net", "victim@example.com", "victim-token", ""); err != nil {
		t.Fatal(err)
	}
	if err := database.SetUserTrackerCredential("thief", "jira", "https://acme.atlassian.net", "thief@example.com", "thief-token", ""); err != nil {
		t.Fatal(err)
	}
	// Somebody with write access to the database moves the victim's ciphertext
	// into their own row, which is the cheapest attack there is.
	if _, err := database.conn.Exec(`
		UPDATE user_tracker_credentials
		SET record = (SELECT record FROM user_tracker_credentials WHERE user_id = 'victim' AND tracker = 'jira')
		WHERE user_id = 'thief' AND tracker = 'jira'
	`); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := database.userTrackerCredential("thief", "jira"); !errors.Is(err, secrets.ErrWrongKey) {
		t.Fatalf("the stolen row must not open: %v", err)
	}
	// The victim still works.
	if _, _, token, err := database.userTrackerCredential("victim", "jira"); err != nil || token != "victim-token" {
		t.Fatalf("the owner keeps their credential: %q %v", token, err)
	}
}

func TestASealedTokenNeedsItsPassphrase(t *testing.T) {
	database := testDB(t)
	if err := database.SetUserTrackerCredential("u1", "jira", "https://acme.atlassian.net", "ada@example.com", "sealed-token", "open sesame"); err != nil {
		t.Fatal(err)
	}
	// Storing it leaves it unlocked for the session that stored it: the person
	// just typed the passphrase.
	if _, _, token, err := database.userTrackerCredential("u1", "jira"); err != nil || token != "sealed-token" {
		t.Fatalf("just stored: %q %v", token, err)
	}

	// A restart forgets every derived key.
	database.LockUserTrackerCredential("u1", "jira")
	if _, _, _, err := database.userTrackerCredential("u1", "jira"); !errors.Is(err, ErrCredentialLocked) {
		t.Fatalf("a sealed credential must be locked again: %v", err)
	}
	if err := database.UnlockUserTrackerCredential("u1", "jira", "wrong"); !errors.Is(err, secrets.ErrWrongKey) {
		t.Fatalf("a wrong passphrase must be refused: %v", err)
	}
	if _, _, _, err := database.userTrackerCredential("u1", "jira"); !errors.Is(err, ErrCredentialLocked) {
		t.Fatal("a failed unlock must leave it locked")
	}
	if err := database.UnlockUserTrackerCredential("u1", "jira", "open sesame"); err != nil {
		t.Fatal(err)
	}
	if _, _, token, err := database.userTrackerCredential("u1", "jira"); err != nil || token != "sealed-token" {
		t.Fatalf("after unlocking: %q %v", token, err)
	}
}

// A sealed credential is unreadable by the server alone: that is the whole
// point of the passphrase, and it is what a database plus key-file copy
// cannot defeat.
func TestTheServerKeyDoesNotOpenASealedToken(t *testing.T) {
	database := testDB(t)
	if err := database.SetUserTrackerCredential("u1", "jira", "https://acme.atlassian.net", "", "sealed-token", "passphrase"); err != nil {
		t.Fatal(err)
	}
	database.LockUserTrackerCredential("u1", "jira")

	var record []byte
	if err := database.conn.QueryRow(`SELECT record FROM user_tracker_credentials WHERE user_id = 'u1'`).Scan(&record); err != nil {
		t.Fatal(err)
	}
	if _, err := secrets.Open(database.serverKey, secrets.Binding{UserID: "u1", Tracker: "jira"}, record); !errors.Is(err, secrets.ErrWrongKey) {
		t.Fatalf("the server key must not open a sealed credential: %v", err)
	}
}

func TestReplacingACredentialRetiresTheKeyItWasSealedWith(t *testing.T) {
	database := testDB(t)
	if err := database.SetUserTrackerCredential("u1", "jira", "https://acme.atlassian.net", "", "first", "old passphrase"); err != nil {
		t.Fatal(err)
	}
	if err := database.SetUserTrackerCredential("u1", "jira", "https://acme.atlassian.net", "", "second", ""); err != nil {
		t.Fatal(err)
	}
	// Now unsealed: the server opens it, and the old passphrase is meaningless.
	credentials, err := database.UserTrackerCredentials("u1")
	if err != nil || len(credentials) != 1 || credentials[0].Sealed {
		t.Fatalf("credential state: %+v %v", credentials, err)
	}
	if _, _, token, err := database.userTrackerCredential("u1", "jira"); err != nil || token != "second" {
		t.Fatalf("the replacement must be what is served: %q %v", token, err)
	}
}

func TestListingCredentialsNeverCarriesAToken(t *testing.T) {
	database := testDB(t)
	if err := database.SetUserTrackerCredential("u1", "jira", "https://acme.atlassian.net", "ada@example.com", "ATATT-secret", ""); err != nil {
		t.Fatal(err)
	}
	if err := database.SetUserTrackerCredential("u1", "github", "https://acme.atlassian.net", "", "gh-secret", "phrase"); err != nil {
		t.Fatal(err)
	}
	database.LockUserTrackerCredential("u1", "github")

	credentials, err := database.UserTrackerCredentials("u1")
	if err != nil || len(credentials) != 2 {
		t.Fatalf("credentials: %+v %v", credentials, err)
	}
	for _, credential := range credentials {
		switch credential.Tracker {
		case "jira":
			if credential.Sealed || !credential.Unlocked || credential.Email != "ada@example.com" {
				t.Errorf("jira: %+v", credential)
			}
		case "github":
			if !credential.Sealed || credential.Unlocked {
				t.Errorf("github must read as sealed and locked: %+v", credential)
			}
		}
	}
	// Another user sees nothing of it.
	if other, err := database.UserTrackerCredentials("u2"); err != nil || len(other) != 0 {
		t.Fatalf("credentials must not leak across users: %+v %v", other, err)
	}
}

func TestClearingRemovesTheCredentialAndItsKey(t *testing.T) {
	database := testDB(t)
	if err := database.SetUserTrackerCredential("u1", "jira", "https://acme.atlassian.net", "", "token", "phrase"); err != nil {
		t.Fatal(err)
	}
	if err := database.ClearUserTrackerCredential("u1", "jira"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := database.userTrackerCredential("u1", "jira"); !errors.Is(err, ErrNoUserCredential) {
		t.Fatalf("a cleared credential must be gone: %v", err)
	}
	if err := database.UnlockUserTrackerCredential("u1", "jira", "phrase"); !errors.Is(err, ErrNoUserCredential) {
		t.Fatalf("unlocking a credential that no longer exists: %v", err)
	}
}

func TestStoringRefusesWhatItCannotAttribute(t *testing.T) {
	database := testDB(t)
	for _, testCase := range []struct{ user, tracker, token string }{
		{"", "jira", "token"},
		{"u1", "", "token"},
		{"u1", "jira", "   "},
	} {
		if err := database.SetUserTrackerCredential(testCase.user, testCase.tracker, "", "", testCase.token, ""); err == nil {
			t.Errorf("%+v must be refused", testCase)
		}
	}
}

// The resolution helper the tracker path uses: no personal credential is a
// normal answer, a locked one is an error the caller must surface.
func TestResolutionDistinguishesAbsentFromLocked(t *testing.T) {
	database := testDB(t)
	if site, email, token, err := database.UserTrackerCredentialsFor("u1", "jira"); err != nil || site != "" || email != "" || token != "" {
		t.Fatalf("no credential must resolve to nothing, without an error: %q %q %q %v", site, email, token, err)
	}
	if err := database.SetUserTrackerCredential("u1", "jira", "https://acme.atlassian.net", "ada@example.com", "token", "phrase"); err != nil {
		t.Fatal(err)
	}
	database.LockUserTrackerCredential("u1", "jira")
	if _, _, _, err := database.UserTrackerCredentialsFor("u1", "jira"); !errors.Is(err, ErrCredentialLocked) {
		t.Fatalf("a locked credential must be reported: %v", err)
	}
}

// The server must serve without a secret key: a read-only volume, or a
// deployment that never stores a personal credential, is not a reason to
// refuse to start. Only what needs the key refuses, and it says why.
func TestAMissingServerKeyBlocksOnlyWhatNeedsIt(t *testing.T) {
	database := testDB(t)
	database.serverKeyErr = errors.New("volume en lecture seule")

	err := database.SetUserTrackerCredential("u1", "jira", "acme.atlassian.net", "ada@example.com", "token", "")
	if err == nil {
		t.Fatal("an unsealed credential must not be stored under a key that could not be read")
	}
	if !strings.Contains(err.Error(), "SECTILE_SECRET_KEY") {
		t.Errorf("the refusal must name the way out: %v", err)
	}

	// Sealing derives its own key, so it still works.
	if err := database.SetUserTrackerCredential("u1", "jira", "acme.atlassian.net", "ada@example.com", "token", "ma phrase"); err != nil {
		t.Fatalf("a sealed credential needs no server key: %v", err)
	}
	if _, _, token, err := database.userTrackerCredential("u1", "jira"); err != nil || token != "token" {
		t.Fatalf("a sealed credential must still open: %q %v", token, err)
	}
}
