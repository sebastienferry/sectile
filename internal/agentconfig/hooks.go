package agentconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Where the hook is registered, and what it is registered as.
const (
	// claudeHookDir held the installed hook scripts. Nothing is written there
	// any more — the hook is a subcommand of the agent binary — but the name
	// stays, because the manifest must still be allowed to retire what earlier
	// releases put in it.
	claudeHookDir      = ".claude/hooks"
	claudeSettingsFile = ".claude/settings.json"
	// claudeHookSubcommand is both the argument that makes the agent binary
	// answer a hook and the marker that tells a Sectile-owned registration from
	// one the user wrote. It is read from the argument rather than from the
	// executable because the binary has no stable name: `make build-agent`
	// produces `agent`, the desktop package ships `sectile-agent`, and either
	// may be renamed or moved.
	claudeHookSubcommand = "sectile-hook"
)

// claudeHookEvents are the Claude Code events the hook answers. Together they
// bracket a wait from both sides: Notification and Stop open it, and the other
// three close it, because each of them only fires while the agent is working.
// The hook reads the event from its payload, so one registration per event is
// all the settings need to carry.
var claudeHookEvents = []string{"Notification", "Stop", "UserPromptSubmit", "PreToolUse", "PostToolUse"}

// retiredClaudeHookFiles are the scripts earlier releases installed: one per
// event first, then the single `sectile-hook.sh` that replaced them, and now
// none at all. They stay recognised for two reasons: a manifest that records
// them must stay readable so the refresh retires the files, and the settings
// merge must drop their registrations rather than leave dead entries pointing
// at scripts that are gone.
var retiredClaudeHookFiles = []string{"sectile-notification.sh", "sectile-stop.sh", "sectile-hook.sh"}

// claudeHookCommand renders the command a registration carries: the agent
// binary, quoted for the shell that will read it, then the subcommand. Claude
// Code hands the string to the host shell, so the quoting differs per platform,
// and goos is a parameter rather than runtime.GOOS so both forms can be
// asserted from one host.
func claudeHookCommand(executable, goos string) string {
	if goos == "windows" {
		// A Windows path cannot contain a double quote, so there is nothing to
		// escape inside; the quotes are what protects `C:\Program Files\...`.
		return `"` + executable + `" ` + claudeHookSubcommand
	}
	return "'" + strings.ReplaceAll(executable, "'", `'\''`) + "' " + claudeHookSubcommand
}

// managedHookPath guards the manifest exactly as managedPath does for skills:
// only the hook destinations Sectile owned may be recorded or retired.
func managedHookPath(p string) bool {
	_, owned := sectileHookFile(filepath.ToSlash(p))
	return owned && filepath.ToSlash(filepath.Dir(p)) == claudeHookDir
}

// sectileHookFile reports whether a path names a script Sectile once installed,
// and which one. Ownership is read from the file name, not from the full path,
// so a workstation whose home directory moved has its entry retired rather than
// left behind.
func sectileHookFile(path string) (string, bool) {
	name := filepath.Base(filepath.FromSlash(path))
	for _, retired := range retiredClaudeHookFiles {
		if name == retired {
			return name, true
		}
	}
	return "", false
}

// sectileHookRegistration reports whether a command already present is one
// Sectile wrote, now or in an earlier release: a command ending in the hook
// subcommand, or one naming a retired script. Neither test reads the path, so a
// workstation whose home directory or agent location moved is updated in place
// rather than gaining a second, dead entry.
func sectileHookRegistration(command string) bool {
	fields := strings.Fields(command)
	if len(fields) > 1 && fields[len(fields)-1] == claudeHookSubcommand {
		return true
	}
	_, owned := sectileHookFile(command)
	return owned
}

// registerClaudeHooks adds the Sectile hook entries to the user's Claude
// settings. The file belongs to the user: it is read, merged on the entries
// Sectile owns, and rewritten whole, so unrelated keys and third-party hooks
// survive. A file that cannot be parsed is left exactly as it is and reported;
// rewriting it would destroy configuration Sectile does not understand.
func registerClaudeHooks(fs *os.Root, command string) (string, error) {
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
// matcher, since the hook filters the payload itself, and one command.
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
// wrote, current or retired.
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
			if sectileHookRegistration(command) {
				return true
			}
		}
	}
	return false
}
