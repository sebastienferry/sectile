# Tasks #515 - Next-step and full-chain icon buttons in the desktop toolbar

Ordered checklist. Each group is one commit (Conventional Commits) and leaves
the tree buildable. Merge `origin/main` first (the spec branch is already
pushed: merge, do not rebase).

## 1. Icon next-step button and step badge (US1, FR1-FR3)

- [x] T1.1 Markup: `#next-step` gets the `icon-button` class; add
  `<span id="next-step-label" class="step-badge" aria-hidden="true" hidden>`
  after the future `#pickup-chain` slot (plan decision 1).
- [x] T1.2 `iconPaths['next-step']` single chevron (plan decision 2).
- [x] T1.3 `renderNextStep()`: a helper sets `aria-label`, `title` and the
  badge text to the `Next:` / `Current:` string instead of `textContent`;
  the badge is visible exactly when `#next-step` is (plan decision 3).
- [x] T1.4 `.step-badge` style; `#next-step` keeps its colours (plan
  decision 7).
- [x] T1.5 `desktop/tests/next-step.ui.cjs`: replace the `textContent`
  assertions by `aria-label` assertions, add badge-text assertions for
  `Next: Specify` (idle) and `Current: Implement` (queued run, older console
  selected), check the badge is hidden with `>` on a finished idle task.

## 2. Full-chain button (US2, US3, FR4-FR10)

- [ ] T2.1 Markup `#pickup-chain` (`icon-button`, `aria-label` / `title`
  `Pickup (full chain)`, `hidden disabled`) between `#next-step` and
  `#next-step-label`; `iconPaths['pickup-chain']` double chevron.
- [ ] T2.2 `readNextStep()` returns the project (plan decision 4);
  `pickupAvailable(project)` helper.
- [ ] T2.3 `renderNextStep()`: `>>` visibility and enablement (plan decision
  3; FR5, FR6, FR10).
- [ ] T2.4 `launchNextStep(force)` becomes `launchTaskWork(kind, force)`;
  `>>` sends `pickup` with mode `autonomous`; kind-aware recheck; `kind`
  stored in `submittedSteps` so a pickup in flight is not dropped by the
  stage-step comparison (plan decision 5).
- [ ] T2.5 `forceableLaunches` becomes a `Map` key -> kind; "Launch anyway"
  re-sends the refused kind (plan decision 6).
- [ ] T2.6 `#pickup-chain` joins the `#next-step` style rules.
- [ ] T2.7 Extend `desktop/tests/next-step.ui.cjs` (add `pickup` to the stub
  project's skills where needed):
  - `>>` visible and enabled on an idle task at `new`, named
    `Pickup (full chain)` (US2.1);
  - clicking it posts `{taskID, skillID:'pickup', prompt:'', mode:'autonomous'}`
    and selects the new console (US2.2);
  - while that launch is queued, `>` and `>>` are disabled and the badge reads
    `Current: Pickup` (US2.3), with no `Next:` flash between the click and the
    run's appearance;
  - a run started elsewhere disables `>>` without hiding it (US3.1);
  - a failed `>>` launch shows `Could not launch full chain` and re-enables
    both buttons (US2.5);
  - a stage change between display and click does not abandon a `>>` launch;
    a run that became active does (FR8);
  - `>>` hidden without a `pickup` skill (US3.2), on a finished idle task
    (US3.3), and on a finished task with an active `discuss` run while `>`
    reads `Current: Discuss` (FR10);
  - `>` still posts no `mode` (FR7).
- [ ] T2.8 Update the DOM order assertions of `next-step.ui.cjs` to
  `#stop, #next-step, #pickup-chain, #next-step-label, #mark-reviewed,
  #retry-next-step, #force-next-step`.
- [ ] T2.9 `desktop/tests/concurrent-launch.ui.cjs`: a `>>` launch refused
  with the duplicate error reveals "Launch anyway", which posts `pickup` with
  `mode:'autonomous'` and `force:true` (US2.4, FR9); the existing next-step
  refusal case still re-sends the next step.

## 3. Changelog (FR12)

- [ ] T3.1 Under `## [Unreleased]` / `### Changed`, one line, for example:
  "**Next step and full chain from the desktop toolbar.** The console toolbar
  of Sectile Desktop launches the selected task's next step from a `>` button
  and its whole workflow, unattended, from a `>>` button, as the web task card
  does; a small badge next to them reads `Next: <step>` or, while something
  runs, `Current: <skill>`. (#515)"

## 4. Verification

- [ ] T4.1 `cd desktop && npm test`.
- [ ] T4.2 `cd desktop && npx vite build && npm run test:ui` (whole suite;
  restore `internal/webui/dist/.gitkeep` if a build removed it).
- [ ] T4.3 Screenshot of the toolbar (the `next-step.ui.cjs` screenshot) in
  the dark and light appearances, checked for layout and contrast.
- [ ] T4.4 `git status` clean before the `implemented` transition.
