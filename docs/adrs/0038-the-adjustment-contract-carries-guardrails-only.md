# ADR 0038: The adjustment contract carries guardrails only

Status: Accepted

Amends: [ADR 0004](0004-adjustment-and-earlier-pr-ownership.md), its sentence
"Keep the mandatory adjustment contract with customized instructions", for the
part of that contract that prescribes the work.

## Context

Every adjustment run, native or managed, appends a fixed contract to the prompt,
whatever skill carries the stage. That contract did two things. It set
guardrails: verify the existing PR, never create or replace one, never push onto
a merged PR, never merge, approve, close the task or remove its worktree. It also
prescribed the work: review the whole branch against the specification,
reconcile the default branch, run build, lint and tests, commit, push and update
the PR.

The second half overrode the skill. A project whose adjustment skill only reviews
and reports, without correcting anything, still had its agent told to fix, check
and push, so the skill could not express that choice.

## Decision

The contract keeps the guardrails and the feedback rule: it verifies the PR,
forbids creating or replacing one and pushing onto a merged one, leaves a
missing PR to the earlier creation stage, requires retrieving the available
feedback and recording a disposition for each, treats a retrieval failure as
blocking, requires preserving work on failure, and forbids merging, approving,
closing the task and removing its worktree.

Everything else is the skill's call: whether and how it reviews, reconciles the
default branch, corrects findings, runs checks and pushes. The bundled
`adjust-issue` skill keeps doing all of it, so the default behaviour does not
change. The built-in fallback prompt, used when no skill is installed and no
prompt is configured, is unchanged too.

## Consequences

A custom adjustment skill that forbids corrections is now obeyed: the run
verifies the PR, collects feedback and reports, nothing more.

The server-side gates are untouched and remain the real guarantee: the forge must
confirm the matching PR, a still-open PR must no longer be a draft, its head must
be the agent checkout commit and the checkout must be clean. A skill that changes
nothing therefore still has to leave the PR ready for the stage to be accepted.

A custom skill that neither corrects nor checks is a deliberate choice of the
project; Sectile no longer compensates for it in the prompt.
