# Design

## Current behavior
`desktop/src/main.js` writes `#title` during `select()` using a local name or task key/ID followed by the skill. The sidebar already resolves a local name before `taskTitles`, which `refreshPRs()` fills through `api.serverTasks()`. Metadata completion invokes `render()` but does not refresh the header. Local rename has another direct header write. Empty-selection paths restore the selection prompt.

## Decisions
1. Centralize header text derivation and rendering so selection, metadata refresh, and rename use the same rules. Use the current selected run from `runs`, not a run captured by a completed asynchronous request.
2. Format the header as `<key or ID> · <effective title> · <skill>`. Effective title is the nonblank local name, otherwise the nonblank fetched title. Omit the title segment when unavailable or identical to the displayed identity. Continue to show the existing empty-selection prompt when no execution is selected.
3. Reuse the existing task metadata fetch cadence and full task IDs; do not add header-specific requests. Update or remove cached titles on successful metadata responses so an explicitly blank or missing task title cannot leave an obsolete title displayed. On request failure, retain a previously known title for that same task; without one, use identity and skill. Local names remain authoritative across refreshes.
4. Make metadata-driven header updates part of presentation rendering. They must not call `select()`, attach/detach the console, or reset terminal output. Late responses for another task may update that task's cache but must not replace the selected task's header.
5. Render all title content with `textContent`. Keep full header text in the DOM and a native hover title while CSS ellipsis limits the visible single line. Give the header text area flexible width and `min-width: 0`; preserve control visibility and keyboard access. If necessary at narrow supported window widths, allow toolbar controls to wrap while the title itself remains one line. Preserve directory visibility and terminal space.
6. Keep local rename storage, execution history order, selected execution, and existing control behavior unchanged. Historical executions show the current effective task title and their own execution skill.

## Rejected alternatives
- A backend title snapshot on every execution: unnecessary data contract change and would diverge from current task naming.
- Updating only in `select()`: misses delayed metadata and later title refreshes.
- Reselecting the run when metadata arrives: can reset or reconnect the console.
- Replacing task identity or skill with the title: removes context explicitly retained by clarification.

## Validation approach
Add focused Electron/Playwright coverage following `desktop/tests/task-order-render.ui.cjs` and the existing mock local-agent server pattern. Exercise delayed and changed metadata, selection changes during requests, local rename, history selection, missing/blank titles, request failure, and literal markup-like title text. Assert metadata-only refreshes do not introduce terminal attach/detach calls or clear existing output. Use a long title at a narrow supported desktop size with history and PR controls visible to verify layout, hover text, and keyboard access.

Run `npm --prefix desktop run build` and `npm --prefix desktop run test:ui` during implementation. The desktop package has no separate lint script. Validate this change with `openspec validate 82-task-tty-header-title --strict`. Implementation uses a toolbar ResizeObserver to refit the existing terminal when controls wrap; title refreshes do not change console attachment.
