# Tasks

## 1. Ordering module
- [x] 1.1 Create `desktop/src/task-list-order.mjs` with `PRIORITY_RANK`, `compareIdentity`, `compareBy`, `orderedTasks`, `DEFAULT_SORT` and `nextSort` as described in `design.md`.
- [x] 1.2 Add `desktop/tests/task-list-order.test.mjs` covering: default order (priority desc then `#9 < #100`), `PROJ-9 < PROJ-10`, missing key falling back to id, unknown priority last, each sortable field in both directions, identity tie-break kept ascending under a descending sort, and `nextSort` transitions (new field ascending except priority, same field flips).

## 2. Pane markup and lifecycle
- [x] 2.1 Add `<section id="tickets-pane" aria-label="Tickets" hidden>` beside `#agent-log-pane` in the `#app` template of `desktop/src/main.js` (the workspace markup lives there, not in `desktop/index.html`).
- [x] 2.2 In `desktop/src/main.js`, add `ticketsOpen`, `openTickets(projectID, initialQuery)` and `closeTickets(restoreFocus)` mirroring the log pane (`main.js:527-560`): hide/show `#workspace article`, call `resize()` on close, close the other pane and any open dialog on open.
- [x] 2.3 Extend the Escape handler, `terminal.onData` and `resize` guards to `ticketsOpen`.
- [x] 2.4 Point the project row **Open tasks** icon (`main.js:294-296`), `newProjectTask` "Run an existing ticket" (`main.js:835-843`) and any **Launch task** success action at `openTickets`; delete `browseTasks`.

## 3. Table rendering
- [x] 3.1 Render heading `Tickets · <project name>`, a **Close** button, the search form (`Search server tasks` textbox, **Search** button), a `role=status` line and the `<table class="tickets-table">` with caption.
- [x] 3.2 Render sortable headers Key, Title, Stage, Priority as `<button class="sort-header">` inside `<th aria-sort>`, plus plain PR and actions headers; wire clicks to `nextSort` and re-render rows in place.
- [x] 3.5 Make the key cell a control that opens the task in Sectile through `api.openTask`, reusing the sidebar `.task-number` presentation, and add a command-palette **Tasks list** action that opens the pane for the selected project, the only configured project, or a project the user picks.
- [x] 3.3 Render rows keyed by `task.id`: run-state cell, key, one-line title with `title` attribute, `taskStage` label (tracker status as its tooltip when present), priority word with coloured dot, PR icon reusing the sidebar glyph and external-open handler.
- [x] 3.4 Keep loading (`Loading open tasks…`, `aria-busy`), empty (`No open tasks in this project` / `No matching open tasks`), error (`role=alert`, `Could not load open tasks: … Use Search to retry.`) and unconfigured (`Configure a local repository before launching tasks.`) messages, and the `generation` guard against stale responses.

## 4. Row actions
- [x] 4.1 **Run** button: label `Run: <label>` from `nextTaskStep` / `closingStep`; disabled with the step message as tooltip when unavailable; disabled with `An execution is active on this task` while a `queued | preparing | running` run exists for the task; click submits `launchServerTask(projectID, task.id, skillId, '', '')`.
- [x] 4.2 **…** menu (`aria-haspopup="menu"`, `role="menu"`, `role="menuitem"`): Pickup when available, other server skills, Discussion (no skill), Custom instructions…; closes on selection, Escape and focus-out; not disabled by an active execution.
- [x] 4.3 Custom instructions inline row: textarea (`aria-label="Custom instructions"`), `modeSelect(document, 'Execution mode for <key>')`, **Launch**, `role=status`; refuse an empty prompt with `Enter custom instructions.`; submit with `launchModeOverride(mode.value)`.
- [x] 4.4 On success write `Execution submitted for <key>` to the pane status and call `refresh()` without closing the pane; on failure show the error and re-enable the control.
- [x] 4.5 In `render()`, update run-state glyphs and **Run** disabled state of visible rows in place from `runs`, without rebuilding the table.

## 5. Styles
- [x] 5.1 Add `#tickets-pane`, `.tickets-table`, `.sort-header`, `.ticket-compose`, menu and priority-dot rules to `desktop/src/style.css`; remove the `.server-task` rules.
- [x] 5.2 Check focus outlines on header buttons, **Run**, **…** and menu items, and title truncation at narrow widths.

## 6. Tests and documentation
- [x] 6.1 Rewrite `desktop/tests/project-open-tasks.ui.cjs` against the pane: entry point still hover/focus revealed; pane opens without changing project collapse; default row order with mixed priorities and `#9` vs `#100`; header sort and flip with `aria-sort`; search, loading, empty, error, unconfigured; `Run: Clarify` launches once with the expected payload; menu launches Pickup; custom instructions with a mode override; **Run** disabled while a run is active for the task while the menu stays enabled; Escape and **Close** restore the console view and focus.
- [x] 6.2 Update `desktop/tests/skill-mode.ui.cjs` to reach the mode control through **… → Custom instructions…**, and `desktop/tests/console.ui.cjs`, whose Quick add **Launch task** flow and existing-ticket launch also drove the dialog cards.
- [x] 6.3 Run `npx vite build`, `npm test` and `npm run test:ui` in `desktop/`; confirm `task-order*.ui.cjs`, `agent-logs.ui.cjs` and `next-step.ui.cjs` still pass.
- [x] 6.4 Update `desktop/README.md`: the "Open tasks" paragraphs describe the pane, the table, the default and column ordering, **Run** and the **…** menu, and the active-execution rule.
- [x] 6.5 Run `openspec validate 226-desktop-tickets-list --strict`.
