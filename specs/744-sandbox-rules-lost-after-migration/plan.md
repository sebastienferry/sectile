# Plan #744 - Workstation Sandbox allow rules lost after the #730 migration

## Stack

Go agent (`internal/agentconfig`, `internal/agent`). No Desktop, server,
database or migration change.

## Cause, as far as it is known

The Desktop save paths keep the folded rules (`MergeClaudeSandbox` only drops
an entry the dialog showed and the owner removed). The only known code path
that drops `defaults.claudeSandbox` is an agent built before #730: its
`Defaults` has no such field and `WriteSettings` replaces the owned key
`defaults` whole, stamping layout 3. Today a running agent then folds, on its
next read, whatever the projects hold into the emptied workstation level,
without a log line, because `readConverted` folds on every read.

## Design

### The high-water key

`WriteSettings` writes a top-level `maxLayout`: the highest of the file's
current `maxLayout`, its `layout` and `SettingsLayout`. It is not one of the
`ownedKeys`, so every agent since #305, which replaces only its owned keys,
keeps it when it saves. It is not a `Settings` field: only the write and the
start-up migration read it, from the raw file.

A file without `maxLayout` (every file written before this change) reads as
`maxLayout` = its `layout`.

### Fold at start only

`readConverted` keeps the engine conversion and the retired-provider drop and
no longer calls `foldProjectSandboxes`. `MigrateSettingsReport` calls it after
`readConverted`, only when `layout` and `maxLayout` are both below
`layoutSandboxWorkstation`. A file an agent of layout 4 once wrote is never
folded again: its project values were added per project on purpose ("Always
allow" writes there since #730).

Consequence: a running agent that reads a layout 3 file leaves the project
values on their projects and its next save stamps layout 4. The values still
apply, through `ResolvedClaudeSandbox`.

### Downgrade trace

A downgrade is a file whose `layout` is below its `maxLayout`.

- At start, `MigrateSettingsReport` already backs up any file below
  `SettingsLayout` as `settings.json.bak-layout<N>` (timestamp suffix when
  taken) before rewriting it. It now also reports `Downgraded` (the
  `maxLayout` the file had reached), and the agent logs it.
- On a save of a running agent, `WriteSettings` finds the downgrade in the raw
  file it reads anyway, copies that file with the same naming before
  replacing it, and logs it with `log.Printf` (the agentconfig package gains
  its first log line; the agent's log output is the process-wide one). The
  migration writes through an internal variant that skips this, since it has
  already backed up and reported.

Backup naming is shared in one helper (`backupSettingsFile`).

### Refusing newer settings

`ErrSettingsNewer` is exported. `WriteSettings` returns it, wrapped with the
two layouts and "update this Sectile agent", when `max(layout, maxLayout)` of
the file on disk is above `SettingsLayout`, before writing anything.
`UpdateSettings` passes it through; the Desktop handlers already answer a
failed save with a 500 carrying the message, which the Desktop shows as
"Not saved: …". `MigrateSettingsReport` returns without writing and reports
`NewerLayout`, which the agent logs.

### Log lines (English, like their neighbours in `internal/agent/agent.go`)

- `[Agent] Workstation settings were rewritten by an older Sectile agent (layout N after layout M); that file is kept beside them`
- `[Agent] Workstation settings were written by a newer Sectile agent (layout N); this agent does not change them: update it`

## Target files

- `internal/agentconfig/settings.go`: `readConverted`, `WriteSettings`,
  `writeSettings`, `ErrSettingsNewer`, `maxLayout` handling.
- `internal/agentconfig/engines_migration.go`: `MigrateSettingsReport`,
  `backupSettingsFile`.
- `internal/agentconfig/claude_sandbox.go`: doc comment of
  `foldProjectSandboxes`.
- `internal/agent/agent.go`: start-up log lines.
- Tests: `internal/agentconfig/claude_sandbox_global_test.go`,
  `internal/agentconfig/settings_test.go` (or a new
  `settings_layout_test.go`), `internal/agent/agent_desktop_sandbox_test.go`.
- `docs/contracts/server-agent-v1.md`, ADR 0050 (amended, not superseded),
  `CHANGELOG.md`.

## Rejected alternatives

- **Marker inside `defaults`**: an older agent replaces `defaults` whole and
  would drop it.
- **Keeping the fold on every read, gated by `maxLayout`**: it would still
  move values in a running agent with no log line; the owner chose start only.
- **Passthrough of unknown `defaults` keys** (Q2 C): it helps only agents
  built after it, like the refusal, and keeps an older agent writing a format
  it does not understand.
