package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"tasks/internal/atlassian"
	"tasks/internal/secrets"
	"tasks/internal/trackerapi"
)

// A Jira grant (#654, ADR 0044) is the second kind of personal Jira
// credential: what a person's consent on Atlassian's screen gave Sectile. It
// lives in the person's user_tracker_credentials row, replacing any API token,
// sealed under the server key with a binding of its own kind, and never
// behind a passphrase: it is refreshed while its owner is away, which is the
// point of it.
//
// Atlassian rotates refresh tokens, and a used one stops working at once. Two
// instances refreshing one grant from the same refresh token would therefore
// leave one of them with invalid_grant and, if it believed it, disconnect a
// grant the other had just renewed. The refresh is claimed first, by a
// compare-and-set on the row's version: exactly one caller refreshes for a
// given state of the row, and the others wait for what it writes.

// Kinds of personal credential.
const (
	CredentialKindAPIToken = "api_token"
	CredentialKindOAuth    = "oauth"
)

// errOAuthGrant tells the API token paths that the row holds a grant, of Jira
// or, since #804, of GitHub or GitLab: there is no token to open, check or
// re-store.
var errOAuthGrant = errors.New("the credential of this user is an OAuth grant")

// ErrJiraOAuthNotConfigured refuses a connection, or the refresh of a grant,
// while no OAuth app is configured.
var ErrJiraOAuthNotConfigured = errors.New("l'application OAuth Jira n'est pas configurée")

// jiraGrantRecord is what the sealed record of a grant holds.
type jiraGrantRecord struct {
	RefreshToken string           `json:"refreshToken"`
	AccessToken  string           `json:"accessToken"`
	ExpiresAt    time.Time        `json:"expiresAt"`
	Scope        string           `json:"scope,omitempty"`
	Sites        []atlassian.Site `json:"sites"`
}

func jiraGrantBinding(userID string) secrets.Binding {
	return secrets.Binding{UserID: userID, Tracker: "jira", Kind: CredentialKindOAuth}
}

func (d *DB) sealJiraGrant(userID string, grant jiraGrantRecord) ([]byte, error) {
	if d.serverKeyErr != nil {
		return nil, fmt.Errorf("la clé de chiffrement du serveur est indisponible (%w) : définissez %s sur le serveur", d.serverKeyErr, secrets.KeyEnvVar)
	}
	raw, err := json.Marshal(grant)
	if err != nil {
		return nil, err
	}
	return secrets.Seal(d.serverKey, jiraGrantBinding(userID), string(raw))
}

func (d *DB) openJiraGrant(userID string, record []byte) (jiraGrantRecord, error) {
	var grant jiraGrantRecord
	if d.serverKeyErr != nil {
		return grant, fmt.Errorf("la clé de chiffrement du serveur est indisponible : %w", d.serverKeyErr)
	}
	raw, err := secrets.Open(d.serverKey, jiraGrantBinding(userID), record)
	if err != nil {
		return grant, err
	}
	if err := json.Unmarshal([]byte(raw), &grant); err != nil {
		return grant, fmt.Errorf("the stored Jira grant is unreadable")
	}
	return grant, nil
}

// SaveJiraGrant stores a person's grant as their Jira credential, replacing
// any API token and its unlock: connecting never asks for a passphrase.
func (d *DB) SaveJiraGrant(userID string, tokens atlassian.Tokens, sites []atlassian.Site, account string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("sign in before storing a personal credential")
	}
	record, err := d.sealJiraGrant(userID, jiraGrantRecord{
		RefreshToken: tokens.RefreshToken, AccessToken: tokens.AccessToken,
		ExpiresAt: tokens.ExpiresAt, Scope: tokens.Scope, Sites: sites,
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
		VALUES (?, 'jira', '', '', ?, 0, NULL, ?, 'oauth', 0, NULL, ?, ?)
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
	`, userID, record, strings.TrimSpace(account), now, now); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM user_credential_unlocks WHERE user_id = ? AND tracker = 'jira'`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

// grantRow is the part of a credential row the refresh reads, for a Jira
// grant or a forge one.
type grantRow struct {
	kind         string
	version      int64
	record       []byte
	disconnected bool
	// claimedAt is when a refresh in flight was claimed, zero when none is.
	claimedAt time.Time
}

// readJiraGrantRow reads the row of a person's Jira grant; see readGrantRow.
func (d *DB) readJiraGrantRow(userID string) (*grantRow, error) {
	return d.readGrantRow("jira", userID)
}

// readGrantRow takes no lock, and is never called under d.mu: a refresh may
// follow it, and none is held across a call to the provider.
func (d *DB) readGrantRow(tracker, userID string) (*grantRow, error) {
	var row grantRow
	var disconnected, claimed sql.NullTime
	err := d.conn.QueryRow(`SELECT kind, version, record, disconnected_at, refresh_claimed_at FROM user_tracker_credentials WHERE user_id = ? AND tracker = ?`, userID, tracker).
		Scan(&row.kind, &row.version, &row.record, &disconnected, &claimed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoUserCredential
	}
	if err != nil {
		return nil, err
	}
	row.disconnected = disconnected.Valid
	if claimed.Valid {
		row.claimedAt = claimed.Time
	}
	return &row, nil
}

// How long before its expiry an access token is renewed, how long a claimed
// refresh is waited for, and how often a waiter looks. The wait is several
// times a refresh's own timeout, so a claim older than it is one whose
// instance died before writing, even read on a replica whose clock is ahead,
// and may be taken over.
var (
	jiraGrantRefreshMargin = time.Minute
	jiraGrantRefreshWait   = time.Minute
	jiraGrantPollInterval  = 50 * time.Millisecond
	jiraGrantCallTimeout   = 15 * time.Second
)

// jiraGrantAccess answers the gateway address and a live access token of one
// person's grant for one site, refreshing it first when it is about to
// expire. A disconnected grant, or one that does not cover the site, answers
// the missing-credential error of ADR 0029. It takes no lock of the store, and
// none is held across a call to Atlassian.
func (d *DB) jiraGrantAccess(ctx context.Context, userID, site string) (apiBase, accessToken string, err error) {
	site = trackerapi.JiraSite(site)
	deadline := time.Now().Add(2 * jiraGrantRefreshWait)
	for time.Now().Before(deadline) {
		row, err := d.readJiraGrantRow(userID)
		if err != nil {
			return "", "", err
		}
		if row.kind != CredentialKindOAuth {
			return "", "", fmt.Errorf("the Jira credential of this user is not an OAuth grant")
		}
		if row.disconnected {
			return "", "", &trackerapi.MissingPersonalCredentialError{Tracker: "jira", Reason: trackerapi.ReasonDisconnected}
		}
		grant, err := d.openJiraGrant(userID, row.record)
		if err != nil {
			return "", "", err
		}
		cloudID, ok := grantedCloudID(grant.Sites, site)
		if !ok {
			return "", "", &trackerapi.MissingPersonalCredentialError{Tracker: "jira", Reason: trackerapi.ReasonSiteNotGranted, Site: site}
		}
		if time.Until(grant.ExpiresAt) > jiraGrantRefreshMargin {
			return d.atlassianEndpoints().JiraAPIBase(cloudID), grant.AccessToken, nil
		}

		// Another caller, here or on another instance, is refreshing: its
		// refresh token is spent, so wait for what it writes.
		if !row.claimedAt.IsZero() && time.Since(row.claimedAt) < jiraGrantRefreshWait {
			if err := d.awaitGrantRefresh(ctx, "jira", userID, row.version, row.claimedAt.Add(jiraGrantRefreshWait)); err != nil {
				return "", "", err
			}
			continue
		}
		claimed, err := d.claimGrantRefresh("jira", userID, row.version)
		if err != nil {
			return "", "", err
		}
		if !claimed {
			continue
		}
		return d.refreshJiraGrant(ctx, userID, row.version+1, grant, cloudID)
	}
	return "", "", fmt.Errorf("the Jira connection of this user is being refreshed by another instance that did not finish: try again")
}

// refreshJiraGrant runs a refresh this caller claimed, at version, and writes
// what it got.
func (d *DB) refreshJiraGrant(ctx context.Context, userID string, version int64, grant jiraGrantRecord, cloudID string) (string, string, error) {
	app, _, err := d.JiraOAuthApp()
	if err == nil && !app.Configured() {
		err = ErrJiraOAuthNotConfigured
	}
	if err != nil {
		d.releaseGrantClaim("jira", userID, version)
		return "", "", err
	}
	// The caller's deadline never cuts a refresh in half: once Atlassian has
	// rotated the refresh token, the answer must be written, or the spent token
	// stays in the row and the next refresh disconnects a live grant.
	refreshCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), jiraGrantCallTimeout)
	tokens, err := d.atlassianEndpoints().Refresh(refreshCtx, d.atlassianHTTP, app, grant.RefreshToken)
	cancel()
	switch {
	case errors.Is(err, atlassian.ErrInvalidGrant):
		// Holding the claim, nobody else spent this refresh token: the grant
		// itself is dead. Only a new consent repairs it.
		if err := d.disconnectClaimedGrant("jira", userID, version); err != nil {
			return "", "", err
		}
		return "", "", &trackerapi.MissingPersonalCredentialError{Tracker: "jira", Reason: trackerapi.ReasonDisconnected}
	case err != nil:
		// A passing failure changes nothing; the next call tries again.
		d.releaseGrantClaim("jira", userID, version)
		return "", "", fmt.Errorf("refreshing the Jira connection: %w", err)
	}
	grant.AccessToken, grant.RefreshToken, grant.ExpiresAt = tokens.AccessToken, tokens.RefreshToken, tokens.ExpiresAt
	if tokens.Scope != "" {
		grant.Scope = tokens.Scope
	}
	record, err := d.sealJiraGrant(userID, grant)
	if err != nil {
		return "", "", err
	}
	result, err := d.conn.Exec(`UPDATE user_tracker_credentials SET record = ?, version = version + 1, refresh_claimed_at = NULL, updated_at = ?
		WHERE user_id = ? AND tracker = 'jira' AND version = ?`+jiraGrantClaimHeld, record, time.Now().UTC(), userID, version)
	if err != nil {
		return "", "", err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		// The person reconnected or deleted it meanwhile: what is there now
		// is what counts, and this refresh is theirs to drop.
		return "", "", fmt.Errorf("the Jira connection of this user changed while it was being refreshed: try again")
	}
	return d.atlassianEndpoints().JiraAPIBase(cloudID), grant.AccessToken, nil
}

// jiraGrantClaimHeld narrows a claimant's write to the row it claimed. The
// version alone is not enough: a row deleted and created again restarts at 0,
// and may be back at the claimed version, unclaimed or holding a token.
const jiraGrantClaimHeld = ` AND kind = 'oauth' AND refresh_claimed_at IS NOT NULL`

// disconnectClaimedGrant marks a grant disconnected once its provider refused
// the refresh this caller claimed at version.
func (d *DB) disconnectClaimedGrant(tracker, userID string, version int64) error {
	_, err := d.conn.Exec(`UPDATE user_tracker_credentials SET disconnected_at = ?, refresh_claimed_at = NULL, version = version + 1
		WHERE user_id = ? AND tracker = ? AND version = ?`+jiraGrantClaimHeld, time.Now().UTC(), userID, tracker, version)
	return err
}

// claimJiraGrantRefresh claims the refresh of a Jira grant; see
// claimGrantRefresh.
func (d *DB) claimJiraGrantRefresh(userID string, version int64) (bool, error) {
	return d.claimGrantRefresh("jira", userID, version)
}

// claimGrantRefresh marks a refresh in flight, which only one caller can do
// from a given version.
func (d *DB) claimGrantRefresh(tracker, userID string, version int64) (bool, error) {
	now := time.Now().UTC()
	result, err := d.conn.Exec(`UPDATE user_tracker_credentials SET version = version + 1, refresh_claimed_at = ?
		WHERE user_id = ? AND tracker = ? AND kind = 'oauth' AND version = ?
		  AND (refresh_claimed_at IS NULL OR refresh_claimed_at < ?)`, now, userID, tracker, version, now.Add(-jiraGrantRefreshWait))
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return err == nil && affected == 1, err
}

// releaseGrantClaim gives up a claim whose refresh token was never spent, so
// the next caller tries at once rather than waiting it out.
func (d *DB) releaseGrantClaim(tracker, userID string, version int64) {
	_, _ = d.conn.Exec(`UPDATE user_tracker_credentials SET refresh_claimed_at = NULL, version = version + 1
		WHERE user_id = ? AND tracker = ? AND version = ?`+jiraGrantClaimHeld, userID, tracker, version)
}

// awaitGrantRefresh waits until the row moves on from version, which a
// claimant's write, a disconnection or a new row all do, or until until.
func (d *DB) awaitGrantRefresh(ctx context.Context, tracker, userID string, version int64, until time.Time) error {
	for time.Now().Before(until) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(jiraGrantPollInterval):
		}
		row, err := d.readGrantRow(tracker, userID)
		if err != nil {
			return err
		}
		if row.version != version {
			return nil
		}
	}
	return nil
}

// grantedCloudID finds the site of a grant matching a Jira site, the way the
// client spells sites. With no site to match, the first one serves: the call
// will say for itself that no site is configured.
func grantedCloudID(sites []atlassian.Site, site string) (string, bool) {
	for _, granted := range sites {
		if site == "" || trackerapi.JiraSite(granted.URL) == site {
			return granted.CloudID, true
		}
	}
	return "", false
}

// grantedSiteURLs lists the sites of a grant for the profile.
func grantedSiteURLs(sites []atlassian.Site) []string {
	out := make([]string, 0, len(sites))
	for _, site := range sites {
		out = append(out, trackerapi.JiraSite(site.URL))
	}
	return out
}

// ConfiguredJiraSites lists the Jira sites the deployment works with: its own
// and every site a Jira project sets for itself, each once. A grant covering
// none of them is of no use.
func (d *DB) ConfiguredJiraSites() ([]string, error) {
	seen := map[string]bool{}
	sites := []string{}
	add := func(raw string) {
		if site := trackerapi.JiraSite(raw); site != "" && !seen[site] {
			seen[site] = true
			sites = append(sites, site)
		}
	}
	if settings, err := d.getSettingsUnsafe(); err == nil && settings != nil {
		add(settings.JiraUrl)
	}
	if d.trackers != nil {
		add(d.trackers.JiraURL)
	}
	rows, err := d.conn.Query(`SELECT tracker_url FROM projects WHERE LOWER(issue_tracker) = 'jira' AND TRIM(tracker_url) <> '' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		add(raw)
	}
	return sites, rows.Err()
}

// atlassianEndpoints are Atlassian's, or the fake a test set.
func (d *DB) atlassianEndpoints() atlassian.Endpoints {
	if d.atlassian.Token == "" {
		return atlassian.DefaultEndpoints
	}
	return d.atlassian
}

// SetAtlassianEndpoints points the store at another Atlassian, for tests.
func (d *DB) SetAtlassianEndpoints(endpoints atlassian.Endpoints, client *http.Client) {
	d.atlassian, d.atlassianHTTP = endpoints, client
}

// JiraOAuthAuthorizeURL starts a consent for one person in one web session and
// answers where to send their browser.
func (d *DB) JiraOAuthAuthorizeURL(userID, sessionToken string) (string, error) {
	app, _, err := d.JiraOAuthApp()
	if err != nil {
		return "", err
	}
	if !app.Configured() {
		return "", ErrJiraOAuthNotConfigured
	}
	state, err := d.StartJiraOAuthFlow(userID, sessionToken)
	if err != nil {
		return "", err
	}
	return d.atlassianEndpoints().AuthorizeURL(app, state), nil
}

// Outcomes of a Jira consent, as the callback reports them to the profile.
const (
	JiraOAuthConnected   = "connected"
	JiraOAuthCancelled   = "cancelled"
	JiraOAuthInvalid     = "invalid"
	JiraOAuthNoSite      = "no_site"
	JiraOAuthUnreachable = "unreachable"
)

// CompleteJiraOAuth finishes a consent: the state is consumed for this
// session and person, the code exchanged, the grant's sites matched against
// the configured ones, the account confirmed through the grant, and the grant
// stored. Every outcome but "connected" stores nothing. The error, when there
// is one, says why for the log; it never carries a code, a token or the
// secret.
func (d *DB) CompleteJiraOAuth(ctx context.Context, userID, sessionToken, state, code string, denied bool) (string, error) {
	consumed := d.ConsumeJiraOAuthFlow(state, userID, sessionToken)
	if denied {
		// Declining stores nothing whatever the state says; a valid one is
		// used up all the same.
		return JiraOAuthCancelled, nil
	}
	if consumed != nil {
		return JiraOAuthInvalid, consumed
	}
	if strings.TrimSpace(code) == "" {
		return JiraOAuthInvalid, fmt.Errorf("callback without a code")
	}
	app, _, err := d.JiraOAuthApp()
	if err != nil {
		return JiraOAuthUnreachable, err
	}
	if !app.Configured() {
		return JiraOAuthInvalid, ErrJiraOAuthNotConfigured
	}
	ctx, cancel := context.WithTimeout(ctx, jiraGrantCallTimeout)
	defer cancel()
	endpoints := d.atlassianEndpoints()
	tokens, err := endpoints.Exchange(ctx, d.atlassianHTTP, app, code)
	if errors.Is(err, atlassian.ErrInvalidGrant) {
		return JiraOAuthInvalid, err
	}
	if err != nil {
		return JiraOAuthUnreachable, err
	}
	sites, err := endpoints.Sites(ctx, d.atlassianHTTP, tokens.AccessToken)
	if err != nil {
		return JiraOAuthUnreachable, err
	}
	configured, err := d.ConfiguredJiraSites()
	if err != nil {
		return JiraOAuthUnreachable, err
	}
	covered, cloudID := "", ""
	for _, site := range configured {
		if id, ok := grantedCloudID(sites, site); ok && site != "" {
			covered, cloudID = site, id
			break
		}
	}
	if covered == "" {
		return JiraOAuthNoSite, fmt.Errorf("the grant covers none of the configured Jira sites")
	}
	account, err := d.trackers.CheckJiraBearer(ctx, covered, endpoints.JiraAPIBase(cloudID), tokens.AccessToken)
	if err != nil {
		return JiraOAuthUnreachable, err
	}
	if err := d.SaveJiraGrant(userID, tokens, sites, account); err != nil {
		return JiraOAuthUnreachable, err
	}
	return JiraOAuthConnected, nil
}

// ResolvePersonalCredential is the tracker client's ResolveUser: one person's
// own credential for one tracker, for the site the call is for. An API token
// resolves as it always did; a Jira grant answers its gateway and a live
// access token for that site, a GitHub or GitLab grant (#804) a live access
// token for github.com or gitlab.com, and either answers the missing-credential
// error when it cannot serve the site.
func (d *DB) ResolvePersonalCredential(userID, tracker, site string) (trackerapi.PersonalCredential, error) {
	siteURL, email, token, err := d.userTrackerCredential(userID, tracker)
	switch {
	case errors.Is(err, ErrNoUserCredential):
		return trackerapi.PersonalCredential{}, nil
	case errors.Is(err, errOAuthGrant):
		ctx, cancel := context.WithTimeout(context.Background(), jiraGrantCallTimeout+jiraGrantRefreshWait)
		defer cancel()
		userID, tracker = strings.TrimSpace(userID), strings.ToLower(strings.TrimSpace(tracker))
		if tracker != "jira" {
			return d.resolveForgeGrant(ctx, tracker, userID, site)
		}
		apiBase, bearer, err := d.jiraGrantAccess(ctx, userID, site)
		if err != nil {
			return trackerapi.PersonalCredential{}, err
		}
		return trackerapi.PersonalCredential{APIBase: apiBase, Bearer: bearer}, nil
	case err != nil:
		return trackerapi.PersonalCredential{}, err
	}
	return trackerapi.PersonalCredential{SiteURL: siteURL, Email: email, Token: token}, nil
}
