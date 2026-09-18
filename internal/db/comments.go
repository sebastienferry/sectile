package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"tasks/internal/models"
	"tasks/internal/tracker"
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
	// The Sectile user behind a local comment; empty on rows written before.
	_, _ = d.conn.Exec("ALTER TABLE task_comments ADD COLUMN user_id TEXT NOT NULL DEFAULT '';")
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
// GetTaskComments runs with no acting user, which is what an unattended caller
// does: a tracker whose credential is personal then uses the server one.
func (d *DB) GetTaskComments(taskIDOrKey string) ([]models.TaskComment, error) {
	return d.GetTaskCommentsAs(context.Background(), taskIDOrKey)
}

// GetTaskCommentsAs runs on behalf of whoever asked, so a personal tracker
// credential can be resolved for the call.
func (d *DB) GetTaskCommentsAs(ctx context.Context, taskIDOrKey string) ([]models.TaskComment, error) {
	task, err := d.GetTaskByID(taskIDOrKey)
	if err != nil || task == nil {
		return nil, fmt.Errorf("tâche non trouvée")
	}

	var proj *models.Project
	if task.ProjectID != "" {
		proj, _ = d.GetProjectByID(task.ProjectID)
	}
	ts, tsErr := d.TrackerForTask(task)
	if tsErr == nil && ts != nil && ts.Name() != "local" && ts.Supports(tracker.CapComment) {
		comments, err := ts.GetComments(ctx, tracker.GetCommentsRequest{
			Project: proj,
			Key:     task.Key,
		})
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
	}
	return d.getLocalComments(task.ID)
}

func (d *DB) getLocalComments(taskID string) ([]models.TaskComment, error) {
	d.mu.Lock()
	d.ensureCommentsTable()
	d.mu.Unlock()

	d.mu.RLock()
	rows, err := d.conn.Query(`
		SELECT id, task_id, author, body, created_at, user_id
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
		if err := rows.Scan(&c.ID, &c.TaskID, &c.Author, &c.Body, &created, &c.UserID); err != nil {
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
// have to guess how the tracker rendered it. The comment carries no Sectile
// author; callers that know who is writing use PostTaskCommentBy.
func (d *DB) PostTaskComment(taskIDOrKey string, body string) ([]models.TaskComment, error) {
	return d.PostTaskCommentBy(Actor{}, taskIDOrKey, body)
}

// PostTaskCommentBy is PostTaskComment attributed to a user. A local comment is
// signed with the actor's name and keeps their id; a tracker comment is posted
// under the tracker credential, which is the only author the tracker knows.
func (d *DB) PostTaskCommentBy(actor Actor, taskIDOrKey string, body string) ([]models.TaskComment, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, fmt.Errorf("commentaire vide")
	}

	task, err := d.GetTaskByID(taskIDOrKey)
	if err != nil || task == nil {
		return nil, fmt.Errorf("tâche non trouvée")
	}

	if d.taskTrackerSource(task) == "local" {
		author := strings.TrimSpace(actor.Name)
		if author == "" {
			// A caller without an identity falls back to the legacy profile
			// name, as before roles existed.
			author = "Moi"
			if settings, _ := d.GetSettings(); settings != nil && strings.TrimSpace(settings.UserName) != "" {
				author = settings.UserName
			}
		}
		d.mu.Lock()
		d.ensureCommentsTable()
		_, execErr := d.conn.Exec(
			"INSERT INTO task_comments (id, task_id, author, body, created_at, user_id) VALUES (?, ?, ?, ?, ?, ?)",
			uuid.New().String(), task.ID, author, body, time.Now(), strings.TrimSpace(actor.ID),
		)
		d.mu.Unlock()
		if execErr != nil {
			return nil, execErr
		}
		return d.getLocalComments(task.ID)
	}

	// AddTaskComment routes to the task's own tracker by source.
	if err := d.AddTaskComment(task.ID, body); err != nil {
		return nil, err
	}
	return d.GetTaskComments(task.ID)
}
