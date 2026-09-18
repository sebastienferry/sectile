package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
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
// use it.

// UserCredential is what the interface may know about a stored credential:
// everything except the token.
type UserCredential struct {
	Tracker string `json:"tracker"`
	// SiteURL belongs to the credential because an Atlassian account belongs to
	// a site: the person, their instance and their token travel together.
	SiteURL string `json:"siteUrl,omitempty"`
	Email   string `json:"email,omitempty"`
	// Sealed says the credential needs its owner's passphrase; Unlocked says
	// the passphrase was supplied in this server's lifetime.
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

// unlockedKeys holds the keys derived from a sealing passphrase, for as long as
// the server runs. They are deliberately nowhere else: written down, they would
// defeat the passphrase.
type unlockedKeys struct {
	mu   sync.RWMutex
	keys map[string]secrets.Key
}

func (u *unlockedKeys) get(key string) (secrets.Key, bool) {
	u.mu.RLock()
	defer u.mu.RUnlock()
	value, ok := u.keys[key]
	return value, ok
}

func (u *unlockedKeys) set(key string, value secrets.Key) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.keys == nil {
		u.keys = map[string]secrets.Key{}
	}
	u.keys[key] = value
}

func (u *unlockedKeys) clear(key string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	delete(u.keys, key)
}

func unlockKey(userID, tracker string) string {
	return strings.TrimSpace(userID) + "\x00" + strings.ToLower(strings.TrimSpace(tracker))
}

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
	_, _ = d.conn.Exec(`ALTER TABLE user_tracker_credentials ADD COLUMN site_url TEXT NOT NULL DEFAULT '';`)
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
	if token == "" {
		return fmt.Errorf("the token is required")
	}

	binding := secrets.Binding{UserID: userID, Tracker: tracker}
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

	sealedValue := 0
	if sealed {
		sealedValue = 1
	}
	now := time.Now()

	d.mu.Lock()
	defer d.mu.Unlock()
	if _, err := d.conn.Exec(`
		INSERT INTO user_tracker_credentials (user_id, tracker, site_url, email, record, sealed, salt, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, tracker) DO UPDATE SET
			site_url = excluded.site_url,
			email = excluded.email,
			record = excluded.record,
			sealed = excluded.sealed,
			salt = excluded.salt,
			updated_at = excluded.updated_at
	`, userID, tracker, strings.TrimSpace(siteURL), strings.TrimSpace(email), record, sealedValue, salt, now, now); err != nil {
		return err
	}
	// Storing it again replaces the key it was sealed with, so any key held
	// from a previous passphrase must go.
	d.unlocked.clear(unlockKey(userID, tracker))
	if sealed {
		d.unlocked.set(unlockKey(userID, tracker), key)
	}
	return nil
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
	result, err := d.conn.Exec(`DELETE FROM user_tracker_credentials WHERE user_id = ? AND tracker = ?`, userID, tracker)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return ErrNoUserCredential
	}
	d.unlocked.clear(unlockKey(userID, tracker))
	return nil
}

// UnlockUserTrackerCredential accepts the sealing passphrase and keeps the key
// it derives for as long as the server runs. A wrong passphrase is refused
// without saying whether the credential exists, which is also what a wrong
// owner gets.
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
	d.unlocked.set(unlockKey(userID, tracker), key)
	return nil
}

// LockUserTrackerCredential forgets the derived key, so the credential needs
// its passphrase again.
func (d *DB) LockUserTrackerCredential(userID, tracker string) {
	d.unlocked.clear(unlockKey(strings.TrimSpace(userID), tracker))
}

// UserTrackerCredentials lists what one person stored, tokens excluded.
func (d *DB) UserTrackerCredentials(userID string) ([]UserCredential, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return []UserCredential{}, nil
	}
	d.mu.RLock()
	rows, err := d.conn.Query(`SELECT tracker, site_url, email, sealed, updated_at FROM user_tracker_credentials WHERE user_id = ? ORDER BY tracker`, userID)
	d.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []UserCredential{}
	for rows.Next() {
		var credential UserCredential
		var sealed int
		if err := rows.Scan(&credential.Tracker, &credential.SiteURL, &credential.Email, &sealed, &credential.UpdatedAt); err != nil {
			// Skipping it silently showed a profile with no credential while
			// the tracker kept using one.
			return nil, err
		}
		credential.Sealed = sealed == 1
		if credential.Sealed {
			_, credential.Unlocked = d.unlocked.get(unlockKey(userID, credential.Tracker))
		} else {
			credential.Unlocked = true
		}
		out = append(out, credential)
	}
	return out, nil
}

// userTrackerCredential opens one person's token for one tracker. It answers
// ErrNoUserCredential when there is none, and ErrCredentialLocked when the
// owner sealed it and has not unlocked it in this server's lifetime.
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

	var record []byte
	var sealed int
	scanErr := d.conn.QueryRow(`SELECT site_url, email, record, sealed FROM user_tracker_credentials WHERE user_id = ? AND tracker = ?`, userID, tracker).Scan(&siteURL, &email, &record, &sealed)
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
		held, ok := d.unlocked.get(unlockKey(userID, tracker))
		if !ok {
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
