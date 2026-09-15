package trackerapi

import (
	"context"
	"fmt"
	"strings"
	"tasks/internal/models"
	"tasks/internal/tracker"
)

// NewDefaultRegistry constructs a tracker.Registry populated with Github, Linear,
// and Local adapters using the given tracker API client.
func NewDefaultRegistry(client *Client) *tracker.Registry {
	reg := tracker.NewRegistry()
	reg.Register("github", NewGithubAdapter(client))
	reg.Register("linear", NewLinearAdapter(client))
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

// LinearAdapter adapts Client to tracker.TicketingSystem for Linear.
type LinearAdapter struct {
	tracker.BaseTicketingSystem
	client *Client
}

// NewLinearAdapter creates a TicketingSystem adapter for Linear.
func NewLinearAdapter(client *Client) *LinearAdapter {
	return &LinearAdapter{
		BaseTicketingSystem: tracker.BaseTicketingSystem{
			TrackerName: "linear",
			Capabilities: []tracker.Capability{
				tracker.CapCreate,
				tracker.CapUpdate,
				tracker.CapDelete,
				tracker.CapSync,
				tracker.CapGet,
				tracker.CapComment,
				tracker.CapLabels,
				tracker.CapTransition,
			},
		},
		client: client,
	}
}

func (l *LinearAdapter) CreateIssue(ctx context.Context, req tracker.CreateIssueRequest) (*models.Task, error) {
	team := req.Team
	if team == "" && req.Project != nil {
		team = req.Project.LinearTeam
	}
	return l.client.CreateLinearIssue(team, req.Title, req.Description, req.Priority, req.Labels)
}

func (l *LinearAdapter) GetIssue(ctx context.Context, req tracker.GetIssueRequest) (*models.Task, error) {
	node, _, err := l.client.issue(req.Key)
	if err != nil {
		return nil, err
	}
	tasks, err := linearTasks(LinearQueryResponse{Nodes: []LinearIssueNode{node}})
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("linear issue not found: %s", req.Key)
	}
	return &tasks[0], nil
}

func (l *LinearAdapter) UpdateIssue(ctx context.Context, req tracker.UpdateIssueRequest) error {
	key := req.Key
	if key == "" && req.Task != nil {
		key = req.Task.Key
	}
	if key == "" {
		return fmt.Errorf("issue key is required")
	}
	return l.client.UpdateLinearIssue(key, req.Title, req.Description, req.Priority, req.Status, req.Labels)
}

func (l *LinearAdapter) DeleteIssue(ctx context.Context, req tracker.DeleteIssueRequest) error {
	if req.Key == "" {
		return fmt.Errorf("issue key is required")
	}
	return l.client.UpdateLinearIssueState(req.Key, models.StatusFinished)
}

func (l *LinearAdapter) SyncIssues(ctx context.Context, req tracker.SyncRequest) ([]models.Task, error) {
	team := req.Team
	if team == "" && req.Project != nil {
		team = req.Project.LinearTeam
	}
	return l.client.SyncFromLinear(team)
}

func (l *LinearAdapter) AddComment(ctx context.Context, req tracker.AddCommentRequest) error {
	return l.client.addLinearComment(req.Key, req.Body)
}

func (l *LinearAdapter) GetComments(ctx context.Context, req tracker.GetCommentsRequest) ([]models.TaskComment, error) {
	repoPath := ""
	if req.Project != nil {
		repoPath = req.Project.RepoPath
	}
	return l.client.GetLinearIssueComments(repoPath, req.Key)
}

func (l *LinearAdapter) Transition(ctx context.Context, key string, status string) error {
	st := models.Status(strings.ToLower(strings.ReplaceAll(status, " ", "_")))
	return l.client.UpdateLinearIssueState(key, st)
}

func (l *LinearAdapter) UpdateLabels(ctx context.Context, key string, add []string, remove []string) error {
	return l.client.UpdateLinearIssue(key, nil, nil, nil, nil, add)
}
