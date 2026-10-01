# #575: Tasks

Spec: [`spec.md`](spec.md). Plan: [`plan.md`](plan.md). Work on `feat/575`.

## Phase 1: Agent contract

- [x] T001 Add `DiffDocument`, the `Document` field, `diffDocumentLimit` and
  `diffDocumentBudget` to `internal/runner/worktree_diff.go`, with an `isMarkdownPath`
  helper (`.md` / `.markdown`, case-insensitive).
- [x] T002 In `inspectWorktree`, after the file-list truncation loop and before returning,
  read the Markdown blobs with `cat-file --batch-check` then `cat-file --batch` through `snapshot.command`
  (`<tree>:<path>` for new side, `<ancestor>:<path>` for deleted), skip the call when no
  Markdown file is listed, and attach content or reason per the bounds of the plan.
- [x] T003 Write `TestWorktreeDiffMarkdownDocuments` (cases listed in the plan), plus a
  budget case asserting that every file keeps its patch. Run
  `go test ./internal/runner/ -run WorktreeDiff`.
- [x] T004 Advertise `markdown-documents` in `internal/agent/agent_desktop.go` and assert
  it in a status test. Run `go test ./internal/agent/ -run 'Desktop'`.

## Phase 2: Desktop main process

- [x] T005 `desktop/electron/main.cjs`: add `markdownDocuments` to the `git-diff` result;
  add the `open-link` handler (http, https without credentials, mailto only).
- [x] T006 `desktop/electron/preload.cjs`: expose `openLink`.

## Phase 3: Desktop renderer

- [x] T007 Add `markdown-it` ^14 to `desktop/package.json` dependencies and update
  `desktop/package-lock.json` (`npm install markdown-it@^14` in `desktop/`).
- [x] T008 Create `desktop/src/markdownView.mjs` with `markdownModel` and
  `renderMarkdown`, using no HTML-string sink.
- [x] T009 Write `desktop/tests/markdown-view.test.mjs` (cases listed in the plan) and run
  `node --test tests/markdown-view.test.mjs` in `desktop/`.
- [x] T010 `desktop/src/gitDiff.js`: toggle, note, rendered container, session-held
  `rendered` state and the four `showFile` branches of the plan.
- [x] T011 `desktop/src/style.css`: `.diff-rendered` and `.md-*` rules on existing tokens,
  checked in light and dark appearances.

## Phase 4: End-to-end checks

- [x] T012 Add `desktop/tests/markdown-view.ui.cjs`
  with the UI cases of the plan, including the hostile document and the stubbed
  `shell.openExternal`.
- [x] T013 `npx vite build` in `desktop/`, restore `webui/.gitkeep` if the build removed
  it, then `npm test` and `npm run test:ui` in `desktop/` (sandbox off).
- [x] T014 `go test ./internal/runner/ ./internal/agent/` and `go vet ./...`.

## Phase 5: Documentation and handoff

- [x] T015 Add the `Added` line of the plan under `## [Unreleased]` in `CHANGELOG.md`.
- [x] T016 Walk the acceptance criteria AC-001 to AC-005 of the spec and tick each one in
  the implementation report.

## Test plan summary

| Requirement | Covered by |
| --- | --- |
| FR-001, FR-002, FR-003 | T012 |
| FR-004, FR-005, AC-003 | T003, T012 |
| FR-006, FR-007, FR-009, FR-010 | T009, T012 |
| FR-008, AC-002 | T005, T012 |
| FR-011 | T012 |
| FR-012, FR-013, AC-004 | T003, T012 |
| FR-014 | T004, T012 |
| FR-015 | T011, T012 |
| AC-005 | T015 |
