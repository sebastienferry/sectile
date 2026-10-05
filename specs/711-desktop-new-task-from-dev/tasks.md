# Tasks #711 - A launch from the web follows the Desktop console view

Ordered checklist. Each group is one commit and leaves the tree buildable and
the tests green.

## 1. The stored value (FR1, FR3)

- [ ] T1.1 `internal/agentconfig/workstation.go`: `Defaults.ConsoleView`, the
  two constants and `ConsoleViewOrDefault()`.
- [ ] T1.2 `internal/agentconfig/local.go`, `overlay`: carry `ConsoleView`
  with `firstSet`.
- [ ] T1.3 Tests: round trip through `WriteSettings` and `ReadSettings`, the
  overlay with a legacy repository file, and an absent value reading
  `terminal`.

## 2. The endpoint (FR7, FR9)

- [ ] T2.1 `internal/agent/agent_desktop_console_view.go`: capability constant
  and `desktopConsoleView` (GET, PUT, 400, 405), saved under `d.prepareMu`
  with `UpdateSettings`.
- [ ] T2.2 `internal/agent/agent_desktop.go`: route `/desktop/console-view`
  and add the capability to `/desktop/status`.
- [ ] T2.3 Tests: PUT then GET, unknown value refused and stored value kept,
  capability listed. Update any test that pins the capability list.

## 3. The dispatch (FR4, FR5, FR6)

- [ ] T3.1 `internal/agent/agent.go`: take the mark first, then open a
  conversation when marked or when the workstation view is `conversation`,
  with eligibility unchanged; a settings read error means `terminal`.
- [ ] T3.2 Rewrite the comments of `pendingDiscussionViews` and of the `View`
  field of `desktopTasks` to describe the mark as an explicit request and the
  workstation view as the fallback.
- [ ] T3.3 Tests: conversation without a mark when the view is
  `conversation`; PTY when it is `terminal` or absent; autonomous,
  `open_terminal` and a non-Claude engine unchanged with the view
  `conversation`; a mark still consumed.

## 4. Desktop hands the setting over (FR2, FR7, FR8)

- [ ] T4.1 `desktop/electron/main.cjs`: `syncConsoleView()` gated by the
  capability, called after `set-console-view` saves, errors swallowed, and the
  `sync-console-view` IPC handler.
- [ ] T4.2 `desktop/electron/preload.cjs`: expose `syncConsoleView`.
- [ ] T4.3 `desktop/src/main.js`, `ready()`: call `api.syncConsoleView()`
  once per connection, next to `loadEditorSetting()`.
- [ ] T4.4 UI test (`desktop/tests/conversation.ui.cjs` or a new
  `console-view-sync.ui.cjs`): a change of **Claude consoles** sends the PUT
  to a fake agent with the capability and nothing to one without it; a
  connection sends the saved value; the setting is saved when the agent
  refuses.

## 5. Documentation

- [ ] T5.1 `CHANGELOG.md`, `[Unreleased]`: amend the line **A task's
  interactive launches open as a Claude conversation.** so that it covers
  launches started from Desktop or from the web app, a chain the server
  continues included. No new line.
- [ ] T5.2 `desktop/README.md:7` (**Claude consoles → Conversation**): say
  that the setting also applies to launches from the web app.

## 6. Gates

- [ ] T6.1 `go test ./internal/agentconfig/... ./internal/agent/...` outside
  the sandbox; `go vet ./...`.
- [ ] T6.2 `cd desktop && npx vite build`, then `npm test` and
  `npm run test:ui` (sandbox off); restore `internal/webui/.gitkeep` if the
  build removed it.
- [ ] T6.3 `git diff origin/main --stat` reviewed: only the files of the plan.
