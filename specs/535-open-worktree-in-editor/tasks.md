# Tasks #535 - Open the worktree in the configured editor

## Agent

- [ ] T1 Add the `openEditorFn` seam to `agentDaemon`.
- [ ] T2 Write `desktopOpenEditor` in `agent_desktop_editor.go` (FR1-FR3).
- [ ] T3 Route `/desktop/open-editor` and announce `open-editor` (FR4).
- [ ] T4 Test the handler: 405, 400, 404, no editor (409), no directory
      (409), removed directory (410), launch error (500), success on a
      running and on an exited run, capability in `/desktop/status`.

## Desktop bridge

- [ ] T5 `open-editor` IPC in `main.cjs`, refusing an agent without the
      capability (FR4); `openEditor(runId)` in `preload.cjs`.

## Desktop renderer

- [ ] T6 `editors.mjs`: presets, `editorChoice`, `editorLabel` (FR5, FR7),
      with `editors.test.mjs`.
- [ ] T7 Replace the Editor text input with `editorPicker` (US3).
- [ ] T8 Load `configuredEditor` on connect and after save (FR6).
- [ ] T9 Add `#open-editor` after `#worktree`, visibility and name in
      `showDirectory`, click handler with error display (US1, US2).
- [ ] T10 Style the icon on the worktree line.

## Tests

- [ ] T11 `open-editor.ui.cjs`: button hidden without editor, without
      capability; shown with `Open in Cursor`; click posts the run ID; agent
      refusal shown; appears after saving a preset.
- [ ] T12 Update `workstation-settings.ui.cjs` for the picker (preset,
      custom, None).

## Documentation

- [ ] T13 `desktop/README.md`: the Editor picker and the toolbar button.
- [ ] T14 `CHANGELOG.md`: one `Added` line (FR8).

## Test plan

- `go build ./... && go vet ./internal/agent/`
- `go test ./internal/agent/ -run 'OpenEditor|DesktopStatus'`
- `cd desktop && node --test tests/editors.test.mjs tests/execution-fields.test.mjs`
- `cd desktop && npx vite build && node --test tests/open-editor.ui.cjs tests/workstation-settings.ui.cjs tests/discussion-header.ui.cjs`
