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
	requested := PersonalSettings(s)
	// The editor and the terminal are the workstation's (#305): an empty value
	// keeps the stored one in the statement below, which is only read to seed
	// each workstation once.
	requested.EditorCommand, requested.ExternalTerminalCommand = "", ""
	merged := requested
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

	// The merge happens in the statement itself: a field the request leaves
	// empty keeps what the row holds when the statement runs, not what it held
	// when it was read above, so two saves of different fields on two server
	// instances both land. The values read above only seed a new row.
	keep := func(column string) string {
		return column + " = CASE WHEN ? = '' THEN user_settings." + column + " ELSE excluded." + column + " END"
	}
	d.mu.Lock()
	_, err = d.conn.Exec(`
		INSERT INTO user_settings (user_id, theme, accent_color, language, density, default_view, detail_mode,
		                           ui_scale, user_name, user_email, user_avatar, editor_command,
		                           external_terminal_command, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			`+keep("theme")+`,
			`+keep("accent_color")+`,
			`+keep("language")+`,
			`+keep("density")+`,
			`+keep("default_view")+`,
			`+keep("detail_mode")+`,
			ui_scale = CASE WHEN ? = 0 THEN user_settings.ui_scale ELSE excluded.ui_scale END,
			`+keep("user_name")+`,
			`+keep("user_email")+`,
			`+keep("user_avatar")+`,
			`+keep("editor_command")+`,
			`+keep("external_terminal_command")+`,
			updated_at = excluded.updated_at
	`, userID, merged.Theme, merged.AccentColor, merged.Language, merged.Density, merged.DefaultView,
		merged.DetailMode, merged.UIScale, merged.UserName, merged.UserEmail, merged.UserAvatar,
		merged.EditorCommand, merged.ExternalTerminalCommand, merged.UpdatedAt,
		requested.Theme, requested.AccentColor, requested.Language, requested.Density, requested.DefaultView,
		requested.DetailMode, requested.UIScale, requested.UserName, requested.UserEmail, requested.UserAvatar,
		requested.EditorCommand, requested.ExternalTerminalCommand)
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}
	// What the row holds now, which may carry a field another save wrote.
	return d.UserSettings(userID)
}
