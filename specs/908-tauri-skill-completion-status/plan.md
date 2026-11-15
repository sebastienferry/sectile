# Plan #908 - Tauri: show skill completion status in ticket conversations

Specification: `spec.md`. Clarification: `docs/clarifications/908.md`.
Reference implementation: `desktop/src/skill-result.mjs`,
`desktop/src/skill-result-refresh.mjs`, the `launchStages` handling in
`desktop/src/main.js` (Electron, #878).

## Stack and constraints

- React 19 / TypeScript in `desktop-tauri/src`. Pure domain code lives in
  `desktop-tauri/src/domain/` and is tested with `node --test`
  (`npm test`, files `tests/*.test.ts`, run with `--experimental-strip-types`:
  relative imports carry the `.ts` extension, no enums, no parameter
  properties). Components are tested with Vitest and Testing Library
  (`npm run test:ui`, files `tests/*.ui.test.tsx`).
- `desktop-tauri` imports nothing from `desktop/` or `shared/`. The
  reconciliation, the refresh rule and the few `shared/runStates.ts` helpers
  they need are **ported**, not imported.
- The result comes from the existing Tauri command `run_result`
  (`runResult(runId)` in `src/lib/workspace.ts`), which proxies the agent's
  `GET /desktop/run-result` (`internal/agent/agent_desktop.go`,
  `desktopRunResult` / `desktopSkillRecord`). No Rust or Go change.
- The run list comes from `useExecutionMonitor()` (poll every 3 s). A
  conversation's turn and pending questions come from `useConversation(runId)`
  (`snapshot.busy`, `snapshot.approvals`), a `useSyncExternalStore` over an
  already loaded controller; the sidebar never fetches a transcript.
- A conversation run's `waitingSince` may be the Go zero time
  (`0001-01-01T00:00:00Z`); `statusLabel` already treats it as no mark.
- User-visible strings stay in English, as the Tauri interface is today.

## Design

### 1. Result type (`src/domain/workflow.ts`)

Widen `RunResult` to what the agent sends:

```ts
export interface SkillRecord {
  id: string; taskId: string; skillId: string; status: string;
  summary?: string; error?: string; disconnected?: boolean;
  waitingReason?: string; completedAt?: string;
}
export interface RunResult {
  activity?: SkillRecord | null;
  successor?: (SkillRecord & { waitingSince?: string }) | null;
  task?: { status: string; labels?: string[] };
}
```

Remove `skillResultLabel` and its tests. `taskStage` is reused on
`result.task` as `skillResultLabel` did (`{id:run.taskId,title:"",...task}`).

### 2. Reconciliation (`src/domain/skill-status.ts`, new, pure)

```ts
export type SkillStatusKind = "working"|"waiting"|"done"|"verify"|"failed"|"stopped"|"stopping";
export interface SkillStatus {
  kind: SkillStatusKind; label: string; detail: string; next: string;
  reason: string; diagnostics: string;
}
export interface SkillStatusOptions {
  nextStep?: string;      // label of nextSkill(...) when it has a skillId
  launchStage?: string;   // pickup start stage, '' when unknown
  asking?: boolean;       // the conversation has a pending question (A1)
}
export function skillStatus(run: Run|undefined, result: RunResult|undefined, options?: SkillStatusOptions): SkillStatus|null;
export function isTicketSkillRun(run: Run): boolean; // taskId set and skill !== "discuss"
export function skillStatusText(status: SkillStatus): string; // "label · detail-or-next"
export function skillStatusTitle(status: SkillStatus): string; // label, detail, next, diagnostics joined by newlines
```

A line-by-line port of `primaryStatus`, `verdict`, `stopped`,
`processStatus`, `withDiagnostics`, `recordedReason`, `workflowSkill`,
`skillId` and `expectedStage` from `desktop/src/skill-result.mjs`, with these
adaptations, each covered by a test:

- **Free runs**: `null` when `!run.taskId || run.skill==="discuss"`, unless a
  stop is pending, then `Stopping` (as Electron's `kind==='console'` branch).
  The Tauri `Run` has no `kind`; a console without a ticket has no `taskId`.
- **Wait mark**: `hasWaitMark(value)` = non-empty and not starting with
  `0001`. Row 6 fires when `run.status==="running"` and
  (`hasWaitMark(successor ? successor.waitingSince : run.waitingSince)` or
  `options.asking`).
- **Silence**: the Tauri `Run` carries no summary; "No output for a while"
  reads only the record's summary (`isSilentSummary(record?.summary)`).
- **Skill name**: `skillLabels[skillId(skill)] || skillId(skill)` (A2).
- **Stage ordering**: the Tauri `stages` array (same order as Electron's
  `STAGES`).
- **Ported helpers**, private to the module: `RUN_SILENCE_PREFIX = "No MCP call for "`,
  `isSilentSummary`, `runStateOf` and `runStateLabel` with the labels of
  `shared/runStates.ts` (`waiting` "Waiting for you", `silent` "Silent",
  `running` "Running", `queued` "Queued", `completed` "Finished", `failed`
  "Failed", `canceled` "Stopped"). `runStateOf` maps the Tauri statuses
  `pending`/`preparing` to `queued` and applies `hasWaitMark`.
- Constants copied verbatim: `LAUNCH_PLACEHOLDER`, the summary separator
  (a space, U+2014 em dash, a space, built with `String.fromCharCode(0x2014)`
  as Electron does), `REASON_LIMIT = 120`, the outcome and expected-stage tables, the
  pickup set `{pickup, pickup_issues}`.

A comment at the top names `desktop/src/skill-result.mjs` and
`specs/878-clear-skill-status/spec.md` as the source, so both clients are
changed together.

### 3. Refresh rule (`src/domain/skill-result-refresh.ts`, new, pure)

Port of `desktop/src/skill-result-refresh.mjs` verbatim (`SETTLE_MS`,
`LIVE_REFRESH_MS`, `ENDED_REFRESH_MS`, `skillResultDue`, `skillResultStamp`,
`skillEnded`), typed. `LIVE` also holds the Tauri `pending` and `waiting`
statuses. The wait mark compared is normalised with `hasWaitMark` so a zero
date never reads as a change.

### 4. Pickup start stage (`src/domain/skill-status.ts`)

```ts
export function rememberLaunchStage(run: Run, result: RunResult, liveRunIds: string[]): void;
export function launchStageOf(runId: string): string;
```

Same rule as Electron's `main.js`: recorded once, for a `pickup` or
`pickup_issues` run, the first time a result is read while the run is not
ended and its own skill has not ended; stored in `localStorage` under
`sectile:launchStages` as a `{runId: stage}` object; entries whose run is no
longer in the monitor's run list are pruned at each write. Storage errors are
ignored (restricted environments), as `readTaskLaunchView` does.

### 5. Shared reader (`src/lib/skill-results.ts`, new)

A module-level store keyed by run id, consumed through a hook:

```ts
export function useSkillResult(run: Run|undefined, turnBusy?: boolean): RunResult|undefined;
```

- Each entry holds the last result, the last stamp (`skillResultStamp`), an
  in-flight flag and a subscriber count. `useSyncExternalStore` exposes the
  result; the entry is dropped when its last subscriber leaves.
- The hook is called with the run object from the monitor snapshot. On every
  snapshot change (the monitor's 3 s poll) it asks `skillResultDue(run, stamp,
  Date.now())`, and reads with `runResult(run.id)` when due and no read is in
  flight (FR4). It also reads when `turnBusy` goes from `true` to `false`.
- A successful read stores the result, stamps it with
  `skillEnded(result)`, and calls `rememberLaunchStage`. A failed read keeps
  the previous result and leaves the stamp unchanged except `at`, so the next
  due read retries without hammering.
- Only ticket skill runs are read (`isTicketSkillRun`); for others the hook
  returns `undefined` and never calls the agent (US6.1).

Three surfaces subscribe to the same entry: the strip and the panel of an
open conversation, and the sidebar row. One entry, so at most one read per
run per due moment.

### 6. Hook combining everything (`src/lib/skill-results.ts`)

```ts
export function useSkillStatus(run: Run|undefined, opts: {task?: Task; project?: ProjectContext; asking?: boolean; turnBusy?: boolean}): SkillStatus|null;
```

Calls `useSkillResult`, computes `nextStep` as
`step.skillId ? step.label : ""` from `nextSkill(task, project)` when both are
known (the sidebar passes none: it shows no next step), `launchStageOf(run.id)`,
and returns `skillStatus(...)`, memoised on its inputs.

### 7. Conversation status strip (`src/components/ProjectPrompt.tsx`)

`ConversationStatus` receives `projectContext` and calls
`useSkillStatus(run, {task, project: projectContext, asking: !!snapshot.approvals?.length, turnBusy: busy})`.

- `status` non-null and agent available: the `<summary>` reads
  `skillStatusText(status)`, carries `title={skillStatusTitle(status)}`, and
  starts with `<span className={"skill-status-mark " + status.kind}/>`. The
  `<details>` gets the class `skill-status-<kind>`.
- The panel shows, before its existing buttons, a
  `<div className="conversation-execution-details">` with
  `Execution: {execution}` (today's computed text), the full
  `status.reason` when non-empty, and `status.diagnostics`.
- `status` null: today's rendering, unchanged. Agent unavailable: "Status
  unavailable", unchanged.

### 8. Ticket workflow panel (`src/components/WorkflowTask.tsx`)

- Remove the local `result` state and the result read from the 4 s effect;
  keep that effect's task refresh (`refresh()`), now independent of the
  result.
- `const status = useSkillStatus(run, {task, project, turnBusy: undefined})`;
  the `workflow-run-status` line renders `skillStatusText(status)` with
  `title={skillStatusTitle(status)}` and the same kind mark; when `status` is
  null (a discuss run), it renders nothing.
- The pending-question flag is not known here without subscribing to the
  conversation; pass `asking` from `useConversation(run.id)` only when
  `run.conversation` is true, so the panel and the strip agree (US1.1). The
  hook must be called unconditionally: subscribe with the run id and ignore
  the value for non-conversation runs.

### 9. Sidebar conversation row (`src/App.tsx`, `ConversationNavigation`)

- `const status = useSkillStatus(run, {task, asking: needsInputFromSnapshot, turnBusy: loaded && snapshot.busy})`.
- Indicator class: `status ? "skill-" + status.kind : <today's class>`.
- Tooltip: today's prefix, then `status ? status.label : activity`. The
  button gets `aria-label` equal to the same text (today it has none and its
  name is the inner text); keep the task key's own `aria-label`.

### 10. Styles (`src/App.css`)

Next to the existing `.conversation-indicator` rules (line ~1498), reusing
the existing palette:

| Class | Rule |
|-------|------|
| `.skill-working` | as `.live`: `#60a5fa`, `run-state-pulse` |
| `.skill-waiting` | as `.needs-input`: `#dca04f` |
| `.skill-done` | `#4ade80` (green) background and border |
| `.skill-verify` | amber `#f5c542`, `border-radius:1px; transform:rotate(45deg)` (diamond) |
| `.skill-failed` | red `#f87171`, `border-radius:1px` (square) |
| `.skill-stopped` | `var(--muted)` background and border |
| `.skill-stopping` | `var(--muted)`, `run-state-pulse` |

The same classes apply to `.skill-status-mark` in the strip and the panel
(an 8 px inline mark). Add `.skill-working`/`.skill-stopping` to the
`prefers-reduced-motion` rule. Check contrast against both themes' sidebar
background; adjust the hex values if one fails 3:1 for a graphical object.

### 11. Docs

- `desktop-tauri/README.md`: a "Skill status" section after "Read
  conversation output": the three surfaces, the kinds, the precedence in one
  paragraph with a link to `../specs/878-clear-skill-status/spec.md`, the
  diagnostics, the sidebar colours and shapes, and that reasons need a local
  agent recent enough to send them.
- `CHANGELOG.md` `[Unreleased]` / `Added`: "Sectile Desktop shows how a
  ticket skill ended (done with its outcome and next step, needs your answer,
  needs verification, could not finish with the reason, stopped) in the
  conversation, the ticket workflow panel and the sidebar (#908)."
- `tests/fixtures/ux-baseline.json` `runResult`: add `summary`, `error`,
  `disconnected` so the UX fixture exercises the new fields.

## Rejected alternatives

- **Import `desktop/src/skill-result.mjs` or `shared/runStates.ts`**: Tauri
  bundles nothing from those trees today, and the Electron module depends on
  `desktop/src/workflow.mjs`. A port keeps the build graph unchanged; the
  duplication is bounded and pinned by the same test cases.
- **Keep `skillResultLabel` beside the new status**: two vocabularies on the
  same screen is the defect #878 removed (owner choice, round 2).
- **One poller per surface** (today's 4 s poll in `WorkflowTask` plus new ones
  in the strip and every sidebar row): multiplies reads of the server (two
  server reads per result) by the number of visible rows.
- **Green check icon in the sidebar**: dropped by the owner in round 4 for a
  green dot in the existing slot.

## Target files

| File | Change |
|------|--------|
| `desktop-tauri/src/domain/workflow.ts` | widen `RunResult`, remove `skillResultLabel` |
| `desktop-tauri/src/domain/skill-status.ts` | new: reconciliation, launch stages |
| `desktop-tauri/src/domain/skill-result-refresh.ts` | new: refresh rule |
| `desktop-tauri/src/lib/skill-results.ts` | new: shared reader, `useSkillResult`, `useSkillStatus` |
| `desktop-tauri/src/components/ProjectPrompt.tsx` | strip summary, panel details, pass `projectContext` |
| `desktop-tauri/src/components/WorkflowTask.tsx` | status line, drop own result poll |
| `desktop-tauri/src/App.tsx` | sidebar indicator, tooltip, accessible name |
| `desktop-tauri/src/App.css` | kind classes |
| `desktop-tauri/tests/skill-status.test.ts` | new |
| `desktop-tauri/tests/skill-result-refresh.test.ts` | new |
| `desktop-tauri/tests/workflow.test.ts` | drop `skillResultLabel` cases |
| `desktop-tauri/tests/skill-status.ui.test.tsx` | new: strip, panel, sidebar |
| `desktop-tauri/tests/workflow.ui.test.tsx`, `workspace.ui.test.tsx` | adapt mocks/expectations |
| `desktop-tauri/tests/fixtures/ux-baseline.json` | richer `runResult` |
| `desktop-tauri/README.md`, `CHANGELOG.md` | docs |

## Test plan

- **Unit, `skill-status.test.ts`**: one test per precedence row 1 to 9; US2.2
  (open conversation after completion is Done); US2.3 (zero-date wait mark
  ignored, pending question counts, wait on an ended run ignored, verdict
  outranks wait); US2.4 (status `waiting`, `waitingReason: repository`);
  US2.5 (foreign record ignored by id, task and skill); US2.6 (no result);
  US3.2 every outcome and "reported by the skill"; US3.3 pickup forward, not
  forward, unknown start stage; US3.4 next step present, absent when
  finished; US4 reason first line, error before note, placeholder and silence
  removed, 120-character cut, "reason not recorded", "reason not available"
  (record without `summary`/`error` keys); US5 by you, disconnected,
  recorded, not recorded; US6 free run null, discuss null, stopping free run;
  US7 successor working, successor waiting, successor completed, diagnostics
  wording; console-style skill ids (`sectile:specify-issue`,
  `pickup-issues`). Launch stage memory: recorded once, not for an ended run,
  pruned, survives a reload of the module from `localStorage`.
- **Unit, `skill-result-refresh.test.ts`**: first read, status change, wait
  change (zero date is no change), settling window, ended skill with live
  run, live and ended intervals, stamp keeps `changedAt`.
- **UI, `skill-status.ui.test.tsx`**: strip summary reads
  `Done · Clarified · Next: Specify`, panel shows run state, reason and
  diagnostics; free conversation strip still reads `Execution: …`; workflow
  panel line equals the strip text for the same run; sidebar dot class per
  kind (`skill-done` green slot, `skill-failed`), tooltip/accessible name end
  with the label; free conversation row keeps `idle`/`live`; two surfaces on
  the same run cause one `runResult` call (fake timers).
