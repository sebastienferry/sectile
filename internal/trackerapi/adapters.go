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
	repo := resolveGithubRepo(req.Project)
	if repo == "" {
		return nil, fmt.Errorf("configure an explicit GitHub owner/repository")
	}
	num, err := cleanGithubIssueNum(req.Key)
	if err != nil {
		return nil, err
	}
	client, err := g.forProject(ctx, req.Project)
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

// forProject is the client of one read: the same one when nothing is stored,
// one carrying the project's own connection parameters otherwise, and the
// acting person's own token where they stored one. A read nobody asked for, the
// synchronisation, carries no acting person and gets the server credential
// (#464).
//
// A personal token that cannot be resolved refuses the call: a sealed token
// nobody unlocked is not an absence, and reading under the server token then
// would put on the activity the name of somebody whose credential was never
// used (ADR 0018).
//
// A person who stored no GitHub token at all keeps the server token for reads
// only: a read attributes nothing, and refusing it would blank their screens.
// Writes go through forWrite, which has no such fallback (#482, ADR 0029). The
// project, too, is read from the context when the request carries none, for the
// calls that take a key and nothing else.
func (g *GithubAdapter) forProject(ctx context.Context, p *models.Project) (*Client, error) {
	projectID := ""
	if p != nil {
		projectID = p.ID
	}
	if projectID == "" {
		projectID = tracker.Project(ctx)
	}
	client, _, err := g.client.ForActingUser(tracker.ActingUser(ctx), "github", projectID)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// forWrite is the client of one write: the acting person's own token, the
// server's for unattended work, and a refusal otherwise (ForWrite). GitHub
// attributes an issue, a comment or a label change to the account behind the
// token, so a person without a token of their own writes nothing.
func (g *GithubAdapter) forWrite(ctx context.Context, p *models.Project) (*Client, error) {
	projectID := ""
	if p != nil {
		projectID = p.ID
	}
	if projectID == "" {
		projectID = tracker.Project(ctx)
	}
	return g.client.ForWrite(ctx, "github", projectID)
}

func resolveGithubRepo(p *models.Project) string {
	if p == nil {
		return ""
	}
	return CleanGithubRepo(p.GithubRepo)
}

func resolveRepoPath(p *models.Project) string {
	if p == nil {
		return ""
	}
	return p.RepoPath
}

func (g *GithubAdapter) CreateIssue(ctx context.Context, req tracker.CreateIssueRequest) (*models.Task, error) {
	repo := resolveGithubRepo(req.Project)
	if repo == "" {
		return nil, fmt.Errorf("configure an explicit GitHub owner/repository")
	}
	repoPath := resolveRepoPath(req.Project)
	client, err := g.forWrite(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	return client.CreateGithubIssue(repo, repoPath, req.Title, req.Description, req.Labels)
}

func (g *GithubAdapter) GetIssue(ctx context.Context, req tracker.GetIssueRequest) (*models.Task, error) {
	repo := resolveGithubRepo(req.Project)
	if repo == "" {
		return nil, fmt.Errorf("configure an explicit GitHub owner/repository")
	}
	repoPath := resolveRepoPath(req.Project)
	num, err := cleanGithubIssueNum(req.Key)
	if err != nil {
		return nil, err
	}
	client, err := g.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	return client.FetchSingleGithubIssue(repo, repoPath, num)
}

func (g *GithubAdapter) UpdateIssue(ctx context.Context, req tracker.UpdateIssueRequest) error {
	repo := resolveGithubRepo(req.Project)
	repoPath := resolveRepoPath(req.Project)
	key := req.Key
	if key == "" && req.Task != nil {
		key = req.Task.Key
	}
	if key == "" {
		return fmt.Errorf("issue key is required")
	}
	client, err := g.forWrite(ctx, req.Project)
	if err != nil {
		return err
	}
	return client.UpdateGithubIssue(repo, repoPath, key, req.Title, req.Description, req.Status, req.Labels, req.RemovedLabels)
}

func (g *GithubAdapter) DeleteIssue(ctx context.Context, req tracker.DeleteIssueRequest) error {
	repo := resolveGithubRepo(req.Project)
	repoPath := resolveRepoPath(req.Project)
	if req.Key == "" {
		return fmt.Errorf("issue key is required")
	}
	client, err := g.forWrite(ctx, req.Project)
	if err != nil {
		return err
	}
	return client.UpdateGithubIssueState(repo, repoPath, req.Key, models.StatusFinished)
}

func (g *GithubAdapter) SyncIssues(ctx context.Context, req tracker.SyncRequest) ([]models.Task, error) {
	repo := req.Repo
	if repo == "" && req.Project != nil {
		repo = req.Project.GithubRepo
	}
	repoPath := req.RepoPath
	if repoPath == "" && req.Project != nil {
		repoPath = req.Project.RepoPath
	}
	client, err := g.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	return client.SyncFromGithub(repo, repoPath, req.UpdatedWithinMin)
}

func (g *GithubAdapter) AddComment(ctx context.Context, req tracker.AddCommentRequest) error {
	repo := resolveGithubRepo(req.Project)
	repoPath := resolveRepoPath(req.Project)
	client, err := g.forWrite(ctx, req.Project)
	if err != nil {
		return err
	}
	return client.AddIssueComment("github", repo, repoPath, req.Key, req.Body)
}

func (g *GithubAdapter) GetComments(ctx context.Context, req tracker.GetCommentsRequest) ([]models.TaskComment, error) {
	repo := resolveGithubRepo(req.Project)
	repoPath := resolveRepoPath(req.Project)
	client, err := g.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	return client.GetGithubIssueComments(repo, repoPath, req.Key)
}

func (g *GithubAdapter) UpdateLabels(ctx context.Context, key string, add []string, remove []string) error {
	client, err := g.forWrite(ctx, nil)
	if err != nil {
		return err
	}
	return client.UpdateGithubIssue("", "", key, nil, nil, nil, add, remove)
}

func (g *GithubAdapter) FormatTaskID(projectID string, key string, rawID string) string {
	cleanNum := strings.TrimPrefix(key, "#")
	cleanNum = strings.TrimPrefix(cleanNum, "GH-#")
	cleanNum = strings.TrimPrefix(cleanNum, "gh-")
	if cleanNum == "" && rawID != "" {
		parts := strings.Split(rawID, "-")
		cleanNum = parts[len(parts)-1]
		cleanNum = strings.TrimPrefix(cleanNum, "#")
	}
	if projectID != "" && projectID != "default" {
		return fmt.Sprintf("gh-%s-%s", projectID, cleanNum)
	}
	return fmt.Sprintf("gh-%s", cleanNum)
}
