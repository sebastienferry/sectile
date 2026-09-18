# Tasks

## 1. The hook as a subcommand
- [x] 1.1 Create `internal/agenthook/hook.go`: `Report(payload []byte, getenv func(string) string, home string)` decoding `hook_event_name`, `notification_type` and `cwd`, applying the #174 event table (`Notification` with a prompt type or none, and `Stop`, open a wait; `UserPromptSubmit`, `PreToolUse`, `PostToolUse` close it; `Stop` alerts `completed`, a prompt alerts `waiting`, nothing else alerts), with a two-second HTTP client, every transport error swallowed and nothing written to either stream.
- [x] 1.2 In the same file, `Run()` reading `os.Stdin`, `os.Getenv` and `os.UserHomeDir`, returning nothing so the process always exits 0.
- [x] 1.3 Implement the launched-session branch: `SECTILE_RUN_ID` plus `SECTILE_LOOPBACK_URL` (falling back to `SECTILE_AGENT_URL`) plus `SECTILE_AGENT_TOKEN` → `POST <loopback>/control/runs/<id>/waiting` with `{"waiting":<bool>}` and the bearer token, and no session alert afterwards.
- [x] 1.4 Implement the unlaunched-session branch: read `<home>/.taskflow/agent-connection.json`, take `url` and `token`, `POST <url>/desktop/session-alert` with `{"session":…,"state":…}`; name the session with `filepath.Base` of `cwd`, falling back to `Claude Code`, and encode the body with `json.Marshal`.
- [x] 1.5 Dispatch `sectile-hook` in `cmd/agent/main.go`, beside `pair`, `mcp` and `agent-exec`, without `log.Fatal`.
- [x] 1.6 Carry over the two guards the script had and the port would otherwise lose: ignore `SIGHUP`, `SIGINT` and `SIGTERM` (the script trapped them and exited 0; Go's default is a non-zero death, which Claude Code reads as a failed hook), and bound the read of stdin, which nothing else would free once the signals are ignored.

## 2. Registration
- [x] 2.1 In `internal/agentconfig/hooks.go`, add `claudeHookSubcommand = "sectile-hook"` and `claudeHookCommand(executable, goos string) string` quoting the executable for `cmd.exe` on `windows` and for POSIX `sh` elsewhere.
- [x] 2.2 Move `sectile-hook.sh` into `retiredClaudeHookFiles`; delete `claudeHookFile`, `claudeHookSource`, the `//go:embed hooks/hook.sh` directive, `hookScripts`, `hookFiles` and `executableHooks`; delete `internal/agentconfig/hooks/hook.sh`; keep `claudeHookDir` and `managedHookPath` so the retirement may touch `.claude/hooks/sectile-hook.sh`.
- [x] 2.3 Replace `sectileHookFile`'s current-file case with an argument check: a command whose last token is `sectile-hook` is owned; a command whose base name is a retired script is owned. Keep `sectileHookFile` itself for `managedHookPath`, and route `ownsHookGroup` through the new check.
- [x] 2.4 Change `registerClaudeHooks(fs *os.Root, home string)` to take the resolved command instead of the home directory; have `Scaffold` resolve `os.Executable()` and build the command with `runtime.GOOS`, and drop the `hookFiles` and `executableHooks` calls from `local.go`.

## 3. Tests
- [x] 3.1 Delete `internal/agentconfig/hookscripts_test.go`; move `startJSONRecorder` out of `hookrecorder_test.go` into the new package's own test helper, or keep both copies if `agentconfig` still needs one.
- [x] 3.2 New `internal/agenthook/hook_test.go`, table-driven over the five events and the notification types: the run report and its `waiting` value; the session alert and its state; no alert for a launched session; the generic fallback name; a Windows `cwd` naming the session `sectile`; a name containing a quote or a backslash producing a well-formed body; a dead loopback, an unresponsive server (bounded by the timeout), an empty payload, a non-JSON payload, a payload with no event, and an unanswered event each making no report and returning.
- [x] 3.3 New registration tests in `internal/agentconfig/hooks_test.go`: `claudeHookCommand` for `windows` and for `linux`, with and without a space in the path; `TestHooksAreInstalledExecutableAndRegistered` replaced by one asserting every event registers `<executable> sectile-hook` and that no file is installed under `.claude/hooks`.
- [x] 3.4 Extend `TestLegacyHookScriptsAreRetiredWithTheirRegistrations` to seed `sectile-hook.sh` with its manifest digest and a registration under a former home, and assert the file is removed, the registration replaced in place, and no duplicate entry is left.
- [x] 3.5 Add a case to the same file for a hook script the user edited: the file survives, the registration is still replaced.
- [x] 3.6 Confirm `TestHookRegistrationPreservesEverythingElse`, `TestHookRegistrationIsIdempotent`, `TestUnparseableSettingsAreLeftUntouchedAndReported` and `TestNoHookIsWrittenForAnotherProvider` still hold, adjusting only what the new command shape requires.

## 4. Verification and documentation
- [x] 4.1 `gofmt -l` clean on every file this change touches, `go build ./...`, `go vet ./...`; `go test ./...` compared against an `origin/main` worktree on the same host: no new failure, eleven fewer. The 35 that remain are a pre-existing Windows-host baseline (`t.Setenv` of HOME not redirecting `os.UserHomeDir`, POSIX paths failing `filepath.IsAbs`, 0666 permission assertions), identical on `origin/main`.
- [x] 4.2 `GOOS=windows go build ./...` and `GOOS=darwin go build ./...` to confirm the cross-compilation CI performs.
- [x] 4.3 Update `.agents/MEMORY.md` §6: the hook is a subcommand of the agent binary, ownership is read from the trailing argument, `sectile-hook.sh` is retired, and there is no executable bit to set any more.
- [x] 4.4 Update `docs/adrs/0012-waiting-for-input-as-a-timestamp.md` and any documentation naming `sectile-hook.sh` or `~/.claude/hooks`.
- [x] 4.5 Run `openspec validate 260-hooks-on-windows --strict` when the CLI is available; otherwise record that it could not be run and why.
