# Plan #525 - Lighter execution history drop-down in the desktop toolbar

Implementation choices for `spec.md`. Behaviour lives there; this file says
where and how.

## Stack

- Electron desktop renderer only: `desktop/src/style.css`. No change to
  `desktop/src/main.js`, the server, the agent or the web client.
- Tests: the Playwright Electron UI tests (`desktop/tests/*.ui.cjs`, run after
  `npx vite build`).

## What exists

- Markup (`desktop/src/main.js:42`): `<select id="execution-history"
  aria-label="Execution history" hidden>` first in `.toolbar-actions`.
- `render()` (`desktop/src/main.js:618`) fills the options and hides the
  select under two executions.
- `desktop/src/style.css`: the global `select` rule (surface background,
  `--border-strong` border, 6px radius, 8px padding),
  `#execution-history{max-width:170px}`, and
  `.toolbar-actions>select{flex-shrink:0;max-width:100%;margin:0}`.
- Theme tokens `--hover-bg`, `--accent`, `--text`, `--text-muted`,
  `--surface` are defined for dark (`:root`) and light
  (`@media (prefers-color-scheme:light)`); the Appearance setting drives
  that media query through `nativeTheme.themeSource`.

## Decisions

### 1. One rule block scoped to `#toolbar #execution-history`

Scoping by id keeps the global `select` rule and every other select as they
are (FR6). The block sits next to the "Icon actions stay unboxed" rules at the
end of `style.css`, so it wins over the global rule without `!important`.

### 2. Chevron from two gradients

`appearance:none` removes the native arrow. A data-URI SVG cannot read CSS
variables, so it would need one icon per theme. Two `linear-gradient`
triangles in `var(--text-muted)` draw a 5px chevron that follows the theme with
no new token. The gradients live in `background-image`; hover only changes
`background-color`, so the chevron survives the hover. Right padding reserves
room for it.

### 3. Text overflow

The select keeps `max-width:170px` and gets `text-overflow:ellipsis`,
`white-space:nowrap` and `overflow:hidden`, so a long label is cut before the
chevron.

### 4. Opened list

With a transparent select, Chromium on Windows and Linux paints the opened
list from the select's colours. `#execution-history option` gets
`background-color:var(--surface)` and `color:var(--text)` (FR5). macOS uses
the native menu and ignores it.

### 5. Hover and focus

`:hover:not(:disabled)` sets `background-color:var(--hover-bg)` (FR3).
`:focus-visible` sets `outline:2px solid var(--accent);outline-offset:2px`
(FR4), the value used by `.icon-button:focus-visible`. `:focus:not(:focus-visible)`
keeps no outline so a mouse click does not draw the ring.

## Tests

A new `desktop/tests/execution-history.ui.cjs` with a fake agent serving two
executions of one task and one of another:

- at rest: `border-top-width` is `0px` (or border style `none`),
  `background-color` is transparent, `appearance` is `none`,
  `background-image` holds the gradients;
- hover: `background-color` equals the computed `--hover-bg`;
- keyboard focus (Tab from the preceding control, or `focus()` with
  keyboard modality): `outline-style` is `solid`, colour equals `--accent`;
- a settings-dialog select still has a visible border (FR6);
- picking the older entry selects its console; a single-execution task hides
  the drop-down (FR7);
- the same rest assertions under the Light appearance (FR5, US1-4).

The existing UI tests touching the control (console, free-console,
task-header, next-step, discussion-header, task-order-render) run unchanged.

## Changelog

`### Changed` under `## [Unreleased]`: the execution history drop-down of the
desktop toolbar is lighter.
