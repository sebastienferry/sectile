# Desktop tickets list: a sortable table of a project's open tasks

## Why
The desktop "Launch task" dialog (`browseTasks`, `desktop/src/main.js:845-892`)
stacks one card per open task inside a 720 px modal, in whatever order the
server returns (`status, position, created_at`). Every card carries its own
skill selector, execution-mode control, hidden textarea and Launch button, so a
project with twenty open tasks is a long scroll of identical forms, with no way
to see the highest-priority ticket first or to sort by anything. The web backlog
(`web/src/components/ListView.tsx`) already solves this with a table sorted by
priority descending and sortable column headers; the desktop should read the
same way.

Clarification #226 (`docs/clarifications/226.md`) settled the six open points
with the owner: the list lives in the content area, each row has a **Run**
button for the next workflow step plus a compact **…** menu, identity order is
natural-numeric, four columns sort, the sort choice is session-only, and tasks
with an active execution stay listed with **Run** disabled.

## What Changes
- Replace the modal card stack with a **tickets pane** in the content area,
  where the console sits, on the pattern of the agent-log pane. The two existing
  entry points (project **Open tasks** icon, **+ → Run an existing ticket**) open
  it; a **Close** control or Escape returns to the selected execution.
- Render the project's open tasks as a table with columns **Key**, **Title**,
  **Stage**, **Priority**, a non-sortable **PR** icon column and an actions
  column. Default order: priority descending (`urgent > high > medium > low`,
  unknown last), then identity ascending with natural-numeric comparison on the
  task key, falling back to the id.
- Make **Key**, **Title**, **Stage** and **Priority** sortable from their
  headers: first click sorts ascending (Priority: descending), a second click
  flips; the active column and direction are shown and announced. The choice
  lasts for the window session only.
- Give each row a **Run** button labelled with the task's next workflow step and
  a **…** menu offering **Pickup**, the other server skills, **Discussion (no
  skill)** and **Custom instructions…** with the one-off execution mode.
- Show the shared run-state glyph on rows whose task has a local execution, and
  disable **Run** while that execution is active; the menu stays available.
- Keep the search form, the loading, empty and error messages, the finished-task
  filter and the unconfigured-project notice as they are.
- **BREAKING** for the desktop UI tests only: `desktop/tests/project-open-tasks.ui.cjs`
  and `desktop/tests/skill-mode.ui.cjs` drive the dialog cards and must be
  rewritten against the pane.

## Impact
- Desktop renderer only: `desktop/index.html` (pane element), `desktop/src/main.js`
  (`browseTasks` rewritten, open/close logic shared with the log pane),
  new `desktop/src/task-list-order.mjs` and its unit test, `desktop/src/style.css`.
- `desktop/README.md` "Open tasks" paragraphs.
- No server, agent, database or web change: `GET /desktop/tasks` already returns
  `key`, `id`, `priority`, `status`, `labels` and `prUrl`; launches keep going
  through `launchServerTask`.
- Modifies capability `desktop-project-tasks`. Does not touch
  `desktop-task-ordering` (sidebar execution order) or `desktop-next-step`.
- Related tickets: #225 (actions bar) may later add a toolbar entry to the same
  pane; #109 (multi-selection) is out of scope.
