# ADR 0051: Macro and issue specifications have a folder each

- Status: Proposed
- Date: 2026-10-05
- Issue: [#736](https://github.com/sebastienferry/sectile/issues/736)
- Supersedes in part: [ADR 0027](0027-specifications-folder-is-a-workstation-setting.md),
  its single specifications folder per project

## Context

ADR 0027 gave each project one specifications folder per workstation, and only
the macro side used it: the macro worktree, the macro skills and the slicing
import. The issue skills wrote their clarification reports and specifications
in the task's worktree of the code repository, with no way to keep them
elsewhere. A team that keeps its issue specifications in a repository of their
own, or apart from its macro specifications, could not.

## Decision

**Two workstation settings per project, one per skill-set.** The **Macro
specifications folder** is the former specifications folder, stored under the
same `specPath` key, so an existing workstation keeps its value and needs no
migration. The **Issue specifications folder** is new, stored as
`issueSpecPath`, empty by default. Either one left empty is the code checkout,
which is what every workstation did before. Both stay workstation settings: the
server holds no path, for the reasons ADR 0027 gives.

**Macro operations read the Macro folder only, task launches the Issue folder
only.** A task's folder map lists the Issue folder; a project-level session,
which may work on either, lists both.

**A distinct Issue folder gets a worktree per task, prepared as a macro's is.**
When the Issue folder is not the task's code checkout, the agent prepares a
worktree of it under `.tasks/worktrees/`, on a branch named like the task's own
branch, started from the up-to-date default branch or from the remote branch
when it exists, and reused as it is afterwards. Worktrees off and a plain
folder behave as for a macro. The preparation code is shared with the macro
worktree rather than copied, so the two cannot drift. Every task run receives
the result as `SECTILE_SPEC_REPO`, `SECTILE_SPEC_BRANCH` and
`SECTILE_SPEC_WORKTREE`, the names a macro run already uses; without a distinct
folder they repeat the task's worktree, so a skill always reads one place.

**A skill typed by hand asks the agent.** The `prepare_task_spec_worktree` MCP
tool, relayed to the caller's agent as the `task_spec_worktree` operation,
returns the workspace a launch would have prepared, as
`prepare_macro_worktree` does for macros.

**The skill publishes the branch; the human opens its pull request.** With a
distinct folder, the issue skills commit only the task's artefacts there and
push the branch with a plain push. Opening a pull request in that repository
is left to the owner, as for macros, and the code pull request names the
specifications repository and branch. Nothing new is recorded on the task. The
code branch then carries no artefact before implementation, so no code pull
request is opened at the clarified or specified stage, as when the artefacts
are dropped (#487). `handoff-issue` removes the specifications worktree and its
local branch only once that branch is merged into its repository's default
branch, and keeps both otherwise.

## Consequences

- An existing workstation behaves exactly as before: its folder becomes the
  Macro folder and the Issue folder is the code checkout.
- A configured folder that no longer exists refuses the launches and operations
  that need it, naming the setting to fix.
- The dropped artefacts exclusion (#487) is written in the Issue folder when it
  is distinct, since that is where the artefacts are.
- A pull request in the specifications repository is a manual step, and
  Sectile does not track it. A follow-up could record it on the task beside
  the code pull request.
- The MCP catalog gains a tool: a server and an agent from before this change
  must be upgraded together.
- The single "Specifications folder" of the Desktop project settings becomes
  two rows built by the same code.

## Rejected alternatives

- **Reusing the secondary repository worktree** (`prepare_repository_worktree`)
  for the Issue folder: it starts a new branch from the checkout's `HEAD` and
  knows neither plain folders nor a bounded fetch, which a specifications
  repository needs and the macro worktree already does.
- **A new key for the Macro folder** (`macroSpecPath`): it would have needed a
  settings migration for no visible benefit.
- **Launched runs only**, with no MCP tool: a skill typed by hand would have
  written in the code worktree, ignoring the setting.
- **The skill opening the specifications pull request** and recording it on the
  task: it puts a second repository's pull request in the stage contract for a
  gesture the owner already makes for macros.
