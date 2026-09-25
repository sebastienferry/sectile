package secrets

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testKey(t *testing.T) Key {
	t.Helper()
	key, err := ServerKey(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestSealedCredentialComesBackToItsOwner(t *testing.T) {
	key := testKey(t)
	binding := Binding{UserID: "u1", Tracker: "jira"}
	record, err := Seal(key, binding, "ATATT-secret")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(record, []byte("ATATT-secret")) {
		t.Fatal("the token must not appear in the record")
	}
	token, err := Open(key, binding, record)
	if err != nil || token != "ATATT-secret" {
		t.Fatalf("round trip: %q %v", token, err)
	}
}

// The question this whole package answers: someone with the database must not
// be able to reuse another user's token by moving its row.
func TestARecordDoesNotOpenForAnotherOwner(t *testing.T) {
	key := testKey(t)
	record, err := Seal(key, Binding{UserID: "victim", Tracker: "jira"}, "victim-token")
	if err != nil {
		t.Fatal(err)
	}
	for _, theft := range []Binding{
		{UserID: "thief", Tracker: "jira"},
		{UserID: "victim", Tracker: "github"},
		{UserID: "", Tracker: "jira"},
	} {
		if _, err := Open(key, theft, record); !errors.Is(err, ErrWrongKey) {
			t.Errorf("%+v opened a record it does not own: %v", theft, err)
		}
	}
}

func TestATamperedOrTruncatedRecordIsRefused(t *testing.T) {
	key := testKey(t)
	binding := Binding{UserID: "u1", Tracker: "jira"}
	record, err := Seal(key, binding, "token")
	if err != nil {
		t.Fatal(err)
	}
	flipped := append([]byte{}, record...)
	flipped[len(flipped)-1] ^= 0xff
	if _, err := Open(key, binding, flipped); !errors.Is(err, ErrWrongKey) {
		t.Errorf("a tampered record must be refused: %v", err)
	}
	if _, err := Open(key, binding, record[:4]); !errors.Is(err, ErrWrongKey) {
		t.Errorf("a truncated record must be refused: %v", err)
	}
	if _, err := Open(key, binding, nil); !errors.Is(err, ErrWrongKey) {
		t.Errorf("an empty record must be refused: %v", err)
	}
}

func TestTwoSealsOfTheSameTokenDiffer(t *testing.T) {
	key := testKey(t)
	binding := Binding{UserID: "u1", Tracker: "jira"}
	first, _ := Seal(key, binding, "same")
	second, _ := Seal(key, binding, "same")
	if bytes.Equal(first, second) {
		t.Fatal("a random nonce must make two records of the same token differ")
	}
}

func TestAnUnboundCredentialIsRefusedOutright(t *testing.T) {
	key := testKey(t)
	for _, binding := range []Binding{{Tracker: "jira"}, {UserID: "u1"}, {}} {
		if _, err := Seal(key, binding, "token"); err == nil {
			t.Errorf("%+v: a credential with no owner must not be sealed", binding)
		}
	}
}

func TestAnotherServerKeyOpensNothing(t *testing.T) {
	binding := Binding{UserID: "u1", Tracker: "jira"}
	record, err := Seal(testKey(t), binding, "token")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(testKey(t), binding, record); !errors.Is(err, ErrWrongKey) {
		t.Errorf("a copy of the database without the key must be useless: %v", err)
	}
}

// The sealing passphrase is the stronger mode: the key exists only while its
// owner supplies it, and nothing stored can confirm a guess.
func TestASealingPassphraseDerivesItsOwnKey(t *testing.T) {
	salt, err := NewSalt()
	if err != nil {
		t.Fatal(err)
	}
	binding := Binding{UserID: "u1", Tracker: "jira"}
	key := DeriveKey("open sesame", salt)
	record, err := Seal(key, binding, "sealed-token")
	if err != nil {
		t.Fatal(err)
	}
	if token, err := Open(DeriveKey("open sesame", salt), binding, record); err != nil || token != "sealed-token" {
		t.Fatalf("the same passphrase must open it: %q %v", token, err)
	}
	if _, err := Open(DeriveKey("wrong", salt), binding, record); !errors.Is(err, ErrWrongKey) {
		t.Errorf("a wrong passphrase must be refused: %v", err)
	}
	other, _ := NewSalt()
	if _, err := Open(DeriveKey("open sesame", other), binding, record); !errors.Is(err, ErrWrongKey) {
		t.Errorf("the salt must make one passphrase yield one key per record: %v", err)
	}
	if Fingerprint(key) == Fingerprint(DeriveKey("open sesame", other)) {
		t.Error("two salts must not derive the same key")
	}
}

func TestTheServerKeyIsReadFromTheEnvironmentFirst(t *testing.T) {
	dir := t.TempDir()
	// 32 bytes, hexadecimal, as an operator's secret manager would hand it over.
	t.Setenv(KeyEnvVar, strings.Repeat("ab", 32))
	key, err := ServerKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, keyFileName)); !os.IsNotExist(err) {
		t.Error("no file must be written when the environment carries the key")
	}
	t.Setenv(KeyEnvVar, "")
	fromFile, err := ServerKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	if Fingerprint(fromFile) == Fingerprint(key) {
		t.Error("the generated key must not be the environment one")
	}
	// Generated once, then reused: a new key at every start would strand every
	// stored credential.
	again, err := ServerKey(dir)
	if err != nil || Fingerprint(again) != Fingerprint(fromFile) {
		t.Fatalf("the key file must be reused: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, keyFileName))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("the key file must not be readable by the rest of the machine: %v", info.Mode().Perm())
	}
}

func TestAnUnusableServerKeyIsReported(t *testing.T) {
	t.Setenv(KeyEnvVar, "not-a-key")
	if _, err := ServerKey(t.TempDir()); err == nil {
		t.Error("an unreadable key must be reported, not silently replaced")
	}
	t.Setenv(KeyEnvVar, strings.Repeat("ab", 8))
	if _, err := ServerKey(t.TempDir()); err == nil {
		t.Error("a short key must be refused")
	}
	t.Setenv(KeyEnvVar, "")
	if _, err := ServerKey(""); err == nil {
		t.Error("with nowhere to put a key and none in the environment, say so")
	}
}

// A phrase typed with a stray space around it is the same phrase. Telling
// somebody their passphrase is wrong while they look at what they typed is the
// failure this avoids.
func TestSurroundingSpacesDoNotChangeThePassphrase(t *testing.T) {
	salt, err := NewSalt()
	if err != nil {
		t.Fatal(err)
	}
	reference := Fingerprint(DeriveKey("ma phrase", salt))
	for _, typed := range []string{" ma phrase", "ma phrase ", "  ma phrase  ", "\tma phrase\n"} {
		if Fingerprint(DeriveKey(typed, salt)) != reference {
			t.Errorf("%q must derive the same key", typed)
		}
	}
	// An inner space is part of the phrase, and still matters.
	if Fingerprint(DeriveKey("maphrase", salt)) == reference {
		t.Error("removing an inner space must change the key")
	}
}

// A server credential belongs to its tracker and to the server, never to a
// person: it opens under its own binding only.
func TestAServerRecordOpensAsItsTrackerServerCredentialOnly(t *testing.T) {
	key := testKey(t)
	record, err := Seal(key, ServerBinding("github"), "ghp-server")
	if err != nil {
		t.Fatal(err)
	}
	if token, err := Open(key, ServerBinding("GitHub"), record); err != nil || token != "ghp-server" {
		t.Fatalf("round trip: %q %v", token, err)
	}
	for _, theft := range []Binding{
		ServerBinding("jira"),
		{UserID: "", Tracker: "github"},
		{UserID: "admin", Tracker: "github"},
	} {
		if _, err := Open(key, theft, record); !errors.Is(err, ErrWrongKey) {
			t.Errorf("%+v opened the server record: %v", theft, err)
		}
	}

	personal, err := Seal(key, Binding{UserID: "admin", Tracker: "github"}, "ghp-personal")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(key, ServerBinding("github"), personal); !errors.Is(err, ErrWrongKey) {
		t.Errorf("a personal record opened as the server credential: %v", err)
	}
}

// The personal serialisation is what every stored record was sealed with: it
// must not move, or those records stop opening.
func TestThePersonalBindingSerialisationIsUnchanged(t *testing.T) {
	got := string(Binding{UserID: "u1", Tracker: "Jira"}.bytes())
	if want := "sectile:v1:user:2:u1:tracker:4:jira"; got != want {
		t.Fatalf("personal binding = %q, want %q", got, want)
	}
	if _, err := Seal(testKey(t), Binding{Server: true}, "x"); err == nil {
		t.Fatal("a server credential without a tracker must be refused")
	}
}
