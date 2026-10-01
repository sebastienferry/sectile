package db

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"tasks/internal/atlassian"
	"tasks/internal/secrets"
)

// The OAuth app is the integration registered on developer.atlassian.com that
// people's Jira grants are issued to (#654, ADR 0044). It is configured like a
// server credential (ADR 0028): saved from the Administration page, sealed
// under the server key, or else taken from the environment. A saved app wins
// as a whole, so a client id is never paired with another app's secret.

// Environment variables of the Jira OAuth app.
const (
	JiraOAuthClientIDVar     = "SECTILE_JIRA_OAUTH_CLIENT_ID"
	JiraOAuthClientSecretVar = "SECTILE_JIRA_OAUTH_CLIENT_SECRET"
	JiraOAuthRedirectURLVar  = "SECTILE_JIRA_OAUTH_REDIRECT_URL"
)

// ErrOAuthAppUnreadable is a saved app whose secret the server key does not
// open. It is never answered with the environment's app instead.
var ErrOAuthAppUnreadable = errors.New("the saved Jira OAuth app secret cannot be decrypted with the server key")

// OAuthAppState is what the Administration page shows of the Jira OAuth app.
// It never carries the secret.
type OAuthAppState struct {
	Configured  bool                   `json:"configured"`
	ClientID    string                 `json:"clientId"`
	SecretSet   bool                   `json:"secretSet"`
	RedirectURL string                 `json:"redirectUrl"`
	Source      ServerCredentialSource `json:"source"`
	Unreadable  bool                   `json:"unreadable,omitempty"`
	UpdatedAt   *time.Time             `json:"updatedAt,omitempty"`
}

type storedOAuthApp struct {
	clientID, redirectURL string
	record                []byte
	updatedAt             time.Time
}

// readOAuthApp answers the saved row, nil when there is none. It takes no
// lock: it is read from the credential resolver, called under d.mu.
func (d *DB) readOAuthApp(tracker string) (*storedOAuthApp, error) {
	var row storedOAuthApp
	err := d.conn.QueryRow(`SELECT client_id, redirect_url, record, updated_at FROM tracker_oauth_apps WHERE tracker = ?`, tracker).
		Scan(&row.clientID, &row.redirectURL, &row.record, &row.updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func environmentJiraOAuthApp() atlassian.App {
	return atlassian.App{
		ClientID:     strings.TrimSpace(os.Getenv(JiraOAuthClientIDVar)),
		ClientSecret: strings.TrimSpace(os.Getenv(JiraOAuthClientSecretVar)),
		RedirectURL:  strings.TrimSpace(os.Getenv(JiraOAuthRedirectURLVar)),
	}
}

// JiraOAuthApp answers the app in force and where it comes from. It is read
// at each use, never cached, so a change on the Administration page takes
// effect at once on every instance. An unreadable saved app is an error, not a
// reason to fall back to the environment.
func (d *DB) JiraOAuthApp() (atlassian.App, ServerCredentialSource, error) {
	row, err := d.readOAuthApp("jira")
	if err != nil {
		return atlassian.App{}, ServerCredentialNone, err
	}
	if row != nil {
		if d.serverKeyErr != nil {
			return atlassian.App{}, ServerCredentialStored, ErrOAuthAppUnreadable
		}
		secret, err := secrets.Open(d.serverKey, secrets.OAuthAppBinding("jira"), row.record)
		if err != nil {
			return atlassian.App{}, ServerCredentialStored, ErrOAuthAppUnreadable
		}
		return atlassian.App{ClientID: row.clientID, ClientSecret: secret, RedirectURL: row.redirectURL}, ServerCredentialStored, nil
	}
	app := environmentJiraOAuthApp()
	if app.ClientID == "" && app.ClientSecret == "" && app.RedirectURL == "" {
		return atlassian.App{}, ServerCredentialNone, nil
	}
	return app, ServerCredentialEnvironment, nil
}

// JiraOAuthConfigured says whether people can connect Jira. An unreadable app
// is not configured.
func (d *DB) JiraOAuthConfigured() bool {
	app, _, err := d.JiraOAuthApp()
	return err == nil && app.Configured()
}

// JiraOAuthAppState answers what the Administration page shows.
func (d *DB) JiraOAuthAppState() (OAuthAppState, error) {
	app, source, err := d.JiraOAuthApp()
	state := OAuthAppState{Source: source}
	switch {
	case errors.Is(err, ErrOAuthAppUnreadable):
		row, readErr := d.readOAuthApp("jira")
		if readErr != nil || row == nil {
			return state, readErr
		}
		state.ClientID, state.RedirectURL, state.SecretSet, state.Unreadable = row.clientID, row.redirectURL, true, true
		updated := row.updatedAt
		state.UpdatedAt = &updated
		return state, nil
	case err != nil:
		return state, err
	}
	state.Configured = app.Configured()
	state.ClientID = app.ClientID
	state.SecretSet = app.ClientSecret != ""
	state.RedirectURL = app.RedirectURL
	if source == ServerCredentialStored {
		if row, err := d.readOAuthApp("jira"); err == nil && row != nil {
			updated := row.updatedAt
			state.UpdatedAt = &updated
		}
	}
	return state, nil
}

// ValidateOAuthRedirectURL accepts the callback Atlassian will redirect to: an
// absolute HTTPS address, or HTTP on the machine itself for a local
// deployment.
func ValidateOAuthRedirectURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("l'URL de rappel doit être une adresse absolue, par exemple https://sectile.example.com/auth/jira/callback")
	}
	switch parsed.Scheme {
	case "https":
		return nil
	case "http":
		if host := parsed.Hostname(); host == "localhost" || host == "127.0.0.1" || host == "::1" {
			return nil
		}
	}
	return fmt.Errorf("l'URL de rappel doit être en HTTPS, sauf sur localhost")
}

// SaveJiraOAuthApp saves the app from the Administration page. An empty secret
// keeps the saved one; the first save needs it.
func (d *DB) SaveJiraOAuthApp(clientID, secret, redirectURL, adminID string) error {
	clientID, secret, redirectURL = strings.TrimSpace(clientID), strings.TrimSpace(secret), strings.TrimSpace(redirectURL)
	if clientID == "" {
		return fmt.Errorf("l'identifiant client est requis")
	}
	if err := ValidateOAuthRedirectURL(redirectURL); err != nil {
		return err
	}
	if d.serverKeyErr != nil {
		return fmt.Errorf("la clé de chiffrement du serveur est indisponible (%w) : définissez %s sur le serveur", d.serverKeyErr, secrets.KeyEnvVar)
	}
	var record []byte
	if secret == "" {
		row, err := d.readOAuthApp("jira")
		if err != nil {
			return err
		}
		if row == nil {
			return ErrOAuthAppSecretRequired
		}
		if _, err := secrets.Open(d.serverKey, secrets.OAuthAppBinding("jira"), row.record); err != nil {
			// Keeping a secret nothing can open would save an app that never
			// works.
			return fmt.Errorf("le secret enregistré ne peut plus être déchiffré : saisissez-le à nouveau")
		}
		record = row.record
	} else {
		var err error
		if record, err = secrets.Seal(d.serverKey, secrets.OAuthAppBinding("jira"), secret); err != nil {
			return err
		}
	}
	_, err := d.conn.Exec(`
		INSERT INTO tracker_oauth_apps (tracker, client_id, record, redirect_url, updated_at, updated_by)
		VALUES ('jira', ?, ?, ?, ?, ?)
		ON CONFLICT(tracker) DO UPDATE SET
			client_id = excluded.client_id,
			record = excluded.record,
			redirect_url = excluded.redirect_url,
			updated_at = excluded.updated_at,
			updated_by = excluded.updated_by`,
		clientID, record, redirectURL, time.Now().UTC(), strings.TrimSpace(adminID))
	return err
}

// ErrOAuthAppSecretRequired refuses a first save without a secret.
var ErrOAuthAppSecretRequired = errors.New("le secret client est requis")

// ClearJiraOAuthApp forgets the saved app; the environment applies again if it
// configures one. Grants already stored stay, and fail to refresh until an app
// is configured again.
func (d *DB) ClearJiraOAuthApp() error {
	_, err := d.conn.Exec(`DELETE FROM tracker_oauth_apps WHERE tracker = 'jira'`)
	return err
}
