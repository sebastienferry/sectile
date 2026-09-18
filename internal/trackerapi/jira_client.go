package trackerapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Jira Cloud is reached through the same guarded request as GitHub and GitLab.
// What differs is the authentication (Basic, e-mail plus API token), the base
// URL (the site, with the API prefixes added per call) and the error shape,
// which names the offending field and is worth quoting.

const (
	// jiraPageSize is what the platform and Agile APIs serve at most per page.
	jiraPageSize = 100
	// jiraMaxPages caps a runaway pagination: 10 000 work items, past any project
	// this tool has met.
	jiraMaxPages = 100
	// jiraErrorMessageLimit bounds the quoted refusal. Jira names the offending
	// field in it ("customfield_10011: Epic Type is required"), which is what
	// the activity has to show.
	jiraErrorMessageLimit = 400
)

// jiraBaseURL accepts what a user realistically types, a bare site, a full
// URL or a deep link, and reduces it to the scheme plus host the APIs live on.
func jiraBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return ""
	}
	scheme := parsed.Scheme
	if scheme != "http" {
		scheme = "https"
	}
	return scheme + "://" + parsed.Host
}

func jiraBasicAuth(email, token string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(strings.TrimSpace(email)+":"+strings.TrimSpace(token)))
}

// jiraConfigured says what is missing before any network call is attempted.
func (c *Client) jiraConfigured() error {
	switch {
	case c == nil || c.JiraURL == "":
		return fmt.Errorf("configure the Jira site URL")
	case c.JiraEmail == "":
		return fmt.Errorf("configure the Jira account e-mail")
	case c.JiraToken == "":
		return fmt.Errorf("configure the Jira API token or %s on the server", genericTokenVar)
	}
	return nil
}

// jira runs one request against the site. path is absolute on the site
// ("/rest/api/3/issue/PE-1"), query is optional, payload is JSON-encoded when
// not nil, result is decoded when not nil. A refusal quotes Jira's own error
// messages.
func (c *Client) jira(ctx context.Context, method, path string, query url.Values, payload, result any) error {
	if err := c.jiraConfigured(); err != nil {
		return err
	}
	endpoint := c.JiraURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	raw, _, err := c.request(ctx, method, endpoint, jiraBasicAuth(c.JiraEmail, c.JiraToken), payload)
	if err != nil {
		return jiraError(err)
	}
	if result != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, result); err != nil {
			return fmt.Errorf("Jira returned an unreadable answer: %w", err)
		}
	}
	return nil
}

// jiraError appends the messages Jira put in its refusal body, and only those:
// the body itself never travels, so nothing echoing a credential can leak.
func jiraError(err error) error {
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		return err
	}
	switch httpErr.Status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("%w: Jira refused the credentials, check the e-mail and the API token", err)
	}
	var body struct {
		ErrorMessages []string          `json:"errorMessages"`
		Errors        map[string]string `json:"errors"`
	}
	if json.Unmarshal(httpErr.Body, &body) != nil {
		return err
	}
	messages := append([]string{}, body.ErrorMessages...)
	for field, msg := range body.Errors {
		messages = append(messages, field+": "+msg)
	}
	if len(messages) == 0 {
		return err
	}
	quoted := strings.Join(messages, "; ")
	if len(quoted) > jiraErrorMessageLimit {
		quoted = quoted[:jiraErrorMessageLimit] + "…"
	}
	return fmt.Errorf("%w: %s", err, quoted)
}

// jiraAgilePages walks a startAt/isLast pagination and returns the raw items of
// every page's "values".
func (c *Client) jiraAgilePages(ctx context.Context, path string, query url.Values) ([]json.RawMessage, error) {
	if query == nil {
		query = url.Values{}
	}
	var items []json.RawMessage
	startAt := 0
	for page := 0; page < jiraMaxPages; page++ {
		query.Set("startAt", fmt.Sprint(startAt))
		query.Set("maxResults", fmt.Sprint(jiraPageSize))
		var payload struct {
			Values []json.RawMessage `json:"values"`
			IsLast bool              `json:"isLast"`
		}
		if err := c.jira(ctx, http.MethodGet, path, query, nil, &payload); err != nil {
			return items, err
		}
		items = append(items, payload.Values...)
		if payload.IsLast || len(payload.Values) == 0 {
			return items, nil
		}
		startAt += len(payload.Values)
	}
	return items, fmt.Errorf("tracker pagination did not end")
}

// jiraSearchPages walks the token pagination of the enhanced search endpoint
// and returns the raw issues. The removed startAt form of /search answers 410
// on Cloud, which is why this is the only search the client knows.
func (c *Client) jiraSearchPages(ctx context.Context, jql string, fields []string) ([]json.RawMessage, error) {
	var issues []json.RawMessage
	token := ""
	seen := map[string]bool{}
	for page := 0; page < jiraMaxPages; page++ {
		query := url.Values{}
		query.Set("jql", jql)
		query.Set("fields", strings.Join(fields, ","))
		query.Set("maxResults", fmt.Sprint(jiraPageSize))
		if token != "" {
			if seen[token] {
				return issues, fmt.Errorf("tracker pagination repeated a page")
			}
			seen[token] = true
			query.Set("nextPageToken", token)
		}
		var payload struct {
			Issues        []json.RawMessage `json:"issues"`
			NextPageToken string            `json:"nextPageToken"`
			IsLast        bool              `json:"isLast"`
		}
		if err := c.jira(ctx, http.MethodGet, "/rest/api/3/search/jql", query, nil, &payload); err != nil {
			return issues, err
		}
		issues = append(issues, payload.Issues...)
		if payload.IsLast || payload.NextPageToken == "" || len(payload.Issues) == 0 {
			return issues, nil
		}
		token = payload.NextPageToken
	}
	return issues, fmt.Errorf("tracker pagination did not end")
}

// CheckJira authenticates a site, an e-mail and a token against the site's own
// identity endpoint and answers with the account's display name. The setup
// screen saves nothing until it succeeds.
func (c *Client) CheckJira(ctx context.Context, siteURL, email, token string) (string, error) {
	probe := *c
	probe.JiraURL = jiraBaseURL(siteURL)
	probe.JiraEmail = strings.TrimSpace(email)
	probe.JiraToken = strings.TrimSpace(token)
	var me struct {
		AccountID   string `json:"accountId"`
		DisplayName string `json:"displayName"`
	}
	if err := probe.jira(ctx, http.MethodGet, "/rest/api/3/myself", nil, nil, &me); err != nil {
		return "", err
	}
	if me.AccountID == "" {
		return "", fmt.Errorf("tracker returned no account for these credentials")
	}
	if me.DisplayName == "" {
		return me.AccountID, nil
	}
	return me.DisplayName, nil
}
