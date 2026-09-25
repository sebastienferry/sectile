package db

import (
	"errors"
	"slices"
	"strings"
	"time"

	"tasks/internal/models"
)

// A saved view may name the repository its work lives in (#429). A launch from
// that view records the repository on the ticket, with the user who launched:
// the ticket's pull request may then live there, whatever the project's own
// repository, and GitLab is only readable through that user's agent.

// ErrViewDoesNotSelectProject refuses a launch from a view that does not show
// the ticket: nothing about the view applies to it.
var ErrViewDoesNotSelectProject = errors.New("la vue ne sélectionne pas le projet de ce ticket")

// ViewLaunchRepository checks that the user's view may launch a ticket of
// projectID and returns the identity of the view's repository, empty when the
// view names none.
func (d *DB) ViewLaunchRepository(userID, viewID, projectID string) (string, error) {
	view, err := d.GetBoardView(userID, viewID)
	if err != nil {
		return "", err
	}
	if !slices.Contains(view.ProjectIDs, projectID) {
		return "", ErrViewDoesNotSelectProject
	}
	if strings.TrimSpace(view.Repository) == "" {
		return "", nil
	}
	return models.RepositoryIdentity(view.Repository), nil
}

// RecordTaskViewRepository records the view repository a ticket was launched
// with and who launched it. An empty identity records nothing: a view without a
// repository says nothing about where the ticket's work lives.
func (d *DB) RecordTaskViewRepository(taskID, identity, userID string) error {
	identity = strings.TrimSpace(identity)
	if identity == "" {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.conn.Exec("UPDATE tasks SET view_repository = ?, view_repository_by = ?, updated_at = ? WHERE id = ?",
		identity, strings.TrimSpace(userID), time.Now(), taskID)
	return err
}

// taskViewRepository is the ticket's recorded view repository when it is not
// the project's own: the project's repository already follows its own rules.
func taskViewRepository(project *models.Project, task *models.Task) string {
	if task == nil {
		return ""
	}
	view := strings.TrimSpace(task.ViewRepository)
	if view == "" || slices.Contains(projectRepositoryIdentities(project), view) {
		return ""
	}
	return view
}

// evidencePrimary is the repository whose pull request is the ticket's own
// for stage evidence: its recorded view repository, else its primary
// repository (#456).
func evidencePrimary(project *models.Project, task *models.Task) string {
	if view := taskViewRepository(project, task); view != "" {
		return view
	}
	return TaskPrimaryRepository(project, task)
}

// taskViewRepositoryUser is who launched the ticket with its view repository.
func (d *DB) taskViewRepositoryUser(taskID string) string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var userID string
	if err := d.conn.QueryRow("SELECT view_repository_by FROM tasks WHERE id = ?", taskID).Scan(&userID); err != nil {
		return ""
	}
	return userID
}
