package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// A sign-in that never completes must not leave a usable state behind.
const loginFlowTTL = 15 * time.Minute

// WebSessionTTL bounds how long one browser session stays valid without
// signing in again.
const WebSessionTTL = 12 * time.Hour

// ErrLoginFlow reports a callback that matches no pending sign-in, or one that
// has already been consumed or has expired.
var ErrLoginFlow = errors.New("unknown or expired sign-in attempt")

// LoginFlow is the server side of one in-progress sign-in.
type LoginFlow struct {
	Nonce        string
	CodeVerifier string
	Redirect     string
}

func (d *DB) initSessionSchema() error {
	queries := []string{
		// Only hashes are stored: a database copy must not yield a usable
		// session cookie or let a pending sign-in be hijacked.
		`CREATE TABLE IF NOT EXISTS web_sessions (
			token_hash TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL,
			revoked_at DATETIME,
			FOREIGN KEY (user_id) REFERENCES users(id)
		);`,
		`CREATE TABLE IF NOT EXISTS login_flows (
			state_hash TEXT PRIMARY KEY,
			nonce TEXT NOT NULL,
			code_verifier TEXT NOT NULL,
			redirect TEXT NOT NULL DEFAULT '/',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL,
			consumed_at DATETIME
		);`,
		`CREATE INDEX IF NOT EXISTS idx_web_sessions_user ON web_sessions(user_id);`,
	}
	for _, query := range queries {
		if _, err := d.conn.Exec(query); err != nil {
			return fmt.Errorf("session schema: %w", err)
		}
	}
	return nil
}

// StartLoginFlow records one pending sign-in and returns its state parameter.
// The state is what comes back from the provider, so only its hash is kept.
func (d *DB) StartLoginFlow(flow LoginFlow) (string, error) {
	state, err := newSecret()
	if err != nil {
		return "", err
	}
	redirect := strings.TrimSpace(flow.Redirect)
	if redirect == "" {
		redirect = "/"
	}
	_, err = d.conn.Exec(`INSERT INTO login_flows (state_hash, nonce, code_verifier, redirect, expires_at)
		VALUES (?, ?, ?, ?, ?)`,
		hashSecret(state), flow.Nonce, flow.CodeVerifier, redirect, time.Now().Add(loginFlowTTL).UTC())
	if err != nil {
		return "", err
	}
	return state, nil
}

// ConsumeLoginFlow matches a callback to its pending sign-in, exactly once.
// Consuming and reading happen in one transaction, so a replayed callback
// finds nothing.
func (d *DB) ConsumeLoginFlow(state string) (*LoginFlow, error) {
	tx, err := d.conn.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var flow LoginFlow
	var expires time.Time
	var consumed sql.NullTime
	err = tx.QueryRow(`SELECT nonce, code_verifier, redirect, expires_at, consumed_at
		FROM login_flows WHERE state_hash = ?`, hashSecret(state)).
		Scan(&flow.Nonce, &flow.CodeVerifier, &flow.Redirect, &expires, &consumed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrLoginFlow
	}
	if err != nil {
		return nil, err
	}
	if consumed.Valid || time.Now().UTC().After(expires) {
		return nil, ErrLoginFlow
	}
	if _, err = tx.Exec(`UPDATE login_flows SET consumed_at = ? WHERE state_hash = ?`,
		time.Now().UTC(), hashSecret(state)); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &flow, nil
}

// CreateWebSession opens a browser session for a user and returns the cookie
// value, which is never stored.
func (d *DB) CreateWebSession(userID string) (string, time.Time, error) {
	if strings.TrimSpace(userID) == "" {
		return "", time.Time{}, errors.New("userID is required")
	}
	token, err := newSecret()
	if err != nil {
		return "", time.Time{}, err
	}
	expires := time.Now().Add(WebSessionTTL).UTC()
	_, err = d.conn.Exec(`INSERT INTO web_sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?)`,
		hashSecret(token), userID, expires)
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expires, nil
}

// UserForWebSession resolves a session cookie to its user, or returns an empty
// string. Revoked and expired sessions resolve to nothing.
//
// A resolved session is also marked as seen, which is what the active-user
// count reads. The mark is written at most once per sessionTouchInterval: the
// read happens on every request, several times on some, and a write on each
// would turn every page load into a write lock on SQLite.
func (d *DB) UserForWebSession(token string) string {
	if strings.TrimSpace(token) == "" {
		return ""
	}
	hash := hashSecret(token)
	var userID string
	var expires time.Time
	var seen sql.NullTime
	err := d.conn.QueryRow(`SELECT user_id, expires_at, last_seen_at FROM web_sessions
		WHERE token_hash = ? AND revoked_at IS NULL`, hash).Scan(&userID, &expires, &seen)
	now := time.Now().UTC()
	if err != nil || now.After(expires) {
		return ""
	}
	if !seen.Valid || now.Sub(seen.Time) >= sessionTouchInterval {
		_, _ = d.conn.Exec(`UPDATE web_sessions SET last_seen_at = ? WHERE token_hash = ?`, now, hash)
	}
	return userID
}

// WebSessionOwner is the user a session cookie belongs to, revoked or expired
// alike, or "". Unlike UserForWebSession it marks nothing as seen: sign-out
// reads it to know whose presence just ended.
func (d *DB) WebSessionOwner(token string) string {
	if strings.TrimSpace(token) == "" {
		return ""
	}
	var userID string
	if err := d.conn.QueryRow(`SELECT user_id FROM web_sessions WHERE token_hash = ?`, hashSecret(token)).Scan(&userID); err != nil {
		return ""
	}
	return userID
}

// RevokeWebSession ends one browser session, on sign-out.
func (d *DB) RevokeWebSession(token string) error {
	_, err := d.conn.Exec(`UPDATE web_sessions SET revoked_at = ? WHERE token_hash = ? AND revoked_at IS NULL`,
		time.Now().UTC(), hashSecret(token))
	return err
}

// RevokeUserSessions ends every browser session of one account at once, which
// is what blocking an account has to do to mean anything: a refusal that only
// applies at the next sign-in leaves the open tab working.
func (d *DB) RevokeUserSessions(userID string) error {
	_, err := d.conn.Exec(`UPDATE web_sessions SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL`,
		time.Now().UTC(), strings.TrimSpace(userID))
	return err
}

// PurgeExpiredSessions drops what can no longer be used.
func (d *DB) PurgeExpiredSessions() error {
	now := time.Now().UTC()
	if _, err := d.conn.Exec(`DELETE FROM web_sessions WHERE expires_at < ?`, now); err != nil {
		return err
	}
	_, err := d.conn.Exec(`DELETE FROM login_flows WHERE expires_at < ? OR consumed_at IS NOT NULL`, now)
	return err
}
