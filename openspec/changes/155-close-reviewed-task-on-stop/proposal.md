# Offer to close a reviewed task when its execution is stopped

## Why
Stopping an execution from the desktop toolbar is the last thing a user does once
a task has reached `reviewed` and the pull request is out of the agent's hands.
Nothing then proposes the closing step: the footer only says "Awaiting human
merge", and the `handoff` skill — the single path from `reviewed` to `finished`
in this product — is reachable from the web board but from nowhere in the
desktop app. Tasks therefore stay at `reviewed` long after the work is done.

## What Changes
- After a stop request succeeds in the desktop toolbar, read the stopped
  execution's task and, when it sits at the `reviewed` stage and the project
  offers the `handoff` skill, open a dialog offering to close the task.
- Confirming the dialog launches `handoff` for that task through the existing
  server launch path, which transitions the task to `finished`.
- Dismissing the dialog, a free console, an unconfigured project, a project
  without the `handoff` skill, a task at any other stage, or a failed metadata
  read all leave the stop behaviour exactly as it is today.

## Impact
`desktop/src/main.js` (stop handler and a new closing dialog),
`desktop/src/workflow.mjs` (expose the closing step alongside `nextTaskStep`),
`desktop/tests` (new UI test). No server, agent, contract or database change.
