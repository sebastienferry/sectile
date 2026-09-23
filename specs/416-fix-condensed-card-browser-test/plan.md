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
