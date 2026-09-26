# Tasks #500 - Current and Next labels on the desktop workflow button

Ordered checklist. Each group is one commit (Conventional Commits) and leaves
the tree buildable. Merge `origin/main` first (the spec branch is already
pushed: merge, do not rebase).

## 1. Skill label helper (spec definitions)

- [x] T1.1 Export `skillLabel(skillId)` from `desktop/src/workflow.mjs`
  (plan decision 1).
- [x] T1.2 Unit tests in a new `desktop/tests/workflow.test.mjs`: every workflow id maps to its label (`create_pr` ->
  `Create PR`), an unknown id is capitalized (`pickup` -> `Pickup`,
  `discuss` -> `Discuss`), an empty or missing id returns `''`. Run
  `npm test` in `desktop/`.

## 2. Current label in the toolbar (FR1-FR8, US1-US3)

- [x] T2.1 `submittingSteps` becomes a `Map` key -> skill id; update its
  `add`/`delete`/`has` call sites in `desktop/src/main.js` (plan decision 2).
- [x] T2.2 `renderNextStep()`: resolve `current` from the most recent active
  run, then the submitting skill, then the submitted skill; show
  `Current: <skillLabel>` disabled when set, the existing `Next:` branch
  otherwise; keep the loading/error early returns first (plan decision 3).
- [x] T2.3 Extend `desktop/tests/next-step.ui.cjs`:
  - after the `implement` launch, the button reads `Current: Implement` and
    is disabled while the run is queued (US2.1), and still reads it when the
    older `old` console is selected in the history (US1.5);
  - once `active=false`, it reads `Next: <stage step>` and is enabled
    (US3.1/US3.2);
  - the `Create PR` launch reads `Current: Implement` (US2.2);
  - a run whose skill differs from the stage step (for example a queued
    `pickup` on a task at `new`, or an `adjust` on a task at `clarified`)
    reads `Current: Pickup` / `Current: Adjust` (US1.2/US1.3);
  - a launch failure returns to `Next: <step>` enabled (US2.3, existing
    assertion kept);
  - optional: an active `discuss` run on a `#finished` task shows
    `Current: Discuss` (FR7).
- [x] T2.4 Update any lookup in the other `*.ui.cjs` files that searches the
  button by `Next: <label>` while a run is active.

## 3. Changelog (FR9)

- [x] T3.1 Add under `## [Unreleased]` a `### Changed` section (create it
  after `### Added` if absent) with one line, for example:
  "**The desktop workflow button says what is running.** While an execution
  of the selected task is active or being launched, the console toolbar
  button reads `Current: <skill>` (for example `Current: Pickup`) instead of a
  greyed-out `Next:`; it proposes `Next: <step>` again once the execution
  ends. (#500)"

## 4. Verification

- [x] T4.1 `cd desktop && npm test`.
- [x] T4.2 `cd desktop && npx vite build && npm run test:ui` (whole suite;
  restore `internal/webui/dist/.gitkeep` if a build removed it).
- [ ] T4.3 (not run by the agent: the Electron UI tests cover these paths; left to the reviewer) Manual check in the desktop app against a throwaway server (never
  the dev database): launch a step, watch `Current:` then `Next:`; launch a
  pickup and check `Current: Pickup`.
- [x] T4.4 `git status` clean before the `implemented` transition.
