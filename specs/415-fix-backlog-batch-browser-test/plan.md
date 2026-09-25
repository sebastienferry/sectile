# Implementation plan

Test-only change in `web/tests/backlog-batch.browser.mjs`.

- Harness: define one project `{ id: 'p', epicColors: true }`, expose it as
  `window.ctx.projects` and `window.ctx.currentProject`, so `useEpicColors`
  (`src/components/EpicMarker.tsx`) receives the list it reads.
- Give the `specified` fixture task a `parentKey`. It is the one row the
  existing scenario never selects for a launch, so the batch assertions keep
  their meaning.
- After the page loads, assert that exactly one `[data-epic-bar]` is drawn and
  that it sits in the `Ticket specified` row.
- Keep `epicColorsEnabled` unchanged (decision 1 of the clarification).

Validation: run every `web/tests/*.browser.mjs` from the batch worktree (a
path without `#`) with the Playwright module from `desktop/node_modules`, and
`npm test` in `web/`. Baseline on `origin/main` 9f400fe: 8/10 browser tests
pass, `backlog-batch` and `condensed-card` fail.
