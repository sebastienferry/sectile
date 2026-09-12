---
name: create-pr
description: "Review the branch like a peer would, fix what the review finds, then open the merge request and leave the merge to the user."
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
- **Standalone invocation**: After verifying each completed step, use the local handler below. Check its exit status and response. If it is unavailable, preserve work and report the pending transition; do not silently diverge local and tracker state.
Transition implemented → reviewed only when this step is complete.
```bash
curl --fail-with-body --silent --show-error -X POST http://localhost:8090/api/tasks/stage \
  -H 'Content-Type: application/json' \
  -d '{"taskKey":"<KEY>","stage":"reviewed","note":"<REPORT_NOTE>","prUrl":"<PR_URL>"}'
```
This POST calls the local TaskFlow handler directly. Confirm HTTP success before continuing. If it is unavailable, preserve work and report the pending transition; do not silently diverge local and tracker state.
Reuse the assigned worktree and actual branch. Never merge or delete remote objects. Keep work available for review and retry until confirmed handoff.
