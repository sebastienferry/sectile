package agent

import (
	"context"
	"path/filepath"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
)

type settingsPathContextKey struct{}

func (d *agentDaemon) withSettingsPath(ctx context.Context) (context.Context, error) {
	path, err := d.settingsPath()
	if err != nil {
		return nil, err
	}
	return context.WithValue(ctx, settingsPathContextKey{}, path), nil
}

func settingsPathFromContext(ctx context.Context) string {
	path, _ := ctx.Value(settingsPathContextKey{}).(string)
	return path
}

func (d *agentDaemon) primaryRoot(ctx context.Context, config agentconfig.Config, overrides agentconfig.Settings, projectRoot string, task models.Task) (string, string, error) {
	ctx, err := d.withSettingsPath(ctx)
	if err != nil {
		return "", "", err
	}
	return primaryRoot(ctx, config, overrides, projectRoot, task)
}

func (d *agentDaemon) settingsPath() (string, error) {
	if d.settingsRoot != "" {
		return filepath.Join(d.settingsRoot, "settings.json"), nil
	}
	return agentconfig.SettingsPath()
}

func (d *agentDaemon) readSettings() (agentconfig.Settings, error) {
	path, err := d.settingsPath()
	if err != nil {
		return agentconfig.Settings{}, err
	}
	return agentconfig.ReadSettingsAt(path, d.localSettingsRoot())
}

func (d *agentDaemon) updateSettings(change func(*agentconfig.Settings) error) (agentconfig.Settings, error) {
	path, err := d.settingsPath()
	if err != nil {
		return agentconfig.Settings{}, err
	}
	return agentconfig.UpdateSettingsAt(path, d.localSettingsRoot(), change)
}

func (d *agentDaemon) writeSettings(settings agentconfig.Settings) error {
	path, err := d.settingsPath()
	if err != nil {
		return err
	}
	return agentconfig.WriteSettingsAt(path, settings)
}

func (d *agentDaemon) migrateSettings() (bool, agentconfig.SettingsMigration, error) {
	path, err := d.settingsPath()
	if err != nil {
		return false, agentconfig.SettingsMigration{}, err
	}
	return agentconfig.MigrateSettingsReportAt(path, d.localSettingsRoot())
}

func (d *agentDaemon) claudeSettingsFile(projectID string, sandbox *agentconfig.ClaudeSandbox) (string, error) {
	path, err := d.settingsPath()
	if err != nil {
		return "", err
	}
	return agentconfig.ClaudeSettingsFileAt(path, projectID, sandbox)
}

func (d *agentDaemon) claudeSettingsPath(projectID string) string {
	path, err := d.settingsPath()
	if err != nil {
		return ""
	}
	path, _ = agentconfig.ClaudeSettingsPathAt(path, projectID)
	return path
}
