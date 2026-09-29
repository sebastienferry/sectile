package models

import (
	"fmt"
	"strings"
)

// DefaultBranchNameFormat is the branch name format of a project that sets
// none (#621): "feat/" and the lower-case key slug, which is what Sectile
// created before the setting existed.
const DefaultBranchNameFormat = "feat/{key_lower}"

// BranchNamePlaceholders lists the placeholders a branch name format accepts.
var BranchNamePlaceholders = []string{"key", "key_lower", "title"}

// branchNameSeparators are dropped next to a {title} that renders empty, so
// that "feat/{key}-{title}" gives "feat/AUC-1234" rather than a dangling dash.
const branchNameSeparators = "-_./"

// The sample a format is rendered for when it is validated.
const (
	branchNameSampleKey   = "AUC-1234"
	branchNameSampleTitle = "Sample title"
)

type branchNameToken struct {
	literal     string
	placeholder string
}

// parseBranchNameFormat splits a format into literals and placeholders. The
// errors are shown to the user as they are, hence in French.
func parseBranchNameFormat(format string) ([]branchNameToken, error) {
	var tokens []branchNameToken
	rest := format
	for rest != "" {
		open := strings.IndexAny(rest, "{}")
		if open < 0 {
			tokens = append(tokens, branchNameToken{literal: rest})
			break
		}
		if rest[open] == '}' {
			return nil, fmt.Errorf("format de nom de branche invalide : accolade fermante sans accolade ouvrante")
		}
		if open > 0 {
			tokens = append(tokens, branchNameToken{literal: rest[:open]})
		}
		end := strings.IndexAny(rest[open+1:], "{}")
		if end < 0 || rest[open+1+end] == '{' {
			return nil, fmt.Errorf("format de nom de branche invalide : accolade non fermée")
		}
		name := rest[open+1 : open+1+end]
		known := false
		for _, placeholder := range BranchNamePlaceholders {
			known = known || name == placeholder
		}
		if !known {
			return nil, fmt.Errorf("format de nom de branche invalide : placeholder inconnu {%s} (acceptés : {key}, {key_lower}, {title})", name)
		}
		tokens = append(tokens, branchNameToken{placeholder: name})
		rest = rest[open+1+end+1:]
	}
	return tokens, nil
}

// branchKeySlug keeps letters, digits and dashes of a task key, every other
// rune becoming a dash, and trims the dashes at both ends: "#621" gives "621".
// It is the rule the agent applied before formats existed, so the default
// format renders exactly the branch names it rendered then.
func branchKeySlug(key string) string {
	return strings.Trim(strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return '-'
	}, key), "-")
}

// TaskBranchName renders a branch name format (empty means the default one)
// for a task key and title.
func TaskBranchName(format, key, title string) (string, error) {
	format = strings.TrimSpace(format)
	if format == "" {
		format = DefaultBranchNameFormat
	}
	tokens, err := parseBranchNameFormat(format)
	if err != nil {
		return "", err
	}
	keySlug := branchKeySlug(key)
	if keySlug == "" {
		return "", fmt.Errorf("task key cannot produce a branch name")
	}
	var out string
	// trimNext is set by an empty {title} that has nothing before it on its
	// path segment: the separators that follow it are dropped instead.
	trimNext := false
	for _, token := range tokens {
		value := token.literal
		switch token.placeholder {
		case "key":
			value = keySlug
		case "key_lower":
			value = strings.ToLower(keySlug)
		case "title":
			value = titleSlug(title)
			if value == "" {
				if out == "" || strings.HasSuffix(out, "/") {
					trimNext = true
				} else {
					out = strings.TrimRight(out, branchNameSeparators)
				}
				continue
			}
		}
		if trimNext {
			value = strings.TrimLeft(value, branchNameSeparators)
			trimNext = value == ""
		}
		out += value
	}
	if trimNext {
		out = strings.TrimRight(out, branchNameSeparators)
	}
	return out, nil
}

// ValidateBranchNameFormat checks a format before a project stores it: known
// placeholders only, a key placeholder so that two tasks never render the same
// branch, and a sample render that Git accepts as a branch other than the
// default ones. The empty format is the default and is valid. The error is
// shown to the user as it is, hence in French.
func ValidateBranchNameFormat(format string) error {
	format = strings.TrimSpace(format)
	if format == "" {
		return nil
	}
	tokens, err := parseBranchNameFormat(format)
	if err != nil {
		return err
	}
	hasKey := false
	for _, token := range tokens {
		hasKey = hasKey || token.placeholder == "key" || token.placeholder == "key_lower"
	}
	if !hasKey {
		return fmt.Errorf("le format de nom de branche doit contenir {key} ou {key_lower}")
	}
	sample, err := TaskBranchName(format, branchNameSampleKey, branchNameSampleTitle)
	if err != nil {
		return err
	}
	if !ValidBranchName(sample) || sample == "main" || sample == "master" || sample == "HEAD" {
		return fmt.Errorf("le format de nom de branche donne « %s », qui n'est pas un nom de branche Git utilisable", sample)
	}
	return nil
}

// ValidBranchName applies the rules of `git check-ref-format --branch` that a
// rendered branch name can break, so the server can check a format without
// running Git. The agent still runs Git's own check before creating a branch.
func ValidBranchName(name string) bool {
	if name == "" || name == "@" || name == "HEAD" || strings.HasPrefix(name, "-") || strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") || strings.HasSuffix(name, ".") {
		return false
	}
	if strings.Contains(name, "..") || strings.Contains(name, "@{") || strings.Contains(name, "//") {
		return false
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(" ~^:?*[\\", r) {
			return false
		}
	}
	for _, component := range strings.Split(name, "/") {
		if strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".lock") {
			return false
		}
	}
	return true
}
