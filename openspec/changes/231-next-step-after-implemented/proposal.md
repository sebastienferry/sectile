# Offer Adjust or PR creation after implementation in the desktop console

## Why
The desktop console shows one action per workflow stage: `Next: Clarify`,
`Next: Specify`, `Next: Implement`, and, once a task is implemented,
`Next: Review and create PR`. That last action launches the `create_pr` skill,
which since ADR 0004 is a stage-neutral utility: it publishes or refreshes a pull
request and is forbidden to advance the workflow. The task therefore stays at
`implemented`, and the desktop offers the very same button after every run. The
user can end the execution, as #230 made natural, but the workflow never moves.

The server (`internal/db/board.go`, `stageSteps`) and the web board
(`web/src/lib/workflow.ts`) already treat `adjust` as the step that takes a task
from `implemented` to `reviewed`, and the web offers a recovery action when the
implemented task has no pull request yet. The desktop is the only client still
on the pre-ADR-0004 chain.

## What Changes
- After `implemented`, the desktop offers **Next: Adjust** when the task already
  records a pull request, and **Next: Create PR** when it records none.
- **Next: Create PR** launches the pull request creation owner the project has
  configured (`implement`, or `specify` when the project creates its pull
  request at specification time), not the stage-neutral `create_pr` skill. That
  owner is the only path that records the pull request on the task, which is
  what lets the next click become **Next: Adjust**.
- The `create_pr` skill leaves the desktop's next-step chain. It stays available
  as a utility, unchanged.
- The pending `desktop-next-step` requirement, the desktop README and the desktop
  test fixtures stop naming "Review and create PR" as the implemented step.

## Out of Scope
- Any change to the server workflow, the skill contracts, or the web board.
- The desktop transitioning a stage or linking a pull request itself.
- An "Adjust again" action on reviewed tasks; #230 settled the reviewed stage on
  the closing step and **Awaiting human merge**.
- Live forge lookups from the desktop: whether a pull request exists is read from
  the task record the server already returns.

## Impact
Desktop renderer only: `desktop/src/workflow.mjs` (the stage-to-skill chain),
its unit and UI tests, `desktop/README.md`, and the `desktop-next-step`
specification. The agent's `/desktop/tasks` and `/desktop/project` payloads
already carry the two inputs the decision needs, the task's `prUrl` and the
project's `prCreationStage`; no Go change is expected.
