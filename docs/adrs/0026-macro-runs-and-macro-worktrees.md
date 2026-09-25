# ADR 0026: Macro skill runs are project activities, and macro worktrees live on the agent

Status: Accepted

## Context

#426 ports taskativ's `realign-macro`: a skill that runs for a macro, not for a
task, and writes the macro's specification on the macro's own branch, in a
worktree of its own so that two macros specified at once never share untracked
files. Until then every hop of a skill launch in Sectile was keyed on a task:
the `run-skill` route, the `remote_run` activity, the `dispatch_step` payload,
the agent's workspace preparation, and `start_run` / `finish_run`.

Two questions had to be answered before any code: what a run belongs to when
there is no task, and which machine creates the macro worktree.

## Decision

**A macro skill run is a project activity naming its macro.** It is a
`task_activities` row with `project_id` set, `task_id` NULL (the schema's
`CHECK (task_id IS NULL OR project_id IS NULL)` holds) and a new `macro_key`
column (numbered migration). The busy check, the run list of the macro panel
and the closure all read `macro_key` through dedicated queries; the six
explicit column lists of `task_activities` are untouched, `macro_key` being
written by an UPDATE after the insert, as `run_mode` already is. A macro run has
no stage: it checks none, moves none, and its end hands nothing back to a
workflow chain. `start_run` and `finish_run` take `projectId` with `macroKey`
instead of `taskKey`; naming both forms is refused.

**The macro worktree is created by the local agent**, on the machine that holds
the checkouts and runs the session, exactly as task worktrees are. The
dispatch carries `macroKey` and no task; the agent prepares
`.tasks/worktrees/<KEY>` in the specifications repository, on the macro's
branch, from the fetched default branch, and passes it to the session as
`SECTILE_SPEC_REPO` / `SECTILE_SPEC_BRANCH`. A skill invoked by hand asks for the
same preparation through the `prepare_macro_worktree` MCP tool, which the
server relays to the caller's agent as the `macro_worktree` operation.

**The specifications repository has two declarations.** The project's
`specRepoPath` is read by the server only, for the slicing import it runs on
its own filesystem. The agent never receives it: `AgentConfig` carries no
server path by design, so the workstation declares its own specifications
checkout per project (`specRepos` in the local settings, edited in the desktop
project dialog), and without one the project's local checkout carries the
specifications (owner decision, 2026-09-24).

## Rejected alternatives

- **A pseudo task identifier** (`macro-<key>` in `task_id`) would have reused
  every task path unchanged. It is the overloading #310 removed from that
  column, and it breaks the column's foreign key on PostgreSQL.
- **A server-side macro worktree.** The server already reads the slicing from
  its filesystem, but the session and the skill write on the agent's machine; a
  worktree the server creates names a path the agent may not have.
- **Sending the server's specifications path to the agent as a hint**, used when
  it exists locally. Simpler, but the first exception to the rule that the agent
  contract carries no server path, and a silent wrong checkout whenever the two
  machines happen to share a path layout.

## Consequences

- An agent older than #426 refuses a macro dispatch (it reads a task dispatch
  without a task), and the stdio MCP bridge refuses a catalogue of eleven tools:
  server and agent are upgraded together, as for every catalogue change.
- The macro worktree is never reset: an existing tree is reused with its
  uncommitted work, and a non-empty stale directory at its path is refused by
  name rather than deleted.
- A workstation whose specifications live apart from the code must declare
  them in the desktop app as well as in the project options; the option hint
  and the desktop field say so.
