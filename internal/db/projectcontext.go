package db

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"tasks/internal/models"
)

const (
	taskflowAgentsBlockStart = "<!-- taskflow:project-context:start -->"
	taskflowAgentsBlockEnd   = "<!-- taskflow:project-context:end -->"
)

type taskflowProjectConfig struct {
	SchemaVersion int    `json:"schemaVersion"`
	ProjectID     string `json:"projectId"`
	ProjectName   string `json:"projectName"`

	GitRemoteURL string `json:"gitRemoteUrl,omitempty"`
	IssueTracker string `json:"issueTracker"`
	TrackerURL   string `json:"trackerUrl,omitempty"`
	LinearTeam   string `json:"linearTeam,omitempty"`
	GithubRepo   string `json:"githubRepo,omitempty"`
	JiraProject  string `json:"jiraProject,omitempty"`

	Workflow taskflowWorkflowConfig `json:"workflow"`
}

type taskflowWorkflowConfig struct {
	Operator     string   `json:"operator"`
	UseWorktrees bool     `json:"useWorktrees"`
	Stages       []string `json:"stages"`
}

func projectTaskflowConfig(project *models.Project) taskflowProjectConfig {
	return taskflowProjectConfig{
		SchemaVersion: 1,
		ProjectID:     project.ID,
		ProjectName:   project.Name,
		GitRemoteURL:  project.GitRemoteUrl,
		IssueTracker:  project.IssueTracker,
		TrackerURL:    project.TrackerUrl,
		LinearTeam:    project.LinearTeam,
		GithubRepo:    project.GithubRepo,
		JiraProject:   project.JiraProject,
		Workflow: taskflowWorkflowConfig{
			Operator:     "Sectile",
			UseWorktrees: project.UseWorktrees,
			Stages:       []string{"clarify", "specify", "implement", "review", "handoff"},
		},
	}
}

// writeProjectContextFiles records the tracker connection and TaskFlow workflow
// in every checkout currently associated with the project. It replaces only the
// marked TaskFlow section in AGENTS.md, leaving repository instructions intact.
func writeProjectContextFiles(project *models.Project) error {
	if project == nil || strings.TrimSpace(project.RepoPath) == "" {
		return nil
	}

	for _, targetDir := range getGitWorktreePaths(project.RepoPath) {
		if err := os.MkdirAll(filepath.Join(targetDir, ".taskflow"), 0755); err != nil {
			return fmt.Errorf("création du dossier .taskflow dans %s : %w", targetDir, err)
		}
		if err := writeProjectTaskflowConfig(filepath.Join(targetDir, ".taskflow", "config.json"), projectTaskflowConfig(project)); err != nil {
			return err
		}
		if err := upsertTaskflowAgentsBlock(filepath.Join(targetDir, "AGENTS.md"), renderTaskflowAgentsBlock(project)); err != nil {
			return err
		}
	}
	return nil
}

// writeProjectTaskflowConfig updates TaskFlow-owned keys while preserving
// optional settings written by skill installation or a newer TaskFlow version.
func writeProjectTaskflowConfig(path string, config taskflowProjectConfig) error {
	updated, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("encodage de la configuration TaskFlow : %w", err)
	}

	values := map[string]json.RawMessage{}
	if current, readErr := os.ReadFile(path); readErr == nil && len(strings.TrimSpace(string(current))) > 0 {
		if err := json.Unmarshal(current, &values); err != nil {
			return fmt.Errorf("lecture de la configuration TaskFlow existante dans %s : %w", path, err)
		}
	} else if readErr != nil && !os.IsNotExist(readErr) {
		return fmt.Errorf("lecture de %s : %w", path, readErr)
	}

	var projectValues map[string]json.RawMessage
	if err := json.Unmarshal(updated, &projectValues); err != nil {
		return fmt.Errorf("décodage de la configuration TaskFlow : %w", err)
	}
	for key, value := range projectValues {
		values[key] = value
	}

	contents, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return fmt.Errorf("encodage de la configuration TaskFlow : %w", err)
	}
	if err := os.WriteFile(path, contents, 0644); err != nil {
		return fmt.Errorf("écriture de .taskflow/config.json dans %s : %w", filepath.Dir(path), err)
	}
	return nil
}

func renderTaskflowAgentsBlock(project *models.Project) string {
	tracker := strings.TrimSpace(project.IssueTracker)
	if tracker == "" {
		tracker = "local"
	}

	var b strings.Builder
	b.WriteString(taskflowAgentsBlockStart + "\n")
	b.WriteString("## Sectile workflow\n\n")
	b.WriteString("Sectile operates the development workflow for this repository. Use `.taskflow/config.json` as the source of truth for the project and remote tracker context.\n\n")
	fmt.Fprintf(&b, "- Tracker: `%s`\n", tracker)
	if project.TrackerUrl != "" {
		fmt.Fprintf(&b, "- Tracker URL: `%s`\n", project.TrackerUrl)
	}
	if project.GithubRepo != "" {
		fmt.Fprintf(&b, "- GitHub repository: `%s`\n", project.GithubRepo)
	}
	if project.LinearTeam != "" {
		fmt.Fprintf(&b, "- Linear team: `%s`\n", project.LinearTeam)
	}
	if project.JiraProject != "" {
		fmt.Fprintf(&b, "- Jira project: `%s`\n", project.JiraProject)
	}
	if project.GitRemoteUrl != "" {
		fmt.Fprintf(&b, "- Git remote: `%s`\n", project.GitRemoteUrl)
	}
	b.WriteString("\nDevelopment work follows Sectile's stages: clarify, specify, implement, review and pull request, then human merge and handoff. Keep the assigned branch/worktree, use Sectile's local stage handler for standalone runs, and let managed Sectile runs own stage transitions and tracker synchronization.\n")
	b.WriteString(taskflowAgentsBlockEnd + "\n")
	return b.String()
}

func upsertTaskflowAgentsBlock(path, block string) error {
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("lecture de %s : %w", path, err)
	}

	content := string(raw)
	start := strings.Index(content, taskflowAgentsBlockStart)
	end := strings.Index(content, taskflowAgentsBlockEnd)
	switch {
	case start >= 0 && end >= start:
		end += len(taskflowAgentsBlockEnd)
		if end < len(content) && content[end] == '\n' {
			end++
		}
		content = content[:start] + block + content[end:]
	case start >= 0 || end >= 0:
		return fmt.Errorf("bloc TaskFlow incomplet dans %s", path)
	case strings.TrimSpace(content) == "":
		content = block
	default:
		content = strings.TrimRight(content, "\n") + "\n\n" + block
	}

	mode := os.FileMode(0644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		return fmt.Errorf("écriture de %s : %w", path, err)
	}
	return nil
}
