# Plan #786 - Desktop: Launch dialog with an AI engine

## Stack

Desktop renderer only: plain JavaScript in `desktop/src/main.js`, pure helpers
in `desktop/src/engines.mjs`, unit tests with `node --test`
(`desktop/tests/*.test.mjs`), UI tests with Playwright on Electron
(`desktop/tests/*.ui.cjs`) against a fake agent. No agent, server or
migration change.

## Design

### Dialog

The body of `document.querySelector('#rerun').onclick` moves into
`openLaunchDialog({projectID,taskId,taskKey,skill,prompt})`. The toolbar
handler keeps its guards (no run, macro run, free console) and calls it with
the selected execution's skill and prompt. The dialog is titled
`Launch <key>`, its controls are renamed (FR4) and its submit reads `Launch`.

When `taskEnginesAvailable()` answers true, the dialog loads
`api.taskEngines(projectID)` and adds an `AI engine` select built as
`openAgentConsole` builds its own: one option per catalogue engine, labelled
with `engineTooltip(engine, engine.id===projectDefault)`, valued by id, set to
`taskEngine(view,taskId)?.id`. A failure to read the catalogue leaves the
select out and the launch as before.

On submit:

1. `launchEngineChange(view,taskId,chosen)` (new, `engines.mjs`) returns the
   id to store, or `null`.
2. When not `null`, `api.setTaskEngine(projectID,taskId,id)`. A refusal sets
   the notice to `Could not switch the engine: <message>`, re-enables submit
   and stops.
3. When the ticket table of the same project is open, its `engines` take the
   stored answer and every row's toggle is re-rendered
   (`renderEngineToggle`).
4. `api.launchServerTask(...)` as today, then `dialog.close()` and
   `refresh()`.

### Row entry

`ticketRow()` adds a `Launch…` menu item, after "Custom instructions…",
disabled like the others when the project is not configured. Its click closes
the menu and calls `openLaunchDialog` with `nextTaskStep(task,view.info)`'s
`skillId` when the project offers it, else `discuss`, and an empty prompt.

### Rejected alternatives

- A one-off engine on the launch body: settled against in round 2, the choice
  sticks to the task and the launch contract stays as is.
- Building the select from the Engine column's cycle helper: a select lists
  every engine at once, which is what the console does.

## Target files

- `desktop/src/engines.mjs`: `launchEngineChange`.
- `desktop/src/main.js`: `openLaunchDialog`, `#rerun` markup and handler,
  `ticketRow()` menu item.
- `desktop/tests/engines.test.mjs`: helper cases.
- `desktop/tests/task-engine.ui.cjs`: dialog engine select, stored engine,
  refusal, row entry.
- `desktop/tests/console.ui.cjs`, `conversation.ui.cjs`,
  `discussion-header.ui.cjs`, `free-console.ui.cjs`, `skill-mode.ui.cjs`:
  renamed button and controls.
- `CHANGELOG.md`, `docs/CAPABILITIES.md`.
