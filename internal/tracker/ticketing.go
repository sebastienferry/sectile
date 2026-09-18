package tracker

import (
	"context"
	"tasks/internal/models"
)

// TicketingSystem is the unified abstraction for issue and work item backends
// (e.g. GitHub Issues, GitLab, Jira, Local SQLite, etc.).
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

	// FormatTaskID computes the canonical local database ID for a task belonging to this ticketing system.
	// projectID is the Sectile/Sectile project ID.
	// key is the tracker-specific issue key (e.g. "#42" or "ENG-123").
	// rawID is the identifier returned by the tracker (or empty if not yet known).
	FormatTaskID(projectID string, key string, rawID string) string

	// The read side: what a project is made of, for the trackers that expose
	// it. The server asks the resolved tracker and never names one; a tracker
	// without the notion answers ErrUnsupported through the base, so the
	// interface shows a limit rather than a failure.

	// ListBoards returns the boards attached to the project (CapBoard).
	ListBoards(ctx context.Context, req BoardsRequest) ([]models.TrackerBoard, error)
	// ListBoardColumns returns a board's columns with the statuses they group (CapBoard).
	ListBoardColumns(ctx context.Context, req BoardRequest) ([]models.TrackerColumn, error)
	// ListSprints returns a board's sprints with their state and dates (CapSprint).
	ListSprints(ctx context.Context, req BoardRequest) ([]models.TrackerSprint, error)
	// ListStatuses returns the statuses a project's work items can be in (CapBoard).
	ListStatuses(ctx context.Context, req ProjectRequest) ([]TrackerStatus, error)
	// ListIssueTypes returns the work item types a project exposes (CapBoard).
	ListIssueTypes(ctx context.Context, req ProjectRequest) ([]string, error)
	// ListEpics returns the project's epics as tasks (CapEpic).
	ListEpics(ctx context.Context, req ProjectRequest) ([]models.Task, error)
	// SearchTeams looks up the tracker's teams by name (CapTeam).
	SearchTeams(ctx context.Context, req TeamSearchRequest) ([]models.TrackerTeam, error)
	// TeamMembers returns the people of one team (CapTeam).
	TeamMembers(ctx context.Context, req TeamRequest) ([]models.TeamMember, error)
	// RequiredCreateFields lists the fields the site makes mandatory on
	// creation for one issue type, beyond project, type and summary (CapCreate).
	RequiredCreateFields(ctx context.Context, req CreateMetaRequest) ([]RequiredField, error)
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
	// ParentKey attaches the new work item to an epic, when the tracker has them.
	ParentKey string
	// Fields carries the values of the fields a site makes mandatory on
	// creation, keyed by field id, as RequiredCreateFields names them.
	Fields map[string]string
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

// ProjectRequest names the project a read-side question is about.
type ProjectRequest struct {
	Project *models.Project
}

// BoardsRequest asks for the boards of a project.
type BoardsRequest struct {
	Project *models.Project
}

// BoardRequest asks about one board of a project.
type BoardRequest struct {
	Project *models.Project
	BoardID string
}

// TeamSearchRequest looks teams up by name.
type TeamSearchRequest struct {
	Project *models.Project
	Query   string
}

// TeamRequest names one team.
type TeamRequest struct {
	Project *models.Project
	TeamID  string
}

// CreateMetaRequest asks what a creation of the given issue type requires.
type CreateMetaRequest struct {
	Project   *models.Project
	IssueType string
}

// TrackerStatus is one status of the tracker's workflow, with the category it
// belongs to ("new", "indeterminate", "done") when the tracker has categories.
type TrackerStatus struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category,omitempty"`
}

// FieldOption is one allowed value of a mandatory creation field.
type FieldOption struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

// RequiredField is a field a site makes mandatory on creation, beyond the
// universal project, type and summary.
type RequiredField struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Options []FieldOption `json:"options"`
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

func (b *BaseTicketingSystem) FormatTaskID(projectID string, key string, rawID string) string {
	if rawID != "" {
		return rawID
	}
	return key
}

func (b *BaseTicketingSystem) ListBoards(ctx context.Context, req BoardsRequest) ([]models.TrackerBoard, error) {
	return nil, Unsupported(b.TrackerName, CapBoard)
}

func (b *BaseTicketingSystem) ListBoardColumns(ctx context.Context, req BoardRequest) ([]models.TrackerColumn, error) {
	return nil, Unsupported(b.TrackerName, CapBoard)
}

func (b *BaseTicketingSystem) ListSprints(ctx context.Context, req BoardRequest) ([]models.TrackerSprint, error) {
	return nil, Unsupported(b.TrackerName, CapSprint)
}

func (b *BaseTicketingSystem) ListStatuses(ctx context.Context, req ProjectRequest) ([]TrackerStatus, error) {
	return nil, Unsupported(b.TrackerName, CapBoard)
}

func (b *BaseTicketingSystem) ListIssueTypes(ctx context.Context, req ProjectRequest) ([]string, error) {
	return nil, Unsupported(b.TrackerName, CapBoard)
}

func (b *BaseTicketingSystem) ListEpics(ctx context.Context, req ProjectRequest) ([]models.Task, error) {
	return nil, Unsupported(b.TrackerName, CapEpic)
}

func (b *BaseTicketingSystem) SearchTeams(ctx context.Context, req TeamSearchRequest) ([]models.TrackerTeam, error) {
	return nil, Unsupported(b.TrackerName, CapTeam)
}

func (b *BaseTicketingSystem) TeamMembers(ctx context.Context, req TeamRequest) ([]models.TeamMember, error) {
	return nil, Unsupported(b.TrackerName, CapTeam)
}

func (b *BaseTicketingSystem) RequiredCreateFields(ctx context.Context, req CreateMetaRequest) ([]RequiredField, error) {
	return nil, Unsupported(b.TrackerName, CapCreate)
}
