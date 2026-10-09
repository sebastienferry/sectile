# Plan #806 - Desktop project board

## Stack

- Desktop renderer: plain JavaScript in `desktop/src/main.js`, pure helpers
  in `desktop/src/*.mjs`, styles in `desktop/src/style.css`, unit tests with
  `node --test` (`desktop/tests/*.test.mjs`), UI tests with Playwright on
  Electron (`desktop/tests/*.ui.cjs`) against `desktop/tests/fake-agent.cjs`.
- Desktop main process: `desktop/electron/main.cjs` and `preload.cjs` (IPC to
  the local agent).
- Shared code: `shared/*.mjs` with a `.d.mts` declaration, already imported by
  both clients (`shared/sidebarShortcut.mjs`, `shared/mcpConfig.mjs`).
- Agent: Go, `internal/agent/agent_desktop.go`.
- Web: only the helpers that move to `shared/`, behaviour unchanged.

No migration, no server change.

## Data contracts

### `GET /desktop/project?projectId=` (agent, extended)

Adds one optional field, `board`, read from `GET /api/projects/:id` (the
agent already does it in `internal/agent/evidence.go:100`):

```json
"board": {
  "epicColors": true,
  "trackerColumns": [{"name": "In Review", "statuses": ["In Review"]}],
  "stageColumns": {"implemented": ["In Review"]},
  "trackers": [
    {"trackerId": "…", "trackerColumns": [ … ], "stageColumns": { … }}
  ]
}
```

The values are copied from `models.Project` (`EpicColors`, `TrackerColumns`,
`StageColumns`, `Trackers[].TrackerID/TrackerColumns/StageColumns`). When the
project cannot be read the field is omitted and the rest of the payload is
answered as today. Desktop treats a missing `board` as
`{epicColors:false, trackerColumns:[], stageColumns:{}, trackers:[]}`.

### `GET /desktop/tasks?projectId=&q=` (unchanged)

The board calls it without `launchable=true`, so finished tasks come back.
`api.serverTasks(projectID, q, false)` already does that.

### `POST /desktop/tasks/stage-move?projectId=` (agent, new)

Request:

```json
{"taskId": "…", "labels": ["bug", "#implemented"], "status": "to_test",
 "trackerStatus": "In Review"}
```

The agent:

1. refuses any method but POST (405), a body over 64 KiB or not JSON (400),
   a missing project or task (400), a `status` outside the six internal
   statuses `to_clarify, clarified, to_implement, to_test, to_close,
   finished` (400);
2. checks the project with `fetchConfig` as `desktopTaskTransition` does;
3. sends `PUT /api/tasks/:id` to the server with exactly
   `{labels, status, trackerStatus?, stageProjectId: projectId}`
   (`trackerStatus` omitted when empty), with the agent token;
4. passes the server's status code and body back (capped at 1 MiB).

Capability `stage-move` is added to the `/desktop/status` list.

### Desktop IPC

- `preload.cjs`: `moveTaskStage:(projectId,taskId,move)=>ipcRenderer.invoke('move-task-stage',{projectId,taskId,move})`.
- `main.cjs`: `ipcMain.handle('move-task-stage', …)` →
  `api('/desktop/tasks/stage-move?projectId='+…,'POST',{taskId,...move})`.

### localStorage keys (renderer)

- `boardHideFinished`: `'true'` / `'false'`; absent reads `true`.
- `boardCardDisplay`: `'condensed'` / `'full'`; absent or unknown reads
  `condensed`.

## Shared helpers

### `shared/epicColor.mjs` (+ `.d.mts`)

- `EPIC_PALETTE`: the hex values of the web's `ACCENT_COLORS`, in order.
- `fnv1a(value)`: moved verbatim from `web/src/lib/epicColor.ts`.
- `epicColorIndex(parentKey)`: index in the palette, or `-1` without a key.
- `epicColorHex(parentKey)`: the hex value, or `null`.

`web/src/lib/epicColor.ts` re-exports `fnv1a` and keeps `epicColor`,
`epicColorHex` and `epicColorsEnabled`, computing the index through
`epicColorIndex` so `ACCENT_COLORS` stays its palette. A web test asserts
`ACCENT_COLORS.map(c=>c.hex)` equals `EPIC_PALETTE`, so the two cannot drift;
the existing pinned-hash test stays.

### `shared/workflowStage.mjs` (+ `.d.mts`)

Moved out of `web/src/lib/workflow.ts` and `stageMapping.ts`, untyped:

- `WORKFLOW_STAGES` (the six stages in order),
  `INTERNAL_STATUS_BY_STAGE`, `STAGE_LABELS`
  (`untouched,new,clarified,specified,implemented,reviewed,finished,closed`).
- `trackerBoard(project, trackerId)` and `columnOfTask(task, project)`.
- `stageFromColumn(task, project)`.
- `explicitStage(task)`: the explicit workflow label step of
  `resolveTaskStage` (finished family, then reviewed … new/untouched), or
  `null`.
- `stageMove(task, targetStage, project)`: returns
  `{labels, status, trackerStatus}` exactly as `moveTaskWorkflowStage`
  computes them (`AppContext.tsx:3836-3866`), `trackerStatus` being the
  task's own when the project maps no column to the stage. `project` is the
  one `projectForTracker` returns.

The web's `resolveTaskStage`, `stageFromColumn`, `columnOfTask`,
`trackerBoard` and `moveTaskWorkflowStage` call these; their signatures and
results are unchanged.

## Desktop design

### Stage resolution (`desktop/src/workflow.mjs`)

`taskStage(task, board)` gains an optional second argument:

1. `task.status` finished or done → `finished` (kept first, as today: the
   agent and the tickets list already treat such a task as finished);
2. `explicitStage(task)` when not `null`;
3. `stageFromColumn(task, board)` when `board` is given and it answers;
4. the existing status map.

Callers without a board (`nextTaskStep`, `task-list-order.mjs`, the sidebar)
keep today's result. The tickets list and the sidebar's grouping pass the
project's `board` when they have it, so the three surfaces agree (FR2).
Existing `workflow.test.mjs` cases stay green.

### Board module (`desktop/src/board.mjs`, new, pure)

- `boardOptions(storage)` / `saveBoardOption(storage, key, value)`: read and
  write the two localStorage keys with their defaults, tolerant of a storage
  that throws.
- `boardColumns(tasks, board)`: `{stage, tasks}[]` for the six stages, tasks
  sorted with `compareBy('priority')` then `compareIdentity`
  (`task-list-order.mjs`).
- `boardCardLabels(task)`: the labels minus the workflow ones
  (`STAGE_LABELS`, `#` and case insensitive).
- `cardEpicColor(task, board)`: `epicColorHex(task.parentKey)` when
  `board.epicColors`, else `null`.

### Page (`desktop/src/main.js`)

- `openBoard(projectID, initialQuery='')` mirrors `openTickets`: same agent
  check, same opener capture, same `ticketsPane` element and `ticketsOpen`
  flag (so `resize` and console focus guards stay correct), with
  `view.kind='board'`. `openTickets` and `openBoard` each close the other
  (US1.5). `closeTickets` is renamed nowhere; it already restores the
  workspace and the focus and serves both.
- Toolbar: heading `Board · <name>`, a `Condensed`/`Full` segmented toggle
  (`role=radiogroup`, accessible name `Card display`), `Close board`. Search
  form as in the tickets list (`aria-label` `Search server tasks`).
- `load()`: `Promise.all([api.serverTasks(projectID,q,false), api.project(projectID), agentStatus])`
  with the same generation guard; `view.board = info.board ?? EMPTY_BOARD`,
  `view.canMove = status.capabilities?.includes('stage-move')`.
- `renderBoard(view)`: a `div.board` grid of six `section.board-column`
  (`aria-label` `<Stage> column, <n> tasks`), header with name and count,
  `data-stage` for the tint. The finished column renders either expanded with
  a `Hide finished` button, or as `button.board-collapsed` (`aria-label`
  `Show finished tasks (<n>)`, vertical text, count).
- Cards: `article.board-card[data-display=condensed|full]` with
  `style="--epic-color:…"` and class `has-epic` when a colour applies. The
  title is a button that selects the task as a tickets row does. The `…`
  menu is built by the code that builds the row menu today: the body of
  `ticketRow`'s menu (`main.js:2972-3080`) is extracted into
  `taskActionsMenu(view, task, anchor)` and called from both, so the entries
  and their handlers stay single (US7.1). Run state badge with
  `runStateOf`/`runStateSvg`, as the row's state cell.
- Drag and drop (only when `view.canMove`): `draggable=true` on cards,
  `dragover`/`drop` on each column body and on the collapsed strip,
  `.drop-target` highlight while hovering. On drop onto another stage:
  mark the card `aria-busy=true` and not draggable, compute
  `stageMove(task, stage, projectForTracker(view.board, task.trackerId))`,
  call `api.moveTaskStage(projectID, task.id, move)`; on success `load()`;
  on failure render again from `view.tasks` (the card is back) and put the
  server message in `view.status` (`role=status`) as an alert line. Same
  column: nothing (US8.7).
- Refresh: `skill-result-refresh` and the launch paths that call
  `ticketsView.load()` when `ticketsView.projectID` matches already cover the
  board, since it is the same view slot.
- Entries: `{label:'Open board',run:()=>openBoard(project.id)}` after
  `Open tasks` in the project menu (`main.js:672`); palette action
  `{label:'Project board',detail:'See a project’s tasks by workflow stage',run:()=>openBoardFromPalette()}`,
  `openBoardFromPalette` built as `openTicketsFromPalette` (the project
  chooser is shared by passing the opener function).

### Styles (`desktop/src/style.css`)

`.board` horizontal grid with scroll; `.board-column[data-stage=…]` tinted
with `--stage-<stage>-bg`; `.board-collapsed` narrow strip; condensed card
one line with ellipsis; `.board-card.has-epic` left border
`4px solid var(--epic-color)`; `.drop-target` outline; focus styles reuse the
existing ones. Light and dark themes both use the existing variables.

## Rejected alternatives

- **Reusing `/desktop/tasks/transition`** for the drop: it posts to
  `/api/tasks/:id/stage`, which demands pull request evidence and records a
  stage report, unlike the web's drop (clarification D10).
- **Computing the move in the agent (Go)**: a second implementation of the
  web's rules that could drift; the renderer already has the task and the
  mapping, and the agent restricts the fields it forwards.
- **A separate pane element for the board**: duplicates the open/close and
  focus logic that the tickets slot already handles.
- **Copying the hash and palette into Desktop**: a palette change in the web
  would recolour macros on one client only (clarification D4).

## Target files

- `shared/epicColor.mjs`, `shared/epicColor.d.mts`,
  `shared/workflowStage.mjs`, `shared/workflowStage.d.mts` (new).
- `web/src/lib/epicColor.ts`, `web/src/lib/workflow.ts`,
  `web/src/lib/stageMapping.ts`, `web/src/context/AppContext.tsx`
  (delegate), web tests next to them.
- `internal/agent/agent_desktop.go` (route, capability, `board` field),
  `internal/agent/agent_desktop_test.go`.
- `desktop/electron/main.cjs`, `desktop/electron/preload.cjs`.
- `desktop/src/workflow.mjs`, `desktop/src/board.mjs` (new),
  `desktop/src/main.js`, `desktop/src/style.css`.
- `desktop/tests/workflow.test.mjs`, `desktop/tests/board.test.mjs` (new),
  `desktop/tests/fake-agent.cjs`, `desktop/tests/board.ui.cjs` (new).
- `docs/USER_GUIDE.md`, `CHANGELOG.md`.

## Risks

- `main.js` is large: edit it in small hunks (subagents stall on it).
- UI tests load `dist/index.html`: run `npx vite build` in `desktop/` first,
  and restore `internal/webui/.gitkeep` if a web build deletes it.
- The web keeps its own stage tests; moving code to `shared/` must not change
  one of their expectations. Run the web unit tests before and after.
