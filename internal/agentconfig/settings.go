package agentconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// SettingsPath is shared by the standalone agent and its optional companion.
func SettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "sectile", "settings.json"), nil
}

// ReadSettings falls back to the legacy repository file until settings are saved.
func ReadSettings(legacyRoot string) (Overrides, error) {
	path, err := SettingsPath()
	if err != nil {
		return Overrides{}, err
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ReadOverrides(legacyRoot)
	}
	if err != nil {
		return Overrides{}, err
	}
	var settings Overrides
	err = json.Unmarshal(raw, &settings)
	if err != nil {
		return settings, err
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil {
		return settings, err
	}
	legacy, err := ReadOverrides(legacyRoot)
	if err != nil {
		return settings, err
	}
	if _, ok := fields["projects"]; !ok {
		settings.Projects = legacy.Projects
	}
	if _, ok := fields["worktrees"]; !ok {
		settings.Worktrees = legacy.Worktrees
	}
	if _, ok := fields["parallelism"]; !ok {
		settings.Parallelism = legacy.Parallelism
	}
	return settings, nil
}

// WriteSettings preserves connection fields while updating local overrides.
func WriteSettings(settings Overrides) error {
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
	raw, err = json.Marshal(settings)
	if err != nil {
		return err
	}
	updates := map[string]json.RawMessage{}
	if err = json.Unmarshal(raw, &updates); err != nil {
		return err
	}
	for _, key := range []string{"disconnectedProjects", "projects", "worktrees", "parallelism", "commands", "commandsAutonomous", "aiProviders", "aiModels", "aiProvider", "aiCommandTemplate", "aiCommandTemplateAutonomous", "aiModel", "aiSkillModels", "terminal", "terminals", "skills"} {
		delete(fields, key)
	}
	// A map emptied by the caller is omitted from its JSON; writing it as null
	// is what removes the last entry instead of keeping the file's copy.
	for _, key := range []string{"projects", "worktrees", "parallelism", "commands", "commandsAutonomous", "aiProviders", "aiModels", "terminals", "specRepos", "repositories"} {
		if _, ok := updates[key]; !ok {
			updates[key] = json.RawMessage("null")
		}
	}
	// A workstation that never overrode the specification artefacts keeps a
	// file without the key, and clearing the last override removes it rather
	// than leaving a null behind.
	if _, ok := updates["specArtifacts"]; !ok {
		delete(fields, "specArtifacts")
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
