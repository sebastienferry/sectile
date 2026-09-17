package agentconfig

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// hookScripts carries the managed hook bodies into the binary. They are shell,
// not Go, because the only client that runs them is Claude Code, which invokes a
// command and reads its exit code.
//
//go:embed hooks/notification.sh hooks/stop.sh
var hookScripts embed.FS

// Where the hooks live and which Claude event each answers. Both paths are
// relative to Home, like every other managed destination, so the write stays
// inside the one guarded root.
const (
	claudeHookDir      = ".claude/hooks"
	claudeSettingsFile = ".claude/settings.json"
)

// claudeHooks pairs a Claude Code event with the script Sectile installs for it.
// The file names carry the sectile- prefix: it is what tells a Sectile-owned
// registration from a hook the user wrote themselves, when the settings file is
// merged.
var claudeHooks = []struct {
	Event  string
	File   string
	Source string
}{
	{Event: "Notification", File: "sectile-notification.sh", Source: "hooks/notification.sh"},
	{Event: "Stop", File: "sectile-stop.sh", Source: "hooks/stop.sh"},
}

// hookFiles returns the managed hook scripts by destination path. Only Claude
// Code has a hook convention Sectile supports, so every other provider installs
// none, which is a valid installation rather than an error.
func hookFiles(provider string) (map[string]string, error) {
	files := map[string]string{}
	if provider != "claude" {
		return files, nil
	}
	for _, hook := range claudeHooks {
		raw, err := hookScripts.ReadFile(hook.Source)
		if err != nil {
			return nil, err
		}
		files[filepath.Join(claudeHookDir, hook.File)] = string(raw)
	}
	return files, nil
}

// managedHookPath guards the manifest exactly as managedPath does for skills:
// only the two hook destinations Sectile owns may be recorded or retired.
func managedHookPath(p string) bool {
	for _, hook := range claudeHooks {
		if filepath.ToSlash(p) == claudeHookDir+"/"+hook.File {
			return true
		}
	}
	return false
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
	for _, hook := range claudeHooks {
		command := filepath.Join(home, claudeHookDir, hook.File)
		groups, _ := hooks[hook.Event].([]any)
		hooks[hook.Event] = mergeHookGroups(groups, hook.File, command)
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

// mergeHookGroups replaces the group Sectile already owns for this event, or
// appends one when there is none. Ownership is read from the script file name,
// not from the full command, so a workstation whose home directory moved is
// updated in place rather than gaining a second, dead entry.
func mergeHookGroups(groups []any, file, command string) []any {
	entry := map[string]any{
		"matcher": "",
		"hooks":   []any{map[string]any{"type": "command", "command": command}},
	}
	for index, group := range groups {
		if ownsHookGroup(group, file) {
			groups[index] = entry
			return groups
		}
	}
	return append(groups, entry)
}

// ownsHookGroup reports whether a registration already present is the one
// Sectile writes, by the managed script it points at.
func ownsHookGroup(group any, file string) bool {
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
		if command, _ := entry["command"].(string); strings.HasSuffix(command, "/"+file) {
			return true
		}
	}
	return false
}

// executableHooks marks the installed scripts executable. Scaffold writes every
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
