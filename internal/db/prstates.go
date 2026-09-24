package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

const taskPullRequestRefreshTimeout = 5 * time.Second

// refreshPullRequestStates reads forge metadata in bounded batches. It never
// reads an issue, enqueues a story sync, or changes link order or workflow.
func (d *DB) refreshPullRequestStates(ctx context.Context, projectID string, tasks []models.Task) []string {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var warnings []string
	for _, forge := range []string{"github", "gitlab"} {
		// Resolve errors (including locked credentials) must never fall back to a
		// different identity's token.
		client, _, err := d.trackers.ForActingUser(tracker.ActingUser(ctx), forge, projectID)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("⚠️ Pull requests %s : %v", forge, err))
			continue
		}
		if client == nil {
			continue
		}
		var urls []string
		for _, task := range tasks {
			for _, link := range task.PrLinks {
				// A merge is final on both forges: re-reading it would only make
				// every sync grow with the project's history.
				if link.State == "merged" {
					continue
				}
				if client.PullRequestForge(link.URL) == forge {
					urls = append(urls, link.URL)
				}
			}
		}
		if len(urls) == 0 {
			continue
		}
		states, err := client.PullRequestStates(ctx, forge, urls)
		if err != nil {
			if isRateLimited(err) {
				d.enterAutoSyncBackoff()
			}
			warnings = append(warnings, fmt.Sprintf("⚠️ Pull requests %s : %v", forge, err))
		}
		for _, task := range tasks {
			if err := d.applyPullRequestStates(task.ID, states); err != nil {
				warnings = append(warnings, fmt.Sprintf("⚠️ Pull requests %s : %v", task.Key, err))
			}
		}
	}
	return warnings
}

// Re-read on the locked row: a concurrent user detachment or new PR, on this
// instance or another, must survive a slow response from the forge. Only state
// on matching URLs changes.
func (d *DB) applyPullRequestStates(taskID string, states map[string]string) error {
	if len(states) == 0 {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.conn.WithTx(func(tx *sqlTx) error {
		task, err := d.lockTaskUnsafe(tx, taskID)
		if err != nil || task == nil {
			return err
		}
		changed := false
		for i := range task.PrLinks {
			state := models.NormalizePullRequestState(states[task.PrLinks[i].URL])
			if state != "" && task.PrLinks[i].State != state {
				task.PrLinks[i].State = state
				changed = true
			}
		}
		if !changed {
			return nil
		}
		_, err = tx.Exec("UPDATE tasks SET pr_links = ? WHERE id = ?", encodePullRequestLinks(task.PrLinks), taskID)
		return err
	})
}

func (d *DB) refreshProjectPullRequestStates(ctx context.Context, projectID string) []string {
	tasks, err := d.GetTasks("", "", "", "", projectID, "", "", "", "", nil, nil, false)
	if err != nil {
		return []string{fmt.Sprintf("⚠️ Pull requests : %v", err)}
	}
	return d.refreshPullRequestStates(ctx, projectID, tasks)
}

func (d *DB) refreshTaskPullRequestStates(ctx context.Context, task *models.Task) *models.Task {
	if task == nil || len(task.PrLinks) == 0 {
		return task
	}
	// The caller is answering a person (an edit, a post-back, a story resync):
	// keep the wait on the forge short. The project sync keeps the full budget.
	ctx, cancel := context.WithTimeout(ctx, taskPullRequestRefreshTimeout)
	defer cancel()
	for _, warning := range d.refreshPullRequestStates(ctx, task.ProjectID, []models.Task{*task}) {
		log.Print(warning)
	}
	if fresh, err := d.GetTaskByID(task.ID); err == nil && fresh != nil {
		states := map[string]string{}
		for _, link := range fresh.PrLinks {
			states[link.URL] = link.State
		}
		for i := range task.PrLinks {
			task.PrLinks[i].State = states[task.PrLinks[i].URL]
		}
		return task
	}
	return task
}
