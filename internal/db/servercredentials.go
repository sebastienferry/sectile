package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"tasks/internal/secrets"
)

// The server credential of a provider is what Sectile itself authenticates
// with when nobody asked for the work: the synchronisation, timer-driven or
// started by hand (#464, ADR 0028). There is one per provider, an admin sets
// it, and it is sealed under the server key with no passphrase, since nobody
// is there to unlock it. When nothing is stored, the provider's own environment
// variable supplies it; nothing else does.

// ServerCredentialTrackers are the providers that have a server credential,
// in the order the Administration page lists them.
var ServerCredentialTrackers = []string{"github", "jira", "gitlab"}

// IsServerCredentialTracker reports whether a provider has a server credential.
func IsServerCredentialTracker(tracker string) bool {
	for _, name := range ServerCredentialTrackers {
		if name == tracker {
			return true
		}
	}
	return false
}

// ErrServerCredentialUnreadable is a stored server credential the server key
// does not open: the key changed, or was lost. It is never answered with the
// environment credential instead.
var ErrServerCredentialUnreadable = errors.New("the stored server credential cannot be decrypted with the server key")

// ServerCredentialSource says where a provider's server credential comes from.
type ServerCredentialSource string

const (
	ServerCredentialStored      ServerCredentialSource = "database"
	ServerCredentialEnvironment ServerCredentialSource = "environment"
	ServerCredentialNone        ServerCredentialSource = "none"
)

// ServerCredentialState is what the Administration page shows of one provider.
// It never carries the token.
type ServerCredentialState struct {
	Tracker    string                 `json:"tracker"`
	Source     ServerCredentialSource `json:"source"`
	Unreadable bool                   `json:"unreadable,omitempty"`
	Email      string                 `json:"email,omitempty"`
	Account    string                 `json:"account,omitempty"`
	CheckedAt  *time.Time             `json:"checkedAt,omitempty"`
	UpdatedAt  *time.Time             `json:"updatedAt,omitempty"`
}

type storedServerCredential struct {
	email, account       string
	record               []byte
	checkedAt, updatedAt sql.NullTime
}

// readServerCredential answers the stored row of one provider, nil when there
// is none. It takes no lock: it is called from the tracker client, itself
// called under d.mu, and a single SELECT needs none.
func (d *DB) readServerCredential(tracker string) (*storedServerCredential, error) {
	var row storedServerCredential
	err := d.conn.QueryRow(`SELECT email, account, record, checked_at, updated_at
		FROM server_tracker_credentials WHERE tracker = ?`, tracker).
		Scan(&row.email, &row.account, &row.record, &row.checkedAt, &row.updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// environmentServerCredential is the credential the server environment gives
// one provider, as the tracker client read it at startup.
func (d *DB) environmentServerCredential(tracker string) (email, token string) {
	if d.trackers == nil {
		return "", ""
	}
	switch tracker {
	case "github":
		return "", d.trackers.GithubToken
	case "gitlab":
		return "", d.trackers.GitlabToken
	case "jira":
		return d.trackers.JiraEmail, d.trackers.JiraToken
	}
	return "", ""
}

// storedServerTrackerCredential opens the stored credential of one provider.
// found is false when nothing is stored, in which case the environment answers
// for it; an error means something is stored and cannot be used.
func (d *DB) storedServerTrackerCredential(tracker string) (email, token string, found bool, err error) {
	row, err := d.readServerCredential(tracker)
	if err != nil || row == nil {
		return "", "", false, err
	}
	if d.serverKeyErr != nil {
		return row.email, "", true, ErrServerCredentialUnreadable
	}
	token, err = secrets.Open(d.serverKey, secrets.ServerBinding(tracker), row.record)
	if err != nil {
		return row.email, "", true, ErrServerCredentialUnreadable
	}
	return row.email, token, true, nil
}

// ServerTrackerCredentialStates lists every provider's state for the
// Administration page.
func (d *DB) ServerTrackerCredentialStates() ([]ServerCredentialState, error) {
	states := make([]ServerCredentialState, 0, len(ServerCredentialTrackers))
	for _, tracker := range ServerCredentialTrackers {
		state, err := d.ServerTrackerCredentialState(tracker)
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	return states, nil
}

// ServerTrackerCredentialState answers one provider's state.
func (d *DB) ServerTrackerCredentialState(tracker string) (ServerCredentialState, error) {
	state := ServerCredentialState{Tracker: tracker, Source: ServerCredentialNone}
	row, err := d.readServerCredential(tracker)
	if err != nil {
		return state, err
	}
	if row != nil {
		state.Source = ServerCredentialStored
		state.Email = row.email
		state.Account = row.account
		if row.checkedAt.Valid {
			checked := row.checkedAt.Time
			state.CheckedAt = &checked
		}
		if row.updatedAt.Valid {
			updated := row.updatedAt.Time
			state.UpdatedAt = &updated
		}
		if _, _, _, openErr := d.storedServerTrackerCredential(tracker); openErr != nil {
			state.Unreadable = true
		}
		return state, nil
	}
	email, token := d.environmentServerCredential(tracker)
	if token != "" && (tracker != "jira" || email != "") {
		state.Source = ServerCredentialEnvironment
		state.Email = email
	}
	return state, nil
}

// SaveServerTrackerCredential seals and stores one provider's server
// credential, replacing the previous one. The caller has checked it against the
// instance; account is who the check said it belongs to.
func (d *DB) SaveServerTrackerCredential(tracker, email, token, account, adminID string) error {
	if !IsServerCredentialTracker(tracker) {
		return fmt.Errorf("tracker %q inconnu", tracker)
	}
	token = strings.TrimSpace(token)
	email = strings.TrimSpace(email)
	if token == "" {
		return fmt.Errorf("le jeton est requis")
	}
	if tracker == "jira" && email == "" {
		return fmt.Errorf("l'e-mail du compte Jira est requis avec son jeton")
	}
	if tracker != "jira" {
		email = ""
	}
	if d.serverKeyErr != nil {
		// A zero key is a valid AES key: storing under it would look like
		// encryption and protect nothing.
		return fmt.Errorf("la clé de chiffrement du serveur est indisponible (%w) : définissez %s sur le serveur", d.serverKeyErr, secrets.KeyEnvVar)
	}
	record, err := secrets.Seal(d.serverKey, secrets.ServerBinding(tracker), token)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	var checkedAt any
	if strings.TrimSpace(account) != "" {
		checkedAt = now
	}
	_, err = d.conn.Exec(`
		INSERT INTO server_tracker_credentials (tracker, email, record, account, checked_at, updated_at, updated_by)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tracker) DO UPDATE SET
			email = excluded.email,
			record = excluded.record,
			account = excluded.account,
			checked_at = excluded.checked_at,
			updated_at = excluded.updated_at,
			updated_by = excluded.updated_by
	`, tracker, email, record, strings.TrimSpace(account), checkedAt, now, strings.TrimSpace(adminID))
	return err
}

// RecordServerCredentialCheck dates a successful check of the stored
// credential and keeps the account it named. Nothing stored, nothing recorded:
// an environment credential has no row to write on.
func (d *DB) RecordServerCredentialCheck(tracker, account string) error {
	_, err := d.conn.Exec(`UPDATE server_tracker_credentials SET account = ?, checked_at = ? WHERE tracker = ?`,
		strings.TrimSpace(account), time.Now().UTC(), tracker)
	return err
}

// ClearServerTrackerCredential deletes one provider's stored credential, which
// hands it back to the environment, if the environment has one.
func (d *DB) ClearServerTrackerCredential(tracker string) error {
	if !IsServerCredentialTracker(tracker) {
		return fmt.Errorf("tracker %q inconnu", tracker)
	}
	_, err := d.conn.Exec(`DELETE FROM server_tracker_credentials WHERE tracker = ?`, tracker)
	return err
}

// legacyServerTokenColumns are the settings columns that held the server
// credentials in clear text before #464, per provider.
var legacyServerTokenColumns = []struct{ tracker, token, email string }{
	{"github", "github_token", ""},
	{"gitlab", "gitlab_token", ""},
	{"jira", "jira_api_token", "jira_email"},
}

// adoptLegacyServerTrackerTokens seals the server tokens a previous version
// kept in clear text in the settings row, stores them as the providers' server
// credentials and blanks the columns.
//
// It is not a numbered migration because it needs the server key, which SQL
// cannot reach, and it does not need to be one: it is idempotent. An empty
// column is done; a column whose provider already has a row was adopted by a
// run interrupted before blanking, or by another instance starting at the same
// time, so it is only blanked.
//
// With something to adopt and no key, it refuses rather than discard the only
// working credential: the caller stops the server with a message naming the
// variable to set. Only PostgreSQL can get there, SQLite generates its key.
func (d *DB) adoptLegacyServerTrackerTokens() error {
	for _, legacy := range legacyServerTokenColumns {
		emailColumn := "''"
		if legacy.email != "" {
			emailColumn = legacy.email
		}
		var token, email string
		err := d.conn.QueryRow(`SELECT `+legacy.token+`, `+emailColumn+` FROM settings WHERE id = 1`).Scan(&token, &email)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading the %s server token: %w", legacy.tracker, err)
		}
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		existing, err := d.readServerCredential(legacy.tracker)
		if err != nil {
			return err
		}
		if existing == nil && d.serverKeyErr != nil {
			return fmt.Errorf("the settings hold a %s server token in clear text and the encryption key is unavailable (%w): set %s so it can be sealed, or it would be lost",
				legacy.tracker, d.serverKeyErr, secrets.KeyEnvVar)
		}
		err = d.conn.WithTx(func(tx *sqlTx) error {
			if existing == nil {
				record, err := secrets.Seal(d.serverKey, secrets.ServerBinding(legacy.tracker), token)
				if err != nil {
					return err
				}
				if _, err := tx.Exec(`
					INSERT INTO server_tracker_credentials (tracker, email, record, account, checked_at, updated_at, updated_by)
					VALUES (?, ?, ?, '', NULL, ?, 'upgrade')
					ON CONFLICT(tracker) DO NOTHING
				`, legacy.tracker, strings.TrimSpace(email), record, time.Now().UTC()); err != nil {
					return err
				}
			}
			blank := `UPDATE settings SET ` + legacy.token + ` = ''`
			if legacy.email != "" {
				blank += `, ` + legacy.email + ` = ''`
			}
			_, err := tx.Exec(blank + ` WHERE id = 1`)
			return err
		})
		if err != nil {
			return fmt.Errorf("sealing the %s server token: %w", legacy.tracker, err)
		}
		if existing == nil {
			log.Printf("[TrackerCredentials] jeton serveur %s chiffré et déplacé hors des réglages", legacy.tracker)
		}
	}
	return nil
}
