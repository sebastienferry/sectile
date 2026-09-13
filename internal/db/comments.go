package db

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"tasks/internal/models"
)

// Comments live where the ticket lives. On a tracker-backed task the tracker is
// the source of truth, so they are read from it on demand rather than mirrored
// locally, which would drift. A local task has nowhere else to put them, so it
// uses the local table.

const commentsTimeout = 90 * time.Second

func (d *DB) ensureCommentsTable() {
	_, _ = d.conn.Exec(`CREATE TABLE IF NOT EXISTS task_comments (
		id TEXT PRIMARY KEY,
		task_id TEXT NOT NULL,
		author TEXT NOT NULL DEFAULT '',
		body TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
	);`)
	_, _ = d.conn.Exec("CREATE INDEX IF NOT EXISTS idx_task_comments_task ON task_comments(task_id, created_at ASC);")
}

func (d *DB) taskTrackerSource(task *models.Task) string {
	source := task.Source
	if source == "" && task.ProjectID != "" {
		if proj, _ := d.GetProjectByID(task.ProjectID); proj != nil {
			source = proj.IssueTracker
		}
	}
	if source == "" {
		source = "local"
	}
	return source
}

// GetTaskComments returns a task's comments, from the tracker when it has one.
func (d *DB) GetTaskComments(taskIDOrKey string) ([]models.TaskComment, error) {
	task, err := d.GetTaskByID(taskIDOrKey)
	if err != nil || task == nil {
		return nil, fmt.Errorf("tâche non trouvée")
	}

	var proj *models.Project
	if task.ProjectID != "" {
		proj, _ = d.GetProjectByID(task.ProjectID)
	}
	settings, _ := d.GetSettings()

	source := d.taskTrackerSource(task)
	switch source {
	case "github":
		repo := ""
		repoPath := ""
		if proj != nil {
			repo = proj.GithubRepo
			repoPath = proj.RepoPath
		}
		if repo == "" && settings != nil {
			repo = settings.GithubRepo
			repoPath = settings.RepoPath
		}
		comments, err := d.trackers.GetGithubIssueComments(repo, repoPath, task.Key)
		if err == nil {
			for i := range comments {
				comments[i].TaskID = task.ID
			}
			return comments, nil
		}
		local, _ := d.getLocalComments(task.ID)
		if len(local) > 0 {
			return local, nil
		}
		return nil, err

	case "linear":
		repoPath := ""
		if proj != nil {
			repoPath = proj.RepoPath
		}
		if repoPath == "" && settings != nil {
			repoPath = settings.RepoPath
		}
		comments, err := d.trackers.GetLinearIssueComments(repoPath, task.Key)
		if err == nil {
			for i := range comments {
				comments[i].TaskID = task.ID
			}
			return comments, nil
		}
		local, _ := d.getLocalComments(task.ID)
		if len(local) > 0 {
			return local, nil
		}
		return nil, err

	default:
		return d.getLocalComments(task.ID)
	}
}

func (d *DB) getLocalComments(taskID string) ([]models.TaskComment, error) {
	d.mu.Lock()
	d.ensureCommentsTable()
	d.mu.Unlock()

	d.mu.RLock()
	rows, err := d.conn.Query(`
		SELECT id, task_id, author, body, created_at
		FROM task_comments WHERE task_id = ? ORDER BY created_at ASC
	`, taskID)
	d.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.TaskComment{}
	for rows.Next() {
		var c models.TaskComment
		var created time.Time
		if err := rows.Scan(&c.ID, &c.TaskID, &c.Author, &c.Body, &created); err != nil {
			continue
		}
		c.CreatedAt = &created
		c.Source = "local"
		out = append(out, c)
	}
	return out, nil
}

// PostTaskComment records a comment: on the tracker when the task has one, in
// the local table otherwise. Returns the refreshed list so the caller does not
// have to guess how the tracker rendered it.
func (d *DB) PostTaskComment(taskIDOrKey string, body string) ([]models.TaskComment, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, fmt.Errorf("commentaire vide")
	}

	task, err := d.GetTaskByID(taskIDOrKey)
	if err != nil || task == nil {
		return nil, fmt.Errorf("tâche non trouvée")
	}

	if d.taskTrackerSource(task) == "local" {
		author := "Moi"
		if settings, _ := d.GetSettings(); settings != nil && strings.TrimSpace(settings.UserName) != "" {
			author = settings.UserName
		}
		d.mu.Lock()
		d.ensureCommentsTable()
		_, execErr := d.conn.Exec(
			"INSERT INTO task_comments (id, task_id, author, body, created_at) VALUES (?, ?, ?, ?, ?)",
			uuid.New().String(), task.ID, author, body, time.Now(),
		)
		d.mu.Unlock()
		if execErr != nil {
			return nil, execErr
		}
		return d.getLocalComments(task.ID)
	}

	// AddTaskComment routes to the right CLI (acli, gh, linear) by source.
	if err := d.AddTaskComment(task.ID, body); err != nil {
		return nil, err
	}
	return d.GetTaskComments(task.ID)
}
