package models

import "strings"

// SanitizeBranchName removes characters illegal in git branch names. It lives
// here rather than in the database package because both sides of the wire need
// it and the agent binary must not link that package.
func SanitizeBranchName(branch string) string {
	branch = strings.TrimSpace(branch)
	var b strings.Builder
	for _, r := range branch {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '/' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	res := b.String()
	for strings.Contains(res, "--") {
		res = strings.ReplaceAll(res, "--", "-")
	}
	res = strings.Trim(res, "-")
	return res
}
