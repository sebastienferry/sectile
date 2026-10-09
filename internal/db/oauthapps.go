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
	"tasks/internal/forgeoauth"
	"tasks/internal/secrets"
)

// The OAuth app is the integration registered on developer.atlassian.com that
// people's Jira grants are issued to (#654, ADR 0044), and since #804 the OAuth
// App on GitHub and the application on gitlab.com their forge grants are
// issued to, one row per tracker. It is configured like a server credential
// (ADR 0028): saved from the Administration page, sealed under the server key,
// or else taken from the environment. A saved app wins as a whole, so a client
// id is never paired with another app's secret.

// Environment variables of the Jira OAuth app.
const (
	JiraOAuthClientIDVar     = "SECTILE_JIRA_OAUTH_CLIENT_ID"
	JiraOAuthClientSecretVar = "SECTILE_JIRA_OAUTH_CLIENT_SECRET"
	JiraOAuthRedirectURLVar  = "SECTILE_JIRA_OAUTH_REDIRECT_URL"
)

// oauthAppEnvPrefix is where the environment configures each tracker's OAuth
// app: <prefix>CLIENT_ID, <prefix>CLIENT_SECRET and <prefix>REDIRECT_URL. A
// tracker missing from it has no OAuth app.
var oauthAppEnvPrefix = map[string]string{
	"jira":   "SECTILE_JIRA_OAUTH_",
	"github": "SECTILE_GITHUB_OAUTH_",
	"gitlab": "SECTILE_GITLAB_OAUTH_",
}

// ErrOAuthAppUnreadable is a saved app whose secret the server key does not
// open. It is never answered with the environment's app instead.
var ErrOAuthAppUnreadable = errors.New("the saved OAuth app secret cannot be decrypted with the server key")

// OAuthAppState is what the Administration page shows of a tracker's OAuth
// app. It never carries the secret.
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

// oauthAppTracker normalises a tracker name and refuses one with no OAuth app.
func oauthAppTracker(tracker string) (string, error) {
	tracker = strings.ToLower(strings.TrimSpace(tracker))
	if _, ok := oauthAppEnvPrefix[tracker]; !ok {
		return "", fmt.Errorf("aucune application OAuth pour le tracker %q", tracker)
	}
	return tracker, nil
}

// readOAuthApp answers the saved row, nil when there is none. It takes no
// lock.
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

func environmentOAuthApp(tracker string) forgeoauth.App {
	prefix := oauthAppEnvPrefix[tracker]
	return forgeoauth.App{
		ClientID:     strings.TrimSpace(os.Getenv(prefix + "CLIENT_ID")),
		ClientSecret: strings.TrimSpace(os.Getenv(prefix + "CLIENT_SECRET")),
		RedirectURL:  strings.TrimSpace(os.Getenv(prefix + "REDIRECT_URL")),
	}
}

// OAuthApp answers a tracker's app in force and where it comes from. It is
// read at each use, never cached, so a change on the Administration page takes
// effect at once on every instance. An unreadable saved app is an error, not a
// reason to fall back to the environment.
func (d *DB) OAuthApp(tracker string) (forgeoauth.App, ServerCredentialSource, error) {
	tracker, err := oauthAppTracker(tracker)
	if err != nil {
		return forgeoauth.App{}, ServerCredentialNone, err
	}
	row, err := d.readOAuthApp(tracker)
	if err != nil {
		return forgeoauth.App{}, ServerCredentialNone, err
	}
	if row != nil {
		if d.serverKeyErr != nil {
			return forgeoauth.App{}, ServerCredentialStored, ErrOAuthAppUnreadable
		}
		secret, err := secrets.Open(d.serverKey, secrets.OAuthAppBinding(tracker), row.record)
		if err != nil {
			return forgeoauth.App{}, ServerCredentialStored, ErrOAuthAppUnreadable
		}
		return forgeoauth.App{ClientID: row.clientID, ClientSecret: secret, RedirectURL: row.redirectURL}, ServerCredentialStored, nil
	}
	app := environmentOAuthApp(tracker)
	if app.ClientID == "" && app.ClientSecret == "" && app.RedirectURL == "" {
		return forgeoauth.App{}, ServerCredentialNone, nil
	}
	return app, ServerCredentialEnvironment, nil
}

// OAuthAppConfigured says whether people can connect a tracker. An unreadable
// app is not configured.
func (d *DB) OAuthAppConfigured(tracker string) bool {
	app, _, err := d.OAuthApp(tracker)
	return err == nil && app.Configured()
}

// OAuthAppState answers what the Administration page shows of a tracker's app.
func (d *DB) OAuthAppState(tracker string) (OAuthAppState, error) {
	app, source, err := d.OAuthApp(tracker)
	state := OAuthAppState{Source: source}
	switch {
	case errors.Is(err, ErrOAuthAppUnreadable):
		row, readErr := d.readOAuthApp(strings.ToLower(strings.TrimSpace(tracker)))
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
		if row, err := d.readOAuthApp(strings.ToLower(strings.TrimSpace(tracker))); err == nil && row != nil {
			updated := row.updatedAt
			state.UpdatedAt = &updated
		}
	}
	return state, nil
}

// JiraOAuthApp answers the Jira app in force and where it comes from; see
// OAuthApp.
func (d *DB) JiraOAuthApp() (atlassian.App, ServerCredentialSource, error) {
	app, source, err := d.OAuthApp("jira")
	return atlassian.App{ClientID: app.ClientID, ClientSecret: app.ClientSecret, RedirectURL: app.RedirectURL}, source, err
}

// JiraOAuthConfigured says whether people can connect Jira. An unreadable app
// is not configured.
func (d *DB) JiraOAuthConfigured() bool {
	return d.OAuthAppConfigured("jira")
}

// JiraOAuthAppState answers what the Administration page shows of the Jira
// app.
func (d *DB) JiraOAuthAppState() (OAuthAppState, error) {
	return d.OAuthAppState("jira")
}

// ValidateOAuthRedirectURL accepts the callback the provider will redirect to:
// an absolute HTTPS address, or HTTP on the machine itself for a local
// deployment.
func ValidateOAuthRedirectURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("l'URL de rappel doit être une adresse absolue, par exemple https://sectile.example.com/auth/<tracker>/callback")
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

// SaveJiraOAuthApp saves the Jira app from the Administration page; see
// SaveOAuthApp.
func (d *DB) SaveJiraOAuthApp(clientID, secret, redirectURL, adminID string) error {
	return d.SaveOAuthApp("jira", clientID, secret, redirectURL, adminID)
}

// SaveOAuthApp saves a tracker's app from the Administration page. An empty
// secret keeps the saved one, even under a corrected client id; the first save
// needs it. "Never paired with another app's secret" is about the saved app
// and the environment's: one is never completed with the other.
func (d *DB) SaveOAuthApp(tracker, clientID, secret, redirectURL, adminID string) error {
	tracker, err := oauthAppTracker(tracker)
	if err != nil {
		return err
	}
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
		row, err := d.readOAuthApp(tracker)
		if err != nil {
			return err
		}
		if row == nil {
			return ErrOAuthAppSecretRequired
		}
		if _, err := secrets.Open(d.serverKey, secrets.OAuthAppBinding(tracker), row.record); err != nil {
			// Keeping a secret nothing can open would save an app that never
			// works.
			return fmt.Errorf("le secret enregistré ne peut plus être déchiffré : saisissez-le à nouveau")
		}
		record = row.record
	} else if record, err = secrets.Seal(d.serverKey, secrets.OAuthAppBinding(tracker), secret); err != nil {
		return err
	}
	_, err = d.conn.Exec(`
		INSERT INTO tracker_oauth_apps (tracker, client_id, record, redirect_url, updated_at, updated_by)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(tracker) DO UPDATE SET
			client_id = excluded.client_id,
			record = excluded.record,
			redirect_url = excluded.redirect_url,
			updated_at = excluded.updated_at,
			updated_by = excluded.updated_by`,
		tracker, clientID, record, redirectURL, time.Now().UTC(), strings.TrimSpace(adminID))
	return err
}

// ErrOAuthAppSecretRequired refuses a first save without a secret.
var ErrOAuthAppSecretRequired = errors.New("le secret client est requis")

// ClearJiraOAuthApp forgets the saved Jira app; see ClearOAuthApp.
func (d *DB) ClearJiraOAuthApp() error {
	return d.ClearOAuthApp("jira")
}

// ClearOAuthApp forgets a tracker's saved app; the environment applies again
// if it configures one. Grants already stored stay, and fail to refresh until
// an app is configured again.
func (d *DB) ClearOAuthApp(tracker string) error {
	tracker, err := oauthAppTracker(tracker)
	if err != nil {
		return err
	}
	_, err = d.conn.Exec(`DELETE FROM tracker_oauth_apps WHERE tracker = ?`, tracker)
	return err
}
