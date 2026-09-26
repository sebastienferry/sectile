package db

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tasks/internal/models"
	"tasks/internal/secrets"
	"tasks/internal/tracker"
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

	// Locking forgets the unlock.
	if err := database.LockUserTrackerCredential("u1", "jira"); err != nil {
		t.Fatal(err)
	}
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
	if n := unlockRows(t, database, "u1"); n != 0 {
		t.Fatalf("the unlock of the old passphrase must go, %d left", n)
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
	if n := unlockRows(t, database, "u1"); n != 0 {
		t.Fatalf("the unlock must go with the credential, %d left", n)
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

	// Sealing derives its own key, so storing still works. Keeping it
	// unlocked needs the server key (#501): it is saved locked, and unlocking
	// it says why.
	if err := database.SetUserTrackerCredential("u1", "jira", "acme.atlassian.net", "ada@example.com", "token", "ma phrase"); err != nil {
		t.Fatalf("a sealed credential needs no server key: %v", err)
	}
	if _, _, _, err := database.userTrackerCredential("u1", "jira"); !errors.Is(err, ErrCredentialLocked) {
		t.Fatalf("with nowhere to keep its unlock, it must be saved locked: %v", err)
	}
	if err := database.UnlockUserTrackerCredential("u1", "jira", "ma phrase"); !errors.Is(err, ErrServerKeyUnavailable) {
		t.Fatalf("unlocking must name the missing server key: %v", err)
	}
}

// A task created on a Jira project by a signed-in person goes through the real
// adapter, which resolves their personal token from inside CreateTaskAs — a
// path that already holds the store's write lock. Resolving it used to take the
// read lock there, and sync.RWMutex is not re-entrant: the call never returned
// and the lock was never given back, wedging every other request until the
// server was restarted. The test bounds itself because a deadlock has no other
// symptom than never finishing.
func TestCreatingAThirdPartyTaskDoesNotWedgeTheStore(t *testing.T) {
	instance := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/rest/api/3/issue") && r.Method == http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "1", "key": "PE-1"})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{})
		}
	}))
	defer instance.Close()

	database := testDB(t)
	database.trackers.HTTP = instance.Client()
	if err := database.SetUserTrackerCredential("u-ada", "jira", instance.URL, "ada@example.com", "tok", ""); err != nil {
		t.Fatal(err)
	}
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Jira", IssueTracker: "jira", JiraProject: "PE", TrackerUrl: instance.URL})
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		ctx := tracker.WithActingUser(context.Background(), "u-ada")
		_, err := database.CreateTaskAs(ctx, models.CreateTaskRequest{Title: "Ticket", ProjectID: project.ID})
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("creation refused: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("creating the task never returned: the store is deadlocked on its own lock")
	}

	// And the store still answers, which a held write lock would prevent.
	if _, err := database.GetProjects(); err != nil {
		t.Fatalf("the store stayed locked: %v", err)
	}
}

// Three answers that used to say a thing was done when nothing was.
func TestAnActionOnNothingIsRefusedRatherThanReportedDone(t *testing.T) {
	database := testDB(t)

	// Forgetting a credential that is not there.
	if err := database.ClearUserTrackerCredential("u-ada", "jira"); !errors.Is(err, ErrNoUserCredential) {
		t.Fatalf("forgetting nothing must say so, got %v", err)
	}
	// Forgetting without naming a tracker deleted nothing and answered success,
	// so the screen showed "credential forgotten" over a credential still there.
	if err := database.ClearUserTrackerCredential("u-ada", ""); !errors.Is(err, ErrNoUserCredential) {
		t.Fatalf("a request naming no tracker must be refused, got %v", err)
	}

	// Unsealing what was never sealed.
	if err := database.SetUserTrackerCredential("u-ada", "jira", "https://acme.atlassian.net", "ada@example.com", "tok", ""); err != nil {
		t.Fatal(err)
	}
	if err := database.UnlockUserTrackerCredential("u-ada", "jira", "anything at all"); !errors.Is(err, ErrNotSealed) {
		t.Fatalf("an unsealed credential has nothing to unseal, got %v", err)
	}
	// And the real one still works.
	if err := database.ClearUserTrackerCredential("u-ada", "jira"); err != nil {
		t.Fatalf("forgetting a stored credential: %v", err)
	}
}

// The screen says "already configured, leave empty to keep it" over a token it
// never receives back. Demanding it again made every other change impossible to
// save: the site, the e-mail and the sealing could not be touched without
// retyping a secret the person may no longer have.
func TestSavingWithNoTokenKeepsTheStoredOne(t *testing.T) {
	database := testDB(t)
	if err := database.SetUserTrackerCredential("u-ada", "jira", "https://acme.atlassian.net", "ada@example.com", "ATATT-secret", ""); err != nil {
		t.Fatal(err)
	}

	// Only the site changes.
	if err := database.SetUserTrackerCredential("u-ada", "jira", "https://other.atlassian.net", "ada@example.com", "", ""); err != nil {
		t.Fatalf("saving without retyping the token: %v", err)
	}
	site, email, token, err := database.userTrackerCredential("u-ada", "jira")
	if err != nil || token != "ATATT-secret" {
		t.Fatalf("the stored token must survive: %q %v", token, err)
	}
	if site != "https://other.atlassian.net" || email != "ada@example.com" {
		t.Fatalf("the rest must have changed: %q %q", site, email)
	}

	// Sealing an existing credential without retyping it works too, and the
	// token that comes back out is still the same one.
	if err := database.SetUserTrackerCredential("u-ada", "jira", "https://other.atlassian.net", "ada@example.com", "", "open sesame"); err != nil {
		t.Fatalf("sealing without retyping the token: %v", err)
	}
	if _, _, token, err = database.userTrackerCredential("u-ada", "jira"); err != nil || token != "ATATT-secret" {
		t.Fatalf("the sealed token must still be the stored one: %q %v", token, err)
	}

	// Locked, it cannot be read to be written again, and says so rather than
	// failing on "the token is required".
	database.LockUserTrackerCredential("u-ada", "jira")
	err = database.SetUserTrackerCredential("u-ada", "jira", "https://other.atlassian.net", "ada@example.com", "", "")
	if err == nil || !strings.Contains(err.Error(), "scellé") {
		t.Fatalf("a locked credential must say to unseal it: %v", err)
	}

	// And with nothing stored at all, the token really is required.
	if err := database.SetUserTrackerCredential("u-bob", "jira", "https://acme.atlassian.net", "bob@example.com", "", ""); err == nil {
		t.Fatal("a first credential needs its token")
	}
}

// The account a credential belongs to is learnt when the tracker confirms it,
// forgotten with it, and unconfirmed again whenever the credential changes
// (#468).
func TestTheConfirmedAccountOfAPersonalCredential(t *testing.T) {
	database := testDB(t)
	if err := database.SetUserTrackerCredentialAccount("u1", "github", "ada"); !errors.Is(err, ErrNoUserCredential) {
		t.Fatalf("an account needs a credential to attach to: %v", err)
	}
	if err := database.SetUserTrackerCredential("u1", "github", "", "", "ghp-one", ""); err != nil {
		t.Fatal(err)
	}
	if accounts, err := database.TrackerAccounts("u1"); err != nil || len(accounts) != 0 {
		t.Fatalf("a credential nobody confirmed names no account: %v %v", accounts, err)
	}
	if err := database.SetUserTrackerCredentialAccount("u1", "GitHub ", "ada"); err != nil {
		t.Fatal(err)
	}
	if accounts, _ := database.TrackerAccounts("u1"); accounts["github"] != "ada" {
		t.Fatalf("accounts: %v", accounts)
	}
	if listed, _ := database.UserTrackerCredentials("u1"); len(listed) != 1 || listed[0].Account != "ada" {
		t.Fatalf("the profile lists the account: %+v", listed)
	}
	if accounts, _ := database.TrackerAccounts("u2"); len(accounts) != 0 {
		t.Fatalf("another user's accounts: %v", accounts)
	}

	// A new token may belong to somebody else.
	if err := database.SetUserTrackerCredential("u1", "github", "", "", "ghp-two", ""); err != nil {
		t.Fatal(err)
	}
	if accounts, _ := database.TrackerAccounts("u1"); len(accounts) != 0 {
		t.Fatalf("saving again must forget the account: %v", accounts)
	}

	if err := database.SetUserTrackerCredentialAccount("u1", "github", "ada"); err != nil {
		t.Fatal(err)
	}
	if err := database.ClearUserTrackerCredential("u1", "github"); err != nil {
		t.Fatal(err)
	}
	if accounts, _ := database.TrackerAccounts("u1"); len(accounts) != 0 {
		t.Fatalf("deleting the credential must forget the account: %v", accounts)
	}
}

// The account is a name, not the secret the seal protects: a sealed and
// locked credential keeps it.
func TestASealedLockedCredentialKeepsItsAccount(t *testing.T) {
	database := testDB(t)
	if err := database.SetUserTrackerCredential("u1", "jira", "https://acme.atlassian.net", "ada@example.com", "ATATT", "a long phrase"); err != nil {
		t.Fatal(err)
	}
	if err := database.SetUserTrackerCredentialAccount("u1", "jira", "Ada Lovelace"); err != nil {
		t.Fatal(err)
	}
	database.LockUserTrackerCredential("u1", "jira")
	if _, _, _, err := database.userTrackerCredential("u1", "jira"); !errors.Is(err, ErrCredentialLocked) {
		t.Fatalf("the credential must be locked: %v", err)
	}
	if accounts, err := database.TrackerAccounts("u1"); err != nil || accounts["jira"] != "Ada Lovelace" {
		t.Fatalf("accounts: %v %v", accounts, err)
	}
}

// Confirming asks the tracker with the stored token and records its answer; a
// check of the caller's own stored token records it too, a typed one does not.
func TestTheAccountIsLearntFromTheTracker(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch r.Header.Get("Authorization") {
		case "Bearer ghp-mine":
			_, _ = w.Write([]byte(`{"login":"ada"}`))
		case "Bearer ghp-other":
			_, _ = w.Write([]byte(`{"login":"bob"}`))
		default:
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer server.Close()

	database := testDB(t)
	database.trackers.GithubURL = server.URL
	if err := database.SetUserTrackerCredential("u1", "github", server.URL, "", "ghp-mine", ""); err != nil {
		t.Fatal(err)
	}
	account, err := database.ConfirmUserTrackerCredential(context.Background(), "u1", "github")
	if err != nil || account != "ada" {
		t.Fatalf("confirm: %q %v", account, err)
	}
	if accounts, _ := database.TrackerAccounts("u1"); accounts["github"] != "ada" {
		t.Fatalf("accounts after confirming: %v", accounts)
	}

	ctx := tracker.WithActingUser(context.Background(), "u1")
	if err := database.SetUserTrackerCredentialAccount("u1", "github", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := database.CheckTrackerCredentials(ctx, "github", server.URL, "", "ghp-other"); err != nil {
		t.Fatal(err)
	}
	if accounts, _ := database.TrackerAccounts("u1"); len(accounts) != 0 {
		t.Fatalf("a typed token proves nothing about the caller: %v", accounts)
	}
	if _, err := database.CheckTrackerCredentials(ctx, "github", "https://elsewhere.example", "", ""); err == nil {
		t.Fatal("another site must not answer")
	}
	if accounts, _ := database.TrackerAccounts("u1"); len(accounts) != 0 {
		t.Fatalf("a check on another site proves nothing: %v", accounts)
	}
	if _, err := database.CheckTrackerCredentials(ctx, "github", server.URL+"/", "", ""); err != nil {
		t.Fatal(err)
	}
	if accounts, _ := database.TrackerAccounts("u1"); accounts["github"] != "ada" {
		t.Fatalf("verifying the stored token records its account: %v", accounts)
	}

	if err := database.SetUserTrackerCredential("u1", "github", server.URL, "", "ghp-wrong", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ConfirmUserTrackerCredential(context.Background(), "u1", "github"); err == nil {
		t.Fatal("a refused token must be reported")
	}
	if accounts, _ := database.TrackerAccounts("u1"); len(accounts) != 0 {
		t.Fatalf("a refused token names no account: %v", accounts)
	}
	if calls == 0 {
		t.Fatal("the fake tracker was never asked")
	}
}
