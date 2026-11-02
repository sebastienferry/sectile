# Plan #878 - Clear, outcome-focused skill status in Sectile Desktop

## Stack

Sectile Desktop (Electron, plain ES modules in `desktop/src/`) and the local
agent (Go, `internal/agent/`). No server change, no API change on the server,
no database or migration change: every field read here is already returned by
`GET /api/tasks/{id}/activities`.

Files touched:

- Changed: `desktop/src/skill-result.mjs` (rewritten as the status function),
  `desktop/src/main.js` (`renderTaskSkillStatuses`, the task row built in the
  sidebar render, `renderHeader` / `renderHeaderState` and the `#skill-result`
  element, the launch request), the Desktop stylesheet holding `.skill-result`
  and `.task-skill-status` rules, `internal/agent/agent_desktop.go`
  (`desktopRunResult`, the desktop run record, the launch input),
  `desktop/README.md`, `docs/USER_GUIDE.md` (anchor check only),
  `CHANGELOG.md`.
- Tests: `desktop/tests/skill-result.ui.cjs` (rewritten),
  `desktop/tests/skill-status-reset.ui.cjs` (selectors and labels),
  `internal/agent/agent_skill_result_test.go` (new fields).
- Not touched: `shared/runStates.ts`, `desktop/src/skill-result-refresh.mjs`,
  notifications, the execution queue, the tickets pane (`updateTicketRow`),
  the orchestration overview, the web client.

## Data contract: agent skill-result lookup

`GET` desktop run result (`desktopRunResult`), additive fields only. Both
`activity` and `successor` gain:

| Field | Source (`models.TaskActivity`) | Notes |
|-------|-------------------------------|-------|
| `summary` | `Summary` | verbatim; Desktop extracts the reason |
| `error` | `Error` | verbatim |
| `waitingReason` | `WaitingReason` | `"repository"` for a parked launch |
| `completedAt` | `CompletedAt` | RFC 3339, omitted when nil |
| `disconnected` | `strings.Contains(Summary, models.RunDisconnectNote)` | so Desktop never matches server prose |

`activity` stays a JSON object; it moves from `map[string]string` to
`map[string]any`. `successor.waitingSince` is unchanged. The top-level `task`
object is unchanged. Presence of the `summary` key (even empty) is how Desktop
tells a reason-capable agent from an older one (US4.4).

Size: the summary and error are trimmed to 2000 characters by the agent; the
first line is extracted by Desktop.

## Launch stage (US3.4)

The pickup outcome needs the stage the task was at when the run started.
Desktop already knows the stage it displays (`taskStages`). The launch request
to the agent gains `launchStage` (string, optional); the agent stores it on
the desktop run record (`LaunchStage string json:"launchStage,omitempty"`)
and returns it with the run list, persisted with the run like the other
fields so a restored run keeps it. A run launched elsewhere (orchestrator,
web, another client) has none: its pickup completion reads Needs
verification, which is honest. Only pickups use it.

## Status function (`desktop/src/skill-result.mjs`)

```js
export function primaryStatus(run, result, {nextStep, stageOf} = {})
// -> null | {kind, label, detail, next, reason, diagnostics}
```

- `kind`: `working | waiting | done | verify | failed | stopped | stopping`.
- `label`: e.g. `Done · Specification ready`, `Working · Clarify`.
- `detail`: short text (reason truncated to 120 chars with `…`).
- `next`: hint text or `''` (`Next: Specify`, `Open the console`,
  `Relaunch Implement`).
- `reason`: the full recorded reason text for the details line, `''` when
  none.
- `diagnostics`: `Execution: … · Skill record: … · Task stage: …`, built from
  `runStateLabel(runStateOf(run))`, the matched record's status (and the
  launched skill's when a successor exists: `Skill record: clarify completed;
  now specify running`), and `taskStage(result.task)`.
- `nextStep` is the label of `nextTaskStep(task, project)` computed by
  `main.js` (it needs the project); `null` gives no hint.
- Returns `null` only for no run. Free consoles and discussions return a
  `stopping` status while a stop is pending, and `null` otherwise: the caller
  then renders the run state as today (spec Scope).

Kept from today: `workflowSkill()`, the `expected` stage table, the record
matching (`id`, `taskId`, `skillId`), the successor rule. Removed: `verdict()`,
`skillResult()` and the labels "Skill completed", "Skill failed", "Skill
stopped", "Awaiting stage validation", "Execution ended · skill completion
unconfirmed". The new function walks the precedence table top to bottom, one
`if` per row, with the row number in a comment so a test failure maps back to
the spec.

Outcome table (US3.2): `clarify: 'Clarified'`, `specify: 'Specification
ready'`, `implement: 'Implemented'`, `adjust/review: 'Review complete'`,
`handoff: 'Handed off'`. Pickup: `'Now ' + stage`. Stageless skills
(`create_pr`, `rewrite_story`, `refine_macro`, `realign_macro`, matched after
the same `-issue` / `sectile:` normalisation): `'reported by the skill'`.

Reason extraction: `error` first, else `summary` split on the separator `appendToRunSummary` and `finishRemoteRun`
write (a space, U+2014, a space; escaped as `\u2014` in the JavaScript source) with the parts
containing `RUN_SILENCE_PREFIX` (from `shared/runStates.ts`) and the launch
placeholder `Execution reported by a local agent or native client` dropped;
first non-empty line of what remains. Empty → `reason not recorded`; no
`summary` key → `reason not available`.

Stop reason (US5): `run.cancelRequested` → `Stopped by you`;
`record.disconnected` → `Session disconnected`; else the reason as above.

Process-only rows: row 7 detail from `run.status` (`queued` → `Queued`,
`preparing` → `Preparing`) and `isSilentSummary(run.summary)` → `No output for
a while`. Parked launch (`run.status === 'waiting'` or `waitingReason ===
'repository'`) → `waiting` with `Choose a repository for this ticket`.

## Rendering (`desktop/src/main.js`)

- Glyphs: each kind maps to an existing shared glyph through `runStateSvg`:
  working → `running` (or `silent`), waiting → `waiting`, done → `completed`,
  failed → `failed`, stopped → `canceled`, stopping → `queued`. `verify` gets a
  Desktop-local glyph (lucide `circle-help`, same 14 px box and stroke) in
  `skill-result.mjs` or a small `status-glyphs.mjs`, never in `shared/`.
  Colour per kind through a `data-kind` attribute in the stylesheet; `waiting`
  keeps the amber of the shared waiting state, `verify` uses a neutral
  amber-grey distinct from it (US6.1).
- Task row: the leading `.run-state` element renders the primary status glyph
  (`data-kind`, title and `aria-label` = label, detail, next, diagnostics,
  joined by ` · ` / newline). The `.task-skill-status` badge element is
  removed from skill rows. When `primaryStatus` returns `null` (free console,
  discussion) the row calls `renderRunState` as today.
  `renderTaskSkillStatuses()` becomes the refresher of those leading glyphs
  for rows whose run has a skill (it keeps its name and its call sites).
- Status bar: `#run-state` and `#skill-result` merge into one `#run-status`
  block: glyph + label + detail (+ next) on the first line, the diagnostics
  and the full reason on a second, smaller `.status-details` line.
  `role="status"` stays on the block. For a free console or a discussion,
  `renderHeaderState` keeps its current output. The conversation status row
  hosts the same block (`badgeHome` logic kept).
- Text and DOM are only rewritten when they change (the current
  `textContent!==` guards are kept), so selection, focus and the terminal are
  untouched (FR6).

## Agent tests (`internal/agent/agent_skill_result_test.go`)

- The new fields appear on `activity` and `successor` with the stored values.
- `disconnected` is true only for a summary containing `RunDisconnectNote`.
- A nil `CompletedAt` omits the key; a long summary is trimmed.
- The launch input `launchStage` is kept on the run and returned in the run
  list.

## Desktop tests

- `skill-result.ui.cjs` rewritten: one `test()` per precedence row (1 to 9),
  plus: open console after completion (row 2 with `run.status: 'running'`),
  exit without report (row 8), completion before stage (row 3 then row 2 once
  the label moves), user stop (`cancelRequested` + canceled record → `Stopped
  by you`), disconnection (`disconnected: true` → `Session disconnected`),
  canceled without record or flag (`reason not recorded`), older agent (no
  `summary` key → `reason not available`), successor (running, waiting,
  completed, failed, console exited under a running successor), pickup with
  and without `launchStage`, stageless skill, parked launch, silence sentence
  never used as a reason, 120-character truncation, free console and
  discussion → `null` (or `stopping`), foreign record ignored, diagnostics
  string.
- `skill-status-reset.ui.cjs`: selectors updated to the merged status element;
  its assertions keep checking that a status change preserves selection and
  focus.
- `run-state-animation.ui.cjs`: still passes (the pulsing `running` glyph is
  reused for Working).

## Rejected alternatives

- Keep the run-state glyph and reword the badge: rejected in clarification
  round 2 (question 1).
- Parse the console output for a "current action": not reliable, no skill
  reports steps (clarification, settled choice).
- Match the disconnection by its text in Desktop: duplicates server prose in
  JavaScript; the agent computes `disconnected` from the Go constant instead.
- Add the new kinds to `shared/runStates.ts`: they would leak into the web UI
  and notifications, both out of scope.
- Infer the pickup start stage from activity history: the activities do not
  record stage moves with their run; the launch stage sent by Desktop is
  explicit and verifiable.

## Implementation notes (changes to this plan)

Recorded during implementation; the acceptance criteria of the spec are
unchanged.

- **Launch stage kept by Desktop, not by the agent.** The desktop run record is
  rebuilt in five places in the agent (launch, dispatch, conversation, headless,
  macro), and the run id is not known when Desktop sends the launch, so a
  `launchStage` field would have to be threaded through all of them. Desktop
  instead records, for a pickup, the task stage of the first skill result it
  reads while the run is still going and its record has not ended
  (`noteLaunchStage` in `main.js`), persisted in `localStorage` and pruned with
  the runs. Stages only move forward, so a stage read late can only make a
  pickup read Needs verification, never a Done nothing supports (US3.4). No
  launch request change, no agent run-record change.
- **Header and status bar.** The run-state element sits in the toolbar beside
  the task ID, not in the status bar. It now hosts the primary status (glyph
  and label); `#skill-result`, in the status bar or the conversation status
  row, carries its detail and next step and the details line (diagnostics,
  full reason). Both render from the same `primaryStatus()` call, so they
  cannot disagree; no `#run-status` merge was needed.
- **Glyphs.** `desktop/src/status-glyphs.mjs`. A Working status that is queued
  or preparing keeps the shared queued glyph, so it does not pulse.
- **Tests.** The pure status tests moved from `skill-result.ui.cjs` to
  `skill-result.test.mjs` and run in `npm test`. The agent's new fields are
  tested on `desktopSkillRecord()` directly.
