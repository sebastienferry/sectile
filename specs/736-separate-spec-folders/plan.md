# Plan #736 - Separate Macro and Issue specifications folders on the workstation

## Stack

Go agent and server (`internal/agentconfig`, `internal/agent`,
`internal/agentprotocol`, `internal/db`, `internal/taskmcp`, `internal/agentmcp`,
`internal/mcptest`, `internal/skills`), Sectile Desktop (`desktop/src/main.js`,
Playwright UI tests under `desktop/tests`), Markdown docs.

## Data

### Workstation settings (`internal/agentconfig/workstation.go`)

```go
type ProjectSettings struct {
    Path string `json:"path,omitempty"`
    // MacroSpecPath is the Macro specifications folder. Its JSON key stays
    // "specPath", the key of the single folder it replaces, so an existing
    // workstation keeps its value with no migration.
    MacroSpecPath string `json:"specPath,omitempty"`
    // IssueSpecPath is the Issue specifications folder (#736).
    IssueSpecPath string `json:"issueSpecPath,omitempty"`
    ...
}
```

- `isZero` tests both fields; `overlay` in `local.go` merges both with
  `firstSet`; the Desktop edit helper sets one or the other.
- `Settings.SpecPath(id)` becomes `MacroSpecPath(id)` and `IssueSpecPath(id)`.
- No settings layout bump: the new key is additive.

### Desktop settings API (`agent_desktop_settings.go`, `agent_desktop.go`)

- Input: `IssueSpecPath *string json:"issueSpecPath"`, normalised by
  `normalizeSpecFolder` like `specPath`; nil keeps, "" clears.
- GET adds `issueSpecPath`, `issueSpecDefault` (the root, as `specDefault`) and
  `issueSpecKind` (`specFolderKind` of the effective folder).

### Agent protocol (`internal/agentprotocol/operations.go`)

- New task-scoped operation `task_spec_worktree` in `Operations`, answered with
  `models.TaskSpecWorkspace`:

```go
type TaskSpecWorkspace struct {
    Repository string `json:"repository"` // resolved Issue folder
    Path       string `json:"path"`
    Branch     string `json:"branch"`
    Worktree   bool   `json:"worktree"`
    Distinct   bool   `json:"distinct"`
    Warning    string `json:"warning,omitempty"`
}
```

## Resolution (`internal/agent/agent_macro_dispatch.go` → `spec_folders.go`)

- `resolveSpecFolder(stored, root, setting string) (string, error)` holds the
  current `localSpecRepo` body; the error names the setting:
  "le dossier des spécifications Macro %s déclaré sur ce poste est
  introuvable (réglage « Macro specifications folder »)", and the Issue
  counterpart. The setting name is quoted in English because that is the label
  Desktop shows.
- `localMacroSpecRepo(overrides, projectID, root)` and
  `localIssueSpecRepo(overrides, projectID, root)` wrap it. Every current
  `localSpecRepo` caller is a macro one except `taskFolderMap`, which moves to
  the Issue resolver.

## Specifications worktree (`internal/agent/agent_macro_worktree.go`)

`ensureMacroWorktree` is split so its body serves both owners:

- `ensureSpecWorktree(ctx, specRepo, branch, name, useWorktrees)`: plain folder
  in place, lock, bounded fetch, default branch, reuse by branch, worktrees off,
  `.tasks/worktrees/<name>`, start point `origin/<branch>` else the default
  branch base. Returns `models.MacroWorkspace`-shaped data.
- `ensureMacroWorktree` keeps its signature: it validates the key, picks the
  existing macro branch or `MacroBranchName`, then calls the helper.
- `ensureTaskSpecWorktree(ctx, config, overrides, root, task, codeWorkDir,
  codeBranch)`:
  - Issue folder resolved; error as above.
  - Not distinct (`sameDirectory(issue, root)` or the task's primary root):
    returns the code worktree, `codeBranch`, `Worktree` true when
    `codeWorkDir` differs from root, `Distinct` false.
  - Distinct: branch = `codeBranch` (the task branch the launch computed;
    `taskWorktreeBranch` when called from the MCP operation before any launch),
    name = `safeWorktreeName(task.Key)`, then `ensureSpecWorktree`. The
    excludeTaskWorktrees call keeps `.tasks/` out of that checkout.
  - `applySpecArtifacts` is run on the Issue folder too when it is distinct, so
    the #487 exclusion lands where the artefacts are.

## Launches

- `prepareTaskLaunch` (`agent_config.go` around line 359) returns the
  specifications workspace with the rest; the two launch paths
  (`agent.go` around line 1245 and `agent_desktop.go` around line 1385) add
  `SECTILE_SPEC_REPO`, `SECTILE_SPEC_BRANCH`, `SECTILE_SPEC_WORKTREE` to the
  environment and, when there is a warning, a "Specifications workspace
  notice" line to the prompt, as the macro launch does.
- `agent_console.go` keeps clearing the three variables for a bare console.
- `taskFolderMap` reads the Issue folder and gives its `spec` entry the
  specifications worktree in `Worktree`.
- `attachFolder` refuses the Macro and the Issue folder with distinct messages
  ("is already the project's Macro specifications folder" / "Issue").

## Server and MCP

- `internal/db`: `PrepareTaskSpecWorktree(ctx, userID, taskKey)` resolves the
  task and project and calls the agent with `Action: "task_spec_worktree"`,
  `TaskID`, `ProjectID`; the task's `BranchName` travels in the operation so the
  agent names the same branch.
- `internal/agent/agent_operations.go`: the task-shaped path answers
  `task_spec_worktree` after the task is read, through
  `ensureTaskSpecWorktree` with the code worktree `worktreeForBranch` finds
  (no code worktree is created by this operation).
- `internal/taskmcp/server.go`: tool `prepare_task_spec_worktree`
  (`taskKey`), description modelled on `prepare_macro_worktree`.
- `internal/agentmcp/mcp.go`: whitelist entry; count tests and
  `internal/mcptest/contract.go` updated (memory "New MCP tool touches the
  bridge").

## Skills (`internal/skills/fragments`)

- A shared paragraph, in each of clarify, specify, implement, adjust, handoff
  read-first: "Where the issue artefacts live": use `SECTILE_SPEC_REPO` /
  `SECTILE_SPEC_BRANCH` / `SECTILE_SPEC_WORKTREE` when set, else call
  `prepare_task_spec_worktree`; write and read `docs/clarifications/<n>.md` and
  the specification under that path, never relative to the session directory;
  never switch branch there; repeat the warning.
- clarify and specify steps: when the specifications path is not the task
  worktree, commit only the task's artefacts there (`git -C <path> add
  <files>`), push the branch with a plain push (`-u` the first time), never
  forced, never the default branch; a refused push is reported and does not
  block; on an empty branch (plain folder) nothing is committed or pushed.
  The #487 `git check-ignore` runs in that path.
- implement and adjust: read the specification there; the pull request
  description names the specifications repository and branch when distinct.
- handoff steps: when distinct, remove the specifications worktree and local
  branch only when the branch is merged into that repository's default branch
  (squash merges checked by content, as for the code branch); otherwise keep
  them and say so.
- pickup and pickup_issues inherit through the nested stages; their own text
  only mentions the second branch in the final report.
- Plugin goldens under `internal/skills/testdata/plugin` regenerated with
  `UPDATE_GOLDEN=1` outside the sandbox.

## Desktop (`desktop/src/main.js`)

- The current row becomes a `specFolderRow(kind)` builder used twice:
  "Macro specifications folder" (field `specPath`) and "Issue specifications
  folder" (field `issueSpecPath`), each with its "live in the code repository"
  checkbox, browse button, Git offer and kind line. Hints: "Macro skills read
  and write specifications in …" / "Issue skills write clarifications and
  specifications in …".
- Save sends both values.
- `desktop/tests/spec-folder.ui.cjs` extended to both rows;
  `attached-folders.ui.cjs` and `git-init.ui.cjs` labels updated.

## Docs

- New ADR `docs/adrs/0051-macro-and-issue-specifications-folders.md`,
  superseding ADR 0027 in part; ADR 0027 header gains the pointer.
- `docs/USER_GUIDE.md`, `desktop/README.md`, `docs/contracts/server-agent-v1.md`
  (new operation, `SECTILE_SPEC_*` on task runs).
- `CHANGELOG.md` under `[Unreleased]`: `Changed` (Specifications folder renamed
  Macro specifications folder) and `Added` (Issue specifications folder,
  `prepare_task_spec_worktree`).

## Rejected alternatives

- **Reusing `ensureLocalWorktree` for the Issue folder** (as
  `prepare_repository_worktree` does): it starts a new branch from `HEAD` and
  knows neither plain folders nor the fetch bound; the macro preparation
  already does what a specifications repository needs.
- **A second JSON key for the Macro folder** (`macroSpecPath`): it would need a
  settings migration for no user-visible benefit.
- **Listing both folders in a task folder map**: no task operation reads the
  Macro folder; listing it would invite skills to write there.

## Target files

`internal/agentconfig/{workstation.go,local.go}`,
`internal/agent/{agent_macro_dispatch.go,agent_macro_worktree.go,agent_config.go,agent.go,agent_desktop.go,agent_desktop_settings.go,agent_desktop_repositories.go,repositories.go,agent_operations.go}`,
`internal/agentprotocol/operations.go`, `internal/models` (new type),
`internal/db` (new method), `internal/taskmcp/server.go`,
`internal/agentmcp/mcp.go`, `internal/mcptest/contract.go`,
`internal/skills/fragments/*`, `internal/skills/testdata/plugin/*`,
`desktop/src/main.js`, `desktop/tests/*.ui.cjs`, docs listed above, and their
tests.

## Implementation notes (deviations from the plan above)

- The launch keeps `prepareDispatch` as it was: the specifications workspace is
  prepared right after it by `agentDaemon.taskSpecWorkspace`
  (`internal/agent/spec_folders.go`), which avoids widening a signature used by
  a dozen callers and tests.
- The native terminal opened on a task (`desktopTasksTerminalExternal`) creates
  no code worktree, so it creates no specifications worktree either: it names a
  distinct Issue folder only once the task's specifications worktree exists,
  and otherwise sets no `SECTILE_SPEC_*`, so a skill typed there calls
  `prepare_task_spec_worktree`.
- A project-level session (conversation, console) lists both folders in its
  folder map, Macro first, since it may work on either; a ticket lists the Issue
  folder only.
- The skill guidance is one contract fragment,
  `internal/skills/fragments/contracts/spec-workspace.md`, rendered as a
  "Specifications workspace" section in clarify, specify, implement, adjust,
  handoff and both pickups, instead of a paragraph repeated in each read-first.
  It also carries the pull request timing: with a distinct workspace no code
  pull request is opened before implementation. `create-pr` is left unchanged.
