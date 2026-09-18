# Design

## Context
Today `browseTasks(projectID, initialQuery)` (`desktop/src/main.js:845-892`)
calls `showDialog('Launch task')`, builds a search `<form>`, then for each task
returned by `api.serverTasks(projectID, q, true)` appends a
`<section class="server-task">` with a `<select>` of skills (default `pickup`),
a hidden `<textarea>` for custom instructions, `modeSelect(...)` from
`skill-mode.mjs`, a **Launch** button and a `role=status` notice. The launch
calls `api.launchServerTask(projectID, task.id, skill, prompt,
launchModeOverride(mode))`, closes the dialog and refreshes.

Its callers are the project row's **Open tasks** icon (`main.js:294-296`) and
`newProjectTask` (`main.js:835-843`, "Run an existing ticket").

The agent-log pane (`#agent-log-pane`, `main.js:527-560`) is the existing
pattern for replacing the console with another view: it hides
`#workspace article`, shows the pane, tracks `logsOpen`, blocks terminal input
and resize while open, closes on Escape when no dialog is open, and restores
focus to its opener on close.

Other reusable pieces:
- `taskStage`, `nextTaskStep(task, project)` and `closingStep(task, project)`
  (`desktop/src/workflow.mjs`) already resolve the stage label and the next
  skill for the toolbar **Next: …** button, including its unavailability
  messages.
- `renderRunState(element, run)` (`main.js:166-171`) draws the shared run-state
  glyph from `shared/runStates.ts`; `runs` (global, polled every two seconds)
  carries `taskId`, `projectId` and `status` for every local execution.
- The sidebar PR icon (`main.js:333-340`) and the `pullRequests` map read
  `task.prUrl` and open it externally.
- `task-order.mjs` is the precedent for a pure ordering module with unit tests
  (`npm test` runs `tests/*.test.mjs`).
- The server orders `/api/tasks` by `status, position, created_at`
  (`internal/db/db.go:1256`); the agent proxies it unchanged
  (`internal/agent/agent_desktop.go:636-670`).

## Decisions

### A content-area pane, sharing the log pane's mechanics
A new `<section id="tickets-pane" aria-label="Tickets" hidden>` sits next to
`#agent-log-pane` in the `#app` template of `desktop/src/main.js` (the workspace
markup is built there; `desktop/index.html` only hosts the `#app` root). `browseTasks` becomes `openTickets(projectID,
initialQuery)`: it closes the dialog if open, closes the log pane if open, sets
a `ticketsOpen` flag, hides `#workspace article`, shows the pane and renders a
heading `Tickets · <project name>`, the search form, a `role=status` line and
the table. `closeTickets(restoreFocus)` mirrors `closeLogs`: hides the pane,
restores the article, calls `resize()`, and focuses the **Open tasks** icon of
that project when asked. The Escape handler and the `terminal.onData` /
`resize` guards gain `ticketsOpen` next to `logsOpen`. Opening the log pane
closes the tickets pane and vice versa: only one replacement view at a time.

Rejected: keeping the modal and putting the table inside it. The dialog is
`min(720px, 90vw)` wide and blocks the console; a table with six columns and
an actions cell does not fit, and the owner confirmed the pane during
clarification.

Rejected: a third button in `.execution-views` (Console / Changes / Tickets).
Those views belong to the selected execution; the tickets list belongs to a
project and must open even when nothing is selected.

### Ordering in a pure module
`desktop/src/task-list-order.mjs` exports:

- `PRIORITY_RANK = {urgent: 4, high: 3, medium: 2, low: 1}`; anything else
  ranks `0`.
- `compareIdentity(a, b)`: `String(a.key || a.id).localeCompare(String(b.key ||
  b.id), undefined, {numeric: true, sensitivity: 'base'})`, then `a.id`
  against `b.id` the same way.
- `compareBy(field)`: `priority` by rank, `key` by identity, `title` by
  `localeCompare(…, {sensitivity: 'base'})`, `stage` by the index of
  `taskStage(task)` in the workflow order `new, clarified, specified,
  implemented, reviewed, finished`, unknown stages last.
- `orderedTasks(tasks, {field = 'priority', ascending = false})`: stable copy
  sorted by `compareBy(field)`, direction applied to that comparison only, then
  `compareIdentity` ascending as the final tie-break regardless of direction.
- `DEFAULT_SORT = {field: 'priority', ascending: false}` and
  `nextSort(current, field)`: same field flips `ascending`; a new field starts
  `ascending: field !== 'priority'`.

The sort state is a local variable of `openTickets`; nothing is written to
`settings.json`. The web backlog uses the same rules, so the two surfaces agree
on `#9 < #100` and `PROJ-9 < PROJ-10`.

Rejected: sorting server-side. The server's order serves the board's manual
positions; adding sort parameters to `/api/tasks` and `/desktop/tasks` would
touch two Go components for a purely presentational need.

### Table markup and sortable headers
A `<table class="tickets-table">` with `<caption class="visually-hidden">Open
tasks in <project></caption>`. Header cells for Key, Title, Stage and Priority
contain a `<button type="button" class="sort-header" data-field="…">` so they
are keyboard-reachable; the `<th>` carries `aria-sort="ascending" |
"descending"` on the active column and `aria-sort="none"` elsewhere. An inline
arrow glyph next to the label shows the direction. The PR column header and the
actions header are plain text.

Row cells: state glyph (a `<span class="run-state">` filled by
`renderRunState` when a local run exists for `task.id`, otherwise empty with
`aria-hidden`), Key, Title (truncated to one line with `title` attribute),
Stage (`taskStage(task)`, with the tracker status as tooltip when the server
supplies one, so the information the old card printed is not lost), Priority
(the word, plus a small coloured dot using
the same four colours as the web: danger, warn, info, muted), PR icon (reusing
the sidebar SVG, `title` = URL, opens externally via the existing handler), and
the actions cell.

### Per-row actions: Run plus a compact menu
- **Run**: `const step = nextTaskStep(task, info)`; the button text is
  `Run: <step.label>` when `step.skillId` exists, else the button is disabled
  with `title = step.message`. `closingStep(task, info)` covers reviewed tasks
  (`Run: Close the task`). It is also disabled while any run in `runs` with
  `taskId === task.id` has status `queued | preparing | running`, with
  `title = 'An execution is active on this task'`. One click submits
  `launchServerTask(projectID, task.id, step.skillId, '', '')`: no prompt, no
  mode override, the configured mode applies.
- **…** menu: a `<button aria-haspopup="menu" aria-label="More actions for
  <key>">` toggling a `<div role="menu">` positioned under it, with
  `role="menuitem"` buttons: `Pickup (full chain)` when the project exposes
  `pickup`, one item per other server skill (`item.command || item.id`),
  `Discussion (no skill)` (`skillId = 'discuss'`), and `Custom instructions…`.
  Direct items launch immediately with no mode override. `Custom
  instructions…` expands an inline form row under the task row (`<tr
  class="ticket-compose">`) holding the existing textarea, `modeSelect(...)`, a
  **Launch** button and a `role=status` notice; the mode control lives only
  here because it is the one place a user composes a launch rather than
  clicking through. The menu closes on selection, Escape, or focus leaving it.
  It is not disabled by an active execution, so **Discussion** stays reachable.
- Submission feedback goes to the pane's `role=status` line (`Execution
  submitted for #226`) and the pane stays open; the sidebar picks up the new
  run on the next refresh. This differs from the dialog, which closed itself:
  a list is a place to launch several tickets in a row.
- With `info.configured === false` the notice `Configure a local repository
  before launching tasks.` is shown and every **Run**, menu item and Launch is
  disabled, as today.

Rejected: a `<select>` per row. Twenty selects in a table is the current
dialog's problem in another shape, and the owner confirmed Run + menu.

Rejected: launching Pickup from **Run**. Pickup runs the whole chain; the
toolbar's **Next** convention is one step at a time, and the list should not
default to more than the toolbar does.

### The key opens the task, the palette opens the list
The key cell holds a button reusing the sidebar's `.task-number` presentation and
`api.openTask(task.id)` (`main.js:294` area, sidebar row), so the identity means
the same thing on both surfaces and the desktop keeps a single way to reach a
task in Sectile.

`openCommandPalette` (`main.js`) hardcoded its single **Quick add task** button
and filtered it against a literal string. It becomes a `COMMANDS` list so a new
action is a row rather than another hidden-state to maintain, and **Tasks list**
joins it. That action needs a project: the selected one answers it, a single
configured project answers it too, and otherwise the palette asks with the same
`.discovered-project` buttons the new-task chooser uses rather than guessing.

Rejected: putting **Tasks list** in the header toolbar. The header is the local
agent's controls; #225 owns the project actions bar, and the palette is where
the app already collects global actions.

### Menu dismissal is bound to the opener, not only the menu
The row menu is a popup, so Escape and an outside press must dismiss it. Focus
can sit on the opener rather than inside the menu — opening it on an
unconfigured project leaves every item disabled and focus on the button — and a
handler bound to the menu alone never sees those events, which would let the
window-level Escape close the whole pane instead. The dismissal is therefore
bound to the opener as well, and an outside pointer press is watched while the
menu is open.

### The row reflects only the executions the sidebar shows
Row state filters `runs` with `hiddenRun`, the same predicate the sidebar uses,
so an execution the user archived does not reappear here. `hiddenRun` never
hides an active run, so archiving cannot silently re-enable **Run** on a task
that is still executing.

### The pane is a view of the workspace, not a mode beside it
`select()` closes the pane as it closes the log pane: otherwise selecting a
sidebar execution, or opening an agent console, changes state behind a hidden
view, and the console it creates cannot be seen or typed into. `agentUnavailable()`
closes it too, since its contents come from that agent, and `openTickets` refuses
to run while no agent is connected rather than hiding the connection screen the
user is looking at.

The poll's deferred render protects the sidebar rows under the pointer from
reordering. The pane is a different region whose update reorders nothing, so
`renderTicketRows()` runs on the deferred path as well; otherwise a run that
ends while the pointer rests on the sidebar leaves its ticket row disabled.

Disabling the focused **Run** would send focus to the body, so focus moves to
that row's menu, which is never disabled.

### One open menu, always detached
The menu's outside-press listener lives on the document, so it must not outlive
its menu. Most dismissals detach it on their own, but a rebuild or a
disconnection removes the row without any pointer event, which would leave the
listener bound to a detached menu. The view holds the open menu's closer, and
closing the pane or rebuilding the table calls it. That also keeps a single menu
open at a time.

### Typed instructions are the user's work, not render state
Sorting or searching rebuilds every row. The composed instructions and the
execution mode are held on the view and restored into the rebuilt row, so a
click on a column header cannot silently discard what the user was writing.

### Data refresh and stale responses
`openTickets` keeps a `generation` counter like today so a slow response cannot
overwrite a newer one or another project's pane. The task list is fetched on
open and on search submit only; the run-state glyphs and the **Run** disabled
state are recomputed from `runs` inside the existing `render()` cycle (every
poll) without refetching tasks and without rebuilding the table (rows keyed by
`task.id`, cells updated in place) so focus and the open menu survive a poll.

### Removal of the dialog form
`browseTasks` is deleted; `newProjectTask`'s "Run an existing ticket" and the
project row icon call `openTickets`. `.server-task` CSS rules are removed.
`quickAdd`'s success screen **Launch task** action, if it points at
`browseTasks`, points at `openTickets(projectID, task.key)` instead.

## Risks
- `desktop/tests/project-open-tasks.ui.cjs`, `desktop/tests/skill-mode.ui.cjs`
  and `desktop/tests/console.ui.cjs` address `.server-task`, the `Skill for #1` select and the dialog **Close**;
  both are rewritten against the pane's roles (table, columnheader buttons,
  `Run: Clarify`, `More actions for #1`). The mock agent in those tests must
  return `priority` on tasks to exercise the default order.
- Desktop UI tests load `dist/index.html`, so `npx vite build` runs before them.
- `runs` polling every two seconds must not rebuild the table; the in-place
  update is the guard against losing focus on a header button or an open menu.
- Long project task lists: no pagination is added; the pane scrolls
  (`overflow: auto`) like the log pane. Server search remains the way to narrow.
