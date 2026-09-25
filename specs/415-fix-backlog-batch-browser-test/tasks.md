# Tasks

- [x] Provide `projects` and a matching `currentProject` in the fixture context.
- [x] Give one fixture task a parent and assert the single epic bar row.
- [x] Run the backlog batch browser test and every other browser test; compare
      with the baseline.
- [x] Run `npm test` in `web/`.

## Validation

- `node tests/backlog-batch.browser.mjs` (from `web/`, Playwright from
  `desktop/node_modules`): passed.
- Mutation check: with `epicColors: false` in the fixture, the new epic bar
  assertion times out, so it does exercise the #388 path.
- Browser suite: 9/10 pass; `condensed-card` still fails as on the baseline
  (handled by #416).
- `npm test` in `web/`: 189 passed, 0 failed.

## Validation after merging origin/main (2026-09-25)

- Merging `main` brought `priority-select.browser.mjs`, now rooted through
  `browserRoot.mjs` (#417 requirement 3), and made `issue-detail-layout` fail
  as it already did on `origin/main`: `PrioritySelect` (#436) reads `useApp`,
  which that fixture only mocked in `TaskDetailModal`. It now mocks `useApp` in
  every component, as `condensed-card` does (#416).
- Browser suite: 11/11 pass. `npm test` in `web/`: 268 passed, 0 failed.
  `tsc -b` clean; `oxlint` 0 errors.
