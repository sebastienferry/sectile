# #391: Implementation plan

References: [`spec.md`](./spec.md), [`tasks.md`](./tasks.md).

## Stack and scope

Go server and agent (`internal/agent`, `internal/agentprotocol`,
`internal/handlers`, `internal/db`), and the desktop Electron main process and
renderer (`desktop/electron/main.cjs`, `desktop/src/main.js`). No migration, no
new endpoint, no `agentconfig.Version` bump, no web client change.

## Target files

| File | Change |
| --- | --- |
| `internal/agentprotocol/operations.go` | `Operations` (the canonical operation list), `ErrUnsupportedOperation`, `UnsupportedOperationError`, `IsUnknownOperationReply`. |
| `internal/agentprotocol/operations_test.go` (new or extended) | Message wording, `errors.Is`, reply matching. |
| `internal/agent/agent_operations.go` | The allow-list `switch` reads `agentprotocol.Operations`. |
| `internal/agent/agent.go` | Announcement query parameters on the WebSocket URL; binary fingerprint captured at start. |
| `internal/agent/agent_desktop.go` | `/desktop/version` adds `binarySha256`. |
| `internal/agent/agent_version_test.go` | Fingerprint on `/desktop/version`; announcement in the dial URL. |
| `internal/handlers/handlers.go` | `HandleAgentConnect` parses the announcement and hands it to `Register`. |
| `internal/handlers/agent_dispatcher.go` | `AgentConn` gains `Build`; `AgentConnInfo` gains the build fields and `outdated`. |
| `internal/handlers/agent_operations.go` | Capability check and legacy conversion in `callOperationLocal`. |
| `internal/handlers/agent_cluster.go` | `codeAgentOutdated` in `internalAnswer` and `forward`. |
| `internal/handlers/macroruns.go` | `slicingReadError` uses `errors.Is`. |
| `internal/db/adjustment.go` | `errAgentTooOld` wraps `ErrUnsupportedOperation`. |
| `internal/handlers/agent_operations_test.go`, `agent_dispatcher_test.go`, `agent_cluster_test.go`, `macroslicing_test.go` | See the test plan. |
| `internal/db/adjustment_test.go` | Legacy and announced `pr_evidence` wording. |
| `desktop/electron/agent-identity.cjs` (new) | `fileSha256(path)`, `agentOutdated({running, bundled})`. |
| `desktop/electron/main.cjs` | Check after each successful connection; `version` IPC returns `outdated`; restart that relaunches the bundled binary. |
| `desktop/electron/preload.cjs` | `agent-outdated` event subscription. |
| `desktop/src/main.js` | Outdated mark in the settings versions panel. |
| `desktop/tests/agent-identity.test.cjs` (new), `desktop/tests/settings-version.ui.cjs` | See the test plan. |
| `CHANGELOG.md` | One `Fixed` line. |

## Decisions

### Canonical operation list in `agentprotocol`

`agentprotocol` is already imported by the agent, the handlers and `internal/db`.
It gains:

```go
// Operations lists every workspace operation an agent of this build runs.
var Operations = []string{"macro_worktree", "macro_spec_file", "git_status", ... "init_git"}
```

`executeOperation` replaces its literal `case` list with a lookup in a set
built from `Operations` (the two macro actions stay handled before it, but are
listed too). What the agent announces is therefore exactly what it dispatches
on (FR1). The server uses the same slice to compute `outdated`: an announcing
agent is outdated when any entry of the server's `Operations` is missing from
its list.

Rejected: comparing `version.Current()` strings. Between tags every build says
the same thing (`dev` or the last tag), which is the reported incident.

### Announcement on the WebSocket URL

The agent adds three query parameters to `/ws/agent-connect`, next to
`projectId` and `deviceId`:

| Parameter | Value |
| --- | --- |
| `agentVersion` | `version.Current().Version` |
| `agentCommit` | `version.Current().Commit`, omitted when empty |
| `operations` | `strings.Join(agentprotocol.Operations, ",")` |

The server treats the presence of `operations` (even empty) as "announced" and
its absence as legacy. A query parameter is known at `Register` time, so no
operation can race an announcement message, and an older server ignores
unknown parameters (FR2). The list is ~25 short names, well under URL limits.

Rejected: a new `agent_hello` message after the upgrade. It opens a window in
which the server relays operations with no capabilities, and it needs ordering
guarantees the query parameter gets for free.

```go
type AgentBuild struct {
    Announced  bool
    Version    string
    Commit     string
    Operations map[string]bool
}
```

`Register(userID, projectID, deviceID string, build AgentBuild, conn)` stores it
on `AgentConn` (immutable after registration, so no lock). A rebind builds a
new `AgentConn`, which replaces the announcement.

### One typed error

```go
var ErrUnsupportedOperation = errors.New("local agent does not support the operation")

type UnsupportedOperationError struct {
    Device    string
    Build     string // "" for a legacy agent
    Operation string
}

func (e *UnsupportedOperationError) Error() string  // see wording below
func (e *UnsupportedOperationError) Unwrap() error { return ErrUnsupportedOperation }
```

Wording (English, FR6):

```
the local agent on <device> (<build>) does not support the "<operation>" operation; restart or update the Sectile desktop app, then retry
```

`<build>` is `version` or `version, commit <12 chars>` when a commit is known,
and `unknown build` for a legacy agent.

`IsUnknownOperationReply(reply, operation string) bool` matches exactly
`unknown local operation "<operation>"`, the agent's current text, so that FR5
converts only that reply.

### Where the check sits

In `callOperationLocal`, after the `execute_skill` / `open_terminal` branch
(those are dispatches, not operations) and before anything is sent:

1. announced and `op.Action` is in the server's `Operations` but not in the
   agent's list → return `&UnsupportedOperationError{...}` (FR4);
2. otherwise relay as today; on `res.Error`, if `!Announced` and
   `IsUnknownOperationReply(res.Error, op.Action)` → the same error (FR5);
   any other reply keeps today's `local agent: %s`.

An action the server itself does not list is relayed unchecked (edge case).
`callOperationLocal` is also what `serveInternal` runs for a forwarded
operation, so the check applies on the instance holding the agent whichever
instance received the request.

### Crossing instances

`internalAnswer` maps `errors.Is(err, agentprotocol.ErrUnsupportedOperation)`
to a new `codeAgentOutdated = "agent_outdated"`. `forward` turns that code back
into an `outdatedRelay{message: answer.Error}` whose `Error()` is the message
verbatim and whose `Unwrap()` is `ErrUnsupportedOperation`, so the text is
identical on both instances and `errors.Is` holds (FR7).

Rejected: `fmt.Errorf("%w: %s", ErrUnsupportedOperation, answer.Error)`, the
pattern the other codes use. It would prefix the sentinel's text and make the
forwarded message differ from the local one.

### Callers

- `internal/db/adjustment.go`: `agentBranchPullRequest` keeps its
  `"%s lookup failed on the local agent: %w"` wrap, so the transition error
  reads "GitLab merge request lookup failed on the local agent: the local agent
  on … does not support the "pr_evidence" operation; restart or update …". It
  never reads as absence (FR8, unchanged path). `errAgentTooOld` becomes a
  wrapped error (`fmt.Errorf("%w: local agent is too old to look up a pull
  request in another repository; restart or update the Sectile desktop app",
  agentprotocol.ErrUnsupportedOperation)`), and its text changes to name the
  same fix (FR9).
- `internal/handlers/macroruns.go`: `slicingReadError` replaces the
  `strings.Contains(... "macro_spec_file")` branch with
  `errors.Is(err, agentprotocol.ErrUnsupportedOperation)`, same French message.
  The db layer returns the dispatcher's error unwrapped (`callAgentContext`
  returns it as is), so the chain reaches the handler intact.

### Status fields

`AgentConnInfo` gains, all `omitempty` (US3.4):

```go
AgentVersion string   `json:"agentVersion,omitempty"`
AgentCommit  string   `json:"agentCommit,omitempty"`
Operations   []string `json:"operations,omitempty"`   // sorted
Outdated     *bool    `json:"outdated,omitempty"`
```

`ConnectedAgents` fills them from `AgentConn.Build`. `clusterAgents` fills them
only for the connections this instance holds (the same place it already reads
`LastSeen`); an entry held elsewhere keeps them empty and `Outdated` nil, since
presence rows carry no build and nothing is persisted (US3.3, no migration).
A legacy agent gets `Outdated: true` and no version or list (US3.2).

### Binary identity

The one detail the clarification left to the specification. The identity is the
SHA-256 of the agent executable's content (FR11):

- the agent hashes `os.Executable()` once at start, before serving the desktop
  interface, and `/desktop/version` answers
  `{"version", "commit", "date", "binarySha256"}` (the `version.Info` fields
  plus one, in a local struct; `/api/version` is not touched);
- the desktop hashes the binary `resolveAgentBinary` returns, the one it would
  spawn, with `crypto.createHash('sha256')` over a read stream.

Content hashing is the only signal that sees a same-tag rebuild of
`desktop/bin` with uncommitted changes; the version and the VCS commit do not.
Hashing a ~40 MB binary costs tens of milliseconds, once per agent start and
once per desktop check. The agent hashes at start, not on request, because the
file on disk may already be the new build by the time the desktop asks.

Rejected: running the bundled binary with `--version`. It spawns a process on
every connection, and it only sees the version and the commit.

`agentOutdated({running, bundled})` in `agent-identity.cjs` is the pure rule:

| `running.binarySha256` | `bundled` | Result |
| --- | --- | --- |
| missing (legacy agent) | any readable | outdated |
| equal | equal | not outdated |
| different | readable | outdated |
| any | unreadable (`null`) | not outdated (no prompt, US edge case) |

### Desktop flow

- `main.cjs` keeps `outdated` (boolean) and `promptedFor` (the running agent's
  `binarySha256`, or `'legacy'`) in module state.
- `checkAgentIdentity()` runs after every successful `connectAgent()` and after
  the spawn loop of `start`: it reads `/desktop/version`, hashes the bundled
  binary, and sets `outdated`. It sends `agent-outdated` to the renderer with
  the new value. When `outdated` and `promptedFor` differs from the current
  identity, it records `promptedFor` and calls `lifecycle('restart', {reason:
  'outdated'})` (US4.1, US4.3).
- `lifecycle` accepts the reason: the dialog `detail` gets a first line
  "The running agent is not the one bundled with this app." before the existing
  active-executions text. The buttons and the rest of the dialog are unchanged.
  The dialog is English today, and the line follows it (D8).
- On confirmation with the `outdated` reason, `lifecycle` stops the agent
  (`/desktop/shutdown`, as `stop` does) and then runs the `start` path with the
  saved settings, so the bundled binary is spawned even when the running agent
  came from another path (US4.2). `/desktop/restart` re-executes the agent's
  own path and would relaunch the old file in that case.
- The `version` IPC returns `{desktop, agent, outdated}`. The settings panel
  (`fillChangelogPanel`) shows, when `outdated`, a mark after the agent version
  value: "outdated: restart the local agent" (English, as the panel). The
  renderer also refreshes the mark on `agent-outdated`.

## Test plan

| Test | Proves |
| --- | --- |
| `agentprotocol` unit | The message for a known build, a build with commit, and `unknown build`; `errors.Is(err, ErrUnsupportedOperation)`; `IsUnknownOperationReply` matches only the exact reply for that operation. |
| `internal/agent` dial URL | `agentVersion`, `operations` (equal to `Operations`), `agentCommit` omitted when empty. |
| `internal/agent` `/desktop/version` | `binarySha256` equals the SHA-256 of the test binary. |
| `internal/agent` dispatch | Every entry of `Operations` passes the allow-list; an unknown action still answers `unknown local operation`. |
| `handlers` connect | A WebSocket handshake with the three parameters registers them on `AgentConn`; one without `operations` registers a legacy build. |
| `handlers` operation, announced | An agent announcing no `pr_evidence` gets nothing sent (no `workspace_request` read on the fake socket) and the caller gets `UnsupportedOperationError` with device, build and operation. |
| `handlers` operation, legacy | A legacy agent answering `unknown local operation "pr_evidence"` yields the same error type; another error reply is returned as `local agent: …`. |
| `handlers` cluster | A forwarded operation refused on the holding instance returns the same text and `errors.Is` on the forwarding one. |
| `handlers` status | `/api/agent/status` exposes build, sorted operations and `outdated` (false, true for a missing operation, true for legacy). |
| `handlers` macro slicing | `macroslicing_test` passes with the typed error (announced and legacy), same French message. |
| `db` adjustment | Replaces the string-injected case at `adjustment_test.go:394`: with a `pr_evidence` lookup failing with `UnsupportedOperationError`, the transition error contains the device, build, `"pr_evidence"`, "restart or update the Sectile desktop app", and neither `unknown local operation` nor `no matching`; the task stage is unchanged. |
| `desktop` unit | `agentOutdated` decision table; `fileSha256` on a temp file. |
| `desktop` UI | `settings-version.ui.cjs`: an agent answering without `binarySha256` shows the outdated mark; one answering the bundled hash shows none. |

Suites to run: `go test ./internal/agentprotocol ./internal/agent
./internal/handlers ./internal/db` (see the sandbox and PostgreSQL notes in the
project memory), `node --test desktop/tests/*.test.cjs`, and the desktop UI test
after `npx vite build`.
