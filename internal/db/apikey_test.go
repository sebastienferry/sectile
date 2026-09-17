package db

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAPIKeyCarriesThePrefixAndTheDefaultExpiry(t *testing.T) {
	database := identityDB(t)
	userID, _ := database.UpsertUser("okta|carol", "", "")
	key, credential, err := database.CreateAPIKey(userID, " laptop ", DefaultAPIKeyTTL)
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	if !strings.HasPrefix(key, APIKeyPrefix) {
		t.Fatalf("key %q lacks the %q prefix", key, APIKeyPrefix)
	}
	if credential.Label != "laptop" {
		t.Fatalf("label %q not trimmed", credential.Label)
	}
	if credential.ExpiresAt == nil {
		t.Fatal("a key created with a period has no expiry")
	}
	remaining := time.Until(*credential.ExpiresAt)
	if remaining < DefaultAPIKeyTTL-time.Minute || remaining > DefaultAPIKeyTTL {
		t.Fatalf("expiry %s from now, want about %s", remaining, DefaultAPIKeyTTL)
	}
	looked, err := database.LookupDeviceToken(key)
	if err != nil || looked.UserID != userID || looked.ExpiresAt == nil {
		t.Fatalf("lookup = %+v, %v", looked, err)
	}
}

func TestAPIKeyWithoutExpiryNeverRunsOut(t *testing.T) {
	database := identityDB(t)
	userID, _ := database.UpsertUser("okta|dave", "", "")
	key, credential, err := database.CreateAPIKey(userID, "server", 0)
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	if credential.ExpiresAt != nil {
		t.Fatalf("a key created without a period expires at %s", credential.ExpiresAt)
	}
	if credential.ExpiresWithin(100 * 365 * 24 * time.Hour) {
		t.Fatal("a key without expiry reports an upcoming expiry")
	}
	if got := database.UserForDeviceToken(key); got != userID {
		t.Fatalf("resolved to %q, want %q", got, userID)
	}
}

func TestExpiredAPIKeyIsRefusedByName(t *testing.T) {
	database := identityDB(t)
	userID, _ := database.UpsertUser("okta|erin", "", "")
	key, credential, err := database.CreateAPIKey(userID, "old", DefaultAPIKeyTTL)
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	if _, err := database.conn.Exec(`UPDATE device_credentials SET expires_at = ? WHERE id = ?`,
		time.Now().Add(-time.Minute).UTC(), credential.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.LookupDeviceToken(key); !errors.Is(err, ErrAPIKeyExpired) {
		t.Fatalf("lookup error = %v, want ErrAPIKeyExpired", err)
	}
	if got := database.UserForDeviceToken(key); got != "" {
		t.Fatalf("an expired key still resolves to %q", got)
	}
	if _, err := database.LookupDeviceToken("sectile_never_issued"); !errors.Is(err, ErrAPIKeyUnknown) {
		t.Fatalf("unknown key error = %v, want ErrAPIKeyUnknown", err)
	}
}

func TestRenewalMovesTheExpiryAndKeepsTheSecret(t *testing.T) {
	database := identityDB(t)
	userID, _ := database.UpsertUser("okta|frank", "", "")
	key, credential, err := database.CreateAPIKey(userID, "laptop", time.Hour)
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	renewed, err := database.RenewDeviceCredential(userID, credential.ID, DefaultAPIKeyTTL)
	if err != nil {
		t.Fatalf("renew: %v", err)
	}
	if renewed == nil || time.Until(*renewed) < DefaultAPIKeyTTL-time.Minute {
		t.Fatalf("renewed expiry %v is not about %s away", renewed, DefaultAPIKeyTTL)
	}
	if got := database.UserForDeviceToken(key); got != userID {
		t.Fatal("the secret stopped working after renewal")
	}
	// Removing the expiry is the same call with no period.
	if cleared, err := database.RenewDeviceCredential(userID, credential.ID, 0); err != nil || cleared != nil {
		t.Fatalf("clear expiry = %v, %v", cleared, err)
	}
	listed, err := database.ListDeviceCredentials(userID)
	if err != nil || len(listed) != 1 || listed[0].ExpiresAt != nil {
		t.Fatalf("listing after clearing = %+v, %v", listed, err)
	}
	// Another user cannot renew it.
	if _, err := database.RenewDeviceCredential("usr_other", credential.ID, time.Hour); err == nil {
		t.Fatal("a stranger renewed the key")
	}
}

// Credentials issued before keys expired have no expires_at. They must keep
// resolving after the upgrade and be listed as keys without expiry.
func TestCredentialsFromBeforeExpiryKeepWorking(t *testing.T) {
	database := identityDB(t)
	userID, _ := database.UpsertUser("okta|grace", "", "")
	// The shape of a row written by the previous release: no prefix, no expiry.
	legacy := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if _, err := database.conn.Exec(`INSERT INTO device_credentials (id, user_id, token_hash, label) VALUES (?, ?, ?, ?)`,
		"dev_legacy", userID, hashSecret(legacy), "old laptop"); err != nil {
		t.Fatal(err)
	}
	credential, err := database.LookupDeviceToken(legacy)
	if err != nil || credential.UserID != userID {
		t.Fatalf("legacy credential = %+v, %v", credential, err)
	}
	if credential.ExpiresAt != nil {
		t.Fatalf("legacy credential acquired an expiry: %s", credential.ExpiresAt)
	}
	listed, err := database.ListDeviceCredentials(userID)
	if err != nil || len(listed) != 1 || listed[0].ExpiresAt != nil {
		t.Fatalf("listing = %+v, %v", listed, err)
	}
}

func TestPairingIssuesAKeyWithTheDefaultExpiry(t *testing.T) {
	database := identityDB(t)
	userID, _ := database.UpsertUser("okta|heidi", "", "")
	code, _, _ := database.CreatePairingCode(userID)
	key, credential, err := database.RedeemPairingCode(code, "desktop")
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}
	if !strings.HasPrefix(key, APIKeyPrefix) || credential.ExpiresAt == nil {
		t.Fatalf("pairing produced %q expiring %v", key, credential.ExpiresAt)
	}
}
