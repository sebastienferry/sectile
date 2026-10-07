# Tasks #784 - Desktop Changes

Order matters: each step leaves the tree building and green.

## 1. Runner: inspect a folder (US2)

- [x] T1.1 `InspectFolder` in `internal/runner/worktree_diff.go` (D2).
- Tests: a context checkout on its default branch shows its uncommitted
  change; a linked worktree of another repository is inspected on its branch;
  a missing folder, a plain folder, a sub-folder and a detached HEAD are each
  refused with their own code.

## 2. Agent: routes and capability (US1, US2, US4)

- [x] T2.1 `runFolderPath` and the `folder-selection` capability (D1, D3).
- [x] T2.2 `/desktop/git-diff` accepts `folder` (FR2, FR3).
- [x] T2.3 `/desktop/open-editor` accepts `folder` (FR2, FR3).
- Tests: no folder and the run's directory inspect the primary; a listed folder
  is inspected or opened; an unlisted folder is refused and opens nothing.

## 3. Desktop (US1 to US4)

- [x] T3.1 Electron handlers and preload forward the folder (D4).
- [x] T3.2 `chosenFolder` in `folder-menu.mjs` (D5).
- [x] T3.3 `gitDiff.js` folder state (D7).
- [x] T3.4 `main.js`: selection map, menu radio items, path, editor and
      Changes wiring, fallback (D5, D6).
- Tests: unit tests of `chosenFolder`; a UI test against a fake agent:
  selection changes the path, the copy, the editor request and the diff
  request; the selection survives switching executions; it falls back when the
  folder leaves the list; the existing #762 test keeps covering an agent
  without the capability.

## 4. Documentation

- [x] T4.1 `docs/contracts/server-agent-v1.md`: `folder-selection` and the
      `folder` parameter.
- [x] T4.2 `desktop/README.md`: the chevron selects; Changes and the editor
      follow.
- [x] T4.3 `CHANGELOG.md`: one `Changed` line under `[Unreleased]`.
