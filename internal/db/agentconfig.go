package db

import (
	"fmt"
	"strings"
	"tasks/internal/agentconfig"
	"tasks/internal/models"
	"tasks/internal/skills"
)

// AgentConfig exposes the project's method, never server paths, tracker
// credentials or execution settings.
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
	// The execution settings are the workstation's (#305): the configuration
	// carries the method only, and the agent resolves the invocation from its
	// own file.
	c := &agentconfig.Config{
		Skills: []agentconfig.Skill{}, SchemaVersion: agentconfig.Version, ProjectID: p.ID, ProjectName: p.Name, Description: p.Description,
		TrackerURL: p.TrackerUrl, JiraProject: p.JiraProject, GitRemoteURL: p.GitRemoteUrl, GithubRepo: p.GithubRepo, IssueTracker: p.IssueTracker,
		SpecFramework: p.SpecFramework, PRCreationStage: p.PRCreationStage, SpecArtifacts: models.NormalizeSpecArtifacts(p.SpecArtifacts),
		DefaultSkillMode: models.NormalizeSkillMode(p.DefaultSkillMode), FullChainStopStage: models.NormalizeFullChainStopStage(p.FullChainStopStage),
	}
	for _, repository := range p.Repositories {
		c.Repositories = append(c.Repositories, repository.URL)
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
		stage, ok := skills.StageSkillByID(skill.ID)
		if !ok {
			continue
		}
		// The stage's standard command: a workstation's own command name is the
		// workstation's to set (#305).
		content, _ := commandContentFromSkill(stage, skill.Content, c.SpecFramework)
		c.Skills = append(c.Skills, agentconfig.Skill{RequiresReconciliation: skill.ID == "adjust" && reconcile, ID: skill.ID, Directory: stage.DirName, Command: stage.Command, Content: skill.Content, CommandContent: content})
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// LegacyProjectExecution is the execution composition AgentConfig served
// before #305, project row over deployment settings, kept verbatim so a
// workstation seeds exactly what it used to receive. It reads the columns
// #492 will drop, and writes nothing. The terminal is the project row's own:
// the deployment's travels with the workstation defaults.
func (d *DB) LegacyProjectExecution(projectID string) (*agentconfig.SeedProject, error) {
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
	provider := firstNonEmpty(p.AIProvider, s.AIProvider)
	aiModels := agentconfig.MergeModels(
		agentconfig.ModelConfig{Model: p.AIModel, SkillModels: p.AISkillModels},
		agentconfig.ModelConfig{Model: s.AIModel, SkillModels: s.AISkillModels},
	)
	useWorktrees := p.UseWorktrees
	seed := &agentconfig.SeedProject{
		ProjectID:                   p.ID,
		AIProvider:                  provider,
		AICommandTemplate:           agentconfig.EffectiveCommandTemplate(provider, firstNonEmpty(p.AICommandTemplate, s.AICommandTemplate)),
		AICommandTemplateAutonomous: agentconfig.EffectiveCommandTemplate(provider, firstNonEmpty(p.AICommandTemplateAutonomous, s.AICommandTemplateAutonomous)),
		AIModel:                     aiModels.Model,
		AISkillModels:               aiModels.SkillModels,
		Terminal:                    strings.TrimSpace(p.ExternalTerminalCommand),
		UseWorktrees:                &useWorktrees,
		SetupProviders:              models.NormalizeSetupProviders(p.SetupProviders),
	}
	for skill, name := range p.SkillOverrides {
		if _, ok := skills.StageSkillByID(skill); ok && strings.TrimSpace(name) != "" {
			if seed.SkillCommands == nil {
				seed.SkillCommands = map[string]string{}
			}
			seed.SkillCommands[skill] = strings.TrimSpace(name)
		}
	}
	return seed, nil
}

// LegacyWorkstationExecution is what the workstation defaults are seeded
// from: the deployment's execution values, with the terminal and the editor
// of the paired account, which already fall back to the deployment's. The
// column default editor, code, is the provider default and is not sent.
func (d *DB) LegacyWorkstationExecution(userID string) (*agentconfig.SeedDefaults, error) {
	s, err := d.GetSettings()
	if err != nil {
		return nil, err
	}
	seed := &agentconfig.SeedDefaults{
		AIProvider: s.AIProvider, AICommandTemplate: s.AICommandTemplate, AICommandTemplateAutonomous: s.AICommandTemplateAutonomous,
		AIModel: s.AIModel, AISkillModels: s.AISkillModels, AIProviderModels: s.AIProviderModels,
		Terminal: s.ExternalTerminalCommand, EditorCommand: s.EditorCommand,
	}
	if personal, err := d.UserSettings(userID); err == nil && personal != nil {
		seed.Terminal, seed.EditorCommand = personal.ExternalTerminalCommand, personal.EditorCommand
	}
	if strings.TrimSpace(seed.EditorCommand) == agentconfig.DefaultEditor {
		seed.EditorCommand = ""
	}
	return seed, nil
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
