# Design

## Context
`nextTaskStep(task, project)` in `desktop/src/workflow.mjs` maps a stage to a
`[skillId, label]` pair through a static table; `implemented` maps to
`['create_pr', 'Review and create PR']`. `renderNextStep` in
`desktop/src/main.js` turns the pair into the `Next: <label>` toolbar button and
`launchServerTask(projectId, taskId, skillId, '')` dispatches it with an empty
prompt. `readNextStep` already fetches the full server task through
`/desktop/tasks` (a `models.Task`, which carries `prUrl`) and the project through
`/desktop/project`, whose `server` field is the `agentconfig.Config` and carries
`prCreationStage` next to `skills`. The desktop already reads `task.prUrl` for the
pull request button in the toolbar (`main.js`, `pullRequests` map).

## Decisions

### The implemented step is decided by the task's pull request record
At `implemented`, the next step is `adjust` when `task.prUrl` is a non-empty
string, and the pull request creation owner otherwise. The table entry for
`implemented` becomes a function of `(task, project)` rather than a constant;
the other stages keep their constants.

Rejected: a live forge lookup from the desktop. The forge check already lives in
the adjust skill's own gate (ADR 0004, relaxed for merged pull requests by
ADR 0009), and the desktop has no forge credentials. `prUrl` is the record the
server itself validates on the `reviewed` transition, so it is the right input.
A merged pull request still has a `prUrl` and still leads to Adjust, which is
what ADR 0009 wants.

### Missing pull request: recover through the creation owner, not `create_pr`
When `prUrl` is empty, the step launches `specify` if
`project.server.prCreationStage === 'specified'`, and `implement` otherwise. The
button reads **Next: Create PR** in both cases.

Rejected: keeping the `create_pr` skill. Its contract forbids advancing the
stage, and `transition_stage` with `prUrl` is the only agent-facing path that
persists a pull request on the task, so a `create_pr` run can never turn the
button into **Next: Adjust**: the desktop would loop exactly as it does today.
The creation owner path is the one the web board uses ("Complete PR setup in the
earlier stage", `prRecoverySkill`), and the server already handles it: when
`specify` or `implement` is launched on an implemented task it appends the PR
recovery prompt itself (`internal/db/db.go`, `PR recovery: preserve all accepted
work…`), and `PostBackTask` keeps the stage at `implemented` while recording the
verified `prUrl`. The desktop therefore dispatches with an empty prompt, as it
does for every other step, and adds no recovery wording of its own.

This was the one question the clarification left with a recommendation rather
than a confirmed answer. The specification applies the recommendation; the
requirement below states it as the behaviour, and the ticket comment records
that a reversal means giving `create_pr` a way to record the pull request first.

### Skill availability keeps the existing message
The step is offered only when the chosen skill (`adjust`, `implement` or
`specify`) is present in `project.server.skills`; otherwise the footer shows the
existing `Next skill is unavailable: <label>` message. No new message is added.

### Skill result confirmation needs no change
`desktop/src/skill-result.mjs` expects `implement` to reach `implemented` and
`specify` to reach `specified`. A recovery run ends at `implemented`, which is at
or past both, so the console shows **Skill completed** as it does today. `adjust`
already expects `reviewed`.

### Label
**Create PR** rather than **Review and create PR**: the review is the adjust
skill's job, and the old label described the pre-ADR-0004 chain. **Adjust**
matches the skill name the server and the web use.

## Reconciling the pending changes on the same requirement
`157-desktop-app-ux` renames the requirement to *Contextual next step in the
execution toolbar* and `230-close-step-control` modifies it again; both still
list "Review and create PR" as the implemented step. Neither is archived. This
change modifies the requirement under its archived name and restates the
scenario with the two implemented outcomes; whichever of the three changes is
archived last must carry the scenario text below, and 157's rename still applies.

## Risks
- `desktop/tests/next-step.ui.cjs` and `desktop/tests/workflow.ui.cjs` list
  `create_pr` as the implemented step; both fixtures change. Desktop UI tests
  load `dist/index.html`, so `npx vite build` runs before them.
- A task whose `prUrl` points at a closed, unmerged pull request will offer
  Adjust, and the adjust skill will refuse and ask for recovery. That is the
  server's rule (ADR 0009 ignores closed pull requests) and the desktop does not
  second-guess it; the footer then shows the skill's own report.
