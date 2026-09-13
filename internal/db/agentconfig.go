package db

import (
	"fmt"
	"tasks/internal/agentconfig"
)

// AgentConfig exposes only execution settings, never server paths or tracker credentials.
func (d *DB) AgentConfig(projectID, taskKey string) (*agentconfig.Config, error) {
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
		GitRemoteURL: p.GitRemoteUrl,
		Parallelism:  p.Parallelism, SpecFramework: p.SpecFramework, UseWorktrees: p.UseWorktrees, PRCreationStage: p.PRCreationStage,
		AIProvider: p.AIProvider, AICommandTemplate: p.AICommandTemplate, ExternalTerminalCommand: p.ExternalTerminalCommand,
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
	if c.ExternalTerminalCommand == "" {
		c.ExternalTerminalCommand = s.ExternalTerminalCommand
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
		c.Skills = append(c.Skills, agentconfig.Skill{ID: skill.ID, Directory: stage.DirName, Command: command, Content: skill.Content, CommandContent: content})
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
