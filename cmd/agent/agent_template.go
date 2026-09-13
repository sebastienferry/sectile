package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"tasks/internal/models"
)

// agentCommandContext contains launch-time data, never persisted configuration.
type agentCommandContext struct {
	Task      models.Task
	Branch    string
	Directory string
	Tracker   string
	Repo      string
}

func (c agentCommandContext) values(prompt string) map[string]string {
	tracker := c.Task.Source
	if tracker == "" {
		tracker = c.Tracker
	}
	if tracker == "" {
		tracker = "github"
	}
	repo := c.Repo
	if repo == "" {
		repo = filepath.Base(c.Directory)
	}
	return map[string]string{
		"prompt": prompt, "issueKey": c.Task.Key, "issueTitle": c.Task.Title,
		"issueDesc": c.Task.Description, "branchName": c.Branch,
		"repoPath": c.Directory, "tracker": strings.ToLower(tracker), "repo": repo,
	}
}

// expandAgentTemplate interpolates argument data in the original template only.
// Quoting state belongs to the template; inserted values cannot change it.
func expandAgentTemplate(template string, values map[string]string) (string, error) {
	var out strings.Builder
	var quote byte
	foundPrompt := false
	for i := 0; i < len(template); {
		ch := template[i]
		if ch == '\\' && quote != '\'' && i+1 < len(template) {
			// Preserve escaped template characters without treating escaped quotes as delimiters.
			next := template[i+1]
			if quote == 0 || strings.ContainsRune("$`\"\\\n", rune(next)) {
				out.WriteString(template[i : i+2])
				i += 2
				continue
			}
		}
		if ch == '{' {
			if end := strings.IndexByte(template[i:], '}'); end >= 0 {
				name := template[i+1 : i+end]
				if value, ok := values[name]; ok {
					if strings.ContainsRune(value, 0) {
						return "", fmt.Errorf("command placeholder {%s} contains a NUL byte", name)
					}
					if name == "prompt" {
						foundPrompt = true
					}
					switch quote {
					case '\'':
						out.WriteString(strings.ReplaceAll(value, "'", `'\''`))
					case '"':
						// Temporarily leave double quotes so interactive history expansion
						// cannot interpret exclamation marks in task text either.
						out.WriteString(`"` + quoteShell(value) + `"`)
					default:
						out.WriteString(quoteShell(value))
					}
					i += end + 1
					continue
				}
			}
		}
		if ch == '\'' || ch == '"' {
			if quote == 0 {
				quote = ch
			} else if quote == ch {
				quote = 0
			}
		}
		out.WriteByte(ch)
		i++
	}
	if !foundPrompt {
		return "", fmt.Errorf("AI command template must contain {prompt}")
	}
	return out.String(), nil
}
