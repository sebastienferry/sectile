package models

import "strings"

func CleanGithubRepo(repo string) string {
	raw := strings.TrimSpace(repo)
	raw = strings.TrimSuffix(raw, ".git")
	if strings.Contains(raw, "github.com/") {
		parts := strings.Split(raw, "github.com/")
		if len(parts) > 1 {
			return strings.TrimSpace(parts[1])
		}
	} else if strings.Contains(raw, "github.com:") {
		parts := strings.Split(raw, "github.com:")
		if len(parts) > 1 {
			return strings.TrimSpace(parts[1])
		}
	}
	if strings.Contains(raw, "/") && !strings.Contains(raw, ":") && !strings.Contains(raw, " ") {
		return raw
	}
	return raw
}

func NormalizeIssueTypes(types []string) []string {
	out := make([]string, 0, len(types))
	seen := map[string]bool{}
	for _, t := range types {
		t = strings.TrimSpace(t)
		if t == "" || seen[strings.ToLower(t)] {
			continue
		}
		seen[strings.ToLower(t)] = true
		out = append(out, t)
	}
	if len(out) == 0 {
		return []string{"Task", "Story"}
	}
	return out
}

// SetupProviders are the agents Sectile can install skills and an MCP
// registration for, beyond the one that runs the tasks.
var SetupProviders = []string{"claude", "codex", "agy"}

// NormalizeSetupProviders keeps the supported agents only, lowercased, without
// duplicates and in the order the caller listed them.
func NormalizeSetupProviders(list []string) []string {
	supported := make(map[string]bool, len(SetupProviders))
	for _, provider := range SetupProviders {
		supported[provider] = true
	}
	seen := make(map[string]bool, len(list))
	out := make([]string, 0, len(list))
	for _, provider := range list {
		provider = strings.ToLower(strings.TrimSpace(provider))
		if !supported[provider] || seen[provider] {
			continue
		}
		seen[provider] = true
		out = append(out, provider)
	}
	return out
}

func NormalizeSpecFramework(framework string) string {
	switch strings.ToLower(strings.TrimSpace(framework)) {
	case "openspec", "open-spec", "open spec":
		return "openspec"
	case "speckit", "spec-kit", "spec kit", "specify":
		return "speckit"
	case "":
		return "speckit"
	default:
		// "openfeature" and anything unknown falls back to Spec Kit rather than
		// silently installing the wrong toolchain.
		return "speckit"
	}
}

// OptionalViews are the workspace views a project shows only when it asks for
// them. They answer a planning need (sorting unclassified work, laying macros
// on horizons, reading the sprint schedule) that a project tracking a single
// stream of tickets never has, and an empty sidebar entry costs more than it
// gives. None of them is enabled by default.
var OptionalViews = []string{"triage", "roadmap", "timeline"}

// NormalizeEnabledViews keeps the supported views only, lowercased, without
// duplicates, in the canonical order of OptionalViews so two projects that
// enabled the same set store the same value.
func NormalizeEnabledViews(list []string) []string {
	asked := make(map[string]bool, len(list))
	for _, view := range list {
		asked[strings.ToLower(strings.TrimSpace(view))] = true
	}
	out := make([]string, 0, len(OptionalViews))
	for _, view := range OptionalViews {
		if asked[view] {
			out = append(out, view)
		}
	}
	return out
}
