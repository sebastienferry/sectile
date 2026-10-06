# Tasks #762 - Desktop: Folders

Order matters: each step leaves the tree building and green.

## 1. Agent: the list of a run (US1, US2, US3)

- [x] T1.1 `runFolder` and `runFolders` in
      `internal/agent/agent_run_folder_list.go`; `desktopRun.Folders` (D1).
- [x] T1.2 Set the list at every launch that computes a folder map: ticket
      runs (conversation, interactive, headless), the Desktop discussion, the
      free console and the project conversation (D2).
- [x] T1.3 Refresh a conversation's list at each turn (D3).
- Tests: primary first from the run directory; worktree preferred over folder;
  unmapped and missing folders left out; duplicates dropped; names and roles;
  a lazy-code map without a primary entry still lists the directory first.

## 2. Agent: additions and persistence (US4, US5)

- [x] T2.1 `addRunFolder`; `recordTaskFolder`, called beside `addDirToTaskRuns`, records the prepared worktree
      on every run of the task that has not ended (D4).
- [x] T2.2 `desktopRunFolder` records the attached folder on its run (D4).
- [x] T2.3 `runSave` carries the folder count (D5).
- Tests: a prepared worktree reaches the task's runs, not another task's nor an
  ended one, and is not listed twice; an attached folder reaches its run; a
  stored run restores its list; a list change alone triggers a write.

## 3. Desktop (US1, US3, US6)

- [x] T3.1 `desktop/src/folder-menu.mjs` with `menuFolders` and
      `folderRoleLabel` (D7).
- [x] T3.2 Chevron, menu, keyboard and dismissal in `main.js`; styles in
      `style.css` (D6).
- Tests: unit tests of the helpers; a UI test against the fake agent: no
  chevron with one folder or no field; the chevron opens the menu; an item
  copies its path and shows **Copied**; Escape returns focus to the chevron.

## 4. Documentation

- [x] T4.1 `docs/contracts/server-agent-v1.md`: the `folders` field.
- [x] T4.2 `desktop/README.md`: the chevron paragraph after the path control.
- [x] T4.3 `CHANGELOG.md`: one `Added` line under `[Unreleased]`.
