package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"tasks/internal/models"
	"tasks/internal/tracker"
	"tasks/internal/trackerapi"
)

// Les macros ne sont pas des cartes simples : le tracker les traite comme des
// conteneurs (ex : GitHub milestones) et la synchro n'importe que Task et Story.
// Leur horizon - NOW, NEXT, LATER - est une décision produit, et le travail
// de cadrage (framing, description, checklist TODOs) vit dans Sectile.

const (
	HorizonNow   = "now"
	HorizonNext  = "next"
	HorizonLater = "later"
	// HorizonHidden est le tout-venant : des macros qui n'ont pas
	// vocation à apparaître dans la roadmap active.
	HorizonHidden = "hidden"
)

const RoadmapLabelPrefix = "roadmap:"

// RoadmapLabel is the label for a horizon, empty for "unclassified".
func RoadmapLabel(horizon string) string {
	h := normalizeHorizon(horizon)
	if h == "" {
		return ""
	}
	return RoadmapLabelPrefix + h
}

// AllRoadmapLabels lists all roadmap horizon labels.
func AllRoadmapLabels() []string {
	return []string{RoadmapLabelPrefix + HorizonNow, RoadmapLabelPrefix + HorizonNext, RoadmapLabelPrefix + HorizonLater, RoadmapLabelPrefix + HorizonHidden}
}

// HorizonFromLabels reads the horizon from labels, empty when it carries none.
func HorizonFromLabels(labels []string) string {
	for _, l := range labels {
		clean := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(l, "#")))
		if strings.HasPrefix(clean, RoadmapLabelPrefix) {
			if h := normalizeHorizon(strings.TrimPrefix(clean, RoadmapLabelPrefix)); h != "" {
				return h
			}
		}
	}
	return ""
}

func normalizeHorizon(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case HorizonNow:
		return HorizonNow
	case HorizonNext:
		return HorizonNext
	case HorizonLater, "future":
		return HorizonLater
	case HorizonHidden:
		return HorizonHidden
	default:
		return ""
	}
}

func (d *DB) ensureMacrosTable() {
	_, _ = d.conn.Exec(`CREATE TABLE IF NOT EXISTS macros (
		framing_comment TEXT NOT NULL DEFAULT '',
		project_id TEXT NOT NULL,
		key TEXT NOT NULL,
		horizon TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '',
		todos TEXT NOT NULL DEFAULT '[]',
		title TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT '',
		closed INTEGER NOT NULL DEFAULT 0,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (project_id, key)
	);`)
	_, _ = d.conn.Exec("CREATE INDEX IF NOT EXISTS idx_macros_project ON macros(project_id, horizon);")
	if d.dialect.RunsLegacyMigrations() {
		_, _ = d.conn.Exec("ALTER TABLE macros ADD COLUMN framing_comment TEXT NOT NULL DEFAULT '';")
	}

	// Migrate from legacy epics table if it exists
	var hasEpics int
	_ = d.conn.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='epics';").Scan(&hasEpics)
	if hasEpics > 0 {
		_, _ = d.conn.Exec(`
			INSERT INTO macros (project_id, key, horizon, description, todos, title, status, closed, updated_at)
			SELECT project_id, key, horizon, description, todos, title, status, closed, updated_at FROM epics
			ON CONFLICT DO NOTHING;
		`)
	}
}

func (d *DB) ensureEpicsTable() {
	d.ensureMacrosTable()
}

// GetProjectMacros returns the macro metadata of a project, keyed by macro key.
func (d *DB) GetProjectMacros(projectID string) ([]models.MacroMeta, error) {
	d.mu.Lock()
	d.ensureMacrosTable()
	d.mu.Unlock()

	proj, _ := d.GetProjectByID(projectID)
	if githubMilestoneMacros(proj) {
		if milestones, err := d.tracker(proj.ID).ListGithubMilestones(proj.GithubRepo, proj.RepoPath); err == nil && len(milestones) > 0 {
			d.mu.Lock()
			for _, m := range milestones {
				key := fmt.Sprintf("M-%d", m.Number)
				closedVal := 0
				if strings.EqualFold(m.State, "closed") {
					closedVal = 1
				}
				_, _ = d.conn.Exec(`
					INSERT INTO macros (project_id, key, title, description, status, closed, updated_at)
					VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
					ON CONFLICT(project_id, key) DO UPDATE SET
						title = excluded.title,
						status = excluded.status,
						closed = excluded.closed
				`, projectID, key, m.Title, m.Description, m.State, closedVal)
			}
			d.mu.Unlock()
		}
	}

	d.mu.RLock()
	rows, err := d.conn.Query(`
		SELECT project_id, key, horizon, description, framing_comment, todos, title, status, closed, updated_at
		FROM macros WHERE project_id = ? ORDER BY key ASC
	`, projectID)
	d.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.MacroMeta{}
	for rows.Next() {
		var e models.MacroMeta
		var todosJSON string
		var closed int
		if err := rows.Scan(&e.ProjectID, &e.Key, &e.Horizon, &e.Description, &e.FramingComment, &todosJSON, &e.Title, &e.Status, &closed, &e.UpdatedAt); err != nil {
			continue
		}
		e.Closed = closed == 1
		e.Todos = parseMacroTodos(todosJSON)
		out = append(out, e)
	}
	return out, nil
}

// GetProjectEpics is an alias for GetProjectMacros.
func (d *DB) GetProjectEpics(projectID string) ([]models.MacroMeta, error) {
	return d.GetProjectMacros(projectID)
}

func parseMacroTodos(raw string) []models.MacroTodo {
	if strings.TrimSpace(raw) == "" || raw == "[]" {
		return []models.MacroTodo{}
	}
	var list []models.MacroTodo
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return []models.MacroTodo{}
	}
	return list
}

func parseEpicTodos(raw string) []models.EpicTodo {
	return parseMacroTodos(raw)
}

// SaveMacroMeta upserts a macro's horizon, description and todos checklist.
// SaveMacroMeta records what Sectile alone owns on a macro: its horizon,
// description, framing and slicing. It writes nothing to the tracker; an edit a
// person makes goes through UpdateMacro, which carries their credential.
func (d *DB) SaveMacroMeta(projectID string, key string, horizon *string, description *string, framingComment *string, todos *[]models.MacroTodo) (*models.MacroMeta, error) {
	projectID = strings.TrimSpace(projectID)
	key = strings.TrimSpace(key)
	if projectID == "" || key == "" {
		return nil, fmt.Errorf("projet et clé de macro obligatoires")
	}
	return d.saveMacroMetaFull(projectID, key, horizon, description, framingComment, todos, nil, nil, nil)
}

func (d *DB) SaveEpicMeta(projectID string, key string, horizon *string, description *string, todos *[]models.EpicTodo) (*models.EpicMeta, error) {
	return d.SaveMacroMeta(projectID, key, horizon, description, nil, todos)
}

// UpdateMacro updates macro metadata (title, horizon, description, framingComment, todos, closed) locally and in GitHub milestone if applicable.
//
// The milestone is written as whoever the context names. A write the tracker
// client refuses for want of their credential keeps the local edit, as any
// failed milestone write does, and is returned with the saved macro so the
// person learns that GitHub was not updated (#482).
func (d *DB) UpdateMacro(ctx context.Context, projectID string, key string, title *string, horizon *string, description *string, framingComment *string, todos *[]models.MacroTodo, closed *bool) (*models.MacroMeta, error) {
	projectID = strings.TrimSpace(projectID)
	key = strings.TrimSpace(key)
	if projectID == "" || key == "" {
		return nil, fmt.Errorf("projet et clé de macro obligatoires")
	}

	var refused error
	proj, _ := d.GetProjectByID(projectID)
	// Only what the milestone carries travels: the horizon, the framing and the
	// slicing are Sectile's own.
	if githubMilestoneMacros(proj) && (title != nil || description != nil || closed != nil) {
		var num int
		if strings.HasPrefix(strings.ToUpper(key), "M-") {
			_, _ = fmt.Sscanf(strings.ToUpper(key), "M-%d", &num)
		}
		if num > 0 {
			state := ""
			if closed != nil {
				if *closed {
					state = "closed"
				} else {
					state = "open"
				}
			}
			newTitle := ""
			if title != nil {
				newTitle = strings.TrimSpace(*title)
			}
			newDesc := ""
			if description != nil {
				newDesc = *description
			}
			if client, err := d.trackerForWrite(ctx, "github", proj.ID); err != nil {
				refused = err
			} else {
				_ = client.UpdateGithubMilestone(proj.GithubRepo, proj.RepoPath, num, newTitle, newDesc, state)
			}
		}
	}

	// If title changed, update any task parent_title in tasks table as well
	if title != nil && strings.TrimSpace(*title) != "" {
		newTitle := strings.TrimSpace(*title)
		d.mu.Lock()
		_, _ = d.conn.Exec("UPDATE tasks SET parent_title = ? WHERE project_id = ? AND (parent_key = ? OR parent_title = ?)", newTitle, projectID, key, key)
		d.mu.Unlock()
	}

	saved, err := d.saveMacroMetaFull(projectID, key, horizon, description, framingComment, todos, title, nil, closed)
	if err == nil && refused != nil {
		return saved, fmt.Errorf("milestone GitHub de %s non mis à jour, modification gardée en local : %w", key, refused)
	}
	return saved, err
}

func (d *DB) UpdateEpic(ctx context.Context, projectID string, key string, title *string, horizon *string, description *string, todos *[]models.EpicTodo, closed *bool) (*models.EpicMeta, error) {
	return d.UpdateMacro(ctx, projectID, key, title, horizon, description, nil, todos, closed)
}

func (d *DB) saveMacroMetaFull(projectID string, key string, horizon *string, description *string, framingComment *string, todos *[]models.MacroTodo, title *string, status *string, closed *bool) (*models.MacroMeta, error) {
	projectID = strings.TrimSpace(projectID)
	key = strings.TrimSpace(key)
	if projectID == "" || key == "" {
		return nil, fmt.Errorf("projet et clé de macro obligatoires")
	}

	d.mu.Lock()
	d.ensureMacrosTable()

	// The row is created if missing, then locked and read: an edit of another
	// field racing on another server instance waits, and this merge starts from
	// what it committed. A row that did not exist reads as the defaults, as
	// before.
	tx, err := d.conn.Begin()
	if err != nil {
		d.mu.Unlock()
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec("INSERT INTO macros (project_id, key) VALUES (?, ?) ON CONFLICT (project_id, key) DO NOTHING", projectID, key); err != nil {
		d.mu.Unlock()
		return nil, err
	}
	current := models.MacroMeta{ProjectID: projectID, Key: key, Todos: []models.MacroTodo{}}
	var todosJSON string
	var closedInt int
	err = tx.QueryRow(`
		SELECT horizon, description, framing_comment, todos, title, status, closed FROM macros WHERE project_id = ? AND key = ?`+d.forUpdate(),
		projectID, key).Scan(&current.Horizon, &current.Description, &current.FramingComment, &todosJSON, &current.Title, &current.Status, &closedInt)
	if err == nil {
		current.Todos = parseMacroTodos(todosJSON)
		current.Closed = closedInt == 1
	}

	if title != nil {
		current.Title = *title
	}
	if status != nil {
		current.Status = *status
	}
	if closed != nil {
		current.Closed = *closed
	}

	if horizon != nil {
		current.Horizon = normalizeHorizon(*horizon)
	}
	if description != nil {
		current.Description = *description
	}
	if framingComment != nil {
		current.FramingComment = *framingComment
	}
	if todos != nil {
		cleaned := make([]models.MacroTodo, 0, len(*todos))
		for _, todo := range *todos {
			text := strings.TrimSpace(todo.Text)
			if text == "" {
				continue
			}
			if strings.TrimSpace(todo.ID) == "" {
				todo.ID = uuid.New().String()
			}
			todo.Text = text
			// L'origine est nettoyée comme le texte : une source composée de
			// blancs est une absence d'origine, pas une origine qui s'appelle
			// « espace ». Un SourceKind inconnu est gardé tel quel plutôt que
			// rejeté : une version ultérieure qui en ajoute un ne doit pas voir
			// une version antérieure effacer ses lignes en les relisant.
			todo.TargetProjectID = strings.TrimSpace(todo.TargetProjectID)
			todo.SourceKind = strings.TrimSpace(todo.SourceKind)
			todo.SourceEntry = strings.TrimSpace(todo.SourceEntry)
			cleaned = append(cleaned, todo)
		}
		current.Todos = cleaned
	}
	current.UpdatedAt = time.Now()

	payload, _ := json.Marshal(current.Todos)
	closedValue := 0
	if current.Closed {
		closedValue = 1
	}
	_, execErr := tx.Exec(`
		INSERT INTO macros (project_id, key, horizon, description, framing_comment, todos, title, status, closed, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id, key) DO UPDATE SET
			horizon = excluded.horizon,
			description = excluded.description,
			framing_comment = excluded.framing_comment,
			todos = excluded.todos,
			title = excluded.title,
			status = excluded.status,
			closed = excluded.closed,
			updated_at = excluded.updated_at
	`, projectID, key, current.Horizon, current.Description, current.FramingComment, string(payload), current.Title, current.Status, closedValue, current.UpdatedAt)
	if execErr == nil {
		execErr = tx.Commit()
	}
	d.mu.Unlock()
	if execErr != nil {
		return nil, execErr
	}

	return &current, nil
}

func (d *DB) saveEpicMetaFull(projectID string, key string, horizon *string, description *string, todos *[]models.EpicTodo, title *string, status *string, closed *bool) (*models.EpicMeta, error) {
	return d.saveMacroMetaFull(projectID, key, horizon, description, nil, todos, title, status, closed)
}

// CreateStoryFromMacroTodo turns a line of macro shaping into a real story in the tracker
// and returns the macro metadata with the story it created, and a notice saying
// what the tracker refused (the Jira parent) when the story exists anyway.
//
// The story and its parent are written as whoever the context names.
func (d *DB) CreateStoryFromMacroTodo(ctx context.Context, projectID string, macroKey string, todoID string) (*models.MacroMeta, *models.Task, string, error) {
	projectID = strings.TrimSpace(projectID)
	macroKey = strings.TrimSpace(macroKey)
	todoID = strings.TrimSpace(todoID)
	if projectID == "" || macroKey == "" || todoID == "" {
		return nil, nil, "", fmt.Errorf("projet, macro et ligne de TODO obligatoires")
	}

	proj, err := d.GetProjectByID(projectID)
	if err != nil || proj == nil {
		return nil, nil, "", fmt.Errorf("projet non trouvé")
	}
	metas, err := d.GetProjectMacros(projectID)
	if err != nil {
		return nil, nil, "", err
	}
	var meta *models.MacroMeta
	for i := range metas {
		if metas[i].Key == macroKey {
			meta = &metas[i]
			break
		}
	}
	if meta == nil {
		return nil, nil, "", fmt.Errorf("macro %s sans cadrage enregistré", macroKey)
	}

	var todo *models.MacroTodo
	for i := range meta.Todos {
		if meta.Todos[i].ID == todoID {
			todo = &meta.Todos[i]
			break
		}
	}
	if todo == nil {
		return nil, nil, "", fmt.Errorf("ligne de TODO introuvable")
	}
	if isRoadmapProjectKey(proj, todo.StoryKey) {
		return nil, nil, "", fmt.Errorf("cette ligne est rattachée à %s, d'un projet de roadmap que Sectile lit sans jamais y écrire", todo.StoryKey)
	}
	if strings.TrimSpace(todo.StoryKey) != "" {
		return nil, nil, "", fmt.Errorf("cette ligne a déjà produit %s", todo.StoryKey)
	}

	// The line's target project is where its story lands; empty is the
	// macro's own project. It is refused before anything is written when the
	// macro could not be the story's parent there.
	target := proj
	if targetID := strings.TrimSpace(todo.TargetProjectID); targetID != "" && targetID != proj.ID {
		target, err = d.GetProjectByID(targetID)
		if err != nil {
			return nil, nil, "", err
		}
		if target == nil {
			return nil, nil, "", fmt.Errorf("le projet cible %s n'existe plus : choisissez-en un autre pour cette ligne", targetID)
		}
		if same, reason := d.sameTrackerInstance(proj, target); !same {
			return nil, nil, "", fmt.Errorf("%s", reason)
		}
	}

	task, notice, err := d.createStoryUnder(ctx, proj, target, macroKey, todo.Text)
	if err != nil {
		return nil, nil, "", fmt.Errorf("erreur création de story: %w", err)
	}

	todo.StoryKey = task.Key
	saved, err := d.SaveMacroMeta(projectID, macroKey, nil, nil, nil, &meta.Todos)
	if err != nil {
		return meta, task, notice, nil
	}
	return saved, task, notice, nil
}

func (d *DB) CreateStoryFromEpicTodo(ctx context.Context, projectID string, epicKey string, todoID string) (*models.EpicMeta, *models.Task, string, error) {
	return d.CreateStoryFromMacroTodo(ctx, projectID, epicKey, todoID)
}

// SetTaskMacro queues the attachment of a ticket to a macro.
func (d *DB) SetTaskMacro(ctx context.Context, taskIDOrKey string, macroKey string) (*models.Task, *models.TaskActivity, error) {
	task, err := d.GetTaskByID(taskIDOrKey)
	if err != nil || task == nil {
		return nil, nil, fmt.Errorf("tâche introuvable")
	}
	cleanMacroKey := strings.TrimSpace(macroKey)
	if err := d.writeTaskParentLocally(task, cleanMacroKey); err != nil {
		return nil, nil, err
	}
	act, err := d.EnqueueTrackerOp(ctx, TrackerOp{
		Kind:      TrackerOpSetParent,
		ProjectID: task.ProjectID,
		TaskID:    task.ID,
		TaskKey:   task.Key,
		EpicKey:   cleanMacroKey,
	})
	return task, act, err
}

func (d *DB) SetTaskEpic(ctx context.Context, taskIDOrKey string, epicKey string) (*models.Task, *models.TaskActivity, error) {
	return d.SetTaskMacro(ctx, taskIDOrKey, epicKey)
}

// applyTaskMacro performs the attachment of a task to a macro (milestone), as
// whoever the context names. A milestone write refused for want of their
// credential keeps the local attachment and is returned with the task, so the
// queued activity ends in failure rather than claiming GitHub was updated.
func (d *DB) applyTaskMacro(ctx context.Context, taskIDOrKey string, macroKey string, steps *[]string) (*models.Task, error) {
	task, err := d.GetTaskByID(taskIDOrKey)
	if err != nil || task == nil {
		return nil, fmt.Errorf("tâche introuvable")
	}
	cleanMacroKey := strings.TrimSpace(macroKey)
	if err := d.writeTaskParentLocally(task, cleanMacroKey); err != nil {
		return nil, err
	}

	proj, _ := d.GetProjectByID(task.ProjectID)
	if proj != nil && (proj.IssueTracker == "github" || task.Source == "github") {
		cleanKey := strings.TrimPrefix(task.Key, "#")
		var issueNum int
		_, _ = fmt.Sscanf(cleanKey, "%d", &issueNum)
		if issueNum > 0 {
			milestoneTarget := ""
			if cleanMacroKey != "" {
				var mTitle string
				d.mu.RLock()
				_ = d.conn.QueryRow("SELECT title FROM macros WHERE project_id = ? AND key = ?", task.ProjectID, cleanMacroKey).Scan(&mTitle)
				d.mu.RUnlock()
				if mTitle != "" {
					milestoneTarget = mTitle
				} else if strings.HasPrefix(strings.ToUpper(cleanMacroKey), "M-") {
					var num int
					_, _ = fmt.Sscanf(strings.ToUpper(cleanMacroKey), "M-%d", &num)
					milestoneTarget = fmt.Sprintf("%d", num)
				} else {
					milestoneTarget = cleanMacroKey
				}
			}
			client, refused := d.trackerForWrite(ctx, "github", proj.ID)
			if refused != nil {
				*steps = append(*steps, fmt.Sprintf("⚠️ Synchro milestone GitHub échouée pour %s: %v, gardé en local", task.Key, refused))
				return task, refused
			}
			if err := client.SetGithubIssueMilestone(proj.GithubRepo, proj.RepoPath, issueNum, milestoneTarget); err != nil {
				*steps = append(*steps, fmt.Sprintf("⚠️ Synchro milestone GitHub échouée pour %s: %v, gardé en local", task.Key, err))
			} else {
				if milestoneTarget == "" {
					*steps = append(*steps, fmt.Sprintf("✅ %s retiré du milestone GitHub", task.Key))
				} else {
					*steps = append(*steps, fmt.Sprintf("✅ %s rattaché au milestone GitHub « %s »", task.Key, milestoneTarget))
				}
			}
		}
	} else if task.Source == "gitlab" && proj != nil {
		if err := d.writeGitlabMacroLabels(ctx, proj, task, cleanMacroKey); err != nil {
			*steps = append(*steps, fmt.Sprintf("⚠️ Labels de macro GitLab non posés sur %s : %v, gardé en local", task.Key, err))
			if isTrackerWriteRefusal(err) {
				return task, err
			}
		} else if cleanMacroKey == "" {
			*steps = append(*steps, fmt.Sprintf("✅ %s retiré de sa macro sur GitLab", task.Key))
		} else {
			*steps = append(*steps, fmt.Sprintf("✅ %s rattaché à la macro %s sur GitLab", task.Key, cleanMacroKey))
		}
	} else {
		*steps = append(*steps, fmt.Sprintf("✅ %s macro mise à jour en local", task.Key))
	}

	return task, nil
}

// writeGitlabMacroLabels writes a task's macro on GitLab, where it is carried
// by labels on every tier: macro:<title> and parent:<key> replace the task's
// previous ones, and an empty key removes them.
func (d *DB) writeGitlabMacroLabels(ctx context.Context, proj *models.Project, task *models.Task, macroKey string) error {
	ts, err := d.TrackerForProject(proj)
	if err != nil {
		return err
	}
	var add []string
	if macroKey != "" {
		title := ""
		d.mu.RLock()
		_ = d.conn.QueryRow("SELECT title FROM macros WHERE project_id = ? AND key = ?", proj.ID, macroKey).Scan(&title)
		d.mu.RUnlock()
		if strings.TrimSpace(title) == "" {
			title = macroKey
		}
		add = []string{"macro:" + title, "parent:" + macroKey}
	}
	var remove []string
	for _, l := range task.Labels {
		low := strings.ToLower(strings.TrimSpace(l))
		if (strings.HasPrefix(low, "macro:") || strings.HasPrefix(low, "parent:")) && !containsFold(add, l) {
			remove = append(remove, l)
		}
	}
	if len(add) == 0 && len(remove) == 0 {
		return nil
	}
	return ts.UpdateLabels(tracker.WithProject(ctx, proj.ID), task.Key, add, remove)
}

func containsFold(list []string, s string) bool {
	for _, item := range list {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(s)) {
			return true
		}
	}
	return false
}

func (d *DB) applyTaskEpic(ctx context.Context, taskIDOrKey string, epicKey string, steps *[]string) (*models.Task, error) {
	return d.applyTaskMacro(ctx, taskIDOrKey, epicKey, steps)
}

// writeTaskParentLocally mirrors the attachment in the local database.
func (d *DB) writeTaskParentLocally(task *models.Task, macroKey string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	// The title is copied by the statement itself, so a rename of the macro
	// committed on another instance a moment before is the title written.
	if macroKey == "" {
		_, err := d.conn.Exec("UPDATE tasks SET parent_key = '', parent_title = '', parent_type = 'macro', updated_at = ? WHERE id = ?", time.Now(), task.ID)
		return err
	}
	_, err := d.conn.Exec(`UPDATE tasks SET parent_key = ?, parent_type = 'macro', updated_at = ?,
		parent_title = COALESCE(NULLIF((SELECT title FROM macros WHERE project_id = ? AND key = ?), ''), ?)
		WHERE id = ?`, macroKey, time.Now(), task.ProjectID, macroKey, macroKey, task.ID)
	return err
}

// CreateStoryUnderMacro creates a story in the macro's own project, attached
// under the macro, as whoever the context names. The notice says what could not
// be written on the tracker.
func (d *DB) CreateStoryUnderMacro(ctx context.Context, projectID string, macroKey string, title string) (*models.Task, string, error) {
	proj, err := d.GetProjectByID(projectID)
	if err != nil || proj == nil {
		return nil, "", fmt.Errorf("projet non trouvé")
	}
	return d.createStoryUnder(ctx, proj, proj, macroKey, title)
}

// createStoryUnder creates a story in target, attached under a macro of
// macroProject, the caller having checked that the two share a tracker
// instance. The parent is written on the tracker where the tracker has one: a
// GitHub milestone, a Jira epic. A parent the tracker refuses does not undo the
// story, which exists by then: the notice says it was kept locally only.
func (d *DB) createStoryUnder(ctx context.Context, macroProject, target *models.Project, macroKey string, title string) (*models.Task, string, error) {
	parentTitle := ""
	d.mu.RLock()
	_ = d.conn.QueryRow("SELECT title FROM macros WHERE project_id = ? AND key = ?", macroProject.ID, macroKey).Scan(&parentTitle)
	d.mu.RUnlock()
	if parentTitle == "" {
		parentTitle = macroKey
	}

	task, err := d.CreateTaskAs(ctx, models.CreateTaskRequest{
		ProjectID: target.ID,
		Title:     title,
		Priority:  models.PriorityMedium,
	})
	if err != nil {
		return nil, "", err
	}

	_ = d.writeTaskParentLocally(task, macroKey)
	task.ParentKey = macroKey
	task.ParentTitle = parentTitle
	task.ParentType = "macro"

	notice := ""
	switch {
	case target.IssueTracker == "github" || task.Source == "github":
		var issueNum int
		_, _ = fmt.Sscanf(strings.TrimPrefix(task.Key, "#"), "%d", &issueNum)
		if issueNum > 0 {
			client, err := d.trackerForWrite(ctx, "github", target.ID)
			if err == nil {
				err = client.SetGithubIssueMilestone(target.GithubRepo, target.RepoPath, issueNum, parentTitle)
			}
			if err != nil {
				notice = fmt.Sprintf("milestone %s non posé sur GitHub : %v ; rattachement gardé en local", macroKey, err)
			}
		}
	case task.Source == "gitlab":
		if err := d.writeGitlabMacroLabels(ctx, target, task, macroKey); err != nil {
			notice = fmt.Sprintf("labels de la macro %s non posés sur GitLab : %v ; rattachement gardé en local", macroKey, err)
		}
	case task.Source == "jira":
		// The epic parents the story on Jira itself, not only on the board.
		if ts, tsErr := d.TrackerForProject(target); tsErr != nil {
			notice = fmt.Sprintf("épic %s non posé comme parent sur Jira : %v ; rattachement gardé en local", macroKey, tsErr)
		} else if setErr := ts.SetParent(tracker.WithProject(ctx, target.ID), task.Key, macroKey); setErr != nil {
			notice = fmt.Sprintf("épic %s non posé comme parent sur Jira : %v ; rattachement gardé en local", macroKey, setErr)
		}
	}
	return task, notice, nil
}

func (d *DB) CreateStoryUnderEpic(ctx context.Context, projectID string, epicKey string, title string) (*models.Task, string, error) {
	return d.CreateStoryUnderMacro(ctx, projectID, epicKey, title)
}

// CreateMacro creates the macro in the tracker (e.g. GitHub milestone) and records it locally.
//
// The milestone is created as whoever the context names. When the tracker
// client refuses it for want of their credential, the macro is recorded locally
// as on any failed creation, and the refusal is returned with it (#482).
func (d *DB) CreateMacro(ctx context.Context, projectID string, title string, horizon string, fields map[string]string) (*models.MacroMeta, error) {
	projectID = strings.TrimSpace(projectID)
	title = strings.TrimSpace(title)
	if projectID == "" || title == "" {
		return nil, fmt.Errorf("projet et titre de la macro obligatoires")
	}

	proj, err := d.GetProjectByID(projectID)
	if err != nil || proj == nil {
		return nil, fmt.Errorf("projet non trouvé")
	}

	key := ""
	var refused error
	if githubMilestoneMacros(proj) && proj.GithubRepo != "" {
		if client, err := d.trackerForWrite(ctx, "github", proj.ID); err != nil {
			refused = err
		} else if num, err := client.CreateGithubMilestone(proj.GithubRepo, proj.RepoPath, title, ""); err == nil && num > 0 {
			key = fmt.Sprintf("M-%d", num)
		}
	}

	if key == "" {
		var maxM int
		row := d.conn.QueryRow("SELECT COALESCE(MAX(CAST(SUBSTR(key, 3) AS INTEGER)), 0) FROM macros WHERE project_id = ? AND key LIKE 'M-%'", projectID)
		_ = row.Scan(&maxM)
		if maxM > 0 {
			key = fmt.Sprintf("M-%d", maxM+1)
		} else {
			key = fmt.Sprintf("M-%d", (time.Now().Unix()%90000)+10000)
		}
	}

	status := "open"
	closed := false
	h := horizon
	if h == "" {
		h = HorizonNow
	}
	created, err := d.saveMacroMetaFull(projectID, key, &h, nil, nil, nil, &title, &status, &closed)
	if err == nil && refused != nil {
		return created, fmt.Errorf("milestone GitHub non créé, macro %s gardée en local : %w", key, refused)
	}
	return created, err
}

func (d *DB) CreateEpic(ctx context.Context, projectID string, title string, horizon string, fields map[string]string) (*models.EpicMeta, error) {
	return d.CreateMacro(ctx, projectID, title, horizon, fields)
}

// DeleteMacro deletes a macro (and its GitHub milestone if applicable) and detaches its child tasks.
//
// The milestone is deleted as whoever the context names. A deletion the tracker
// client refuses for want of their credential still deletes the macro locally,
// as any failed milestone deletion does, and the refusal is returned (#482).
func (d *DB) DeleteMacro(ctx context.Context, projectID string, key string) error {
	projectID = strings.TrimSpace(projectID)
	key = strings.TrimSpace(key)
	if projectID == "" || key == "" {
		return fmt.Errorf("projet et clé de macro obligatoires")
	}

	var refused error
	proj, _ := d.GetProjectByID(projectID)
	if githubMilestoneMacros(proj) {
		var num int
		if strings.HasPrefix(strings.ToUpper(key), "M-") {
			_, _ = fmt.Sscanf(strings.ToUpper(key), "M-%d", &num)
		}
		if num > 0 {
			if client, err := d.trackerForWrite(ctx, "github", proj.ID); err != nil {
				refused = err
			} else {
				_ = client.DeleteGithubMilestone(proj.GithubRepo, proj.RepoPath, num)
			}
		}
	}

	d.mu.Lock()
	d.ensureMacrosTable()
	_, err := d.conn.Exec("DELETE FROM macros WHERE project_id = ? AND key = ?", projectID, key)
	_, _ = d.conn.Exec("UPDATE tasks SET parent_key = '', parent_title = '' WHERE project_id = ? AND (parent_key = ? OR parent_title = ?)", projectID, key, key)
	d.mu.Unlock()
	if err == nil && refused != nil {
		return fmt.Errorf("macro %s supprimée en local, milestone GitHub non supprimé : %w", key, refused)
	}
	return err
}

func (d *DB) DeleteEpic(ctx context.Context, projectID string, key string) error {
	return d.DeleteMacro(ctx, projectID, key)
}

// MoveTasksToMacro queues moving a batch of tickets to a macro.
func (d *DB) MoveTasksToMacro(ctx context.Context, projectID string, taskIDs []string, targetMacroKey string, newMacroTitle string, fields map[string]string) (*models.TaskActivity, error) {
	if len(taskIDs) == 0 {
		return nil, fmt.Errorf("aucun ticket sélectionné")
	}

	targetMacroKey = strings.ToUpper(strings.TrimSpace(targetMacroKey))
	if targetMacroKey == "" && strings.TrimSpace(newMacroTitle) == "" {
		return nil, fmt.Errorf("macro cible ou intitulé de la nouvelle macro obligatoire")
	}

	if targetMacroKey != "" {
		for _, id := range taskIDs {
			if task, err := d.GetTaskByID(id); err == nil && task != nil {
				_ = d.writeTaskParentLocally(task, targetMacroKey)
			}
		}
	}

	return d.EnqueueTrackerOp(ctx, TrackerOp{
		Kind:         TrackerOpMoveToEpic,
		ProjectID:    projectID,
		TaskIDs:      taskIDs,
		EpicKey:      targetMacroKey,
		NewEpicTitle: strings.TrimSpace(newMacroTitle),
		Fields:       fields,
	})
}

func (d *DB) MoveTasksToEpic(ctx context.Context, projectID string, taskIDs []string, targetEpicKey string, newEpicTitle string, fields map[string]string) (*models.TaskActivity, error) {
	return d.MoveTasksToMacro(ctx, projectID, taskIDs, targetEpicKey, newEpicTitle, fields)
}

// appendActivityStep adds a line to an activity's step list.
func (d *DB) appendActivityStep(activityID string, step string) {
	if strings.TrimSpace(activityID) == "" || strings.TrimSpace(step) == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	_ = d.conn.WithTx(func(tx *sqlTx) error {
		steps, err := d.lockActivityStepsUnsafe(tx, activityID)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(append(steps, step))
		if err != nil {
			return err
		}
		_, err = tx.Exec("UPDATE task_activities SET steps = ? WHERE id = ?", string(payload), activityID)
		return err
	})
}

// lockActivityStepsUnsafe reads an activity's steps on its locked row, so a step
// appended at the same time by another server instance is kept rather than
// overwritten by a list read before it. A list that does not parse reads as
// empty, as it always has.
func (d *DB) lockActivityStepsUnsafe(tx *sqlTx, activityID string) ([]string, error) {
	var raw string
	if err := tx.QueryRow("SELECT steps FROM task_activities WHERE id = ?"+d.forUpdate(), activityID).Scan(&raw); err != nil {
		return nil, err
	}
	steps := []string{}
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &steps)
	}
	return steps, nil
}

// IsProjectCompatible checks whether two projects can share tasks and macros.
func IsProjectCompatible(p1, p2 *models.Project) bool {
	if p1 == nil || p2 == nil {
		return false
	}
	t1 := strings.ToLower(strings.TrimSpace(p1.IssueTracker))
	t2 := strings.ToLower(strings.TrimSpace(p2.IssueTracker))
	if t1 == "" {
		t1 = "local"
	}
	if t2 == "" {
		t2 = "local"
	}
	if t1 == t2 || t1 == "local" || t2 == "local" {
		return true
	}
	if githubMilestoneMacros(p1) && githubMilestoneMacros(p2) {
		return true
	}
	return false
}

// MigrateMacro moves a macro and optionally its attached tasks from one project to another compatible project.
// MigrateMacro moves a macro, and optionally its tasks, to another compatible
// project. Its GitHub writes are made as whoever the context names; a refusal
// for want of their credential stops the migration before anything moves.
func (d *DB) MigrateMacro(ctx context.Context, sourceProjectID string, macroKey string, targetProjectID string, migrateTasks bool) (*models.MacroMeta, int, error) {
	sourceProjectID = strings.TrimSpace(sourceProjectID)
	macroKey = strings.TrimSpace(macroKey)
	targetProjectID = strings.TrimSpace(targetProjectID)

	if sourceProjectID == "" || macroKey == "" || targetProjectID == "" {
		return nil, 0, fmt.Errorf("projet source, clé de macro et projet cible obligatoires")
	}
	if sourceProjectID == targetProjectID {
		return nil, 0, fmt.Errorf("le projet cible doit être différent du projet source")
	}

	sourceProj, err := d.GetProjectByID(sourceProjectID)
	if err != nil || sourceProj == nil {
		return nil, 0, fmt.Errorf("projet source non trouvé")
	}
	targetProj, err := d.GetProjectByID(targetProjectID)
	if err != nil || targetProj == nil {
		return nil, 0, fmt.Errorf("projet cible non trouvé")
	}

	if !IsProjectCompatible(sourceProj, targetProj) {
		return nil, 0, fmt.Errorf("les projets %s et %s ne sont pas compatibles (trackers différents)", sourceProj.Name, targetProj.Name)
	}

	// The credentials are resolved before anything moves: a migration half
	// done because the person has no GitHub token would leave the macro in one
	// project and its milestone in none.
	targetIsGithub := githubMilestoneMacros(targetProj)
	var targetWriter, sourceWriter *trackerapi.Client
	if targetIsGithub {
		if targetWriter, err = d.trackerForWrite(ctx, "github", targetProj.ID); err != nil {
			return nil, 0, err
		}
	}
	if migrateTasks && githubTransferBetween(sourceProj, targetProj) {
		if sourceWriter, err = d.trackerForWrite(ctx, "github", sourceProj.ID); err != nil {
			return nil, 0, err
		}
	}

	d.mu.Lock()
	d.ensureMacrosTable()
	d.mu.Unlock()

	// 1. Read existing macro data from source project
	var horizon, description, todosJSON, title, status string
	var closed int
	d.mu.RLock()
	err = d.conn.QueryRow(`
		SELECT horizon, description, todos, title, status, closed
		FROM macros
		WHERE project_id = ? AND key = ?
	`, sourceProjectID, macroKey).Scan(&horizon, &description, &todosJSON, &title, &status, &closed)
	d.mu.RUnlock()

	if err != nil {
		if err == sql.ErrNoRows {
			title = macroKey
		} else {
			return nil, 0, err
		}
	}
	if title == "" {
		title = macroKey
	}

	targetMacroKey := macroKey

	// 2. Handle GitHub milestones migration if target is GitHub
	if targetIsGithub {
		targetMilestones, _ := d.tracker(targetProj.ID).ListGithubMilestones(targetProj.GithubRepo, targetProj.RepoPath)
		var existingNum int
		for _, m := range targetMilestones {
			if strings.EqualFold(strings.TrimSpace(m.Title), strings.TrimSpace(title)) {
				existingNum = m.Number
				break
			}
		}
		if existingNum > 0 {
			targetMacroKey = fmt.Sprintf("M-%d", existingNum)
		} else {
			newNum, createErr := targetWriter.CreateGithubMilestone(targetProj.GithubRepo, targetProj.RepoPath, title, description)
			if createErr == nil && newNum > 0 {
				targetMacroKey = fmt.Sprintf("M-%d", newNum)
			}
		}
	}

	// 3. Insert or update macro in target project, then delete it from the
	// source, in one transaction. The source row is locked and read again
	// first: the milestone calls above can take seconds, and an edit committed
	// meanwhile, on this instance or another, is what gets copied.
	d.mu.Lock()
	err = d.conn.WithTx(func(tx *sqlTx) error {
		var freshTitle string
		readErr := tx.QueryRow(`
			SELECT horizon, description, todos, title, status, closed
			FROM macros
			WHERE project_id = ? AND key = ?`+d.forUpdate(), sourceProjectID, macroKey).Scan(&horizon, &description, &todosJSON, &freshTitle, &status, &closed)
		if readErr != nil && readErr != sql.ErrNoRows {
			return readErr
		}
		if readErr == nil && freshTitle != "" {
			title = freshTitle
		}
		if _, err := tx.Exec(`
			INSERT INTO macros (project_id, key, horizon, description, todos, title, status, closed, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(project_id, key) DO UPDATE SET
				horizon = excluded.horizon,
				description = excluded.description,
				todos = excluded.todos,
				title = excluded.title,
				status = excluded.status,
				closed = excluded.closed,
				updated_at = CURRENT_TIMESTAMP
		`, targetProjectID, targetMacroKey, horizon, description, todosJSON, title, status, closed); err != nil {
			return err
		}
		// Delete from source project
		_, err := tx.Exec("DELETE FROM macros WHERE project_id = ? AND key = ?", sourceProjectID, macroKey)
		return err
	})
	d.mu.Unlock()

	if err != nil {
		return nil, 0, fmt.Errorf("erreur enregistrement macro cible: %w", err)
	}

	// 4. Migrate tasks if requested
	migratedTasksCount := 0
	if migrateTasks {
		d.mu.RLock()
		rows, err := d.conn.Query(`
			SELECT id, key, title, source, external_url
			FROM tasks
			WHERE project_id = ? AND (parent_key = ? OR parent_title = ?)
		`, sourceProjectID, macroKey, title)
		d.mu.RUnlock()

		if err == nil {
			type taskToMigrate struct {
				id          string
				key         string
				title       string
				source      sql.NullString
				externalUrl sql.NullString
			}
			var tasksList []taskToMigrate
			for rows.Next() {
				var t taskToMigrate
				if scanErr := rows.Scan(&t.id, &t.key, &t.title, &t.source, &t.externalUrl); scanErr == nil {
					tasksList = append(tasksList, t)
				}
			}
			rows.Close()

			for _, t := range tasksList {
				newID := t.id
				newKey := t.key
				newExternalUrl := t.externalUrl.String

				if sourceWriter != nil {
					var issueNum int
					_, _ = fmt.Sscanf(strings.TrimPrefix(t.key, "#"), "%d", &issueNum)
					if issueNum > 0 {
						num, u, transferErr := sourceWriter.TransferGithubIssue(sourceProj.GithubRepo, sourceProj.RepoPath, issueNum, targetProj.GithubRepo, targetProj.RepoPath)
						if transferErr == nil && num > 0 {
							newKey = fmt.Sprintf("#%d", num)
							newID = fmt.Sprintf("gh-%s-%d", targetProjectID, num)
							if u != "" {
								newExternalUrl = u
							}
							_ = targetWriter.SetGithubIssueMilestone(targetProj.GithubRepo, targetProj.RepoPath, num, title)
						}
					}
				}

				d.mu.Lock()
				_, updateErr := d.conn.Exec(`
					UPDATE tasks
					SET id = ?, key = ?, project_id = ?, parent_key = ?, parent_title = ?, parent_type = 'macro', external_url = ?, updated_at = CURRENT_TIMESTAMP
					WHERE id = ?
				`, newID, newKey, targetProjectID, targetMacroKey, title, newExternalUrl, t.id)
				d.mu.Unlock()

				if updateErr == nil {
					migratedTasksCount++
				}
			}
		}
	}

	var todos []models.MacroTodo
	if todosJSON != "" {
		_ = json.Unmarshal([]byte(todosJSON), &todos)
	}
	res := &models.MacroMeta{
		ProjectID:   targetProjectID,
		Key:         targetMacroKey,
		Title:       title,
		Horizon:     horizon,
		Description: description,
		Todos:       todos,
		Status:      status,
		Closed:      closed == 1,
	}

	return res, migratedTasksCount, nil
}

func (d *DB) MigrateEpic(ctx context.Context, sourceProjectID string, epicKey string, targetProjectID string, migrateTasks bool) (*models.EpicMeta, int, error) {
	return d.MigrateMacro(ctx, sourceProjectID, epicKey, targetProjectID, migrateTasks)
}

// githubMilestoneMacros reports whether a project's macros are GitHub
// milestones: a GitHub project, or one that names a repository without
// choosing another tracker. A GitLab project never is, even with a repository
// left from an earlier configuration: its macros live in labels (#398).
func githubMilestoneMacros(proj *models.Project) bool {
	return proj != nil && !strings.EqualFold(proj.IssueTracker, "gitlab") &&
		(proj.IssueTracker == "github" || proj.GithubRepo != "")
}

// githubTransferBetween reports whether moving a task between the two projects
// transfers its GitHub issue from one repository to the other.
func githubTransferBetween(source, target *models.Project) bool {
	return source != nil && target != nil &&
		githubMilestoneMacros(source) && githubMilestoneMacros(target) &&
		source.GithubRepo != "" && target.GithubRepo != "" &&
		source.GithubRepo != target.GithubRepo
}

// MigrateTasks moves a slice of tasks from their current project to another compatible project.
//
// An issue transfer is made as whoever the context names. A refusal for want of
// their credential stops the migration before that task moves, and returns how
// many had moved by then.
func (d *DB) MigrateTasks(ctx context.Context, taskIDs []string, targetProjectID string) (int, error) {
	targetProjectID = strings.TrimSpace(targetProjectID)
	if targetProjectID == "" {
		return 0, fmt.Errorf("projet cible obligatoire")
	}
	targetProj, err := d.GetProjectByID(targetProjectID)
	if err != nil || targetProj == nil {
		return 0, fmt.Errorf("projet cible non trouvé")
	}

	migratedCount := 0
	for _, taskID := range taskIDs {
		taskID = strings.TrimSpace(taskID)
		if taskID == "" {
			continue
		}
		task, err := d.GetTaskByID(taskID)
		if err != nil || task == nil {
			continue
		}
		if task.ProjectID == targetProjectID {
			continue
		}

		sourceProj, _ := d.GetProjectByID(task.ProjectID)
		if sourceProj != nil && !IsProjectCompatible(sourceProj, targetProj) {
			continue
		}

		newID := task.ID
		newKey := task.Key
		newExternalUrl := ""
		if task.ExternalURL != nil {
			newExternalUrl = *task.ExternalURL
		}
		newParentKey := task.ParentKey
		newParentTitle := task.ParentTitle

		if githubTransferBetween(sourceProj, targetProj) {
			var issueNum int
			_, _ = fmt.Sscanf(strings.TrimPrefix(task.Key, "#"), "%d", &issueNum)
			if issueNum > 0 {
				sourceWriter, err := d.trackerForWrite(ctx, "github", sourceProj.ID)
				if err != nil {
					return migratedCount, err
				}
				targetWriter, err := d.trackerForWrite(ctx, "github", targetProj.ID)
				if err != nil {
					return migratedCount, err
				}
				num, u, transferErr := sourceWriter.TransferGithubIssue(sourceProj.GithubRepo, sourceProj.RepoPath, issueNum, targetProj.GithubRepo, targetProj.RepoPath)
				if transferErr == nil && num > 0 {
					newKey = fmt.Sprintf("#%d", num)
					newID = fmt.Sprintf("gh-%s-%d", targetProjectID, num)
					if u != "" {
						newExternalUrl = u
					}
					// If task had a milestone, check if milestone exists in target project
					if newParentTitle != "" {
						targetMilestones, _ := d.tracker(targetProj.ID).ListGithubMilestones(targetProj.GithubRepo, targetProj.RepoPath)
						for _, m := range targetMilestones {
							if strings.EqualFold(strings.TrimSpace(m.Title), strings.TrimSpace(newParentTitle)) {
								newParentKey = fmt.Sprintf("M-%d", m.Number)
								_ = targetWriter.SetGithubIssueMilestone(targetProj.GithubRepo, targetProj.RepoPath, num, newParentTitle)
								break
							}
						}
					}
				}
			}
		}

		d.mu.Lock()
		_, updateErr := d.conn.Exec(`
			UPDATE tasks
			SET id = ?, key = ?, project_id = ?, parent_key = ?, parent_title = ?, external_url = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, newID, newKey, targetProjectID, newParentKey, newParentTitle, newExternalUrl, task.ID)
		d.mu.Unlock()

		if updateErr == nil {
			migratedCount++
		}
	}

	return migratedCount, nil
}

// RefineMacro processes a macro's framing text (description) and generates structured MacroTodo items
// and proposed Sectile tasks formatted according to the project's selected SSD framework (SpecKit vs OpenSpec).
func (d *DB) RefineMacro(projectID string, key string) ([]models.MacroTodo, []models.ProposedMacroTask, string, error) {
	projectID = strings.TrimSpace(projectID)
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, nil, "", fmt.Errorf("clé de macro obligatoire")
	}

	d.mu.RLock()
	d.ensureMacrosTable()
	var macro models.MacroMeta
	var todosJSON string
	var closedInt int
	var err error
	if projectID != "" {
		err = d.conn.QueryRow(`
			SELECT project_id, key, horizon, description, todos, title, status, closed FROM macros WHERE project_id = ? AND key = ?
		`, projectID, key).Scan(&macro.ProjectID, &macro.Key, &macro.Horizon, &macro.Description, &todosJSON, &macro.Title, &macro.Status, &closedInt)
	} else {
		err = d.conn.QueryRow(`
			SELECT project_id, key, horizon, description, todos, title, status, closed FROM macros WHERE key = ?
		`, key).Scan(&macro.ProjectID, &macro.Key, &macro.Horizon, &macro.Description, &todosJSON, &macro.Title, &macro.Status, &closedInt)
	}
	d.mu.RUnlock()

	if err != nil {
		return nil, nil, "", fmt.Errorf("macro %s non trouvée", key)
	}

	if strings.TrimSpace(macro.Description) == "" {
		return nil, nil, "", fmt.Errorf("le texte de cadrage (description) de la macro %s est vide", key)
	}

	framework := "speckit"
	if macro.ProjectID != "" {
		proj, _ := d.GetProjectByID(macro.ProjectID)
		if proj != nil && strings.TrimSpace(proj.SpecFramework) != "" {
			framework = strings.ToLower(strings.TrimSpace(proj.SpecFramework))
		}
	}

	todos := GenerateMacroTodosFromFraming(macro.Title, macro.Description, framework)
	proposed := GenerateProposedMacroTasksFromFraming(macro.Title, macro.Description, framework)
	return todos, proposed, framework, nil
}

// GenerateMacroTodosFromFraming structures framing text into MacroTodo items based on the SSD framework.
func GenerateMacroTodosFromFraming(title, description, framework string) []models.MacroTodo {
	lines := strings.Split(description, "\n")
	var rawItems []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "---") {
			continue
		}
		// Strip common markdown list markers
		cleaned := trimmed
		cleaned = strings.TrimPrefix(cleaned, "- [ ] ")
		cleaned = strings.TrimPrefix(cleaned, "- [x] ")
		cleaned = strings.TrimPrefix(cleaned, "- ")
		cleaned = strings.TrimPrefix(cleaned, "* ")
		cleaned = strings.TrimPrefix(cleaned, "+ ")
		if idx := strings.Index(cleaned, ". "); idx > 0 && idx <= 3 {
			digitsOnly := true
			for _, r := range cleaned[:idx] {
				if r < '0' || r > '9' {
					digitsOnly = false
					break
				}
			}
			if digitsOnly {
				cleaned = strings.TrimSpace(cleaned[idx+2:])
			}
		}
		cleaned = strings.TrimSpace(cleaned)
		if cleaned != "" {
			rawItems = append(rawItems, cleaned)
		}
	}

	if len(rawItems) == 0 {
		if strings.TrimSpace(title) != "" {
			rawItems = append(rawItems, strings.TrimSpace(title))
		}
	}

	isOpenSpec := strings.EqualFold(framework, "openspec")

	out := make([]models.MacroTodo, 0, len(rawItems))
	capCount := 1
	changeCount := 1
	usCount := 1
	featCount := 1

	for _, item := range rawItems {
		var text string
		if isOpenSpec {
			if strings.HasPrefix(item, "[CAP") || strings.HasPrefix(item, "[CHANGE") || strings.HasPrefix(item, "[OPENSPEC") {
				text = item
			} else {
				if capCount <= changeCount {
					text = fmt.Sprintf("[CAP-%d] %s", capCount, item)
					capCount++
				} else {
					text = fmt.Sprintf("[CHANGE-%d] %s", changeCount, item)
					changeCount++
				}
			}
		} else {
			// SpecKit default
			if strings.HasPrefix(item, "[US") || strings.HasPrefix(item, "[FEAT") || strings.HasPrefix(item, "[SPEC") {
				text = item
			} else {
				if usCount <= featCount {
					text = fmt.Sprintf("[US-%d] %s", usCount, item)
					usCount++
				} else {
					text = fmt.Sprintf("[FEAT-%d] %s", featCount, item)
					featCount++
				}
			}
		}

		out = append(out, models.MacroTodo{
			ID:   uuid.New().String(),
			Text: text,
			Done: false,
		})
	}

	return out
}

// GenerateProposedMacroTasksFromFraming extracts proposed Sectile tasks from macro framing items.
func GenerateProposedMacroTasksFromFraming(title, description, framework string) []models.ProposedMacroTask {
	todos := GenerateMacroTodosFromFraming(title, description, framework)
	out := make([]models.ProposedMacroTask, 0, len(todos))
	for _, todo := range todos {
		t := strings.TrimSpace(todo.Text)
		if t == "" {
			continue
		}
		issueType := "Story"
		lower := strings.ToLower(t)
		if strings.Contains(lower, "bug") || strings.Contains(lower, "fix") || strings.Contains(lower, "erreur") {
			issueType = "Bug"
		} else if strings.Contains(lower, "feat") || strings.Contains(lower, "task") || strings.Contains(lower, "change") || strings.Contains(lower, "tâche") {
			issueType = "Task"
		}
		out = append(out, models.ProposedMacroTask{
			Title:       t,
			IssueType:   issueType,
			Description: fmt.Sprintf("Tâche issue du cadrage de la macro %s.\n\nDescription initiale : %s", title, t),
		})
	}
	return out
}
