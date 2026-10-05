# Tasks #730 - Desktop: global Sandbox settings with a project whitelist

Order matters: each step builds on the previous one. Tests go with the step
they cover.

## 1. Workstation settings

- [x] T1.1 Add `Defaults.ClaudeSandbox`, `Defaults.ClaudeSandboxProjects` and
      settings layout 4 (in place of `Seeded.SandboxFolded`, see plan.md) (`internal/agentconfig/workstation.go`).
- [x] T1.2 Carry the three fields through `overlay`
      (`internal/agentconfig/local.go`).
- Tests: `WriteSettings` / `ReadSettings` round trip of the new fields; an
  unknown key of the file kept; `overlay` keeps them from the top level and
  falls back to the base.

## 2. Resolution

- [x] T2.1 `Defaults.CoversProject(id)`: empty whitelist covers all.
- [x] T2.2 `Settings.ResolvedClaudeSandbox(id)`: global then project lists,
      deduplicated in order; project state over global; global ignored for an
      uncovered project; nil when nothing is stated.
- [x] T2.3 `projectClaudeSettings` (`internal/agent/agent_claude_sandbox.go`)
      writes the file from the resolved values.
- Tests: table test of the resolution (no values, global only, project only,
  both, uncovered project, state precedence On/Off/Inherited, duplicate across
  levels); a launch test asserting `--settings=` for a project with only
  global values; command line goldens unchanged with no values anywhere.

## 3. Fold

- [x] T3.1 `foldProjectSandboxes` (`internal/agentconfig/claude_sandbox.go`),
      called from `readConverted` (`internal/agentconfig/settings.go`); its
      change flag feeds `MigrateSettingsReport`; its warnings are logged at
      agent start.
- Tests: lists combined in sorted project order without duplicates; agreeing
  states moved up and cleared; diverging states kept and global left
  Inherited; emptied project sections dropped; an invalid entry left on its
  project with a warning; nothing to fold still sets the marker; a second
  read with the marker set changes nothing; a value added after the marker is
  not folded; `MigrateSettings` writes the `.bak-layout<n>` backup once.

## 4. Desktop API

- [x] T4.1 `GET /desktop/workstation/sandbox` returns `claudeSandbox`,
      `projects`, `platformSandbox` (in place of `workstationView` fields, see
      plan.md).
- [x] T4.2 `PUT /desktop/workstation` keeps the stored Sandbox fields.
- [x] T4.3 `PUT /desktop/workstation/sandbox` with the base-and-merge save and
      the whitelist.
- [x] T4.4 `GET /desktop/project` gains `claudeSandboxGlobal` and
      `claudeSandboxCovered`.
- [x] T4.5 `POST /desktop/project/sandbox/promote`.
- [x] T4.6 Project removal drops the project from the whitelist.
- Tests: an Execution save leaves the Sandbox values untouched; a sandbox save
  keeps an entry the store gained since its base; an invalid entry refused
  with 400; the whitelist trimmed, deduplicated and restricted to known
  projects; promote moves the rule, does not duplicate it, 404 on an unknown
  rule; removal empties the whitelist entry; the project payload for a covered
  and an uncovered project.

## 5. Desktop

- [x] T5.1 `sandboxSettings` options `inherited` and `onPromote`; inherited
      entries marked and not removable; state hint; overlap warning on the
      union (`desktop/src/sandbox-settings.mjs`).
- [x] T5.2 `whitelistEditor` with its hints.
- [x] T5.3 Workstation "Sandbox" category in `SETTINGS_CATEGORIES`, its save
      and reload (`desktop/src/main.js`); the two API calls in
      `desktop/electron/preload.cjs` and `desktop/electron/main.cjs`.
- [x] T5.4 Project panel: inherited values, "Move to global", the covered or
      not covered hint, the preview from the resolved values.
- Tests: unit tests of the module helpers (union for the preview, overlap on
  the union); `desktop/tests/sandbox-settings.ui.cjs` extended: set global
  values and reopen; check one project and see the other uncovered; a covered
  project shows inherited entries without a Remove button; "Move to global"
  moves a rule; Windows disables the sandbox part of both levels. Build with
  `npx vite build` first, run unsandboxed (memories "Desktop UI tests need a
  build" and "need the sandbox off").

## 6. Documentation

- [x] T6.1 `docs/adrs/0050-claude-sandbox-values-have-a-workstation-level.md`;
      ADR 0048 status line "Amended by ADR 0050".
- [x] T6.2 `CHANGELOG.md`: the `Added` line of `plan.md` under
      `[Unreleased]`.
- [x] T6.3 Check `docs/` for a guide describing the project Sandbox category
      and add the workstation level there.

## 7. Verification

- [x] T7.1 `go test ./internal/agentconfig/... ./internal/agent/...` (outside
      the sandbox, memory "Go tests under the sandbox").
- [x] T7.2 `node --test` for the desktop unit tests, and the desktop UI suite.
- [ ] T7.3 Manual check on a copy of a settings file holding project Sandbox
      values from #700: start the agent, find them in the workstation
      category, the backup beside the file, and a launch of each project
      carrying them. Never on the dev data directory (memory "Never run a
      branch server on the dev DB").
