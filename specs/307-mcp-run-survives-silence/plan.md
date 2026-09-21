# Implementation plan — #307

Companion to `spec.md`. Every choice below is settled; the clarification
(`docs/clarifications/307.md`, S1–S5 and D1–D5) is the authority for the ones
that were product calls.

## Stack and constraints

- Go 1.x server, `github.com/modelcontextprotocol/go-sdk v1.4.0`.
- Dual store: SQLite and PostgreSQL behind `internal/db`. **No migration**, so no
  new column and no new status value (FR12).
- No new dependency, no external service.

## Architecture

Today the inactivity bound lives in the SDK: `StreamableHTTPOptions.SessionTimeout`
arms a timer rearmed only by `sessionInfo.startPOST`/`endPOST`, and firing it
closes the session, which makes `session.Wait()` return, which makes
`SessionRegistry.Close` cancel every adopted run. The bound and the reaper are
the same mechanism, so silence cannot be observed without also being punished.

The fix separates them:

1. **`SessionTimeout: 0`** hands the SDK no bound (`streamable.go` guards both
   arming sites with `if i.timeout <= 0 { return }`). `session.Wait()` then
   returns only on a real ending — DELETE or a broken connection — which is
   exactly US4.
2. **`SessionRegistry` carries its own idle clock.** Each live session records a
   `lastSeen`; a sweeper compares it against the bound and, past it, *marks* the
   adopted runs instead of closing anything.

### Observing activity

`req.GetSession().ID()` is available on every received message, so a receiving
middleware on the MCP server is the one place that sees all client-to-server
traffic regardless of tool (FR3):

```go
s.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
    return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
        sessions.Touch(req.GetSession().ID())
        return next(ctx, method, req)
    }
})
```

Rejected: sniffing the `Mcp-Session-Id` header in the HTTP handler — it works for
Streamable HTTP only and puts transport parsing in `internal/handlers`, while
`SessionRegistry` already owns session identity.

### Marking, not cancelling

`liveSession` gains `lastSeen time.Time` and `silentSince *time.Time` (nil when
the session is not currently in a remarked silence). The sweeper, a goroutine
started with the registry and stopped by a `Stop()`/context, ticks at a fraction
of the bound and, for each session whose `now - lastSeen >= bound` and whose
`silentSince` is nil:

- sets `silentSince`, so the sentence is emitted once per stretch (FR5);
- for each adopted run, appends one sentence through a new narrow interface.

`Touch` clears `silentSince`, which is what rearms the next observation.

The append reuses `db.noteRun`'s SQL semantics — concatenate with ` — ` rather
than replace — but `noteRun` is unexported and the registry must not depend on
`*db.DB`. So, mirroring the existing `RunCloser`:

```go
type RunNoter interface{ NoteRemoteRun(runID, note string) error }
```

`internal/db` exports `NoteRemoteRun` as a thin wrapper over `noteRun`, restricted
to `skill_id='remote_run' AND status='running'` so a finished run is never
annotated after the fact. `NewSessionRegistry` keeps its signature by accepting a
`RunCloser` that may also satisfy `RunNoter` (type assertion), or — preferred for
testability — gains `NewSessionRegistryWith(runs RunCloser, notes RunNoter, bound time.Duration)`;
`NewSessionRegistry` delegates to it with the default bound and a nil noter.

Sentence shape, one line, operator-readable:

    No MCP call for 4h12m: the client may be busy or waiting for input, and this run stays open

### The bound

`internal/handlers/agent_api.go` keeps `mcpSessionTimeout()` unchanged in its
parsing (empty, unparseable or non-positive → default) but the default becomes
`4 * time.Hour` and the constant is renamed to what it now means, e.g.
`defaultMCPSilenceNotice`. Its comment — currently asserting a bridge keepalive —
is rewritten. The value is passed to the registry, not to the transport;
`StreamableHTTPOptions` keeps `JSONResponse: true` and gains `SessionTimeout: 0`
with a comment saying why the bound moved.

Wiring: the registry is built where `h.mcpSessions` is constructed (see
`internal/handlers`/server bootstrap); the bound is read there rather than in
`MCPHandler`, which is called per request.

### Rewriting a disconnected run

`disconnectNote` moves to a package both sides may import without a cycle —
`internal/models` is the existing neutral ground (`internal/taskmcp` and
`internal/db` both already import it). `taskmcp` keeps `disconnectNote` as an
alias of the shared constant so existing call sites and tests read unchanged
(FR11).

`finishRemoteRun` currently updates under `AND status='running'` and reports the
mismatch when `count == 0`. The predicate widens to:

```sql
AND (status='running' OR (status='canceled' AND summary = <disconnect note>))
```

`summary` is compared for equality because `Close` writes the note as the whole
summary; a run recovered once is no longer `canceled`, so the path cannot be
re-entered. Placeholders differ between engines — follow the existing
`d.conn.Exec` convention used throughout `internal/db` (#296/#302/#304 dual-store
rules apply: the statement must be exercised on both engines by the test).

Authorization is untouched: `FinishRemoteRunAs` already reads the activity and
applies owner-or-admin. The consequence is that the widened predicate is only
reachable by the owner or an admin, which is FR7, and `FinishRemoteRun` (no
identity, used by `Close` itself) is unaffected because it only ever finishes
`running` runs.

Hand-back: `count == 1` still gates `d.handBackRun`, and a recovered run produces
exactly one affected row, so FR9 holds with no extra code. The `count == 0`
mismatch message stays as the refusal for US3's last two cases.

## Data contracts

Unchanged. `finish_run` keeps its input schema and its `completed`/`failed`/
`canceled` vocabulary; `SessionView` gains nothing — a silence is a property of
the run's summary, not of the session projection. `MCPHandler`'s tool
descriptions need one wording pass: `start_run` and `finish_run` both claim a run
is closed when "this client disconnects", which stays true for a disconnection
but must no longer imply silence.

## Target files

| File | Change |
| --- | --- |
| `internal/handlers/agent_api.go` | `SessionTimeout: 0`; default 4h; constant renamed and its comment corrected; bound handed to the registry |
| `internal/handlers/*` (registry construction site) | pass the bound; start/stop the sweeper with the server lifecycle |
| `internal/taskmcp/sessions.go` | `lastSeen`/`silentSince`, `Touch`, the sweeper, `RunNoter`, the shared-constant alias |
| `internal/taskmcp/server.go` | receiving middleware calling `Touch`; tool-description wording |
| `internal/models/` | the shared disconnect-note constant |
| `internal/db/remoterun.go` | widened `UPDATE` predicate; `NoteRemoteRun` |
| `internal/db/chain.go` | `noteRun` reused as-is (no behaviour change) |
| `internal/taskmcp/sessions_test.go` | silence marks and does not cancel; once per stretch; `Touch` rearms; DELETE still cancels |
| `internal/db/remoterun_test.go` (or equivalent) | recovery allowed, UI-cancel refused, hand-back replayed once, both engines |
| `docs/adrs/0007-mcp-session-ownership.md` | amendment to the "silence proves death" consequence |
| `README.md` (l.354-356), `docs/contracts/server-agent-v1.md` (l.335), `.env.sample` | new meaning, new default, keepalive claim removed |
| `CHANGELOG.md` | unreleased entry |

## Rejected alternatives

- **Raise the timeout to 4h and stop there** (the ticket's palliative): leaves a
  real 4h cliff and does not recover the runs already lost. Owner chose the deep
  fix plus the rewrite (S1).
- **`ServerOptions.KeepAlive`**: with `JSONResponse: true` the ping travels on a
  standalone SSE stream a JSON-only client never opens; the session then dies
  faster (D1).
- **Client-side pings**: unenforceable on third-party clients, which are the
  population ADR 0007 exists to make visible (D2).
- **A new status or column**: dual-store migration for something a summary
  sentence already conveys (S2).
- **Any `canceled` run rewritable**: would let a late agent undo an operator's
  deliberate cancellation (S4).
