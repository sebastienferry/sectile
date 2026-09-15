package db

import (
	"fmt"
	"strings"
	"tasks/internal/agentconfig"
	"tasks/internal/models"
)

// AgentConfig exposes only execution settings, never server paths or tracker credentials.
func (d *DB) AgentConfig(projectID, taskKey string, framework ...string) (*agentconfig.Config, error) {
	if taskKey != "" {
		task, err := d.GetTaskByID(taskKey)
		if err != nil {
			return nil, err
		}
		if task == nil {
			return nil, fmt.Errorf("task not found: %s", taskKey)
		}
		if projectID != "" && projectID != task.ProjectID {
			return nil, fmt.Errorf("task does not belong to project %s", projectID)
		}
		projectID = task.ProjectID
	}
	if projectID == "" {
		return nil, fmt.Errorf("projectId or taskKey is required")
	}
	p, err := d.GetProjectByID(projectID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, fmt.Errorf("project not found: %s", projectID)
	}
	s, err := d.GetSettings()
	if err != nil {
		return nil, err
	}
	c := &agentconfig.Config{
		Skills: []agentconfig.Skill{}, SchemaVersion: agentconfig.Version, ProjectID: p.ID, ProjectName: p.Name, Description: p.Description,
		TrackerURL: p.TrackerUrl, LinearTeam: p.LinearTeam, JiraProject: p.JiraProject, GitRemoteURL: p.GitRemoteUrl, GithubRepo: p.GithubRepo, IssueTracker: p.IssueTracker,
		Parallelism: p.Parallelism, SpecFramework: p.SpecFramework, UseWorktrees: p.UseWorktrees, PRCreationStage: p.PRCreationStage,
		AIProvider: p.AIProvider, AICommandTemplate: p.AICommandTemplate, ExternalTerminalCommand: p.ExternalTerminalCommand,
	}
	if c.GithubRepo == "" {
		c.GithubRepo = s.GithubRepo
	}
	if c.IssueTracker == "" {
		c.IssueTracker = s.IssueTracker
	}
	if c.SpecFramework == "" {
		c.SpecFramework = s.SpecFramework
	}
	if c.AIProvider == "" {
		c.AIProvider = s.AIProvider
	}
	if c.AICommandTemplate == "" {
		c.AICommandTemplate = s.AICommandTemplate
	}
	// Legacy rows store a bare CLI name here; the runner never used it, so the agent must not see it.
	c.AICommandTemplate = agentconfig.EffectiveCommandTemplate(c.AIProvider, c.AICommandTemplate)
	if c.ExternalTerminalCommand == "" {
		c.ExternalTerminalCommand = s.ExternalTerminalCommand
	}
	if c.LinearTeam == "" {
		c.LinearTeam = s.LinearTeam
	}
	if len(framework) > 0 && framework[0] != "" {
		if !isKnownFrameworkAlias(framework[0]) {
			return nil, fmt.Errorf("unknown specification framework")
		}
		c.SpecFramework = models.NormalizeSpecFramework(framework[0])
	}
	origin, _ := adjustmentOverrideOrigin(d.projectSkillOverrides(p.ID))
	reconcile := origin == "review" || (origin != "adjust" && strings.TrimSpace(s.PromptCreatePR) != "")
	if origin != "adjust" && strings.TrimSpace(p.SkillOverrides["adjust"]) == "" && (strings.TrimSpace(p.SkillOverrides["review"]) != "") {
		reconcile = true
	}
	for _, skill := range d.EffectiveProjectSkills(p.ID, c.SpecFramework) {
		stage, ok := StageSkillByID(skill.ID)
		if !ok {
			continue
		}
		command := stage.Command
		if override := p.SkillOverrides[skill.ID]; override != "" {
			command = override
		}
		content, _ := commandContentFromSkill(stage, skill.Content, c.SpecFramework)
		c.Skills = append(c.Skills, agentconfig.Skill{RequiresReconciliation: skill.ID == "adjust" && reconcile, ID: skill.ID, Directory: stage.DirName, Command: command, Content: skill.Content, CommandContent: content})
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// AgentProjects omits server filesystem paths and credentials.
func (d *DB) AgentProjects() (*agentconfig.Projects, error) {
	projects, err := d.GetProjects()
	if err != nil {
		return nil, err
	}
	result := &agentconfig.Projects{SchemaVersion: agentconfig.Version, Projects: []agentconfig.Project{}}
	for _, p := range projects {
		result.Projects = append(result.Projects, agentconfig.Project{ID: p.ID, Name: p.Name, GitRemoteURL: p.GitRemoteUrl})
	}
	return result, nil
}
