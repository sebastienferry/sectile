# Plan #761 - Desktop: Open terminal

## Stack

- Agent: Go, `internal/agent`.
- Desktop: Electron, `desktop/electron/main.cjs` (main process, IPC to the
  agent), `desktop/electron/preload.cjs` (renderer API), `desktop/src/main.js`
  (sidebar and project menu).

## Decisions

- **D1 - One new endpoint.** `POST /desktop/project-terminal` with
  `{projectId}`, in a new file `internal/agent/agent_project_terminal.go`, routed
  from `desktopHandler`. It follows `desktopConversationTerminal`: read the
  input, resolve, `resolveTerminalForProject`, `openDirectoryTerminal`.
- **D2 - Folder resolution.** `fetchConfig(ctx, projectID, "")`, then
  `localProjectRoot(ctx, config)`, as the task launch does before calling the
  server. No `prepareMu`: opening a terminal writes nothing a preparation
  reads, and the projects listing resolves the same root without it. A server
  error is 502, a resolution error 409 with its message.
- **D3 - Capability.** `projectTerminalCapability = "project-terminal"`,
  appended to the `/desktop/status` list.
- **D4 - Visibility.** `openDirectoryTerminal` already starts the terminal
  without `agentexec.Hidden`; its comment is widened to name both launches the
  user asks for. The new handler states the same in its own comment.
- **D5 - Desktop gating.** `loadEditorSetting` reads one more capability into
  `projectTerminalAvailable`, as it does for `open-editor`; `projectMenu` adds
  the item only when it is set, disabled when `project.path` is empty. A change
  of the flag re-renders the sidebar, which `loadEditorSetting` already does.
- **D6 - IPC.** `project-terminal` handler in `main.cjs` checks the project ID
  is a non-empty string and posts it; `api.projectTerminal(projectId)` in the
  preload. The renderer reports a refusal with `error(Error(ipcMessage(err)))`,
  as the open-editor button does.

## Rejected alternatives

- Reusing `/desktop/conversation-terminal` with a project ID: it is keyed on a
  run and its refusals talk about conversations.
- Sending the path from the renderer: the menu knows `project.path`, but the
  agent is the one that decides which folders it opens (open-editor rule).

## Data contract

```
POST /desktop/project-terminal
{"projectId": "<id>"}
200 {"opened": true, "terminal": "<app>", "directory": "<folder>"}
400 | 405 | 409 | 500 | 502  text/plain reason
```

## Target files

- `internal/agent/agent_project_terminal.go` (new) and its test
  `internal/agent/agent_project_terminal_test.go`.
- `internal/agent/agent_desktop.go`: route and capability.
- `internal/agent/terminal_directory.go`: comment.
- `desktop/electron/main.cjs`, `desktop/electron/preload.cjs`,
  `desktop/src/main.js`.
- `desktop/tests/project-terminal.ui.cjs` (new).
- `docs/contracts/server-agent-v1.md`, `desktop/README.md`, `CHANGELOG.md`.
