package db

import (
	"errors"
	"testing"
)

// The situation this file is about: a token stored while `default` was an
// acting identity, on a deployment that later grew real accounts without ever
// writing a `default` row.
func TestACredentialWithoutAnAccountIsReportedOrphaned(t *testing.T) {
	database := testDB(t)
	if err := database.EnsureUser("usr_real"); err != nil {
		t.Fatal(err)
	}
	if err := database.SetUserTrackerCredential(ImplicitUserID, "jira", "https://acme.atlassian.net", "ada@example.com", "ATATT-legacy", ""); err != nil {
		t.Fatal(err)
	}
	if err := database.SetUserTrackerCredential("usr_real", "jira", "https://acme.atlassian.net", "real@example.com", "ATATT-real", ""); err != nil {
		t.Fatal(err)
	}

	orphans, err := database.OrphanedTrackerCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if len(orphans) != 1 {
		t.Fatalf("expected the one ownerless credential, got %d: %+v", len(orphans), orphans)
	}
	if orphans[0].UserID != ImplicitUserID || orphans[0].Tracker != "jira" {
		t.Fatalf("wrong row reported: %+v", orphans[0])
	}
	// Enough to act on: which account the token was entered for, and where.
	if orphans[0].Email != "ada@example.com" || orphans[0].SiteURL != "https://acme.atlassian.net" {
		t.Fatalf("the report must name the account and the site: %+v", orphans[0])
	}
	if orphans[0].Sealed {
		t.Fatal("this one was stored unsealed")
	}
	if orphans[0].UpdatedAt.IsZero() {
		t.Fatal("the report must say when the access was registered")
	}
}

// The decision itself: detection never writes. An upgrade must not delete a row
// the agent paths may still be resolving, nor hand a token to another account.
func TestReportingOrphansChangesNothing(t *testing.T) {
	database := testDB(t)
	if err := database.EnsureUser("usr_real"); err != nil {
		t.Fatal(err)
	}
	if err := database.SetUserTrackerCredential(ImplicitUserID, "jira", "https://acme.atlassian.net", "ada@example.com", "ATATT-legacy", ""); err != nil {
		t.Fatal(err)
	}

	database.reportOrphanedTrackerCredentials()

	// The row is still there, still bound to the identity it was sealed for.
	if _, _, token, err := database.userTrackerCredential(ImplicitUserID, "jira"); err != nil || token != "ATATT-legacy" {
		t.Fatalf("the orphan must survive its own report: %q %v", token, err)
	}
	// And it was not silently handed to the account that does exist.
	site, email, token, err := database.UserTrackerCredentialsFor("usr_real", "jira")
	if err != nil {
		t.Fatal(err)
	}
	if site != "" || email != "" || token != "" {
		t.Fatalf("the orphan must not be rebound to another account: %q %q %q", site, email, token)
	}
}

// Why deleting it automatically is not free: the agent paths act as
// ImplicitUser, so the row is still resolvable there. This test exists to fail
// the day somebody makes the cleanup automatic.
func TestAnOrphanedCredentialStillServesTheIdentityItNames(t *testing.T) {
	database := testDB(t)
	if err := database.SetUserTrackerCredential(ImplicitUserID, "jira", "https://acme.atlassian.net", "ada@example.com", "ATATT-legacy", ""); err != nil {
		t.Fatal(err)
	}
	if _, _, token, err := database.UserTrackerCredentialsFor(ImplicitUserID, "jira"); err != nil || token != "ATATT-legacy" {
		t.Fatalf("the agent paths still resolve it: %q %v", token, err)
	}
}

// The documented way out for `default` specifically: an admin claims the
// account, as ADR 0015 says. The row stops being an orphan without a single
// byte of the record being touched, so the token keeps opening.
func TestClaimingTheAccountEndsTheOrphanWithoutResealing(t *testing.T) {
	database := testDB(t)
	if err := database.SetUserTrackerCredential(ImplicitUserID, "jira", "https://acme.atlassian.net", "ada@example.com", "ATATT-legacy", ""); err != nil {
		t.Fatal(err)
	}
	if err := database.EnsureUser(ImplicitUserID); err != nil {
		t.Fatal(err)
	}

	orphans, err := database.OrphanedTrackerCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if len(orphans) != 0 {
		t.Fatalf("an account now resolves it: %+v", orphans)
	}
	if _, _, token, err := database.userTrackerCredential(ImplicitUserID, "jira"); err != nil || token != "ATATT-legacy" {
		t.Fatalf("the record was never re-sealed, so it still opens: %q %v", token, err)
	}
}

func TestDiscardingAnOrphanRemovesIt(t *testing.T) {
	database := testDB(t)
	if err := database.SetUserTrackerCredential(ImplicitUserID, "jira", "https://acme.atlassian.net", "ada@example.com", "ATATT-legacy", ""); err != nil {
		t.Fatal(err)
	}
	if err := database.DiscardOrphanedTrackerCredential(ImplicitUserID, "JIRA"); err != nil {
		t.Fatalf("the tracker name is folded like everywhere else: %v", err)
	}
	if _, _, _, err := database.userTrackerCredential(ImplicitUserID, "jira"); !errors.Is(err, ErrNoUserCredential) {
		t.Fatalf("the row must be gone: %v", err)
	}
	orphans, err := database.OrphanedTrackerCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if len(orphans) != 0 {
		t.Fatalf("nothing left to report: %+v", orphans)
	}
}

// The guard that keeps the cleanup from becoming a way to delete a colleague's
// personal token by naming their id.
func TestDiscardRefusesACredentialThatHasAnOwner(t *testing.T) {
	database := testDB(t)
	if err := database.EnsureUser("usr_real"); err != nil {
		t.Fatal(err)
	}
	if err := database.SetUserTrackerCredential("usr_real", "jira", "https://acme.atlassian.net", "real@example.com", "ATATT-real", ""); err != nil {
		t.Fatal(err)
	}

	if err := database.DiscardOrphanedTrackerCredential("usr_real", "jira"); !errors.Is(err, ErrCredentialNotOrphaned) {
		t.Fatalf("a live account's credential must be refused: %v", err)
	}
	if _, _, token, err := database.userTrackerCredential("usr_real", "jira"); err != nil || token != "ATATT-real" {
		t.Fatalf("and left untouched: %q %v", token, err)
	}
}

func TestDiscardingWhatIsNotThereIsNotAnOrphan(t *testing.T) {
	database := testDB(t)
	if err := database.DiscardOrphanedTrackerCredential("usr_ghost", "jira"); !errors.Is(err, ErrNoUserCredential) {
		t.Fatalf("an unknown row is a missing credential, not a refused one: %v", err)
	}
	// A request naming nothing deletes nothing rather than sweeping the table.
	if err := database.DiscardOrphanedTrackerCredential("", ""); !errors.Is(err, ErrNoUserCredential) {
		t.Fatalf("an empty target must be refused: %v", err)
	}
}

// A sealed orphan is the case where rebinding is not merely declined but
// impossible: the server cannot open it at all. The report says so, so the
// interface can tell the person their only way out is re-entering the token.
func TestASealedOrphanIsReportedAsSealed(t *testing.T) {
	database := testDB(t)
	if err := database.SetUserTrackerCredential(ImplicitUserID, "jira", "https://acme.atlassian.net", "ada@example.com", "ATATT-legacy", "open sesame"); err != nil {
		t.Fatal(err)
	}
	orphans, err := database.OrphanedTrackerCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if len(orphans) != 1 || !orphans[0].Sealed {
		t.Fatalf("expected one sealed orphan: %+v", orphans)
	}
	// Once its unlock is forgotten, nothing can open it.
	if err := database.LockUserTrackerCredential(ImplicitUserID, "jira"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := database.userTrackerCredential(ImplicitUserID, "jira"); !errors.Is(err, ErrCredentialLocked) {
		t.Fatalf("a sealed orphan is unreadable by the server: %v", err)
	}

	// Discarding it still works: it needs no key. And an unlock kept for it
	// goes with it.
	if err := database.UnlockUserTrackerCredential(ImplicitUserID, "jira", "open sesame"); err != nil {
		t.Fatal(err)
	}
	if err := database.DiscardOrphanedTrackerCredential(ImplicitUserID, "jira"); err != nil {
		t.Fatal(err)
	}
	if n := unlockRows(t, database, ImplicitUserID); n != 0 {
		t.Fatalf("the unlock must not outlive the row it opens: %d left", n)
	}
}
