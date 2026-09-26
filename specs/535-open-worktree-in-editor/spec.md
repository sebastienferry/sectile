# Specification #535 - Open the worktree in the configured editor

- Ticket: https://github.com/sebastienferry/sectile/issues/535
- Branch: `feat/535`
- Clarification: `docs/clarifications/535.md` (rounds 1 and 2, confirmed by
  the owner)
- Framework: Spec Kit

## Summary

In the desktop app, the toolbar line that shows the selected execution's
worktree path gains a small "code" icon button. One click opens that
directory in the editor chosen in **Settings → Execution defaults**. The
button only exists when an editor is chosen there, and the Editor setting
becomes a picker of the known editors plus a custom command.

## Scope

In scope: the desktop toolbar's worktree line, the Editor row of the desktop
execution defaults, and the local agent route that opens the editor.

Out of scope:

- The web client, and the server's `/api/open-editor` route and `open_editor`
  operation, which keep their `code` fallback.
- The macro panel, and the read-only context folders of multi-repo projects.
- Any placeholder in the editor command (`{path}`): the path is appended.
- A per-project editor: the editor stays a workstation default.

## Definitions

- **Selected execution**: the run shown in the console, whose `directory` the
  toolbar displays.
- **Worktree path**: that `directory`, the task worktree or the project root
  when worktrees are off.
- **Configured editor**: the non-empty `editorCommand` of the workstation
  defaults.
- **Open-in-editor button**: the icon button next to the worktree path.
- **Capable agent**: a local agent whose `/desktop/status` lists the
  `open-editor` capability.

## User stories (prioritised)

### US1 - Open the worktree from the toolbar (P1)

As a desktop user, I open the selected execution's worktree in my editor from
an icon next to its path, without copying the path into a shell.

**Acceptance scenarios**

1. **Given** a configured editor `cursor` and a selected execution whose
   worktree path is shown, **when** I look at the worktree line, **then** an
   icon button follows the path, with the accessible name and tooltip
   `Open in Cursor`.
2. **Given** that state, **when** I click the button, **then** the local agent
   starts the configured editor on the selected execution's worktree path, and
   the path button still copies the path when clicked.
3. **Given** a configured custom command `nvim-qt --maximized`, **when** I look
   at the button, **then** its name is `Open in nvim-qt`.
4. **Given** a selected execution whose worktree was deleted, **when** I click
   the button, **then** the desktop shows a readable error that names the
   missing path, and nothing is launched.
5. **Given** an execution that has exited but whose worktree still exists,
   **when** I click the button, **then** the editor opens it.

### US2 - The button only exists when an editor is chosen (P1)

As a desktop user without an editor, I see the path alone, as today.

**Acceptance scenarios**

1. **Given** no configured editor, **when** an execution is selected, **then**
   the worktree line shows the path and no open-in-editor button.
2. **Given** no selected execution, or an execution without a worktree path,
   **then** the button is hidden, like the path.
3. **Given** a local agent that is not a capable agent, **then** the button is
   hidden whatever the settings say.
4. **Given** no configured editor, **when** I choose `VS Code` in the Editor
   row and save the execution defaults, **then** the button appears next to
   the path without restarting the app; **when** I choose `None` and save,
   **then** it disappears.

### US3 - Choose the editor from a picker (P2)

As a desktop user, I pick my editor from a list instead of typing its command.

**Acceptance scenarios**

1. **Given** the Execution defaults panel, **then** the Editor row is a picker
   offering `None`, `VS Code`, `Cursor`, `Zed`, `Sublime Text` and
   `Custom command…`, in that order.
2. **Given** a stored `editorCommand` of `""`, `code`, `cursor`, `zed` or
   `subl`, **when** the panel opens, **then** the picker shows `None`,
   `VS Code`, `Cursor`, `Zed` or `Sublime Text`, and no free-text field.
3. **Given** a stored `editorCommand` that matches no preset, such as
   `cursor -n`, **when** the panel opens, **then** the picker shows
   `Custom command…` and a text field holding `cursor -n`.
4. **Given** the picker on `Custom command…`, **when** I type a command and
   save, **then** the stored `editorCommand` is that command, trimmed.
5. **Given** the picker on `None`, **when** I save, **then** no
   `editorCommand` is stored, and the row's hint reads `Default · None`.
6. **Given** a preset chosen, **when** I press the row's reset control,
   **then** the picker returns to `None`.

## Functional requirements

- **FR1** The desktop sends the agent the run ID of the selected execution,
  never a path. The agent resolves the directory from its own run registry.
- **FR2** The agent refuses, with a readable message and without launching
  anything: an unknown run, a run without a directory, a directory that no
  longer exists or is not a directory, and a workstation without a configured
  editor. It never falls back to `code`.
- **FR3** The agent launches the configured editor exactly as the existing
  `open_editor` operation does: the command is split on spaces, known editors
  are resolved on macOS, the absolute path is appended, and the process is
  detached. A launch failure is reported with its message.
- **FR4** The agent announces the `open-editor` capability in
  `/desktop/status`. The desktop hides the button against an agent without
  it, and a call to such an agent fails with
  `Update and restart the local agent to open the editor.`
- **FR5** The button's name is `Open in <label>`: the preset label for a
  preset command, else the first word of the custom command.
- **FR6** The desktop reads the configured editor from the workstation
  settings when the agent connects, and again after the execution defaults
  are saved.
- **FR7** The stored value keeps its name and format: a plain
  `editorCommand` string, absent when `None` is chosen. No migration.
- **FR8** `CHANGELOG.md` gains one `Added` line under `[Unreleased]`.

## Success criteria

- With an editor chosen, a click on the icon opens the worktree in it.
- With no editor chosen, the toolbar is unchanged from today.
- No code path lets the renderer choose the directory that is opened.
