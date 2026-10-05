package agent

import (
	"context"
	"fmt"
	"log"
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
		addWork(out, skill.ID, skill.WorkContent, "project "+config.ProjectID)
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
		addWork(out, skill.ID, skill.WorkstationWork, "workstation")
	}
	return out
}

// directSetupConfig is the configuration a direct setup writes: resolved with this workstation's settings,
// its DirectContent carrying every known project's work-only overrides as variants (#732).
//
// config is the server's configuration, not yet resolved: Resolve drops the
// project's work rows a workstation full override shadows, while the direct
// copy, shared by every project, still carries them. The other projects are
// those the server lists minus the ones this workstation disconnected.
//
// Only unreadable workstation settings are an error. A project list or another
// project's configuration that cannot be read leaves that project's variants
// out and returns a warning naming it. The caller decides: init and the Desktop
// setup write the copies and surface the warnings, as they did before work
// overrides existed, while sync_config and refresh_skills refuse, leaving the
// copies already there rather than writing one that silently lacks a project's
// variant.
func (d *agentDaemon) directSetupConfig(ctx context.Context, config agentconfig.Config) (agentconfig.Config, []string, error) {
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		return config, nil, err
	}
	var warnings []string
	var projects []skills.ProjectOverrides
	if work := projectWork(config); len(work) > 0 {
		projects = append(projects, skills.ProjectOverrides{ProjectID: config.ProjectID, Skills: work})
	}
	listed, err := d.discoverProjects(ctx)
	if err != nil {
		listed = agentconfig.Projects{}
		warnings = append(warnings, fmt.Sprintf("projects not listed, only project %q's skill variants reach the direct skill copies: %v", config.ProjectID, err))
	}
	for _, project := range listed.Projects {
		if project.ID == config.ProjectID || settings.DisconnectedProjects[project.ID] {
			continue
		}
		other, err := d.fetchConfig(ctx, project.ID, "")
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("project %q configuration not read, its skill variants are left out of the direct skill copies: %v", project.ID, err))
			continue
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
			addWork(workstation, id, override.Content, "workstation")
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
	return resolved, warnings, nil
}

// directSetupWarnings are the warnings of a lenient direct setup, one line
// each, to append to its message.
func directSetupWarnings(warnings []string) string {
	var b strings.Builder
	for _, warning := range warnings {
		b.WriteString("\nWarning: " + warning)
	}
	return b.String()
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

// addWork parses a work-only override into out. One for an unknown skill or
// one that does not parse is skipped and logged, origin saying whose it is.
func addWork(out skills.SkillOverrides, id, content, origin string) {
	stage, ok := skills.StageSkillByID(id)
	if !ok {
		log.Printf("[Agent] %s skill %q: work-only override skipped, unknown skill", origin, id)
		return
	}
	work, err := skills.ParseWorkSections(stage, content)
	if err != nil {
		log.Printf("[Agent] %s skill %q: work-only override skipped as unparsable: %v", origin, id, err)
		return
	}
	out[stage.ID] = work
}
