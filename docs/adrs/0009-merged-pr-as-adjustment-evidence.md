# ADR 0009: A merged pull request is valid adjustment evidence

Status: Accepted

## Context

ADR 0004 requires an existing open task-branch PR for the implemented-to-reviewed
transition. Adjustment deliberately stops just short of merging, because merging is
the human's call. Those two rules race: the human can merge as soon as the PR is
ready, which is before Sectile records the reviewed stage.

When that happens the forge reports no open PR for the branch and the transition is
rejected. The task is then stranded at implemented with no recovery path — the branch
is merged, so no legitimate new open PR can be produced, and the earlier creation
stage has nothing left to create. Observed on task #131, whose PR #140 was merged
before its reviewed transition was recorded.

## Decision

Accept a merged task-branch PR as adjustment evidence, alongside an open one, in the
forge lookup, in the server-side stage validation and in the agent's own pre-check.

An open PR stays authoritative when a branch has both, so a branch reopened for
follow-up work is adjusted against the live PR. A PR closed without merging is
ignored: it is abandoned work, not evidence. Readiness is not evaluated for a merged
PR, since the forge cannot report a merged PR as a draft and the draft gate exists
only to stop adjustment declaring success on a PR no reviewer can see.

Every other gate is unchanged: the PR must be for the task branch, must match the
PR already recorded on the task, its head commit must be the agent checkout commit,
and the checkout must be clean. The adjustment contract now tells the agent that a
merged PR is reviewed in place and never pushed onto.

## Consequences

A human merge no longer strands a task before reviewed. The relaxation is bounded by
the head-commit check, so a merged PR belonging to other work cannot complete a task.
Adjustment on a merged PR can review and report but cannot deliver corrections; those
need a new task and a new branch. Sectile still never merges, approves or closes a
task, and no new workflow state is introduced.
