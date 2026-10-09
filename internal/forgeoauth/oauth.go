// Package forgeoauth owns every call Sectile makes to the OAuth endpoints of
// GitHub and GitLab: the consent screen a person is sent to, the exchange of
// the code it returns, the refresh of a grant, and its revocation when the
// person disconnects. Nothing else in the code builds a forge OAuth request.
//
// The client is confidential: the code and the refresh token are only ever
// exchanged from the server, with the client secret. None of the three, nor an
// access token, may reach an error message, since errors end up in logs and
// activities. Every error here carries an HTTP status and the provider's error
// code at most.
//
// Unlike Atlassian, both forges take their token requests form-encoded, and
// the answer comes in two shapes: a token that never expires and has no
// refresh token (a GitHub OAuth App), or a short-lived token with a rotating
// refresh token (GitLab, and a GitHub app that opted into expiring tokens).
// Tokens says which by its ExpiresAt.
package forgeoauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// App is the OAuth application registered on the forge for one deployment.
type App struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// Configured says whether the app can start a flow: all three values are
// needed, and half an app is none.
func (a App) Configured() bool {
	return strings.TrimSpace(a.ClientID) != "" && strings.TrimSpace(a.ClientSecret) != "" && strings.TrimSpace(a.RedirectURL) != ""
}

// Provider is one forge's OAuth endpoints, and the scopes its consent screen
// asks for. Tests point the endpoints at a fake.
type Provider struct {
	// Name is the tracker the provider serves, and decides the few places
	// where GitHub and GitLab differ.
	Name string
	// AuthorizeEndpoint is the consent screen. It is not called AuthorizeURL,
	// which is the method building the address a person is sent to.
	AuthorizeEndpoint string
	TokenURL          string
	// RevokeURL may carry a {client_id} placeholder, which Revoke fills in.
	RevokeURL string
	// APIBase is the REST root the grant's calls go to, GET /user included.
	APIBase string
	// Scopes is what the consent screen asks for, and the registered app must
	// allow the same list. Every endpoint Sectile calls maps to one of them
	// (TestGithubConsentAsksForExactlyTheNeededScopes and its GitLab twin in
	// trackerapi): a new endpoint needing another scope changes this list and
	// the registered app together.
	Scopes []string
}

// GitHub is an OAuth App on github.com. repo covers the issues, milestones
// and comments of private repositories and the transferIssue mutation;
// read:project covers the ProjectsV2 board status read. GET /user needs no
// scope to read the login.
var GitHub = Provider{
	Name:              "github",
	AuthorizeEndpoint: "https://github.com/login/oauth/authorize",
	TokenURL:          "https://github.com/login/oauth/access_token",
	RevokeURL:         "https://api.github.com/applications/{client_id}/grant",
	APIBase:           "https://api.github.com",
	Scopes:            []string{"repo", "read:project"},
}

// GitLab is an application on gitlab.com. api is the only scope that allows
// the REST writes and the iteration mutations; read_api and write_repository
// cannot perform them.
var GitLab = Provider{
	Name:              "gitlab",
	AuthorizeEndpoint: "https://gitlab.com/oauth/authorize",
	TokenURL:          "https://gitlab.com/oauth/token",
	RevokeURL:         "https://gitlab.com/oauth/revoke",
	APIBase:           "https://gitlab.com/api/v4",
	Scopes:            []string{"api"},
}

// ForTracker answers the provider of a tracker, if it has one.
func ForTracker(tracker string) (Provider, bool) {
	switch strings.ToLower(strings.TrimSpace(tracker)) {
	case GitHub.Name:
		return GitHub, true
	case GitLab.Name:
		return GitLab, true
	}
	return Provider{}, false
}

// ErrInvalidGrant means the forge will never honour this code or refresh
// token again: the person revoked the app, or the code was used or expired.
// Only a new consent repairs it.
var ErrInvalidGrant = errors.New("forgeoauth: the grant is no longer valid")

// Tokens is what an exchange or a refresh returns. A zero ExpiresAt means the
// access token never expires, and then RefreshToken is empty.
type Tokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	Scope        string
}

// deadGrantCodes are the refusals that mean the code or the refresh token is
// gone for good. GitHub names them its own way; GitLab says invalid_grant.
// incorrect_client_credentials is left out on purpose: a wrong client secret
// must not disconnect anybody, the admin fixes the app, not every person.
var deadGrantCodes = map[string]bool{
	"invalid_grant":         true,
	"bad_verification_code": true,
	"bad_refresh_token":     true,
}

// errorBodyLimit bounds what is read of an answer: only its error code is
// kept from a refusal.
const errorBodyLimit = 64 << 10

// AuthorizeURL is where a person is sent to consent. GitHub is told not to
// offer a sign-up, since the person needs an account that already sees the
// repositories.
func (p Provider) AuthorizeURL(app App, state string) string {
	query := url.Values{}
	query.Set("client_id", app.ClientID)
	query.Set("redirect_uri", app.RedirectURL)
	query.Set("scope", strings.Join(p.Scopes, " "))
	query.Set("state", state)
	if p.Name == GitHub.Name {
		query.Set("allow_signup", "false")
	} else {
		query.Set("response_type", "code")
	}
	return p.AuthorizeEndpoint + "?" + query.Encode()
}

// Exchange trades the code of a consent for a grant.
func (p Provider) Exchange(ctx context.Context, hc *http.Client, app App, code string) (Tokens, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", app.RedirectURL)
	form.Set("client_id", app.ClientID)
	form.Set("client_secret", app.ClientSecret)
	return p.token(ctx, hc, form)
}

// Refresh trades a refresh token for a new access token. GitLab rotates
// refresh tokens: the answer carries a new one and the one sent stops working,
// so the caller must keep the new one before anything else uses the grant.
func (p Provider) Refresh(ctx context.Context, hc *http.Client, app App, refreshToken string) (Tokens, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", app.ClientID)
	form.Set("client_secret", app.ClientSecret)
	if p.Name == GitLab.Name {
		// GitLab refuses a refresh that does not repeat the callback.
		form.Set("redirect_uri", app.RedirectURL)
	}
	tokens, err := p.token(ctx, hc, form)
	if err == nil && tokens.RefreshToken == "" {
		// A refresh that rotates nothing leaves the one sent valid.
		tokens.RefreshToken = refreshToken
	}
	return tokens, err
}

func (p Provider) token(ctx context.Context, hc *http.Client, form url.Values) (Tokens, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Tokens{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := client(hc).Do(req)
	if err != nil {
		return Tokens{}, p.transportError("token", err)
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(res.Body, errorBodyLimit))
	if err != nil {
		return Tokens{}, fmt.Errorf("%s token endpoint: unreadable answer", p.Name)
	}
	var answer struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		Scope        string `json:"scope"`
		Error        string `json:"error"`
	}
	decoded := json.Unmarshal(body, &answer) == nil
	// GitHub answers a refusal with status 200 and the reason in the body, so
	// an error code is a refusal whatever the status.
	if answer.Error != "" || res.StatusCode < 200 || res.StatusCode >= 300 {
		if deadGrantCodes[answer.Error] {
			return Tokens{}, fmt.Errorf("%s token endpoint refused the grant (%s): %w", p.Name, answer.Error, ErrInvalidGrant)
		}
		return Tokens{}, p.refusalError("token", res.StatusCode, answer.Error)
	}
	if !decoded || answer.AccessToken == "" {
		return Tokens{}, fmt.Errorf("%s token endpoint: no access token in the answer", p.Name)
	}
	tokens := Tokens{AccessToken: answer.AccessToken, RefreshToken: answer.RefreshToken, Scope: answer.Scope}
	if answer.ExpiresIn > 0 {
		tokens.ExpiresAt = time.Now().Add(time.Duration(answer.ExpiresIn) * time.Second).UTC()
	}
	return tokens, nil
}

// Revoke withdraws a grant at the provider. On GitHub it withdraws every
// token the app holds for that person, which is what a disconnect means.
// A grant the provider no longer knows counts as revoked.
func (p Provider) Revoke(ctx context.Context, hc *http.Client, app App, accessToken string) error {
	var req *http.Request
	var err error
	if p.Name == GitHub.Name {
		raw, marshalErr := json.Marshal(map[string]string{"access_token": accessToken})
		if marshalErr != nil {
			return marshalErr
		}
		endpoint := strings.ReplaceAll(p.RevokeURL, "{client_id}", url.PathEscape(app.ClientID))
		req, err = http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, bytes.NewReader(raw))
		if err != nil {
			return err
		}
		req.SetBasicAuth(app.ClientID, app.ClientSecret)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/vnd.github+json")
	} else {
		form := url.Values{}
		form.Set("client_id", app.ClientID)
		form.Set("client_secret", app.ClientSecret)
		form.Set("token", accessToken)
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, p.RevokeURL, strings.NewReader(form.Encode()))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
	}
	res, err := client(hc).Do(req)
	if err != nil {
		return p.transportError("revoke", err)
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(res.Body, errorBodyLimit))
	if (res.StatusCode >= 200 && res.StatusCode < 300) || (p.Name == GitHub.Name && res.StatusCode == http.StatusNotFound) {
		return nil
	}
	var refusal struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(body, &refusal)
	return p.refusalError("revoke", res.StatusCode, refusal.Error)
}

func client(hc *http.Client) *http.Client {
	if hc != nil {
		return hc
	}
	return &http.Client{Timeout: 15 * time.Second}
}

// transportError keeps the cause of a failed request, which the HTTP client
// prints with the method and the URL only: the secret and the tokens travel
// in the body or a header, never in the URL.
func (p Provider) transportError(endpoint string, err error) error {
	return fmt.Errorf("%s %s endpoint unreachable: %w", p.Name, endpoint, err)
}

func (p Provider) refusalError(endpoint string, status int, code string) error {
	if code != "" {
		return fmt.Errorf("%s %s endpoint returned HTTP %d (%s)", p.Name, endpoint, status, code)
	}
	return fmt.Errorf("%s %s endpoint returned HTTP %d", p.Name, endpoint, status)
}
