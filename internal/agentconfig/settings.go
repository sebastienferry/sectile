package agentconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// SettingsPath is shared by the standalone agent and its optional companion.
func SettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "sectile", "settings.json"), nil
}

// ReadSettings reads the workstation settings, and falls back to the legacy
// repository file until they are saved. A file in the layout that predates
// #305 is folded into the current one with the same meaning; the next
// WriteSettings rewrites it.
func ReadSettings(legacyRoot string) (Settings, error) {
	path, err := SettingsPath()
	if err != nil {
		return Settings{}, err
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return readLegacyRepositoryFile(legacyRoot)
	}
	if err != nil {
		return Settings{}, err
	}
	var settings Settings
	if err = json.Unmarshal(raw, &settings); err != nil {
		return settings, err
	}
	// Legacy keys are folded whatever the layout: an agent that predates #305
	// keeps the keys it does not know when it saves, and adds its own beside
	// them. The current keys win.
	var legacy legacySettings
	if err = json.Unmarshal(raw, &legacy); err != nil {
		return settings, err
	}
	settings = overlay(legacy.fold(), settings)
	if settings.Layout >= SettingsLayout {
		return settings, nil
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil {
		return settings, err
	}
	checkout, err := readLegacyRepositoryFile(legacyRoot)
	if err != nil {
		return settings, err
	}
	_, hasProjects := fields["projects"]
	_, hasWorktrees := fields["worktrees"]
	_, hasParallelism := fields["parallelism"]
	for id, from := range checkout.ProjectSettings {
		p := settings.Project(id)
		if !hasProjects && p.Path == "" {
			p.Path = from.Path
		}
		if !hasWorktrees && p.UseWorktrees == nil {
			p.UseWorktrees = from.UseWorktrees
		}
		if !hasParallelism && p.Parallelism == 0 {
			p.Parallelism = from.Parallelism
		}
		settings.SetProject(id, p)
	}
	return settings, nil
}

// ownedKeys are the keys WriteSettings replaces as a whole. Any other key
// (server, deviceId, apiKey and what the desktop stores beside them) is kept.
var ownedKeys = []string{"layout", "defaults", "projectSettings", "repositories", "disconnectedProjects", "mcpConnections", "skills", "seeded"}

// WriteSettings preserves the connection fields while replacing the settings
// it owns. The legacy keys are removed and the file is written in the current
// layout. A map emptied by the caller is omitted from the JSON, and since the
// owned keys are replaced as a whole, it disappears from the file instead of
// keeping its previous content.
func WriteSettings(settings Settings) error {
	path, err := SettingsPath()
	if err != nil {
		return err
	}
	fields := map[string]json.RawMessage{}
	raw, err := os.ReadFile(path)
	if err == nil {
		if err = json.Unmarshal(raw, &fields); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	settings.Layout = SettingsLayout
	raw, err = json.Marshal(settings)
	if err != nil {
		return err
	}
	updates := map[string]json.RawMessage{}
	if err = json.Unmarshal(raw, &updates); err != nil {
		return err
	}
	for _, key := range append(append([]string{}, legacyKeys...), ownedKeys...) {
		delete(fields, key)
	}
	for key, value := range updates {
		fields[key] = value
	}
	raw, err = json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".settings-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(raw); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

// settingsMu serializes the read-modify-write cycles of this process on the
// settings file. A seed triggered by a project listing must not lose a save
// the desktop makes at the same moment, nor the other way round.
var settingsMu sync.Mutex

// UpdateSettings reads the settings, lets change edit them, and writes them
// back unless change fails. The whole cycle holds settingsMu.
func UpdateSettings(legacyRoot string, change func(*Settings) error) (Settings, error) {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	settings, err := ReadSettings(legacyRoot)
	if err != nil {
		return settings, err
	}
	if err := change(&settings); err != nil {
		return settings, err
	}
	return settings, WriteSettings(settings)
}

// LockSettings holds settingsMu for a read-modify-write cycle a caller spells
// out itself. It returns the unlock function.
func LockSettings() func() {
	settingsMu.Lock()
	return settingsMu.Unlock
}
