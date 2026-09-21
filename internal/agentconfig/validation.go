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

	if err := ValidProvider(c.AIProvider); err != nil {
		return err
	}
	if c.AIProvider == "custom" && strings.TrimSpace(c.AICommandTemplate) == "" {
		return fmt.Errorf("custom provider requires aiCommandTemplate")
	}
	if c.AICommandTemplate != "" && !strings.Contains(c.AICommandTemplate, "{prompt}") {
		return fmt.Errorf("aiCommandTemplate must contain {prompt}")
	}
	if err := ValidModelConfig(c.Models()); err != nil {
		return err
	}
	return validateSkills(c.Skills)
}

// validateSkills rejects the whole contract before Scaffold touches any file, so a
// single malformed identifier can never leave a partial installation behind.
func validateSkills(skills []Skill) error {
	ids, dirs, commands := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, s := range skills {
		if !component.MatchString(s.ID) || ids[s.ID] {
			return fmt.Errorf("invalid or duplicate skill ID %q", s.ID)
		}
		ids[s.ID] = true
		if !component.MatchString(s.Directory) || dirs[s.Directory] {
			return fmt.Errorf("invalid or duplicate skill directory %q", s.Directory)
		}
		dirs[s.Directory] = true
		cmd := skillCommand(s)
		if !component.MatchString(cmd) || commands[cmd] {
			return fmt.Errorf("invalid or duplicate skill command %q", s.Command)
		}
		commands[cmd] = true
	}
	return nil
}

func skillCommand(s Skill) string {
	cmd := strings.TrimPrefix(s.Command, "/")
	if cmd == "" {
		cmd = s.Directory
	}
	return cmd
}

// skillFiles validates every destination before Scaffold touches the configuration.
// A provider without a skill convention yields no destination, which is a valid
// installation rather than an error.
func skillFiles(skills []Skill, loc Locations) (map[string]string, error) {
	if err := validateSkills(skills); err != nil {
		return nil, err
	}
	files := map[string]string{}
	if !loc.InstallsSkills() {
		return files, nil
	}
	for _, s := range skills {
		path := filepath.Join(loc.SkillDir, s.Directory, "SKILL.md")
		if _, exists := files[path]; exists {
			return nil, fmt.Errorf("duplicate skill destination %q", path)
		}
		files[path] = skillBody(s, loc)
	}
	return files, nil
}

// skillBody picks the text to install. An agent that substitutes arguments gets
// the body carrying the ticket reference; the others would render the
// placeholder literally, so they get the plain instructions.
func skillBody(s Skill, loc Locations) string {
	if loc.SubstitutesArguments && s.CommandContent != "" {
		return s.CommandContent
	}
	return s.Content
}

// managedPath guards the manifest: only paths Sectile installs for the resolved
// provider may be recorded, refreshed or retired.
func managedPath(p string, loc Locations) bool {
	if !loc.InstallsSkills() {
		return false
	}
	parts := strings.Split(filepath.ToSlash(p), "/")
	dir := strings.Split(loc.SkillDir, "/")
	return len(parts) == len(dir)+2 && strings.Join(parts[:len(dir)], "/") == loc.SkillDir &&
		component.MatchString(parts[len(dir)]) && parts[len(parts)-1] == "SKILL.md"
}

// retiredSkillDirs are user-level destinations an earlier release installed to
// before each agent's real path was confirmed. They are still recognised so the
// manifest can retire them instead of treating them as foreign files.
var retiredSkillDirs = []string{".agy/skills", ".codex/skills"}

// managedRetiredPath reports a destination this release no longer installs to but
// previously owned: the withdrawn skill directories, and Claude's command files,
// which duplicated the command its skill already declares.
func managedRetiredPath(p string) bool {
	parts := strings.Split(filepath.ToSlash(p), "/")
	if len(parts) == 3 && parts[0] == ".claude" && parts[1] == "commands" {
		return strings.HasSuffix(parts[2], ".md") && component.MatchString(strings.TrimSuffix(parts[2], ".md"))
	}
	for _, dir := range retiredSkillDirs {
		prefix := strings.Split(dir, "/")
		if len(parts) == len(prefix)+2 && strings.Join(parts[:len(prefix)], "/") == dir &&
			component.MatchString(parts[len(prefix)]) && parts[len(parts)-1] == "SKILL.md" {
			return true
		}
	}
	return false
}

// managedLegacyPath recognises the repository layout that preceded the move to
// user-level configuration, so those copies can be retired from a checkout.
func managedLegacyPath(p string) bool {
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

// ValidProvider checks that provider is a supported AI provider or empty.
func ValidProvider(provider string) error {
	switch provider {
	case "", "agy", "codex", "claude", "gemini", "cursor", "vibe", "custom":
		return nil
	default:
		return fmt.Errorf("unsupported AI provider %q", provider)
	}
}
