# Plan #588 - Consoles survive an agent restart

Implements `spec.md`. The agent already holds everything the sidebar shows:
the run index (`runQueue.runs`), each terminal session's 64 KiB history and
each headless run's trace. This change writes a private copy of each run to
the workstation and reloads it at start; a restored run is served through the
read-only replay path the headless trace already uses.

## Stack and constraints

- Go agent (`internal/agent`, `internal/terminal`), Electron desktop
  (`desktop/src`). No server change, no migration, no MCP change.
- Runtime strings (log lines, desktop notices) keep the language their surface
  already speaks; code, comments and docs are in English.
- The desktop and the agent are versioned apart: the only contract change is
  one optional field, `restored`, on `/desktop/runs`.

## Architecture

```
controlledRun ──(status, output)──▶ runStore.save ──▶ <state>/runs/<id>.json (0600)
      ▲                                                       │
      │                                                       │ agent start
      └──────────── restored controlledRun ◀── runStore.load ◀┘
                       (exited closed, trace = closed replay)
```

### The store (`internal/agent/run_store.go`, new)

```go
// runStore keeps the runs the desktop lists between two agent processes.
type runStore struct {
	dir string // <state dir>/runs, "" disables the store
}

type storedRun struct {
	Version    int        `json:"version"` // runStoreVersion = 1
	Run        desktopRun `json:"run"`
	Console    []byte     `json:"console,omitempty"` // base64 in JSON
	Trace      []string   `json:"trace,omitempty"`
	FinishedAt time.Time  `json:"finishedAt,omitzero"`
}
```

- `newRunStore(stateDir)`: `MkdirAll(dir, 0700)` then `Chmod(dir, 0700)`;
  on error it logs and returns a disabled store.
- `save(record)`: refuses an id that is not a safe file name
  (`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`), marshals, writes a temp file in the
  same directory, `Chmod 0600`, `Rename` over `<id>.json`. Same pattern as
  `writeDesktopInfo`.
- `remove(ids...)`: deletes files, ignoring `ErrNotExist`.
- `load() []storedRun`: reads every `*.json`, skips and logs unreadable,
  unparsable, unknown-version or unsafe-id files; deletes the files of
  `queued`/`preparing` runs; rewrites `running` as `canceled`, with
  `FinishedAt` set to the file's modification time.
- `prune(keep int, live map[string]bool)`: among files whose run is finished
  and not live, keeps the `keep` most recent by `FinishedAt` (fallback
  `CreatedAt`) and deletes the rest. `runStoreRetained = 100`.

### Console capture (`internal/agent/run_store.go`)

```go
// consoleTap is a run's own copy of what its terminal printed, bounded as the
// session history is, and kept after the session closes.
type consoleTap struct {
	mu    sync.Mutex
	bytes []byte
	dirty bool
}
```

Added to `controlledRun` as `console *consoleTap`. Wherever a run is given its
terminal session (`agent.go` skill runs, `agent_macro_dispatch.go`,
`agent_console.go` free consoles, `agent_desktop.go` discussion sessions),
`d.tapConsole(run, sessionID)` registers
`terminal.Manager.AddOutputListener(sessionID, tap.write)`. The limit is the
terminal's: `internal/terminal` exports `HistoryLimit = 65536` and uses it for
`maxHistBytes`.

The trace needs no tap: `runTrace` gains `snapshot() []string` (copy of
`lines` under its lock) and a `dirty` counter read by the flusher.

### When runs are written (`internal/agent/run_store.go`)

- `d.persistRun(id)`: copies the record, the tap bytes and the trace snapshot
  under the queue lock, then saves outside it. Sets `FinishedAt` once the run
  has exited.
- `d.persistLoop(ctx)`: a 5 s ticker; saves each run whose tap or trace is
  dirty or whose status changed since its last save (`controlledRun.savedStatus`).
- On exit: `d.trackRun(id, run)`, called where a run enters the index
  (`enqueueRunLocked`, `wrapRun`, `registerHeadlessRun`), starts a goroutine
  that waits on `run.exited`, then calls `persistRun` and `prune`. No store
  I/O happens under the queue lock, and none of the places that close
  `exited` had to change. *Changed during implementation: the plan first
  funnelled every close through a `markExited` helper; waiting on the channel
  covers the same paths without touching them.* The one path that deletes a
  run it never started (external terminal failure) now closes `exited` first,
  so its watcher ends.
- On graceful stop: `Run` saves every run in a `defer` registered after the
  session-closing `defer`, so it runs first (defers are LIFO) and captures
  the output before the sessions close.

### Loading (`internal/agent/agent.go`, `Run`)

After `writeDesktopInfo` is decided and before `startLocalProxy`:
`daemon.runStore = newRunStore(filepath.Join(filepath.Dir(desktopInfo), "runs"))`,
then `daemon.restoreRuns()`. Each loaded record becomes a `controlledRun` with
`exited` closed, `restored = true`, `desktop.Restored = true`,
`desktop.SessionID = ""`, `WaitingSince` zero, `sequence` from the queue
counter, and `trace` set to a closed `runTrace` holding either the stored trace
lines or one line with the stored console bytes. Tests that build an
`agentDaemon` by hand have no store and keep today's behaviour.

### Serving a restored run (`internal/agent/agent_desktop.go`)

- `desktopRun` gains `Restored bool `json:"restored,omitempty"``.
- `/desktop/terminal`: unchanged. A restored run has no live session, so the
  existing `trace != nil` branch calls `serveRunTrace`, which replays, sees the
  closed channel, and ends; frames sent by the viewer are discarded.
- `/desktop/history` (DELETE): after removing runs from the index, calls
  `d.runStore.remove(removed...)` outside the queue lock.
- The failure path that deletes a run it could not start (line ~1282) also
  removes its file.

### Desktop (`desktop/src/run-console.mjs`)

- `needsConsoleNotice(run)`: `false` for `run.restored===true`.
- `readOnlyConsole(run)`: `true` for `run.restored===true`.
- No change in `main.js`: attach, relaunch, clear and archive already work from
  the run record.
- `desktop/electron/main.cjs`: the restart and stop confirmation said "Console
  history will be cleared". It reads `run-store` in the `capabilities` of
  `/desktop/status` (advertised by an agent that has a store) and says the
  consoles are kept; an older agent keeps the old warning. *Added during
  implementation: the sentence would otherwise be false.*

## Data contract

`GET /desktop/runs` entries gain `"restored": true` on restored runs; absent
otherwise. `GET /desktop/status` lists `run-store` in `capabilities` when the
agent has a store. Nothing else changes on the loopback API, the server protocol or
MCP.

## Rejected alternatives

- **Storing in the Electron renderer**: the desktop can be closed while the
  agent keeps running, and the agent owns the PTY history and the trace (D1).
- **Storing on the server**: needs a table, a migration and an upload of
  console output that never leaves the workstation today (Q4, ADR 0016).
- **Reading the session history at save time only**: the session closes when
  the shell exits or when history is cleared, taking its buffer with it; the
  tap keeps the output independently of the session (FR4).
- **One SQLite file for the store**: one JSON file per run needs no schema,
  makes deletion a file removal and keeps a corrupt run from hiding the others.

## Target files

- `internal/agent/run_store.go` (new), `internal/agent/run_store_test.go` (new)
- `internal/agent/agent.go`, `agent_run.go`, `agent_desktop.go`,
  `agent_console.go`, `agent_headless.go`, `agent_macro_dispatch.go`,
  `agent_trace.go`
- `internal/terminal/terminal.go` (`HistoryLimit`)
- `desktop/src/run-console.mjs`, `desktop/tests/run-console.test.cjs`
- `docs/adrs/0040-consoles-survive-an-agent-restart.md`, status lines of
  `docs/adrs/0003-local-desktop-consoles.md` and
  `docs/adrs/0016-the-trace-of-an-autonomous-run-stays-on-the-workstation.md`
- `CHANGELOG.md`
