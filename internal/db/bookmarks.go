package db

import (
	"database/sql"
	"fmt"
	"strings"
)

// EnsureDefaultBookmark seeds the deployment default project bookmark for the user
// if the user currently has zero project bookmarks.
func (d *DB) EnsureDefaultBookmark(userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	return d.ensureDefaultBookmarkUnsafe(userID)
}

func (d *DB) ensureDefaultBookmarkUnsafe(userID string) error {
	var count int
	err := d.conn.QueryRow("SELECT COUNT(*) FROM user_project_bookmarks WHERE user_id = ?", userID).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	var defaultProjectID string
	err = d.conn.QueryRow("SELECT id FROM projects WHERE is_default = 1 LIMIT 1").Scan(&defaultProjectID)
	if err == sql.ErrNoRows || defaultProjectID == "" {
		err = d.conn.QueryRow("SELECT id FROM projects ORDER BY created_at ASC LIMIT 1").Scan(&defaultProjectID)
	}
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}

	_, err = d.conn.Exec("INSERT OR IGNORE INTO user_project_bookmarks (user_id, project_id) VALUES (?, ?)", userID, defaultProjectID)
	return err
}

// GetUserProjectBookmarks returns the project IDs bookmarked by the user.
// It ensures that a user with zero bookmarks has the default project seeded first.
func (d *DB) GetUserProjectBookmarks(userID string) ([]string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return []string{}, nil
	}

	_ = d.EnsureDefaultBookmark(userID)

	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.getUserProjectBookmarksUnsafe(userID)
}

func (d *DB) getUserProjectBookmarksUnsafe(userID string) ([]string, error) {
	rows, err := d.conn.Query("SELECT project_id FROM user_project_bookmarks WHERE user_id = ? ORDER BY created_at ASC", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projectIDs []string
	for rows.Next() {
		var pid string
		if err := rows.Scan(&pid); err != nil {
			return nil, err
		}
		projectIDs = append(projectIDs, pid)
	}
	if projectIDs == nil {
		projectIDs = []string{}
	}
	return projectIDs, nil
}

func (d *DB) getUserProjectBookmarksMapUnsafe(userID string) (map[string]bool, error) {
	ids, err := d.getUserProjectBookmarksUnsafe(userID)
	if err != nil {
		return nil, err
	}
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m, nil
}

// BookmarkProject adds a project to the user's bookmarks.
func (d *DB) BookmarkProject(userID, projectID string) error {
	userID = strings.TrimSpace(userID)
	projectID = strings.TrimSpace(projectID)
	if userID == "" || projectID == "" {
		return fmt.Errorf("user ID and project ID are required")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	var actualID string
	err := d.conn.QueryRow("SELECT id FROM projects WHERE id = ? OR slug = ? LIMIT 1", projectID, projectID).Scan(&actualID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("projet non trouvé")
		}
		return err
	}

	_, err = d.conn.Exec("INSERT OR IGNORE INTO user_project_bookmarks (user_id, project_id) VALUES (?, ?)", userID, actualID)
	return err
}

// UnbookmarkProject removes a project from the user's bookmarks.
func (d *DB) UnbookmarkProject(userID, projectID string) error {
	userID = strings.TrimSpace(userID)
	projectID = strings.TrimSpace(projectID)
	if userID == "" || projectID == "" {
		return fmt.Errorf("user ID and project ID are required")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.conn.Exec("DELETE FROM user_project_bookmarks WHERE user_id = ? AND (project_id = ? OR project_id = (SELECT id FROM projects WHERE slug = ?))", userID, projectID, projectID)
	return err
}

// ToggleProjectBookmark toggles a project bookmark. Returns whether the project is now bookmarked.
func (d *DB) ToggleProjectBookmark(userID, projectID string) (bool, error) {
	userID = strings.TrimSpace(userID)
	projectID = strings.TrimSpace(projectID)
	if userID == "" || projectID == "" {
		return false, fmt.Errorf("user ID and project ID are required")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	var actualID string
	err := d.conn.QueryRow("SELECT id FROM projects WHERE id = ? OR slug = ? LIMIT 1", projectID, projectID).Scan(&actualID)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, fmt.Errorf("projet non trouvé")
		}
		return false, err
	}

	var count int
	err = d.conn.QueryRow("SELECT COUNT(*) FROM user_project_bookmarks WHERE user_id = ? AND project_id = ?", userID, actualID).Scan(&count)
	if err != nil {
		return false, err
	}

	if count > 0 {
		_, err = d.conn.Exec("DELETE FROM user_project_bookmarks WHERE user_id = ? AND project_id = ?", userID, actualID)
		return false, err
	}

	_, err = d.conn.Exec("INSERT OR IGNORE INTO user_project_bookmarks (user_id, project_id) VALUES (?, ?)", userID, actualID)
	return true, err
}
