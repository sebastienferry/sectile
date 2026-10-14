# Design: Skills No Longer Describe Multi-Repo Projects (#541)

## Context
The agent writes the folder map (`internal/agent/repositories.go`, `folderMapPrompt`) whenever a task has more than one folder, whatever the project. Its roles are `primary`, `changed`, `context`, `local` and `spec`. The skills must describe that rule, not a project kind.

## Decisions
1. **Implement step 2** says, in order:
   - `$SECTILE_REPOSITORIES` and the prompt's folder block list the task's folders whenever there is more than one: project repositories, folders attached on the workstation, and the specifications folder.
   - Work in the primary worktree.
   - A `context` folder is read-only: to change it, call `prepare_repository_worktree` and work in the worktree it returns. It then has role `changed` and needs its own pull request, given to `transition_stage` in `prUrls`.
   - A `local` folder (attached, no remote) is changed in place, with no worktree and no pull request.
   - The existing sentences on the Any repository option, the missing code worktree and `/add-dir` are kept word for word.
2. **Handoff step 6** names "a task with repositories of role `changed` in `$SECTILE_REPOSITORIES`" rather than "a task that changed several repositories", and step 7 starts on its own line.
3. **Guard test** `TestFragmentsNameNoProjectKind` walks `fragments/` and fails on any case-insensitive match of `mono-repo`, `monorepo`, `multi-repo` or `multirepo`.
4. **No changelog entry**: only the agent's instructions change.
