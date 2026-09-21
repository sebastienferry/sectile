package agentconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Sectile once registered a Claude Code hook (#174): a script installed under
// ~/.claude/hooks and wired into ~/.claude/settings.json on five events, which
// reported whether a session was waiting for the user. It was withdrawn (#260).
// It made Sectile a writer of a file it otherwise only read, and it ran a
// process on every tool call of every Claude Code session on the workstation,
// launched by Sectile or not. What remains here is the retirement: a
// workstation that still carries the script and its registrations has both
// removed on its next setup, and nothing new is installed in their place.
const (
	claudeHookDir      = ".claude/hooks"
	claudeSettingsFile = ".claude/settings.json"
)

// retiredClaudeHookFiles are the scripts earlier releases installed under
// claudeHookDir: the two per-event scripts of the first release, then the one
// script that replaced them. They stay recognised for two reasons: a manifest
// that records them must stay readable so the refresh retires the files, and
// the settings cleanup must drop their registrations rather than leave dead
// entries that fail on every turn.
var retiredClaudeHookFiles = []string{"sectile-notification.sh", "sectile-stop.sh", "sectile-hook.sh"}

// retiredClaudeHookSubcommand is the trailing argument a pre-release build of
// #260 registered in place of a script, as `<agent binary> sectile-hook`. That
// build never shipped, but a workstation that ran it carries the registration,
// so it is recognised and dropped like the scripts.
const retiredClaudeHookSubcommand = "sectile-hook"

// managedHookPath guards the manifest exactly as managedPath does for skills:
// only the hook destinations Sectile once owned may be recorded or retired.
func managedHookPath(p string) bool {
	return sectileHookFile(filepath.ToSlash(p)) && filepath.ToSlash(filepath.Dir(p)) == claudeHookDir
}

// sectileHookFile reports whether a path or command names a script Sectile
// installed. Ownership is read from the file name, not from the full command,
// so a registration written under a former home directory is still recognised.
func sectileHookFile(command string) bool {
	name := filepath.Base(filepath.FromSlash(command))
	for _, retired := range retiredClaudeHookFiles {
		if name == retired {
			return true
		}
	}
	return false
}

// sectileHookRegistration reports whether a registered command is one Sectile
// wrote: a retired script, or the binary subcommand of the pre-release build.
func sectileHookRegistration(command string) bool {
	if sectileHookFile(command) {
		return true
	}
	fields := strings.Fields(command)
	return len(fields) > 1 && fields[len(fields)-1] == retiredClaudeHookSubcommand
}

// retireClaudeHooks removes every Sectile registration from the user's Claude
// settings. The file belongs to the user: it is read, stripped of the entries
// Sectile owns, and rewritten whole only when something was removed, so
// unrelated keys, third-party hooks and a file Sectile never touched are left
// exactly as they are. A file that cannot be parsed is left alone and reported;
// rewriting it would destroy configuration Sectile does not understand.
func retireClaudeHooks(fs *os.Root) (string, error) {
	raw, err := fs.ReadFile(claudeSettingsFile)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return "", nil
	}
	settings := map[string]any{}
	if err := json.Unmarshal(raw, &settings); err != nil {
		return "Claude settings left untouched, Sectile hooks not removed: " + claudeSettingsFile + " is not valid JSON", nil
	}
	hooks, _ := settings["hooks"].(map[string]any)
	changed := false
	for event, rawGroups := range hooks {
		groups, ok := rawGroups.([]any)
		if !ok {
			continue
		}
		kept := make([]any, 0, len(groups))
		for _, group := range groups {
			if !ownsHookGroup(group) {
				kept = append(kept, group)
			}
		}
		if len(kept) == len(groups) {
			continue
		}
		changed = true
		if len(kept) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = kept
		}
	}
	if !changed {
		return "", nil
	}
	if len(hooks) == 0 {
		delete(settings, "hooks")
	}
	encoded, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return "", err
	}
	encoded = append(encoded, '\n')
	return "", atomicWrite(fs, claudeSettingsFile, encoded)
}

// ownsHookGroup reports whether a registration is one Sectile wrote, by the
// command it points at.
func ownsHookGroup(group any) bool {
	values, _ := group.(map[string]any)
	entries, _ := values["hooks"].([]any)
	for _, raw := range entries {
		entry, _ := raw.(map[string]any)
		if command, _ := entry["command"].(string); command != "" && sectileHookRegistration(command) {
			return true
		}
	}
	return false
}
