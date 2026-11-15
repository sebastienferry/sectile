# Plan #904 - Desktop (Tauri): tabbed question form with proposals

Spec: `spec.md`. Branch: `feat/904`.

## Stack and surfaces

- Tauri Desktop front end: React + TypeScript (`desktop-tauri/src`), UI tests
  with the existing `desktop-tauri/tests/*.ui.test.tsx` harness.
- Local agent: Go (`internal/agent`), conversation control path.
- Agent ↔ server relay: `internal/agentprotocol` messages over the agent
  WebSocket, handled in `internal/handlers/handlers.go`.
- Activity storage: `models.Activity.Steps []string`, persisted by
  `internal/db` (`db.go` around the activity upsert, l. ~3998).

## 1. Front end: `QuestionForm.tsx`

Target: `desktop-tauri/src/components/QuestionForm.tsx`, styles in
`desktop-tauri/src/App.css` (`.question-card`, `.question-option`).
`ProjectPrompt.tsx` (l. ~381) keeps its wiring: `onAnswer(answers)` →
`control({ action: "answer", ... })`, `onSkip()` → `decision: "deny"`.
No change to the props contract.

### State

Per question index: `{ selected: string[]; custom: string; skipped: boolean }`,
plus `active: number` (selected tab). Derived:

- `state(i)`: `skipped` → *skipped*; non-empty joined answer → *answered*;
  else *to answer*.
- `answers`: for each question, `SKIPPED` (`"Skipped by the owner"`) when
  skipped, else the existing join of `selected` and trimmed `custom` by `, `.
- `valid`: every question answered or skipped, size ≤ 16384 bytes (same
  computation as today, on the final `answers`).

Export `SKIPPED_ANSWER = "Skipped by the owner"` and a pure helper
`recommendedLabel(label): { text: string; recommended: boolean }` matching
`/\s*\(Recommended\)\s*$/` (case-sensitive), so tests can import them.

### Rendering

- `questions.length > 1`: a `role="tablist"` strip of `role="tab"` buttons
  (`aria-selected`, `aria-controls`), each labelled `header.trim() || "Q" +
  (i + 1)` followed by a state marker (`✓` answered, a "skipped" marker) with
  an accessible name such as `"Scope, answered"`. ArrowLeft/ArrowRight/Home/End
  move the selection (roving `tabIndex`). One `role="tabpanel"` renders the
  active question's existing `fieldset`; inactive panels are not rendered, the
  state lives in the parent so values survive tab changes.
- `questions.length === 1`: the `fieldset` alone, no tablist.
- Options: label rendered from `recommendedLabel(option.label).text`; when
  recommended, add a `question-option recommended` class and a `<span
  className="badge">Recommended</span>`. The radio/checkbox value and what is
  stored in `selected` stay the original `option.label`.
- "Skip this question" button inside each panel (`type="button"`): sets
  `skipped: true`, clears `selected` and `custom`, then advances like FR-5.
  Any option change or typing sets `skipped: false`.
- Auto-advance: in the radio `onChange` of a non-multiSelect, non-secret
  question, after updating state, `setActive(Math.min(i + 1, n - 1))`.
- Actions row unchanged (`Skip request`, `Send answers`), rendered below the
  tab panel.

## 2. Agent: answer path and masking

`internal/agent/agent_conversation_approvals.go`

- `checkAnswers` is unchanged: `Skipped by the owner` is a non-empty answer.
- Claude path (`decideApprovalLocked`): build the trace detail through a new
  helper `maskedAnswers(approval, answers)` that hides questions marked
  `isSecret` in `approval.Input` (parsed like `checkAnswers` does), and use it
  for `answersDetail`. Codex's inline masking (`agent_codex_conversation.go`
  l. ~990-1000) moves to the same helper, reading `QuestionSecrets` when set.
- Codex path (`decideCodexApprovalLocked`): no change needed for the skipped
  literal: `answers[question]` is already sent as a one-element list.

## 3. Run activity step

There is no channel today for the agent to append a step to a running
activity: `Activity.Steps` is written when the activity is stored and never by
the agent afterwards. Add one, modelled on `run_answered`.

### Protocol (`internal/agentprotocol/message.go`)

```go
// RunStepType is the message an agent sends to add one line to the activity
// of a run it owns (#904). A server that predates it logs the unknown type.
const RunStepType = "run_step"

type RunStep struct {
    RunID string `json:"runId"`
    Text  string `json:"text"`
}
```

### Agent

- After a successful `answer` decision (both providers), when
  `run.desktop.TaskID != ""` and `run.desktop.ID != ""`, send `RunStep{RunID:
  run.desktop.ID, Text: "Owner answers:\n" + maskedDetail}` through the
  existing link writer (`d.link.write`), outside the queue lock and without
  waiting for a reply. A missing connection or a write error is logged and
  ignored (FR-17). No step on `deny` (FR-16).
- `Text` bounded to 16 KB + header; the answers are already bounded by
  `conversationAnswersLimit`.

### Server (`internal/handlers/handlers.go`, agent read loop l. ~4020)

- `case agentprotocol.RunStepType:` decode, then append `Text` to the
  activity's `Steps` only when the activity exists, belongs to the connected
  agent's device and user (same ownership check as `answerRunWait`), and is
  `queued`/`running`. Persist with the existing activity store method (add a
  narrow `AppendActivityStep(id, text)` in `internal/db` if the upsert would
  race with other writers), then broadcast the existing activity-changed
  event so boards refresh. Anything else is logged and dropped.

### Rejected alternatives

- **Posting a ticket comment**: excluded by the owner.
- **Recording through `report_waiting` or a new MCP tool**: excluded by the
  owner; also the answer originates in the agent, not in the CLI session.
- **Writing the step from the Tauri app through the server API**: the
  Desktop may be in companion mode without a server session for the run, and
  the agent already knows the masked answers and the run; one writer is
  simpler.
- **Representing a skipped question as an empty answer**: `checkAnswers`
  would refuse it, and Claude would not distinguish it from a bug; the
  literal is explicit to the agent.
- **Preselecting the recommended option**: rejected by the owner.

## Data contracts

- Answers payload (unchanged shape): `Record<questionText, string>`; a
  skipped question maps to `"Skipped by the owner"`.
- `run_step` message: `{ "type": "run_step", "payload": { "runId", "text" } }`.

## Target files

- `desktop-tauri/src/components/QuestionForm.tsx`
- `desktop-tauri/src/App.css`
- `desktop-tauri/tests/questions.ui.test.tsx`
- `internal/agent/agent_conversation_approvals.go` (+ `_test.go`)
- `internal/agent/agent_codex_conversation.go` (+ `_test.go`)
- `internal/agentprotocol/message.go`
- `internal/handlers/handlers.go` (+ test of the read-loop case)
- `internal/db/db.go` (only if a narrow append is needed, + test)
- `CHANGELOG.md`

No new `exec.Command` is introduced, so the Windows `agentexec.Hidden` rule is
not engaged.
