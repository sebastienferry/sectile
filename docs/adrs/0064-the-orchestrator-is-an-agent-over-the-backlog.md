# ADR 0064: The orchestrator is an agent over the backlog

- Status: Proposed
- Date: 2026-10-29

Supersedes: [ADR 0060](0060-project-backlog-orchestrator-agent.md) on
invocation, backlog triage and eligibility. Keeps ADR 0060's concurrency and
capacity guard as amended by
[ADR 0063](0063-only-orchestrated-runs-take-an-orchestrator-slot.md).

## Context

ADR 0060 called its feature an orchestrator agent, but no agent ever looks at
the backlog. `EvaluateAndOrchestrateBacklog` walks the tickets in board order in
Go, drops those with a short title or description or a blocker label, and
launches a full chain on each remaining ticket until the cap is reached. The
natural-language policy a user writes in an orchestration agent's Markdown file
is not read by anything that sees the backlog: it is pasted into the prompt of
every ticket run, behind the `Backlog orchestration:` prefix, and each chain
step is asked to decide again whether its own ticket was eligible.

This has three consequences users notice:

- **No backlog-level judgement.** Nothing ranks tickets against each other,
  notices that one depends on another, or explains why it picked one first.
  Board order is the only order.
- **Refusal is expensive.** Declining a ticket costs a full agent session on
  that ticket, repeated at every step of its chain.
- **Nothing iterates.** An invocation fills the free slots and returns. When a
  chain ends, nothing takes the next ticket; someone must press Run agent again.

## Decision

1. **The orchestrator is one agent session per project.** It is a
   project-scoped run, stored like a macro run (`project_id` set, `task_id`
   NULL), with its own skill `orchestrate_backlog`, run autonomously on the
   workstation that started orchestration. At most one is queued or running per
   project, enforced by a partial unique index as for macros. The session takes
   no orchestrator slot: slots bound ticket work.

2. **Its policy is the orchestration agent's Markdown file.** The selected
   profile's file, read and validated as today (ADR 0060, layout 6), becomes the
   session's instructions. It is no longer copied into ticket runs.

3. **It decides with the whole backlog in view and acts through two tools.**
   It reads the backlog with the existing MCP tools (`list_tasks`, `get_task`,
   `get_project_context`) and the orchestration state with a new
   `get_orchestration_state` (cap, slots in use, busy tickets, previous
   decisions). It acts only through:
   - `launch_task_chain(task, rationale)`, which admits the launch under the
     project row lock exactly as today and starts a chain that stops at
     `implemented`;
   - `defer_task(task, reason)`, which records why a ticket is not taken now.

   It does not edit, comment on, create or move tickets.

4. **The server keeps the hard guards, outside the model's reach.** Admission
   under the locked project row, the cap read from that row, one agent per
   ticket, the blocker labels (`blocked`, `hold`, `on-hold`, `needs-info`,
   `manual`) and the stop at `implemented` are enforced by `launch_task_chain`.
   The tool refuses rather than the agent being asked to comply. The title and
   description length filters are dropped: whether a ticket has enough context
   is the agent's judgement.

5. **Ticket runs no longer re-evaluate their eligibility.** The per-step
   preflight gate is removed. A launched chain carries the
   `Backlog orchestration:` prefix, which ADR 0063 relies on to count slots and
   `enqueueChainStep` relies on to repeat the prompt, followed by the
   orchestrator's rationale for that ticket, as context and not as a gate. A
   step that meets a genuine ambiguity stops as any autonomous step does today.

6. **Orchestration is armed, and a freed slot wakes it.** Starting
   orchestration arms the project for one user, one workstation and one profile
   and runs a first session. While armed, the server queues a new session when
   an orchestrated chain ends, whatever its outcome, and no session is already
   queued or running. Triggers inside a short coalescing window (2 minutes)
   produce one session. A session that launches nothing frees no slot and
   therefore wakes nothing: an exhausted backlog stays quiet until a slot frees,
   the user runs it again, or orchestration is stopped. There is no periodic
   timer. Stopping, disabling orchestration or removing the profile disarms it.
   A wake-up whose workstation is offline waits for it, and says so.

7. **Decisions are remembered.** Each `launch_task_chain` and `defer_task` is
   stored with the session that made it, its reason and the ticket's
   `updated_at`. `get_orchestration_state` returns them, so a later session
   re-examines only tickets that changed since they were deferred, and the
   desktop overview shows why each ticket was taken or left.

8. **The entry points keep their names and change meaning.**
   `POST /api/projects/{id}/orchestrate` and the MCP tool `orchestrate_project`
   arm the project and start a session, answering 202 with its run ID instead of
   a synchronous `OrchestrationResult`. `POST /api/projects/{id}/orchestrate/stop`
   disarms it. Enablement, unknown project and unknown field errors stay as
   ADR 0060 defined them; there is still no capacity or eligibility override.

## Consequences

- A user's policy is applied once, by an agent that sees every candidate,
  instead of once per ticket per step by agents that see one.
- Declining a ticket costs a line in the orchestrator's session, not a ticket
  session.
- The backlog drains on its own while armed, bounded by the cap, and stops
  costing tokens when nothing is left to take.
- The orchestrator session is the "genuine dedicated orchestration session" the
  desktop membership rule of #871 waits for: it is what the Orchestration
  section shows, with its console, and the ticket workers it launches stay
  ordinary workers.
- One profile is armed per project at a time. Several profiles stay
  configurable; starting one while another is armed is refused with the name of
  the armed one.
- An armed orchestration depends on its workstation being online. Failing over
  to another workstation is out of scope.
- The prompt prefix remains load-bearing for capacity and chaining. The
  eligibility text that used to follow it disappears, and so does the
  repetition of the user's policy in every step.
