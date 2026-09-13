---
description: "Rédige le compte-rendu de fin, vérifie la fusion et nettoie l'espace local."
argument-hint: <TICKET-KEY> [contexte]
---
# Handoff and Close

Stage: reviewed -> finished.

## Goal
Leave two things behind: a handover a colleague can act on without asking you,
and a local workspace with nothing stale in it.

## Read first
- The state of the branch against the default branch.
- What the implementation and review steps reported, so the handover matches reality.

## Steps
1. Confirm the ticket's branch is actually merged into the default branch. If it is
   not, stop, say so, and clean nothing.
2. Write the handover: what shipped, what changed for the user, what is still open.
3. Write the acceptance checklist as checkboxes, each item something a human can
   verify in the running product.
4. Confirm documentation shipped with the change. If a correction is still needed,
   record it as follow-up work; do not create uncommitted edits just before cleanup.
5. Turn any remaining follow-up into a separate ticket to create, rather than a
   paragraph nobody will read.
6. Clean up locally only after checking for uncommitted or unpushed work and other
   tickets sharing this worktree. Preserve a shared batch worktree until every ticket
   is handed off. Remove only an unused, clean worktree and its confirmed merged branch.

## Do not
- Do not delete anything remote: no remote branch, no tag, no release.
- Do not clean up while the merge is unconfirmed.

## Report
- The handover.
- The acceptance checklist, as checkboxes.
- What was cleaned locally, and what could not be, with the reason.
- Follow-up tickets worth creating.

## Execution and ticket state
- **Managed TaskFlow run**: When the invocation supplies a result-file contract, follow it. TaskFlow validates the result and owns transitions and tracker reports. Do not also call stage/postback APIs or edit tracker labels.
- **Remote execution indicator (standalone only)**: Before doing work, call taskflow_start_run with the full task primary key and skill name. If TASKFLOW_RUN_ID or a launch runId is supplied, reuse it. Keep the returned activity ID as runId. Nested skills reuse the outer run; only the owner finishes it. Call taskflow_finish_run with taskKey, runId, status (completed, failed or canceled), and a note when the entire invocation ends, including errors or stopping for user input. Intermediate stage transitions do not finish an enclosing pickup run. A batch tracks each task separately. Never start a run merely to read a task.
- **Standalone invocation**: Read live context with `taskflow_get_task` and `taskflow_get_project_context`. After verifying each completed step, invoke `taskflow_transition_stage` with the task key, completed stage, structured report note and actual branch. Check the tool result for errors before continuing.
Transition reviewed → finished only when this step is complete.
Use `taskflow_add_comment` for an authorized ticket discussion update. Managed runs must not also invoke transition/comment tools for reports owned by TaskFlow. If MCP is unavailable, preserve work and report the pending transition; do not silently write to a different server or database.
Reuse the assigned worktree and actual branch. Never merge or delete remote objects. Keep work available for review and retry until confirmed handoff.

## Ticket
$ARGUMENTS
