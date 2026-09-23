package trackerapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

const pullRequestStateBatchSize = 50

type pullRequestReference struct {
	URL, Repository string
	Number          int
}

// PullRequestForge recognizes only links belonging to a configured forge. The
// link is an identifier, never a credential destination or an API endpoint.
func (c *Client) PullRequestForge(raw string) string {
	for _, forge := range []string{"github", "gitlab"} {
		if _, ok := c.pullRequestReference(forge, raw); ok {
			return forge
		}
	}
	return ""
}

func (c *Client) pullRequestReference(forge, raw string) (pullRequestReference, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return pullRequestReference{}, false
	}
	base := c.GithubURL
	if forge == "gitlab" {
		base = c.GitlabURL
	}
	api, err := url.Parse(base)
	if err != nil || api.Host == "" {
		return pullRequestReference{}, false
	}
	host := api.Host
	if forge == "github" && host == "api.github.com" {
		host = "github.com"
	}
	if !strings.EqualFold(u.Host, host) || u.Scheme != api.Scheme {
		return pullRequestReference{}, false
	}
	path := strings.Trim(u.Path, "/")
	if forge == "gitlab" {
		prefix := strings.TrimSuffix(strings.Trim(api.Path, "/"), "api/v4")
		if !strings.HasPrefix(path, prefix) {
			return pullRequestReference{}, false
		}
		path = strings.TrimPrefix(path, prefix)
	}
	parts := strings.Split(path, "/")
	var repo, number string
	if forge == "github" {
		if len(parts) != 4 || parts[2] != "pull" {
			return pullRequestReference{}, false
		}
		repo, number = strings.Join(parts[:2], "/"), parts[3]
	} else {
		n := len(parts)
		if n < 5 || parts[n-3] != "-" || parts[n-2] != "merge_requests" {
			return pullRequestReference{}, false
		}
		repo, number = strings.Join(parts[:n-3], "/"), parts[n-1]
	}
	for _, part := range strings.Split(repo, "/") {
		if part == "" || part == "." || part == ".." {
			return pullRequestReference{}, false
		}
	}
	n, err := strconv.Atoi(number)
	if err != nil || n <= 0 {
		return pullRequestReference{}, false
	}
	return pullRequestReference{raw, repo, n}, true
}

// PullRequestStates batches linked PRs without reading any issue. A failed
// batch or repository does not stop the others: every state that could be read
// is returned with the joined errors, and omitted links keep their state. Only a
// rate limit or an expired context stops the reads, since every further call
// would fail the same way.
func (c *Client) PullRequestStates(ctx context.Context, forge string, links []string) (map[string]string, error) {
	states := map[string]string{}
	groups := map[string][]pullRequestReference{}
	seen := map[string]bool{}
	for _, raw := range links {
		ref, ok := c.pullRequestReference(forge, raw)
		if !ok || seen[raw] {
			continue
		}
		seen[raw] = true
		group := "github"
		if forge == "gitlab" {
			group = ref.Repository
		}
		groups[group] = append(groups[group], ref)
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var errs []error
	for _, key := range keys {
		refs := groups[key]
		for start := 0; start < len(refs); start += pullRequestStateBatchSize {
			end := min(start+pullRequestStateBatchSize, len(refs))
			var err error
			if forge == "github" {
				err = c.githubPullRequestStates(ctx, refs[start:end], states)
			} else {
				err = c.gitlabPullRequestStates(ctx, refs[start:end], states)
			}
			if err != nil {
				errs = append(errs, err)
				if IsRateLimited(err) || ctx.Err() != nil {
					return states, errors.Join(errs...)
				}
			}
		}
	}
	return states, errors.Join(errs...)
}

func forgePullRequestState(state string, conflicts bool) string {
	switch strings.ToLower(state) {
	case "merged":
		return "merged"
	case "closed":
		return "closed"
	case "open", "opened":
		if conflicts {
			return "conflicting"
		}
		return "open"
	}
	return ""
}

func (c *Client) githubPullRequestStates(ctx context.Context, refs []pullRequestReference, states map[string]string) error {
	var query strings.Builder
	query.WriteString("query {")
	for i, ref := range refs {
		parts := strings.Split(ref.Repository, "/")
		fmt.Fprintf(&query, " p%d: repository(owner:%q,name:%q){pullRequest(number:%d){state mergeable}}", i, parts[0], parts[1], ref.Number)
	}
	query.WriteString(" }")
	var data map[string]*struct {
		PullRequest *struct{ State, Mergeable string }
	}
	if strings.TrimSpace(c.GithubToken) == "" {
		return fmt.Errorf("GitHub credentials are not configured")
	}
	err := c.graphqlPartial(ctx, c.githubGraphQLEndpoint(), "Bearer "+c.GithubToken, query.String(), nil, &data)
	for i, ref := range refs {
		node := data[fmt.Sprintf("p%d", i)]
		if node == nil || node.PullRequest == nil {
			continue
		}
		pr := node.PullRequest
		if state := forgePullRequestState(pr.State, pr.Mergeable == "CONFLICTING"); state != "" {
			states[ref.URL] = state
		}
	}
	return err
}

func (c *Client) gitlabPullRequestStates(ctx context.Context, refs []pullRequestReference, states map[string]string) error {
	if strings.TrimSpace(c.GitlabToken) == "" {
		return fmt.Errorf("GitLab credentials are not configured")
	}
	query := url.Values{"scope": {"all"}, "state": {"all"}, "per_page": {"100"}}
	for _, ref := range refs {
		query.Add("iids[]", strconv.Itoa(ref.Number))
	}
	endpoint := strings.TrimRight(c.GitlabURL, "/") + "/projects/" + url.PathEscape(refs[0].Repository) + "/merge_requests?" + query.Encode()
	raw, _, err := c.request(ctx, http.MethodGet, endpoint, "Bearer "+c.GitlabToken, nil)
	if err != nil {
		return err
	}
	var found []struct {
		IID                 int    `json:"iid"`
		State               string `json:"state"`
		HasConflicts        bool   `json:"has_conflicts"`
		DetailedMergeStatus string `json:"detailed_merge_status"`
	}
	if err = json.Unmarshal(raw, &found); err != nil {
		return err
	}
	byID := map[int]string{}
	for _, pr := range found {
		byID[pr.IID] = forgePullRequestState(pr.State, pr.HasConflicts || pr.DetailedMergeStatus == "conflict")
	}
	for _, ref := range refs {
		if state := byID[ref.Number]; state != "" {
			states[ref.URL] = state
		}
	}
	return nil
}
