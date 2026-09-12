---
description: "Implement the ticket from its specification and prove it works with the project's own build, linters and tests."
argument-hint: <TICKET-KEY> [contexte]
---
# Implement Code

Stage: specified -> implemented.

## Goal
Ship the change described by the specification, in code that reads like the code
already there, with the project's checks green.

## Read first
- The specification and its task checklist. It is the contract, follow its order.
- The surrounding code: naming, error handling, comment density, test style. Match it.
- How this project builds and tests. Find the real commands, do not assume them.

## Steps
1. Reuse the assigned worktree and branch, including a shared batch branch. Never implement on the default branch.
2. Work through the checklist in small steps, each one leaving the tree buildable.
3. Add the tests that cover the new behaviour and its edge cases, not just the
   happy path. A change with no test needs a stated reason.
4. Run build, static analysis and tests. Fix until green, and quote the real output.
5. Re-read your own diff before finishing, as a reviewer would.

## Recovery and blockers
- Repair routine technical issues and update design/tasks when the implementation
  needs to change while preserving acceptance criteria. Continue after documenting why.
- Establish whether a failing test predates the change. Fix failures in scope; report
  unrelated failures with baseline evidence. Never hide them or mark checks green.
- Stop only for an essential product decision, an unavailable dependency after
  bounded recovery attempts, or work that materially expands the requested scope.
- Preserve the work branch, completed checklist items and remaining next action so
  a retry can resume instead of starting over.

## Report
- What changed, file by file, and why.
- The real output of build, linters and tests, remaining failures included.
- What you deliberately left out, and what it would take to finish it.

## Execution and ticket state
- **Managed TaskFlow run**: When the invocation supplies a result-file contract, follow it. TaskFlow validates the result and owns transitions and tracker reports. Do not also call stage/postback APIs or edit tracker labels.
- **Standalone invocation**: After verifying each completed step, use the local handler below. Check its exit status and response. If it is unavailable, preserve work and report the pending transition; do not silently diverge local and tracker state.
Transition specified → implemented only when this step is complete.
```bash
curl --fail-with-body --silent --show-error -X POST http://localhost:8090/api/tasks/stage \
  -H 'Content-Type: application/json' \
  -d '{"taskKey":"<KEY>","stage":"implemented","note":"<REPORT_NOTE>","branch":"<ACTUAL_BRANCH>"}'
```
This POST calls the local TaskFlow handler directly. Confirm HTTP success before continuing. If it is unavailable, preserve work and report the pending transition; do not silently diverge local and tracker state.
Reuse the assigned worktree and actual branch. Never merge or delete remote objects. Keep work available for review and retry until confirmed handoff.

## Ticket
$ARGUMENTS
