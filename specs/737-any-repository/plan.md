# Plan #737 - Agent: option to work in any repository, not only the project's

Behaviour and acceptance criteria are in `spec.md`; this file says how.

## Stack

Go agent (`internal/agent`, `internal/agentconfig`), Go server
(`internal/db`, `internal/taskmcp`, `internal/agentmcp`, `internal/handlers`),
shared models (`internal/models`, `internal/agentprotocol`), Desktop
(`desktop/src/main.js` and its settings panel), web (`web/src`), skill
fragments (`internal/skills/fragments`). No database migration: the option is
a workstation setting and the pin already has its column (`tasks.repository`).

## Decisions

Choices the clarification left to the specification, taken from the code.

- D1. **Settings shape.** `ProjectSettings` gains `AnyRepository bool`
  (`anyRepository`) and `ClonesPath string` (`clonesPath`). Both are omitted
  when zero, so an untouched section stays as it is, and both join `isZero`,
  `overlay` and the edit helper like `IssueSpecPath` did in #736.
- D2. **Remembering writes the existing mapping.** `Settings.Repositories`
  (identity to folder, workstation-wide) is already read by `repositoryRoot`
  for any identity. A found or cloned folder is written there under
  `LockSettings`, as `rememberConvertedFolders` does. No new key.
- D3. **The operation carries the URL and the path.** `agentprotocol.Operation`
  gains `RepositoryURL` (what the caller typed, trimmed) and `Path`. The
  server keeps sending `Repository` as the identity, so an older agent still
  answers for a known folder; a `path` sent to an older agent is detected by
  a new echo field `PathChecked` in `models.RepositoryWorktree`, and the server
  refuses with "update the agent" when it is missing.
- D4. **Clone target name.** `<clones folder>/<last segment of the identity>`.
  An existing folder of another repository is a refusal, never a rename.
  The clone goes into a temporary sibling (`.<name>.cloning`) renamed on
  success, so a failure leaves nothing named like the repository.
- D5. **Clone URL scheme.** From `RepositoryURL` when it parses as a URL or an
  scp-like `git@host:path`; else from `host/path`, with the scheme of the
  project's `GitRemoteURL` (`git@host:path.git` for SSH, `https://host/path.git`
  otherwise).
- D6. **Fetch.** `ensureLocalWorktree` gains a `fetch bool`. Secondary callers
  (`repositoryWorktree`, the pinned primary of an undeclared repository, the
  lazy code worktree) pass true: `git fetch origin` (no prune, no tags), then
  base `origin/<branch>` when present, else `origin/HEAD`, else the remote
  default read by `git ls-remote --symref origin HEAD`, else local `HEAD` with
  a warning. The launch's code worktree keeps passing false: its behaviour is
  out of scope.
- D7. **"Specifications away from the code".** `config.DropsSpecArtifacts()`
  or `resolveSpecFolder` for the Issue folder returning a distinct directory
  (`spec_folders.go`). Both are already computed at launch.
- D8. **Lazy code worktree at launch.** In `prepareDispatchLocked`, when the
  option is on, specifications are away from the code, the ticket is not
  pinned, and `worktreeForBranch(root, branch)` finds no existing tree, the
  working directory is the Issue specifications worktree when distinct, else
  the project checkout; `ensureLocalWorktree` is skipped. The folder map lists
  the code repository with role `context`, which the session prompt already
  explains ("call prepare_repository_worktree for its repository first").
- D9. **`prepare_repository_worktree` on the code repository.** The server's
  refusal "is the primary repository of …: it already has its worktree" is
  lifted for the code repository; the agent answers with
  `ensureLocalWorktree(root, task, true, …, fetch=true)` on the project
  checkout, which reuses an existing tree. The code repository is then
  recorded with `AddChangedRepository` like any other, which is what tells
  the stage check it changed.
- D10. **Branch name before any launch.** `PrepareRepositoryWorktree` no longer
  refuses a task without `BranchName`: it computes
  `models.TaskBranchName(project.BranchNameFormat, key, title)` and sends it,
  as `taskWorktreeBranch` would on the agent.
- D11. **Pull request exemption.** `branch_changes` answers a new
  `LazyCode bool`: true when the option is on and specifications are away from
  the code on that workstation. `validateStagePRs` treats the primary code
  repository like a secondary one (skipped when unchanged) only when no pull
  request was given for it, the code repository is not in
  `ChangedRepositories`, `branch_changes` reports `LazyCode` and
  `Exists == false`, and at least one other repository has a pull request.
  The set's current pull request is then the first changed repository's
  (`stagePRSet.primary` takes the first URL when the primary has none).
  `multiRepoTask` is already true for such a task, since it changed a
  secondary repository.
- D12. **Pin by URL on the server.** The server cannot see a workstation
  option, so it accepts a pin that is a declared repository or any remote
  identity (`remoteIdentity`), storing the URL as typed. `taskPin` returns the
  identity for an undeclared pin too. `ErrRepositoryNotInProject` remains for
  a value that is neither (a bare word, a path). The launching agent decides
  (US8.4).
- D13. **Pin resolution on the agent.** `models.ResolvePrimaryRepository` gains
  an `any bool`; with it, an undeclared pin resolves to
  `ProjectRepository{URL: pin, Identity: RepositoryIdentity(pin)}` and the
  outcome is `PrimaryResolved` when mapped, `PrimaryUnmapped` otherwise.
  `primaryRoot` then clones on `PrimaryUnmapped` when the option is on (D4,
  D5), and fails with a message naming the option when it is off.
- D14. **Adding the folder to the running session.** After a worktree is
  created by `repository_worktree`, the agent looks up its own running Claude
  Code sessions of the ticket (`d.terminal.manager`, the runs of
  `d.queue` with that task ID) and types `/add-dir` through `typeAddDir`
  (`agent_run_folders.go`), skipping a folder already inside the session's
  directories. `models.RepositoryWorktree` gains `AddedToSession bool`; the MCP
  answer says to run `/add-dir <path>` when it is false.
- D15. **Message language.** New runtime messages are written in English;
  the French messages they sit next to (`repositoryNotFound`, `primaryRoot`)
  only gain a sentence naming the option, in French, inside the existing
  string.

## Data contracts

### Workstation settings (`agentconfig.ProjectSettings`)

```json
{ "anyRepository": true, "clonesPath": "/Users/me/Sources" }
```

### Agent operation `repository_worktree`

Request (`agentprotocol.Operation`): `repository` (identity), `repositoryUrl`
(new, as typed), `path` (new, optional), `branch`.

Answer (`models.RepositoryWorktree`):

```json
{
  "repository": "github.com/acme/akamai-python",
  "path": "/Users/me/Sources/akamai-python/.tasks/worktrees/PE-1668",
  "branch": "feat/PE-1668",
  "source": "path | clone | mapping | attached | project",
  "pathChecked": true,
  "remembered": true,
  "addedToSession": true,
  "warning": ""
}
```

`source`, `pathChecked`, `remembered`, `addedToSession` and `warning` are new.

### Agent operation `branch_changes`

Answer gains `lazyCode bool` (D11).

### MCP `prepare_repository_worktree`

Input gains `path` (string, optional): "Absolute path of a local checkout of
the repository, used only when the workstation's Any repository option is on
and no mapped or attached folder already holds it." The description says the
agent clones when the option is on and no folder answers, and that the code
repository may be prepared this way when the ticket was launched without its
worktree.

## Target files

| Area | Files |
| --- | --- |
| Settings | `internal/agentconfig/workstation.go`, `internal/agentconfig/local.go` |
| Desktop settings API | `internal/agent/agent_desktop_settings.go`, `internal/agent/agent_desktop.go` |
| Lookup, path, clone, remember | `internal/agent/attached.go`, `internal/agent/repositories.go`, new `internal/agent/any_repository.go` |
| Fetch | `internal/agent/agent_config.go` (`ensureLocalWorktree`) |
| Lazy code worktree | `internal/agent/agent_config.go` (`prepareDispatchLocked`), `internal/agent/spec_folders.go`, `internal/agent/repositories.go` (`buildFolderMap`) |
| Running session | `internal/agent/agent_run_folders.go`, `internal/agent/agent_operations.go` |
| Protocol and models | `internal/agentprotocol/operations.go`, `internal/models/repository.go`, `internal/models/models.go` |
| Server | `internal/db/repositories.go`, `internal/db/stageprs.go`, `internal/db/db.go` (pin check) |
| MCP | `internal/taskmcp/server.go`, `internal/agentmcp/mcp.go`, `internal/mcptest/contract.go` |
| Desktop UI | `desktop/src/main.js` and the project settings renderer that edits `issueSpecPath` |
| Web | `web/src/components/TaskDetailModal.tsx`, its locales |
| Skills | `internal/skills/fragments/implement/steps.md`, `internal/skills/fragments/contracts/transition.md`, goldens |
| Docs | `docs/USER_GUIDE.md`, `docs/CAPABILITIES.md`, `docs/adrs/0052-any-repository-option.md`, `CHANGELOG.md` |

## Risks

- **Workstation-wide memory.** A mapping written for one project serves all
  (owner's choice, Q4). A wrong `path` is prevented by the `origin` check.
- **Clone duration.** A large clone runs inside an MCP call. The existing
  agent call timeout applies; the clone runs with the call's context so a
  cancelled call stops it, and D4's temporary folder is removed.
- **Sandbox writes.** Claude Code's sandbox allows writing in the session's
  directories, `--add-dir` and `/add-dir` included; US4 relies on it. Verify
  on a sandboxed launch before closing the ticket.
- **Server pin acceptance (D12).** A pin to an undeclared repository is
  accepted on every workstation's view of the board; only a launch on a
  workstation without the option fails, with a message that names it.
