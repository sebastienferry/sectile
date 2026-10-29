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

  orchestrated chain ends ──► armed? no session queued/running? outside window?
                              └─► queue a new orchestrator session
```

## Session record

Stored as a project activity, like a macro run (`internal/db/macroruns.go`):
`project_id` set, `task_id` NULL, `skill_id = 'orchestrate_backlog'`,
`run_mode = autonomous`. A partial unique index on `project_id` where the skill
is `orchestrate_backlog` and the status is queued, pending or running allows one
per project across server instances. The activity carries the profile ID so the
desktop overview matches it to its profile.

The session does not match the orchestrated-run predicate of ADR 0063: its
prompt does not open with `Backlog orchestration:`, so it takes no slot.

## Dispatch

The dispatch reuses the macro path (`agentDispatcher.dispatchAndWaitRoute`) with
no `TaskID` and no `MacroKey`, a new `Scope: "project"`, and the guidelines
snapshot read by the workstation from the profile's file, as
`readOrchestrationAgentGuidelines` does today. On a wake-up, the server does not
hold the file: it asks the armed workstation to launch, and the agent reads the
profile's file again at that moment. A profile whose file has become invalid
fails the session with the existing validation message and disarms.

## MCP tools

| Tool | Effect | Refusals |
| --- | --- | --- |
| `get_orchestration_state` | cap, orchestrated runs in use, free slots, busy tickets, decisions since arming with the ticket's `updated_at` then and now | project disabled |
| `launch_task_chain(task, rationale)` | `admitOrchestratedActivity` then the first step from the ticket's stage, `ChainStop = implemented`, prompt = prefix + rationale; records a `launched` decision | not armed or not the armed session, capacity, ticket busy, blocker label, stage at or past `implemented`, ticket of another project |
| `defer_task(task, reason)` | records a `deferred` decision | not the armed session |

Both write tools accept calls only from the running orchestrator session of the
project, identified by its run ID in the MCP session, so another agent cannot
launch chains in the orchestrator's name.

## Decisions table

`orchestrator_decisions(id, project_id, task_id, run_id, decision, reason,
task_updated_at, created_at)`. Kept for the last 30 days. `get_orchestration_state`
returns the latest decision per ticket; a deferred ticket whose current
`updated_at` is unchanged is flagged `unchanged_since_decision`, which the skill
uses to skip it.

## Armed state

Table `orchestrator_arming(project_id PRIMARY KEY, user_id, device_id,
profile_id, profile_name, armed_at, last_wake_at)`. A row means armed. It is a
table rather than columns on `projects` so that arming never goes through the
project scan and update, which carry every project setting.

Wake-up hook: at the end of `handBackRun`, and in the failure and cancel paths
of an orchestrated run, when the chain will not continue. Condition, checked in
one transaction under the project row lock: armed, enabled, no orchestrator
session queued, pending or running, and `last_wake_at` older than the
coalescing window. Otherwise the trigger is dropped: the running session reads
the fresh state anyway, and a later chain end retries.

An offline armed workstation leaves the session queued with the existing "waiting
for the workstation" dispatch state.

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

- **Token spend.** Bounded by one session per freed slot, plus manual runs. The
  coalescing window absorbs bursts of chain ends.
- **Agent misbehaviour.** It cannot exceed the cap, take a busy or blocked
  ticket, or go past `implemented`: those are tool refusals. The worst case is a
  poor choice among eligible tickets, which its recorded rationale makes visible.
- **Cross-change dependency.** The desktop membership predicate of #871 needs
  the session's profile ID and skill to recognise it.
