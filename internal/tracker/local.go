package tracker

import (
	"context"
	"tasks/internal/models"

	"github.com/google/uuid"
)

// LocalAdapter handles local tasks that are not backed by any remote issue tracker.
type LocalAdapter struct {
	BaseTicketingSystem
}

// NewLocalAdapter creates a TicketingSystem for local SQLite-only tasks.
func NewLocalAdapter() *LocalAdapter {
	return &LocalAdapter{
		BaseTicketingSystem: BaseTicketingSystem{
			TrackerName: "local",
			Capabilities: []Capability{
				CapCreate,
				CapUpdate,
				CapDelete,
				CapSync,
				CapGet,
				CapComment,
				CapLabels,
				CapAssign,
				CapTransition,
			},
		},
	}
}

func (l *LocalAdapter) CreateIssue(ctx context.Context, req CreateIssueRequest) (*models.Task, error) {
	// Local storage is managed by DB, so adapter returns nil task to let DB generate local ID
	return nil, nil
}

func (l *LocalAdapter) GetIssue(ctx context.Context, req GetIssueRequest) (*models.Task, error) {
	// Retrieved from local database
	return nil, nil
}

func (l *LocalAdapter) UpdateIssue(ctx context.Context, req UpdateIssueRequest) error {
	// Written to local database
	return nil
}

func (l *LocalAdapter) DeleteIssue(ctx context.Context, req DeleteIssueRequest) error {
	// Deleted in local database
	return nil
}

func (l *LocalAdapter) SyncIssues(ctx context.Context, req SyncRequest) ([]models.Task, error) {
	// No remote sync needed for local tasks
	return []models.Task{}, nil
}

func (l *LocalAdapter) AddComment(ctx context.Context, req AddCommentRequest) error {
	// Comments are stored in the local comments table
	return nil
}

func (l *LocalAdapter) GetComments(ctx context.Context, req GetCommentsRequest) ([]models.TaskComment, error) {
	// Retrieved from local comments table
	return []models.TaskComment{}, nil
}

func (l *LocalAdapter) Transition(ctx context.Context, key string, status string) error {
	return nil
}

func (l *LocalAdapter) Assign(ctx context.Context, key string, personID string) error {
	return nil
}

func (l *LocalAdapter) UpdateLabels(ctx context.Context, key string, add []string, remove []string) error {
	return nil
}

func (l *LocalAdapter) FormatTaskID(projectID string, key string, rawID string) string {
	if rawID != "" {
		return rawID
	}
	return uuid.New().String()
}
