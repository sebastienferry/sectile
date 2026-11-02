# ADR 0063: Only orchestrated runs take an orchestrator slot

- Status: Accepted
- Date: 2026-10-29

Amends: [ADR 0060](0060-project-backlog-orchestrator-agent.md), on the
concurrency and capacity guard.

## Context

ADR 0060 caps the backlog orchestrator at `orchestrator_max_concurrent`
parallel runs per project, and counted every active run of the project against
that cap: "Existing manual sessions consume slots too."

On a team project that rule turned the cap into a switch held by whoever was
working. Project SFE, capped at 2, reported 3 active runs: sessions people had
started by hand, from several workstations. Every evaluation stopped before
reading the backlog, and the orchestrator launched nothing while nobody had
asked it to wait. The cap was meant to bound the orchestrator's own spending
and the review load it creates, not the team's.

## Decision

1. **The cap counts the orchestrator's own work only.** A run takes a slot when
   its prompt opens with the orchestrator prefix (`Backlog orchestration:`).
   The prefix is already stored on every record the orchestrator produces: the
   queued launch, the `agent_launch` row that hands it to a workstation, the
   remote run, and each chained step, since an orchestrated chain repeats its
   prompt (`enqueueChainStep`). No column is added.
2. **A manual session still keeps the orchestrator off its ticket.** A task with
   an active run is busy, and the orchestrator does not start a second agent on
   it. Manual work no longer limits how many other tickets it may take.
3. **Everything else in ADR 0060's guard stands**: admission under the project
   row lock, the cap read from that row, and no capacity override.

## Consequences

- An orchestrator may run its full cap while people work on other tickets of
  the same project, so the project can carry more agents than the cap.
- The capacity shown in the desktop and in agent project discovery means
  "orchestrated runs in use", and its tooltip says so.
- The prompt prefix becomes load-bearing for capacity as it already was for
  chaining. A change to that prefix, or a launch path that drops the prompt,
  would let orchestrated runs escape the cap; `TestOnlyOrchestratedRunsTakeASlot`
  covers the queued and workstation records.
