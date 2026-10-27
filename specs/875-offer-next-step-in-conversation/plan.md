# Plan #875 - Offer the next step in a task conversation

Spec: `spec.md`. This file says how; `spec.md` says what.

## Stack and boundaries

- Sectile Desktop renderer only: plain ES modules under `desktop/src/`,
  tests with `node --test` (`desktop/tests/*.test.mjs`) and the Electron UI
  suite (`desktop/tests/*.ui.cjs`).
- No change to the server, the agent, the database, the MCP tools or the
  skills. The pieces the offer needs already exist:
  - the next step: `nextTaskStep` in `desktop/src/workflow.mjs`;
  - skill completion and stage validation: `skillResult` in
    `desktop/src/skill-result.mjs` (`kind: 'completed'` only once the task has
    reached the skill's expected stage; it already speaks for a successor skill
    started in the same console, #586);
  - the conversation's idle state, approvals and questions: the agent's
    `GET` conversation response (`busy`, `readOnly`, `approvals`), polled by
    `desktop/src/conversation.js`; the declared wait: `run.waitingSince`;
  - the launch path and its checks: `launchTaskWork` in `desktop/src/main.js`
    (`readNextStep` freshness re-read, `api.runs()` busy re-read, server 409
    with `activeRunId`, `submittingSteps` / `submittedSteps`);
  - closing a conversation: `api.stop(runId)`; a stopped conversation already
    reopens as `Read-only history`.
- Why the server needs nothing: a launched skill reuses its launch activity as
  its run (`SECTILE_RUN_ID`) and ends it with `finish_run`. When `skillResult`
  says "completed", that activity is already `completed`, so the
  `idx_activities_one_active_run` rule does not count it, and the successor's
  launch is not refused because of the predecessor. Only Desktop's local guard
  (`activeRun(run)` on the still-`running` conversation process) blocks today.

## Architecture

### 1. Eligibility, a pure module

New `desktop/src/next-step-offer.mjs`:

```js
// nextStepOffer({run, result, conversation, task, project, runs, submitting})
//   -> {skillId, label} | {reason}
export function nextStepOffer(input)
// Whether `item` counts as active on the task for the launch guard, the
// eligible predecessor excepted.
export function blocksLaunch(item, predecessorId)
```

Inputs:

- `run`: the selected run (`conversation`, `taskId`, `provider`, `skill`,
  `status`, `waitingSince`, `kind`).
- `result`: `skillResults.get(run.id)`, the agent's skill-result read.
- `conversation`: the last polled conversation state `{busy, readOnly,
  approvals}`.
- `task`, `project`: `nextStepData.task`, `nextStepData.project`.
- `runs`: the polled run list; `submitting`: whether `submittingSteps` or
  `submittedSteps` holds the task key.

Rules, in order, each returning a `reason` (used by tests, not shown):

1. `run.conversation && run.taskId`, provider `claude`, `codex` or `agy`, not
   `freeConsole`, not a macro run, skill not `discuss`.
2. `conversation` known, `!readOnly`, `!busy`, `approvals.length === 0`,
   `!run.waitingSince`.
3. The skill spoken for is `workflowSkill(result.successor?.skillId)` when a
   successor exists, else `run.skill`; it must be one of `clarify`, `specify`,
   `implement`, `adjust` (`handoff` ends at `finished`, which has no next
   step).
4. `skillResult(run, result)?.kind === 'completed'`.
5. `taskStage(task) !== 'finished'`; `nextTaskStep(task, project).skillId`
   set.
6. No run of the same task key other than `run` is active
   (`activeRun`), and `!submitting`.

`blocksLaunch(item, predecessorId)` is `activeRun(item) && item.id !==
predecessorId`; `launchTaskWork` uses it only when called from the offer.

### 2. Rendering, in the conversation view

`createConversationView` (`desktop/src/conversation.js`) gains:

- `onState(callback)`: called after each poll with `{runId, busy, readOnly,
  approvals}` so `main.js` keeps the conversation state the module needs, and
  re-evaluates the offer at the conversation's poll pace (250 ms busy, 750 ms
  idle) as well as on `renderNextStep`.
- `showOffer(offer)` / `showOffer(null)`: one `<div class="conversation-offer"
  role="group" aria-label="Next workflow step">` appended after the last
  event, holding a primary `<button>` `Next: <label>`, a hint `Starts a new
  conversation for this step and closes this one.`, and a
  `role="status"` line for progress and errors. It is redrawn in place (no
  flicker, focus kept) and removed by `select()`.
- `offerState({pending, error})`: disables the button and shows
  `Launching <label>…`, or re-enables it and shows the error.

### 3. Launch, in `main.js`

- `renderNextStep()` computes `nextStepOffer(...)` for the selected run and
  calls `conversation.showOffer(...)`. The toolbar part of `renderNextStep`
  is unchanged.
- `launchTaskWork(kind, force, {predecessor})` gains an optional third
  argument. With `predecessor` set:
  - the local guard and the post-read `abandoned` check use
    `blocksLaunch(item, predecessor.id)` instead of `activeRun(item)`;
  - the freshness check also re-reads the predecessor's conversation state and
    skill result, and abandons when `nextStepOffer` no longer returns the same
    `skillId`;
  - the launch passes `'conversation'` as the console view;
  - on success, after `refresh()` finds the new run: `select(launched)`, then
    `api.stop(predecessor.id)`; a failure of that stop is reported in the
    toolbar error line and leaves both runs as they are;
  - on failure: the error goes to `conversation.offerState({error})`, the
    predecessor stays selected, nothing is stopped. A 409 with `activeRunId`
    is shown as `A run is already active on this task.` and still feeds
    `forceableLaunches`, so the toolbar's `Launch anyway` is the force path.
- "Confirmed launch" means: `api.launchServerTask` resolved and the run list
  holds a run of the task that was not in `latestRuns`. If the launch resolves
  but no new run shows within the existing `submittedSteps` wait, the
  predecessor is not stopped; it is stopped on the poll that first shows the
  successor, as long as the offer launched it (tracked in a
  `pendingHandover` map keyed by task key, cleared on selection change of the
  task or on stop).

### 4. Fresh context

The successor is a plain server launch (`api.launchServerTask`) with an empty
prompt: the agent starts a new conversation with the skill's command and no
`--resume`. Nothing from the predecessor's transcript is passed. Persistence
before handover is the skill's contract (it validated its stage, which is the
offer's precondition).

### 5. Styles

`desktop/src/style.css`: `.conversation-offer` reuses the tool-approval card
spacing and the primary button style of the composer; no new colour token.

## Data contracts

No new endpoint or field. Read: agent conversation `GET` (`busy`,
`readOnly`, `approvals`), `/desktop` runs (`status`, `waitingSince`,
`conversation`, `provider`, `skill`, `taskId`), skill result
(`activity`, `successor`, `task`), server task and project. Written: the
existing launch call and `stop`.

## Target files

| File | Change |
| --- | --- |
| `desktop/src/next-step-offer.mjs` | new, eligibility rules |
| `desktop/src/conversation.js` | `onState`, `showOffer`, `offerState` |
| `desktop/src/main.js` | wire the offer into `renderNextStep`, `launchTaskWork` predecessor mode, handover |
| `desktop/src/style.css` | `.conversation-offer` |
| `desktop/tests/next-step-offer.test.mjs` | new, unit tests |
| `desktop/tests/conversation-next-step.ui.cjs` | new, UI tests on the fake agent of `conversation.ui.cjs` |
| `CHANGELOG.md` | one `Added` line |
| `desktop/README.md` | mention the offer where the conversation view is described, if it is |

## Rejected alternatives

- **Continue in the same conversation** (send the next skill as a message to
  the predecessor). Round 1 recommended it; round 2 reversed it: the next step
  must start from persisted state, not from a context the predecessor holds.
- **Relax the server's one-active-run rule.** Not needed: the predecessor's
  activity is already over on the server. Changing the database rule would
  weaken the duplicate check for every client.
- **Enable the toolbar's `Next:` for the idle predecessor too.** The
  clarification keeps the toolbar action as it is; widening it is a separate
  decision.
- **Stop the predecessor before launching.** A failed launch would leave the
  owner with no open conversation; the clarification requires the opposite
  order.

## Risks

- The conversation state and the skill result are polled at different paces;
  the offer may lag the stage move by up to one skill-result settle cycle
  (`SETTLE_MS`). Acceptable: it never shows early, only late.
- Antigravity takes a message sent during a turn as a queued message rather
  than joining the turn. The offer only shows on an idle conversation, so it
  is unaffected; the predecessor check re-reads `busy` before launching.
- Antigravity skill completion: a task launch keeps its environment,
  `SECTILE_RUN_ID` included, in `run.conversation.env`
  (`internal/agent/agent.go`, `startLaunchConversation`), and every Antigravity
  turn merges it into `agy`'s environment
  (`internal/agent/agent_agy_conversation.go`, `agyConversationTurn`), whose
  Sectile MCP is checked with `agy mcp list`. A skill can therefore end its run
  as under Claude and Codex. Should one not, its conversation never reaches
  "Skill completed": the failure mode is a missing offer, never a premature
  one.
- Codex keeps an app-server process per conversation; stopping the predecessor
  after the successor started in the same directory must not touch the
  successor. `api.stop` is per run id, so it does not.
