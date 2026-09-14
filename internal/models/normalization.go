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
