package agentconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const contextStart = "<!-- taskflow:project-context:start -->"
const contextEnd = "<!-- taskflow:project-context:end -->"

// projectContextFiles preserves personal settings and instructions. All reads and
// writes stay within the agent's mapped checkout, including through symlinks.
func projectContextFiles(fs *os.Root, config Config) (map[string][]byte, error) {
	values := map[string]any{}
	raw, err := fs.ReadFile(".taskflow/config.json")
	if err == nil {
		if err := json.Unmarshal(raw, &values); err != nil {
			return nil, fmt.Errorf("read project context: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if values == nil {
		values = map[string]any{}
	}
	owned := map[string]any{"schemaVersion": Version, "projectId": config.ProjectID, "projectName": config.ProjectName, "gitRemoteUrl": config.GitRemoteURL, "issueTracker": config.IssueTracker, "trackerUrl": config.TrackerURL, "githubRepo": config.GithubRepo, "linearTeam": config.LinearTeam, "jiraProject": config.JiraProject,
		"workflow": map[string]any{"operator": "Sectile", "useWorktrees": config.UseWorktrees, "stages": []string{"clarify", "specify", "implement", "adjust", "handoff"}}}
	for k, v := range owned {
		values[k] = v
	}
	configJSON, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return nil, err
	}
	var block strings.Builder
	block.WriteString(contextStart + "\n## Sectile workflow\n\nSectile operates the development workflow for this repository. Use `.taskflow/config.json` for project and tracker identity.\n\n")
	for _, item := range [][2]string{{"Tracker", config.IssueTracker}, {"Tracker URL", config.TrackerURL}, {"GitHub repository", config.GithubRepo}, {"Linear team", config.LinearTeam}, {"Jira project", config.JiraProject}, {"Git remote", config.GitRemoteURL}} {
		if item[1] != "" {
			fmt.Fprintf(&block, "- %s: `%s`\n", item[0], item[1])
		}
	}
	block.WriteString("\nDevelopment follows clarify, specify, implement, adjust the existing pull request, then human merge and handoff. Keep the assigned branch/worktree. Use the agent's MCP interface for task reads, workflow transitions and run completion.\n" + contextEnd + "\n")
	raw, err = fs.ReadFile("AGENTS.md")
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	content := string(raw)
	start, end := strings.Index(content, contextStart), strings.Index(content, contextEnd)
	switch {
	case start >= 0 && end >= start:
		end += len(contextEnd)
		if end < len(content) && content[end] == '\n' {
			end++
		}
		content = content[:start] + block.String() + content[end:]
	case start >= 0 || end >= 0:
		return nil, fmt.Errorf("incomplete project context block in AGENTS.md")
	case strings.TrimSpace(content) == "":
		content = block.String()
	default:
		content = strings.TrimRight(content, "\n") + "\n\n" + block.String()
	}
	return map[string][]byte{".taskflow/config.json": configJSON, "AGENTS.md": []byte(content)}, nil
}
