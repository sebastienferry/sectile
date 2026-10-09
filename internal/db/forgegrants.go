package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"tasks/internal/forgeoauth"
	"tasks/internal/secrets"
	"tasks/internal/trackerapi"
)

// A forge grant (#804) is what a person's consent on GitHub's or GitLab's
// screen gave Sectile: their own credential for that provider, the way a Jira
// grant is for Jira (ADR 0044). It lives in the person's
// user_tracker_credentials row for the tracker, replacing any pasted token,
// sealed under the server key with a binding of its own kind, never behind a
// passphrase.
//
// The token answer comes in two shapes. A GitHub OAuth App token never
// expires and has no refresh token: it is used as stored until GitHub refuses
// it, which marks the grant disconnected. A GitLab token lasts two hours with
// a rotating refresh token: it is refreshed through the same compare-and-set
// as a Jira grant, so of two instances refreshing one grant exactly one spends
// the refresh token.
//
// Both grants serve the public instance only: github.com or gitlab.com. A
// GitHub Enterprise or self-hosted GitLab tracker refuses them, and the person
// uses a pasted token for it instead.

// ErrForgeOAuthNotConfigured refuses a connection, or the refresh of a grant,
// while the forge's OAuth app is not configured.
var ErrForgeOAuthNotConfigured = errors.New("l'application OAuth de cette forge n'est pas configurée")

// forgeGrantRecord is what the sealed record of a forge grant holds. A zero
// ExpiresAt is a token that never expires, with no refresh token.
type forgeGrantRecord struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt"`
	Scope        string    `json:"scope,omitempty"`
}

// forgeRevokeTimeout bounds the revocation of a grant at its provider, which
// is best effort.
const forgeRevokeTimeout = 10 * time.Second

func forgeGrantBinding(tracker, userID string) secrets.Binding {
	return secrets.Binding{UserID: userID, Tracker: tracker, Kind: CredentialKindOAuth}
}

func (d *DB) sealForgeGrant(tracker, userID string, grant forgeGrantRecord) ([]byte, error) {
	if d.serverKeyErr != nil {
		return nil, fmt.Errorf("la clé de chiffrement du serveur est indisponible (%w) : définissez %s sur le serveur", d.serverKeyErr, secrets.KeyEnvVar)
	}
	raw, err := json.Marshal(grant)
	if err != nil {
		return nil, err
	}
	return secrets.Seal(d.serverKey, forgeGrantBinding(tracker, userID), string(raw))
}

func (d *DB) openForgeGrant(tracker, userID string, record []byte) (forgeGrantRecord, error) {
	var grant forgeGrantRecord
	if d.serverKeyErr != nil {
		return grant, fmt.Errorf("la clé de chiffrement du serveur est indisponible : %w", d.serverKeyErr)
	}
	raw, err := secrets.Open(d.serverKey, forgeGrantBinding(tracker, userID), record)
	if err != nil {
		return grant, err
	}
	if err := json.Unmarshal([]byte(raw), &grant); err != nil {
		return grant, fmt.Errorf("the stored %s grant is unreadable", tracker)
	}
	return grant, nil
}

// SaveForgeGrant stores a person's grant as their credential for a forge,
// replacing any pasted token and its unlock: connecting never asks for a
// passphrase.
func (d *DB) SaveForgeGrant(tracker, userID string, tokens forgeoauth.Tokens, account string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("sign in before storing a personal credential")
	}
	provider, ok := forgeoauth.ForTracker(tracker)
	if !ok {
		return fmt.Errorf("no OAuth grant for the tracker %q", tracker)
	}
	tracker = provider.Name
	record, err := d.sealForgeGrant(tracker, userID, forgeGrantRecord{
		AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken,
		ExpiresAt: tokens.ExpiresAt, Scope: tokens.Scope,
	})
	if err != nil {
		return err
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
		INSERT INTO user_tracker_credentials (user_id, tracker, site_url, email, record, sealed, salt, account, kind, version, disconnected_at, created_at, updated_at)
		VALUES (?, ?, '', '', ?, 0, NULL, ?, 'oauth', 0, NULL, ?, ?)
		ON CONFLICT(user_id, tracker) DO UPDATE SET
			site_url = '',
			email = '',
			record = excluded.record,
			sealed = 0,
			salt = NULL,
			account = excluded.account,
			kind = 'oauth',
			version = user_tracker_credentials.version + 1,
			disconnected_at = NULL,
			refresh_claimed_at = NULL,
			updated_at = excluded.updated_at
	`, userID, tracker, record, strings.TrimSpace(account), now, now); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM user_credential_unlocks WHERE user_id = ? AND tracker = ?`, userID, tracker); err != nil {
		return err
	}
	return tx.Commit()
}

// ForgeSiteGranted says whether a forge grant may serve a tracker's site: the
// public instance only, an empty site being it.
func ForgeSiteGranted(tracker, site string) bool {
	switch tracker {
	case "github":
		return forgeSiteHost(site, trackerapi.DefaultGithubURL) == "api.github.com"
	case "gitlab":
		return forgeSiteHost(site, trackerapi.DefaultGitlabURL) == "gitlab.com"
	}
	return false
}

// forgeSiteHost is the host of a site, the provider's default when it is
// empty.
func forgeSiteHost(site, fallback string) string {
	site = strings.TrimSpace(site)
	if site == "" {
		site = fallback
	}
	parsed, err := url.Parse(site)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}

// forgeGrantAccess answers a live access token of one person's grant for one
// forge, refreshing it first when it is about to expire, with the row version
// it belongs to. A site the grant cannot serve, or a disconnected grant,
// answers the missing-credential error of ADR 0029. It takes no lock of the
// store, and none is held across a call to the provider.
func (d *DB) forgeGrantAccess(ctx context.Context, tracker, userID, site string) (accessToken string, version int64, err error) {
	if !ForgeSiteGranted(tracker, site) {
		return "", 0, &trackerapi.MissingPersonalCredentialError{Tracker: tracker, Reason: trackerapi.ReasonSiteNotGranted, Site: strings.TrimSpace(site)}
	}
	deadline := time.Now().Add(2 * jiraGrantRefreshWait)
	for time.Now().Before(deadline) {
		row, err := d.readGrantRow(tracker, userID)
		if err != nil {
			return "", 0, err
		}
		if row.kind != CredentialKindOAuth {
			return "", 0, fmt.Errorf("the %s credential of this user is not an OAuth grant", tracker)
		}
		if row.disconnected {
			return "", 0, &trackerapi.MissingPersonalCredentialError{Tracker: tracker, Reason: trackerapi.ReasonDisconnected}
		}
		grant, err := d.openForgeGrant(tracker, userID, row.record)
		if err != nil {
			return "", 0, err
		}
		if grant.ExpiresAt.IsZero() || time.Until(grant.ExpiresAt) > jiraGrantRefreshMargin {
			return grant.AccessToken, row.version, nil
		}

		// Another caller, here or on another instance, is refreshing: its
		// refresh token is spent, so wait for what it writes.
		if !row.claimedAt.IsZero() && time.Since(row.claimedAt) < jiraGrantRefreshWait {
			if err := d.awaitGrantRefresh(ctx, tracker, userID, row.version, row.claimedAt.Add(jiraGrantRefreshWait)); err != nil {
				return "", 0, err
			}
			continue
		}
		claimed, err := d.claimGrantRefresh(tracker, userID, row.version)
		if err != nil {
			return "", 0, err
		}
		if !claimed {
			continue
		}
		return d.refreshForgeGrant(ctx, tracker, userID, row.version+1, grant)
	}
	return "", 0, fmt.Errorf("the %s connection of this user is being refreshed by another instance that did not finish: try again", tracker)
}

// refreshForgeGrant runs a refresh this caller claimed, at version, and writes
// what it got. The token it answers belongs to version+1.
func (d *DB) refreshForgeGrant(ctx context.Context, tracker, userID string, version int64, grant forgeGrantRecord) (string, int64, error) {
	app, _, err := d.OAuthApp(tracker)
	if err == nil && !app.Configured() {
		err = ErrForgeOAuthNotConfigured
	}
	if err != nil {
		d.releaseGrantClaim(tracker, userID, version)
		return "", 0, err
	}
	// The caller's deadline never cuts a refresh in half: once the forge has
	// rotated the refresh token, the answer must be written, or the spent token
	// stays in the row and the next refresh disconnects a live grant.
	refreshCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), jiraGrantCallTimeout)
	tokens, err := d.forgeProvider(tracker).Refresh(refreshCtx, d.forgeHTTP, app, grant.RefreshToken)
	cancel()
	switch {
	case errors.Is(err, forgeoauth.ErrInvalidGrant):
		// Holding the claim, nobody else spent this refresh token: the grant
		// itself is dead. Only a new consent repairs it.
		if err := d.disconnectClaimedGrant(tracker, userID, version); err != nil {
			return "", 0, err
		}
		return "", 0, &trackerapi.MissingPersonalCredentialError{Tracker: tracker, Reason: trackerapi.ReasonDisconnected}
	case err != nil:
		// A passing failure changes nothing; the next call tries again.
		d.releaseGrantClaim(tracker, userID, version)
		return "", 0, fmt.Errorf("refreshing the %s connection: %w", tracker, err)
	}
	grant.AccessToken, grant.RefreshToken, grant.ExpiresAt = tokens.AccessToken, tokens.RefreshToken, tokens.ExpiresAt
	if tokens.Scope != "" {
		grant.Scope = tokens.Scope
	}
	record, err := d.sealForgeGrant(tracker, userID, grant)
	if err != nil {
		return "", 0, err
	}
	result, err := d.conn.Exec(`UPDATE user_tracker_credentials SET record = ?, version = version + 1, refresh_claimed_at = NULL, updated_at = ?
		WHERE user_id = ? AND tracker = ? AND version = ?`+jiraGrantClaimHeld, record, time.Now().UTC(), userID, tracker, version)
	if err != nil {
		return "", 0, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		// The person reconnected or deleted it meanwhile: what is there now
		// is what counts, and this refresh is theirs to drop.
		return "", 0, fmt.Errorf("the %s connection of this user changed while it was being refreshed: try again", tracker)
	}
	return grant.AccessToken, version + 1, nil
}

// MarkForgeGrantDisconnected records that the forge refused a grant's access
// token, which on GitHub means the person revoked the app. It is a
// compare-and-set on the version the token was resolved at, so a refusal
// that comes back after the person reconnected never disconnects the new
// grant. It takes no lock: it runs inside a tracker call, from paths that may
// already hold d.mu.
func (d *DB) MarkForgeGrantDisconnected(tracker, userID string, version int64) error {
	_, err := d.conn.Exec(`UPDATE user_tracker_credentials SET disconnected_at = ?, version = version + 1
		WHERE user_id = ? AND tracker = ? AND kind = 'oauth' AND version = ? AND disconnected_at IS NULL`,
		time.Now().UTC(), strings.TrimSpace(userID), strings.ToLower(strings.TrimSpace(tracker)), version)
	return err
}

// resolveForgeGrant is ResolvePersonalCredential for a GitHub or GitLab grant.
// A GitHub token never expires, so the forge refusing it is the only sign the
// person revoked the app: the client reports it through OnUnauthorized, which
// marks this version of the grant disconnected. A GitLab grant is
// disconnected by its refresh, as a Jira one is.
func (d *DB) resolveForgeGrant(ctx context.Context, tracker, userID, site string) (trackerapi.PersonalCredential, error) {
	access, version, err := d.forgeGrantAccess(ctx, tracker, userID, site)
	if err != nil {
		return trackerapi.PersonalCredential{}, err
	}
	credential := trackerapi.PersonalCredential{Token: access, OAuth: true}
	if tracker == "github" {
		credential.OnUnauthorized = func() {
			if err := d.MarkForgeGrantDisconnected(tracker, userID, version); err != nil {
				log.Printf("[ForgeOAuth] connexion %s non marquée comme révoquée : %v", tracker, err)
			}
		}
	}
	return credential, nil
}

// forgeGrantRevocation reads a person's forge grant before it is deleted or
// replaced, and answers what revokes it at the provider afterwards: best
// effort, under its own deadline, a failure only logged. A row holding no
// forge grant, or a forge whose app is not configured, revokes nothing. It
// takes no lock, and its answer must be called once the store's lock is
// released, since it calls the provider.
func (d *DB) forgeGrantRevocation(userID, tracker string) func() {
	nothing := func() {}
	if _, ok := forgeoauth.ForTracker(tracker); !ok {
		return nothing
	}
	row, err := d.readGrantRow(tracker, userID)
	if err != nil || row.kind != CredentialKindOAuth {
		return nothing
	}
	grant, err := d.openForgeGrant(tracker, userID, row.record)
	if err != nil {
		return nothing
	}
	app, _, err := d.OAuthApp(tracker)
	if err != nil || !app.Configured() {
		return nothing
	}
	return func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(context.Background()), forgeRevokeTimeout)
		defer cancel()
		if err := d.forgeProvider(tracker).Revoke(ctx, d.forgeHTTP, app, grant.AccessToken); err != nil {
			log.Printf("[ForgeOAuth] révocation %s échouée : %v", tracker, err)
		}
	}
}

// forgeProvider is the forge's endpoints, or the fake a test set.
func (d *DB) forgeProvider(tracker string) forgeoauth.Provider {
	if provider, ok := d.forge[tracker]; ok {
		return provider
	}
	provider, _ := forgeoauth.ForTracker(tracker)
	return provider
}

// SetForgeEndpoints points the store at another forge, for tests.
func (d *DB) SetForgeEndpoints(tracker string, provider forgeoauth.Provider, client *http.Client) {
	if d.forge == nil {
		d.forge = map[string]forgeoauth.Provider{}
	}
	d.forge[tracker], d.forgeHTTP = provider, client
}

// ConfiguredForgeTrackers lists the API addresses of the trackers of one forge
// the deployment works with, each once, as the tracker client resolves them.
// A grant only serves the ones on the public instance.
func (d *DB) ConfiguredForgeTrackers(tracker string) ([]string, error) {
	provider, ok := forgeoauth.ForTracker(tracker)
	if !ok {
		return nil, fmt.Errorf("no OAuth grant for the tracker %q", tracker)
	}
	settings, _ := d.getSettingsUnsafe()
	rows, err := d.conn.Query(`SELECT site FROM trackers WHERE provider = ? ORDER BY created_at, id`, provider.Name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	sites := []string{}
	for rows.Next() {
		var site string
		if err := rows.Scan(&site); err != nil {
			return nil, err
		}
		resolved := strings.TrimRight(strings.TrimSpace(resolvedTrackerSite(provider.Name, site, settings)), "/")
		if resolved != "" && !seen[strings.ToLower(resolved)] {
			seen[strings.ToLower(resolved)] = true
			sites = append(sites, resolved)
		}
	}
	return sites, rows.Err()
}

// ForgeOAuthAuthorizeURL starts a consent on a forge for one person in one web
// session and answers where to send their browser.
func (d *DB) ForgeOAuthAuthorizeURL(tracker, userID, sessionToken string) (string, error) {
	provider, ok := forgeoauth.ForTracker(tracker)
	if !ok {
		return "", fmt.Errorf("no OAuth grant for the tracker %q", tracker)
	}
	app, _, err := d.OAuthApp(provider.Name)
	if err != nil {
		return "", err
	}
	if !app.Configured() {
		return "", ErrForgeOAuthNotConfigured
	}
	state, err := d.StartOAuthFlow(provider.Name, userID, sessionToken)
	if err != nil {
		return "", err
	}
	return d.forgeProvider(provider.Name).AuthorizeURL(app, state), nil
}

// Outcomes of a forge consent, as the callback reports them to the profile.
// There is no "no_site": a forge grant serves its public instance whatever the
// trackers are.
const (
	ForgeOAuthConnected   = "connected"
	ForgeOAuthCancelled   = "cancelled"
	ForgeOAuthInvalid     = "invalid"
	ForgeOAuthUnreachable = "unreachable"
)

// CompleteForgeOAuth finishes a consent on a forge: the state is consumed for
// this tracker, session and person, the code exchanged, the account read
// through the grant, and the grant stored. Every outcome but "connected"
// stores nothing. The error, when there is one, says why for the log; it
// never carries a code, a token or the secret.
func (d *DB) CompleteForgeOAuth(ctx context.Context, tracker, userID, sessionToken, state, code string, denied bool) (string, error) {
	known, ok := forgeoauth.ForTracker(tracker)
	if !ok {
		return ForgeOAuthInvalid, fmt.Errorf("no OAuth grant for the tracker %q", tracker)
	}
	tracker = known.Name
	consumed := d.ConsumeOAuthFlow(tracker, state, userID, sessionToken)
	if denied {
		// Declining stores nothing whatever the state says; a valid one is
		// used up all the same.
		return ForgeOAuthCancelled, nil
	}
	if consumed != nil {
		return ForgeOAuthInvalid, consumed
	}
	if strings.TrimSpace(code) == "" {
		return ForgeOAuthInvalid, fmt.Errorf("callback without a code")
	}
	app, _, err := d.OAuthApp(tracker)
	if err != nil {
		return ForgeOAuthUnreachable, err
	}
	if !app.Configured() {
		return ForgeOAuthInvalid, ErrForgeOAuthNotConfigured
	}
	ctx, cancel := context.WithTimeout(ctx, jiraGrantCallTimeout)
	defer cancel()
	provider := d.forgeProvider(tracker)
	tokens, err := provider.Exchange(ctx, d.forgeHTTP, app, code)
	if errors.Is(err, forgeoauth.ErrInvalidGrant) {
		return ForgeOAuthInvalid, err
	}
	if err != nil {
		return ForgeOAuthUnreachable, err
	}
	var account string
	if tracker == "github" {
		account, err = d.trackers.CheckGithub(ctx, provider.APIBase, tokens.AccessToken)
	} else {
		account, err = d.trackers.CheckGitlab(ctx, provider.APIBase, tokens.AccessToken)
	}
	if err != nil {
		return ForgeOAuthUnreachable, err
	}
	if err := d.SaveForgeGrant(tracker, userID, tokens, account); err != nil {
		return ForgeOAuthUnreachable, err
	}
	return ForgeOAuthConnected, nil
}
