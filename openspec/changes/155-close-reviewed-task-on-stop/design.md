# Design

## Context
The desktop renderer already reads task workflow state for the console footer
(`readNextStep` in `desktop/src/main.js`, `nextTaskStep` in
`desktop/src/workflow.mjs`) and already launches skills through
`api.launchServerTask(projectId, taskId, skillId, prompt)`. The desktop bridge
(`desktop/electron/preload.cjs`) exposes no stage-transition call, so closing a
task can only happen by running a skill.

## Decisions

### The closing step is the `handoff` skill
`web/src/lib/workflow.ts` maps stage `reviewed` to the `handoff` skill, and
`desktop/src/skill-result.mjs` already declares `handoff -> finished`. The
desktop reuses that mapping instead of inventing a direct transition, so a single
definition of "what closes a task" stays true across surfaces.

Rejected: adding a `transition` IPC channel and calling the server stage API from
the renderer. It would create a second, skill-free way of reaching `finished`,
bypassing the handoff report and local cleanup the skill performs.

### The offer comes after the stop, never before
`api.stop` runs first and unchanged. The offer is computed afterwards, from a
fresh server read. A failed stop, a failed task read, or a missing skill produces
no dialog and no error: the user asked to stop, and that is what happened.

Rejected: a confirmation dialog in front of the stop. It would add a click to the
common path, where the task is not at `reviewed` at all.

### Eligibility is computed by `closingStep` in `workflow.mjs`
A small exported function next to `nextTaskStep`, taking the same `(task,
project)` pair and returning the skill id or `null`. Keeping it in the pure
module lets the rule be unit-tested with the rest of the workflow logic, and
keeps `main.js` to rendering and dispatch.

### The dialog reuses `showDialog`/`paragraph`
Same helpers as the archive and relaunch dialogs, so focus handling, dismissal
and styling need no new code. Launch failures are written into the dialog's
status line, which is how `openAgentConsole` and the quick-add form already
report them.

## Risks
Low. The change is additive in the renderer; every failure path degrades to
today's behaviour.
