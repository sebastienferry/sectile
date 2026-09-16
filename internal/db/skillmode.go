package db

import "tasks/internal/models"

// ResolveSkillMode decides how one launch runs. The first level with an opinion
// wins: the one-off override chosen for that launch, then the skill's own
// setting, then the project default, then interactive.
//
// Every level is read through models.NormalizeSkillMode, so an unrecognised
// stored value falls through instead of pinning a mode nobody configured. The
// resolution lives here, on the server, and the resolved value travels to the
// agent on the dispatch: an agent holding a stale configuration copy must never
// be able to decide this on its own.
func ResolveSkillMode(override, skill, project string) string {
	for _, level := range []string{override, skill, project} {
		if mode := models.NormalizeSkillMode(level); mode != models.SkillModeUnset {
			return mode
		}
	}
	return models.SkillModeInteractive
}
