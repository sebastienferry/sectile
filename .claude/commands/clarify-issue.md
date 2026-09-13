---
description: "Résout les ambiguïtés réversibles et identifie les décisions indispensables."
argument-hint: <TICKET-KEY> [contexte]
---
# Clarify Issue

Stage: new -> clarified.

## Goal
Turn a vague ticket into a decided one. You are looking for the decisions that
would be expensive to reverse later, not for a list of everything unknown.

## Read first
- The ticket: title, description, comments, parent epic if there is one.
- The code the change would touch. Name the files you actually read.
- Neighbouring features that already solve a similar problem in this codebase.

## Steps
1. Restate the request in two sentences, including what you believe is out of scope.
2. List the ambiguities you found, worst first. An ambiguity is worth listing only
   if two readings lead to different code.
3. Name the critical dependencies: other services, other teams, migrations, data
   you do not have.
4. Resolve reversible choices using existing code and project conventions. Record
   the choice and rationale; do not ask questions merely to fill a quota.
   Ask only when an essential product decision changes acceptance criteria or an
   unavailable dependency prevents progress. In an unattended run, report the
   concrete blocker and the decision needed; do not invent settled requirements.
   A TTY alone does not make a run interactive: follow the invocation's mode.
5. Persist the settled scope and assumptions in the report for specification.

## Do not
- Do not write production code at this stage, and do not start the specification.
- Do not invent an answer to your own question and move on without stating your assumption.
- Do not pad the list to reach five questions.

## Report
- Restated request and scope.
- Ambiguities, worst first.
- Critical dependencies.
- Numbered questions with your recommended option.
- Settled scope and assumptions.

## Execution and ticket state
- **Managed TaskFlow run**: When the invocation supplies a result-file contract, follow it. TaskFlow validates the result and owns transitions and tracker reports. Do not also call stage/postback APIs or edit tracker labels.
- **Remote execution indicator (standalone only)**: Before doing work, call taskflow_start_run with the full task primary key and skill name. If TASKFLOW_RUN_ID or a launch runId is supplied, reuse it. Keep the returned activity ID as runId. Nested skills reuse the outer run; only the owner finishes it. Call taskflow_finish_run with taskKey, runId, status (completed, failed or canceled), and a note when the entire invocation ends, including errors or stopping for user input. Intermediate stage transitions do not finish an enclosing pickup run. A batch tracks each task separately. Never start a run merely to read a task.
- **Standalone invocation**: Read live context with `taskflow_get_task` and `taskflow_get_project_context`. After verifying each completed step, invoke `taskflow_transition_stage` with the task key, completed stage, structured report note and actual branch. Check the tool result for errors before continuing.
Transition new → clarified only when this step is complete.
Use `taskflow_add_comment` for an authorized ticket discussion update. Managed runs must not also invoke transition/comment tools for reports owned by TaskFlow. If MCP is unavailable, preserve work and report the pending transition; do not silently write to a different server or database.
Reuse the assigned worktree and actual branch. Never merge or delete remote objects. Keep work available for review and retry until confirmed handoff.

## Ticket
$ARGUMENTS
