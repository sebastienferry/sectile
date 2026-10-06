package agent

import (
	"errors"
	"fmt"
	"strings"

	"tasks/internal/agentconfig"
)

// projectClaudeSettings writes the settings file a Claude launch of the
// project is handed (#700), from its resolved Sandbox values as they are now
// (the workstation ones under the project's own, #730), and returns its path:
// "" when they state nothing. A failure fails the launch rather than starting
// Claude without the owner's deny rules.
func (d *agentDaemon) projectClaudeSettings(projectID string) (string, error) {
	if strings.TrimSpace(projectID) == "" {
		return "", nil
	}
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err == nil {
		var path string
		if path, err = agentconfig.ClaudeSettingsFile(projectID, settings.ResolvedClaudeSandbox(projectID)); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("the project's Claude settings could not be written: %w", err)
}

// launchClaudeSettings is projectClaudeSettings for a launch of config: an
// engine other than Claude Code receives nothing, so nothing is written for it.
func (d *agentDaemon) launchClaudeSettings(config agentconfig.Config) (string, error) {
	if liveProvider(config) != "claude" {
		return "", nil
	}
	return d.projectClaudeSettings(config.ProjectID)
}

// addProjectAllowRules adds the rules an "Always allow" answer approved to the
// project's allow rules, so they outlive the task's worktree (#700). It
// reports the rules that were new.
func (d *agentDaemon) addProjectAllowRules(projectID string, rules []string) ([]string, error) {
	if strings.TrimSpace(projectID) == "" || len(rules) == 0 {
		return nil, nil
	}
	var added []string
	_, err := agentconfig.UpdateSettings(d.localSettingsRoot(), func(settings *agentconfig.Settings) error {
		project := settings.Project(projectID)
		sandbox := agentconfig.ClaudeSandbox{}
		if project.ClaudeSandbox != nil {
			sandbox = *project.ClaudeSandbox
			sandbox.Allow = append([]string{}, sandbox.Allow...)
		}
		for _, rule := range rules {
			if sandbox.AddAllow(rule) {
				added = append(added, strings.TrimSpace(rule))
			}
		}
		if len(added) == 0 {
			return errNothingToSave
		}
		project.ClaudeSandbox = &sandbox
		settings.SetProject(projectID, project)
		return nil
	})
	if errors.Is(err, errNothingToSave) {
		return nil, nil
	}
	return added, err
}

// errNothingToSave stops an UpdateSettings cycle that would write the same
// file back.
var errNothingToSave = errors.New("nothing to save")

// claudeSandboxPayload is the project's sandbox values as the desktop reads
// them: every list present, empty when unset.
func claudeSandboxPayload(sandbox *agentconfig.ClaudeSandbox) map[string]any {
	value := agentconfig.ClaudeSandbox{}
	if sandbox != nil {
		value = *sandbox
	}
	list := func(entries []string) []string {
		if entries == nil {
			return []string{}
		}
		return entries
	}
	return map[string]any{
		"enabled":                  value.Enabled,
		"autoAllowBashIfSandboxed": value.AutoAllowBashIfSandboxed,
		"allowUnsandboxedCommands": value.AllowUnsandboxedCommands,
		"additionalDirectories":    list(value.AdditionalDirectories),
		"allowedDomains":           list(value.AllowedDomains),
		"allowWrite":               list(value.AllowWrite),
		"allow":                    list(value.Allow),
		"deny":                     list(value.Deny),
	}
}

// claudeSettingsPathOf is the path the desktop shows in the command preview,
// "" when the project id cannot name a file.
func claudeSettingsPathOf(projectID string) string {
	path, _ := agentconfig.ClaudeSettingsPath(projectID)
	return path
}

// addProjectDirectories keeps approved access across turns and worktrees.
func (d *agentDaemon) addProjectDirectories(projectID string, directories []string) error {
	if projectID == "" || len(directories) == 0 {
		return nil
	}
	_, err := agentconfig.UpdateSettings(d.localSettingsRoot(), func(settings *agentconfig.Settings) error {
		project := settings.Project(projectID)
		sandbox := agentconfig.ClaudeSandbox{}
		if project.ClaudeSandbox != nil {
			sandbox = *project.ClaudeSandbox
		}
		sandbox.AdditionalDirectories = append(append([]string{}, sandbox.AdditionalDirectories...), directories...)
		normalized, err := agentconfig.NormalizeClaudeSandbox(sandbox)
		if err != nil {
			return err
		}
		project.ClaudeSandbox = &normalized
		settings.SetProject(projectID, project)
		return nil
	})
	return err
}
