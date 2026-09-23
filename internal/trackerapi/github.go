package trackerapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"tasks/internal/models"
	"time"
)

// SyncFromGithub reads a repository's issues. updatedWithinMin narrows the read
// to the ones GitHub has touched in the last so many minutes, through the
// `since` parameter of the issues endpoint; zero reads the whole repository.
func (c *Client) SyncFromGithub(repo, repoPath string, updatedWithinMin int) ([]models.Task, error) {
	repo, err := repository(repo)
	if err != nil {
		return nil, err
	}
	path := "repos/" + repo + "/issues?per_page=100&state=all"
	if updatedWithinMin > 0 {
		// GitHub dates `updated_at` in UTC and takes the same here, so the
		// instant travels without a timezone having to be agreed on.
		since := time.Now().UTC().Add(-time.Duration(updatedWithinMin) * time.Minute)
		path += "&since=" + url.QueryEscape(since.Format(time.RFC3339))
	}
	pages, err := c.githubPages(context.Background(), path)
	if err != nil {
		return nil, err
	}
	tasks := []models.Task{}
	for _, raw := range pages {
		var item GithubIssueItem
		if err = json.Unmarshal(raw, &item); err != nil {
			return nil, err
		}
		if len(item.PullRequest) > 0 && string(item.PullRequest) != "null" {
			continue
		}
		task, err := githubTask(repo, item)
		if err != nil {
			return nil, err
		}
		task.Position = len(tasks)
		tasks = append(tasks, *task)
	}
	return tasks, nil
}
func (c *Client) FetchSingleGithubIssue(repo, repoPath string, number int) (*models.Task, error) {
	repo, err := repository(repo)
	if err != nil {
		return nil, err
	}
	var item GithubIssueItem
	err = c.github(context.Background(), "GET", fmt.Sprintf("repos/%s/issues/%d", repo, number), nil, &item)
	if err != nil {
		return nil, err
	}
	return githubTask(repo, item)
}
func (c *Client) CreateGithubIssue(repo, repoPath, title, description string, labels []string) (*models.Task, error) {
	repo, err := repository(repo)
	if err != nil {
		return nil, err
	}
	var item GithubIssueItem
	err = c.github(context.Background(), "POST", "repos/"+repo+"/issues", map[string]any{"title": title, "body": description, "labels": labels}, &item)
	if err != nil {
		return nil, err
	}
	if item.Number < 1 {
		return nil, fmt.Errorf("tracker did not confirm a created issue")
	}
	return githubTask(repo, item)
}
func (c *Client) UpdateGithubIssueState(repo, repoPath, key string, status models.Status) error {
	return c.UpdateGithubIssue(repo, repoPath, key, nil, nil, &status, nil, nil)
}
func (c *Client) UpdateGithubIssue(repo, repoPath, key string, title, description *string, status *models.Status, labels, removed []string) error {
	repo, err := repository(repo)
	if err != nil {
		return err
	}
	number, err := cleanGithubIssueNum(key)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("repos/%s/issues/%d", repo, number)
	payload := map[string]any{}
	if title != nil {
		payload["title"] = *title
	}
	if description != nil {
		payload["body"] = *description
	}
	if closed, explicit := isFinishedStatus(status, labels); explicit {
		state := "open"
		if closed {
			state = "closed"
		}
		payload["state"] = state
	}
	if labels != nil || len(removed) > 0 {
		var current GithubIssueItem
		if err = c.github(context.Background(), "GET", path, nil, &current); err != nil {
			return err
		}
		drops := map[string]bool{}
		for _, l := range removed {
			drops[l] = true
		}
		set := map[string]bool{}
		merged := []string{}
		for _, l := range current.Labels {
			if !drops[l.Name] && !set[l.Name] {
				merged = append(merged, l.Name)
				set[l.Name] = true
			}
		}
		for _, l := range labels {
			if l != "" && !set[l] {
				merged = append(merged, l)
				set[l] = true
			}
		}
		payload["labels"] = merged
	}
	if len(payload) == 0 {
		return nil
	}
	return c.github(context.Background(), "PATCH", path, payload, nil)
}

type GithubMilestoneItem struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Description string `json:"description"`
	State       string `json:"state"`
}

func (c *Client) CreateGithubMilestone(repo, repoPath, title, description string) (int, error) {
	repo, err := repository(repo)
	if err != nil {
		return 0, err
	}
	var result GithubMilestoneItem
	err = c.github(context.Background(), "POST", "repos/"+repo+"/milestones", map[string]any{"title": title, "description": description}, &result)
	if err == nil && result.Number < 1 {
		err = fmt.Errorf("tracker did not confirm milestone creation")
	}
	return result.Number, err
}
func (c *Client) DeleteGithubMilestone(repo, repoPath string, n int) error {
	repo, err := repository(repo)
	if err != nil {
		return err
	}
	return c.github(context.Background(), "DELETE", fmt.Sprintf("repos/%s/milestones/%d", repo, n), nil, nil)
}
func (c *Client) UpdateGithubMilestone(repo, repoPath string, n int, title, description, state string) error {
	repo, err := repository(repo)
	if err != nil {
		return err
	}
	payload := map[string]any{}
	if title != "" {
		payload["title"] = title
	}
	if description != "" {
		payload["description"] = description
	}
	if state != "" {
		payload["state"] = state
	}
	return c.github(context.Background(), "PATCH", fmt.Sprintf("repos/%s/milestones/%d", repo, n), payload, nil)
}
func (c *Client) ListGithubMilestones(repo, repoPath string) ([]GithubMilestoneItem, error) {
	repo, err := repository(repo)
	if err != nil {
		return nil, err
	}
	pages, err := c.githubPages(context.Background(), "repos/"+repo+"/milestones?state=all&per_page=100")
	if err != nil {
		return nil, err
	}
	items := []GithubMilestoneItem{}
	for _, raw := range pages {
		var item GithubMilestoneItem
		if err = json.Unmarshal(raw, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}
func (c *Client) SetGithubIssueMilestone(repo, repoPath string, n int, target string) error {
	repo, err := repository(repo)
	if err != nil {
		return err
	}
	var milestone any
	target = strings.TrimSpace(target)
	if target != "" && target != "0" && !strings.EqualFold(target, "null") && !strings.EqualFold(target, "none") {
		num, e := strconv.Atoi(target)
		if e != nil {
			items, e := c.ListGithubMilestones(repo, repoPath)
			if e != nil {
				return e
			}
			for _, item := range items {
				if item.Title == target {
					num = item.Number
					break
				}
			}
		}
		if num < 1 {
			return fmt.Errorf("milestone not found: %s", target)
		}
		milestone = num
	}
	return c.github(context.Background(), "PATCH", fmt.Sprintf("repos/%s/issues/%d", repo, n), map[string]any{"milestone": milestone}, nil)
}
func (c *Client) TransferGithubIssue(source, path string, n int, target, targetPath string) (int, string, error) {
	source, err := repository(source)
	if err != nil {
		return 0, "", err
	}
	target, err = repository(target)
	if err != nil {
		return 0, "", err
	}
	var issue, repo struct {
		ID string `json:"node_id"`
	}
	if err = c.github(context.Background(), "GET", fmt.Sprintf("repos/%s/issues/%d", source, n), nil, &issue); err != nil {
		return 0, "", err
	}
	if err = c.github(context.Background(), "GET", "repos/"+target, nil, &repo); err != nil {
		return 0, "", err
	}
	var res struct {
		TransferIssue struct {
			Issue struct {
				Number int
				URL    string
			}
		}
	}
	err = c.graphql(context.Background(), c.githubGraphQLEndpoint(), "Bearer "+c.GithubToken, `mutation($issue:ID!,$repo:ID!){transferIssue(input:{issueId:$issue,repositoryId:$repo}){issue{number url}}}`, map[string]any{"issue": issue.ID, "repo": repo.ID}, &res)
	if err == nil && (res.TransferIssue.Issue.Number < 1 || res.TransferIssue.Issue.URL == "") {
		err = fmt.Errorf("tracker did not confirm issue transfer")
	}
	return res.TransferIssue.Issue.Number, res.TransferIssue.Issue.URL, err
}
func (c *Client) AddIssueComment(source, repo, repoPath, key, body string) error {
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("comment body is required")
	}
	if source != "github" && !strings.HasPrefix(key, "#") {
		return fmt.Errorf("unsupported tracker %q", source)
	}
	repo, err := repository(repo)
	if err != nil {
		return err
	}
	n, err := cleanGithubIssueNum(key)
	if err != nil {
		return err
	}
	return c.github(context.Background(), "POST", fmt.Sprintf("repos/%s/issues/%d/comments", repo, n), map[string]any{"body": body}, nil)
}
func (c *Client) GetGithubIssueComments(repo, repoPath, key string) ([]models.TaskComment, error) {
	repo, err := repository(repo)
	if err != nil {
		return nil, err
	}
	n, err := cleanGithubIssueNum(key)
	if err != nil {
		return nil, err
	}
	pages, err := c.githubPages(context.Background(), fmt.Sprintf("repos/%s/issues/%d/comments?per_page=100", repo, n))
	if err != nil {
		return nil, err
	}
	comments := []models.TaskComment{}
	for _, raw := range pages {
		var item struct {
			ID        int64
			NodeID    string `json:"node_id"`
			Body      string
			CreatedAt time.Time `json:"created_at"`
			User      struct{ Login string }
		}
		if err = json.Unmarshal(raw, &item); err != nil {
			return nil, err
		}
		id := item.NodeID
		if id == "" {
			id = strconv.FormatInt(item.ID, 10)
		}
		comments = append(comments, models.TaskComment{ID: id, Author: item.User.Login, Body: item.Body, CreatedAt: &item.CreatedAt, Source: "github"})
	}
	return comments, nil
}

// PullRequest is forge evidence independent of an agent checkout.
type PullRequest struct {
	URL    string
	Branch string
	SHA    string
	Open   bool
	Draft  bool
	Merged bool
	// CreatedAt orders a set of pull requests the way the task recorded them:
	// oldest first, so the newest ends up being the current one.
	CreatedAt time.Time
	// Forge is "gitlab" for a merge request the local agent read from GitLab;
	// empty means GitHub, the only forge this client reads.
	Forge string
}

func (c *Client) BranchPullRequest(repo, branch string) (PullRequest, error) {
	repo, err := repository(repo)
	if err != nil {
		return PullRequest{}, err
	}
	if branch == "" {
		return PullRequest{}, fmt.Errorf("task branch is required")
	}
	owner := strings.Split(repo, "/")[0]
	// A merged PR is still the task's PR: the human merge boundary must not strand the task before reviewed.
	pages, err := c.githubPages(context.Background(), "repos/"+repo+"/pulls?state=all&head="+url.QueryEscape(owner+":"+branch)+"&per_page=100")
	if err != nil {
		return PullRequest{}, err
	}
	var open []PullRequest
	var merged []mergedPullRequest
	for _, raw := range pages {
		var pr struct {
			URL       string     `json:"html_url"`
			State     string     `json:"state"`
			Draft     bool       `json:"draft"`
			MergedAt  *time.Time `json:"merged_at"`
			CreatedAt time.Time  `json:"created_at"`
			Head      struct {
				Ref string
				SHA string
			}
		}
		if err = json.Unmarshal(raw, &pr); err != nil {
			return PullRequest{}, err
		}
		found := PullRequest{URL: pr.URL, Branch: pr.Head.Ref, SHA: pr.Head.SHA, Open: pr.State == "open", Draft: pr.Draft, Merged: pr.MergedAt != nil, CreatedAt: pr.CreatedAt}
		switch {
		case found.Open:
			open = append(open, found)
		case found.Merged:
			merged = append(merged, mergedPullRequest{found, *pr.MergedAt})
		}
		// A closed-unmerged PR is abandoned work, never evidence.
	}
	if len(open) == 1 {
		return open[0], nil
	}
	// A branch that produced several merged pull requests is not ambiguous: the
	// branch moved on and the latest merge is its state. Several *open* ones are
	// ambiguous — which is current cannot be guessed without letting the caller's
	// swap guard be decided by the order the forge happened to list them in.
	if len(open) == 0 && len(merged) > 0 {
		latest := merged[0]
		for _, candidate := range merged[1:] {
			if candidate.mergedAt.After(latest.mergedAt) {
				latest = candidate
			}
		}
		return latest.PullRequest, nil
	}
	return PullRequest{}, fmt.Errorf("expected one matching open or merged pull request, got %d open and %d merged", len(open), len(merged))
}

// IssuePullRequests answers the question no branch lookup can: which pull
// requests belong to this issue. It is what lets an instance that knows nothing
// but the issue number — a project recreated elsewhere — find the work again.
//
// The closing references are the authoritative source (OPEN-1 of the
// specification). The issue timeline and a text search on the issue number both
// match a mere mention, and an unrelated pull request quoting "#298" would be
// attached to it.
func (c *Client) IssuePullRequests(repo string, issueNumber int) ([]PullRequest, error) {
	repo, err := repository(repo)
	if err != nil {
		return nil, err
	}
	if issueNumber < 1 {
		return nil, fmt.Errorf("issue number is required")
	}
	if c.GithubToken == "" {
		return nil, fmt.Errorf("%s", missingCredential("GitHub"))
	}
	parts := strings.Split(repo, "/")
	var res struct {
		Repository struct {
			Issue struct {
				ClosedByPullRequestsReferences struct {
					Nodes []struct {
						URL         string    `json:"url"`
						HeadRefName string    `json:"headRefName"`
						CreatedAt   time.Time `json:"createdAt"`
						State       string    `json:"state"`
						Merged      bool      `json:"merged"`
						IsDraft     bool      `json:"isDraft"`
					}
				} `json:"closedByPullRequestsReferences"`
			}
		}
	}
	query := `query($owner:String!,$name:String!,$number:Int!){repository(owner:$owner,name:$name){issue(number:$number){closedByPullRequestsReferences(first:20,includeClosedPrs:true){nodes{url headRefName createdAt state merged isDraft}}}}}`
	vars := map[string]any{"owner": parts[0], "name": parts[1], "number": issueNumber}
	if err = c.graphql(context.Background(), c.githubGraphQLEndpoint(), "Bearer "+c.GithubToken, query, vars, &res); err != nil {
		return nil, err
	}
	var found []PullRequest
	for _, node := range res.Repository.Issue.ClosedByPullRequestsReferences.Nodes {
		if strings.TrimSpace(node.URL) == "" {
			continue
		}
		found = append(found, PullRequest{
			URL:       node.URL,
			Branch:    node.HeadRefName,
			Open:      strings.EqualFold(node.State, "OPEN"),
			Draft:     node.IsDraft,
			Merged:    node.Merged,
			CreatedAt: node.CreatedAt,
		})
	}
	// Oldest first: the task's set is ordered, and its last link is the current
	// pull request.
	sort.SliceStable(found, func(i, j int) bool { return found[i].CreatedAt.Before(found[j].CreatedAt) })
	return found, nil
}

// mergedPullRequest keeps the merge date out of PullRequest: the callers reason
// about identity and state, and only this selection needs the ordering.
type mergedPullRequest struct {
	PullRequest
	mergedAt time.Time
}

// GithubStatusQuery supports both user-owned and organization-owned Projects.
func GithubStatusQuery(repo string) (string, error) {
	repo, err := repository(repo)
	if err != nil {
		return "", err
	}
	parts := strings.Split(repo, "/")
	fields := `projectsV2(first:5){nodes{title fields(first:100){nodes{... on ProjectV2SingleSelectField{name options{name}}}}}}`
	return fmt.Sprintf(`query{repository(owner:%q,name:%q){%s} user:repositoryOwner(login:%q){... on User{%s} ... on Organization{%s}}}`, parts[0], parts[1], fields, parts[0], fields, fields), nil
}
