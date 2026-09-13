package agentconfig

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var component = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// Validate checks the downloaded contract before any local preparation or execution.
func (c Config) Validate() error {
	if c.PRCreationStage != "" && c.PRCreationStage != "specified" && c.PRCreationStage != "implemented" {
		return fmt.Errorf("invalid prCreationStage")
	}

	if c.SchemaVersion != Version {
		return fmt.Errorf("unsupported configuration version %d (agent supports %d)", c.SchemaVersion, Version)
	}
	if strings.TrimSpace(c.ProjectID) == "" {
		return fmt.Errorf("configuration projectId is required")
	}

	switch c.AIProvider {
	case "", "agy", "codex", "claude", "gemini", "cursor", "vibe", "custom":
	default:
		return fmt.Errorf("unsupported AI provider %q", c.AIProvider)
	}
	if c.AIProvider == "custom" && strings.TrimSpace(c.AICommandTemplate) == "" {
		return fmt.Errorf("custom provider requires aiCommandTemplate")
	}
	if c.AICommandTemplate != "" && !strings.Contains(c.AICommandTemplate, "{prompt}") {
		return fmt.Errorf("aiCommandTemplate must contain {prompt}")
	}
	_, err := skillFiles(c.Skills)
	return err
}

// skillFiles validates every destination before Scaffold touches the checkout.
func skillFiles(skills []Skill) (map[string]string, error) {
	files := map[string]string{}
	ids := map[string]bool{}
	for _, s := range skills {
		if !component.MatchString(s.ID) || ids[s.ID] {
			return nil, fmt.Errorf("invalid or duplicate skill ID %q", s.ID)
		}
		ids[s.ID] = true
		if !component.MatchString(s.Directory) {
			return nil, fmt.Errorf("invalid skill directory %q", s.Directory)
		}
		cmd := strings.TrimPrefix(s.Command, "/")
		if cmd == "" {
			cmd = s.Directory
		}
		if !component.MatchString(cmd) {
			return nil, fmt.Errorf("invalid skill command %q", s.Command)
		}
		paths := []string{filepath.Join(".claude/commands", cmd+".md")}
		for _, dir := range []string{".agents/skills", ".agy/skills", ".claude/skills", ".gemini/skills", ".skills"} {
			paths = append(paths, filepath.Join(dir, s.Directory, "SKILL.md"))
		}
		for i, p := range paths {
			if _, exists := files[p]; exists {
				return nil, fmt.Errorf("duplicate skill destination %q", p)
			}
			content := s.Content
			if i == 0 {
				content = s.CommandContent
			}
			files[p] = content
		}
	}
	return files, nil
}

func managedSkillPath(p string) bool {
	parts := strings.Split(filepath.ToSlash(p), "/")
	if len(parts) == 3 && parts[0] == ".claude" && parts[1] == "commands" {
		return strings.HasSuffix(parts[2], ".md") && component.MatchString(strings.TrimSuffix(parts[2], ".md"))
	}
	if len(parts) == 3 && parts[0] == ".skills" {
		return component.MatchString(parts[1]) && parts[2] == "SKILL.md"
	}
	if len(parts) != 4 || parts[1] != "skills" || parts[3] != "SKILL.md" || !component.MatchString(parts[2]) {
		return false
	}
	switch parts[0] {
	case ".agents", ".agy", ".claude", ".gemini":
		return true
	}
	return false
}
