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