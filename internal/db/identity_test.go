package db

import (
	"errors"
	"path/filepath"
	"testing"
)

func identityDB(t *testing.T) *DB {
	t.Helper()
	database, err := NewDB(filepath.Join(t.TempDir(), "identity.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func TestPairingCodeBindsWorkstationToUser(t *testing.T) {
	database := identityDB(t)
	userID, err := database.UpsertUser("okta|alice", "alice@example.com", "Alice")
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	code, _, err := database.CreatePairingCode(userID)
	if err != nil {
		t.Fatalf("create pairing code: %v", err)
	}
	token, credential, err := database.RedeemPairingCode(code, "laptop")
	if err != nil {
		t.Fatalf("redeem pairing code: %v", err)
	}
	if credential.UserID != userID {
		t.Fatalf("credential bound to %q, want %q", credential.UserID, userID)
	}
	if got := database.UserForDeviceToken(token); got != userID {
		t.Fatalf("device token resolved to %q, want %q", got, userID)
	}
}

func TestPairingCodeIsSingleUse(t *testing.T) {
	database := identityDB(t)
	userID, _ := database.UpsertUser("okta|bob", "", "")
	code, _, err := database.CreatePairingCode(userID)
	if err != nil {
		t.Fatalf("create pairing code: %v", err)
	}
	if _, _, err = database.RedeemPairingCode(code, "first"); err != nil {
		t.Fatalf("first redemption: %v", err)
	}
	if _, _, err = database.RedeemPairingCode(code, "replay"); !errors.Is(err, ErrPairingCode) {
		t.Fatalf("replayed redemption returned %v, want ErrPairingCode", err)
	}
}

func TestUnknownPairingCodeIsRejected(t *testing.T) {
	database := identityDB(t)
	if _, _, err := database.RedeemPairingCode("not-a-code", ""); !errors.Is(err, ErrPairingCode) {
		t.Fatalf("unknown code returned %v, want ErrPairingCode", err)
	}
}

func TestRevokedCredentialStopsResolving(t *testing.T) {
	database := identityDB(t)
	userID, _ := database.UpsertUser("okta|carol", "", "")
	code, _, _ := database.CreatePairingCode(userID)
	token, credential, err := database.RedeemPairingCode(code, "desktop")
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}
	if err := database.RevokeDeviceCredential(userID, credential.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if got := database.UserForDeviceToken(token); got != "" {
		t.Fatalf("revoked token resolved to %q, want empty", got)
	}
}

// Revoking one workstation must not disturb the others: that is the whole
// point of a credential per device rather than a shared secret.
func TestRevocationIsScopedToOneDevice(t *testing.T) {
	database := identityDB(t)
	userID, _ := database.UpsertUser("okta|dave", "", "")

	firstCode, _, _ := database.CreatePairingCode(userID)
	firstToken, first, _ := database.RedeemPairingCode(firstCode, "laptop")
	secondCode, _, _ := database.CreatePairingCode(userID)
	secondToken, _, _ := database.RedeemPairingCode(secondCode, "desktop")

	if err := database.RevokeDeviceCredential(userID, first.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if got := database.UserForDeviceToken(firstToken); got != "" {
		t.Fatalf("revoked device still resolves to %q", got)
	}
	if got := database.UserForDeviceToken(secondToken); got != userID {
		t.Fatalf("second device resolved to %q, want %q", got, userID)
	}
}

func TestDeviceCredentialsAreNotStoredInPlaintext(t *testing.T) {
	database := identityDB(t)
	userID, _ := database.UpsertUser("okta|erin", "", "")
	code, _, _ := database.CreatePairingCode(userID)
	token, _, err := database.RedeemPairingCode(code, "laptop")
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}
	var count int
	if err := database.conn.QueryRow(`SELECT COUNT(*) FROM device_credentials WHERE token_hash = ?`, token).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 0 {
		t.Fatal("device credential is stored in plaintext")
	}
}

func TestUpsertUserIsIdempotentOnSubject(t *testing.T) {
	database := identityDB(t)
	first, err := database.UpsertUser("okta|frank", "frank@example.com", "Frank")
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	second, err := database.UpsertUser("okta|frank", "frank@new.example.com", "Frank R.")
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if first != second {
		t.Fatalf("same subject produced %q then %q", first, second)
	}
}
