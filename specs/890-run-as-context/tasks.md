# Tasks #890 - Take an earlier run as the context of a new run

Order matters: each step leaves the tree buildable and `make test` green.
Tests go with the step they cover. One commit per step.

## 1. Record the context run

- [ ] T1.1 `internal/db/migrations.go`: migration 57,
      `task_activities.context_run_id`. `models.TaskActivity.ContextRunID`,
      read by the activity scans (FR3).
- [ ] T1.2 `internal/db/runcontext.go`: `RunContextSource` (eligibility,
      `agent_launch` resolved to its remote run, `allowActive`) and
      `RunContextRecord` (layout, 32 KiB budget, markers) (US4, FR6).
- Tests: `runcontext_test.go`; migration on SQLite and PostgreSQL.

## 2. Launch and retry carry it

- [ ] T2.1 `RunSkillRequest.ContextRunID`; `run-skill` validates it before the
      busy check, records it on the remote run (`RunLaunch.ContextRunID`) and
      the `agent_launch` activity; the no-agent path refuses it (FR1).
- [ ] T2.2 `agentconfig.Dispatch`: `ContextRunID`, `ContextRecord`, set by
      `run-skill` (FR4).
- [ ] T2.3 Retry body `withContext`: sets `contextRunId` to the original's
      remote run; refused outside the task-run branch; plain retry unchanged
      (FR2).
- Tests: refused ID leaves no row; accepted ID recorded and dispatched;
  `withContext` retry of a canceled run; plain retry unchanged.

## 3. The agent writes the context

- [ ] T3.1 `runStore.read`; plain-text rendering of a stored console.
- [ ] T3.2 `internal/agent/run_context.go`: `writeRunContext` (transcript or
      server record, 256 KiB budget, `0600` file, pruned with the run).
- [ ] T3.3 `agent.go`: prompt notice, failure notice; `desktopRun.ContextRunID`
      (US2, FR4, FR5).
- Tests: `run_context_test.go` (trace, console, record only, budget, mode,
  failure, prompt notice).

## 4. MCP

- [ ] T4.1 `start_run` takes `contextRunId` (task runs only, launcher value
      wins) and returns the context (FR7, US3.1).
- [ ] T4.2 `get_run` on the server (US3.3).
- [ ] T4.3 Agent loopback `GET /desktop/run-context`; stdio bridge serves
      `get_run` through it when available, else forwards; catalog of 22;
      `mcptest/contract.go` (US3.2).
- Tests: `taskmcp` and `agentmcp` cases.

## 5. Desktop resume

- [ ] T5.1 Agent route `POST /desktop/runs/resume` forwarding to the retry
      with `withContext` and this device; capability `run-resume`.
- [ ] T5.2 `preload.cjs` and `main.cjs`: `resumeRun`, capability check.
- [ ] T5.3 `main.js`: "Resume" button on the sidebar row and the toolbar for
      completed, failed and canceled task runs; selection of the new run; busy
      refusal handled as `launchTaskWork`; context line in the console view
      (US1, US5.1, FR8).
- Tests: agent route test; `desktop/tests/run-resume.ui.cjs`.

## 6. Web context line

- [ ] T6.1 `web/src/types/index.ts`: `contextRunId`; `TaskDetailModal.tsx`:
      context line; locale strings in French and English (US5.2, FR9).
- Tests: task detail case; `i18n.test.mjs` green.

## 7. Finish

- [ ] T7.1 `CHANGELOG.md`: `Added` line under `## [Unreleased]` (FR10).
- [ ] T7.2 Check the plain retry and the launch dialog are unchanged
      (scope).
- [ ] T7.3 `make test` green.
