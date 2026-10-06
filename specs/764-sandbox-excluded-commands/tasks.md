# Tasks #764 - Claude settings: support sandbox excludedCommands

Order matters: each step leaves the tree buildable. Tests go with the step
they cover.

## 1. Agent

- [x] T1.1 `ClaudeSandbox.ExcludedCommands` and its handling in `IsZero`,
      `NormalizeClaudeSandbox`, `MergeClaudeSandbox`, `ResolvedClaudeSandbox`
      and `claudeSettingsDocument`.
- [x] T1.2 `claudeSandboxPayload` returns the list.
- Tests (`internal/agentconfig`, `internal/agent`): US1.1 to US1.6, the
  payload holding `excludedCommands: []` when unset.

## 2. Desktop catalogue

- [x] T2.1 `excludedCommands` on every preset and in `LISTS`; the new preset;
      the four exact force rules.
- Tests (`desktop/tests/claude-presets.test.mjs`): US3, US4.2 with a matcher
  of Claude Code's `Bash(...)` wildcard syntax, the Windows lists unchanged.

## 3. Desktop panel

- [x] T3.1 `sandboxPayload`, `fromStored`, `resolvedValues`,
      `launchesGetSettings`.
- [x] T3.2 The row "Commands outside the sandbox" and the preset line and
      entries.
- Tests: `desktop/tests/sandbox-settings.test.mjs` (US2.4, US2.5, the
  resolved union), `desktop/tests/sandbox-settings.ui.cjs` (US2.1, US2.3).

## 4. Documentation

- [x] T4.1 `docs/CAPABILITIES.md`.
- [x] T4.2 `CHANGELOG.md`.

## 5. Checks

- [x] T5.1 `go build ./...`, `go vet`, Go tests, desktop unit tests, desktop
      UI test of the panel.
