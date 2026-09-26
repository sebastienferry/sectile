# Fix the backlog batch browser test broken by epic colours

Clarification: `docs/clarifications/415.md`.

## User story (P1)

As a Sectile contributor, I run `web/tests/backlog-batch.browser.mjs` and it
passes on `main`, so a red run means the backlog batch regressed rather than
the fixture falling behind the components it renders.

## Requirements and acceptance scenarios

1. Given a checkout path without `#` and `PLAYWRIGHT_MODULE` pointing at an
   installed Playwright `index.mjs`, when the backlog batch browser test runs,
   then it passes and reports no page error.
2. Given the mocked application context of that test, when `ListView` and the
   components it renders read the project list, then they receive an array that
   holds the current project.
3. Given the current project enables epic colours and exactly one listed task
   has a parent, when the backlog renders, then that task's row carries an epic
   bar and no other row does.
4. Given the other browser tests, when they run after the change, then each one
   that passed before still passes.

## Out of scope

Any change to backlog batch behaviour, to the epic colour rules, to
`epicColorsEnabled` (no defensive guard), and to other fixtures (#416).
No changelog entry: nothing a user sees changes.
