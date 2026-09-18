// Package trackerapi accesses remote trackers without workstation tools or files.
package trackerapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// genericTokenVar names the tracker-agnostic server credential. The
// provider-specific variables remain supported as overrides.
const genericTokenVar = "SECTILE_TRACKER_TOKEN"

// DefaultGithubURL and DefaultGitlabURL are the public instances, used when
// neither the stored configuration nor the environment names one.
const (
	DefaultGithubURL = "https://api.github.com"
	DefaultGitlabURL = "https://gitlab.com/api/v4"
)

// Credentials carries the connection parameters of one tracker, resolved for
// one project. An empty field means "nothing stored here": the client keeps the
// value it derived from the server environment.
type Credentials struct {
	GithubURL, GithubToken                string
	GitlabURL, GitlabProject, GitlabToken string
	// Jira Cloud authenticates with the account e-mail and an API token; the URL
	// is the site itself (https://acme.atlassian.net), the API prefixes are
	// added per call.
	JiraURL, JiraEmail, JiraToken string
}

type Client struct {
	HTTP                                  *http.Client
	GithubURL, GithubToken                string
	GitlabURL, GitlabProject, GitlabToken string
	JiraURL, JiraEmail, JiraToken         string
	// Resolve returns the credentials stored for one project, by project id, an
	// empty id meaning "no project". It is injected by the store, which is the
	// only component able to read both the settings and the project row; the
	// values above stay as the environment-derived fallback. Without it the
	// client behaves exactly as it did before the configuration existed.
	Resolve func(projectID string) Credentials
	// ResolveUser returns one person's own credentials for one tracker, empty
	// strings when they stored none. It answers an error when they sealed the
	// credential behind a passphrase and have not unlocked it: writing under the
	// server account while somebody believes they act as themselves would
	// misattribute the work, so the caller has to fail instead.
	ResolveUser func(userID, tracker string) (email string, token string, err error)
}

// ForActingUser is For, with the acting user's own credentials substituted
// where they have any. Nobody acting, or nobody with a personal credential,
// gives exactly the client For would.
func (c *Client) ForActingUser(userID, tracker, projectID string) (*Client, error) {
	resolved := c.For(projectID)
	if resolved == nil || resolved.ResolveUser == nil || strings.TrimSpace(userID) == "" {
		return resolved, nil
	}
	email, token, err := resolved.ResolveUser(userID, tracker)
	if err != nil {
		return nil, err
	}
	if token == "" {
		return resolved, nil
	}
	personal := *resolved
	switch strings.ToLower(strings.TrimSpace(tracker)) {
	case "jira":
		personal.JiraToken = token
		if email != "" {
			personal.JiraEmail = email
		}
	case "github":
		personal.GithubToken = token
	case "gitlab":
		personal.GitlabToken = token
	default:
		return resolved, nil
	}
	return &personal, nil
}

func NewClient() *Client {
	gh := os.Getenv("SECTILE_GITHUB_API_URL")
	if gh == "" {
		gh = DefaultGithubURL
	}
	gl := os.Getenv("SECTILE_GITLAB_API_URL")
	if gl == "" {
		gl = DefaultGitlabURL
	}
	return &Client{
		HTTP:          &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		GithubURL:     strings.TrimRight(gh, "/"),
		GithubToken:   trackerToken("SECTILE_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"),
		GitlabURL:     strings.TrimRight(gl, "/"),
		GitlabProject: os.Getenv("SECTILE_GITLAB_PROJECT"),
		GitlabToken:   trackerToken("SECTILE_GITLAB_TOKEN", "GITLAB_TOKEN"),
		JiraURL:       jiraBaseURL(os.Getenv("SECTILE_JIRA_URL")),
		JiraEmail:     strings.TrimSpace(os.Getenv("SECTILE_JIRA_EMAIL")),
		JiraToken:     trackerToken("SECTILE_JIRA_TOKEN", "JIRA_API_TOKEN"),
	}
}

// For returns the client to use for one project: the same one when nothing is
// stored, a shallow copy carrying the stored credentials otherwise. Resolving
// per request rather than at startup is what lets a token typed in the
// interface take effect without restarting the server, and lets two projects
// reach two instances with two credentials.
func (c *Client) For(projectID string) *Client {
	if c == nil || c.Resolve == nil {
		return c
	}
	cred := c.Resolve(projectID)
	if cred == (Credentials{}) {
		return c
	}
	resolved := *c
	if cred.GithubURL != "" {
		resolved.GithubURL = strings.TrimRight(cred.GithubURL, "/")
	}
	if cred.GithubToken != "" {
		resolved.GithubToken = cred.GithubToken
	}
	if cred.GitlabURL != "" {
		resolved.GitlabURL = strings.TrimRight(cred.GitlabURL, "/")
	}
	if cred.GitlabProject != "" {
		resolved.GitlabProject = cred.GitlabProject
	}
	if cred.GitlabToken != "" {
		resolved.GitlabToken = cred.GitlabToken
	}
	if cred.JiraURL != "" {
		resolved.JiraURL = jiraBaseURL(cred.JiraURL)
	}
	if cred.JiraEmail != "" {
		resolved.JiraEmail = strings.TrimSpace(cred.JiraEmail)
	}
	if cred.JiraToken != "" {
		resolved.JiraToken = cred.JiraToken
	}
	return &resolved
}

// HTTPError is a refusal by the tracker. The message stays the generic one, so
// no response body reaches a log or an activity by accident; adapters that know
// their tracker's error shape read Body themselves and quote only what they
// recognise.
type HTTPError struct {
	Status int
	Body   []byte
}

func (e *HTTPError) Error() string { return fmt.Sprintf("tracker returned HTTP %d", e.Status) }

// IsRateLimited reports whether the tracker asked to be left alone (HTTP 429).
func IsRateLimited(err error) bool {
	var httpErr *HTTPError
	return errors.As(err, &httpErr) && httpErr.Status == http.StatusTooManyRequests
}

// trackerToken resolves one provider's credential. The provider-specific variable
// comes first so a deployment serving two trackers cannot hand one provider's
// credential to another; the tracker-agnostic name covers the common
// single-tracker setup, and the remaining names are the providers' own
// environment conventions.
func trackerToken(specific string, fallbacks ...string) string {
	for _, name := range append([]string{specific, genericTokenVar}, fallbacks...) {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return ""
}

func (c *Client) request(ctx context.Context, method, endpoint, token string, payload any) ([]byte, http.Header, error) {
	if strings.TrimSpace(token) == "" {
		return nil, nil, fmt.Errorf("tracker credentials are not configured on the server")
	}
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, nil, err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	// Follow repository moves without forwarding credentials to another origin
	// or allowing a redirect to turn a mutation into a read.
	safe := *client
	safe.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) >= 10 || next.URL.Scheme != req.URL.Scheme || next.URL.Host != req.URL.Host || next.URL.User != nil || next.Method != req.Method {
			return http.ErrUseLastResponse
		}
		return nil
	}
	res, err := safe.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("tracker request failed: %w", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
	if err != nil {
		return nil, nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, res.Header, &HTTPError{Status: res.StatusCode, Body: raw}
	}
	return raw, res.Header, nil
}

func (c *Client) github(ctx context.Context, method, path string, payload, result any) error {
	if c.GithubToken == "" {
		return fmt.Errorf("configure %s on the server", genericTokenVar)
	}
	raw, _, err := c.request(ctx, method, c.GithubURL+"/"+strings.TrimLeft(path, "/"), "Bearer "+c.GithubToken, payload)
	if err != nil {
		return err
	}
	if result != nil {
		return json.Unmarshal(raw, result)
	}
	return nil
}

func (c *Client) githubPages(ctx context.Context, path string) ([]json.RawMessage, error) {
	if c.GithubToken == "" {
		return nil, fmt.Errorf("configure %s on the server", genericTokenVar)
	}
	next := c.GithubURL + "/" + path
	origin, err := url.Parse(c.GithubURL)
	if err != nil {
		return nil, err
	}
	var items []json.RawMessage
	seen := map[string]bool{}
	for next != "" {
		u, err := url.Parse(next)
		if err != nil || u.Scheme != origin.Scheme || u.Host != origin.Host {
			return nil, fmt.Errorf("tracker pagination changed origin")
		}
		if seen[next] {
			return nil, fmt.Errorf("tracker pagination repeated a page")
		}
		seen[next] = true
		raw, h, err := c.request(ctx, http.MethodGet, next, "Bearer "+c.GithubToken, nil)
		if err != nil {
			return nil, err
		}
		var page []json.RawMessage
		if err = json.Unmarshal(raw, &page); err != nil {
			return nil, err
		}
		items = append(items, page...)
		next = ""
		for _, link := range strings.Split(h.Get("Link"), ",") {
			if strings.Contains(link, `rel="next"`) {
				a := strings.Index(link, "<")
				b := strings.Index(link, ">")
				if a >= 0 && b > a {
					ref, e := url.Parse(link[a+1 : b])
					if e != nil {
						return nil, e
					}
					next = u.ResolveReference(ref).String()
				}
			}
		}
	}
	return items, nil
}

func (c *Client) graphql(ctx context.Context, endpoint, token, query string, variables map[string]any, result any) error {
	raw, _, err := c.request(ctx, http.MethodPost, endpoint, token, map[string]any{"query": query, "variables": variables})
	if err != nil {
		return err
	}
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err = json.Unmarshal(raw, &envelope); err != nil {
		return err
	}
	if len(envelope.Errors) > 0 {
		return fmt.Errorf("tracker GraphQL request failed (%d errors)", len(envelope.Errors))
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return fmt.Errorf("tracker returned no GraphQL data")
	}
	return json.Unmarshal(envelope.Data, result)
}

func repository(repo string) (string, error) {
	repo = CleanGithubRepo(repo)
	p := strings.Split(repo, "/")
	if len(p) != 2 || p[0] == "" || p[1] == "" || strings.ContainsAny(repo, "?# \\\r\n") || p[0] == ".." || p[1] == ".." {
		return "", fmt.Errorf("configure an explicit GitHub owner/repository")
	}
	return repo, nil
}

func (c *Client) GithubGraphQL(query string) ([]byte, error) {
	if c.GithubToken == "" {
		return nil, fmt.Errorf("configure %s on the server", genericTokenVar)
	}
	endpoint := strings.TrimSuffix(c.GithubURL, "/api/v3") + "/graphql"
	if strings.HasSuffix(c.GithubURL, "/api/v3") {
		endpoint = strings.TrimSuffix(c.GithubURL, "/api/v3") + "/api/graphql"
	}
	var data json.RawMessage
	if err := c.graphql(context.Background(), endpoint, "Bearer "+c.GithubToken, query, nil, &data); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"data": data})
}

// CheckGithub and CheckGitlab authenticate the given parameters against the
// instance and answer with the account they belong to. The setup screen saves
// nothing until one of them succeeds, so a wrong URL or a stale token is
// reported while the user is still looking at the field rather than by a
// synchronisation failing later.
func (c *Client) CheckGithub(ctx context.Context, apiURL, token string) (string, error) {
	return c.checkAccount(ctx, defaulted(apiURL, DefaultGithubURL), "/user", "Bearer "+strings.TrimSpace(token), "login")
}

func (c *Client) CheckGitlab(ctx context.Context, apiURL, token string) (string, error) {
	return c.checkAccount(ctx, defaulted(apiURL, DefaultGitlabURL), "/user", "Bearer "+strings.TrimSpace(token), "username")
}

func (c *Client) checkAccount(ctx context.Context, apiURL, path, token, field string) (string, error) {
	raw, _, err := c.request(ctx, http.MethodGet, strings.TrimRight(apiURL, "/")+path, token, nil)
	if err != nil {
		return "", err
	}
	var account map[string]any
	if err := json.Unmarshal(raw, &account); err != nil {
		return "", fmt.Errorf("tracker returned an unreadable account: %w", err)
	}
	name, _ := account[field].(string)
	if name == "" {
		return "", fmt.Errorf("tracker returned no account for these credentials")
	}
	return name, nil
}

func defaulted(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
