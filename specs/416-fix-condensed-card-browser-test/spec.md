# Fix the condensed card browser test failing on useApp outside its provider

Clarification: `docs/clarifications/416.md`.

## User story (P1)

As a Sectile contributor, I run `web/tests/condensed-card.browser.mjs` and it
passes on `main`, so it keeps guarding the condensed card against regressions.

## Requirements and acceptance scenarios

1. Given a checkout path without `#` and `PLAYWRIGHT_MODULE` pointing at an
   installed Playwright `index.mjs`, when the condensed card browser test runs,
   then it passes and reports no page error.
2. Given the fixture, when the card renders, then the real `TaskCard` and every
   component it renders receive the mocked context, and only the context
   source is mocked.
3. Given a component added under `src/components/` and rendered by the card
   later, when the test runs, then it receives the mocked context too rather
   than the real provider hook.
4. Given the other browser tests, when they run after the change, then each one
   that passed before still passes.

## Out of scope

Any change to `TaskCard` or its children, to the condensed card assertions,
and to other fixtures. No changelog entry: nothing a user sees changes.
