package trackerapi

import (
	"context"
	"fmt"
	"strings"
	"tasks/internal/models"
	"tasks/internal/tracker"
)

// NewDefaultRegistry constructs a tracker.Registry populated with the GitHub,
// Jira and Local adapters using the given tracker API client.
func NewDefaultRegistry(client *Client) *tracker.Registry {
	reg := tracker.NewRegistry()
	reg.Register("github", NewGithubAdapter(client))
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
			},
		},
		client: client,
	}
}

// IssuePullRequests implements tracker.PullRequestDiscoverer. GitHub is the only
// tracker that answers it today; Jira and the local board do not declare the
// capability and do not implement the interface.
func (g *GithubAdapter) IssuePullRequests(ctx context.Context, req tracker.IssuePullRequestsRequest) ([]models.TaskPullRequest, error) {
	repo := resolveGithubRepo(req.Project)
	if repo == "" {
		return nil, fmt.Errorf("configure an explicit GitHub owner/repository")
	}
	num, err := cleanGithubIssueNum(req.Key)
	if err != nil {
		return nil, err
	}
	found, err := g.forProject(ctx, req.Project).IssuePullRequests(repo, num)
	if err != nil {
		return nil, err
	}
	links := make([]models.TaskPullRequest, 0, len(found))
	for _, pr := range found {
		links = append(links, models.TaskPullRequest{URL: pr.URL, Branch: pr.Branch})
	}
	return links, nil
}

// forProject is the client to run one request with: the same one when nothing is
// stored, one carrying the project's own connection parameters otherwise, and
// the acting person's own token where they stored one — a GitHub comment is
// attributed to the account behind the token just as a Jira one is.
//
// Unlike Jira it falls back to the server token rather than refusing: a shared
// GitHub token is how deployments run today, and taking that away would stop
// work that has nothing to do with attribution. The project, too, is read from
// the context when the request carries none, for the writes that take a key and
// nothing else.
func (g *GithubAdapter) forProject(ctx context.Context, p *models.Project) *Client {
	projectID := ""
	if p != nil {
		projectID = p.ID
	}
	if projectID == "" {
		projectID = tracker.Project(ctx)
	}
	client, _, err := g.client.ForActingUser(tracker.ActingUser(ctx), "github", projectID)
	if err != nil || client == nil {
		// A credential that cannot be resolved is not a reason to drop the
		// request: the server's own is still there, and the call will say for
		// itself whether it is accepted.
		return g.client.For(projectID)
	}
	return client
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
	return g.forProject(ctx, req.Project).CreateGithubIssue(repo, repoPath, req.Title, req.Description, req.Labels)
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
	return g.forProject(ctx, req.Project).FetchSingleGithubIssue(repo, repoPath, num)
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
	return g.forProject(ctx, req.Project).UpdateGithubIssue(repo, repoPath, key, req.Title, req.Description, req.Status, req.Labels, req.RemovedLabels)
}

func (g *GithubAdapter) DeleteIssue(ctx context.Context, req tracker.DeleteIssueRequest) error {
	repo := resolveGithubRepo(req.Project)
	repoPath := resolveRepoPath(req.Project)
	if req.Key == "" {
		return fmt.Errorf("issue key is required")
	}
	return g.forProject(ctx, req.Project).UpdateGithubIssueState(repo, repoPath, req.Key, models.StatusFinished)
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
	return g.forProject(ctx, req.Project).SyncFromGithub(repo, repoPath)
}

func (g *GithubAdapter) AddComment(ctx context.Context, req tracker.AddCommentRequest) error {
	repo := resolveGithubRepo(req.Project)
	repoPath := resolveRepoPath(req.Project)
	return g.forProject(ctx, req.Project).AddIssueComment("github", repo, repoPath, req.Key, req.Body)
}

func (g *GithubAdapter) GetComments(ctx context.Context, req tracker.GetCommentsRequest) ([]models.TaskComment, error) {
	repo := resolveGithubRepo(req.Project)
	repoPath := resolveRepoPath(req.Project)
	return g.forProject(ctx, req.Project).GetGithubIssueComments(repo, repoPath, req.Key)
}

func (g *GithubAdapter) UpdateLabels(ctx context.Context, key string, add []string, remove []string) error {
	return g.forProject(ctx, nil).UpdateGithubIssue("", "", key, nil, nil, nil, add, remove)
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
