# Plan: aligned run indicators in the Desktop sidebar

Behaviour: `spec.md`. This file records how to build it.

## Stack and scope

Desktop renderer only: `desktop/src/main.js` (vanilla DOM) and
`desktop/src/style.css`. No change to `shared/`, `web/`, the Go server or the
agent. No data contract changes.

## Current structure

The sidebar row is built in `render()` (`desktop/src/main.js`, around the
`const row=document.createElement('div');row.className='local-task …'` block):

```
div.local-task
  button.task-number        (absent for a free console)
  button.run                (selects the run)
    span.run-state          (glyph, data-run-id, data-run-state)
    strong                  (title)
    span.status.task-skill-status
  button.pr-indicator       (optional)
  button.task-archive
  button.task-menu
```

The key sits before the state, has no fixed width, and is missing for a free
console: the glyphs drift horizontally from row to row.

## Target structure

```
div.local-task
  span.run-state            (moved out of button.run, first child)
  span.task-key-slot        (always present, common min-width)
    button.task-number      (absent for a free console)
  button.run
    strong
    span.status.task-skill-status
  button.pr-indicator / button.task-archive / button.task-menu  (unchanged)
```

### Decisions

1. **The glyph leaves `button.run`.** The key is a button of its own; nesting it
   inside `button.run` is invalid HTML, so the glyph has to come out to precede
   it. To keep FR 7 (clicking the glyph selects the run), the glyph gets the same
   `onclick=()=>select(run)` as `button.run`. It stays a `span` (not focusable):
   the row is already reachable by keyboard through `button.run`, and its
   accessible name comes from `title`/`aria-label` set by `renderRunState`,
   unchanged. `button.run`'s own `title` keeps the state label, so the button's
   summary does not lose it.
   - Rejected: moving the key inside `button.run` as a `span` with its own click
     handler: it would lose the separate focus stop and the disabled state the
     clarification says must stay.
2. **A slot wrapper, not a placeholder button.** `span.task-key-slot` is always
   appended, and receives the key button only when `!freeConsole(run)`. The
   width lives on the slot, so a free console reserves it without rendering a
   `.task-number` (the free-console UI test asserts that none exists).
3. **Width.** The slot has `min-width:5ch` in the key's own font size (11 px),
   `font-variant-numeric:tabular-nums` on the key, `flex-shrink:0` and
   `white-space:nowrap`, so `#9999`, `M-12` and shorter keys align titles, and a
   longer key grows the slot (FR 4) instead of being cut. `5ch` covers the
   largest current keys (`#446`, `M-7`) with room for four-digit issues; tuning
   the value is allowed if a visual check shows it too tight or too loose.
4. **Spacing.** The left padding that `.local-task>.task-number` carries today
   (`padding:4px 0 4px 8px`) moves to the glyph (`padding-left:8px` on
   `.local-task>.run-state`), and the key keeps a small right gap so that it does
   not touch the title. `button.run` keeps its `gap:8px` for title and badge.
   The existing `.local-task>.task-number` rules are retargeted to
   `.local-task .task-key-slot>.task-number`.
5. **Live refresh untouched.** `renderTaskRowStates()` selects
   `.local-task .run-state` and matches by `data-run-id`; that selector still
   finds the glyph after the move, so no change is needed there. Same for the
   pulse rules on `.run-state[data-run-state=running] svg`.

## Target files

- `desktop/src/main.js`: sidebar row builder in `render()`.
- `desktop/src/style.css`: `.local-task` rules: glyph padding, `.task-key-slot`,
  retargeted `.task-number`.
- `desktop/tests/sidebar-alignment.ui.cjs` (new): Electron UI test.
- `desktop/tests/run-state-animation.ui.cjs`, `free-console.ui.cjs`,
  `task-order-render.ui.cjs`, `task-order-hold.ui.cjs`: re-run; adjust only a
  selector that assumed the glyph is inside `.run` (e.g.
  `.local-task .run .run-state`), if any.
- `CHANGELOG.md`: one line under `## [Unreleased]` → `### Changed`.

## Test plan

- New UI test `sidebar-alignment.ui.cjs`, on the pattern of
  `run-state-animation.ui.cjs` (fake agent server, `SECTILE_DESKTOP_TEST=1`):
  one project with runs for `#42`, `#446`, macro `M-7`, a free console and a key
  `#123456`. Assert with `getBoundingClientRect()`:
  - every `.local-task>.run-state` has the same `left`;
  - `strong` titles share one `left` for all rows except the `#123456` row, whose
    title is further right and whose key text is `#123456` in full with
    `scrollWidth<=clientWidth`;
  - the free-console row has a `.task-key-slot` of the common width and no
    `.task-number`;
  - the first element child of each row is `.run-state`;
  - clicking the glyph of an unselected row selects it; clicking the `#42` key
    calls the open-task path and does not select; the `M-7` key is disabled.
- Existing suites: desktop `npm test`, then `npx vite build` and the UI tests
  (they load `dist/index.html`). Restore `webui/.gitkeep` if the build deletes it.
- No Go, web or shared tests are affected; run them only if a shared file is
  touched, which this plan does not do.

## Risks

- Selectors in other UI tests that assume `.run .run-state`; grep before
  finishing.
- `sidebarBusy()` uses `:hover` on `.local-task`, so moving the glyph keeps it
  inside the hovered row: no change.
