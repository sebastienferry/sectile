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

	"tasks/internal/tracker"
)

// The server credential of each provider comes from exactly one variable, so a
// deployment driving several trackers can never hand one provider's credential
// to another (#464). Jira authenticates a pair, hence its e-mail.
const (
	GithubTokenVar = "SECTILE_GITHUB_TOKEN"
	GitlabTokenVar = "SECTILE_GITLAB_TOKEN"
	JiraEmailVar   = "SECTILE_JIRA_EMAIL"
	JiraTokenVar   = "SECTILE_JIRA_TOKEN"
)

// RemovedTokenVariable is an environment variable Sectile used to read as a
// tracker credential and no longer does, with what replaces it.
type RemovedTokenVariable struct {
	Name, Replacement string
}

// RemovedTokenVariables lists them, in the order a warning names them. They are
// not unset: the processes Sectile starts keep inheriting them, for the tools
// that read them on their own.
var RemovedTokenVariables = []RemovedTokenVariable{
	{"SECTILE_TRACKER_TOKEN", GithubTokenVar + ", " + JiraTokenVar + " ou " + GitlabTokenVar},
	{"GH_TOKEN", GithubTokenVar},
	{"GITHUB_TOKEN", GithubTokenVar},
	{"JIRA_API_TOKEN", JiraTokenVar},
	{"GITLAB_TOKEN", GitlabTokenVar},
}

// RemovedTokenVariableWarnings names each removed variable the environment still
// sets, one line apiece, for the server to log at startup: a deployment that
// relied on one keeps starting, and its first failed sync already says what to
// set, but the log says it before anybody waits for that.
func RemovedTokenVariableWarnings(getenv func(string) string) []string {
	var warnings []string
	for _, variable := range RemovedTokenVariables {
		if strings.TrimSpace(getenv(variable.Name)) != "" {
			warnings = append(warnings, fmt.Sprintf("%s n'est plus lu comme accès tracker : utilisez %s, ou enregistrez l'accès dans Administration", variable.Name, variable.Replacement))
		}
	}
	return warnings
}

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
	// A server credential stored for a provider but that the server key does
	// not open. The call must fail on it rather than use the environment
	// credential the client still holds: the admin believes the stored one is
	// in use, and another account would put its name on the writes.
	GithubUnreadable, GitlabUnreadable, JiraUnreadable bool
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
	ResolveUser func(userID, tracker string) (siteURL string, email string, token string, err error)

	// actingUser is set on a client resolved for somebody's request, so a
	// missing credential is reported as theirs to add rather than the server's.
	actingUser string
	// unreadable marks the providers whose stored server credential could not
	// be decrypted; see Credentials.
	unreadable struct{ github, gitlab, jira bool }
}

// ForActingUser is For, with the acting user's own credentials substituted
// where they have any. The second result says whether the client carries a
// personal credential: a tracker that attributes its writes to the account
// behind the token uses it to refuse rather than write under the server's name.
func (c *Client) ForActingUser(userID, tracker, projectID string) (*Client, bool, error) {
	resolved := c.For(projectID)
	if resolved == nil || resolved.ResolveUser == nil || strings.TrimSpace(userID) == "" {
		return resolved, false, nil
	}
	siteURL, email, token, err := resolved.ResolveUser(userID, tracker)
	if err != nil {
		return nil, false, err
	}
	// A copy either way: For may have answered the shared client itself.
	personal := *resolved
	personal.actingUser = strings.TrimSpace(userID)
	if token == "" {
		return &personal, false, nil
	}
	switch strings.ToLower(strings.TrimSpace(tracker)) {
	case "jira":
		personal.JiraToken = token
		if email != "" {
			personal.JiraEmail = email
		}
		// An Atlassian account belongs to a site, so the instance travels with
		// the credential rather than with the server.
		if siteURL != "" {
			personal.JiraURL = jiraBaseURL(siteURL)
		}
	case "github":
		personal.GithubToken = token
	case "gitlab":
		personal.GitlabToken = token
	default:
		return resolved, false, nil
	}
	return &personal, true, nil
}

// MissingPersonalCredentialError refuses a write somebody asked for when they
// stored no credential of their own for the provider. The write never falls
// back to the server credential: the tracker would attribute it to the server
// account, a name nobody chose (#482). It reads the same on every provider.
type MissingPersonalCredentialError struct {
	// Tracker is the provider, as the registry names it: "jira", "github" or
	// "gitlab".
	Tracker string
}

func (e *MissingPersonalCredentialError) Error() string {
	return fmt.Sprintf("no personal %s token for this user: add one in Profile → Tracker credentials, or the work would be attributed to the server account", providerName(e.Tracker))
}

// ErrNoActingUser refuses a write whose context names nobody and is not marked
// as unattended work. It is a caller that lost its author on the way, a
// programming error to surface rather than a write to sign with the server
// credential.
var ErrNoActingUser = errors.New("tracker write with no acting user and not marked unattended")

// providerName is how a message spells a provider.
func providerName(tracker string) string {
	switch strings.ToLower(strings.TrimSpace(tracker)) {
	case "jira":
		return "Jira"
	case "github":
		return "GitHub"
	case "gitlab":
		return "GitLab"
	}
	return tracker
}

// ForWrite resolves the client for a tracker write. A named user gets their
// personal credential or an error, never the server's; an unattended context
// gets the server credential; a context naming neither is refused. Reads keep
// ForActingUser and its fallback: a read attributes nothing.
func (c *Client) ForWrite(ctx context.Context, trackerName, projectID string) (*Client, error) {
	user := tracker.ActingUser(ctx)
	if user == "" {
		if tracker.Unattended(ctx) {
			return c.For(projectID), nil
		}
		return nil, ErrNoActingUser
	}
	client, personal, err := c.ForActingUser(user, trackerName, projectID)
	if err != nil {
		return nil, err
	}
	if !personal {
		return nil, &MissingPersonalCredentialError{Tracker: strings.ToLower(strings.TrimSpace(trackerName))}
	}
	return client, nil
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
		GithubToken:   os.Getenv(GithubTokenVar),
		GitlabURL:     strings.TrimRight(gl, "/"),
		GitlabProject: os.Getenv("SECTILE_GITLAB_PROJECT"),
		GitlabToken:   os.Getenv(GitlabTokenVar),
		JiraURL:       jiraBaseURL(os.Getenv("SECTILE_JIRA_URL")),
		JiraEmail:     strings.TrimSpace(os.Getenv(JiraEmailVar)),
		JiraToken:     os.Getenv(JiraTokenVar),
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
	if cred.GithubUnreadable {
		resolved.GithubToken = ""
		resolved.unreadable.github = true
	}
	if cred.GitlabUnreadable {
		resolved.GitlabToken = ""
		resolved.unreadable.gitlab = true
	}
	if cred.JiraUnreadable {
		resolved.JiraEmail, resolved.JiraToken = "", ""
		resolved.unreadable.jira = true
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

// missingCredential says what to do when no token could be resolved, which
// depends on whose credential the call was looking for.
//
// A call made for somebody names the personal credential, because a tracker
// write is attributed to whoever made it and each person stores their own token
// (ADR 0014); pointing them at the deployment would send them to change
// something they usually cannot reach. A call made for nobody, the
// synchronisation, only ever uses the server credential (#464), so it names the
// variable and the Administration page instead.
func (c *Client) missingCredential(tracker string) error {
	name := strings.ToLower(tracker)
	if c != nil {
		unreadable := map[string]bool{"github": c.unreadable.github, "gitlab": c.unreadable.gitlab, "jira": c.unreadable.jira}
		if unreadable[name] {
			return fmt.Errorf("the stored %s server credential cannot be decrypted with the server key: save it again in Administration", tracker)
		}
		if c.actingUser != "" {
			return fmt.Errorf("no %s credential: add yours in Profile → Tracker credentials", tracker)
		}
	}
	return fmt.Errorf("no %s server credential: set %s or save one in Administration", tracker, serverCredentialVariables(name))
}

// serverCredentialVariables names the environment variables of one provider's
// server credential.
func serverCredentialVariables(tracker string) string {
	switch tracker {
	case "jira":
		return JiraEmailVar + " and " + JiraTokenVar
	case "gitlab":
		return GitlabTokenVar
	default:
		return GithubTokenVar
	}
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
		return c.missingCredential("GitHub")
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
		return nil, c.missingCredential("GitHub")
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
	return c.graphqlRequest(ctx, endpoint, token, query, variables, result, false)
}

// graphqlPartial decodes the data a response carries even when some of its
// fields failed. A batch of independent aliases must keep the ones that
// resolved: GitHub answers one deleted or hidden repository with an entry in
// errors next to the data of every other alias. The failures come back as the
// returned error, after result has been filled.
func (c *Client) graphqlPartial(ctx context.Context, endpoint, token, query string, variables map[string]any, result any) error {
	return c.graphqlRequest(ctx, endpoint, token, query, variables, result, true)
}

func (c *Client) graphqlRequest(ctx context.Context, endpoint, token, query string, variables map[string]any, result any, partial bool) error {
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
	failed := len(envelope.Errors) > 0
	if failed && !partial {
		return fmt.Errorf("tracker GraphQL request failed (%d errors)", len(envelope.Errors))
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		if failed {
			return fmt.Errorf("tracker GraphQL request failed (%d errors)", len(envelope.Errors))
		}
		return fmt.Errorf("tracker returned no GraphQL data")
	}
	if err = json.Unmarshal(envelope.Data, result); err != nil {
		return err
	}
	if failed {
		return fmt.Errorf("tracker GraphQL request partly failed (%d errors)", len(envelope.Errors))
	}
	return nil
}

func repository(repo string) (string, error) {
	repo = CleanGithubRepo(repo)
	p := strings.Split(repo, "/")
	if len(p) != 2 || p[0] == "" || p[1] == "" || strings.ContainsAny(repo, "?# \\\r\n") || p[0] == ".." || p[1] == ".." {
		return "", fmt.Errorf("configure an explicit GitHub owner/repository")
	}
	return repo, nil
}

// githubGraphQLEndpoint is the GraphQL entry point of the configured instance:
// github.com serves it beside the REST root, GitHub Enterprise under /api.
func (c *Client) githubGraphQLEndpoint() string {
	if strings.HasSuffix(c.GithubURL, "/api/v3") {
		return strings.TrimSuffix(c.GithubURL, "/api/v3") + "/api/graphql"
	}
	return strings.TrimSuffix(c.GithubURL, "/api/v3") + "/graphql"
}

func (c *Client) GithubGraphQL(query string) ([]byte, error) {
	if c.GithubToken == "" {
		return nil, c.missingCredential("GitHub")
	}
	endpoint := c.githubGraphQLEndpoint()
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
