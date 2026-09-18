# Tasks

## 1. Next-step chain
- [ ] 1.1 In `desktop/src/workflow.mjs`, replace the constant `implemented` entry with a resolver taking `(task, project)`: `adjust` / `Adjust` when `task.prUrl` is a non-empty string, otherwise `specify` when `project.server.prCreationStage === 'specified'` and `implement` otherwise, both labelled `Create PR`.
- [ ] 1.2 Keep the existing guards in `nextTaskStep`: unconfigured project message, and `Next skill is unavailable: <label>` when the resolved skill is not in `project.server.skills`.
- [ ] 1.3 Remove `create_pr` from the desktop chain; do not touch the skill itself or the server.

## 2. Unit tests
- [ ] 2.1 In `desktop/tests/workflow.ui.cjs`, replace the `implemented -> create_pr` expectation with four cases: `prUrl` set -> `adjust`; no `prUrl` and `prCreationStage` `implemented` (or absent) -> `implement`; no `prUrl` and `prCreationStage` `specified` -> `specify`; resolved skill missing from the project -> no `skillId` and the unavailable message.
- [ ] 2.2 Assert the labels `Adjust` and `Create PR`.

## 3. UI test
- [ ] 3.1 In `desktop/tests/next-step.ui.cjs`, replace `create_pr` in the project fixture with `adjust` and add `prCreationStage` to the `server` payload.
- [ ] 3.2 Add a step where task A is `implemented` without `prUrl` and the button reads `Next: Create PR`, then the fixture gains a `prUrl` and the button reads `Next: Adjust` after re-selection.
- [ ] 3.3 Assert the dispatched launch payload for `Next: Create PR` names the creation owner skill with an empty prompt.

## 4. Documentation
- [ ] 4.1 Update the **Next workflow step** section of `desktop/README.md`: list `Next: Adjust` and `Next: Create PR`, and one sentence on how the desktop picks between them.
- [ ] 4.2 Add a `.agents/MEMORY.md` note only if implementation uncovers a non-obvious trap; the stage-neutral nature of `create_pr` is already recorded in ADR 0004.

## 5. Verification
- [ ] 5.1 Run `npx vite build` in `desktop/`, then the desktop tests (`workflow.ui.cjs`, `next-step.ui.cjs`, `task-closure.ui.cjs` unchanged and passing).
- [ ] 5.2 Run `openspec validate 231-next-step-after-implemented --strict`.
