# Plan #589 - An attached repository is seen by a running session

Implements `specs/589-attached-repository-seen-by-running-session/spec.md`.
Behaviour lives in the spec; this file says where and how.

## Stack and constraints

- Go only: the local agent (`internal/agent`), the server's stage pull request
  validation (`internal/db/stageprs.go`) and the MCP tool list
  (`internal/taskmcp/server.go`). No web or desktop change is expected; the
  desktop's attached-folders panel reads `kind`, which keeps its values.
- No database migration. The server-agent contract
  (`docs/contracts/server-agent-v1.md`) is unchanged: the `repository_worktree`
  operation keeps its request and its result, only its error text changes.
- Tests that start `httptest` servers need the sandbox off (memory "Go tests
  under the sandbox"); `GOCACHE` goes under `$TMPDIR`.
- New comments in English. The agent's refusals of `repository_worktree` stay
  in French (AGENTS.md: runtime surface); the `stageprs.go` refusal stays in
  English, as it is today.

## What the code does today

- `DB.PrepareRepositoryWorktree` (`internal/db/repositories.go:297`) resolves
  the repository against the project's repositories or accepts any remote
  identity, then relays `repository_worktree` to the caller's agent through
  `callAgentContext` -> `AgentDispatcher.CallOperation`
  (`internal/handlers/agent_operations.go:20`), routed by user and project
  (`waitForRoute`), possibly to another replica (`route.remote`). On success it
  calls `AddChangedRepository`.
- The agent's `executeOperation` (`internal/agent/agent_operations.go:91`)
  fetches the project configuration, calls `localProjectRoot`
  (`agent_config.go:147`), which reads `~/.config/sectile/settings.json` on
  every call (`agentconfig.ReadSettings`), then `repositoryWorktree`
  (`repositories.go:356`).
- `repositoryWorktree` -> `repositoryFolder` (`attached.go:77`) ->
  `attachedFolders` -> `describeFolder` (`attached.go:43`). `describeFolder`
  maps any `git rev-parse` failure to kind `folder` and any
  `git remote get-url origin` failure to "no remote": the error is dropped.
  `repositoryWorktree` then answers the single message « Le dépôt %s n'est ni
  associé ni attaché à ce projet sur ce poste : attachez son dossier dans les
  réglages du projet de l'app desktop. » whatever the reason.
- `POST /desktop/folders` (`agent_desktop.go:247` ->
  `agent_desktop_repositories.go`, `editFolders` l. 277) writes the folder to
  the same settings file at once, under `agentconfig.LockSettings`.
- `validateStagePRs` (`internal/db/stageprs.go:93`) refuses a link outside
  `taskChangedRepositories` + primary with « pull request %s is not in a
  repository %s changed (%s) ».
- The agent knows its device name (`d.link.deviceID`, `agent.go:104`), the one
  shown in activity steps.

## Phase 1 - Reproduce (FR1, FR2)

1. Test `TestAttachingAFolderAppliesToTheNextOperation` in
   `internal/agent/agent_desktop_repositories_test.go`, built on the fixtures of
   `TestDesktopAttachesFoldersToAProject` and
   `TestRepositoryWorktreeInAnAttachedFolder`: one daemon with a temporary
   `HOME`, a project checkout, a second repository with an origin and a task
   branch.
   - Run `repository_worktree` for the second repository through
     `executeOperation` (with the test server that answers the project
     configuration and the task, as the existing operation tests do): refused
     as not attached.
   - `POST /desktop/folders` with the folder, on the same daemon.
   - Run the same operation again: expected to return the worktree.
   - Detach through the same endpoint, run again: refused.
   If this test fails, the cause is inside the agent and is fixed there
   (Phase 3). Expected green, which rules out caching.
2. Manual repro on the owner's workstation (task T3), with the desktop's agent
   log open:
   - count the agent connections for the user and the SFE project: the server
     log lines on agent registration, or the desktop's agent status, and any
     separately started `sectile agent`;
   - check which server the session's MCP bridge talks to, against the
     desktop's (memory "Sectile MCP bridge endpoint": a bridge on another port
     reaches another server and its own agents);
   - follow the ticket's steps 1 to 5 and note, from the agent logs, which
     device answered the `repository_worktree` operation.
   To make that observable, Phase 2 adds a log line per operation.

## Phase 2 - Diagnose folders and name the reasons (FR4, FR5, FR6)

1. `internal/agent/attached.go`: add `Err string` to `attachedFolder`
   (not serialized to the desktop: the `/desktop/folders` answer builds its own
   struct, l. 131, and keeps its fields). In `describeFolder`:
   - `os.Stat` fails or not a directory: kind `missing`, as today;
   - `git rev-parse --show-toplevel` fails: kind `folder` when the output says
     "not a git repository", else kind `folder` with `Err` holding the `git`
     message;
   - `git remote get-url origin` fails: no remote when the output says there
     is no such remote, else `Err` holding the message.
   The distinction reads `git`'s exit code 128 plus its message; keep the
   matching in one helper with a test on real folders.
2. `internal/agent/repositories.go`, `repositoryWorktree`: when
   `repositoryFolder` finds nothing, build the refusal from the attached
   folders of the project:
   - no folder attached, or every attached folder read cleanly with another
     origin or kind: « Le dépôt R n'est ni associé ni attaché au projet sur ce
     poste (<device>) : attachez son dossier dans les réglages du projet de
     l'app desktop. » followed, when folders were checked, by one line per
     folder with what it is (US2-5);
   - otherwise, one line per folder that failed: missing (US2-2), not a Git
     checkout (US2-3), no origin (US2-4), `git` error with its message (US2-6),
     without the "attach" advice (FR5).
   `repositoryWorktree` needs the device name: pass it in (it is a free
   function today; add a parameter or a small options struct rather than
   reading a global).
3. `executeOperation`: log one line per operation with action, project, task,
   repository and outcome (`log.Printf`, existing style of `agent_trace.go`),
   so the owner's repro shows which device answered.

## Phase 3 - Fix the confirmed cause (FR3)

Depends on Phase 1. Expected shapes per candidate, to be confirmed:

- Another agent answered (two connections for one user and project, or a
  bridge on another server): the device named in the refusal (Phase 2) makes it
  visible. If the dispatcher keeps a stale route after the desktop's agent
  reconnects, fix the route replacement in `AgentDispatcher` (routing by user
  and project); otherwise the refusal naming the device is the by-design
  answer (FR3).
- Attachment saved on another project: nothing to fix in the agent; the refusal
  lists the folders attached to *this* project, which shows it.
- `git` failing in the agent's environment: covered by Phase 2; the fix, if
  any, is in the command environment (`agentexec.Hidden`, `gitLocal`).

Record the confirmed cause in the PR description and append it to
`docs/clarifications/589.md`. If the fix changes routing or introduces a
rule about which agent answers, propose an ADR under `docs/adrs/`.

## Phase 4 - Messages and documentation (FR7, FR8, FR9)

1. `internal/db/stageprs.go:93`: append « ; a repository becomes changed when
   prepare_repository_worktree is called for it » to the refusal. Update the
   tests that match the message (`internal/db/*stageprs*_test.go`,
   grep "is not in a repository").
2. `internal/taskmcp/server.go:390`: extend the tool description with US4-1.
   Update any test that pins the description.
3. `CHANGELOG.md`: one `Fixed` line under `## [Unreleased]` (FR9).
4. `docs/contracts/server-agent-v1.md`: only if the error text of
   `repository_worktree` is documented there (check; no change otherwise).

## Files

| File | Change |
| --- | --- |
| `internal/agent/attached.go` | `Err` on `attachedFolder`, finer `describeFolder` |
| `internal/agent/repositories.go` | refusal built from the folder diagnosis, device name |
| `internal/agent/agent_operations.go` | pass the device name, one log line per operation |
| `internal/agent/agent_desktop_repositories_test.go` | reproduction test (FR1) |
| `internal/agent/repositories_test.go` | one test per refusal case (US2) |
| `internal/db/stageprs.go` and its tests | refusal wording (FR7) |
| `internal/taskmcp/server.go` | tool description (FR8) |
| `CHANGELOG.md` | `Fixed` line (FR9) |
| `docs/clarifications/589.md` | confirmed cause |
| routing code, if Phase 3 points there | per confirmed cause |

## Rejected alternatives

- Refreshing `SECTILE_REPOSITORIES` in a running process: impossible from
  outside the process, and nothing decides on it (clarification, round 1).
- Accepting unprepared repositories in `prUrls` by asking the agent about
  attachments during `transition_stage`: refused by the owner (round 2).
- Caching attachments with an invalidation on `POST /desktop/folders`: the
  per-call read is cheap (one settings file, two `git` calls per folder) and
  is what makes an attachment apply at once.
