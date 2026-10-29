# Design: backlog-level orchestrator

## Today

```
Run agent ──► POST /orchestrate ──► Go loop over tickets (board order)
                                      ├─ length / label filters
                                      └─ per ticket: chain(clarify→specify→code)
                                            each step prompt = "Backlog orchestration:
                                            is THIS ticket eligible? <user policy>"
```

## Target

```
Start ──► arm(project, user, device, profile) ──► queue orchestrator session
                                                        │
        ┌───────────────────────────────────────────────┘
        ▼
  orchestrate_backlog session (task_id NULL, autonomous, policy = profile .md)
    get_orchestration_state ─► cap, slots, busy tickets, previous decisions
    list_tasks / get_task / get_project_context
    launch_task_chain(task, rationale) ─► server guards ─► chain → implemented
    defer_task(task, reason)
    finish_run(summary)

  orchestrated chain ends ──► armed? ──► no session active ──► start a session
                                      └─► session active ────► wake_pending = 1
  session ends ──► wake_pending? ──► start the next session
```

## Session record

Stored as a project agent run, like a macro run (`internal/db/macroruns.go`):
`project_id` set, `task_id` NULL, `skill_id = 'remote_run'`,
`skill_name = 'orchestrate_backlog'`, `run_mode = autonomous`, and a new column
`run_scope = 'orchestrator'`. A partial unique index on `project_id` where
`run_scope = 'orchestrator'` and the status is queued, pending or running allows
one per project across server instances. The agent records the profile ID on
its run, so the desktop overview matches the session to its profile.

The session does not match the orchestrated-run predicate of ADR 0063: its
prompt does not open with `Backlog orchestration:`, so it takes no slot.

## Dispatch

The dispatch is an `execute_skill` operation with no `TaskID` and no
`MacroKey`, `Scope: "orchestrator"`, and the armed `ProfileID` and
`ProfileName`. The server never holds the policy file: the agent reads the
profile's file at each launch, as `readOrchestrationAgentGuidelines` does, and
appends it to the prompt. A profile removed from the workstation, or whose file
has become invalid, fails the session with the validation message.

The workstation is resolved once, at start, as a queued launch resolves it.
When a session is woken while that workstation is offline, it stays `queued`
with `dispatch_state = 'waiting'`, which no restart reclaims, and the
connection of that workstation sends it. Stopping cancels it. A start that no
workstation can run is refused with 409.

A dispatched session that the reconnected workstation no longer holds is closed
as a session by the reconnection reconciliation, so it never blocks the next
one.

## MCP tools

| Tool | Effect | Refusals |
| --- | --- | --- |
| `get_orchestration_state` | cap, orchestrated runs in use, free slots, blocker labels, armed state, active session, latest decision per ticket with whether the ticket changed since | unknown project |
| `launch_task_chain(task, rationale)` | `admitOrchestratedActivity` then the first step from the ticket's stage, `ChainStop = implemented`, prompt = prefix + rationale; records a `launched` decision | not armed or not the armed session, capacity, ticket busy, blocker label, stage at or past `implemented`, ticket of another project |
| `defer_task(task, reason)` | records a `deferred` decision | not the armed session |

Both write tools accept calls only from the running orchestrator session of the
project, identified by the run ID its console sends on every MCP request
(`X-Sectile-Run-Id`), so another agent cannot launch chains in the
orchestrator's name.

## Decisions table

`orchestrator_decisions(id, project_id, task_id, run_id, decision, reason,
task_updated_at, created_at)`. Kept for the last 30 days. `get_orchestration_state`
returns the latest decision per ticket; a deferred ticket whose current
`updated_at` is unchanged is flagged `unchanged_since_decision`, which the skill
uses to skip it.

## Armed state

Table `orchestrator_arming(project_id PRIMARY KEY, user_id, device_id,
profile_id, profile_name, wake_pending, armed_at)`. A row means armed. It is a
table rather than columns on `projects` so that arming never goes through the
project scan and update, which carry every project setting. Arming inserts with
`ON CONFLICT DO NOTHING` and reads the row back, so the first start wins across
instances and a second profile is refused with the armed one's name.

Wake-up hook: at the end of `handBackRun`, once the hand-back is done, when the
ticket holds no active run any more, which covers completion, failure and
cancellation. When no session is active, a new one starts; the partial unique
index arbitrates between instances. When one is active, `wake_pending` is set,
because it may have read the state before the slot freed; the end of that
session clears the flag and starts the next one. There is no time window: a
burst of chain ends yields at most one session after the running one.

## Ticket chain prompt

```
Backlog orchestration: launched by orchestration agent "<profile name>" (<profile id>).
Why this ticket now: <rationale>
The chain stops at implemented; review and merge require a human.
```

`orchestratorLaunchPrompt` and the eligibility text are removed.
`enqueueChainStep` keeps repeating the prompt, so each step keeps the context.

## Removed

- `EvaluateAndOrchestrateBacklog`'s loop and `EvaluateTaskEligibility`'s length
  checks. The label check moves into `launch_task_chain`.
- `OrchestrationResult` as a synchronous answer. The desktop reads the session
  and its decisions instead.

## Risks

- **Token spend.** Bounded by one session per freed slot, plus manual starts.
  The pending flag absorbs bursts of chain ends into one following session.
- **Agent misbehaviour.** It cannot exceed the cap, take a busy or blocked
  ticket, or go past `implemented`: those are tool refusals. The worst case is a
  poor choice among eligible tickets, which its recorded rationale makes visible.
- **Cross-change dependency.** The desktop membership predicate of #871 needs
  the session's profile ID and skill to recognise it.
