package trackerapi

import (
	"context"
	"fmt"
	"strings"
	"tasks/internal/models"
	"tasks/internal/tracker"
)

// NewDefaultRegistry constructs a tracker.Registry populated with the GitHub,
// GitLab, Jira and Local adapters using the given tracker API client.
func NewDefaultRegistry(client *Client) *tracker.Registry {
	reg := tracker.NewRegistry()
	reg.Register("github", NewGithubAdapter(client))
	reg.Register("gitlab", NewGitlabAdapter(client))
	reg.Register("jira", NewJiraAdapter(client))
	reg.Register("local", tracker.NewLocalAdapter())
	return reg
}

// GithubAdapter adapts Client to tracker.TicketingSystem for GitHub Issues.
type GithubAdapter struct {
	tracker.BaseTicketingSystem
	client *Client
}

// NewGithubAdapter creates a TicketingSystem adapter for GitHub Issues.
func NewGithubAdapter(client *Client) *GithubAdapter {
	return &GithubAdapter{
		BaseTicketingSystem: tracker.BaseTicketingSystem{
			TrackerName: "github",
			Capabilities: []tracker.Capability{
				tracker.CapCreate,
				tracker.CapUpdate,
				tracker.CapDelete,
				tracker.CapSync,
				tracker.CapGet,
				tracker.CapComment,
				tracker.CapLabels,
				tracker.CapAssign,
				tracker.CapPullRequests,
				tracker.CapIncrementalSync,
			},
		},
		client: client,
	}
}

// IssuePullRequests implements tracker.PullRequestDiscoverer. GitHub and GitLab
// answer it; Jira and the local board do not declare the capability and do not
// implement the interface.
func (g *GithubAdapter) IssuePullRequests(ctx context.Context, req tracker.IssuePullRequestsRequest) ([]models.TaskPullRequest, error) {
	repo := resolveGithubRepo(req.Tracker)
	if repo == "" {
		return nil, fmt.Errorf("configure an explicit GitHub owner/repository")
	}
	num, err := cleanGithubIssueNum(req.Key)
	if err != nil {
		return nil, err
	}
	client, err := g.forProject(ctx, req.Tracker)
	if err != nil {
		return nil, err
	}
	found, err := client.IssuePullRequests(repo, num)
	if err != nil {
		return nil, err
	}
	links := make([]models.TaskPullRequest, 0, len(found))
	for _, pr := range found {
		links = append(links, models.TaskPullRequest{URL: pr.URL, Branch: pr.Branch})
	}
	return links, nil
}

// forProject is the client of one read: the tracker's server credential,
// whoever is looking (ADR 0058). A read attributes nothing, so a personal
// credential has no business there, and what a screen shows does not depend on
// who opened it. Writes go through forWrite. The tracker is read from the
// context when the request carries none, for the calls that take a key and
// nothing else.
func (g *GithubAdapter) forProject(ctx context.Context, t *models.Tracker) (*Client, error) {
	return g.client.ForRead(trackerIDOf(trackerOf(ctx, t))), nil
}

// forWrite is the client of one write: the acting person's own token, the
// server's for unattended work, and a refusal otherwise (ForWrite). GitHub
// attributes an issue, a comment or a label change to the account behind the
// token, so a person without a token of their own writes nothing.
func (g *GithubAdapter) forWrite(ctx context.Context, t *models.Tracker) (*Client, error) {
	return g.client.ForWrite(ctx, "github", trackerIDOf(trackerOf(ctx, t)))
}

// trackerOf is the tracker a call concerns: the request's, else the one the
// context carries for the calls that take a key and nothing else (#741).
func trackerOf(ctx context.Context, t *models.Tracker) *models.Tracker {
	if t != nil {
		return t
	}
	return tracker.Tracker(ctx)
}

func trackerIDOf(t *models.Tracker) string {
	if t == nil {
		return ""
	}
	return t.ID
}

// resolveGithubRepo is the repository a GitHub tracker names: its scope.
func resolveGithubRepo(t *models.Tracker) string {
	if t == nil {
		return ""
	}
	return CleanGithubRepo(t.Scope)
}

func (g *GithubAdapter) CreateIssue(ctx context.Context, req tracker.CreateIssueRequest) (*models.Task, error) {
	repo := resolveGithubRepo(req.Tracker)
	if repo == "" {
		return nil, fmt.Errorf("configure an explicit GitHub owner/repository")
	}
	repoPath := ""
	client, err := g.forWrite(ctx, req.Tracker)
	if err != nil {
		return nil, err
	}
	return client.CreateGithubIssue(repo, repoPath, req.Title, req.Description, req.Labels)
}

func (g *GithubAdapter) GetIssue(ctx context.Context, req tracker.GetIssueRequest) (*models.Task, error) {
	repo := resolveGithubRepo(req.Tracker)
	if repo == "" {
		return nil, fmt.Errorf("configure an explicit GitHub owner/repository")
	}
	repoPath := ""
	num, err := cleanGithubIssueNum(req.Key)
	if err != nil {
		return nil, err
	}
	client, err := g.forProject(ctx, req.Tracker)
	if err != nil {
		return nil, err
	}
	return client.FetchSingleGithubIssue(repo, repoPath, num)
}

func (g *GithubAdapter) UpdateIssue(ctx context.Context, req tracker.UpdateIssueRequest) error {
	repo := resolveGithubRepo(req.Tracker)
	repoPath := ""
	key := req.Key
	if key == "" && req.Task != nil {
		key = req.Task.Key
	}
	if key == "" {
		return fmt.Errorf("issue key is required")
	}
	client, err := g.forWrite(ctx, req.Tracker)
	if err != nil {
		return err
	}
	return client.UpdateGithubIssue(repo, repoPath, key, req.Title, req.Description, req.Status, req.Labels, req.RemovedLabels)
}

func (g *GithubAdapter) DeleteIssue(ctx context.Context, req tracker.DeleteIssueRequest) error {
	repo := resolveGithubRepo(req.Tracker)
	repoPath := ""
	if req.Key == "" {
		return fmt.Errorf("issue key is required")
	}
	client, err := g.forWrite(ctx, req.Tracker)
	if err != nil {
		return err
	}
	return client.UpdateGithubIssueState(repo, repoPath, req.Key, models.StatusFinished)
}

func (g *GithubAdapter) SyncIssues(ctx context.Context, req tracker.SyncRequest) ([]models.Task, error) {
	repo := req.Repo
	if repo == "" {
		repo = resolveGithubRepo(req.Tracker)
	}
	repoPath := req.RepoPath
	client, err := g.forProject(ctx, req.Tracker)
	if err != nil {
		return nil, err
	}
	return client.SyncFromGithub(repo, repoPath, req.UpdatedWithinMin)
}

func (g *GithubAdapter) AddComment(ctx context.Context, req tracker.AddCommentRequest) error {
	repo := resolveGithubRepo(req.Tracker)
	repoPath := ""
	client, err := g.forWrite(ctx, req.Tracker)
	if err != nil {
		return err
	}
	return client.AddIssueComment("github", repo, repoPath, req.Key, req.Body)
}

func (g *GithubAdapter) GetComments(ctx context.Context, req tracker.GetCommentsRequest) ([]models.TaskComment, error) {
	repo := resolveGithubRepo(req.Tracker)
	repoPath := ""
	client, err := g.forProject(ctx, req.Tracker)
	if err != nil {
		return nil, err
	}
	return client.GetGithubIssueComments(repo, repoPath, req.Key)
}

// UpdateLabels writes on the repository of the tracker the context carries.
// It named no repository at all before the tracker travelled (#741).
func (g *GithubAdapter) UpdateLabels(ctx context.Context, key string, add []string, remove []string) error {
	client, err := g.forWrite(ctx, nil)
	if err != nil {
		return err
	}
	repo := resolveGithubRepo(tracker.Tracker(ctx))
	if repo == "" {
		return fmt.Errorf("configure an explicit GitHub owner/repository")
	}
	return client.UpdateGithubIssue(repo, "", key, nil, nil, nil, add, remove)
}

func (g *GithubAdapter) FormatTaskID(trackerID string, key string, rawID string) string {
	cleanNum := strings.TrimPrefix(key, "#")
	cleanNum = strings.TrimPrefix(cleanNum, "GH-#")
	cleanNum = strings.TrimPrefix(cleanNum, "gh-")
	if cleanNum == "" && rawID != "" {
		parts := strings.Split(rawID, "-")
		cleanNum = parts[len(parts)-1]
		cleanNum = strings.TrimPrefix(cleanNum, "#")
	}
	if trackerID != "" && trackerID != "default" {
		return fmt.Sprintf("gh-%s-%s", trackerID, cleanNum)
	}
	return fmt.Sprintf("gh-%s", cleanNum)
}
