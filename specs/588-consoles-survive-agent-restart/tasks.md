# Tasks #588 - Consoles survive an agent restart

Ordered checklist. Each group is one commit (Conventional Commits) and leaves
the tree buildable. No migration, no server change.

## 1. The store (FR1, FR2, FR10, FR11, US5)

- [x] T1.1 `internal/terminal`: export `HistoryLimit = 65536`, used for
  `maxHistBytes`.
- [x] T1.2 `runStore`, `storedRun`, `save`, `remove`, `load`, `prune`,
  safe-id check, in `internal/agent/run_store.go`.
- [x] T1.3 Tests: round trip; directory `0700`, file `0600` (Unix); no temp
  file left after save; unsafe ids refused on save and skipped on load;
  corrupt and unknown-version files skipped and kept; `running` loaded as
  `canceled`; `queued`/`preparing` dropped with their file; `prune` keeps the
  100 most recent finished runs and never a live one; disabled store (`dir ""`)
  is a no-op.

## 2. Capture and write (FR3, FR4, FR5, US3)

- [x] T2.1 `consoleTap` and `controlledRun.console`; `d.tapConsole` at every
  place a run is given its session (skill run, macro run, free console,
  discussion).
- [x] T2.2 `runTrace.snapshot` and its dirty counter.
- [x] T2.3 `d.trackRun`: a watcher per run that persists and prunes outside the
  queue lock once `exited` closes (replaces the planned `markExited`, see
  plan).
- [x] T2.4 `d.persistRun`, `d.persistLoop` (5 s), and the save-all `defer` in
  `Run` ahead of the session-closing one.
- [x] T2.5 Tests: a tap keeps its bytes after `CloseSession`; a tap is bounded
  to `HistoryLimit`; a finished run is saved at once with `FinishedAt`; the
  loop saves a dirty live run and skips a clean one.

## 3. Restore and serve (FR6, FR7, FR8, FR9, FR12, US1, US2, US4)

- [x] T3.1 `desktopRun.Restored`; `d.restoreRuns` called in `Run` before the
  loopback server starts.
- [x] T3.2 `/desktop/history` removes the files; the failed-start path too.
- [x] T3.3 Tests: a restored run of each kind is listed on `/desktop/runs`
  with its fields, `restored: true`, no session, no waiting mark; the terminal
  route replays the stored console bytes and the stored trace, then closes;
  input sent on it is discarded; clearing deletes the files of finished runs
  and keeps live ones; a stop on a restored run answers as for a finished run;
  a restored id cannot be registered again.

## 4. Desktop (FR13)

- [x] T4.1 `run-console.mjs`: `restored` runs attach read-only, no notice.
- [x] T4.2 `desktop/tests/run-console.test.cjs`: restored interactive and
  headless runs; a run without `restored` keeps today's answers.
- [x] T4.3 Restart and stop confirmation (`electron/main.cjs`) no longer says
  the consoles are cleared when the agent advertises `run-store`.

## 5. Documentation (FR14)

- [x] T5.1 `docs/adrs/0040-consoles-survive-an-agent-restart.md`; "Amended by
  ADR 0040" in ADR 0003 and the trace ADR 0016.
- [x] T5.2 `CHANGELOG.md`, `[Unreleased]` / `Added`: the desktop keeps its
  consoles across an agent restart.
- [x] T5.3 `desktop/README.md` (restart, clear, queue) and
  `docs/contracts/server-agent-v1.md` (`restored`, `run-store`).

## Test plan

- `go build ./...`, `go vet ./...`
- `go test ./internal/agent/... ./internal/terminal/...` (outside the sandbox:
  httptest needs local binding)
- `cd desktop && npm test`
- Manual: launch a discussion and a skill run from the desktop, let them
  finish, restart the agent from the desktop, check both are listed and their
  consoles replay; press "Clear finished consoles", check `~/.taskflow/runs/`
  is empty.
