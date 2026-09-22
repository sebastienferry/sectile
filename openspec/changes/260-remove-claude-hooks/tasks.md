# Tasks

## 1. Stop installing the hook
- [x] 1.1 Delete `internal/agentconfig/hooks/hook.sh` and the `//go:embed` that carried it; delete `hookFiles`, `executableHooks`, `claudeHookEvents`, `claudeHookFile`, `claudeHookSource`, `sectileHookGroup`, `mergeHookGroups` and `registerClaudeHooks` from `hooks.go`.
- [x] 1.2 In `Scaffold` (`local.go`), stop adding hook files to the install set and stop calling `executableHooks`.

## 2. Retire what earlier releases installed
- [x] 2.1 Add `sectile-hook.sh` to `retiredClaudeHookFiles`; keep `claudeHookDir`, `managedHookPath` and `sectileHookFile` so `anyManagedPath` still accepts a manifest recording any of the three scripts and `refresh` retires them.
- [x] 2.2 Add `sectileHookRegistration`: a retired script name, or a command whose last token is `sectile-hook` (the pre-release build of #260).
- [x] 2.3 Implement `retireClaudeHooks(fs)`: read `.claude/settings.json`; return silently when it is missing or empty; report and leave it when it is not JSON; drop only Sectile-owned groups; delete an event or the `hooks` object emptied by that; rewrite atomically only when something was removed.
- [x] 2.4 Call it from `Scaffold` for every provider, append its report to the returned notes, then `Remove` `.claude/hooks` (a no-op unless empty).

## 3. Remove the agent-side receivers
- [x] 3.1 `agent_run.go`: remove the `/waiting` suffix dispatch in `handleRunControl`, `handleRunWaiting`, `relayRunWaiting`, `postRunWaiting`, the `relay` mutex on `controlledRun` and `sessionAlerts` on `runQueue`.
- [x] 3.2 `agent_desktop.go`: remove `desktopRun.WaitingSince`, the `sessionAlert` type, `sessionAlertLimit` and the `/desktop/session-alert` handler.
- [x] 3.3 Delete `internal/agent/waiting_test.go`.

## 4. Desktop
- [x] 4.1 Remove the `session-alerts` IPC handler (`electron/main.cjs`), its preload binding and the poll in `src/main.js`'s `refresh`.
- [x] 4.2 `tests/waiting-notification.ui.cjs`: drop the `/desktop/session-alert` stub and the "session Sectile did not launch" assertions; the run-transition assertions stay.

## 5. Tests
- [x] 5.1 Delete `hookscripts_test.go` and `hookrecorder_test.go`.
- [x] 5.2 Rewrite `hooks_test.go` around the retirement, with a `setHome` helper setting `HOME` and `USERPROFILE`: nothing installed or registered; every retired name and the `sectile-hook` registration removed while third-party hooks, matchers and unrelated keys survive and the emptied directory goes; a foreign file left byte for byte over two runs; idempotence with no empty `hooks` object left; an unparseable file untouched and reported; an edited script surviving while losing its registration; another provider creating no Claude settings yet still retiring a registration.
- [x] 5.3 `go build ./...`, `go vet` on `internal/agentconfig`, `internal/agent` and `cmd/...`; `GOOS=linux` and `GOOS=darwin` builds; `gofmt` clean on every touched Go file.
- [x] 5.4 `go test ./internal/agentconfig/` and `./internal/agent/` on the Windows host with `USERPROFILE` redirected, compared against an `origin/main` worktree run the same way: no new failure in either package; the agentconfig failures that disappeared are the deleted hook tests, the rest is the pre-existing Windows baseline (`t.Setenv("HOME")` not redirecting `os.UserHomeDir`).
- [ ] 5.5 `desktop` unit and UI tests: not run on this host (no `desktop/node_modules`, and the UI test drives Electron under Playwright). CI runs them.

## 6. Documentation
- [x] 6.1 ADR 0012: status line revised, "Revision (#260): the hooks are withdrawn" appended.
- [x] 6.2 `docs/CAPABILITIES.md` §4bis rewritten: no hook, the model kept idle, the desktop banner on run transitions.
- [x] 6.3 `.agents/MEMORY.md` §6 rewritten: the retirement rules that must stay, the Windows `HOME` trap, the idle waiting model.
- [x] 6.4 `docs/clarifications/260.md` records the redirection of the ticket.
- [x] 6.5 `openspec validate 260-remove-claude-hooks --strict`.
