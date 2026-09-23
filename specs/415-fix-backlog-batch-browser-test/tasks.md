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
