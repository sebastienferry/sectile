# Plan #689 - Add a folder from a free console or a native-terminal discussion

Specification: `spec.md`. Stack: Go agent (`internal/agent`), Electron desktop
(`desktop/src`). No server change, no migration.

## Agent

- `internal/agent/agent_console.go`, `desktopConsole`: on the PTY path, set
  `run.interactiveProvider` to the engine the console opens, lower-cased
  (`liveProvider` on a config carrying the console's provider). A console in
  the conversation view keeps going through the conversation branch.
- `internal/agent/agent_run.go`: the comment of `interactiveProvider` covers
  the free console.
- `internal/agent/agent_run_folders.go`, `desktopRunFolder`:
  - admit `run.isConsole() && SessionID != "" && Status == "running"` next to
    the running discussion;
  - drop the `ExternalTerminal` branch that blanked the provider;
  - the refusal reads "A folder is added from a conversation, a running ticket
    discussion or a Project prompt".
  - new constant `runFoldersTerminalsCapability = "run-folders-terminals"`,
    reported in `agent_desktop.go` beside `run-folders`.
- The answer shape is unchanged (`mappedAs`, `typed`, `appliesAt`); the desktop
  knows the run's kind and terminal, so the agent adds nothing to it.

## Desktop

- `desktop/src/run-folders.mjs`:
  - `offersRunFolder(run, available, terminals)`: a free console (`kind ===
    'console'`) or a detached run is offered only with `terminals`; headless
    and ended runs never.
  - `runFolderOutcome(path, answer, run)`: `run` gives the console kind and
    the native terminal name (`terminalName`, already formatted by the
    caller); the wordings of #676 stay for every other case.
- `desktop/src/main.js`: read the `run-folders-terminals` capability with the
  others, pass it to `offersRunFolder`, and pass `{console, terminal}` to
  `runFolderOutcome` with `formatTerminalName`. The button title no longer says
  "discussion" only.

## Documentation

- `docs/contracts/server-agent-v1.md`: run-folder section.
- `docs/USER_GUIDE.md`: the Folders paragraph.
- `CHANGELOG.md`: one `Added` line under `## [Unreleased]`.

## Rejected alternatives

- Returning the terminal name from the agent: the desktop already holds it on
  the run and formats it for the badge.
- Reusing `run-folders` alone: a new desktop would show the button on a console
  against an agent that refuses it with 409.
