package agentconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ClaudeSandbox is what a project's Claude Code sessions are allowed on this
// workstation (#700): Claude Code's sandbox and its permission rules. Every
// field is optional; an unset one leaves Claude Code's own settings to decide.
// A list adds to what those settings already hold: Claude Code merges the
// arrays of every settings source, so nothing here can remove an entry.
type ClaudeSandbox struct {
	// Enabled is a pointer because "off" is a statement: nil inherits.
	Enabled        *bool    `json:"enabled,omitempty"`
	AllowedDomains []string `json:"allowedDomains,omitempty"`
	AllowWrite     []string `json:"allowWrite,omitempty"`
	Allow          []string `json:"allow,omitempty"`
	Deny           []string `json:"deny,omitempty"`
}

// IsZero reports values that state nothing, a nil pointer included.
func (c *ClaudeSandbox) IsZero() bool {
	return c == nil || (c.Enabled == nil && len(c.AllowedDomains) == 0 && len(c.AllowWrite) == 0 &&
		len(c.Allow) == 0 && len(c.Deny) == 0)
}

// ErrEmptyEntry refuses an entry that is empty or only spaces.
var ErrEmptyEntry = errors.New("an entry is empty")

// NormalizeClaudeSandbox trims every entry and drops the duplicates of each
// list, keeping the order of entry. An empty entry is refused rather than
// dropped, so a mistake is reported instead of silently saved.
func NormalizeClaudeSandbox(c ClaudeSandbox) (ClaudeSandbox, error) {
	out := ClaudeSandbox{Enabled: c.Enabled}
	for _, list := range []struct {
		name string
		in   []string
		out  *[]string
	}{
		{"allowedDomains", c.AllowedDomains, &out.AllowedDomains},
		{"allowWrite", c.AllowWrite, &out.AllowWrite},
		{"allow", c.Allow, &out.Allow},
		{"deny", c.Deny, &out.Deny},
	} {
		normalized, err := normalizeEntries(list.in)
		if err != nil {
			return ClaudeSandbox{}, fmt.Errorf("%s: %w", list.name, err)
		}
		*list.out = normalized
	}
	return out, nil
}

func normalizeEntries(entries []string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			return nil, ErrEmptyEntry
		}
		if len(entry) > MaxCommandLength || strings.ContainsAny(entry, "\x00\r\n") {
			return nil, fmt.Errorf("entry %.40q is not a single line of at most %d characters", entry, MaxCommandLength)
		}
		if !seen[entry] {
			seen[entry] = true
			out = append(out, entry)
		}
	}
	return out, nil
}

// AddAllow appends the rules the allow list does not hold yet, trimmed and in
// order, and reports whether the list changed. Empty rules are skipped.
func (c *ClaudeSandbox) AddAllow(rules ...string) bool {
	seen := map[string]bool{}
	for _, rule := range c.Allow {
		seen[rule] = true
	}
	changed := false
	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if rule == "" || seen[rule] {
			continue
		}
		seen[rule] = true
		c.Allow = append(c.Allow, rule)
		changed = true
	}
	return changed
}

// MergeClaudeSandbox reconciles a save of the values with what the store holds
// now. sent is what the owner saves, base what the dialog read when it opened,
// stored the current values: an entry the store gained since base, such as a
// rule an "Always allow" added while the dialog was open, is kept, and only an
// entry the owner removed from base goes. The sandbox state is the owner's.
func MergeClaudeSandbox(sent, base, stored ClaudeSandbox) ClaudeSandbox {
	merge := func(sent, base, stored []string) []string {
		known := map[string]bool{}
		for _, entry := range base {
			known[strings.TrimSpace(entry)] = true
		}
		out := append([]string{}, sent...)
		have := map[string]bool{}
		for _, entry := range sent {
			have[strings.TrimSpace(entry)] = true
		}
		for _, entry := range stored {
			if key := strings.TrimSpace(entry); !known[key] && !have[key] {
				have[key] = true
				out = append(out, entry)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	return ClaudeSandbox{
		Enabled:        sent.Enabled,
		AllowedDomains: merge(sent.AllowedDomains, base.AllowedDomains, stored.AllowedDomains),
		AllowWrite:     merge(sent.AllowWrite, base.AllowWrite, stored.AllowWrite),
		Allow:          merge(sent.Allow, base.Allow, stored.Allow),
		Deny:           merge(sent.Deny, base.Deny, stored.Deny),
	}
}

// claudeSettingsDocument is the values in Claude Code's settings shape, with
// only the keys that are set. Claude Code's sandbox does not run on Windows,
// where the sandbox object is left out and the rules still apply.
func claudeSettingsDocument(c ClaudeSandbox, goos string) map[string]any {
	document := map[string]any{}
	if goos != "windows" {
		sandbox := map[string]any{}
		if c.Enabled != nil {
			sandbox["enabled"] = *c.Enabled
		}
		if len(c.AllowedDomains) > 0 {
			sandbox["network"] = map[string]any{"allowedDomains": c.AllowedDomains}
		}
		if len(c.AllowWrite) > 0 {
			sandbox["filesystem"] = map[string]any{"allowWrite": c.AllowWrite}
		}
		if len(sandbox) > 0 {
			document["sandbox"] = sandbox
		}
	}
	permissions := map[string]any{}
	if len(c.Allow) > 0 {
		permissions["allow"] = c.Allow
	}
	if len(c.Deny) > 0 {
		permissions["deny"] = c.Deny
	}
	if len(permissions) > 0 {
		document["permissions"] = permissions
	}
	return document
}

// claudeSettingsPath is where a project's generated Claude settings live:
// beside the workstation settings, outside every repository and worktree, so
// they are neither committed nor removed with a worktree.
func claudeSettingsPath(projectID string) (string, error) {
	if projectID == "" || projectID != filepath.Base(projectID) || strings.HasPrefix(projectID, ".") {
		return "", fmt.Errorf("invalid project id %q", projectID)
	}
	settings, err := SettingsPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(settings), "claude", projectID+".json"), nil
}

// ClaudeSettingsPath is where the project's generated Claude settings live,
// for the desktop to show the argument a launch adds.
func ClaudeSettingsPath(projectID string) (string, error) { return claudeSettingsPath(projectID) }

// ClaudeSettingsFile writes the settings file a Claude launch of the project
// is handed with --settings, and returns its path. It is rewritten at each
// launch from the current values: Claude reads it once at start, so a running
// session keeps what it started with. Values that state nothing, or nothing
// this platform applies, write no file, remove one left by earlier values,
// and return "".
func ClaudeSettingsFile(projectID string, sandbox *ClaudeSandbox) (string, error) {
	return writeClaudeSettingsFile(projectID, sandbox, runtime.GOOS)
}

func writeClaudeSettingsFile(projectID string, sandbox *ClaudeSandbox, goos string) (string, error) {
	path, err := claudeSettingsPath(projectID)
	if err != nil {
		return "", err
	}
	var document map[string]any
	if sandbox != nil {
		document = claudeSettingsDocument(*sandbox, goos)
	}
	if len(document) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		return "", nil
	}
	raw, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".claude-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(append(raw, '\n')); err != nil {
		file.Close()
		return "", err
	}
	if err = file.Close(); err != nil {
		return "", err
	}
	// CreateTemp already opens the file 0600.
	if err := os.Rename(file.Name(), path); err != nil {
		return "", err
	}
	return path, nil
}

// firstSandbox is overlay's rule for the sandbox values: the top level's when
// it states any, else the base's.
func firstSandbox(top, base *ClaudeSandbox) *ClaudeSandbox {
	if !top.IsZero() {
		return top
	}
	if base.IsZero() {
		return nil
	}
	return base
}
