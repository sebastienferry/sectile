package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"tasks/internal/models"
)

// A saved board view is personal: every lookup is scoped to its owner, and a
// view of somebody else answers exactly like a view that does not exist, so the
// answer reveals nothing about it.
var (
	ErrBoardViewNotFound       = errors.New("vue introuvable")
	ErrBoardViewNameRequired   = errors.New("le nom de la vue est obligatoire")
	ErrBoardViewNameTaken      = errors.New("une vue porte déjà ce nom")
	ErrBoardViewNoProject      = errors.New("une vue sélectionne au moins un projet")
	ErrBoardViewUnknownProject = errors.New("projet inconnu")
)

const boardViewColumns = "id, name, project_ids, labels, created_at, updated_at"

// NormalizeViewLabels trims the labels, drops the empty ones and keeps a single
// entry for labels that differ only by case, in the spelling first entered.
// Matching ignores case anyway; storing `Backend` and `backend` side by side
// would only show the same chip twice.
func NormalizeViewLabels(labels []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, label := range labels {
		label = strings.TrimSpace(label)
		key := strings.ToLower(label)
		if label == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, label)
	}
	return out
}

// ListBoardViews returns the user's views in the order they were created.
func (d *DB) ListBoardViews(userID string) ([]models.BoardView, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.conn.Query("SELECT "+boardViewColumns+" FROM board_views WHERE user_id = ? ORDER BY created_at ASC, id ASC", strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	views := []models.BoardView{}
	for rows.Next() {
		view, err := scanBoardView(rows)
		if err != nil {
			return nil, err
		}
		views = append(views, *view)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	existing, err := d.existingProjectIDsUnsafe()
	if err != nil {
		return nil, err
	}
	for i := range views {
		views[i].ProjectIDs = keepExistingProjects(views[i].ProjectIDs, existing)
	}
	return views, nil
}

// GetBoardView returns one of the user's views, or ErrBoardViewNotFound.
func (d *DB) GetBoardView(userID, id string) (*models.BoardView, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.getBoardViewUnsafe(userID, id)
}

func (d *DB) getBoardViewUnsafe(userID, id string) (*models.BoardView, error) {
	userID = strings.TrimSpace(userID)
	id = strings.TrimSpace(id)
	if userID == "" || id == "" {
		return nil, ErrBoardViewNotFound
	}
	view, err := scanBoardView(d.conn.QueryRow("SELECT "+boardViewColumns+" FROM board_views WHERE id = ? AND user_id = ?", id, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrBoardViewNotFound
	}
	if err != nil {
		return nil, err
	}
	// A project deleted since the view was saved is dropped on write by
	// DeleteProject; filtering on read as well means a cleanup that failed
	// half-way never shows a project that is gone.
	existing, err := d.existingProjectIDsUnsafe()
	if err != nil {
		return nil, err
	}
	view.ProjectIDs = keepExistingProjects(view.ProjectIDs, existing)
	return view, nil
}

// CreateBoardView saves a new view for the user.
func (d *DB) CreateBoardView(userID string, req models.BoardViewRequest) (*models.BoardView, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("userID is required")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	name := ""
	if req.Name != nil {
		name = *req.Name
	}
	var projectIDs, labels []string
	if req.ProjectIDs != nil {
		projectIDs = *req.ProjectIDs
	}
	if req.Labels != nil {
		labels = *req.Labels
	}

	view := &models.BoardView{ID: uuid.New().String()}
	if err := d.fillBoardViewUnsafe(userID, view, name, projectIDs, labels); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	view.CreatedAt, view.UpdatedAt = now, now

	projectsJSON, _ := json.Marshal(view.ProjectIDs)
	labelsJSON, _ := json.Marshal(view.Labels)
	if _, err := d.conn.Exec(`INSERT INTO board_views (id, user_id, name, name_key, project_ids, labels, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		view.ID, userID, view.Name, boardViewNameKey(view.Name), string(projectsJSON), string(labelsJSON), now, now); err != nil {
		return nil, err
	}
	return view, nil
}

// UpdateBoardView changes the fields the request carries and leaves the others.
func (d *DB) UpdateBoardView(userID, id string, req models.BoardViewRequest) (*models.BoardView, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	view, err := d.getBoardViewUnsafe(userID, id)
	if err != nil {
		return nil, err
	}
	name, projectIDs, labels := view.Name, view.ProjectIDs, view.Labels
	if req.Name != nil {
		name = *req.Name
	}
	if req.ProjectIDs != nil {
		projectIDs = *req.ProjectIDs
	}
	if req.Labels != nil {
		labels = *req.Labels
	}
	if err := d.fillBoardViewUnsafe(strings.TrimSpace(userID), view, name, projectIDs, labels); err != nil {
		return nil, err
	}
	view.UpdatedAt = time.Now().UTC()

	projectsJSON, _ := json.Marshal(view.ProjectIDs)
	labelsJSON, _ := json.Marshal(view.Labels)
	if _, err := d.conn.Exec(`UPDATE board_views SET name = ?, name_key = ?, project_ids = ?, labels = ?, updated_at = ?
		WHERE id = ? AND user_id = ?`,
		view.Name, boardViewNameKey(view.Name), string(projectsJSON), string(labelsJSON), view.UpdatedAt, view.ID, strings.TrimSpace(userID)); err != nil {
		return nil, err
	}
	return view, nil
}

// DeleteBoardView removes one of the user's views. Tickets, labels and projects
// are untouched: a view never owned any of them.
func (d *DB) DeleteBoardView(userID, id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	res, err := d.conn.Exec("DELETE FROM board_views WHERE id = ? AND user_id = ?", strings.TrimSpace(id), strings.TrimSpace(userID))
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrBoardViewNotFound
	}
	return nil
}

// fillBoardViewUnsafe validates and normalizes the definition into view.
func (d *DB) fillBoardViewUnsafe(userID string, view *models.BoardView, name string, projectIDs, labels []string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrBoardViewNameRequired
	}
	var clash string
	err := d.conn.QueryRow("SELECT id FROM board_views WHERE user_id = ? AND name_key = ? AND id != ?", userID, boardViewNameKey(name), view.ID).Scan(&clash)
	if err == nil {
		return ErrBoardViewNameTaken
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	resolved := []string{}
	seen := map[string]bool{}
	for _, projectID := range projectIDs {
		projectID = strings.TrimSpace(projectID)
		if projectID == "" {
			continue
		}
		var actualID string
		err := d.conn.QueryRow("SELECT id FROM projects WHERE id = ? OR slug = ? LIMIT 1", projectID, projectID).Scan(&actualID)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w : %s", ErrBoardViewUnknownProject, projectID)
		}
		if err != nil {
			return err
		}
		if !seen[actualID] {
			seen[actualID] = true
			resolved = append(resolved, actualID)
		}
	}
	if len(resolved) == 0 {
		return ErrBoardViewNoProject
	}

	view.Name = name
	view.ProjectIDs = resolved
	view.Labels = NormalizeViewLabels(labels)
	return nil
}

// removeProjectFromBoardViewsUnsafe drops a deleted project from every view
// that selected it. A view left with no project is kept: its owner decides
// whether to edit or delete it.
func (d *DB) removeProjectFromBoardViewsUnsafe(projectID, slug string) error {
	rows, err := d.conn.Query("SELECT id, project_ids FROM board_views WHERE project_ids LIKE ?", "%\""+projectID+"\"%")
	if err != nil {
		return err
	}
	type pending struct{ id, projects string }
	var updates []pending
	for rows.Next() {
		var id, raw string
		if err := rows.Scan(&id, &raw); err != nil {
			rows.Close()
			return err
		}
		var ids []string
		_ = json.Unmarshal([]byte(raw), &ids)
		kept := []string{}
		for _, candidate := range ids {
			if candidate != projectID && (slug == "" || candidate != slug) {
				kept = append(kept, candidate)
			}
		}
		if len(kept) != len(ids) {
			encoded, _ := json.Marshal(kept)
			updates = append(updates, pending{id: id, projects: string(encoded)})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, u := range updates {
		if _, err := d.conn.Exec("UPDATE board_views SET project_ids = ?, updated_at = ? WHERE id = ?", u.projects, time.Now().UTC(), u.id); err != nil {
			return err
		}
	}
	return nil
}

func (d *DB) existingProjectIDsUnsafe() (map[string]bool, error) {
	rows, err := d.conn.Query("SELECT id FROM projects")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	existing := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		existing[id] = true
	}
	return existing, rows.Err()
}

func keepExistingProjects(ids []string, existing map[string]bool) []string {
	kept := []string{}
	for _, id := range ids {
		if existing[id] {
			kept = append(kept, id)
		}
	}
	return kept
}

func boardViewNameKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanBoardView(row rowScanner) (*models.BoardView, error) {
	var view models.BoardView
	var projectsJSON, labelsJSON string
	if err := row.Scan(&view.ID, &view.Name, &projectsJSON, &labelsJSON, &view.CreatedAt, &view.UpdatedAt); err != nil {
		return nil, err
	}
	view.ProjectIDs = []string{}
	view.Labels = []string{}
	_ = json.Unmarshal([]byte(projectsJSON), &view.ProjectIDs)
	_ = json.Unmarshal([]byte(labelsJSON), &view.Labels)
	return &view, nil
}
