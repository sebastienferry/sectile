// Package atlassian owns every call Sectile makes to Atlassian's identity
// endpoints: the consent screen a person is sent to, the exchange of the code
// it returns, the refresh of a grant, and the list of Jira sites a grant
// covers (ADR 0044). Nothing else in the code builds an OAuth request.
//
// The client is confidential: the code and the refresh token are only ever
// exchanged from the server, with the client secret. None of the three, nor an
// access token, may reach an error message, since errors end up in logs and
// activities. Every error here carries an HTTP status and Atlassian's error
// code at most.
package atlassian

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

// App is the OAuth 2.0 integration registered on developer.atlassian.com for
// one deployment.
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

// Endpoints are Atlassian's. Tests point them at a fake.
type Endpoints struct {
	Authorize string
	Token     string
	Resources string
	// API is the gateway every Jira call made through a grant goes to, the
	// site being chosen by its cloudId in the path.
	API string
}

// DefaultEndpoints are the production ones.
var DefaultEndpoints = Endpoints{
	Authorize: "https://auth.atlassian.com/authorize",
	Token:     "https://auth.atlassian.com/oauth/token",
	Resources: "https://api.atlassian.com/oauth/token/accessible-resources",
	API:       "https://api.atlassian.com",
}

// Scopes is what the consent screen asks for, and the registered app must
// declare the same list. Every Jira endpoint Sectile calls maps to one of them
// (specs/654-jira-oauth-connect/spec.md, FR16, and TestEveryJiraPathHasAScope):
// a new endpoint needing another scope changes this list and the registered
// app together. The Agile endpoints only accept granular scopes, which is why
// the two families are mixed. manage:jira-configuration is left out on
// purpose: only the priority search needs it, and a grant reads the plain
// priority list instead (trackerapi.readJiraPriorities).
var Scopes = []string{
	"read:jira-work",
	"write:jira-work",
	"read:jira-user",
	// Without it Atlassian issues no refresh token, and the grant dies with
	// its first access token, an hour later.
	"offline_access",
	"read:board-scope:jira-software",
	"read:board-scope.admin:jira-software",
	"write:board-scope:jira-software",
	"read:sprint:jira-software",
	"write:sprint:jira-software",
	"delete:sprint:jira-software",
	"read:project:jira",
}

// ErrInvalidGrant means Atlassian will never honour this refresh token again:
// the person revoked the app, lost access to every site, or left the grant
// unused for too long. Only a new consent repairs it.
var ErrInvalidGrant = errors.New("atlassian: the grant is no longer valid")

// Tokens is what an exchange or a refresh returns.
type Tokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	Scope        string
}

// Site is one Jira site a grant covers.
type Site struct {
	CloudID string `json:"cloudId"`
	URL     string `json:"url"`
	Name    string `json:"name,omitempty"`
}

// errorBodyLimit bounds what is read of a refusal: only its error code is
// kept.
const errorBodyLimit = 64 << 10

// AuthorizeURL is where a person is sent to consent. prompt=consent shows the
// screen even to someone who consented before, so reconnecting lets them pick
// another site.
func (e Endpoints) AuthorizeURL(app App, state string) string {
	query := url.Values{}
	query.Set("audience", "api.atlassian.com")
	query.Set("client_id", app.ClientID)
	query.Set("scope", strings.Join(Scopes, " "))
	query.Set("redirect_uri", app.RedirectURL)
	query.Set("state", state)
	query.Set("response_type", "code")
	query.Set("prompt", "consent")
	return e.Authorize + "?" + query.Encode()
}

// Exchange trades the code of a consent for a grant.
func (e Endpoints) Exchange(ctx context.Context, hc *http.Client, app App, code string) (Tokens, error) {
	return e.token(ctx, hc, map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     app.ClientID,
		"client_secret": app.ClientSecret,
		"code":          code,
		"redirect_uri":  app.RedirectURL,
	}, false)
}

// Refresh trades a refresh token for a new access token. Atlassian rotates
// refresh tokens: the answer carries a new one and the one sent stops working,
// so the caller must keep the new one before anything else uses the grant.
func (e Endpoints) Refresh(ctx context.Context, hc *http.Client, app App, refreshToken string) (Tokens, error) {
	tokens, err := e.token(ctx, hc, map[string]string{
		"grant_type":    "refresh_token",
		"client_id":     app.ClientID,
		"client_secret": app.ClientSecret,
		"refresh_token": refreshToken,
	}, true)
	if err == nil && tokens.RefreshToken == "" {
		// A refresh that rotates nothing leaves the one sent valid.
		tokens.RefreshToken = refreshToken
	}
	return tokens, err
}

func (e Endpoints) token(ctx context.Context, hc *http.Client, form map[string]string, refreshing bool) (Tokens, error) {
	raw, err := json.Marshal(form)
	if err != nil {
		return Tokens{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.Token, bytes.NewReader(raw))
	if err != nil {
		return Tokens{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := client(hc).Do(req)
	if err != nil {
		return Tokens{}, transportError("token", err)
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(res.Body, errorBodyLimit))
	if err != nil {
		return Tokens{}, fmt.Errorf("atlassian token endpoint: unreadable answer")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var refusal struct {
			Error       string `json:"error"`
			Description string `json:"error_description"`
		}
		_ = json.Unmarshal(body, &refusal)
		if refusal.Error == "invalid_grant" {
			return Tokens{}, ErrInvalidGrant
		}
		// Atlassian also answers a refresh token it no longer knows with
		// unauthorized_client, naming the refresh token in the description.
		// A wrong client id answers the same code without it, and must not
		// disconnect anybody: the admin fixes the app, not every person.
		if refreshing && refusal.Error == "unauthorized_client" && strings.Contains(strings.ToLower(refusal.Description), "refresh_token") {
			return Tokens{}, ErrInvalidGrant
		}
		return Tokens{}, refusalError("token", res.StatusCode, refusal.Error)
	}
	var answer struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &answer); err != nil || answer.AccessToken == "" {
		return Tokens{}, fmt.Errorf("atlassian token endpoint: no access token in the answer")
	}
	return Tokens{
		AccessToken:  answer.AccessToken,
		RefreshToken: answer.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(answer.ExpiresIn) * time.Second).UTC(),
		Scope:        answer.Scope,
	}, nil
}

// Sites lists the Jira sites a grant covers. A resource granting no Jira scope
// (a Confluence site of the same consent) is left out.
func (e Endpoints) Sites(ctx context.Context, hc *http.Client, accessToken string) ([]Site, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.Resources, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	res, err := client(hc).Do(req)
	if err != nil {
		return nil, transportError("accessible-resources", err)
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("atlassian accessible-resources: unreadable answer")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, refusalError("accessible-resources", res.StatusCode, "")
	}
	var resources []struct {
		ID     string   `json:"id"`
		URL    string   `json:"url"`
		Name   string   `json:"name"`
		Scopes []string `json:"scopes"`
	}
	if err := json.Unmarshal(body, &resources); err != nil {
		return nil, fmt.Errorf("atlassian accessible-resources: unreadable answer")
	}
	sites := []Site{}
	for _, resource := range resources {
		if resource.ID == "" || resource.URL == "" || !grantsJira(resource.Scopes) {
			continue
		}
		sites = append(sites, Site{CloudID: resource.ID, URL: resource.URL, Name: resource.Name})
	}
	return sites, nil
}

// JiraAPIBase is the address the Jira REST paths of one site hang under when
// called through a grant.
func (e Endpoints) JiraAPIBase(cloudID string) string {
	return strings.TrimRight(e.API, "/") + "/ex/jira/" + url.PathEscape(cloudID)
}

func grantsJira(scopes []string) bool {
	for _, scope := range scopes {
		if strings.Contains(scope, "jira") {
			return true
		}
	}
	return false
}

func client(hc *http.Client) *http.Client {
	if hc != nil {
		return hc
	}
	return &http.Client{Timeout: 15 * time.Second}
}

// transportError keeps the cause of a failed request, which the HTTP client
// prints with the method and the URL only: the token endpoint carries the
// secret in its body, never in its URL.
func transportError(endpoint string, err error) error {
	return fmt.Errorf("atlassian %s endpoint unreachable: %w", endpoint, err)
}

func refusalError(endpoint string, status int, code string) error {
	if code != "" {
		return fmt.Errorf("atlassian %s endpoint returned HTTP %d (%s)", endpoint, status, code)
	}
	return fmt.Errorf("atlassian %s endpoint returned HTTP %d", endpoint, status)
}
