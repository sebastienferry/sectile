# Tasks #689 - Add a folder from a free console or a native-terminal discussion

Ordered; each task lists its tests. Specification: `spec.md`; design:
`plan.md`.

## 1. Agent (US1, US2, FR1 to FR5)

- [x] T1 `desktopConsole` records `interactiveProvider` on a PTY console.
  - Test (`agent_console_test.go` or `agent_run_folders_test.go`): the
    provider recorded for a built-in claude, codex and an engine whose
    provider is Claude.
- [x] T2 `desktopRunFolder` admits a running console with a session and types
  into a detached run; new refusal text; new capability reported.
  - Test (`agent_run_folders_test.go`): a Claude console has `/add-dir` typed
    (`now`); a Codex console is attached only (`next-launch`); a detached
    Claude discussion has `/add-dir` typed; a console without a session or
    ended is refused; `run-folders-terminals` is on `/desktop/status`.

## 2. Desktop (US1 to US3, FR6, FR7)

- [x] T3 `offersRunFolder` and `runFolderOutcome` in `run-folders.mjs`.
  - Test (`desktop/tests/run-folders.test.mjs`): console and detached runs
    offered only with the new capability; headless and ended never; the
    console and detached wordings; #676 wordings unchanged.
- [x] T4 `main.js` wiring: capability, arguments, button title.
  - Test (`desktop/tests/run-folders.ui.cjs`): a running console with the new
    capability shows the button and the console wording.

## 3. Documentation (FR8)

- [x] T5 Contract, user guide, changelog.

## 4. Checks

- [x] T6 `go build ./...`, `go vet ./internal/agent/...`,
  `go test ./internal/agent/...`, `node --test desktop/tests/run-folders.test.mjs`,
  the desktop UI test after `npx vite build`.
