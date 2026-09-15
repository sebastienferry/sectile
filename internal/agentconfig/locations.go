package agentconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Locations names where a CLI reads the configuration Sectile manages for it.
// Every path is relative to Home so writes stay inside one guarded root.
// An empty SkillDir means the provider has no supported skill convention; its
// dispatch still proceeds. MCPFile is never empty: a provider without a known
// registration target cannot be bootstrapped at all.
//
// SubstitutesArguments marks a CLI that replaces argument placeholders in the
// skill body. Those agents receive the body carrying the ticket reference; the
// others would show the placeholder as literal text.
type Locations struct {
	Home                 string
	SkillDir             string
	MCPFile              string
	SubstitutesArguments bool
}

// EffectiveProvider resolves the identity used to locate configuration. A custom
// template is represented by the CLI it actually launches, so a template wrapping
// a known CLI keeps that CLI's locations.
func EffectiveProvider(provider, template string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = "agy"
	}
	if provider != "custom" {
		return provider
	}
	command := firstWord(template)
	if command == "" {
		return provider
	}
	return strings.ToLower(filepath.Base(command))
}

// ResolveLocations maps a provider to its user-level configuration. Skills are
// installed only where the CLI is known to read them; the repository layout this
// replaced wrote every convention at once, which left four dead copies per project.
func ResolveLocations(provider string) (Locations, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Locations{}, err
	}
	loc := Locations{Home: home}
	switch provider {
	case "claude":
		// Personal skills live here, and custom commands have been merged into them:
		// a directory under .claude/skills is what defines /<name>.
		loc.SkillDir, loc.MCPFile, loc.SubstitutesArguments = ".claude/skills", ".claude.json", true
	case "agy":
		// Antigravity reads its skills and its MCP registry from the same user root,
		// and ignores workspace files for the registry.
		loc.SkillDir, loc.MCPFile = ".gemini/config/skills", ".gemini/config/mcp_config.json"
	case "codex":
		// Codex reads the cross-agent convention, not a directory of its own:
		// .agents/skills up to the repository root, then $HOME/.agents/skills.
		loc.SkillDir, loc.MCPFile = ".agents/skills", ".codex/config.toml"
	case "gemini":
		loc.MCPFile = ".gemini/settings.json"
	case "cursor":
		loc.MCPFile = ".cursor/mcp.json"
	case "vibe":
		loc.MCPFile = ".vibe/config.toml"
	default:
		return Locations{}, fmt.Errorf("automatic setup is unsupported for provider %q; select a supported aiProvider", provider)
	}
	return loc, nil
}

// InstallsSkills reports whether the provider has a supported skill convention.
func (l Locations) InstallsSkills() bool { return l.SkillDir != "" }

// SkillProviders are the agents Sectile can install skills for. Every other
// supported provider receives the MCP registration alone.
var SkillProviders = []string{"claude", "codex", "agy"}

// SetupProviders resolves the agents to set up for a project. The provider that
// actually runs the task is always included: it cannot work without its MCP
// registration. Additional agents are opt-in, so a workstation is never
// configured behind its owner's back.
func SetupProviders(config Config) ([]string, error) {
	selected := EffectiveProvider(config.AIProvider, config.AICommandTemplate)
	providers := []string{selected}
	seen := map[string]bool{selected: true}
	for _, extra := range config.SetupProviders {
		extra = strings.ToLower(strings.TrimSpace(extra))
		if extra == "" || seen[extra] {
			continue
		}
		if _, err := ResolveLocations(extra); err != nil {
			return nil, err
		}
		seen[extra] = true
		providers = append(providers, extra)
	}
	if _, err := ResolveLocations(selected); err != nil {
		return nil, err
	}
	return providers, nil
}

// firstWord extracts the executable a shell would run, honouring the quotes a
// path containing spaces needs. An unresolved executable makes the dispatch fail,
// so splitting on whitespace alone is not good enough here.
func firstWord(template string) string {
	template = strings.TrimSpace(template)
	if template == "" {
		return ""
	}
	if quote := template[0]; quote == '"' || quote == '\'' {
		if end := strings.IndexByte(template[1:], quote); end >= 0 {
			return template[1 : end+1]
		}
		return strings.TrimSpace(template[1:])
	}
	if end := strings.IndexAny(template, " \t"); end >= 0 {
		return template[:end]
	}
	return template
}
