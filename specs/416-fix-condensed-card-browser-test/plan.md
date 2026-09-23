# Implementation plan

Test-only change in `web/tests/condensed-card.browser.mjs`.

- Transform: rewrite `import { useApp } from '../context/AppContext'` to
  `const useApp = () => window.ctx` in every module whose id contains
  `/src/components/`, with the regular expression already used by
  `backlog-batch.browser.mjs` (either quote).
- Context: add `projects: [{ id: 'p' }]` and the matching `currentProject`
  (read by `CopyTaskSkillMenu` and `EpicMarker`), `skillCommand` returning its
  default command (`CopyTaskSkillMenu`), an async no-op `fetchActivities` and a
  spied `addToast` (`RemoteRunBadge`).
- Leave the assertions as they are.

Validation: every `web/tests/*.browser.mjs` from the batch worktree, and
`npm test` in `web/`.

## Implementation notes

Mocking the children let the card render, which uncovered two older drifts
between the test and the card, both fixed in the fixture without changing an
assertion's intent:

- Since `3cd5476` (simplified card view), a card is condensed by its `compact`
  prop, which `BoardView` passes in the condensed board mode, and no longer by
  the `compact` density. The harness now renders `TaskCard` with
  `compact={window.condensed}`; the detailed-density loop clears the flag and
  the steps that return to the condensed card set it again.
- Since #123, the full-chain menu entry reads "Chaîne complète" instead of
  "Avancer automatiquement", and `advanceTask` also receives the launch mode
  and model. The test uses the new label and compares the first three recorded
  arguments (action, task, `auto` flag), which is what it asserted before.
