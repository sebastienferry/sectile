package agentconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// ClaudeSandbox is what a project's Claude Code sessions are allowed on this
// workstation (#700): Claude Code's sandbox and its permission rules. Every
// field is optional; an unset one leaves Claude Code's own settings to decide.
// A list adds to what those settings already hold: Claude Code merges the
// arrays of every settings source, so nothing here can remove an entry.
type ClaudeSandbox struct {
	// Optional booleans preserve explicit false values; nil inherits.
	AutoAllowBashIfSandboxed *bool    `json:"autoAllowBashIfSandboxed,omitempty"`
	AllowUnsandboxedCommands *bool    `json:"allowUnsandboxedCommands,omitempty"`
	AdditionalDirectories    []string `json:"additionalDirectories,omitempty"`
	Enabled                  *bool    `json:"enabled,omitempty"`
	AllowedDomains           []string `json:"allowedDomains,omitempty"`
	// ExcludedCommands are the commands Claude Code runs outside its sandbox
	// (#764), written as the inside of a Bash(...) rule: "glab *" matches
	// glab with or without arguments, "glab" only the bare command.
	ExcludedCommands []string `json:"excludedCommands,omitempty"`
	AllowWrite       []string `json:"allowWrite,omitempty"`
	Allow            []string `json:"allow,omitempty"`
	Deny             []string `json:"deny,omitempty"`
}

// IsZero reports values that state nothing, a nil pointer included.
func (c *ClaudeSandbox) IsZero() bool {
	return c == nil || (c.Enabled == nil && c.AutoAllowBashIfSandboxed == nil && c.AllowUnsandboxedCommands == nil && len(c.AdditionalDirectories) == 0 && len(c.AllowedDomains) == 0 && len(c.ExcludedCommands) == 0 && len(c.AllowWrite) == 0 &&
		len(c.Allow) == 0 && len(c.Deny) == 0)
}

// ErrEmptyEntry refuses an entry that is empty or only spaces.
var ErrEmptyEntry = errors.New("an entry is empty")

// NormalizeClaudeSandbox trims every entry and drops the duplicates of each
// list, keeping the order of entry. An empty entry is refused rather than
// dropped, so a mistake is reported instead of silently saved.
func NormalizeClaudeSandbox(c ClaudeSandbox) (ClaudeSandbox, error) {
	out := ClaudeSandbox{Enabled: c.Enabled, AutoAllowBashIfSandboxed: c.AutoAllowBashIfSandboxed, AllowUnsandboxedCommands: c.AllowUnsandboxedCommands}
	for _, list := range []struct {
		name string
		in   []string
		out  *[]string
	}{
		{"additionalDirectories", c.AdditionalDirectories, &out.AdditionalDirectories},
		{"allowedDomains", c.AllowedDomains, &out.AllowedDomains},
		{"excludedCommands", c.ExcludedCommands, &out.ExcludedCommands},
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
		Enabled:                  sent.Enabled,
		AutoAllowBashIfSandboxed: sent.AutoAllowBashIfSandboxed,
		AllowUnsandboxedCommands: sent.AllowUnsandboxedCommands,
		AdditionalDirectories:    merge(sent.AdditionalDirectories, base.AdditionalDirectories, stored.AdditionalDirectories),
		AllowedDomains:           merge(sent.AllowedDomains, base.AllowedDomains, stored.AllowedDomains),
		ExcludedCommands:         merge(sent.ExcludedCommands, base.ExcludedCommands, stored.ExcludedCommands),
		AllowWrite:               merge(sent.AllowWrite, base.AllowWrite, stored.AllowWrite),
		Allow:                    merge(sent.Allow, base.Allow, stored.Allow),
		Deny:                     merge(sent.Deny, base.Deny, stored.Deny),
	}
}

// claudeSettingsDocument is the values in Claude Code's settings shape, with
// only the keys that are set. Claude Code's sandbox does not run on Windows,
// where the sandbox object is left out and the rules still apply.
func claudeSettingsDocument(c ClaudeSandbox, goos string) map[string]any {
	document := map[string]any{}
	if goos != "windows" {
		sandbox := map[string]any{}
		if c.AutoAllowBashIfSandboxed != nil {
			sandbox["autoAllowBashIfSandboxed"] = *c.AutoAllowBashIfSandboxed
		}
		if c.AllowUnsandboxedCommands != nil {
			sandbox["allowUnsandboxedCommands"] = *c.AllowUnsandboxedCommands
		}
		if c.Enabled != nil {
			sandbox["enabled"] = *c.Enabled
		}
		if len(c.AllowedDomains) > 0 {
			sandbox["network"] = map[string]any{"allowedDomains": c.AllowedDomains}
		}
		if len(c.ExcludedCommands) > 0 {
			sandbox["excludedCommands"] = c.ExcludedCommands
		}
		if len(c.AllowWrite) > 0 {
			sandbox["filesystem"] = map[string]any{"allowWrite": c.AllowWrite}
		}
		if len(sandbox) > 0 {
			document["sandbox"] = sandbox
		}
	}
	permissions := map[string]any{}
	if len(c.AdditionalDirectories) > 0 {
		permissions["additionalDirectories"] = c.AdditionalDirectories
	}
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

// CoversProject reports whether the workstation Sandbox values apply to the
// project (#730): an empty whitelist covers every project, a project added
// later included.
func (d Defaults) CoversProject(projectID string) bool {
	if len(d.ClaudeSandboxProjects) == 0 {
		return true
	}
	for _, id := range d.ClaudeSandboxProjects {
		if strings.TrimSpace(id) == projectID {
			return true
		}
	}
	return false
}

// ResolvedClaudeSandbox is what a launch of the project applies (#730): the
// workstation values under the project's own when the whitelist covers the
// project, the project's alone otherwise. Each list is the workstation
// entries then the project's, without duplicates; the project's sandbox state
// speaks over the workstation's when it states one. Nil when nothing is
// stated.
func (s Settings) ResolvedClaudeSandbox(projectID string) *ClaudeSandbox {
	project := s.Project(projectID).ClaudeSandbox
	var global *ClaudeSandbox
	if s.Defaults.CoversProject(projectID) {
		global = s.Defaults.ClaudeSandbox
	}
	if global.IsZero() {
		if project.IsZero() {
			return nil
		}
		return project
	}
	if project.IsZero() {
		return global
	}
	out := ClaudeSandbox{
		Enabled:                  global.Enabled,
		AutoAllowBashIfSandboxed: global.AutoAllowBashIfSandboxed,
		AllowUnsandboxedCommands: global.AllowUnsandboxedCommands,
		AdditionalDirectories:    unionEntries(global.AdditionalDirectories, project.AdditionalDirectories),
		AllowedDomains:           unionEntries(global.AllowedDomains, project.AllowedDomains),
		ExcludedCommands:         unionEntries(global.ExcludedCommands, project.ExcludedCommands),
		AllowWrite:               unionEntries(global.AllowWrite, project.AllowWrite),
		Allow:                    unionEntries(global.Allow, project.Allow),
		Deny:                     unionEntries(global.Deny, project.Deny),
	}
	if project.AutoAllowBashIfSandboxed != nil {
		out.AutoAllowBashIfSandboxed = project.AutoAllowBashIfSandboxed
	}
	if project.AllowUnsandboxedCommands != nil {
		out.AllowUnsandboxedCommands = project.AllowUnsandboxedCommands
	}
	if project.Enabled != nil {
		out.Enabled = project.Enabled
	}
	return &out
}

// unionEntries is the entries of the lists in order, each once, compared
// trimmed.
func unionEntries(lists ...[]string) []string {
	var out []string
	seen := map[string]bool{}
	for _, list := range lists {
		for _, entry := range list {
			if key := strings.TrimSpace(entry); !seen[key] {
				seen[key] = true
				out = append(out, entry)
			}
		}
	}
	return out
}

// layoutSandboxWorkstation is the layout from which the Sandbox values have a
// workstation level (#730): an older file still holds them per project.
const layoutSandboxWorkstation = 4

// foldProjectSandboxes moves the Sandbox values the projects hold into the
// workstation values, once (#730): only a file written before the workstation
// level has them to move, and the next save stamps the current layout, so a
// value a project gains later stays with it. Only the start-up migration calls
// it, never an ordinary read (#744). The four lists of every project,
// in project ID order (the order of the settings file), join the workstation
// lists and leave the projects; a sandbox state goes up only when every
// project stating one states the same. The whitelist is left empty, so every
// project applies the result. An entry the Sandbox category would refuse
// stays on its project and is reported.
func foldProjectSandboxes(s *Settings) (bool, []string) {
	if s.Layout >= layoutSandboxWorkstation {
		return false, nil
	}
	ids := make([]string, 0, len(s.ProjectSettings))
	for id, project := range s.ProjectSettings {
		if project.ClaudeSandbox != nil {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return false, nil
	}
	global := ClaudeSandbox{}
	if s.Defaults.ClaudeSandbox != nil {
		global = *s.Defaults.ClaudeSandbox
	}
	var warnings []string
	var states []bool
	for _, id := range ids {
		project := s.Project(id)
		sandbox := *project.ClaudeSandbox
		if sandbox.Enabled != nil {
			states = append(states, *sandbox.Enabled)
		}
		for _, list := range []struct {
			name         string
			from, global *[]string
		}{
			{"allowedDomains", &sandbox.AllowedDomains, &global.AllowedDomains},
			{"allowWrite", &sandbox.AllowWrite, &global.AllowWrite},
			{"allow", &sandbox.Allow, &global.Allow},
			{"deny", &sandbox.Deny, &global.Deny},
		} {
			var kept []string
			for _, entry := range *list.from {
				if _, err := normalizeEntries([]string{entry}); err != nil {
					kept = append(kept, entry)
					warnings = append(warnings, fmt.Sprintf("project %s, %s: %v", id, list.name, err))
					continue
				}
				*list.global = unionEntries(*list.global, []string{strings.TrimSpace(entry)})
			}
			*list.from = kept
		}
		project.ClaudeSandbox = &sandbox
		s.SetProject(id, project)
	}
	if len(states) > 0 && allEqual(states) {
		global.Enabled = &states[0]
		for _, id := range ids {
			project := s.Project(id)
			if project.ClaudeSandbox != nil {
				sandbox := *project.ClaudeSandbox
				sandbox.Enabled = nil
				project.ClaudeSandbox = &sandbox
			}
			s.SetProject(id, project)
		}
	}
	for _, id := range ids {
		if project, ok := s.ProjectSettings[id]; ok && project.ClaudeSandbox.IsZero() {
			project.ClaudeSandbox = nil
			s.SetProject(id, project)
		}
	}
	s.Defaults.ClaudeSandbox = nil
	if !global.IsZero() {
		s.Defaults.ClaudeSandbox = &global
	}
	return true, warnings
}

func allEqual(states []bool) bool {
	for _, state := range states[1:] {
		if state != states[0] {
			return false
		}
	}
	return true
}
