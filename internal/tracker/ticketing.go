package tracker

import (
	"context"
	"tasks/internal/models"
)

// TicketingSystem is the unified abstraction for issue and work item backends
// (e.g. GitHub Issues, Linear, GitLab, Jira, Local SQLite, etc.).
type TicketingSystem interface {
	// Writer provides the capability querying and fine-grained writes.
	Writer

	// CreateIssue creates a new issue in the remote tracker.
	CreateIssue(ctx context.Context, req CreateIssueRequest) (*models.Task, error)

	// GetIssue fetches a single issue by key/ID from the remote tracker.
	GetIssue(ctx context.Context, req GetIssueRequest) (*models.Task, error)

	// UpdateIssue updates fields (title, body, status, priority, labels, etc.) on an existing issue.
	UpdateIssue(ctx context.Context, req UpdateIssueRequest) error

	// DeleteIssue closes, archives, or deletes an issue in the remote tracker.
	DeleteIssue(ctx context.Context, req DeleteIssueRequest) error

	// SyncIssues imports or syncs all issues matching the project/filter.
	SyncIssues(ctx context.Context, req SyncRequest) ([]models.Task, error)

	// AddComment posts a comment on an issue.
	AddComment(ctx context.Context, req AddCommentRequest) error

	// GetComments retrieves all comments for an issue.
	GetComments(ctx context.Context, req GetCommentsRequest) ([]models.TaskComment, error)
}

// CreateIssueRequest holds the parameters needed to create an issue.
type CreateIssueRequest struct {
	Project     *models.Project
	Title       string
	Description string
	Priority    models.Priority
	Labels      []string
	Assignee    string
	Team        string
	Sprint      string
	IssueType   string
}

// GetIssueRequest identifies an issue to fetch.
type GetIssueRequest struct {
	Project *models.Project
	Key     string
}

// UpdateIssueRequest holds the fields to update on an existing issue.
type UpdateIssueRequest struct {
	Project       *models.Project
	Task          *models.Task
	Key           string
	Title         *string
	Description   *string
	Priority      *models.Priority
	Status        *models.Status
	TargetStatus  string
	Labels        []string
	RemovedLabels []string
	Assignee      *string
}

// DeleteIssueRequest identifies an issue to delete or close.
type DeleteIssueRequest struct {
	Project   *models.Project
	Key       string
	CloseOnly bool
}

// SyncRequest specifies what to synchronize.
type SyncRequest struct {
	Project  *models.Project
	Team     string
	Repo     string
	RepoPath string
}

// AddCommentRequest holds comment body and issue key.
type AddCommentRequest struct {
	Project *models.Project
	Key     string
	Body    string
}

// GetCommentsRequest identifies the issue whose comments should be retrieved.
type GetCommentsRequest struct {
	Project *models.Project
	Key     string
}

// TransitionRequest describes a status transition.
type TransitionRequest struct {
	Project      *models.Project
	Key          string
	TargetStatus string
}

// AssignRequest describes an assignment change.
type AssignRequest struct {
	Project   *models.Project
	Key       string
	AccountID string
}

// BaseTicketingSystem provides default ErrUnsupported implementations for all
// methods of TicketingSystem, allowing concrete tracker adapters to only implement
// the operations they actually support.
type BaseTicketingSystem struct {
	TrackerName  string
	Capabilities []Capability
}

func (b *BaseTicketingSystem) Name() string {
	return b.TrackerName
}

func (b *BaseTicketingSystem) Supports(c Capability) bool {
	return Has(b.Capabilities, c)
}

func (b *BaseTicketingSystem) CreateIssue(ctx context.Context, req CreateIssueRequest) (*models.Task, error) {
	return nil, Unsupported(b.TrackerName, CapCreate)
}

func (b *BaseTicketingSystem) GetIssue(ctx context.Context, req GetIssueRequest) (*models.Task, error) {
	return nil, Unsupported(b.TrackerName, CapGet)
}

func (b *BaseTicketingSystem) UpdateIssue(ctx context.Context, req UpdateIssueRequest) error {
	return Unsupported(b.TrackerName, CapUpdate)
}

func (b *BaseTicketingSystem) DeleteIssue(ctx context.Context, req DeleteIssueRequest) error {
	return Unsupported(b.TrackerName, CapDelete)
}

func (b *BaseTicketingSystem) SyncIssues(ctx context.Context, req SyncRequest) ([]models.Task, error) {
	return nil, Unsupported(b.TrackerName, CapSync)
}

func (b *BaseTicketingSystem) AddComment(ctx context.Context, req AddCommentRequest) error {
	return Unsupported(b.TrackerName, CapComment)
}

func (b *BaseTicketingSystem) GetComments(ctx context.Context, req GetCommentsRequest) ([]models.TaskComment, error) {
	return nil, Unsupported(b.TrackerName, CapComment)
}

func (b *BaseTicketingSystem) Assign(ctx context.Context, key string, personID string) error {
	return Unsupported(b.TrackerName, CapAssign)
}

func (b *BaseTicketingSystem) Transition(ctx context.Context, key string, status string) error {
	return Unsupported(b.TrackerName, CapTransition)
}

func (b *BaseTicketingSystem) SetSprint(ctx context.Context, sprintID string, keys []string) error {
	return Unsupported(b.TrackerName, CapSprint)
}

func (b *BaseTicketingSystem) SetTeam(ctx context.Context, key string, teamID string) error {
	return Unsupported(b.TrackerName, CapTeam)
}

func (b *BaseTicketingSystem) SetParent(ctx context.Context, key string, parentKey string) error {
	return Unsupported(b.TrackerName, CapEpic)
}

func (b *BaseTicketingSystem) UpdateLabels(ctx context.Context, key string, add []string, remove []string) error {
	return Unsupported(b.TrackerName, CapLabels)
}

func (b *BaseTicketingSystem) SearchAssignable(ctx context.Context, key string, query string, limit int) ([]Person, error) {
	return nil, Unsupported(b.TrackerName, CapAssign)
}
