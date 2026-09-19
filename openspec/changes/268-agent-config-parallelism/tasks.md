# Tasks: Set a project's parallelism from the agent configuration

- [x] 1. **Add the `config` role to the workstation executable**
  - [x] 1.1 `agent.Configure` parses `--project`, `--parallelism` and `--repo`, and reads the settings through `agentconfig.ReadSettings`.
  - [x] 1.2 Writing enforces the 1..`MaxParallelism` bounds and refuses an out-of-range value without storing it.
  - [x] 1.3 An absent `--parallelism` reports the current settings instead of writing, so `--parallelism 0` stays a range error rather than a read.
  - [x] 1.4 Dispatch `config` from `cmd/agent/main.go` beside `pair`, `mcp` and `agent-exec`.

- [x] 2. **Tests**
  - [x] 2.1 Setting a value stores it and changes the effective `ExecutionLimit`.
  - [x] 2.2 Writing one project preserves the other projects' mappings and values.
  - [x] 2.3 Out-of-range and project-less invocations fail and store nothing.
  - [x] 2.4 The report distinguishes a stored value from the default.
  - [x] 2.5 The tests isolate `HOME` **and** `USERPROFILE`, so they never write the developer's real settings on Windows.

- [x] 3. **Documentation**
  - [x] 3.1 README "Execution defaults and local overrides" names both surfaces and shows the command.
  - [x] 3.2 `docs/contracts/server-agent-v1.md` no longer calls the desktop app the only writer.
