# Implementation checklist

- [x] 1. Add local setup-state readback and durable provider/project decisions using existing settings persistence; advertise capability and expose validated/redacted IPC. Test unknown versus incomplete, acknowledgement concurrency and cross-provider isolation.
- [x] 2. Implement actual MCP, skills and SDD observation/fingerprints and permission-review invalidation. Test changed pairing, missing/edited installed assets, changed root/framework, explicit skips and restart persistence. Keep unrelated configuration intact.
- [x] 3. Add pure assistant state/reducer and renderer module; integrate Settings > General, catalogue selection, explicit skills source and SDD target, keyboard/focus navigation and French outcomes. Test unsupported/empty catalogue and stale asynchronous responses.
- [x] 4. Wire optional provider-skills deployment and MCP choices through existing APIs, including advanced STDIO and explicit Claude repair. Test structured failure inside HTTP success, secret-free previews, preserved plugins and other MCP servers, local-agent dependency guidance and retries.
- [x] 5. Reuse Claude permissions/sandbox, Codex reviewer and Antigravity permission mode controls; support explicit retention. Test no automatic escalation, scoped saves, changed settings invalidation and Windows limitation messaging.
- [x] 6. Wire optional framework deployment and summary; preserve independent successful outcomes after cancellation, failure and reopen. Test missing folder/framework, active execution conflict, skip and retry without replaying accepted steps.
- [x] 7. Add once-per-launch in-window startup toast after readable state; test complete, incomplete, unknown, delayed reconnect, dismissal and subsequent relaunch, without OS notification authorization.
- [x] 8. Add meaningful Node tests in desktop/tests for state and IPC and UI cases with existing harness; add Go setup-state tests and extend initialization/MCP tests only where changed. Run `npm --prefix desktop test`, relevant UI tests, `npm --prefix desktop run build`, `go test ./internal/agent/... ./internal/agentconfig/...`, then repository-required checks for the final code scope. Any added process launch uses agentexec.Hidden and Windows CREATE_NO_WINDOW tests.
- [x] 9. Add one English Added changelog line describing the guided Desktop engine setup assistant under Unreleased; check every requirement/scenario, validate OpenSpec strictly and update the existing draft PR after implementation. No workflow promotion to reviewed in this stage.

## Specification-stage validation
Run `openspec validate 842-desktop-engine-setup --strict` and `git diff --check`. Manually cross-check requirements against clarification round 4 and ensure behavior and technical choices remain in separate files. No runtime changes or executable tests are part of this specification commit.

## Implementation evidence — 2026-10-14

- `internal/agent/agent_desktop_setup.go`: authenticated provider-scoped readback,
  optimistic acknowledgement checks, safe MCP comparison, effective permission
  review, and verification of installer-manifest skills/framework-owned assets.
- `internal/agentconfig/setup.go` and `local.go`: durable decisions and scoped
  asset metadata in the existing settings store; no credential-bearing metadata.
- `internal/agentconfig/mcp.go`: complete Sectile-entry comparison without
  exposing native configuration or a credential digest.
- `internal/agent/agent_desktop.go`: capability/routing and verified outcome
  recording after existing skills/framework deployment; active-run guards remain.
- `desktop/src/engine-setup-state.mjs` and `engine-setup.mjs`: generation-safe
  state, explicit project choices, optional skips, provider permission controls,
  recoverable operations, human-readable scope/results, and restart guidance.
- `desktop/src/main.js` and `style.css`: General entry and actionable startup
  toast; Electron `main.cjs`/`preload.cjs` add IPC and a process-lifetime toast
  guard. Project/engine choices refresh on reopen.
- New Go/Node/Electron tests cover persistence, changed assets/pairing/root/
  framework, stale acknowledgement, unknown state, provider isolation, transport
  choices, partial failure/retry, and cancellation/reopen during an accepted MCP
  write. Claude/Antigravity controls and Windows sandbox guidance are exercised.
- `docs/contracts/server-agent-v1.md` documents the additive capability and
  readback/acknowledgement contract; `CHANGELOG.md` adds the user-facing entry.

Validation output:

- `go test ./...`: exit 0, all packages passed.
- `go test -race ./internal/agent/... ./internal/agentconfig/...`:
  `ok tasks/internal/agent 85.567s`; `ok tasks/internal/agentconfig 1.867s`.
- Final additional `go test ./internal/agent/... -run TestSetup`:
  `ok tasks/internal/agent 0.653s` (includes real framework handler guards and
  local HTTP/remote STDIO observations for every supported provider).
- `go vet ./internal/agent/... ./internal/agentconfig/...`: exit 0, no output.
- `make fmt-check`: exit 0.
- `npm --prefix desktop test`: 302 tests, 302 passed, 0 failed.
- Affected Electron tests (engine-setup, provider-init, workstation-settings,
  conversation-mode): 12 tests, 12 passed, 0 failed. The final assistant test
  also passed after project-refresh and permission-review refinements.
- `npm --prefix desktop run build`: exit 0, built successfully; Vite reports
  its nonblocking large-chunk warning.
- `npm --prefix web test`: 751 tests, 751 passed, 0 failed.
- TypeScript (`tsc --noEmit -p web/tsconfig.app.json`): exit 0.
- `npx oxlint src` in web: exit 0, existing React warnings; no web code changed.
- `openspec validate 842-desktop-engine-setup --strict`:
  `Change '842-desktop-engine-setup' is valid`.
- `git diff --check`: exit 0. Assistant screenshot visually inspected.

Initial sandbox-only runs could not bind loopback fixture ports; rerunning with
approved execution permissions passed. A temporary Go cache under /tmp avoids
restricted workstation cache access. No failing check is hidden.

Scope remains configuration verification only: no live AI test, provider login,
AI CLI installation, automatic plugin installation, extra provider, or new child
process. Accepted clarification/specification content and branch history remain
intact. Existing draft PR #846 is reused; implementation never marks it reviewed.
