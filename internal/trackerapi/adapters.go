package trackerapi

import (
	"context"
	"fmt"
	"strings"
	"tasks/internal/models"
	"tasks/internal/tracker"
)

// NewDefaultRegistry constructs a tracker.Registry populated with Github and
// Local adapters using the given tracker API client.
func NewDefaultRegistry(client *Client) *tracker.Registry {
	reg := tracker.NewRegistry()
	reg.Register("github", NewGithubAdapter(client))
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
			},
		},
		client: client,
	}
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
	return g.client.CreateGithubIssue(repo, repoPath, req.Title, req.Description, req.Labels)
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
	return g.client.FetchSingleGithubIssue(repo, repoPath, num)
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
	return g.client.UpdateGithubIssue(repo, repoPath, key, req.Title, req.Description, req.Status, req.Labels, req.RemovedLabels)
}

func (g *GithubAdapter) DeleteIssue(ctx context.Context, req tracker.DeleteIssueRequest) error {
	repo := resolveGithubRepo(req.Project)
	repoPath := resolveRepoPath(req.Project)
	if req.Key == "" {
		return fmt.Errorf("issue key is required")
	}
	return g.client.UpdateGithubIssueState(repo, repoPath, req.Key, models.StatusFinished)
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
	return g.client.SyncFromGithub(repo, repoPath)
}

func (g *GithubAdapter) AddComment(ctx context.Context, req tracker.AddCommentRequest) error {
	repo := resolveGithubRepo(req.Project)
	repoPath := resolveRepoPath(req.Project)
	return g.client.AddIssueComment("github", repo, repoPath, req.Key, req.Body)
}

func (g *GithubAdapter) GetComments(ctx context.Context, req tracker.GetCommentsRequest) ([]models.TaskComment, error) {
	repo := resolveGithubRepo(req.Project)
	repoPath := resolveRepoPath(req.Project)
	return g.client.GetGithubIssueComments(repo, repoPath, req.Key)
}

func (g *GithubAdapter) UpdateLabels(ctx context.Context, key string, add []string, remove []string) error {
	return g.client.UpdateGithubIssue("", "", key, nil, nil, nil, add, remove)
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
