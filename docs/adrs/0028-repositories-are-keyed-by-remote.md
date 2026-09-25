# ADR 0028: A project's repositories are keyed by remote, and each workstation maps them

Status: Accepted

## Context

A run got exactly one working directory: the project root the workstation
maps. On a multi-repo project (`monoRepo=false`) that is wrong in three ways.
Nothing says which repository a ticket belongs to. The agent cannot see the
sibling repositories. A ticket that must change two repositories has nowhere to
do it.

A per-ticket working directory already existed (`tasks.repo_path`, fed into
`projects.repo_paths`), but the local agent never used it. ADR 0003 and ADR 0026
also rule that a server path is never a working directory: the paths came from
whoever typed them, and mean nothing on another workstation.

#456 settles the model. Its clarification is in `docs/clarifications/456.md`,
and its specification is in `specs/456-multi-repo-task-repositories/`.

## Decision

**A repository is identified by its remote.** Every comparison uses
`models.RepositoryIdentity`, the lowercased `host/path` with no scheme, user,
port or `.git`.
- The server stores a project's repositories as remotes (`projects.repositories`).
  The code remote is always first. It is derived from the project and never
  stored twice.
- A ticket stores the identity it is pinned to (`tasks.repository`), plus the
  secondary repositories it has a worktree in (`tasks.changed_repositories`).
- A second spelling of a repository already listed is refused, not merged.

**A workstation maps each repository to a folder.** The mapping lives in
`~/.config/sectile/settings.json` under `repositories`, keyed by identity, so one
checkout serves every project that works in it. A folder is accepted only if its
checkout's `origin` is that repository. The server never receives a path.

**The primary repository is resolved before anything starts.** The order is:
1. the ticket's pin;
2. the project's only repository;
3. the code remote of a mono-repo project;
4. the only repository mapped on this workstation, which is then pinned so later
   stages stay there.

Otherwise the launch waits.

**A launch that cannot choose waits on the same run.** The local agent parks the
dispatch and marks the run `waiting_reason='repository'`. While parked, it holds
no run slot. It reads the ticket back over REST (an MCP call from a session would
clear a wait) until the ticket is pinned, then resumes. A cancel ends the run as
canceled.

This is the one exception to "a headless run is left unmarked"
(`ReportRemoteRunWaitingAs`). The answer is a pin anyone can give from the
board, not a reply in the session, so an autonomous run is marked too.

**The agent is told every folder.** Each launch receives a folder map, built by
the local agent and never returned by the server. It is carried in
`SECTILE_REPOSITORIES` and in the prompt, and gives for each repository:
- its role: primary, changed, context or spec;
- its folder, or that it is not mapped here;
- the ticket's worktree in it, when there is one.

Claude also receives the other folders as `--add-dir`, and templates can place
them with `{addDirs}`. No flag is guessed for the other providers. Context
folders are read-only by instruction only.

**A secondary worktree is created on demand, by the local agent.** A skill that
must change a context repository calls `prepare_repository_worktree`. The
server relays the request to the caller's agent as the `repository_worktree`
operation, exactly as ADR 0026 does for macro worktrees. The agent creates or
reuses the ticket's worktree there with the logic of the primary one, on the
same branch. The repository is then a changed repository.

**Every changed repository needs its pull request.** The transitions that
require pull request evidence, the post-back and the adjust prerequisite match
one pull request to each changed repository. Each is checked with the rules of
#392, on its own checkout's head. A link naming another repository is refused,
and so is a changed repository without a pull request. A ticket that changed a
single repository keeps the single pull request it always had.

**Legacy paths are converted once, by a workstation.** Only a workstation can
read a checkout's `origin`, so the first local agent that sees an unconverted
project handles its old paths:
- it resolves every legacy path it can;
- it drops the others, which are "not found", "not a git checkout" or "no origin";
- it posts the report.

The server applies the first report under a compare-and-set on
`projects.repositories_migration` and keeps it for the project settings to show.

## Consequences

- A multi-repo ticket runs in its own repository on every stage. Its worktree
  can live in a checkout other than the project root, which is kept out of
  that checkout's status through `info/exclude`.
- The folder map makes the sibling repositories visible, but not protected.
  A provider that ignores the instruction can still write in a context folder.
- A secondary worktree exists only on the workstation that created it.
  Validating a transition from another workstation falls back on the forge's
  evidence, with the notice of #392.
- A path typed on a ticket from another workstation is dropped by the first
  workstation that converts. That is the owner's decision; the report names
  every dropped path and the tickets that used it.
- The local MCP bridge accepts thirteen tools, so the server and the agent must
  be upgraded together.

## Rejected alternatives

- **Absolute paths on the server**, today's `repo_paths`. They mean nothing on
  another workstation, and the server cannot check them.
- **A worktree in every repository at launch.** It is slow and noisy for the
  tickets, the common case, that touch a single repository.
- **The skill running `git worktree add` itself.** Branch reuse, naming and the
  record of changed repositories would depend on the model.
- **Re-dispatching once pinned instead of parking.** It creates a second run and
  loses the launch's mode and options.
- **Enforcing read-only with permission rules.** It is provider-specific, and it
  was decided as an instruction only.
