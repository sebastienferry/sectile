# Plan #710 - Pick the changed file from a drop-down

## Stack

Sectile Desktop renderer: plain ES modules bundled by Vite, CSS in
`desktop/src/style.css`, UI tests with Playwright's Electron driver
(`desktop/tests/*.ui.cjs`, run by `node --test` after `npx vite build`).

## Architecture

The Changes panel is built once by `createGitDiff` in
`desktop/src/gitDiff.js`. Its markup holds `.diff-body`, a two-column grid of
`nav.diff-files` (buttons) and `.diff-detail`.

The change:

1. Markup: replace `<nav class="diff-files">` by
   `<select class="diff-files" aria-label="Changed file" hidden></select>`,
   placed before `.diff-body`, which then holds only `.diff-detail`.
2. `render`: fill the select with one `<option>` per file (`value` = path,
   `textContent` = option label), set its value to `selection`, show it when
   there is at least one file.
3. `showFile`: no more `aria-pressed` loop; set the select's value to
   `selection` so it stays in sync.
4. `clear`: empty and hide the select.
5. One `change` listener, set once next to the other handlers:
   `selection=select.value;showFile()`.
6. CSS: drop the grid of `.diff-body` and the `.diff-files` button rules,
   the `.split` and `@media(max-width:750px)` overrides of them; style
   `.diff-files` as a full-width select (`width:100%`, `max-width:100%`,
   `text-overflow:ellipsis`, a bottom margin), with the focus ring of the
   other Changes controls.

No data contract changes: `api.gitDiff` and its payload are untouched.

## Target files

- `desktop/src/gitDiff.js`
- `desktop/src/style.css`
- `desktop/tests/git-diff.ui.cjs`
- `desktop/tests/markdown-view.ui.cjs`
- `CHANGELOG.md`

## Rejected alternatives

- A custom popover list: more code for keyboard and screen-reader support
  that a native select gives for free.
- Keeping the list in the full view and the drop-down only in the split view:
  two code paths and two test sets for one choice.
