---
description: "Relit le diff, prépare le commit et ouvre la merge request."
argument-hint: <TICKET-KEY> [contexte]
---
# Review and Pull Request

Stage: implemented -> reviewed.

## Goal
Hand a reviewer a branch that is already worth reading: the obvious problems
found and fixed, the risky parts pointed out, the test plan written down.

## Read first
- The full diff of the branch against the default branch. All of it, not the summary.
- The specification, to check that what was asked is what was built.
- The current remote default branch: fetch the remote and identify its configured
  default branch before reviewing or publishing.

## Steps
1. Fetch the remote (`git fetch origin`) and compare the work branch with the
   remote default branch (normally `origin/main`; use the repository's configured default when different).
   Integrate missing base commits before the final review: prefer rebase when the branch is private, or merge when
   repository policy or shared-branch state requires it. Resolve conflicts and do not continue until the working tree is clean.
2. Review the resulting diff for correctness, side effects, security, and edge cases with no test.
3. Update documentation affected by the change. Fix what the review finds, now. A known defect belongs in the code, not in the
   description of the merge request.
4. Re-run build, static analysis and tests after integrating the default branch and on the final state.
5. Commit with a conventional message: type, scope, and why the change exists.
6. Push the branch and create or update its existing merge request: summary, test plan, and the specific
   places where you want a reviewer's eyes.
   If rebasing an already-pushed branch, use `git push --force-with-lease`, never an unguarded force push.
7. If the repository has no remote, say so and stop rather than merging locally.

## Do not
- Do not merge, do not approve, do not close the ticket. That is the user's call.
- Do not open a merge request on a red build. Report the failure instead.
- Do not open a merge request from a branch known to be behind the remote default branch.

## Report
- What the review found, and which findings you fixed.
- The merge request URL, or why there is none.
- The test plan a reviewer can replay, as a checklist.

## Execution and ticket state
- **Managed TaskFlow run**: When the invocation supplies a result-file contract, follow it. TaskFlow validates the result and owns transitions and tracker reports. Do not also call stage/postback APIs or edit tracker labels.
- **Remote execution indicator (standalone only)**: Before doing work, call taskflow_start_run with the full task primary key and skill name. If TASKFLOW_RUN_ID or a launch runId is supplied, reuse it. Keep the returned activity ID as runId. Nested skills reuse the outer run; only the owner finishes it. Call taskflow_finish_run with taskKey, runId, status (completed, failed or canceled), and a note when the entire invocation ends, including errors or stopping for user input. Intermediate stage transitions do not finish an enclosing pickup run. A batch tracks each task separately. Never start a run merely to read a task.
- **Standalone invocation**: Read live context with `taskflow_get_task` and `taskflow_get_project_context`. After verifying each completed step, invoke `taskflow_transition_stage` with the task key, completed stage, structured report note and actual branch. Check the tool result for errors before continuing.
Transition implemented → reviewed only when this step is complete.
Include prUrl with the verified pull request URL.
Use `taskflow_add_comment` for an authorized ticket discussion update. Managed runs must not also invoke transition/comment tools for reports owned by TaskFlow. If MCP is unavailable, preserve work and report the pending transition; do not silently write to a different server or database.
Reuse the assigned worktree and actual branch. Never merge or delete remote objects. Keep work available for review and retry until confirmed handoff.

## Project pull request policy
PR creation stage: specified. Read this setting from taskflow_get_project_context before executing. After the specification is written and validated, commit and push the specification on the task branch and open a draft PR/MR for specification review. Reuse an existing PR/MR for that branch. Include its URL as prUrl in the specified transition. Keep it draft while implementing; update the same PR/MR and mark it ready only after implementation and review. Do not mark the task reviewed merely because a draft exists.

## Ticket
$ARGUMENTS
