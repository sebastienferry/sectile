package agentconfig

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// hookScripts carries the managed hook body into the binary. It is shell, not
// Go, because the only client that runs it is Claude Code, which invokes a
// command and reads its exit code.
//
//go:embed hooks/hook.sh
var hookScripts embed.FS

// Where the hook lives and how it is named. Both paths are relative to Home,
// like every other managed destination, so the write stays inside the one
// guarded root.
const (
	claudeHookDir      = ".claude/hooks"
	claudeSettingsFile = ".claude/settings.json"
	// claudeHookFile is the one script every registration points at. The name
	// carries the sectile- prefix: it is what tells a Sectile-owned registration
	// from a hook the user wrote themselves, when the settings file is merged.
	claudeHookFile   = "sectile-hook.sh"
	claudeHookSource = "hooks/hook.sh"
)

// claudeHookEvents are the Claude Code events the script answers. Together they
// bracket a wait from both sides: Notification and Stop open it, and the other
// three close it, because each of them only fires while the agent is working.
// The script reads the event from its payload, so one registration per event
// is all the settings need to carry.
var claudeHookEvents = []string{"Notification", "Stop", "UserPromptSubmit", "PreToolUse", "PostToolUse"}

// retiredClaudeHookFiles are the per-event scripts the first release installed.
// They stay recognised for two reasons: a manifest that records them must stay
// readable so the refresh retires the files, and the settings merge must drop
// their registrations rather than leave two dead entries pointing at nothing.
var retiredClaudeHookFiles = []string{"sectile-notification.sh", "sectile-stop.sh"}

// hookFiles returns the managed hook script by destination path. Only Claude
// Code has a hook convention Sectile supports, so every other provider installs
// none, which is a valid installation rather than an error.
func hookFiles(provider string) (map[string]string, error) {
	files := map[string]string{}
	if provider != "claude" {
		return files, nil
	}
	raw, err := hookScripts.ReadFile(claudeHookSource)
	if err != nil {
		return nil, err
	}
	files[filepath.Join(claudeHookDir, claudeHookFile)] = string(raw)
	return files, nil
}

// managedHookPath guards the manifest exactly as managedPath does for skills:
// only the hook destinations Sectile owns, or once owned, may be recorded or
// retired.
func managedHookPath(p string) bool {
	_, owned := sectileHookFile(filepath.ToSlash(p))
	return owned && filepath.ToSlash(filepath.Dir(p)) == claudeHookDir
}

// sectileHookFile reports whether a path or command names a script Sectile
// installs or installed, and which one. Ownership is read from the file name,
// not from the full command, so a workstation whose home directory moved is
// updated in place rather than gaining a second, dead entry.
func sectileHookFile(command string) (string, bool) {
	name := filepath.Base(filepath.FromSlash(command))
	if name == claudeHookFile {
		return name, true
	}
	for _, retired := range retiredClaudeHookFiles {
		if name == retired {
			return name, true
		}
	}
	return "", false
}

// registerClaudeHooks adds the Sectile hook entries to the user's Claude
// settings. The file belongs to the user: it is read, merged on the entries
// Sectile owns, and rewritten whole, so unrelated keys and third-party hooks
// survive. A file that cannot be parsed is left exactly as it is and reported;
// rewriting it would destroy configuration Sectile does not understand.
func registerClaudeHooks(fs *os.Root, home string) (string, error) {
	settings := map[string]any{}
	raw, err := fs.ReadFile(claudeSettingsFile)
	switch {
	case err == nil:
		if len(strings.TrimSpace(string(raw))) > 0 {
			if err := json.Unmarshal(raw, &settings); err != nil {
				return "Claude settings left untouched, hooks not registered: " + claudeSettingsFile + " is not valid JSON", nil
			}
		}
	case os.IsNotExist(err):
	default:
		return "", err
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	command := filepath.Join(home, claudeHookDir, claudeHookFile)
	// Every event is visited, not only the ones registered today: a Sectile
	// entry on an event this release no longer answers is a dead entry, and
	// a retired script's registration is one too.
	for event, raw := range hooks {
		groups, ok := raw.([]any)
		if !ok {
			continue
		}
		merged := mergeHookGroups(groups, command, slices.Contains(claudeHookEvents, event))
		if len(merged) == 0 {
			delete(hooks, event)
			continue
		}
		hooks[event] = merged
	}
	for _, event := range claudeHookEvents {
		if _, present := hooks[event]; !present {
			hooks[event] = []any{sectileHookGroup(command)}
		}
	}
	settings["hooks"] = hooks

	encoded, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return "", err
	}
	encoded = append(encoded, '\n')
	if err := atomicWrite(fs, claudeSettingsFile, encoded); err != nil {
		return "", err
	}
	return "", nil
}

// sectileHookGroup is the registration Sectile writes for one event: no
// matcher, since the script filters the payload itself, and one command.
func sectileHookGroup(command string) map[string]any {
	return map[string]any{
		"matcher": "",
		"hooks":   []any{map[string]any{"type": "command", "command": command}},
	}
}

// mergeHookGroups rewrites the groups of one event. Third-party groups are kept
// where they are. The first Sectile-owned group is replaced by the current
// registration when the event is one Sectile answers, and every other
// Sectile-owned group is dropped; an event Sectile answers and that had no
// Sectile group gets one appended, after the user's own.
func mergeHookGroups(groups []any, command string, wanted bool) []any {
	merged := make([]any, 0, len(groups)+1)
	placed := false
	for _, group := range groups {
		if !ownsHookGroup(group) {
			merged = append(merged, group)
			continue
		}
		if wanted && !placed {
			merged = append(merged, sectileHookGroup(command))
			placed = true
		}
	}
	if wanted && !placed {
		merged = append(merged, sectileHookGroup(command))
	}
	return merged
}

// ownsHookGroup reports whether a registration already present is one Sectile
// wrote, current or retired, by the managed script it points at.
func ownsHookGroup(group any) bool {
	values, _ := group.(map[string]any)
	if values == nil {
		return false
	}
	entries, _ := values["hooks"].([]any)
	for _, raw := range entries {
		entry, _ := raw.(map[string]any)
		if entry == nil {
			continue
		}
		if command, _ := entry["command"].(string); command != "" {
			if _, owned := sectileHookFile(command); owned {
				return true
			}
		}
	}
	return false
}

// executableHooks marks the installed script executable. Scaffold writes every
// managed file 0600, which is right for a skill and useless for a script Claude
// Code has to run.
func executableHooks(fs *os.Root, files map[string]string) error {
	for path := range files {
		if !managedHookPath(path) {
			continue
		}
		if err := fs.Chmod(path, 0700); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("hook %s could not be made executable: %w", path, err)
		}
	}
	return nil
}
