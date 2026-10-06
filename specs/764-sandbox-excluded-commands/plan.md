# Plan #764 - Claude settings: support sandbox excludedCommands

## Stack and boundaries

- Go agent (`internal/agentconfig`, `internal/agent`): storage, merge,
  resolution, generated file, desktop payload.
- Desktop (`desktop/src`): the list editor and the preset catalogue. The
  renderer in `desktop/src/main.js` passes the payload through and needs no
  change.
- No server, web or database change. No migration: `foldProjectSandboxes`
  only runs on layouts below 4, which cannot hold the new key.

## Data contract

`~/.config/sectile/settings.json`, `defaults.claudeSandbox` and
`projectSettings.<id>.claudeSandbox`:

```json
{"excludedCommands": ["git fetch *", "glab *"]}
```

Generated `~/.config/sectile/claude/<projectId>.json`, outside Windows:

```json
{"sandbox": {"excludedCommands": ["git fetch *", "glab *"]}}
```

Desktop payload (`claudeSandboxPayload`): `excludedCommands` always present,
`[]` when unset.

## Target files

### `internal/agentconfig/claude_sandbox.go`

- `ClaudeSandbox.ExcludedCommands []string \`json:"excludedCommands,omitempty"\``.
- `IsZero`, `NormalizeClaudeSandbox`, `MergeClaudeSandbox`,
  `ResolvedClaudeSandbox`: the new list beside `AllowedDomains`.
- `claudeSettingsDocument`: `sandbox["excludedCommands"]` when non-empty,
  inside the non-Windows branch.
- `foldProjectSandboxes` is left as is (comment "four lists" still true for
  the layouts it reads).

### `internal/agent/agent_claude_sandbox.go`

- `claudeSandboxPayload`: `"excludedCommands": list(value.ExcludedCommands)`.

### `desktop/src/sandbox-settings.mjs`

- `sandboxPayload`, `fromStored`, `resolvedValues`: the new key, as
  `allowedDomains`. `fromStored` defaults it to `[]`, so an older agent that
  does not send it reads as empty.
- `launchesGetSettings`: `values.excludedCommands.length>0` in the
  sandbox-only branch.
- `sandboxSettings`: an `excluded` list editor and its row after the domains
  row, disabled without a platform sandbox; `changed`, `showValues`, `set`
  and `sections` updated; preset line counts (`command`/`commands`) and the
  expanded entries group "Commands outside the sandbox"; the Windows note on
  a preset names commands too.

### `desktop/src/claude-presets.mjs`

- `excludedCommands:[]` on every existing preset, `LISTS` gains it (sandbox
  only, so `applicableLists` on Windows stays `allow`, `deny`).
- New preset `{id:'outside-sandbox',name:'Outside the sandbox (git, gh, glab)'}`
  after `glab`, before `dangerous`.
- "Dangerous actions": the four exact `--force` rules replace the two
  wildcard ones, in the same place.
- Header comment: five lists.

### Documentation

- `docs/CAPABILITIES.md`: the Claude settings entry names the list; a
  paragraph after "Autonomy in sandbox" on excluded commands versus
  `allowUnsandboxedCommands: true`.
- `CHANGELOG.md`: `Added` line for the list and the preset, `Changed` line
  for the force-with-lease rules with the manual removal.
- No new ADR: ADR 0048 already decides that Claude settings reach Claude
  through the generated file; a new key of that file follows it.

## Rejected alternatives

- Appending ` *` to a bare name: an exact match is a legitimate pattern, and
  rewriting what the owner typed hides Claude Code's syntax from them.
- Broad `git *` in the preset: every git command, hooks included, would leave
  the sandbox (clarification Q1).
- Rewriting the old force rules in existing settings (clarification Q2).

## Test plan

- Go (`go test ./internal/agentconfig/ ./internal/agent/`): normalization,
  merge, resolution, generated file with and without Windows, payload.
- Desktop unit (`node --test desktop/tests/claude-presets.test.mjs
  desktop/tests/sandbox-settings.test.mjs`).
- Desktop UI (`desktop/tests/sandbox-settings.ui.cjs`, after `npx vite build`,
  unsandboxed): the new row adds, saves and is disabled on Windows.
- Manual check on macOS (acceptance criterion), left to the owner.
