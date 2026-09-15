package trackerapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"tasks/internal/models"
	"time"
)

func (c *Client) SyncFromGithub(repo, repoPath string) ([]models.Task, error) {
	repo, err := repository(repo)
	if err != nil {
		return nil, err
	}
	pages, err := c.githubPages(context.Background(), "repos/"+repo+"/issues?per_page=100&state=all")
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
	endpoint := strings.TrimSuffix(c.GithubURL, "/api/v3") + "/graphql"
	if strings.HasSuffix(c.GithubURL, "/api/v3") {
		endpoint = strings.TrimSuffix(c.GithubURL, "/api/v3") + "/api/graphql"
	}
	err = c.graphql(context.Background(), endpoint, "Bearer "+c.GithubToken, `mutation($issue:ID!,$repo:ID!){transferIssue(input:{issueId:$issue,repositoryId:$repo}){issue{number url}}}`, map[string]any{"issue": issue.ID, "repo": repo.ID}, &res)
	if err == nil && (res.TransferIssue.Issue.Number < 1 || res.TransferIssue.Issue.URL == "") {
		err = fmt.Errorf("tracker did not confirm issue transfer")
	}
	return res.TransferIssue.Issue.Number, res.TransferIssue.Issue.URL, err
}
func (c *Client) AddIssueComment(source, repo, repoPath, key, body string) error {
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("comment body is required")
	}
	if source == "linear" {
		return c.addLinearComment(key, body)
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
	var open, merged []PullRequest
	for _, raw := range pages {
		var pr struct {
			URL      string     `json:"html_url"`
			State    string     `json:"state"`
			Draft    bool       `json:"draft"`
			MergedAt *time.Time `json:"merged_at"`
			Head     struct {
				Ref string
				SHA string
			}
		}
		if err = json.Unmarshal(raw, &pr); err != nil {
			return PullRequest{}, err
		}
		found := PullRequest{pr.URL, pr.Head.Ref, pr.Head.SHA, pr.State == "open", pr.Draft, pr.MergedAt != nil}
		switch {
		case found.Open:
			open = append(open, found)
		case found.Merged:
			merged = append(merged, found)
		}
		// A closed-unmerged PR is abandoned work, never evidence.
	}
	if len(open) == 1 {
		return open[0], nil
	}
	if len(open) == 0 && len(merged) == 1 {
		return merged[0], nil
	}
	return PullRequest{}, fmt.Errorf("expected one matching open or merged pull request, got %d open and %d merged", len(open), len(merged))
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
