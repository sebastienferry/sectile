# Plan #762 - Desktop: Folders

## Stack

- Local agent, Go: `internal/agent`.
- Sectile Desktop, Electron renderer in plain JavaScript: `desktop/src`.
- No server, database, tracker or MCP change. No child process is started, so
  the `agentexec.Hidden` rule does not apply.

## Data contract

`GET /desktop/runs` entries gain an optional field:

```json
"folders": [
  {"path": "/work/sectile/.tasks/worktrees/issue-762", "name": "sectile", "role": "primary"},
  {"path": "/work/argocd-sp", "name": "argocd-sp", "role": "context"},
  {"path": "/work/notes", "name": "notes", "role": "local", "attached": true}
]
```

- `path`: absolute folder to copy.
- `name`: last segment of the repository identity; `specifications` for the
  specifications folder; the folder's base name otherwise.
- `role`: `primary`, `changed`, `context`, `spec` or `local`, as in
  `SECTILE_REPOSITORIES`.
- `attached`: true for a folder attached to the project on this workstation.
- Omitted when the agent has no list for the run (older agent, macro run,
  queued run not yet launched). No capability string: the field is the signal.

The field is part of `desktopRun`, so `storedRun` carries it to the run store
with no format change: an older stored run reads with no list.

## Design decisions

- **D1 - One conversion, in the agent.** `runFolders(directory string,
  entries []models.FolderMapEntry) []runFolder` in a new
  `internal/agent/agent_run_folder_list.go` turns a folder map into the list:
  the run's directory first as `primary` (named after the primary entry when
  the map has one, else after its base name), then each entry's worktree, or
  its folder when it has none, skipping empty paths, missing attached folders
  (`Kind == folderKindMissing`) and folders already listed (compared cleaned).
  The renderer only draws it, so the naming rules live in one place, next to
  `folderMapPrompt`, which names the same folders for the engine.
- **D2 - Set at every launch that computes a folder map.** Ticket runs in
  `agent.go` (conversation, interactive and headless, which receives the list
  through `headlessRun`), the Desktop discussion launch in `agent_desktop.go`,
  the free console and the project conversation in `agent_console.go`. A macro
  run has no folder map and keeps none.
- **D3 - Conversations refresh per turn.** `conversationFolders` returns the
  folder map rather than its directories; `conversationTurn` derives the
  directories and stores `runFolders` of the map on the run.
- **D4 - Additions.** `addRunFolder(run, folder)` appends a folder unless its
  path is listed already. `recordTaskFolder`, called beside `addDirToTaskRuns`
  when a worktree is prepared, appends a `changed` folder named after the
  repository to every run of the task that has not ended (typing stays
  restricted to live Claude sessions).
  `desktopRunFolder` appends the attached folder to its run, as `context` when
  it became a repository's folder (`mappedAs`), else as `local`, attached.
- **D5 - Persistence.** `runSave` gains the folder count, so a run whose list
  grew is written at the next flush even when its console is quiet.
- **D6 - Desktop.** A chevron button `#worktree-folders` follows `#worktree`
  in the toolbar, hidden unless the selected run has more than one folder. It
  opens `#worktree-folders-menu` (`role=menu`), built from the run's list each
  time it opens. Items are `role=menuitem` buttons showing name, role label and
  path; choosing one copies the path through `api.copyText` and shows the
  existing **Copied** notice. Keyboard and dismissal follow the ticket menu
  pattern (arrows, Home/End, Escape back to the chevron, Tab and outside
  pointer close). Selecting another run, or a refresh that hides the chevron,
  closes the menu.
- **D7 - Pure helpers.** `desktop/src/folder-menu.mjs` exports
  `menuFolders(run)` (the list to show, `[]` under two entries or with a
  malformed field) and `folderRoleLabel(folder)` (`attached` wins, then
  `primary`, `changed`, `context`, `specifications`), unit-tested with
  `node --test`.

## Rejected alternatives

- **Recomputing the folder map on each `/desktop/runs` poll.** It asks the
  server for the project configuration and runs Git per repository every few
  seconds per run; the launch already computes it.
- **Sending `models.FolderMapEntry` as is.** The renderer would have to repeat
  the worktree-or-folder choice, the missing-folder rule and the naming, which
  then could drift from the agent's prompt.
- **A capability string.** The field's presence already says the agent knows
  the list; an empty list and an older agent must look the same anyway.

## Target files

- `internal/agent/agent_run_folder_list.go` (new) and its test.
- `internal/agent/agent_desktop.go`: `desktopRun.Folders`, the Desktop
  discussion launch.
- `internal/agent/agent.go`, `agent_headless.go`, `agent_console.go`,
  `agent_conversation.go`: set or refresh the list.
- `internal/agent/agent_run_folders.go`: additions.
- `internal/agent/run_store.go`: `runSave`.
- `desktop/src/folder-menu.mjs` (new), `desktop/src/main.js`,
  `desktop/src/style.css`.
- `desktop/tests/folder-menu.test.mjs` (new), a UI test against the fake
  agent.
- `docs/contracts/server-agent-v1.md`, `desktop/README.md`, `CHANGELOG.md`.
