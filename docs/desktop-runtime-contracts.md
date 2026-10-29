# Local/Fleet runtime contracts: phase 0

Date: 2026-10-29
Status: approved design baseline; phase 1 implemented and phase 2 lifecycle work in progress.

## Approved scope and ownership

The user approved a separate combined Local executable and preservation of the
existing internal HTTP/WebSocket contracts for the first delivery. The existing
OAuth connection must continue working, including browser reconnect and refresh.
No migration, runtime policy change or service restart is part of phase 0.

`cmd/server` remains the Fleet backend. `cmd/agent` remains the lean Fleet
workstation executable, with no dependency on DB, handlers or SQLite.
A proposed `cmd/companion` composes backend and agent packages in one process.
Only the composition root imports both. Server web assets stay outside the
shared backend. The existing dependency-boundary test remains an acceptance gate.

Runtime topology and tracker authorization are independent:

| Topology | Business state and MCP | Tracker authorization |
| --- | --- | --- |
| Local | Embedded backend on the workstation | Personal API token initially for fresh installations; preserve the current configured OAuth installation in compatibility mode |
| Fleet | Selected independent backend | Capabilities offered by that backend, including configured OAuth |

A future remote OAuth-only service could authorize a Local runtime without moving
its tickets, workflows or MCP to a remote backend. It would participate in Jira
refresh as well as code exchange. That service is outside this extraction.

## Product boundary for V1

The user intends Tauri to replace Sectile's main agentic board experience. The
backend's deployable surface becomes API/MCP/auth, retaining the workflow,
tracker, authorization and persistence logic behind those interfaces. Roadmapping
features may move to another tool after an explicit ownership/consumer inventory.
This direction does not authorize deleting current web routes, Electron support
or macro/workflow data during extraction. Headless packaging and roadmap
separation are independent workstreams. The product-level V1 migration plan lives
in the Tauri repository at `docs/sectile-v1-migration-plan.md`.

## 1. Stable connection identity

The new design distinguishes these identifiers:

- `connectionId`: an opaque UUID allocated for a verified connection and retained
  across restart, token rotation and the current two-process-to-companion upgrade.
- `backendId`: a persistent identifier of the backend data installation, not the
  existing per-process `DB.InstanceID()` and not a URL-derived hash.
- `userId`: the backend principal already owning the profile, projects and
  encrypted credentials. Keep it unchanged during Local adoption.
- `deviceId`: the workstation identity used for execution selection and auditing.
- `generationId`: the selected consistent data/configuration generation; changes
  only through a journaled upgrade or explicit recovery.
- `runtimeId`: the ephemeral process identity used to distinguish a running
  owner from stale discovery. It is never a durable conversation identity.

A connection registry binds `connectionId` to mode, backend identity, user,
permitted origin, private data root and credential references. Discovery and run
snapshots/journals carry this binding. Project IDs and provider session IDs alone
are insufficient provenance. Credential material is never used as an identifier.

### Authentication and adoption rules

- A claimed `backendId` is an assertion, not proof of identity. Authenticate the
  selected endpoint using the existing trusted pairing/TLS boundary in Fleet or
  the private owned-runtime bootstrap in Local before accepting its metadata.
- Do not send a stored Fleet bearer credential to an unapproved different origin
  to discover whether the new endpoint is the same server. Origin changes require
  explicit verification/rebinding and a separately authenticated connection.
- Add durable backend metadata to the new backend without changing the existing
  agent dispatch schema. A supported older Fleet backend without that metadata
  is a pinned legacy connection: retain verified origin/user provenance and
  forbid automatic endpoint migration. Do not invent a backend identity from a
  mutable URL or confuse a process instance UUID with one.
- Legacy Local histories may be adopted only after the stopped installation's
  manifest, private discovery, backend user/device records and run directory are
  verified as one provenance chain. Add binding metadata without changing run
  IDs, provider sessions, journal order, model, mode or directories.
- Ambiguous histories remain readable; resumption requires explicit association.
  Never infer the association from a project UUID or a matching project name.
- A selected Tauri connection never falls back to the shared Electron discovery
  file or global pairing. Connection failure is reported for that connection.
- Local/Fleet changes select separate connections and stores. They do not migrate
  tickets, histories or credentials between backends. Active/queued work blocks
  switching; an idle resumable conversation remains bound to its old connection.

## 2. Immutable runtime configuration

The command boundary resolves environment/flags once, validates them and creates
an immutable configuration. Components receive that configuration explicitly.
No `os.Setenv` mode switch, inherited deployment fallback or shared-directory
fallback is allowed in the new Local composition.

Required configuration groups:

| Group | Required content |
| --- | --- |
| Identity | Mode, connection/backend/user/device IDs, generation, credential references |
| Store | Explicit SQLite and encryption-key locations, settings directory, run snapshot/journal root |
| Transport | Owned backend bind address, separate private agent gateway/discovery, trusted upstream origin in Fleet |
| MCP | Endpoint and per-process alias, caller/run binding, scoped child credential delivery |
| Authorization policy | Declared tracker capabilities and compatibility OAuth configuration, independent of topology |
| Lifecycle | Root context, admission state, shutdown deadline, restart ownership and logging destinations |

Resolve Local paths under the explicit installation root and verify ownership
and canonical targets before use. Never redirect to `~/.taskflow` on a missing
value. Preserve the existing data subdirectories in the first migration rather
than unnecessarily moving credential and journal files.

The current backend origin `http://127.0.0.1:52963` and registered OAuth callback
remain stable. The second private gateway can keep discovery-based allocation.
Port collisions fail without changing the selected connection or callback.

Keep native credential storage private. The renderer receives status/capabilities
and presence flags, never a saved token, client secret or client ID. Auxiliary
roles (`mcp`, `provider-guard`, `agent-exec`) are selected before DB initialization;
a bridge invocation must neither acquire the daemon store lock nor start a new
backend. Child environment delivery remains sanitized and explicit.

## 3. Readiness, protocol and compatibility

Keep version domains separate:

| Domain | Baseline | Proposed change |
| --- | --- | --- |
| Fleet agent configuration | `agentconfig.Version = 1` | Keep unchanged during extraction; additive metadata must not alter dispatch semantics |
| Private companion discovery | Protocol 1, ready flag | Protocol 2 handshake identifying mode, capabilities, connection/backend/runtime/generation and component compatibility |
| Native installation manifest | Current unversioned installation JSON | Version 2 with explicit mode, identity binding, generation and executable/data compatibility |
| Conversation persistence | Existing snapshots and synchronous journals | Add a versioned identity binding with controlled legacy adoption |
| Database schema | Existing numbered migrations | Continue through the same migration owner; no second schema history |

The proposed protocol numbers are contract design, not changes already deployed.
A new launcher recognizes legacy protocol 1 explicitly; it does not mistake it
for the embedded companion. An old launcher must not accept protocol 2 as its old
server and start a second agent. Test the actual known old launcher behavior,
including its port reservation, before depending on that barrier. Arbitrary
manually launched legacy binaries are outside the managed-launch guarantee.

Startup states are ordered: exclusive owner -> stores prepared -> bootstrap
available -> internal agent authenticated -> local validation -> work ready.
Liveness is not work readiness. Bootstrap HTTP/WS must work before the final
ready state, while mutating MCP/API work, queue consumption and auto-sync remain
fenced. Readiness failure never triggers a fallback connection or an empty board.

Fleet compatibility is a tested matrix, not a blanket version-range claim:

1. Preserved baseline agent against extracted/new server.
2. New Fleet agent against the supported baseline server (agent contract 1).
3. Installed Electron against the new server, including shared MCP.
4. Fresh Local companion and upgraded current Local installation.
5. Older Tauri launcher versus new private discovery: safe refusal.

Pin exact artifacts/commits in test evidence before declaring a supported range.
Remote MCP success alone does not prove distributed PostgreSQL/cluster behavior;
that remains covered by the dedicated suites and a configured integration setup.

## 4. Lifetime ownership and stop/restart

A companion lifetime lock is separate from native startup/pairing coordination.
Acquire it before any DB open, migration/recovery or journal adoption and retain
it until all store users terminate. Reserve required listeners before enabling
work. Concurrent-start losers must not write to the database or journals.
The native upgrade coordinator also holds the existing startup lock while
stopping/validating the known old services; it cannot assume old binaries honor
a newly introduced lifetime lock. Do not force-remove a lock based on a PID file.

The composition root owns signals, fatal component errors, restart and cleanup.
A component failure makes the runtime unready and stops admission. An agent
restart request asks this owner to stop the whole composition before respawning;
it never independently races a still-running embedded backend.

| Component | Stop admission/cancel | Completion required before SQL close/ownership release |
| --- | --- | --- |
| API/MCP mutation admission | Refuse new work; keep final-state/postback paths available | All accepted requests finish or are cancelled |
| Auto-sync and producers | Stop timers and reject enqueue after the fence | Producers joined; no late queue sends |
| Queue worker/jobs | Stop consumption as required by migration; cancel/drain accepted work during shutdown | Worker and active jobs joined, including a full queue |
| Agent execution/providers | Refuse launches; stop owned process trees via existing guards | Process termination verified; final events/postbacks persisted |
| Conversations/run store | Stop turns and pending inputs; checkpoint synchronous journals | All journal/snapshot writers joined; no replay on recovery |
| DB callbacks/event subscribers | Stop delivery after final state and unsubscribe safely | No postback/wait callbacks can access the closed store |
| MCP sessions/HTTP/WS | Close after execution finalization; stop sweep/keepalive workers | Session and transport workers joined, listeners closed |
| Presence/event bus | Preserve required final attribution, then stop loops | Returned stop hooks completed; no cluster listener in Local |
| SQLite/key/store lock | Close only after every user above | SQL close confirmed; key retained; lifetime lock released last |

`Stop(ctx)` is bounded by a single owner deadline, not an unbounded series of
fresh component deadlines. Preserve Fleet command shutdown defaults initially.
A timeout cannot be reported as a successful stop while writers remain alive.
Escalate owned child termination and fail closed; a fatal process exit must retain
recoverable journals rather than allow a second runtime beside live writers.

Required tests include partial startup after each acquisition, full queues,
concurrent postback, agent-initiated restart, WS startup gating, repeated stop,
shutdown deadline and SIGKILL with a delayed tool side effect.

## 5. Durable migration transaction

Scope: the current managed two-process Local installation into the combined
companion, preserving identity and OAuth. Local-to-Fleet data transfer is excluded.
Migration inspection and backup outputs are private data, never repository files.

The durable journal carries migration ID, source/candidate generation IDs,
compatible binary/schema/manifest/journal versions, backup integrity metadata and
last completed stage. It contains no token, key, auth code or provider output.

Stages:

1. `prepared`: validate the manifest and source ownership; refuse active work or
   pending operations. This includes backend jobs, provider tools and unresolved
   interactive execution, not merely the desktop's selected conversation.
2. `source_stopped`: fence scheduling; stop the agent while final postbacks remain
   available, drain the backend, verify both known services stopped, and obtain
   exclusive store ownership. Keep startup coordination held throughout.
3. `backup_verified`: snapshot one stopped generation containing SQLite/WAL or a
   verified checkpoint, encryption key, identity/connection manifests, settings,
   run snapshots and matching journals. Verify private permissions, hashes and
   local decryptability; retain original compatible executable artifacts.
4. `candidate_adapted`: prepare a separate candidate generation, apply normal
   schema migrations and identity binding with automatic effects disabled.
   Retain original user/device/run IDs and encryption associated-data bindings.
5. `validated`: verify data/history/journal coherence, private authenticated
   bootstrap and compatible configuration locally. No tracker calls, OAuth
   refresh, background queue consumption, execution or mutating MCP operations.
6. `committed`: atomically and durably select a manifest containing the committed
   generation and compatible binary/configuration set. This manifest replacement
   is the commit point; if the progress journal lags, the committed manifest wins.
7. Enable ordinary work and record completion. Remote verification happens only
   after commit; even a nominal read may refresh an OAuth token.

Before the commit point, recover idempotently to the old verified generation or
resume the unfinished candidate. After the committed manifest exists, recover
forward; never automatically restore a snapshot that could roll back refresh
rotation or duplicate remote work. A crash after manifest commit but before the
journal update is still postcommit. Test every transition, file replacement and
mismatched/incomplete generation, not only a graceful restart.

Do not open a newer schema with an old executable as a fallback. An old service
restarted after a supported precommit rollback uses its matching saved binary
set and exact source paths. Do not import or overwrite Electron's installation.

## 6. OAuth preservation is an acceptance condition

For the existing installation preserve the encrypted application/grant records,
key, principal, registered callback and personal credential kind. Keep code
exchange, account/site verification, reconnect and rotating refresh functional.
Do not require another personal token merely because processes are combined.
No vendor secret is added to a distributable binary.

Fresh Local onboarding can start with API tokens. The compatibility capability
for the current configured OAuth installation is explicit and survives restart;
it is not inferred solely from whether a server URL happens to be loopback.
Fleet retains the existing provider configuration/authorization behavior.

Before migration, record a live authenticated Jira account read and isolated MCP
reads without leaking credentials. During candidate validation, test OAuth record
integrity only locally. After commit, verify reconnect/refresh/read with fixtures
and a consenting live account as appropriate. A snapshot containing a pre-refresh
secret is not a postcommit rollback mechanism.

## Phase boundaries

Phase 0 records these contracts and a reproducible prototype baseline. Phase 1
extracts API/MCP/auth route registration, middleware, metrics and database/handler
assembly into `internal/backend`, with the standalone server as its first caller.
The server command still owns deployment configuration, web assets, listeners and
signals. The backend runtime has a close path and cleans up partial open failures;
the complete stop/join and exclusive Local store ownership contract remains phase
2 work. No combined executable, connection identity or migration is implemented
by phase 1. Independent plan review findings remain acceptance gates, not
completed tests.

Phase 1 verification: `go test` passed for `cmd/server`, `internal/backend`,
`internal/taskmcp` and `internal/agentmcp`; a built server started on disposable
SQLite, passed readiness and local sign-in, served an authenticated project read,
and rejected unauthenticated MCP with HTTP 401. The wider Go run passed its other
selected packages but `internal/handlers` reproduced the previously observed
`TestTryPullLocalAgentTasks` temporary-directory cleanup race; that test passed
in isolation. Phase 2 traced this to asynchronous postback listeners reading
SQLite after close and added callback joining. The full handlers suite now passes.

Phase 2 implementation in progress: `internal/storelock` provides an exclusive
OS lock acquired by `backend.Open` before Local SQLite is opened and released
only after SQL closes. Its tests cover contention, reacquisition, startup
failure and a killed owner; Windows compilation is checked. `DB.CloseContext`
stops auto-sync, cancels pending mirror timers, drains queued jobs and database
callbacks, joins the queue worker and closes SQL last. MCP maintenance and
agent WebSocket/pull work have stop/join paths, and the Fleet internal listener
has an owned stop function.
The agent has an explicit `RunEmbedded` entry point that uses a caller context,
private settings/discovery paths and a restart callback instead of installing
signals or self-execing. A restart test checks that the gateway stops before
the owner callback runs. Explicit-path settings reads, writes, migrations,
repository mappings and generated Claude settings keep the embedded runtime
out of the standalone agent's global settings file. These are reusable
lifecycle primitives, not a Local companion yet. Work admission, authenticated
self-connection/readiness gating now exist as backend primitives: Local HTTP
admits health, profile/pairing setup and authenticated agent bootstrap endpoints
until the expected user/device WebSocket is registered. Fleet keeps its existing
startup path. The embedded agent reports completion of its connection setup
to its owner. Local auto-sync and task pulls start only after activation; a
lost agent connection closes the user-facing work gate again. Tests exercise
the gate with first-run profile pairing, a WebSocket fixture and the real
embedded agent. The composition root still has to coordinate those signals,
HTTP listener drain, provider process termination and full startup/failure
tests. No installation
has been migrated.

### Phase 2 ownership inventory

| Resource | Admission and stop | Join and deadline behavior |
| --- | --- | --- |
| Local store lock | Acquired before SQLite open; released after SQL close | Retained when a backend join fails; process death releases the OS lock |
| Database queue and callbacks | Stop accepting after active work reaches idle | Join the queue worker and callbacks before SQL close; timeout leaves SQL open |
| Auto-sync and mirror timers | Stop ticker and cancel pending timers | Wait for active sync and callbacks before SQL close |
| Handler WebSocket and pull work | Begin drain and close agent connections | Join counted background work before backend close |
| MCP session maintenance | Stop sweeper and keepalive | Wait for both loops before backend close |
| Agent gateway and run journal | Cancel the agent context and close the gateway | Join gateway and persistence loops; provider exit failures are returned |
| Provider processes | Stop conversations while postback paths remain available | Full deadline and crash-path validation remains pending |
| HTTP request admission and Local self-connection | Local gate allows health and authenticated agent bootstrap until activation; closes again on agent loss | Real embedded connection test passes; listener drain and startup-failure tests remain for the composition root |
