# Plan #484 - Attached folders, and one kind of project

Specification: `specs/484-mono-multi-repo/spec.md`.
Clarification: `docs/clarifications/484.md`.

## Stack and surfaces

| Surface | Language | What changes |
| --- | --- | --- |
| Server (`internal/db`, `internal/handlers`, `internal/taskmcp`) | Go | flag removal, migration, changed repositories unfiltered, relay of attached identities, repository wait removed |
| Local agent (`internal/agent`, `internal/agentconfig`) | Go | attached folders in settings, folder map, CLI directories, worktree and removal in attached folders, primary resolution, spec folder default |
| Desktop (`desktop/src/main.js`) | JS | *Attached folders* row, layout row and console picker removed, spec row simplified |
| Web (`web/src`) | TS/React | *Mono-repo* checkbox removed, repositories editor and pin always available, repository wait wording removed |
| Docs | Markdown | ADR 0033, ADR 0027/0028 headers, README, `docs/ARCHITECTURE.md`, `docs/CAPABILITIES.md`, CHANGELOG |

No new dependency.

## Verified external fact

Codex CLI declares `--add-dir <DIR>` in its shared options
(`codex-rs/utils/cli/src/shared_options.rs` on `openai/codex` main, read on
2026-09-26): `#[arg(long = "add-dir", value_name = "DIR")] pub add_dir:
Vec<PathBuf>`, help "Additional directories that should be writable alongside
the primary workspace". It is repeatable, takes one value per occurrence, and
applies to both `codex` and `codex exec`. clap accepts the `--add-dir=<DIR>`
spelling. Codex makes these directories *writable* in its sandbox: a context
folder stays read-only by instruction only, as the specification says.

## Architecture

### 1. Workstation data (agent, `internal/agentconfig`)

- `ProjectSettings` (`internal/agentconfig/workstation.go`) gains
  `Folders []string \`json:"folders,omitempty"\``: absolute, cleaned paths, in
  the order they were added. Nothing else is stored: the kind, the remote and
  the identity are read from the folder each time, so a remote changed after
  attaching is followed (spec, edge cases).
- Known trap (#443, memory "new Overrides key needs a merge"):
  `localProjectRoot` copies known keys only, and `WriteSettings` must write an
  emptied list as absent. Add `Folders` to the project-section merge in
  `overlay` / `overlayProject` and make sure removing the last folder removes
  the key.
- A new helper in `internal/agent` (new file `attached.go`):

  ```go
  type attachedFolder struct {
      Path     string // as stored
      Kind     string // "git", "folder", "missing"
      Remote   string // origin URL, "" when none
      Identity string // models.RepositoryIdentity(Remote)
  }
  func attachedFolders(ctx context.Context, overrides agentconfig.Settings, projectID string) []attachedFolder
  ```

  It reads `overrides.ProjectSettings[projectID].Folders`, stats each path,
  runs `git rev-parse --show-toplevel` and `git remote get-url origin` through
  `gitLocal`. A Git folder is described at its top level.

### 2. One lookup for "where is repository X on this workstation"

`repositoryRoot(overrides, projectRoot, code, identity)` answers for project
repositories only. Add:

```go
// repositoryFolder is the folder holding identity's checkout here: its
// mapping, the project root for the code repository, else an attached Git
// folder whose origin is identity.
func repositoryFolder(ctx context.Context, config agentconfig.Config, overrides agentconfig.Settings, projectRoot, identity string) (root string, attached bool, ok bool)
```

Mapping first, attached second (spec edge case: the mapping wins). Use it in
place of `repositoryRoot` in:

- `repositoryWorktree` (`internal/agent/repositories.go:342`);
- `removeRepositoryWorktrees` (`repositories.go:369`);
- the `git_evidence` branch of `agent_operations.go:338`, so the head of an
  attached repository's pull request is read from that folder;
- `specexclude.go:246` is left on `repositoryRoot` unless the implementer
  finds it must see attached folders (it excludes spec artefacts in project
  repositories).

`excludeTaskWorktrees(ctx, root)` is applied to an attached folder before a
worktree is created in it, as for any repository other than the code one.

### 3. Primary repository resolution (models + agent)

- `models.ResolvePrimaryRepository(pinned, repositories, mapped)` loses its
  `monoRepo` parameter and its ambiguous branch: a pin to a listed repository
  gives `PrimaryResolved` or `PrimaryUnmapped`; anything else gives
  `PrimaryDefault`. `PrimaryAmbiguous` and the returned `pin` flag go away,
  with every caller (`primaryRoot`, `specexclude.go`, `agent_operations.go`
  around line 150).
- `primaryRoot` returns `(root, identity string, err error)`.
- Removed: `errRepositoryAmbiguous`, `repositoryPollInterval`,
  `awaitRepository` (`repositories.go`), the retry loop at `agent.go:1045`, the
  `errors.Is(err, errRepositoryAmbiguous)` exception in
  `agent_operations.go:153`.

### 4. Folder map

- `models.FolderMapEntry` gains `Kind string \`json:"kind,omitempty"\``
  (`git`, `folder`, `missing`) and `Attached bool \`json:"attached,omitempty"\``.
  Existing fields keep their meaning, so a consumer of `SECTILE_REPOSITORIES`
  that ignores the new keys is unaffected.
- New role `models.FolderRoleLocal = "local"`: a folder without a remote, which
  may be changed in place.
- `buildFolderMap` appends, after the project repositories and before the
  specifications folder, one entry per attached folder not already in `seen`:
  - with an identity: role `context`, or `changed` with its worktree when the
    identity is in `task.ChangedRepositories`; an identity equal to a project
    repository is skipped (that repository is already listed with its
    mapping);
  - without an identity: role `local`, `Kind` `git` or `folder`;
  - missing: role `context` when a remote cannot be read, `Kind` `missing`,
    `Path` the stored path.
- `folderMapDirs` gives `local` entries' paths, and skips every entry whose
  `Kind` is `missing`.
- `folderMapPrompt`: an entry without identity is named by its path (`spec`
  keeps "specifications"); a missing one reads "not found on this
  workstation"; the kind is added in the parenthesis for attached entries
  (`context, attached Git repository`, `local, plain folder`). The closing
  instruction becomes:

  > Work in the primary worktree. Context folders are read-only: to change
  > one, call prepare_repository_worktree for its repository first and work in
  > the worktree it returns. Local folders have no remote: change them in
  > place, with no worktree and no pull request. Each changed repository needs
  > its own pull request, given to transition_stage in prUrls.

  The `len(entries) < 2` threshold is kept.

### 5. CLI directories

`addDirArgs(provider, dirs)` (`internal/agent/agent_config.go:505`) emits, for
`codex`, one `--add-dir=<quoted path>` per folder, the same form it uses for
Claude when a template places it. Update its comment ("the only attested flag
is Claude Code's") and the `{addDirs}` placeholder documentation in the README.
Other providers still get nothing.

### 6. Spec folder default

- `localSpecRepo(overrides, projectID, root)` loses `monoRepo`: no setting
  means `root`. Remove `errNoSpecFolder`.
- `desktopProject` (`agent_desktop.go:716`): `specDefault = root` whenever the
  mapping resolved; drop `"monoRepo"` from the answer; `specFolderKind` no
  longer returns `unset` for a configured project.
- `buildFolderMap` calls the new `localSpecRepo`.

### 7. Desktop settings endpoint for attached folders

`/desktop/repositories` keeps its array answer, which the running desktop
reads. A sibling route `/desktop/folders`, handled in
`agent_desktop_repositories.go`, serves the attached folders:

- `GET /desktop/folders?projectId=` returns
  `[{path, kind, remote, identity, duplicate}]`, `duplicate` naming the
  project repository whose mapping wins over this entry (spec, edge cases).
- `POST /desktop/folders` with `{projectId, path}` adds one folder, under
  `d.prepareMu` and `agentconfig.LockSettings()`, applying FR-003/FR-004:
  - not absolute, or not a directory: 400;
  - equal (after `filepath.Clean` and Git top level) to the project root, the
    specifications folder, a mapped repository folder or an attached folder:
    409 naming it;
  - Git with an identity equal to the code repository: 409 ("this is the
    project's local repository");
  - Git with an identity of another project repository: stored through the
    existing `mapRepository`, answer `{"mappedAs": identity}`;
  - Git with an identity already attached: 409 naming that folder;
  - else appended to `Folders`.
- `DELETE /desktop/folders?projectId=&path=` removes one entry (no existence
  check, so a missing folder can be removed).

The desktop (`desktop/src/main.js`, preload and IPC bridge) gets matching
`api.folders(id)`, `api.attachFolder(id, path)`, `api.detachFolder(id, path)`.

### 8. Server

- **Model**: remove `Project.MonoRepo` (`internal/models/models.go:161`) and
  the `MonoRepo *bool` fields of the create/update request structs (lines 413
  and 457). JSON decoding of project requests ignores unknown keys, so an old
  client sending `monoRepo` is accepted (FR-014); verify this for both
  handlers and add a test.
- **Storage**: remove `mono_repo` from every read and write in
  `internal/db/db.go` (lines ~6158, 6276, 6386, 6597, 6656). Do not edit the
  frozen baseline `CREATE TABLE` (memory "new columns only in migrations").
- **Migration 30** (`internal/db/migrations.go`), SQLite and PostgreSQL:
  1. `ALTER TABLE projects DROP COLUMN mono_repo;`
  2. `UPDATE task_activities SET waiting_since = NULL, waiting_session = '', waiting_reason = '' WHERE waiting_reason = 'repository';`

  Check the SQLite version path used for the previous `DROP COLUMN`
  (migration 22, `projects.github_token`) and do the same. Teach the replay
  fixtures to undo migration 30 (re-add `mono_repo`) in `forgetSchemaVersion`
  and in the `DELETE FROM schema_migrations WHERE version >= N` fixtures,
  following `undoServerCredentialsMigration` (memory "migration dropping a
  column breaks replay fixtures"); run the PostgreSQL suite.
- **Agent configuration**: remove `MonoRepo` and `IsMonoRepo()` from
  `internal/agentconfig/config.go` and its fill in `internal/db/agentconfig.go:49`.
  Not sending the field makes an older agent read the default (mono-repo),
  which never parks a launch; it will refuse `prepare_repository_worktree`
  until updated, which the release note says.
- **`internal/db/repositories.go`**:
  - `taskPin`: no mono-repo condition.
  - `taskChangedRepositories`: no mono-repo condition and no filter on the
    project's repositories; primary and duplicates still left out.
  - `PrepareRepositoryWorktree`: remove the mono-repo refusal. Resolve the
    target as the project repository when listed, else as
    `models.RepositoryIdentity(repository)`; refuse an empty identity (a path
    or a bare name) with "give the repository's remote URL or host/path".
    Relay to the caller's agent unchanged; keep the echo check and the
    primary-repository refusal; record the identity only after the agent's
    answer.
  - Remove `MarkRunAwaitingRepository`.
- **`internal/db/adjustment.go:225`**: drop the mono-repo refusal of a foreign
  pull request.
- **`internal/db/stageprs.go`**: no code change expected beyond what
  `taskChangedRepositories` now returns; `multiRepoTask`, `validateStagePRs`,
  `checkSecondaryPRs` and `RemoveTaskWorktree` (`db.go:2204`) follow. Verify
  with tests on an identity outside the project list.
- **Handlers**: remove the `/api/activities/{id}/awaiting-repository` route
  (`internal/handlers/handlers.go:3310`). An older agent posting to it gets a
  404, which it already logs and ignores.
- **MCP** (`internal/taskmcp/server.go`):
  - `prepare_repository_worktree` description, replacing "On a multi-repo
    project, … another of the project's repositories": prepare the task's
    worktree in another repository (one of the project's repositories or a
    Git folder attached to the project on the caller's workstation, given by
    its remote URL or `host/path`) on the task's branch; call it before
    changing a context folder; a local folder (no remote) is changed in place
    without it; the repository then needs its own pull request in `prUrls`.
    Update the `repository` property description the same way.
  - `prepare_macro_worktree` description: "else the code checkout" without
    "of a mono-repo project".
  - The MCP contract fixture (`internal/mcptest/contract.go`) keeps working.

### 9. Web

- `web/src/components/ProjectModal.tsx`: remove the `monoRepo` state, the
  checkbox (line ~691) and the condition on the repositories editor (line
  ~702); stop sending `monoRepo`.
- `web/src/components/TaskDetailModal.tsx:214`:
  `canPinRepository = projectRepositories.length > 1`.
- `web/src/types/index.ts`: remove `monoRepo`.
- `ActivitiesView.tsx:122`, `RemoteRunBadge.tsx:75`: remove the
  `waitingReason === 'repository'` wording and its label constant; keep the
  generic `waitingReason` field.

### 10. Desktop

- Project settings (`main.js` ~1595-1675, ~2007-2032):
  - remove the *Repository layout* row and `renderServer`'s layout line;
  - `renderSpec`: `mono` is always true; the required state and its hint go;
  - *Other repositories*: loaded for every project, shown when the list has
    entries other than the code repository;
  - new *Attached folders* row (stacked), after *Other repositories*: one line
    per folder with its path, a kind label (`Git repository · <remote>`,
    `Git repository, no remote`, `Folder, not a Git repository`,
    `Folder not found`, `Duplicate of <identity>'s folder`), and a *Remove*
    button; an *Add folder…* button using `api.chooseRepository()`. Add and
    remove call the agent at once (like the folder mapping), and an answer
    `mappedAs` refreshes *Other repositories*. Errors are shown with the
    existing `error()` helper, in French where the surrounding messages are.
- Console (`main.js` ~234-300): remove the "Waiting for a repository" group,
  the pin picker and its status line.
- Tests: delete `desktop/tests/repository-choice.ui.cjs`; update
  `console.ui.cjs`, `git-init.ui.cjs`, `spec-artifacts.ui.cjs`,
  `spec-folder.ui.cjs` (their `monoRepo` fixtures); add
  `desktop/tests/attached-folders.ui.cjs`. The UI tests load
  `dist/index.html`: build with `npx vite build` first, then restore
  `webui/.gitkeep` (memories "desktop UI tests need a build", "vite build
  deletes webui .gitkeep").

### 11. Documentation

- `docs/adrs/0033-attached-folders-and-one-kind-of-project.md`: context
  (#456 flag, #484 request, plugin constraint), decision (attached folders
  local only; flag removed; primary = pin else code; changed repositories
  recorded by identity on the ticket and unfiltered; instructions in the
  prompt block and the MCP description; Codex `--add-dir`), consequences
  (older agents, plugin wording), rejected alternatives (adding the remote to
  the project's repositories, round 3; keeping the flag; server-side list of
  folders; instructions in the skills).
- ADR 0027 and ADR 0028: add "Superseded in part by ADR 0033" under their
  status, naming the sections (spec folder required on multi-repo; flag and
  ambiguity wait).
- `README.md` (lines ~626, ~1303 and the `{addDirs}` placeholder),
  `docs/ARCHITECTURE.md:222`, `docs/CAPABILITIES.md:27`: rewrite without the
  mono/multi-repo distinction; document the attached folders, the `local`
  role and the new `kind`/`attached` keys of `SECTILE_REPOSITORIES`.
- `CHANGELOG.md` `[Unreleased]`:
  - Added: attached folders in the desktop project settings, handed to every
    execution, writable through a worktree and a pull request, or in place
    without a remote (#484).
  - Added: Codex receives the task's other folders as `--add-dir` (#484).
  - Changed: the specifications folder defaults to the code checkout on every
    project (#484).
  - Removed: the *Mono-repo* project setting; a ticket runs in the code
    repository unless pinned, and no execution waits for a repository choice
    any more (#484).

## Data contracts

`~/.config/sectile/settings.json`:

```json
{
  "projectSettings": {
    "<projectId>": {
      "path": "/Users/me/src/app",
      "folders": ["/Users/me/src/acme-ui", "/Users/me/notes/acme"]
    }
  },
  "repositories": { "github.com/acme/lib": "/Users/me/src/lib" }
}
```

`SECTILE_REPOSITORIES` entry (new keys `kind`, `attached`; new role `local`):

```json
[
  {"remote": "git@github.com:acme/app.git", "identity": "github.com/acme/app", "role": "primary", "path": "/Users/me/src/app", "worktree": "/Users/me/src/app/.tasks/worktrees/#12"},
  {"remote": "git@github.com:acme/ui.git", "identity": "github.com/acme/ui", "role": "context", "path": "/Users/me/src/acme-ui", "kind": "git", "attached": true},
  {"remote": "", "identity": "", "role": "local", "path": "/Users/me/notes/acme", "kind": "folder", "attached": true}
]
```

Agent desktop API:

| Method | Path | Body / query | Answer |
| --- | --- | --- | --- |
| GET | `/desktop/folders` | `projectId` | `[{path, kind, remote, identity, duplicate}]` |
| POST | `/desktop/folders` | `{projectId, path}` | 204, or `{mappedAs}`; 400/409 with a message |
| DELETE | `/desktop/folders` | `projectId`, `path` | 204 |

MCP `prepare_repository_worktree` input unchanged (`taskKey`, `repository`);
`repository` may now name an attached folder's remote.

## Target files

- `internal/agentconfig/workstation.go`, `internal/agentconfig/local.go`,
  `internal/agentconfig/config.go`
- `internal/agent/attached.go` (new), `internal/agent/repositories.go`,
  `internal/agent/agent.go`, `internal/agent/agent_operations.go`,
  `internal/agent/agent_config.go`, `internal/agent/agent_macro_dispatch.go`,
  `internal/agent/agent_desktop.go`, `internal/agent/agent_desktop_repositories.go`,
  `internal/agent/specexclude.go`
- `internal/models/models.go`, `internal/models/repository.go`
- `internal/db/db.go`, `internal/db/migrations.go`, `internal/db/repositories.go`,
  `internal/db/adjustment.go`, `internal/db/agentconfig.go`
- `internal/handlers/handlers.go`, `internal/taskmcp/server.go`
- `web/src/components/ProjectModal.tsx`, `TaskDetailModal.tsx`,
  `ActivitiesView.tsx`, `RemoteRunBadge.tsx`, `web/src/types/index.ts`,
  `web/src/lib/remoteRunIndicator.ts` (only if the label constant lives there)
- `desktop/src/main.js`, the desktop preload/IPC file that declares
  `api.repositories`
- tests next to each, `desktop/tests/*.ui.cjs`
- `docs/adrs/0033-…`, `docs/adrs/0027-…`, `docs/adrs/0028-…`, `README.md`,
  `docs/ARCHITECTURE.md`, `docs/CAPABILITIES.md`, `CHANGELOG.md`

## Risks

- **Older agents after the server upgrade** read every project as mono-repo:
  launches keep working in the code repository; `prepare_repository_worktree`
  is refused by them. Release note: update the local agent.
- **Replay fixtures and PostgreSQL**: the dropped column fails only under
  PostgreSQL; run that suite (memory "PostgreSQL tests need a DSN").
- **`internal/db` near the CI timeout**: keep new db tests small (memory
  "internal/db near go test timeout").
- **Codex writes in context folders**: `--add-dir` makes them writable in its
  sandbox; the prompt keeps them read-only by instruction.
- **Large file edits**: `desktop/src/main.js` is edited in small hunks
  (memory "subagents stall on large Sectile files").
