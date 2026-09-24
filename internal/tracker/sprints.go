package tracker

import (
	"context"
	"time"

	"tasks/internal/models"
)

// SprintCreateRequest is one sprint to create on a project's board.
type SprintCreateRequest struct {
	Project *models.Project
	BoardID string
	Name    string
	Start   time.Time
	End     time.Time
}

// SprintManager is what a tracker that owns its sprints implements, next to
// CapSprintManage. It is optional, like the tracker notions it manages: an
// adapter without sprints implements nothing and declares nothing.
//
// Every call is synchronous and returns the tracker's own answer, which is
// what the caller mirrors: a sprint must exist on the tracker, with its id,
// before a work item can be moved into it.
type SprintManager interface {
	CreateSprint(ctx context.Context, req SprintCreateRequest) (models.TrackerSprint, error)
	UpdateSprint(ctx context.Context, project *models.Project, sprintID string, patch models.SprintPatch) (models.TrackerSprint, error)
	// DeleteSprint deletes a sprint; one the tracker no longer knows counts as
	// deleted.
	DeleteSprint(ctx context.Context, project *models.Project, sprintID string) error
}
