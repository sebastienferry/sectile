# Tasks #761 - Desktop: Open terminal

Order matters: each step leaves the tree building and green.

## 1. Agent endpoint (US1, US3)

- [ ] T1.1 `internal/agent/agent_project_terminal.go`: the capability constant
      and `desktopProjectTerminal` (D1, D2, FR7).
- [ ] T1.2 Route `/desktop/project-terminal` and announce the capability in
      `/desktop/status` (D3).
- [ ] T1.3 Widen the visibility comment of `openDirectoryTerminal` (D4).
- Tests: opens the mapped folder in the project's terminal setting; a path in
  the body is ignored; 405, 400, 409 for an unmapped and for a disconnected
  project, 500 with the launch error; the capability is announced.

## 2. Desktop (US1, US2, US3)

- [ ] T2.1 IPC handler and preload method (D6).
- [ ] T2.2 Capability flag and the menu item after Project prompt, disabled
      without a local path, absent without the capability; errors to the
      banner (D5).
- Tests: a UI test against a fake agent: the item follows Project prompt and
  posts the project ID; it is disabled on a project with no path; it is absent
  when the capability is missing; a refusal shows in the error banner.

## 3. Documentation

- [ ] T3.1 `docs/contracts/server-agent-v1.md`: the endpoint and capability.
- [ ] T3.2 `desktop/README.md`: the menu paragraph names Open terminal.
- [ ] T3.3 `CHANGELOG.md`: one `Added` line under `[Unreleased]`.

## Test plan

- `go test ./internal/agent/ -run 'ProjectTerminal|DesktopStatus'`
- `go vet ./...`, `gofmt -l`
- `cd desktop && npx vite build && node tests/project-terminal.ui.cjs`
- Manual: on macOS, choose Open terminal on a mapped project; the configured
  terminal opens on its folder.
