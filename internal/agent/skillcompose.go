package agent

import (
	"strings"
	"tasks/internal/agentconfig"
	"tasks/internal/models"
	"tasks/internal/skills"
)

// runSkillContent is what a launched run reads: the server's content, or the
// composite when a work-only override applies (#732). A full replacement, the
// project's or this workstation's, is run as it stands. Otherwise each section
// comes from the project's work override, else this workstation's, else the
// built-in, and pickup inlines its stages the same way.
func runSkillContent(config agentconfig.Config, skill agentconfig.Skill) string {
	if skill.Custom && skill.OverrideKind == models.SkillOverrideFull && skill.WorkstationWork == "" {
		return skill.Content
	}
	stage, ok := skills.StageSkillByID(skill.ID)
	if !ok || !skills.Overridable(stage) {
		return skill.Content
	}
	project, workstation := projectWork(config), workstationWork(config)
	merged := skills.SkillOverrides{}
	for _, id := range append([]string{stage.ID}, skills.ComposedStageIDs(stage.ID)...) {
		p, inProject := project[id]
		w, inWorkstation := workstation[id]
		if inProject || inWorkstation {
			merged[id] = p.Over(w)
		}
	}
	if len(merged) == 0 {
		return skill.Content
	}
	return skills.RenderComposedSkillContent(stage, config.SpecFramework, merged) + skills.ProjectSkillPolicy(stage.ID, config.PRCreationStage)
}

// projectWork are the project's work-only overrides the server sent, by skill
// ID. A composed pickup carries the kind without a work of its own, and a body
// that does not parse is skipped: the server validated it when it was saved.
func projectWork(config agentconfig.Config) skills.SkillOverrides {
	out := skills.SkillOverrides{}
	for _, skill := range config.Skills {
		if skill.OverrideKind != models.SkillOverrideWork || strings.TrimSpace(skill.WorkContent) == "" {
			continue
		}
		addWork(out, skill.ID, skill.WorkContent)
	}
	return out
}

// workstationWork are this workstation's work-only overrides, which Resolve
// kept, by skill ID.
func workstationWork(config agentconfig.Config) skills.SkillOverrides {
	out := skills.SkillOverrides{}
	for _, skill := range config.Skills {
		if strings.TrimSpace(skill.WorkstationWork) == "" {
			continue
		}
		addWork(out, skill.ID, skill.WorkstationWork)
	}
	return out
}

func addWork(out skills.SkillOverrides, id, content string) {
	stage, ok := skills.StageSkillByID(id)
	if !ok {
		return
	}
	if work, err := skills.ParseWorkSections(stage, content); err == nil {
		out[stage.ID] = work
	}
}
