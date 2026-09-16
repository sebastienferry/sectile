package db

import (
	"fmt"
	"strings"

	"tasks/internal/models"
)

// headlessProviders is the set of provider CLIs whose headless invocation this
// repository attests: `claude -p`, `codex exec`, and `vibe -p`, which is already
// how vibe is launched today. Guessing a print flag for the others is worse than
// refusing: an unsupported flag is either rejected opaquely or swallowed as
// prompt text, and inside an autonomous chain that means a window waiting for a
// human nobody is watching for.
var headlessProviders = map[string]bool{
	"claude": true,
	"codex":  true,
	"vibe":   true,
}

// SupportsHeadless reports whether a provider can run a skill without a terminal.
func SupportsHeadless(provider string) bool {
	return headlessProviders[strings.ToLower(strings.TrimSpace(provider))]
}

// ResolveSkillMode decides how one launch runs, worst-specified last:
// the one-off override from the card menu, then the skill's own setting, then
// the project default, then interactive. Each level is normalized, so an unknown
// value falls through instead of pinning a mode nobody chose.
func ResolveSkillMode(override, skillMode string, project *models.Project) string {
	if mode := models.NormalizeSkillMode(override); mode != "" {
		return mode
	}
	if mode := models.NormalizeSkillMode(skillMode); mode != "" {
		return mode
	}
	if project != nil {
		if mode := models.NormalizeSkillMode(project.DefaultSkillMode); mode != "" {
			return mode
		}
	}
	return models.SkillModeInteractive
}

// skillModeFor resolves the mode for a skill on a project, reading the skill's
// own setting from the project's overrides when it has one.
func (d *DB) skillModeFor(projectID, skillID, override string) string {
	project, _ := d.GetProjectByID(projectID)
	skillMode := ""
	if stage, ok := StageSkillByID(skillID); ok {
		skillMode = stage.Mode
	}
	if stored, ok := d.projectSkillModes(projectID)[models.NormalizeSkillID(skillID)]; ok {
		skillMode = stored
	}
	return ResolveSkillMode(override, skillMode, project)
}

// ResolveLaunchMode decides the mode of one launch and refuses it outright when
// the resolved mode is one the project's provider cannot honour. Refusing here
// rather than at the agent keeps a silent fallback to interactive impossible.
func (d *DB) ResolveLaunchMode(projectID, skillID, override string) (string, error) {
	mode := d.skillModeFor(projectID, skillID, override)
	if mode != models.SkillModeNonInteractive {
		return mode, nil
	}
	project, _ := d.GetProjectByID(projectID)
	provider := ""
	if project != nil {
		provider = project.AIProvider
	}
	if err := checkHeadlessProvider(project, provider); err != nil {
		return "", err
	}
	return mode, nil
}

// checkHeadlessProvider turns an unsupported provider into the explicit refusal
// the launch reports, naming the provider so the message is actionable. A custom
// command template owns the mode itself unless it carries the mode placeholder.
func checkHeadlessProvider(project *models.Project, provider string) error {
	if project != nil && strings.TrimSpace(project.AICommandTemplate) != "" {
		if !strings.Contains(project.AICommandTemplate, "{mode}") {
			return fmt.Errorf("the project's custom AI command template has no {mode} placeholder, so it cannot run non-interactively; add the placeholder or launch interactively")
		}
		return nil
	}
	if !SupportsHeadless(provider) {
		return fmt.Errorf("provider %q has no attested headless mode; launch interactively or switch the project to claude, codex or vibe", provider)
	}
	return nil
}
