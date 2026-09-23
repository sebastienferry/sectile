---
description: "Review the branch like a peer would, fix what the review finds, then update the existing merge request and leave the merge to the user."
argument-hint: <TICKET-KEY> [contexte]
---
# Adjust Existing Pull Request

Stage: implemented -> reviewed.

## Session title
- As soon as the ticket is identified, and before doing the work, rename the current session to `<ticket ID> - <ticket title>`, for example `#47 - Remove the parallelism setting`. Keep that title for the whole run.
- This applies to every agent, not only Claude Code: use whatever session renaming capability the running agent exposes, be it a session title tool, a rename command or the host session API. Discover it from the session context instead of assuming a name.
- If no renaming capability is available, skip the rename silently and continue. It never blocks, delays or replaces the work of the skill.

## Sectile reporting
- Task access, the run indicator, stage transitions, pull request links and ticket comments follow `/report-stage`: read `report-stage/SKILL.md` in the same skills directory as this skill (`.claude/commands/report-stage.md` for Claude Code). Load it before the first ticket read.
- If it cannot be loaded, these rules still hold. Standalone: call start_run before work, reusing a supplied SECTILE_RUN_ID or launch runId, and call finish_run when the entire invocation ends, including errors or stopping for user input; a nested skill never finishes the outer run. Managed run: submit only through the supplied result contract, and call no transition or comment tool.
- Gate: call `transition_stage` with stage `reviewed`, the report note, the actual branch and prUrl set to the verified pull request URL, only when this step is complete (implemented → reviewed).

## Goal
Hand a reviewer a branch that is already worth reading: the obvious problems
found and fixed, the risky parts pointed out, the test plan written down.

## Read first
- The full diff of the branch against the default branch. All of it, not the summary.
- The specification, to check that what was asked is what was built.
- The current remote default branch: fetch the remote and identify its configured
  default branch before reviewing or publishing.

## Steps
1. Verify a matching PR exists for the task repository and branch before changing files: open, or already merged by the human. Record its URL. If missing, stop and recover through the configured creation owner (specify or implement). Never create a PR during adjustment, and never push onto a merged PR — review the merged state and report it. Read available PR feedback; retrieval failure is a blocker, not absence of feedback.
   Fetch the remote (`git fetch origin`) and compare the work branch with the
   remote default branch (normally `origin/main`; use the repository's configured default when different).
   Integrate missing base commits before the final review: prefer rebase when the branch is private, or merge when
   repository policy or shared-branch state requires it. Resolve conflicts and do not continue until the working tree is clean.
2. Review the complete resulting diff against the specification for correctness, side effects, security, and edge cases with no test. Address actionable feedback and record dispositions. No human feedback is required.
3. Update documentation affected by the change. Fix what the review finds, now. A known defect belongs in the code, not in the
   description of the merge request.
4. Re-run build, static analysis and tests after integrating the default branch and on the final state.
5. Commit with a conventional message: type, scope, and why the change exists.
6. Push the branch and update the same existing merge request: summary, test plan, and the specific
   places where you want a reviewer's eyes.
   If rebasing an already-pushed branch, use `git push --force-with-lease`, never an unguarded force push.
7. Verify the same PR is open and contains the pushed final commit, update its description and check evidence, then mark it ready. If any check, feedback retrieval, push or readiness verification fails, preserve work and report the blocker. If the repository has no remote, stop.

## Do not
- Do not merge, do not approve, do not close the ticket. That is the user's call.
- Do not create a PR. Do not mark a PR ready on a red build. Report the failure instead.
- Do not complete adjustment on a branch known to be behind the remote default branch.

## Report
- What the review found, and which findings you fixed.
- The merge request URL, or why there is none.
- The test plan a reviewer can replay, as a checklist.

## Ticket
$ARGUMENTS
