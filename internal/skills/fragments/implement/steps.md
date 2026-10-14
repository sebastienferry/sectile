1. Reuse the assigned worktree and branch, including a shared batch branch. Never implement on the default branch.
2. When the task has more than one folder, `$SECTILE_REPOSITORIES` (and the prompt's folder
   block) lists them with their role: the project's repositories, the folders attached to the
   project on this workstation, and the specifications folder. Work in the primary worktree.
   A `context` folder is read-only: to change it, call `prepare_repository_worktree` for its
   repository first and work in the worktree it returns. That repository then has role
   `changed` and needs its own pull request, given to `transition_stage` in `prUrls`. A
   `local` folder (attached, no remote) is changed in place, with no worktree and no pull request.
   A repository the task needs but no folder holds is refused unless the workstation has the
   project's Any repository option on: then pass the top level of a local checkout of it as
   `path`, or omit `path` to let the agent clone it. When the launch says no worktree was
   created in the code repository, prepare it the same way before changing it; left
   unchanged, it needs no pull request. When the answer says the worktree was not added to
   the session, ask the user to run `/add-dir` with its path before writing there.
3. Work through the checklist in small steps, each one leaving the tree buildable. When the
   specification artefacts are ignored by Git, commit the code only and never force-add them.
4. Add the tests that cover the new behaviour and its edge cases, not just the
   happy path. A change with no test needs a stated reason.
5. Run build, static analysis and tests. Fix until green, and quote the real output.
6. Re-read your own diff before finishing, as a reviewer would.