package models

import "strings"

// MacroWorkspace is where a macro's specification is written: the checkout,
// the branch it carries, whether that checkout is a dedicated worktree, and
// what the caller should know about how it was obtained. The agent prepares
// it; the server relays it to the MCP tool that asked.
type MacroWorkspace struct {
	Path     string `json:"path"`
	Branch   string `json:"branch"`
	Worktree bool   `json:"worktree"`
	Warning  string `json:"warning,omitempty"`
	// ProjectID, MacroKey and Todos are filled by the server when it relays the
	// answer, so the skill has the slicing it aligns on without another call.
	ProjectID string      `json:"projectId,omitempty"`
	MacroKey  string      `json:"macroKey,omitempty"`
	Todos     []MacroTodo `json:"todos,omitempty"`
}

// macroBranchSlugMax bounds the title part of a macro branch name, so that a
// long macro title does not produce a branch name nobody can type.
const macroBranchSlugMax = 30

// MacroBranchMatches reports whether a branch reference belongs to a macro:
// its last path segment is the macro key, or starts with "<KEY>-", ignoring
// case. A remote ("origin/...") or a type prefix ("feat/...") is skipped that
// way. It lives here because the server (slicing read) and the agent (macro
// worktree) must agree on it, and the agent binary must not link the database.
func MacroBranchMatches(ref, macroKey string) bool {
	key := strings.ToLower(strings.TrimSpace(macroKey))
	ref = strings.TrimSpace(ref)
	if key == "" || ref == "" {
		return false
	}
	last := ref
	if idx := strings.LastIndex(ref, "/"); idx >= 0 {
		last = ref[idx+1:]
	}
	lower := strings.ToLower(last)
	return lower == key || strings.HasPrefix(lower, key+"-")
}

// MacroBranchName is the branch a macro's specification is written on when
// none exists yet: "<KEY>-<slug of the title>", the slug lower-case and at most
// macroBranchSlugMax characters. A title that yields no slug gives the bare key.
func MacroBranchName(macroKey, title string) string {
	key := strings.ToUpper(strings.TrimSpace(macroKey))
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > macroBranchSlugMax {
		slug = strings.Trim(slug[:macroBranchSlugMax], "-")
	}
	if slug == "" {
		return key
	}
	return key + "-" + slug
}
