package agent

import (
	"context"
	"fmt"
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

// directSetupConfig is the configuration a direct setup writes: resolved with this workstation's settings,
// its DirectContent carrying every known project's work-only overrides as variants (#732).
//
// config is the server's configuration, not yet resolved: Resolve drops the
// project's work rows a workstation full override shadows, while the direct
// copy, shared by every project, still carries them. The other projects are
// those the server lists minus the ones this workstation disconnected; a
// failed fetch is returned, so the caller leaves the copies as they are rather
// than writing one that silently lacks a project's variant.
func (d *agentDaemon) directSetupConfig(ctx context.Context, config agentconfig.Config) (agentconfig.Config, error) {
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		return config, err
	}
	var projects []skills.ProjectOverrides
	if work := projectWork(config); len(work) > 0 {
		projects = append(projects, skills.ProjectOverrides{ProjectID: config.ProjectID, Skills: work})
	}
	listed, err := d.discoverProjects(ctx)
	if err != nil {
		return config, err
	}
	for _, project := range listed.Projects {
		if project.ID == config.ProjectID || settings.DisconnectedProjects[project.ID] {
			continue
		}
		other, err := d.fetchConfig(ctx, project.ID, "")
		if err != nil {
			return config, fmt.Errorf("fetch project %q configuration: %w", project.ID, err)
		}
		if work := projectWork(other); len(work) > 0 {
			projects = append(projects, skills.ProjectOverrides{ProjectID: project.ID, Skills: work})
		}
	}
	// The workstation's work overrides come from its settings rather than
	// from Resolve: the one this project's full replacement shadows is still
	// the fallback of every other project.
	workstation := skills.SkillOverrides{}
	for id, override := range settings.Skills {
		if override.Kind == models.SkillOverrideWork && strings.TrimSpace(override.Content) != "" {
			addWork(workstation, id, override.Content)
		}
	}
	resolved := agentconfig.Resolve(config, settings)
	for i, skill := range resolved.Skills {
		stage, ok := skills.StageSkillByID(skill.ID)
		if !ok || !skills.Overridable(stage) || !overriddenAnywhere(stage.ID, workstation, projects) {
			continue
		}
		resolved.Skills[i].DirectContent = skills.RenderDirectComposedSkillContent(stage, workstation, projects)
		resolved.Skills[i].DirectCommandContent = skills.RenderDirectComposedSkillCommand(stage, workstation, projects)
	}
	return resolved, nil
}

// withDirectContent copies the direct copies directSetupConfig composed onto
// config, the same server configuration already resolved for an operation, so
// what the operation set besides the skills stays as it is.
func withDirectContent(config, direct agentconfig.Config) agentconfig.Config {
	composed := map[string]agentconfig.Skill{}
	for _, skill := range direct.Skills {
		composed[skill.ID] = skill
	}
	config.Skills = append([]agentconfig.Skill{}, config.Skills...)
	for i, skill := range config.Skills {
		if source, ok := composed[skill.ID]; ok {
			config.Skills[i].DirectContent, config.Skills[i].DirectCommandContent = source.DirectContent, source.DirectCommandContent
		}
	}
	return config
}

// overriddenAnywhere says whether the skill, or a stage it inlines, has a work
// override on this workstation or in any project.
func overriddenAnywhere(id string, workstation skills.SkillOverrides, projects []skills.ProjectOverrides) bool {
	for _, stage := range append([]string{id}, skills.ComposedStageIDs(id)...) {
		if _, ok := workstation[stage]; ok {
			return true
		}
		for _, project := range projects {
			if _, ok := project.Skills[stage]; ok {
				return true
			}
		}
	}
	return false
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
