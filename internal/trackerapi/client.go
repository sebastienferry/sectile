// Package trackerapi accesses remote trackers without workstation tools or files.
package trackerapi

import (
	"bytes"
	"context"
	"encoding/json"
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

type Client struct {
	HTTP                   *http.Client
	GithubURL, GithubToken string
	LinearURL, LinearToken string
}

func NewClient() *Client {
	gh := os.Getenv("SECTILE_GITHUB_API_URL")
	if gh == "" {
		gh = "https://api.github.com"
	}
	token := trackerToken("SECTILE_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN")
	linear := os.Getenv("SECTILE_LINEAR_API_URL")
	if linear == "" {
		linear = "https://api.linear.app/graphql"
	}
	lt := trackerToken("SECTILE_LINEAR_API_KEY", "LINEAR_API_KEY")
	return &Client{HTTP: &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, GithubURL: strings.TrimRight(gh, "/"), GithubToken: token, LinearURL: linear, LinearToken: lt}
}

// trackerToken resolves one provider's credential. The provider-specific variable
// comes first so a deployment serving two trackers cannot hand a GitHub credential
// to Linear; the tracker-agnostic name covers the common single-tracker setup, and
// the remaining names are the providers' own environment conventions.
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
		return nil, res.Header, fmt.Errorf("tracker returned HTTP %d", res.StatusCode)
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

func (c *Client) linear(query string, variables map[string]any, result any) error {
	if c.LinearToken == "" {
		return fmt.Errorf("configure SECTILE_LINEAR_API_KEY on the server")
	}
	return c.graphql(context.Background(), c.LinearURL, c.LinearToken, query, variables, result)
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
