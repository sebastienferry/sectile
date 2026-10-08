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
	// trackerID is the Sectile tracker the task belongs to (#741).
	// key is the tracker-specific issue key (e.g. "#42" or "ENG-123").
	// rawID is the identifier returned by the tracker (or empty if not yet known).
	FormatTaskID(trackerID string, key string, rawID string) string

	// The read side: what a tracker's project is made of, for the trackers
	// that expose it. The server asks the resolved tracker and never names one; a tracker
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

// PullRequestDiscoverer is the optional read that answers "which pull requests
// belong to this work item". It is deliberately not part of TicketingSystem and
// not implemented by BaseTicketingSystem: a tracker that cannot answer must be
// skipped silently by a synchronisation, not made to fail, so every call site
// type-asserts it and moves on when the assertion does not hold.
type PullRequestDiscoverer interface {
	// IssuePullRequests returns the pull requests attached to one work item,
	// oldest first, so the last one is the current pull request.
	IssuePullRequests(ctx context.Context, req IssuePullRequestsRequest) ([]models.TaskPullRequest, error)
}

// MarkedCommentWriter keeps one comment Sectile owns on an issue, found again
// by the id it was given or by a marker the tracker stores on the comment
// (#663). Like PullRequestDiscoverer it is optional: a tracker without it keeps
// what it would have carried in Sectile, and every call site type-asserts it.
type MarkedCommentWriter interface {
	// UpsertMarkedComment rewrites the comment the request names, else the one
	// carrying its marker, else creates one with the marker, and returns its id.
	// A comment without the marker is never touched.
	UpsertMarkedComment(ctx context.Context, req UpsertMarkedCommentRequest) (string, error)
}

// EpicAxisFieldManager is implemented by an adapter whose epics can carry
// the epic priority and quarter in custom fields (#680), which is Jira. A
// tracker without one keeps both axes as labels only.
type EpicAxisFieldManager interface {
	// EpicAxisFieldCandidates lists the closed-list custom fields of one
	// epic's edit screen, single and cascading selects, with their options.
	EpicAxisFieldCandidates(ctx context.Context, trk *models.Tracker, epicKey string) ([]models.EpicFieldCandidate, error)
	// SetEpicAxisField writes one option of a mapped field on an epic: an
	// option id for a select, "parentId/childId" for a cascade. An empty
	// path clears the field.
	SetEpicAxisField(ctx context.Context, trk *models.Tracker, epicKey string, field models.EpicAxisField, optionPath string) error
}

// PrioritySchemeReader is implemented by an adapter whose tracker has a
// configurable ticket priority scheme (#679), which is Jira. Like the other
// optional interfaces, every call site type-asserts it: a tracker without one
// has no mapping to discover.
type PrioritySchemeReader interface {
	// PriorityScheme lists the tracker's priority options, most urgent first.
	// fresh skips any cache, for a person who just changed the scheme.
	PriorityScheme(ctx context.Context, trk *models.Tracker, fresh bool) ([]models.PriorityOption, error)
	// ClassifyPriority reads an option the way a write without a mapping
	// would: its level, and whether its name alone says so (false is a guess
	// from its rank among n options).
	ClassifyPriority(name string, rank, n int) (models.Priority, bool)
}

// UpsertMarkedCommentRequest names the comment Sectile owns on one issue.
type UpsertMarkedCommentRequest struct {
	Tracker *models.Tracker
	Key     string
	// CommentID is the id remembered from the last write, "" when none.
	CommentID string
	// Marker is the key of the property that tells Sectile's comment from the
	// others, and Value what it holds.
	Marker string
	Value  map[string]any
	// Body is Markdown.
	Body string
}

// IssuePullRequestsRequest names the work item whose pull requests are read.
type IssuePullRequestsRequest struct {
	Tracker *models.Tracker
	Key     string
}

// CreateIssueRequest holds the parameters needed to create an issue.
type CreateIssueRequest struct {
	Tracker     *models.Tracker
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
	// PriorityMapping is the mapping of the project the ticket is created
	// for (#679), empty when it has none: the mapping stays on the project
	// while the write goes to its tracker (#741).
	PriorityMapping models.PriorityMapping
}

// GetIssueRequest identifies an issue to fetch.
type GetIssueRequest struct {
	Tracker *models.Tracker
	Key     string
}

// UpdateIssueRequest holds the fields to update on an existing issue.
type UpdateIssueRequest struct {
	Tracker       *models.Tracker
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
	// PriorityMapping is the mapping of the project a priority change is
	// made for (#679), empty when it has none or no priority is sent.
	PriorityMapping models.PriorityMapping
}

// DeleteIssueRequest identifies an issue to delete or close.
type DeleteIssueRequest struct {
	Tracker   *models.Tracker
	Key       string
	CloseOnly bool
}

// SyncRequest specifies what to synchronize.
type SyncRequest struct {
	Tracker  *models.Tracker
	Team     string
	Repo     string
	RepoPath string
	// UpdatedWithinMin narrows the read to the work items the tracker has
	// touched in the last so many minutes. Zero means the whole project, which
	// is what a synchronisation somebody asked for wants, and what a tracker
	// without CapIncrementalSync gets whatever is asked.
	//
	// It is a duration rather than an instant on purpose: Jira's JQL takes a
	// relative `-15m`, which no clock difference between Sectile and the site
	// can shift, whereas an absolute timestamp would have to be expressed in
	// the site's own timezone.
	UpdatedWithinMin int
}

// AddCommentRequest holds comment body and issue key.
type AddCommentRequest struct {
	Tracker *models.Tracker
	Key     string
	Body    string
}

// GetCommentsRequest identifies the issue whose comments should be retrieved.
type GetCommentsRequest struct {
	Tracker *models.Tracker
	Key     string
}

// TransitionRequest describes a status transition.
type TransitionRequest struct {
	Tracker      *models.Tracker
	Key          string
	TargetStatus string
}

// AssignRequest describes an assignment change.
type AssignRequest struct {
	Tracker   *models.Tracker
	Key       string
	AccountID string
}

// ProjectRequest names the tracker a read-side question is about: its
// project, repository or space.
type ProjectRequest struct {
	Tracker *models.Tracker
	// EpicAxisFields are the custom fields the project the question is asked
	// for maps its epic priority and quarter to (#680), asked besides an
	// epic's own: they stay on the project while the read goes to its
	// tracker (#741). Empty for a project that maps none.
	EpicAxisFields models.EpicAxisFields
}

// BoardsRequest asks for the boards of a tracker.
type BoardsRequest struct {
	Tracker *models.Tracker
}

// BoardRequest asks about one board of a tracker.
type BoardRequest struct {
	Tracker *models.Tracker
	BoardID string
}

// TeamSearchRequest looks teams up by name.
type TeamSearchRequest struct {
	Tracker *models.Tracker
	Query   string
}

// TeamRequest names one team.
type TeamRequest struct {
	Tracker *models.Tracker
	TeamID  string
}

// CreateMetaRequest asks what a creation of the given issue type requires.
type CreateMetaRequest struct {
	Tracker   *models.Tracker
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

func (b *BaseTicketingSystem) FormatTaskID(trackerID string, key string, rawID string) string {
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
