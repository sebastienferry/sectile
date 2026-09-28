# Plan #526 - Shell and board translation

## Approach

Inventory each file of FR1, move each Sectile-owned literal to the `shell`
namespace (`web/src/locales/shell.ts`, D1), grouped by component
(`shell.sidebar`, `shell.projectPicker`, `shell.header`, `shell.pinned`,
`shell.board`, `shell.list`, `shell.card`, `shell.filters`, `shell.palette`,
`shell.scale`, `shell.statusBar`, `shell.batch`). Existing keys (`t.nav`,
`t.board`, `t.list`, `t.status`, `t.priority`) are reused when the wording is
identical.

Helpers outside React (`lib/boardColumns.ts`, `lib/boardSort.ts`,
`lib/boardGrouping.ts`) that return French display text take the strings as a
parameter, or return a key the component resolves; their stored values
(sort keys, grouping keys, column ids) are unchanged.

## Data contracts

None: no API, storage or settings change.

## Target files

`web/src/locales/shell.ts`, the components of FR1, the `lib/board*.ts`
helpers that produce display text, `web/tests/shellCatalog.test.mjs`.

## Tests

- Unit: a catalog test asserting representative English values and the
  French values unchanged for the shell (picker, card actions, fallback
  heading, count plural forms).
- Browser: `tests/condensed-card.browser.mjs` and the other French
  harnesses keep passing unchanged (D8); `tests/i18n-shell.browser.mjs`
  (#534) renders the card with `translations.en` and asserts English
  accessible names.
