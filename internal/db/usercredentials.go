package db

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"tasks/internal/secrets"
)

// A tracker credential is personal. A Jira write is attributed to the account
// its token belongs to, so a shared token makes every comment and every
// transition look like one service account, whoever actually acted.
//
// The token is stored encrypted, bound to its owner and to the tracker, so a
// copy of the database yields nothing on its own and a row moved from one user
// to another stops opening. See internal/secrets for what that does and does
// not protect against.
//
// A user may additionally seal their credential behind a passphrase only they
// know. The cost is exact and stated at the point of choice: the server can
// only open it while they are there, so nothing running in the background can
// use it once they have left. Their unlock is kept in user_credential_unlocks,
// under the server key, while they are connected; see credentialunlocks.go.

// UserCredential is what the interface may know about a stored credential:
// everything except the token.
type UserCredential struct {
	Tracker string `json:"tracker"`
	// SiteURL belongs to the credential because an Atlassian account belongs to
	// a site: the person, their instance and their token travel together.
	SiteURL string `json:"siteUrl,omitempty"`
	Email   string `json:"email,omitempty"`
	// Account is who the tracker said the credential belongs to, the last time
	// it confirmed it: a GitHub login, a Jira display name. Empty until then.
	// It is a name rather than a secret, so it stays readable while the
	// credential is sealed and locked.
	Account string `json:"account,omitempty"`
	// Sealed says the credential needs its owner's passphrase; Unlocked says
	// the passphrase was supplied and its unlock has not been forgotten since.
	Sealed    bool      `json:"sealed"`
	Unlocked  bool      `json:"unlocked"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ErrNoUserCredential means the user stored none, which is not a failure: the
// caller falls back to the server-wide credential.
var ErrNoUserCredential = errors.New("no personal credential for this user")

// ErrCredentialLocked means a sealed credential needs its passphrase before
// anything can use it.
var ErrCredentialLocked = secrets.ErrSealed

// ErrNotSealed means there is nothing to unlock: the credential is stored
// unsealed, and the server key already opens it.
var ErrNotSealed = errors.New("this credential is not sealed")

// ErrServerKeyUnavailable means an unlock cannot be kept: it is stored under the
// server key, and without it the unlock would either be lost at once or have to
// be written down in clear, which would defeat the passphrase.
var ErrServerKeyUnavailable = errors.New("la clé de chiffrement du serveur est indisponible")

// ensureUserCredentialsTable creates the table and applies its one additive
// migration. It is called once, while the schema is being built: doing it on
// every read issued DDL under a read lock, and put a CREATE and an
// always-failing ALTER in front of every single tracker call.
func (d *DB) ensureUserCredentialsTable() {
	_, _ = d.conn.Exec(`CREATE TABLE IF NOT EXISTS user_tracker_credentials (
		user_id TEXT NOT NULL,
		tracker TEXT NOT NULL,
		site_url TEXT NOT NULL DEFAULT '',
		email TEXT NOT NULL DEFAULT '',
		record BLOB NOT NULL,
		sealed INTEGER NOT NULL DEFAULT 0,
		salt BLOB,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, tracker)
	);`)
	// The site moved into the credential once it became clear an account
	// belongs to an instance. Additive, and refused on a fresh database, like
	// every other migration here.
	if d.dialect.RunsLegacyMigrations() {
		_, _ = d.conn.Exec(`ALTER TABLE user_tracker_credentials ADD COLUMN site_url TEXT NOT NULL DEFAULT '';`)
	}
}

// SetUserTrackerCredential stores one person's token for one tracker. An empty
// passphrase keeps it openable by the server, which is what lets it be used
// without its owner present; a passphrase seals it, and the passphrase itself
// is never stored in any form.
func (d *DB) SetUserTrackerCredential(userID, tracker, siteURL, email, token, passphrase string) error {
	userID = strings.TrimSpace(userID)
	tracker = strings.ToLower(strings.TrimSpace(tracker))
	token = strings.TrimSpace(token)
	if userID == "" {
		return fmt.Errorf("sign in before storing a personal credential")
	}
	if tracker == "" {
		return fmt.Errorf("name the tracker this credential is for")
	}
	binding := secrets.Binding{UserID: userID, Tracker: tracker}
	if token == "" {
		// The screen says "already configured, leave empty to keep it", and
		// that has to be true: the token is never sent back, so demanding it
		// again made every other change (the site, the e-mail, the sealing)
		// impossible to save without retyping a secret the person may not have
		// kept. A sealed credential has to be open for this: re-storing it
		// re-encrypts it, and a locked one cannot be read to be re-encrypted.
		existing, err := d.userTrackerCredentialToken(userID, tracker)
		switch {
		case errors.Is(err, ErrCredentialLocked):
			return fmt.Errorf("votre jeton est scellé et verrouillé : descellez-le, ou saisissez-le à nouveau")
		case errors.Is(err, ErrNoUserCredential):
			return fmt.Errorf("the token is required")
		case err != nil:
			return err
		}
		token = existing
	}
	key := d.serverKey
	var salt []byte
	sealed := strings.TrimSpace(passphrase) != ""
	if !sealed && d.serverKeyErr != nil {
		// A zero key is a valid AES key: storing under it would look like
		// encryption and protect nothing. Sealing is still available, since it
		// derives its own key from the passphrase.
		return fmt.Errorf("la clé de chiffrement du serveur est indisponible (%w) : scellez votre jeton avec une phrase, ou définissez %s sur le serveur", d.serverKeyErr, secrets.KeyEnvVar)
	}
	if sealed {
		var err error
		if salt, err = secrets.NewSalt(); err != nil {
			return err
		}
		key = secrets.DeriveKey(passphrase, salt)
	}
	record, err := secrets.Seal(key, binding, token)
	if err != nil {
		return err
	}
	// Saving a sealed credential unlocks it with its new passphrase. Without a
	// server key there is nowhere to keep that unlock: the credential is saved
	// locked, and unlocking it says why.
	var wrapped []byte
	if sealed && d.serverKeyErr == nil {
		if wrapped, err = secrets.WrapKey(d.serverKey, secrets.Binding{UserID: userID, Tracker: tracker}, key); err != nil {
			return err
		}
	}

	sealedValue := 0
	if sealed {
		sealedValue = 1
	}
	now := time.Now()

	d.mu.Lock()
	defer d.mu.Unlock()
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`
		INSERT INTO user_tracker_credentials (user_id, tracker, site_url, email, record, sealed, salt, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, tracker) DO UPDATE SET
			site_url = excluded.site_url,
			email = excluded.email,
			record = excluded.record,
			sealed = excluded.sealed,
			salt = excluded.salt,
			account = '',
			updated_at = excluded.updated_at
	`, userID, tracker, strings.TrimSpace(siteURL), strings.TrimSpace(email), record, sealedValue, salt, now, now); err != nil {
		return err
	}
	// A new token or site is unconfirmed until the tracker is asked about it,
	// which is why the upsert empties the account: keeping the previous one
	// would name an account the new token may not belong to.
	//
	// Storing it again replaces the key it was sealed with, so any unlock kept
	// from a previous passphrase must go.
	if _, err := tx.Exec(`DELETE FROM user_credential_unlocks WHERE user_id = ? AND tracker = ?`, userID, tracker); err != nil {
		return err
	}
	if wrapped != nil {
		if _, err := tx.Exec(`INSERT INTO user_credential_unlocks (user_id, tracker, wrapped_key, unlocked_at) VALUES (?, ?, ?, ?)`,
			userID, tracker, wrapped, now.UTC()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SetUserTrackerCredentialAccount records the account the tracker confirmed a
// stored credential belongs to. It answers ErrNoUserCredential when there is
// no credential to attach it to.
func (d *DB) SetUserTrackerCredentialAccount(userID, tracker, account string) error {
	userID = strings.TrimSpace(userID)
	tracker = strings.ToLower(strings.TrimSpace(tracker))
	d.mu.Lock()
	defer d.mu.Unlock()
	result, err := d.conn.Exec(`UPDATE user_tracker_credentials SET account = ? WHERE user_id = ? AND tracker = ?`, strings.TrimSpace(account), userID, tracker)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return ErrNoUserCredential
	}
	return nil
}

// TrackerAccounts maps each tracker to the confirmed account of one person's
// credential for it, leaving out the credentials never confirmed. Nothing is
// decrypted, so a sealed and locked credential answers as well.
func (d *DB) TrackerAccounts(userID string) (map[string]string, error) {
	accounts := map[string]string{}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return accounts, nil
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	rows, err := d.conn.Query(`SELECT tracker, account FROM user_tracker_credentials WHERE user_id = ? AND TRIM(account) <> ''`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var tracker, account string
		if err := rows.Scan(&tracker, &account); err != nil {
			return nil, err
		}
		accounts[tracker] = account
	}
	return accounts, rows.Err()
}

// ClearUserTrackerCredential deletes one person's token.
func (d *DB) ClearUserTrackerCredential(userID, tracker string) error {
	userID = strings.TrimSpace(userID)
	tracker = strings.ToLower(strings.TrimSpace(tracker))
	if userID == "" || tracker == "" {
		// A request naming no tracker deleted nothing and answered that the
		// credential was forgotten.
		return ErrNoUserCredential
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.Exec(`DELETE FROM user_tracker_credentials WHERE user_id = ? AND tracker = ?`, userID, tracker)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return ErrNoUserCredential
	}
	if _, err := tx.Exec(`DELETE FROM user_credential_unlocks WHERE user_id = ? AND tracker = ?`, userID, tracker); err != nil {
		return err
	}
	return tx.Commit()
}

// UnlockUserTrackerCredential accepts the sealing passphrase and keeps the key
// it derives, under the server key, until its owner has been gone for the idle
// window (credentialunlocks.go). A wrong passphrase is refused without saying
// whether the credential exists, which is also what a wrong owner gets.
func (d *DB) UnlockUserTrackerCredential(userID, tracker, passphrase string) error {
	userID = strings.TrimSpace(userID)
	tracker = strings.ToLower(strings.TrimSpace(tracker))

	d.mu.RLock()
	var record, salt []byte
	var sealed int
	err := d.conn.QueryRow(`SELECT record, sealed, salt FROM user_tracker_credentials WHERE user_id = ? AND tracker = ?`, userID, tracker).Scan(&record, &sealed, &salt)
	d.mu.RUnlock()
	if err == sql.ErrNoRows {
		return ErrNoUserCredential
	}
	if err != nil {
		return err
	}
	if sealed == 0 {
		// Nothing to unlock, and saying "unlocked" to any passphrase at all
		// would teach the person that their credential is not sealed only by
		// them never being asked again.
		return ErrNotSealed
	}
	key := secrets.DeriveKey(passphrase, salt)
	if _, err := secrets.Open(key, secrets.Binding{UserID: userID, Tracker: tracker}, record); err != nil {
		return secrets.ErrWrongKey
	}
	if d.serverKeyErr != nil {
		// Saying "unlocked" here would be true until the next restart or the
		// next instance, which is the failure this storage exists to end.
		return fmt.Errorf("%w (%w) : définissez %s sur le serveur pour desceller un jeton", ErrServerKeyUnavailable, d.serverKeyErr, secrets.KeyEnvVar)
	}
	wrapped, err := secrets.WrapKey(d.serverKey, secrets.Binding{UserID: userID, Tracker: tracker}, key)
	if err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// A delete or a new record that landed since the read above leaves this key
	// opening nothing: kept, it would show the credential unlocked while every
	// use of it fails. The row lock holds the record still until the commit.
	var current []byte
	err = tx.QueryRow(`SELECT record FROM user_tracker_credentials WHERE user_id = ? AND tracker = ?`+d.forUpdate(), userID, tracker).Scan(&current)
	if err == sql.ErrNoRows {
		return ErrNoUserCredential
	}
	if err != nil {
		return err
	}
	if !bytes.Equal(current, record) {
		return secrets.ErrWrongKey
	}
	if _, err := tx.Exec(`INSERT INTO user_credential_unlocks (user_id, tracker, wrapped_key, unlocked_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (user_id, tracker) DO UPDATE SET wrapped_key = excluded.wrapped_key, unlocked_at = excluded.unlocked_at`,
		userID, tracker, wrapped, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit()
}

// LockUserTrackerCredential forgets the unlock, on every instance, so the
// credential needs its passphrase again.
func (d *DB) LockUserTrackerCredential(userID, tracker string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.conn.Exec(`DELETE FROM user_credential_unlocks WHERE user_id = ? AND tracker = ?`,
		strings.TrimSpace(userID), strings.ToLower(strings.TrimSpace(tracker)))
	return err
}

// UserTrackerCredentials lists what one person stored, tokens excluded.
func (d *DB) UserTrackerCredentials(userID string) ([]UserCredential, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return []UserCredential{}, nil
	}
	d.mu.RLock()
	rows, err := d.conn.Query(`SELECT c.tracker, c.site_url, c.email, c.account, c.sealed, c.updated_at, u.wrapped_key
		FROM user_tracker_credentials c
		LEFT JOIN user_credential_unlocks u ON u.user_id = c.user_id AND u.tracker = c.tracker
		WHERE c.user_id = ? ORDER BY c.tracker`, userID)
	d.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []UserCredential{}
	for rows.Next() {
		var credential UserCredential
		var sealed int
		var wrapped []byte
		if err := rows.Scan(&credential.Tracker, &credential.SiteURL, &credential.Email, &credential.Account, &sealed, &credential.UpdatedAt, &wrapped); err != nil {
			// Skipping it silently showed a profile with no credential while
			// the tracker kept using one.
			return nil, err
		}
		credential.Sealed = sealed == 1
		if credential.Sealed {
			_, ok := d.unwrapUnlock(userID, credential.Tracker, wrapped)
			credential.Unlocked = ok
		} else {
			credential.Unlocked = true
		}
		out = append(out, credential)
	}
	return out, nil
}

// userTrackerCredential opens one person's token for one tracker. It answers
// ErrNoUserCredential when there is none, and ErrCredentialLocked when the
// owner sealed it and holds no unlock of it.
//
// It takes no lock of its own, on purpose and like trackerCredentials: the
// tracker client resolves a credential from paths that already hold d.mu, and
// sync.RWMutex is not re-entrant. Taking the read lock here deadlocked the
// whole store on the first task a signed-in person created on a Jira project,
// and never gave the write lock back.
func (d *DB) userTrackerCredential(userID, tracker string) (siteURL string, email string, token string, err error) {
	userID = strings.TrimSpace(userID)
	tracker = strings.ToLower(strings.TrimSpace(tracker))
	if userID == "" || tracker == "" {
		return "", "", "", ErrNoUserCredential
	}

	var record, wrapped []byte
	var sealed int
	scanErr := d.conn.QueryRow(`SELECT c.site_url, c.email, c.record, c.sealed, u.wrapped_key
		FROM user_tracker_credentials c
		LEFT JOIN user_credential_unlocks u ON u.user_id = c.user_id AND u.tracker = c.tracker
		WHERE c.user_id = ? AND c.tracker = ?`, userID, tracker).Scan(&siteURL, &email, &record, &sealed, &wrapped)
	if scanErr == sql.ErrNoRows {
		return "", "", "", ErrNoUserCredential
	}
	if scanErr != nil {
		return "", "", "", scanErr
	}

	key := d.serverKey
	if sealed == 0 && d.serverKeyErr != nil {
		return "", "", "", fmt.Errorf("la clé de chiffrement du serveur est indisponible : %w", d.serverKeyErr)
	}
	if sealed == 1 {
		held, ok := d.unwrapUnlock(userID, tracker, wrapped)
		if !ok {
			if wrapped != nil && d.serverKeyErr == nil {
				// The server key was replaced since the unlock: it will
				// never open again, so it is not kept. Best effort and
				// without a lock, like the rest of this function.
				_, _ = d.conn.Exec(`DELETE FROM user_credential_unlocks WHERE user_id = ? AND tracker = ?`, userID, tracker)
			}
			return "", "", "", ErrCredentialLocked
		}
		key = held
	}
	token, err = secrets.Open(key, secrets.Binding{UserID: userID, Tracker: tracker}, record)
	if err != nil {
		return "", "", "", err
	}
	return siteURL, email, token, nil
}

// unwrapUnlock opens a stored unlock. No unlock, no server key, or one the
// server key cannot open all answer false: the credential is locked.
func (d *DB) unwrapUnlock(userID, tracker string, wrapped []byte) (secrets.Key, bool) {
	if wrapped == nil || d.serverKeyErr != nil {
		return secrets.Key{}, false
	}
	key, err := secrets.UnwrapKey(d.serverKey, secrets.Binding{UserID: userID, Tracker: tracker}, wrapped)
	return key, err == nil
}

// userTrackerCredentialToken opens just the token of a stored credential, for
// the caller that is about to store it again unchanged. It takes the read lock:
// unlike the resolver it is called from the handler path, where nothing holds
// the store's lock yet.
func (d *DB) userTrackerCredentialToken(userID, tracker string) (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, _, token, err := d.userTrackerCredential(userID, tracker)
	return token, err
}

// UserTrackerCredentialsFor resolves the connection parameters of one acting
// user, leaving every field they did not personalise empty so the caller keeps
// what it resolved for the server. A locked credential is an error rather than
// a silent fallback: writing under the service account while the person
// believes they act as themselves would misattribute the work.
func (d *DB) UserTrackerCredentialsFor(userID, tracker string) (siteURL string, email string, token string, err error) {
	siteURL, email, token, err = d.userTrackerCredential(userID, tracker)
	switch {
	case errors.Is(err, ErrNoUserCredential):
		return "", "", "", nil
	case err != nil:
		return "", "", "", err
	}
	return siteURL, email, token, nil
}
