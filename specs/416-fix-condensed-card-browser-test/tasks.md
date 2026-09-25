# Tasks

- [x] Rewrite `useApp` for every module under `/src/components/`.
- [x] Add the context fields the card's children read.
- [x] Run the condensed card browser test and every other browser test.
- [x] Run `npm test` in `web/`.
- [x] Render the card condensed through its `compact` prop; align the
      full-chain label and the `advanceTask` argument check (see plan).

## Validation

- `node tests/condensed-card.browser.mjs` (from `web/`, Playwright from
  `desktop/node_modules`): passed three times in a row.
- Browser suite: 10/10 pass (baseline 8/10).
- `npm test` in `web/`: 189 passed, 0 failed.
