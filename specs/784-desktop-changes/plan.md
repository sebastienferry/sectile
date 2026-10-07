# Plan #784 - Desktop Changes

## Stack

- Local agent, Go: `internal/agent`, `internal/runner`.
- Sectile Desktop: Electron main process `desktop/electron`, renderer in plain
  JavaScript `desktop/src`.
- No server, database, tracker or MCP change. The new Git calls go through the
  existing `diffGit.command`, which already applies `agentexec.Hidden`.

## Data contract

`GET /desktop/status` capabilities gain `folder-selection`.

`GET /desktop/git-diff?id=<runId>&folder=<path>`:

- `folder` omitted, empty or equal to the run's directory: unchanged.
- `folder` listed in the run's `folders`: inspected as that folder; the result
  has the same shape, its `directory` and `branch` naming the folder.
- otherwise: 404 `{error: {code: "folder_not_found", message}}`.
- new inspection errors (409): `folder_unavailable`, `not_a_repository`,
  `not_repository_root`, `detached_head`.

`POST /desktop/open-editor` with `{runId, folder}`: same resolution; an
unlisted folder answers 404, a missing one 410.

## Design decisions

- **D1 - The agent resolves the folder.** `runFolderPath(run, folder)` in
  `internal/agent/agent_run_folder_list.go` returns the run's directory for an
  empty folder or the directory itself, the listed path for a listed folder,
  and refuses anything else. The renderer still never names an arbitrary path.
  Called under the queue lock, like the existing reads of `run.desktop`.
- **D2 - Inspecting a folder.** `runner.InspectFolder(ctx, directory)` checks
  that the folder exists, is a Git repository's top level and is on a branch,
  then calls `InspectWorktree(ctx, directory, branch, directory)`: the identity
  check then compares the folder with itself, and the baseline is its own
  default branch. A linked worktree of another repository passes, since its
  common directory is its own.
- **D3 - One capability.** `folder-selection` announces both routes. Without it
  the desktop keeps #762's behavior.
- **D4 - Electron.** `git-diff` and `open-editor` handlers take an optional
  folder, a string of at most 4096 characters, and require `folder-selection`
  when one is given. `preload.cjs` forwards it.
- **D5 - Renderer state.** `main.js` keeps `folderSelections`, a `Map` from run
  ID to selected path. `chosenFolder(run, path)` in `folder-menu.mjs` returns
  the listed folder that path names, or null for the primary. The displayed
  path is `chosenFolder(...)?.path || run.directory`. `renderFolders`, already
  called on every refresh of the header, drops a selection that left the list
  and re-applies the displayed path and the **Changes** folder.
- **D6 - Menu.** With `folder-selection`, items are `role=menuitemradio` with
  `aria-checked`, and choosing one selects it; without it, items stay
  `menuitem` and copy, as in #762.
- **D7 - Changes panel.** `createGitDiff` gains a folder: `select(id, folder)`
  and `setFolder(folder)`. A folder change bumps the generation, clears the
  result and reloads when the view is open; `refresh` passes the folder to
  `api.gitDiff` only when set.

## Rejected alternatives

- **The renderer sends the path and the agent trusts it.** It would let the
  renderer point Git and the editor at any folder; the run's own list is the
  allow-list.
- **Inspecting the repository top level of a sub-folder.** The view would then
  name a folder the person did not select; the folder is refused instead.
- **A selector inside the Changes panel.** Rejected by the owner in round 2.

## Target files

- `internal/runner/worktree_diff.go`, `internal/runner/worktree_diff_test.go`
- `internal/agent/agent_run_folder_list.go`, `agent_diff.go`,
  `agent_desktop_editor.go`, `agent_desktop.go` and their tests
- `desktop/electron/main.cjs`, `desktop/electron/preload.cjs`
- `desktop/src/folder-menu.mjs`, `desktop/src/gitDiff.js`, `desktop/src/main.js`
- `desktop/tests/folder-menu.test.mjs`, a new
  `desktop/tests/folder-selection.ui.cjs`
- `docs/contracts/server-agent-v1.md`, `desktop/README.md`, `CHANGELOG.md`
