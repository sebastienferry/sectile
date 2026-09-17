package db

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Pairing codes are short lived on purpose: they exist only for the seconds
// between the web interface showing one and the desktop app exchanging it.
const pairingCodeTTL = 10 * time.Minute

// ErrPairingCode reports a code that is unknown, already used or expired. The
// three cases are deliberately indistinguishable to a caller.
var ErrPairingCode = errors.New("invalid or expired pairing code")

// DeviceCredential is one workstation bound to one user.
type DeviceCredential struct {
	ID        string
	UserID    string
	Label     string
	CreatedAt time.Time
	LastSeen  time.Time
}

func (d *DB) initIdentitySchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			subject TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL DEFAULT '',
			display_name TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		// Only the hash is stored: a database copy must not yield usable
		// credentials.
		`CREATE TABLE IF NOT EXISTS device_credentials (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			label TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_seen DATETIME,
			revoked_at DATETIME,
			FOREIGN KEY (user_id) REFERENCES users(id)
		);`,
		`CREATE TABLE IF NOT EXISTS pairing_codes (
			code_hash TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL,
			consumed_at DATETIME,
			FOREIGN KEY (user_id) REFERENCES users(id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_device_credentials_user ON device_credentials(user_id);`,
	}
	for _, query := range queries {
		if _, err := d.conn.Exec(query); err != nil {
			return fmt.Errorf("identity schema: %w", err)
		}
	}
	return nil
}

// hashSecret stores and compares credentials without keeping the plaintext.
// The secrets are 256 bits of randomness, so a single SHA-256 pass is enough:
// there is no low-entropy input to protect against a dictionary attack.
func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(secret)))
	return hex.EncodeToString(sum[:])
}

func newSecret() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

// UpsertUser records the identity the provider authenticated and returns its
// local ID. The subject is the provider's stable identifier, never the email:
// addresses get reassigned, subjects do not.
func (d *DB) UpsertUser(subject, email, displayName string) (string, error) {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "", errors.New("subject is required")
	}
	var id string
	err := d.conn.QueryRow(`SELECT id FROM users WHERE subject = ?`, subject).Scan(&id)
	if err == nil {
		_, err = d.conn.Exec(`UPDATE users SET email = ?, display_name = ? WHERE id = ?`, email, displayName, id)
		return id, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	id, err = newSecret()
	if err != nil {
		return "", err
	}
	id = "usr_" + id[:16]
	_, err = d.conn.Exec(`INSERT INTO users (id, subject, email, display_name) VALUES (?, ?, ?, ?)`,
		id, subject, email, displayName)
	return id, err
}

// CreatePairingCode issues a single-use code for the signed-in user. The
// plaintext is returned once and never stored.
func (d *DB) CreatePairingCode(userID string) (string, time.Time, error) {
	if strings.TrimSpace(userID) == "" {
		return "", time.Time{}, errors.New("userID is required")
	}
	code, err := newSecret()
	if err != nil {
		return "", time.Time{}, err
	}
	expires := time.Now().Add(pairingCodeTTL).UTC()
	_, err = d.conn.Exec(`INSERT INTO pairing_codes (code_hash, user_id, expires_at) VALUES (?, ?, ?)`,
		hashSecret(code), userID, expires)
	if err != nil {
		return "", time.Time{}, err
	}
	return code, expires, nil
}

// RedeemPairingCode consumes a code and returns a device credential bound to
// the same user. The code is marked consumed in the same transaction that
// creates the credential, so a replayed exchange yields nothing.
func (d *DB) RedeemPairingCode(code, label string) (string, *DeviceCredential, error) {
	tx, err := d.conn.Begin()
	if err != nil {
		return "", nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var userID string
	var expires time.Time
	var consumed sql.NullTime
	err = tx.QueryRow(`SELECT user_id, expires_at, consumed_at FROM pairing_codes WHERE code_hash = ?`,
		hashSecret(code)).Scan(&userID, &expires, &consumed)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, ErrPairingCode
	}
	if err != nil {
		return "", nil, err
	}
	if consumed.Valid || time.Now().UTC().After(expires) {
		return "", nil, ErrPairingCode
	}

	secret, err := newSecret()
	if err != nil {
		return "", nil, err
	}
	id, err := newSecret()
	if err != nil {
		return "", nil, err
	}
	id = "dev_" + id[:16]
	if _, err = tx.Exec(`INSERT INTO device_credentials (id, user_id, token_hash, label) VALUES (?, ?, ?, ?)`,
		id, userID, hashSecret(secret), label); err != nil {
		return "", nil, err
	}
	if _, err = tx.Exec(`UPDATE pairing_codes SET consumed_at = ? WHERE code_hash = ?`,
		time.Now().UTC(), hashSecret(code)); err != nil {
		return "", nil, err
	}
	if err = tx.Commit(); err != nil {
		return "", nil, err
	}
	return secret, &DeviceCredential{ID: id, UserID: userID, Label: label, CreatedAt: time.Now().UTC()}, nil
}

// UserForDeviceToken resolves a device credential to its user, or returns an
// empty string. Revoked credentials resolve to nothing.
func (d *DB) UserForDeviceToken(token string) string {
	if strings.TrimSpace(token) == "" {
		return ""
	}
	var userID string
	err := d.conn.QueryRow(`SELECT user_id FROM device_credentials WHERE token_hash = ? AND revoked_at IS NULL`,
		hashSecret(token)).Scan(&userID)
	if err != nil {
		return ""
	}
	// Best effort: a failed timestamp update must not deny a valid credential.
	_, _ = d.conn.Exec(`UPDATE device_credentials SET last_seen = ? WHERE token_hash = ?`,
		time.Now().UTC(), hashSecret(token))
	return userID
}

// ListDeviceCredentials returns the user's paired workstations.
func (d *DB) ListDeviceCredentials(userID string) ([]DeviceCredential, error) {
	// last_seen is selected as the bare column: the driver reads a DATETIME back
	// as a time only when it can see the column's declared type, and wrapping it
	// in COALESCE hid that type and made every listing fail to scan. The fallback
	// to the pairing date belongs in Go, where it costs nothing.
	rows, err := d.conn.Query(`SELECT id, user_id, label, created_at, last_seen
		FROM device_credentials WHERE user_id = ? AND revoked_at IS NULL ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var credentials []DeviceCredential
	for rows.Next() {
		var c DeviceCredential
		var lastSeen sql.NullTime
		if err := rows.Scan(&c.ID, &c.UserID, &c.Label, &c.CreatedAt, &lastSeen); err != nil {
			return nil, err
		}
		// A workstation that has never called in is shown as of its pairing.
		c.LastSeen = c.CreatedAt
		if lastSeen.Valid {
			c.LastSeen = lastSeen.Time
		}
		credentials = append(credentials, c)
	}
	return credentials, rows.Err()
}

// RevokeDeviceCredential disables one workstation without touching the others.
func (d *DB) RevokeDeviceCredential(userID, id string) error {
	result, err := d.conn.Exec(`UPDATE device_credentials SET revoked_at = ?
		WHERE id = ? AND user_id = ? AND revoked_at IS NULL`, time.Now().UTC(), id, userID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New("unknown device credential")
	}
	return nil
}

// PurgeExpiredPairingCodes drops codes that can no longer be redeemed.
func (d *DB) PurgeExpiredPairingCodes() error {
	_, err := d.conn.Exec(`DELETE FROM pairing_codes WHERE expires_at < ? OR consumed_at IS NOT NULL`,
		time.Now().UTC().Add(-pairingCodeTTL))
	return err
}

// User is the stored identity behind a session.
type User struct {
	ID          string
	Email       string
	DisplayName string
}

// GetUser reads one identity, or nil when it is unknown.
func (d *DB) GetUser(id string) (*User, error) {
	if strings.TrimSpace(id) == "" {
		return nil, nil
	}
	user := User{ID: id}
	err := d.conn.QueryRow(`SELECT email, display_name FROM users WHERE id = ?`, id).
		Scan(&user.Email, &user.DisplayName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}
