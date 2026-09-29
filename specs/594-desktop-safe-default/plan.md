# Plan #594 - Five parallel executions by default

## Stack

Go local agent (`internal/agentconfig`, `internal/agent`), Electron desktop
renderer (`desktop/src/main.js`), Markdown documentation. No server, web, or
database change; no migration.

## Design

### One fallback, in the agent

`agentconfig.ExecutionLimit` is the only place that turns settings into a
limit: the queue (`agent_run.go`), the consoles, the desktop project view
(`agent_desktop.go`) and the settings views (`agent_desktop_settings.go`) all
call it. Its last fallback moves from 1 to a new constant:

```go
// DefaultParallelism is the limit of a project that uses worktrees when
// neither its section nor the workstation defaults set one.
const DefaultParallelism = 5
```

The order stays: project section, else defaults, else `DefaultParallelism`;
a value below 1 (the `-1` written for an explicit zero, which validation
refuses) still yields 1; above `MaxParallelism` yields 10; no worktrees
yields 1.

Because the settings views already report `ExecutionLimit` for the effective
and inherited values, the desktop receives 5 without any API change.

### Desktop display fallbacks

`desktop/src/main.js` mirrors the constant next to `MAX_PARALLELISM`:

```js
// Parallel executions of a project when no setting states one, aligned with
// agentconfig.DefaultParallelism.
const DEFAULT_PARALLELISM=5
```

- Workstation panel: `parallelism||1` becomes `parallelism||DEFAULT_PARALLELISM`
  and the hint `'1 execution'` becomes the readout of that constant
  (`'5 executions'`).
- Project panel: `Number(fields.parallelism.value)||1` becomes
  `||DEFAULT_PARALLELISM`. The value comes from the agent and is never 0 in
  practice; the fallback only covers an older agent.

### Rejected alternatives

- Writing `5` into the defaults of new installs only: keeps existing
  workstations at 1, which the owner rejected (round 2, decision 1).
- A desktop-only default written by the app: the headless agent would
  disagree about what an unset value means (round 2, decision 2).

## Data contracts

Unchanged. `GET /desktop/settings` and the project view already carry the
computed limit; only its value for an unset setting changes.

## Target files

- `internal/agentconfig/local.go`: constant and fallback, doc comment.
- `internal/agentconfig/workstation_test.go`: `TestExecutionLimitInheritance`
  covers the unset case at 5, explicit 1 in defaults and in the project, no
  worktrees.
- `internal/agent/*_test.go`: any test that expected 1 for an unset value.
- `desktop/src/main.js`: constant and the two fallbacks.
- `desktop/tests/workstation-settings.ui.cjs`: default readout and hint.
- `docs/contracts/server-agent-v1.md`: lines 58 and 80.
- `CHANGELOG.md`: one `Changed` line.

## Test plan

- `go test ./internal/agentconfig/... ./internal/agent/...`
- `go vet ./...`
- Desktop: `npx vite build` then the UI tests `workstation-settings.ui.cjs`
  and `console.ui.cjs`.
