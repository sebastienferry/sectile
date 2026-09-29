# Tasks #637 - Card: autonomous next step instead of an idle full chain

Ordered checklist. Each group leaves the tree buildable.

## 1. Rule (FR1)

- [ ] T1.1 `web/src/lib/workflow.ts`: add `DEFAULT_FULL_CHAIN_STOP_STAGE` and
  `fullChainHasWork(stage, project)`.
- [ ] T1.2 `web/tests/workflowAdjustment.test.mjs`: every stage against both
  stop stages and a project without one (AC1).

## 2. Card (FR2-FR6)

- [ ] T2.1 `TaskCard.tsx`: compute `offersAutonomousStep`; give
  `handleAdvance` its optional spinner argument.
- [ ] T2.2 Full card: render the autonomous-step button instead of `>>` when
  swapped.
- [ ] T2.3 Condensed card: omit "Chaîne complète" when swapped.
- [ ] T2.4 String `autonomousStep` in `shell.ts` (fr, en).

## 3. Tests

- [ ] T3.1 `web/tests/card-autonomous-step.browser.mjs` (real `TaskCard`,
  mocked `useApp`), covering AC2 to AC5.
- [ ] T3.2 Keep `web/tests/condensed-card.browser.mjs` green.

## 4. Changelog (FR8)

- [ ] T4.1 `CHANGELOG.md`, `## [Unreleased]` → `Changed`: one user-facing line.

## 5. Verification

- [ ] T5.1 `npx tsc -b` and `npx oxlint` in `web/` (symlink the main
  checkout's `node_modules` if the worktree has none, then remove it).
- [ ] T5.2 `node --test web/tests/workflowAdjustment.test.mjs`.
- [ ] T5.3 The two browser tests with `PLAYWRIGHT_MODULE` from the main
  checkout.
- [ ] T5.4 `npx vite build` in `web/`; restore `webui/.gitkeep` if the build
  deleted it.
