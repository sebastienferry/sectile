package db

import (
	"errors"
	"log"
	"strings"
	"time"
)

// An orphaned tracker credential is a token stored under a user id that no
// account resolves. They exist because `default` used to be a real acting
// identity: before sign-in became mandatory (ADR 0015) an anonymous visitor
// resolved to it, and anything they stored — a Jira token included — was stored
// in its name. A deployment that then grew real accounts, without `default`
// ever being written to `users`, keeps that row with nobody behind it.
//
// The row is not dead weight, which is what makes it worth naming rather than
// quietly sweeping away. `ImplicitUser` is still an acting identity on the agent
// paths: CallOperation defaults an operation carrying no user id to it, and
// resolveAgentCredential still ends on it for a deployment pinning
// SECTILE_SERVER_TOKEN. ADR 0019 narrowed that reach by removing the legacy open
// mode, but it did not close it, so the credential is still resolvable by an
// identity nobody can sign in as and therefore nobody can revoke from the
// interface. At the same time every signed-in person misses it:
// UserTrackerCredentialsFor finds nothing under their own id, the resolution
// falls back to the empty server credential, and the tracker call fails on a
// missing account e-mail.
//
// What Sectile does about it, and what it deliberately does not:
//
//   - It reports them, at startup and in the interface. That is the part that
//     turns a silent misconfiguration into something a person can act on.
//   - It never rebinds one to a real account on its own. Opening the record
//     under the old binding and re-sealing it under a new one is technically
//     available to the server for an unsealed credential, and is exactly the
//     recovery path ADR 0014 refused: one the server can walk is a second way
//     in. Nothing in the database says whose token it is, either — the stored
//     e-mail names a tracker account, not a Sectile one.
//   - It never deletes one on its own. The agent paths may be using it, and an
//     upgrade that silently breaks a working setup is worse than one that says
//     what it found.
//
// The way out is the one ADR 0014 already accepts for a lost passphrase:
// re-enter the token under your own account, which Jira lets anyone recreate.
// Discarding the leftover row is then an explicit act, an admin's, once the
// deployment knows it no longer needs it.

// OrphanedCredential describes a credential whose owner no account resolves.
// It carries no token, like every other credential projection here.
type OrphanedCredential struct {
	UserID  string `json:"userId"`
	Tracker string `json:"tracker"`
	SiteURL string `json:"siteUrl,omitempty"`
	Email   string `json:"email,omitempty"`
	// Sealed says the server could not open it even if it wanted to, which is
	// what closes the rebinding question for this row rather than merely
	// declining it.
	Sealed    bool      `json:"sealed"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ErrCredentialNotOrphaned refuses a discard aimed at a credential that has a
// live owner. The endpoint is a cleanup for leftovers, and without this guard
// it would be a way for one admin to delete anyone's personal token by naming
// their id.
var ErrCredentialNotOrphaned = errors.New("this credential belongs to an existing account")

// orphanedCredentialsQuery reads the credentials whose user id is in no users
// row. It is a NOT EXISTS rather than a LEFT JOIN so it reads the same on both
// dialects, and it names no token column: nothing here needs the record.
const orphanedCredentialsQuery = `
	SELECT c.user_id, c.tracker, c.site_url, c.email, c.sealed, c.updated_at
	FROM user_tracker_credentials c
	WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = c.user_id)
	ORDER BY c.user_id, c.tracker`

// OrphanedTrackerCredentials lists them for a caller holding no lock, which is
// every handler.
func (d *DB) OrphanedTrackerCredentials() ([]OrphanedCredential, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.orphanedTrackerCredentials()
}

// orphanedTrackerCredentials takes no lock, for the startup report and for the
// callers that already hold one.
func (d *DB) orphanedTrackerCredentials() ([]OrphanedCredential, error) {
	rows, err := d.conn.Query(orphanedCredentialsQuery)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []OrphanedCredential{}
	for rows.Next() {
		var credential OrphanedCredential
		var sealed int
		if err := rows.Scan(&credential.UserID, &credential.Tracker, &credential.SiteURL, &credential.Email, &sealed, &credential.UpdatedAt); err != nil {
			return nil, err
		}
		credential.Sealed = sealed == 1
		out = append(out, credential)
	}
	return out, rows.Err()
}

// reportOrphanedTrackerCredentials says at startup what was found, once, with
// enough to act on and no more: the identity, the tracker and the account the
// token was entered for. The token itself is never touched, so there is nothing
// to leak into a log file.
//
// It only reports. Nothing here writes, which is the decision this file exists
// to record.
func (d *DB) reportOrphanedTrackerCredentials() {
	orphans, err := d.orphanedTrackerCredentials()
	if err != nil {
		log.Printf("[orphanedCredentials] lecture impossible : %v", err)
		return
	}
	if len(orphans) == 0 {
		return
	}
	log.Printf("⚠️  %d accès tracker enregistré(s) sous une identité qu'aucun compte ne résout : personne ne peut s'en servir depuis l'interface, et personne ne peut le révoquer.", len(orphans))
	for _, orphan := range orphans {
		account := orphan.Email
		if account == "" {
			account = "compte non renseigné"
		}
		log.Printf("[orphanedCredentials] user_id=%q tracker=%q (%s, %s) : ressaisissez votre jeton depuis Profil > Trackers, puis faites supprimer cette ligne par un admin.",
			orphan.UserID, orphan.Tracker, account, orphan.SiteURL)
	}
}

// DiscardOrphanedTrackerCredential removes one leftover row, and only a
// leftover: a credential whose user id names a live account is refused, so the
// cleanup cannot be turned into a way to delete somebody's token.
//
// The check and the delete are one statement, for the same reason ensureFirstAdmin
// is: an account created between the two reads would otherwise have its
// credential deleted by a call that had already decided it was an orphan.
func (d *DB) DiscardOrphanedTrackerCredential(userID, tracker string) error {
	userID = strings.TrimSpace(userID)
	tracker = strings.ToLower(strings.TrimSpace(tracker))
	if userID == "" || tracker == "" {
		return ErrNoUserCredential
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	result, err := d.conn.Exec(`
		DELETE FROM user_tracker_credentials
		WHERE user_id = ? AND tracker = ?
		  AND NOT EXISTS (SELECT 1 FROM users u WHERE u.id = user_tracker_credentials.user_id)`, userID, tracker)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		// Nothing was deleted, for one of two reasons the caller must be able
		// to tell apart: there is no such row, or there is one and it has an
		// owner.
		var present int
		if err := d.conn.QueryRow(
			`SELECT COUNT(*) FROM user_tracker_credentials WHERE user_id = ? AND tracker = ?`, userID, tracker,
		).Scan(&present); err != nil {
			return err
		}
		if present == 0 {
			return ErrNoUserCredential
		}
		return ErrCredentialNotOrphaned
	}
	// An unlock kept for it would otherwise outlive the row it opens.
	_, err = d.conn.Exec(`DELETE FROM user_credential_unlocks WHERE user_id = ? AND tracker = ?`, userID, tracker)
	return err
}
