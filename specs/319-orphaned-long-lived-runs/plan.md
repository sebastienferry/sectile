# Plan - #319

## Design

1. **Registry** (`internal/taskmcp/sessions.go`). A `RegistryConfig` carries the
   sinks and both bounds; `NewSessionRegistryWith` keeps its signature and
   defaults the abandon bound. `liveSession` keeps the `*mcp.ServerSession` it
   was watched with. The sweep that already remarks on silences also collects the
   sessions past the abandon bound; outside the lock it calls `Close` (which
   cancels the adopted runs with `RunDisconnectNote`) and then closes the SDK
   session, whose watcher's own `Close` then finds nothing left. The sweep period
   stays a tenth of the silence bound, which is also at most a tenth of the
   abandon bound.
2. **Bound parsing** (`internal/handlers/agent_api.go`).
   `mcpAbandonAfter(silence)` reads `SECTILE_MCP_SESSION_ABANDON_AFTER` with the
   same rules as `mcpSilenceNotice`, then raises it to the silence bound.
3. **Rewrite predicate** (`internal/db/remoterun.go`, `macroruns.go`).
   `LIKE '%' || note || '%'` instead of a prefix. The note holds neither `%` nor
   `_`. A cancellation someone typed never contains that sentence.
4. **Silent state** (`shared/runStates.ts`). A `silent` entry (label "Silent",
   announcement "has gone silent", slate-violet, circle-pause glyph).
   `RunStateInput` gains an optional `summary`; `runStateOf` returns `silent` for
   a running run whose summary contains the prefix, after the waiting check. The
   desktop run list carries no summary, so the desktop is unaffected and raises
   no banner for it. `remoteRunIndicator` adds `silent` to its precedence;
   `RemoteRunBadge` and `ActivitiesView` get the label, the class and the
   translation.
5. **Board Close.** `deriveRunIndicator` takes an optional viewer `{userId,
   role}` and returns `closableRunIds`: client-created runs owned by the viewer,
   or any client run for an admin. `RemoteRunBadge` renders a Close button when
   nothing is stoppable and something is closable, posting to the existing
   `/api/tasks/{id}/cancel-run`. `handleCancelRemoteRun` gains a branch for
   `RunActionClient`: `requireOwnerOrAdmin`, then `FinishRemoteRunAs(...,
   "canceled", RunDisconnectNote)`, then `mcpSessions.ReleaseRun`.
6. **Activities cancel** (`internal/handlers/handlers.go`). For a running
   client-created task run: `requireOwnerOrAdmin`, `FinishRemoteRunAs(...,
   "canceled", "Execution canceled from the activities view")`, `ReleaseRun`;
   `ErrRunNotYours` maps to 403. Every other activity keeps `CancelActivity`.
   The web `cancelActivity` shows the server's message on failure.

## Rejected alternatives

- A new `orphaned` status: the tool contract accepts three terminal statuses, and
  a new value would have to be handled by every filter and store (as ADR 0012
  argued for waiting).
- A silent column cleared on the next call: a migration, which the owner declined.
- Cancelling on the abandon bound without closing the SDK session: a client that
  comes back would be served under a registry that forgot it.

## Target files

`internal/taskmcp/sessions.go` and tests, `internal/handlers/{agent_api,remote_run,handlers}.go`
and tests, `internal/db/{remoterun,macroruns}.go` and tests, `shared/runStates.ts`,
`web/src/lib/remoteRunIndicator.ts`, `web/src/components/{RemoteRunBadge,ActivitiesView}.tsx`,
`web/src/locales/translations.ts`, `web/src/context/AppContext.tsx`, web tests,
`docs/adrs/0007-mcp-session-ownership.md`, `docs/contracts/server-agent-v1.md`,
`README.md`, `.env.sample`, `CHANGELOG.md`.
