# Plan #700 - Desktop: sandbox configuration

Behaviour: `spec.md`. This file holds the implementation choices.

## Stack

- Agent (Go): `internal/agentconfig` (workstation settings),
  `internal/agent` (command lines, conversation approvals, headless runs,
  Desktop API).
- Desktop (vanilla JS, Electron): `desktop/src/main.js` (project settings
  dialog), `desktop/src/command-preview.mjs`.
- No server, database, migration or tracker change.

## Data model

New field of `agentconfig.ProjectSettings` (`internal/agentconfig/workstation.go`),
stored under `projectSettings.<projectId>` of `~/.config/sectile/settings.json`:

```go
// ClaudeSandbox holds what this project's Claude Code sessions are allowed
// (#700). It is handed to every built-in Claude line through --settings.
ClaudeSandbox *ClaudeSandbox `json:"claudeSandbox,omitempty"`

type ClaudeSandbox struct {
    Enabled        *bool    `json:"enabled,omitempty"`        // nil: inherited
    AllowedDomains []string `json:"allowedDomains,omitempty"`
    AllowWrite     []string `json:"allowWrite,omitempty"`
    Allow          []string `json:"allow,omitempty"`
    Deny           []string `json:"deny,omitempty"`
}
```

- `ClaudeSandbox.IsZero()` is true when every field is empty;
  `ProjectSettings.IsZero()` takes it into account so an emptied section is
  dropped.
- `projectSettings` is already an owned key of `WriteSettings`, replaced as a
  whole, so no new owned key is needed. Check that `localProjectRoot` /
  `WithRepositoryFile` carry the field through (memory "New Overrides key
  needs a merge": the copy is by known field).
- Helpers: `(*ClaudeSandbox).AddAllow(rules ...string) bool` (trim, dedupe,
  keep order, report a change), and a normaliser used by the Desktop save.

## Generated settings file

`agentconfig.ClaudeSettingsFile(projectID string, sandbox ClaudeSandbox) (string, error)`:

- Path: `<dir of SettingsPath()>/claude/<projectID>.json`, i.e.
  `~/.config/sectile/claude/<projectID>.json`: outside every repository and
  worktree, so neither committed nor removed at handoff. Directory `0700`,
  file `0600`, written atomically (temp file + rename, as `writeDesktopInfo`
  does).
- Content, in Claude Code's settings shape, only the keys that are set:

```json
{
  "sandbox": {
    "enabled": true,
    "network": { "allowedDomains": ["registry.npmjs.org"] },
    "filesystem": { "allowWrite": ["~/.cache/go-build"] }
  },
  "permissions": { "allow": ["Bash(make test:*)"], "deny": ["Bash(git push:*)"] }
}
```

- On Windows the `sandbox` object is omitted; `permissions` is written.
- Returns `""` and writes nothing when the value is zero; a stale file of a
  project emptied since is removed.
- Rewritten at each launch from the current values, so a running session keeps
  the file it read at start (Claude reads `--settings` once) and the next
  launch sees the new values.

## Command lines

One argument, `--settings=<path>`, the `=` form for the reason `addDirArgs`
documents. Carried by `agentCommandContext` as a new field `ClaudeSettings
string`, filled by the launcher from the resolved project settings.

- `headlessCommandLine` (`internal/agent/agent_config.go`): for `claude`,
  append `settingsArg` after `dirFlags`. `bypassPermissions` stays.
- `modeCommandLine`, interactive `claude` branch: append it the same way.
- `expandConfiguredTemplate`: untouched (custom template, FR9).
- `claudeConversationCommand` (`internal/agent/agent_conversation.go`): new
  parameter `settings string`; when non-empty, `args = append(args,
  "--settings="+settings)` (no shell, no quoting). Callers pass the file
  generated for the conversation's project.
- Other providers: nothing (same switch as `addDirArgs`).
- Any new `exec.Command` goes through `agentexec.Hidden` (none is expected).

Where the launchers resolve the project settings today (`localProjectRoot`
returns them), they call `ClaudeSettingsFile` once per launch and put the
path in the context. A write failure fails the launch with a French message in
the activity (`Impossible d'écrire les réglages du bac à sable : <err>`),
rather than launching without the owner's deny rules.

## "Always allow"

`approvalResponse` (`internal/agent/agent_conversation_approvals.go`) hands
`permission_suggestions` back verbatim today, so Claude writes the rule where
it chooses, usually `localSettings`, the worktree's
`.claude/settings.local.json`. New behaviour for `decision == "always"`:

1. Decode the suggestions as a list of Claude `PermissionUpdate` objects
   (`{type, rules:[{toolName, ruleContent}], behavior, destination}` for
   `addRules`; `setMode`, `addDirectories`... otherwise).
2. For each `addRules` with `behavior == "allow"`: render each rule as
   `toolName` or `toolName(ruleContent)` and collect it.
3. Rewrite every update's `destination` to `"session"`, so the running
   conversation applies it and Claude writes no file (FR7, US4.6).
4. After the response is sent, add the collected rules to the project's
   `ClaudeSandbox.Allow` (`ReadSettings`, `AddAllow`, `WriteSettings`, under
   the same lock the Desktop save uses) and write a trace notice in the
   conversation (`Règle ajoutée aux réglages Sandbox du projet : <rule>`).
5. A suggestion that does not decode is passed on with `destination:
   "session"` when it is an object carrying one, verbatim otherwise, and
   logged; nothing is added to the project.

The run knows its project (`run.projectID` or equivalent on `controlledRun`);
the approval path gets it from the run.

## Headless refusals

`internal/agent/agent_headless.go` already parses the `stream-json` result
message. Read its `permission_denials` array (`tool_name`, `tool_input`) and,
for each entry, append one activity line:
`Refusé par Claude Code : <tool>(<command or url>) · autorisez-le dans les
réglages Sandbox du projet`. Bound the list (first 10, then a count). A Bash
call that the sandbox blocked without a permission denial shows only in its
tool output: the line is then not produced, which the activity already shows
as the command's own error.

## Desktop

- `PROJECT_SETTINGS_CATEGORIES` (`desktop/src/main.js`): add
  `{id:'Sandbox',label:'Sandbox',saves:true,icon:...}` after Execution.
- Panel content, built with the existing `settingRow` helper:
  - "Claude Code sandbox": segmented Inherited / On / Off, a list editor
    for allowed domains and one for writable paths. Disabled on Windows
    (`api.platform === 'win32'` or the existing platform probe) with the hint
    of US6.
  - "Permission rules": list editors for Allow and Deny; a warning line when
    a rule is in both lists.
  - A hint on the merge (FR6) and on launches not covered (FR9).
- List editor: an input, an "Add" button, and removable chips; a small local
  helper if none exists already (check before writing one).
- Save: `api.mapProject` gains `claudeSandbox`; the handler of
  `desktopProject` (`internal/agent/agent_desktop.go`, the PUT path that reads
  `input.SpecArtifacts`) validates and stores it; the GET payload returns it
  with the other fields (`claudeSandbox`, `platformSandbox: runtime.GOOS !=
  "windows"`).
- After "Always allow", the next opening of the category reads the fresh
  payload, so the added rule shows (US4.3).
- `desktop/src/command-preview.mjs`: the built-in Claude lines accept an
  optional settings path and append `--settings=<path>`; the Execution
  preview passes it when the project has values. `web/src/lib/commandTemplate.ts`
  stays unchanged: the web has no workstation settings, and its preview
  already omits the other workstation-only argument, `--add-dir`.

## To verify at implementation

1. The sandbox keys of the installed Claude Code (2.1.x): `sandbox.enabled`,
   `sandbox.network.allowedDomains`, `sandbox.filesystem.allowWrite`. Check
   against Claude Code's settings JSON schema; rename the generated keys if
   they differ, the stored Sectile keys stay as above.
2. The shape of `permission_suggestions` and the acceptance of
   `destination: "session"`: capture one control request from a real
   conversation (a Bash call outside the rules) and keep it as a test
   fixture.

## Rejected alternatives

- Writing into `~/.claude/settings.json`: applies to every Claude session on
  the machine, outside Sectile, and edits a file Sectile does not own.
- Copying into each worktree's `.claude/settings.local.json`: the file would
  be removed at handoff, or committed by mistake when the checkout is used
  directly.
- One `--settings` value passed as inline JSON: it would show the rules in
  the process list and in the terminal command line, and needs shell quoting.
- Keeping Claude's destination for "Always allow": it is what loses the rule.

## Files

- `internal/agentconfig/workstation.go`, a new `internal/agentconfig/claude_sandbox.go` (+ tests)
- `internal/agent/agent_config.go`, `agent_conversation.go`,
  `agent_conversation_approvals.go`, `agent_headless.go`, `agent_desktop.go` (+ tests)
- `desktop/src/main.js`, `desktop/src/command-preview.mjs` (+ tests)
- `CHANGELOG.md`
