1. Reuse the assigned worktree and branch, including a shared batch branch. Never implement on the default branch.
2. On a multi-repo project, `$SECTILE_REPOSITORIES` lists the task's folders. Work in the
   primary worktree; the other repositories are read-only context. To change one, call
   `prepare_repository_worktree` for it first and work in the worktree it returns: each
   changed repository then needs its own pull request, given to `transition_stage` in `prUrls`.
3. Work through the checklist in small steps, each one leaving the tree buildable.
4. Add the tests that cover the new behaviour and its edge cases, not just the
   happy path. A change with no test needs a stated reason.
5. Run build, static analysis and tests. Fix until green, and quote the real output.
6. Re-read your own diff before finishing, as a reviewer would.