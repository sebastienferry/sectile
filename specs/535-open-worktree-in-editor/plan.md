# Plan #535 - Open the worktree in the configured editor

## Stack

- Local agent: Go, `internal/agent` (desktop loopback routes).
- Desktop: Electron main process (`desktop/electron/main.cjs`), preload
  bridge (`desktop/electron/preload.cjs`), renderer (`desktop/src/main.js`,
  `desktop/src/style.css`), pure helpers in `desktop/src/*.mjs`.

## Architecture

```
renderer #open-editor click
  -> api.openEditor(runId)                      preload.cjs
  -> ipc 'open-editor'                          main.cjs (capability check)
  -> POST /desktop/open-editor {runId}          agent loopback, bearer token
  -> run registry -> run.desktop.Directory      agent_desktop_editor.go
  -> agentconfig.Settings.Editor()
  -> runner.OpenInEditor(editor, directory)     unchanged
```

The existing `/desktop/terminal/detach` route is the model: same bearer
check (done by `desktopHandler`), same `runId` body, same injectable launcher
field for tests.

## Data contracts

`POST /desktop/open-editor`

- Body: `{"runId": "<id>"}`, at most 8 KiB.
- `200 {"editor": "<command>", "directory": "<path>"}` once the editor process
  is started.
- `400 Run ID required`, `404 Run not found`,
  `409 No editor is configured in Settings`,
  `409 This execution has no worktree`,
  `410 The worktree no longer exists: <path>`,
  `405` for any method but POST, `500 <launch error>`.

`/desktop/status` capabilities gain `open-editor`.

Workstation settings: unchanged. `defaults.editorCommand` is read as is.

## Target files

- `internal/agent/agent_desktop_editor.go` (new): `openEditorCapability`,
  `desktopOpenEditor` handler.
- `internal/agent/agent.go`: `openEditorFn func(editor, dir string) error`
  test seam, next to `launchTerminalFn`.
- `internal/agent/agent_desktop.go`: route and capability.
- `internal/agent/agent_desktop_editor_test.go` (new): handler tests.
- `desktop/electron/main.cjs`: `open-editor` IPC with capability check.
- `desktop/electron/preload.cjs`: `openEditor(runId)`.
- `desktop/src/editors.mjs` (new): `EDITORS` presets, `editorChoice(value)`
  (select value + custom text for a stored command), `editorLabel(command)`.
- `desktop/tests/editors.test.mjs` (new): unit tests of those helpers.
- `desktop/src/main.js`: `#open-editor` button in `.worktree-line`,
  `configuredEditor` state loaded in `ready()` and after save, visibility in
  `showDirectory`, click handler, `editorPicker` replacing the text input.
- `desktop/src/style.css`: icon button sizing on the worktree line.
- `desktop/tests/open-editor.ui.cjs` (new): toolbar button against a fake
  agent.
- `desktop/tests/workstation-settings.ui.cjs`: Editor row is now a picker.
- `desktop/README.md`, `CHANGELOG.md`.

## Decisions

- **Directory from the registry, not the renderer** (FR1): a compromised or
  buggy renderer cannot make the agent start a process on an arbitrary path.
- **Exited runs are accepted** (US1-5): unlike detaching a terminal, opening
  a folder needs no live process; the existence check covers a removed
  worktree.
- **`410 Gone`** for a removed worktree distinguishes it from "not
  configured"; the desktop shows the agent's text either way.
- **Editor visibility cached in the renderer** (FR6): reading the settings on
  each selection would add a round trip per click in the sidebar.
- **Helpers in an `.mjs` module** so the preset matching and the label are
  unit-tested without Electron, like `execution-fields.mjs`.

## Rejected alternatives

- Reusing the server's `/api/open-editor`: it runs through the server and the
  agent operation queue, and falls back to `code`, which US2 forbids.
- Opening from the Electron main process with `shell.openPath`: it opens the
  folder in Finder, not in the chosen editor, and bypasses the agent's editor
  resolution (`FindEditorBinary`).
