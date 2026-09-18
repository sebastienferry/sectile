package db

import (
	"database/sql"
	"strings"
	"time"

	"tasks/internal/models"
)

// The personal half of the settings, one row per account (ADR 0015). The
// deployment half — trackers, auto-sync, AI, prompts, repository path — stays
// in the single settings row and is an admin's to change.
//
// No migration runs: user_settings is created empty and an account without a
// row reads the deployment row's personal columns, so an existing deployment
// shows exactly what it showed before, per account, from the first render.

// PersonalSettings is the personal projection of a settings row: the fields a
// user owns, and the only ones user_settings stores.
func PersonalSettings(s models.Settings) models.Settings {
	return models.Settings{
		Theme:                   s.Theme,
		AccentColor:             s.AccentColor,
		Language:                s.Language,
		Density:                 s.Density,
		DefaultView:             s.DefaultView,
		DetailMode:              s.DetailMode,
		UIScale:                 s.UIScale,
		UserName:                s.UserName,
		UserEmail:               s.UserEmail,
		UserAvatar:              s.UserAvatar,
		EditorCommand:           s.EditorCommand,
		ExternalTerminalCommand: s.ExternalTerminalCommand,
	}
}

// UserSettings reads an account's personal settings, falling back to the
// deployment row when it has none yet.
func (d *DB) UserSettings(userID string) (*models.Settings, error) {
	userID = strings.TrimSpace(userID)
	deployment, err := d.GetSettings()
	if err != nil {
		return nil, err
	}
	seeded := PersonalSettings(*deployment)
	if userID == "" {
		return &seeded, nil
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	var s models.Settings
	var uiScale sql.NullInt64
	err = d.conn.QueryRow(`
		SELECT theme, accent_color, language, density, default_view, detail_mode, ui_scale,
		       user_name, user_email, user_avatar, editor_command, external_terminal_command, updated_at
		FROM user_settings WHERE user_id = ?
	`, userID).Scan(
		&s.Theme,
		&s.AccentColor,
		&s.Language,
		&s.Density,
		&s.DefaultView,
		&s.DetailMode,
		&uiScale,
		&s.UserName,
		&s.UserEmail,
		&s.UserAvatar,
		&s.EditorCommand,
		&s.ExternalTerminalCommand,
		&s.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return &seeded, nil
	}
	if err != nil {
		return nil, err
	}
	s.UIScale = NormalizeUIScale(int(uiScale.Int64))
	return &s, nil
}

// UpdateUserSettings upserts the personal columns of an account. An empty value
// keeps what is stored, exactly as UpdateSettings does for the deployment row,
// so a caller may send only the fields it edits.
func (d *DB) UpdateUserSettings(userID string, s models.Settings) (*models.Settings, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, ErrInvalidEmail
	}
	current, err := d.UserSettings(userID)
	if err != nil {
		return nil, err
	}
	merged := PersonalSettings(s)
	for _, field := range []struct{ value, fallback *string }{
		{&merged.Theme, &current.Theme},
		{&merged.AccentColor, &current.AccentColor},
		{&merged.Language, &current.Language},
		{&merged.Density, &current.Density},
		{&merged.DefaultView, &current.DefaultView},
		{&merged.DetailMode, &current.DetailMode},
		{&merged.UserName, &current.UserName},
		{&merged.UserEmail, &current.UserEmail},
		{&merged.UserAvatar, &current.UserAvatar},
		{&merged.EditorCommand, &current.EditorCommand},
		{&merged.ExternalTerminalCommand, &current.ExternalTerminalCommand},
	} {
		if *field.value == "" {
			*field.value = *field.fallback
		}
	}
	if merged.UIScale == 0 {
		merged.UIScale = current.UIScale
	}
	merged.UIScale = NormalizeUIScale(merged.UIScale)
	merged.UpdatedAt = time.Now()

	d.mu.Lock()
	defer d.mu.Unlock()
	_, err = d.conn.Exec(`
		INSERT INTO user_settings (user_id, theme, accent_color, language, density, default_view, detail_mode,
		                           ui_scale, user_name, user_email, user_avatar, editor_command,
		                           external_terminal_command, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			theme = excluded.theme,
			accent_color = excluded.accent_color,
			language = excluded.language,
			density = excluded.density,
			default_view = excluded.default_view,
			detail_mode = excluded.detail_mode,
			ui_scale = excluded.ui_scale,
			user_name = excluded.user_name,
			user_email = excluded.user_email,
			user_avatar = excluded.user_avatar,
			editor_command = excluded.editor_command,
			external_terminal_command = excluded.external_terminal_command,
			updated_at = excluded.updated_at
	`, userID, merged.Theme, merged.AccentColor, merged.Language, merged.Density, merged.DefaultView,
		merged.DetailMode, merged.UIScale, merged.UserName, merged.UserEmail, merged.UserAvatar,
		merged.EditorCommand, merged.ExternalTerminalCommand, merged.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &merged, nil
}
