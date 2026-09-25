package db

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"tasks/internal/secrets"
)

// recordingRelay keeps what a store tells the other instances.
type recordingRelay struct {
	mu        sync.Mutex
	held      []SharedKey
	forgotten []string
}

func (r *recordingRelay) KeyHeld(shared SharedKey) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.held = append(r.held, shared)
}

func (r *recordingRelay) KeyForgotten(userID, tracker string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.forgotten = append(r.forgotten, userID+"/"+tracker)
}

func (r *recordingRelay) lastHeld(t *testing.T) SharedKey {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.held) == 0 {
		t.Fatal("no key was relayed")
	}
	return r.held[len(r.held)-1]
}

// twoCredentialStores opens two stores on one database file, as two server instances
// sharing one database would be: the rows are common, the held keys are not.
func twoCredentialStores(t *testing.T) (a, b *DB, path string) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "tasks.db")
	var err error
	if a, err = NewDB(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	if b, err = NewDB(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	return a, b, path
}

func TestAKeyUnlockedOnOneInstanceOpensOnAnother(t *testing.T) {
	a, b, _ := twoCredentialStores(t)
	relay := &recordingRelay{}
	a.SetUnlockedKeyRelay(relay)
	if err := a.SetUserTrackerCredential("u1", "jira", "https://acme.atlassian.net", "ada@example.com", "sealed-token", "open sesame"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := b.userTrackerCredential("u1", "jira"); !errors.Is(err, ErrCredentialLocked) {
		t.Fatalf("before sharing, B must read it locked: %v", err)
	}

	// Storing it sealed holds the key, and says so.
	if err := b.AdoptSharedKey(relay.lastHeld(t)); err != nil {
		t.Fatalf("adopting the stored key: %v", err)
	}
	if _, _, token, err := b.userTrackerCredential("u1", "jira"); err != nil || token != "sealed-token" {
		t.Fatalf("B after adopting: %q %v", token, err)
	}
	listed, err := b.UserTrackerCredentials("u1")
	if err != nil || len(listed) != 1 || !listed[0].Unlocked {
		t.Fatalf("B must list it unlocked: %+v %v", listed, err)
	}

	// So does an unlock.
	if err := a.LockUserTrackerCredential("u1", "jira"); err != nil {
		t.Fatal(err)
	}
	if err := a.UnlockUserTrackerCredential("u1", "jira", "open sesame"); err != nil {
		t.Fatal(err)
	}
	if err := b.AdoptSharedKey(relay.lastHeld(t)); err != nil {
		t.Fatalf("adopting the unlocked key: %v", err)
	}
	if _, _, token, err := b.userTrackerCredential("u1", "jira"); err != nil || token != "sealed-token" {
		t.Fatalf("B after the unlock: %q %v", token, err)
	}
}

// The lock guarantee does not depend on the message: an instance that never
// heard of the lock still cannot use its copy of the key.
func TestALockHoldsOnAnInstanceThatMissedIt(t *testing.T) {
	a, b, _ := twoCredentialStores(t)
	relay := &recordingRelay{}
	a.SetUnlockedKeyRelay(relay)
	if err := a.SetUserTrackerCredential("u1", "jira", "", "", "sealed-token", "open sesame"); err != nil {
		t.Fatal(err)
	}
	shared := relay.lastHeld(t)
	if err := b.AdoptSharedKey(shared); err != nil {
		t.Fatal(err)
	}

	if err := a.LockUserTrackerCredential("u1", "jira"); err != nil {
		t.Fatal(err)
	}
	if got := relay.forgotten; len(got) != 1 || got[0] != "u1/jira" {
		t.Fatalf("the lock must be relayed: %v", got)
	}
	// B is not told.
	if _, _, _, err := b.userTrackerCredential("u1", "jira"); !errors.Is(err, ErrCredentialLocked) {
		t.Fatalf("B still opened a locked credential: %v", err)
	}
	listed, _ := b.UserTrackerCredentials("u1")
	if len(listed) != 1 || listed[0].Unlocked {
		t.Fatalf("B must list it locked: %+v", listed)
	}
	// Nor can the old key be handed to it again.
	if err := b.AdoptSharedKey(shared); !errors.Is(err, ErrStaleKey) {
		t.Fatalf("re-adopting a key from before the lock: %v, want ErrStaleKey", err)
	}
}

// Storing the credential again retires every key held for the previous record,
// on every instance, and deleting it leaves nothing to open.
func TestANewRecordRetiresTheKeysHeldElsewhere(t *testing.T) {
	a, b, _ := twoCredentialStores(t)
	relay := &recordingRelay{}
	a.SetUnlockedKeyRelay(relay)
	if err := a.SetUserTrackerCredential("u1", "jira", "", "", "old-token", "old phrase"); err != nil {
		t.Fatal(err)
	}
	old := relay.lastHeld(t)
	if err := b.AdoptSharedKey(old); err != nil {
		t.Fatal(err)
	}
	if err := a.SetUserTrackerCredential("u1", "jira", "", "", "new-token", "new phrase"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := b.userTrackerCredential("u1", "jira"); !errors.Is(err, ErrCredentialLocked) {
		t.Fatalf("B opened the new record with the old key: %v", err)
	}
	if err := b.AdoptSharedKey(old); !errors.Is(err, ErrStaleKey) {
		t.Fatalf("the old key: %v, want ErrStaleKey", err)
	}
	if err := b.AdoptSharedKey(relay.lastHeld(t)); err != nil {
		t.Fatalf("the new key: %v", err)
	}
	if _, _, token, err := b.userTrackerCredential("u1", "jira"); err != nil || token != "new-token" {
		t.Fatalf("B with the new key: %q %v", token, err)
	}

	// Unsealing it forgets the key everywhere; the server key opens it.
	if err := a.SetUserTrackerCredential("u1", "jira", "", "", "plain-token", ""); err != nil {
		t.Fatal(err)
	}
	if n := len(relay.forgotten); n != 1 {
		t.Fatalf("storing it unsealed must relay one forgetting, got %d", n)
	}
	if _, _, token, err := b.userTrackerCredential("u1", "jira"); err != nil || token != "plain-token" {
		t.Fatalf("B after unsealing: %q %v", token, err)
	}

	if err := a.ClearUserTrackerCredential("u1", "jira"); err != nil {
		t.Fatal(err)
	}
	if n := len(relay.forgotten); n != 2 {
		t.Fatalf("deleting it must relay a forgetting, got %d", n)
	}
	if err := b.AdoptSharedKey(old); !errors.Is(err, ErrNoUserCredential) {
		t.Fatalf("adopting for a deleted credential: %v", err)
	}
}

// A key is kept only when it opens the record: one wrapped for somebody else,
// under another server key, or simply not the key, is refused.
func TestAForeignOrWrongKeyIsNotAdopted(t *testing.T) {
	a, b, _ := twoCredentialStores(t)
	relay := &recordingRelay{}
	a.SetUnlockedKeyRelay(relay)
	if err := a.SetUserTrackerCredential("u1", "jira", "", "", "sealed-token", "open sesame"); err != nil {
		t.Fatal(err)
	}
	genuine := relay.lastHeld(t)

	otherOwner := genuine
	otherOwner.UserID = "u2"
	if err := b.AdoptSharedKey(otherOwner); !errors.Is(err, secrets.ErrWrongKey) {
		t.Fatalf("a key wrapped for another owner: %v", err)
	}

	var otherServer secrets.Key
	otherServer[0] = 1
	foreign, err := secrets.WrapKey(otherServer, secrets.Binding{UserID: "u1", Tracker: "jira"}, secrets.Key{})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.AdoptSharedKey(SharedKey{UserID: "u1", Tracker: "jira", Generation: genuine.Generation, Wrapped: foreign}); !errors.Is(err, secrets.ErrWrongKey) {
		t.Fatalf("a key wrapped under another server key: %v", err)
	}

	wrong, err := b.shareKey("u1", "jira", secrets.DeriveKey("guess", []byte("0123456789abcdef")), genuine.Generation)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.AdoptSharedKey(wrong); !errors.Is(err, secrets.ErrWrongKey) {
		t.Fatalf("a well wrapped key that is not the key: %v", err)
	}
	if _, _, _, err := b.userTrackerCredential("u1", "jira"); !errors.Is(err, ErrCredentialLocked) {
		t.Fatalf("nothing refused may be kept: %v", err)
	}

	if err := a.SetUserTrackerCredential("u1", "github", "", "", "plain", ""); err != nil {
		t.Fatal(err)
	}
	unsealed := genuine
	unsealed.Tracker = "github"
	if err := b.AdoptSharedKey(unsealed); !errors.Is(err, ErrNotSealed) && !errors.Is(err, secrets.ErrWrongKey) {
		t.Fatalf("a key for an unsealed credential: %v", err)
	}
}

// What a starting instance pulls: every key held, wrapped.
func TestHeldKeysCanBeTakenOverByAStartingInstance(t *testing.T) {
	a, b, _ := twoCredentialStores(t)
	for _, tracker := range []string{"jira", "github"} {
		if err := a.SetUserTrackerCredential("u1", tracker, "", "", tracker+"-token", "phrase"); err != nil {
			t.Fatal(err)
		}
	}
	keys, err := a.HeldKeys()
	if err != nil || len(keys) != 2 {
		t.Fatalf("held keys: %d %v", len(keys), err)
	}
	for _, key := range keys {
		if err := b.AdoptSharedKey(key); err != nil {
			t.Fatalf("adopting %s: %v", key.Tracker, err)
		}
	}
	for _, tracker := range []string{"jira", "github"} {
		if _, _, token, err := b.userTrackerCredential("u1", tracker); err != nil || token != tracker+"-token" {
			t.Fatalf("%s on B: %q %v", tracker, token, err)
		}
	}
}

// No derived key reaches the database, in clear or wrapped, whatever an
// instance does with it.
func TestNoDerivedKeyIsWrittenToTheDatabase(t *testing.T) {
	a, b, path := twoCredentialStores(t)
	relay := &recordingRelay{}
	a.SetUnlockedKeyRelay(relay)
	if err := a.SetUserTrackerCredential("u1", "jira", "", "", "sealed-token", "open sesame"); err != nil {
		t.Fatal(err)
	}
	if err := a.LockUserTrackerCredential("u1", "jira"); err != nil {
		t.Fatal(err)
	}
	if err := a.UnlockUserTrackerCredential("u1", "jira", "open sesame"); err != nil {
		t.Fatal(err)
	}
	shared := relay.lastHeld(t)
	if err := b.AdoptSharedKey(shared); err != nil {
		t.Fatal(err)
	}
	held, ok := a.unlocked.get(unlockKey("u1", "jira"))
	if !ok {
		t.Fatal("A must hold the key")
	}

	files, _ := filepath.Glob(path + "*")
	if len(files) == 0 {
		t.Fatal("no database file to inspect")
	}
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(content, held.key[:]) {
			t.Fatalf("%s holds the derived key", filepath.Base(file))
		}
		if bytes.Contains(content, shared.Wrapped) {
			t.Fatalf("%s holds the wrapped key", filepath.Base(file))
		}
	}
}
