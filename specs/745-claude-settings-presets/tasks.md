# Tasks #745 - Claude settings and preset rules

Order matters: each step builds on the previous one. Tests go with the step
they cover.

## 1. Rename

- [ ] T1.1 Category labels in `SETTINGS_CATEGORIES` and
      `PROJECT_SETTINGS_CATEGORIES`, plus the save button, notices and
      unavailable texts of `workstationSandboxPanel` (`desktop/src/main.js`).
- [ ] T1.2 The strings of `desktop/src/sandbox-settings.mjs` listed in plan.md.
- [ ] T1.3 The agent strings listed in plan.md (`internal/agent`), the French
      headless line kept in French.
- [ ] T1.4 `docs/CAPABILITIES.md`, the two category lines.
- Tests: update `desktop/tests/settings-version.ui.cjs` (tab names),
  `desktop/tests/sandbox-settings.ui.cjs` (button, notice, whitelist group
  name) and any Go test asserting a renamed text (`grep -rn "Sandbox settings"
  --include='*_test.go' internal`). A test asserts the state control is still
  named "Claude Code sandbox".

## 2. Catalogue

- [ ] T2.1 `desktop/src/claude-presets.mjs`: `PRESETS` exactly as spec.md ›
      Catalogue, `RECOMMENDED`, and the functions of plan.md.
- Tests (`desktop/tests/claude-presets.test.mjs`, `node --test`):
  - Catalogue invariants: unique ids, every entry trimmed and passing
    `addEntry`, no entry twice in a preset, rules matching
    `^(Bash|Read)\(.+\)$`, no `:*` before the end of a rule, no allow rule
    with a `*` before its second word, no `rm -rf` or `git reset --hard` in
    the deny preset, no `storage.googleapis.com`.
  - `applyPreset`: appends missing entries in order; keeps existing entries
    and their order; adds nothing already present; does not mutate its input;
    on Windows (`platformSandbox:false`) adds rules only.
  - `presetApplied`: true after apply; false after one entry is removed; on
    Windows judged on rules only.
  - `removePreset`: removes the preset's entries; keeps an entry shared with
    another applied preset; removes an entry shared with a preset that is not
    applied; removes a hand-typed duplicate (US3.2).
  - `listsEmpty`: true for four empty lists whatever the state.

## 3. Panel

- [ ] T3.1 The Presets row in `sandboxSettings` at workstation level, with the
      apply and remove buttons, the Applied badge, the expandable entries, the
      Windows note and the guardrail description of the deny preset
      (`desktop/src/sandbox-settings.mjs`).
- [ ] T3.2 "Apply recommended" while the lists are empty.
- [ ] T3.3 Styles for the row (`desktop/src/style.css`), reusing the existing
      `sandbox-*` classes where they fit.
- [ ] T3.4 No Presets row in a project's Claude settings.
- Tests (`desktop/tests/sandbox-settings.ui.cjs`; build first with
  `npx vite build`, run unsandboxed):
  - Empty workstation: "Apply recommended" shown; clicking it fills Common and
    Dangerous actions, both read "Applied", the button disappears; save sends
    the entries in `claudeSandbox`.
  - Apply Go, then remove one of its entries by hand: Go offers "Apply" again.
  - Apply Go, then Remove: its entries leave the lists; a Common entry stays.
  - Windows (`platformSandbox:false`): applying Go adds its allow rules only.
  - A project's Claude settings show no Presets row.

## 4. Docs and changelog

- [ ] T4.1 `CHANGELOG.md` `[Unreleased]`: edit the #700 and #730 lines to say
      **Claude settings**; add an `### Added` line for the presets (#745).
- [ ] T4.2 `docs/CAPABILITIES.md`: a sentence on the presets after the
      workstation line.

## 5. Gates

- [ ] `node --test desktop/tests/*.test.mjs`
- [ ] Desktop UI suites touched (`sandbox-settings.ui.cjs`,
      `settings-version.ui.cjs`), after `npx vite build`.
- [ ] `go build ./...`, `go vet ./internal/agent/...`, and
      `go test ./internal/agent/... ./internal/agentconfig/...`.
- [ ] Repository lint used by CI for desktop sources, if any (`oxlint`).
