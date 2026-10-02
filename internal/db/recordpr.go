package db

import (
	"context"
	"fmt"
	"strings"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// RecordPullRequest records a pull request on a task outside a stage
// transition (#697): a skill that opened one, such as create-pr, or the pull
// request of a secondary repository pushed after the stage that required it.
// The link must name a pull request in one of the task's repositories and
// pass the substitution guard of its own repository; it is appended on the
// task branch, and the primary repository's pull request stays the current
// one. The stage, the labels and the tracker are left alone: no stage was
// completed, so there is nothing to report.
func (d *DB) RecordPullRequest(ctx context.Context, actorID, taskKey, url string) (*models.Task, error) {
	url = strings.TrimSpace(url)
	if models.PullRequestRepository(url) == "" {
		return nil, fmt.Errorf("%q is not a GitHub pull request or GitLab merge request URL", url)
	}
	task, err := d.GetTaskByID(taskKey)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, fmt.Errorf("task not found: %s", taskKey)
	}
	project, err := d.GetProjectByID(task.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("read project for the pull request: %w", err)
	}
	primary, allowed := taskPullRequestScope(project, task)

	var links []models.TaskPullRequest
	d.mu.Lock()
	err = d.conn.WithTx(func(tx *sqlTx) error {
		locked, err := d.lockTaskUnsafe(tx, task.ID)
		if err != nil {
			return err
		}
		if locked == nil {
			return fmt.Errorf("task not found: %s", taskKey)
		}
		branch := ""
		if locked.BranchName != nil {
			branch = strings.TrimSpace(*locked.BranchName)
		}
		if err := models.AcceptRepositoryPullRequest(locked.PrLinks, url, branch, allowed); err != nil {
			return err
		}
		links = models.AddPullRequestLink(locked.PrLinks, url, branch, primary)
		_, err = tx.Exec("UPDATE tasks SET pr_url = ?, pr_links = ?, pr_links_detached = 0 WHERE id = ?",
			pullRequestURLValue(links), encodePullRequestLinks(links), task.ID)
		return err
	})
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}
	task.PrLinks = models.NormalizePullRequestLinks(links)
	task.PrURL = pullRequestURLValue(task.PrLinks)
	return d.refreshTaskPullRequestStates(tracker.WithActingUser(ctx, actorID), task), nil
}
