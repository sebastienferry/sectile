# Plan: embed the Sectile backend in the local companion

Date: 2026-10-29
Status: independently reviewed; separate companion and internal HTTP/WebSocket
accepted. Phases 0-3 completed. Managed Local migration and native launcher
switch are implemented and verified on macOS. Tauri now has explicit Local/Fleet
selection, one-time-code Fleet pairing, private attachment for an already paired
workstation and native Fleet setup. Pairing or attachment cannot replace an
already selected mode; paired Fleet setup validates the backend user against
the identity stored at pairing. New conversations persist the selected
runtime's connection UUID in the agent journal, and legacy conversations
require explicit association before they can resume. Live Fleet pairing/setup
qualification remains open; mode switching and data transfer are intentionally
deferred to an explicit migration design.

Product roadmap: [Sectile V1.0.0 migration](sectile-v1-migration-plan.md).
Phase 0 evidence: [baseline and contracts](phase0-runtime-baseline.md).

Independent review: [findings and disposition](local-fleet-runtime-review.md).

## Confirmed product target

- Local: Tauri plus one long-lived Go companion containing the execution agent
  and the Sectile backend. Tracker access uses personal API tokens for this
  iteration. No separately running `sectile-server` is required.
- Fleet: Tauri plus a workstation agent connected to an independently operated
  Sectile backend, on the same machine or remotely. Tracker credentials and
  supported OAuth integrations are managed by that backend.
- Provider CLIs, command guards and per-session STDIO MCP bridges may still run
  as child processes. "One companion" describes the idle service topology, not
  an artificial limit on execution subprocesses.
- Runtime topology and tracker authentication are separate axes: Local/Fleet
  chooses where business state and execution coordination live; API token/OAuth
  chooses how tracker access is authorized. A future Local runtime may use a
  remote OAuth-only service while fetching tickets and serving MCP locally.
  This does not require hosting its projects or conversations remotely. Jira's
  documented confidential-client flow would need that service for token refresh
  as well as initial exchange. Implementing such a service is outside this plan.
- Keep tracker state authoritative, local execution local, and business rules
  shared. Free conversations require no tracker configuration.
- Phase 0 preserved source, created dedicated branches, recorded contracts and
  measured the baseline. The Local runtime migration is now implemented; the
  Fleet/Electron compatibility matrix and broad rollout qualification remain.

## Findings from the current implementation

| Area | Evidence | Consequence |
| --- | --- | --- |
| Server assembly | `cmd/server/main.go` opens the DB, starts instance presence/event bus/auto-sync, configures identity, registers routes, serves assets, owns signals and drains HTTP. | Extract assembly and lifecycle, not `main` into a goroutine. |
| Agent assembly | `internal/agent/agent.go:Run` parses flags, resolves remote credentials, installs signals, starts the loopback gateway, persists runs and enters the WebSocket reconnect loop. | Export an independently startable runtime while preserving the existing CLI wrapper. |
| Agent network coupling | `startLocalProxy` forwards `/api/` and `/mcp`; desktop operations and run output also call server APIs directly. Operations and dispatch handlers accept `*websocket.Conn`. | Replacing the entire transport with direct calls at once is a substantial second refactor. Retain the wire contracts initially. |
| Workflow execution | `internal/handlers/handlers.go:NewHandler` creates the dispatcher and injects DB operation/selection callbacks; `internal/db/agentoperations.go` already defines an operation function boundary. | Useful extension seams exist, but direct execution must preserve identity, target selection, cancellation and activity attribution. |
| MCP ownership | `internal/handlers/agent_api.go` constructs `taskmcp` with a shared session registry and caller resolver. `internal/agentmcp` is a STDIO-to-HTTP bridge. | Reuse the actual server-side MCP tools; embedding the bridge alone does not embed the backend. |
| Provider compatibility | `internal/agent/companion_mcp.go` uses `os.Executable()` for child MCP invocations. Provider guards and agent restart paths also reuse the executable. | A new companion executable must support those auxiliary roles without starting another backend. |
| Database lifecycle | `db.Open` migrates, recovers interrupted runs and starts the queue worker. `DB.Close` drains jobs and stops mirror timers. Auto-sync currently loops until process termination. | Add explicit stop/join ownership before embedding; drain alone is not a complete component lifecycle. |
| Background services | Instance presence and event bus expose stop functions; MCP sessions expose `Stop`. Handler construction installs listeners and session workers. | The owner must retain and invoke every stop/unsubscribe hook, including failed startup cleanup. |
| Build boundary | `cmd/server/runtime_boundary_test.go` prohibits DB/handlers/SQLite in `cmd/agent`, and runner/workspace/terminal in `cmd/server`. | Preserve the lightweight Fleet binary through a separate composition entry point. Do not simply relax both boundaries. |
| Native launcher | `src-tauri/src/local.rs` launches server then agent, persists installation identity and performs local sign-in/pairing. Native setup calls the server while execution commands use agent discovery. | Introduce explicit runtime mode/configuration and route Local setup through the companion. |
| Exclusive ownership | `db.Open` performs SQLite interrupted-run recovery before the current server binds its port; native `runtime.lock` covers startup only. | Acquire a companion-owned lifetime store lock before DB open/recovery. A listener collision is too late to protect stored run state. |
| Connection provenance | `storedConversation` has no backend/user binding; native discovery could fall back to the shared Electron connection file. | Persist and validate connection identity, not only a directory path. Explicit Tauri connections must fail closed. |
| Credentials and storage | Backend SQLite/key, native installation credentials and agent settings/run journals have separate lifetimes and paths. | Preserve IDs and encryption bindings; combining processes does not justify combining or rewriting stores. |

## Proposed composition

Names below are proposed internal package/command names, not final API promises.

```text
cmd/server -----> internal/backend -----> db + handlers + taskmcp + trackerapi

cmd/agent ------> internal/agent -------> execution + workspace + provider CLIs

cmd/companion --> internal/companion ---> internal/backend + internal/agent

Tauri Local ----> companion private loopback API
Tauri Fleet ----> local agent gateway ----> independently operated server
```

`internal/backend` must not import `internal/agent`, workstation execution packages
or web assets. `internal/agent` stays free of DB and backend handlers. The
composition package knows both and supplies configuration and ownership.
The server entry point alone attaches the website assets and deployment adapters.

The first working Local composition may retain two authenticated loopback
listeners and a self-connection over the existing WebSocket. They live in one
process; there is no child server daemon. This removes the extra process without
simultaneously rewriting dispatch, MCP, SSE, streaming and cancellation.

A single listener and/or direct operation adapter is a later measured
optimization. Do not invent a broad transport interface or reimplement every
HTTP operation to claim an in-process design. Establish lifecycle and behavior
parity first, then decide whether the extra local socket is worth removing.

## Product and security contracts

- Local and Fleet are explicit modes, not inferred from whether a URL is
  loopback. A Fleet backend can itself be local. Never silently fall back from
  a disconnected Fleet backend to a new local board.
- Define immutable runtime configuration before component extraction: explicit
  private paths, backend identity, user/device identity, mode, listeners, MCP
  endpoint and credential references. Commands resolve environment once at their
  boundary; components must not reconfigure one another through `os.Setenv` or
  fall back to shared settings. Child environment sanitization must preserve
  only the explicitly required Local configuration.
- A stable `connectionId` binds mode, installation/backend identity and user.
  A token rotation or an explicitly verified backend address change does not
  create a different identity. Persist the binding in manifests, discovery,
  run snapshots/journals and conversation metadata. Verify server and user
  identity on reconnect; a URL string alone is not proof of backend identity.
- Native discovery for an explicitly selected Tauri connection never falls back
  to `.taskflow/agent-connection.json`. Legacy histories are adopted only with
  verified provenance; ambiguous records remain readable but cannot execute
  until explicitly reassociated. Do not infer ownership from a matching project ID.
- A companion-owned lifetime lock is acquired before any DB open, schema change,
  interrupted-run recovery or journal adoption, and held until every store user
  has stopped. It is separate from Tauri's short-lived startup/pairing lock.
  The upgrade coordinator must prove old services have stopped and the source
  stores are quiescent; PID, process name or port checks alone are insufficient.
- The desktop asks the runtime for mode, protocol version, capabilities and
  readiness. "Connected" in Local means its backend and execution path are
  ready; Fleet also reports the actual backend connection status.
- Keep the currently registered local origin on port 52963 during transition.
  A fixed port collision is an explicit error. The private agent gateway can
  retain its existing discovery-assigned port in the first composition.
- Preserve loopback-only binding, Host/Origin protections, native-only discovery
  credentials and authenticated API/MCP calls. A localhost listener is not an
  authentication boundary by itself.
- Local has one persistent installation identity with an idempotent bootstrap.
  Retain existing user/device IDs and credential encryption bindings. Do not
  force a default identity or overwrite the shared Fleet pairing configuration.
- Fleet keeps server sign-in/device pairing and authorization. Backend sign-in
  and tracker OAuth are separate features and must remain separate in the UI.
- New Local installations use API tokens for this iteration, as requested.
  Migrating the existing OAuth-enabled prototype is a separate compatibility
  decision: process embedding must not implicitly invalidate its working grant.
  The user explicitly chose continued OAuth support for this installation.
  Preserve app configuration, grant kind, encrypted records, callback, reconnect
  and refresh through an explicit compatibility capability. No forced API-token
  replacement, revocation, silent conversion or newly unusable credentials.
  Once selected, enforce the declared capability in Go and Tauri, not only the UI.
- Fleet exposes OAuth only when its backend supports/configures it. No shared
  provider client secret is bundled in either workstation executable.
- Native commands route to the selected installation; credentials never enter
  renderer persistence or command arguments. MCP remains process-scoped as in
  ADR 0011; Electron's shared registration stays untouched.
- Closing Tauri preserves running work as today. Explicit service shutdown uses
  one ordered lifecycle. Switching modes/connections is initially blocked while
  there is active or queued work; idle suspended conversations remain attached
  to their original connection and cannot resume against another backend.
- Maintain connection-scoped storage/discovery and conversation identity. Do not
  merge Local data into Fleet or reuse local tracker credentials for Fleet.

## Ordered implementation and reviewable increments

### 0. Establish a reviewable baseline in both repositories

- At initial review Sectile had substantial uncommitted prototype work on
  `feat/desktop-local-runtime`, and Tauri had no commits on `feat/tauri-bootstrap`.
  Phase 0 preserved them as commits `4576ca30` and `afa466c8` before branching;
  see the baseline report. Neither prototype is now an untracked branch base.
- Dedicated branches created: Sectile `refactor/embedded-local-backend`; Tauri
  `feat/local-fleet-modes`. The existing Go worktree is reused; isolate parallel
  implementation work if needed without omitting the captured prototype.
- A fork is not technically required. Decide separately whether upstream
  contribution or independent maintenance is intended. Do not relicense or
  copy upstream Go code into the MIT desktop repository by implication.
- Specify the `connectionId`, immutable configuration schema, lifetime lock,
  migration journal and supported protocol/version matrix before extraction.
  Define the old-discovery refusal/upgrade behavior so an older launcher cannot
  accidentally start a second writer beside the companion.
- Record baseline Fleet API/MCP behavior and Local startup/process count, idle
  memory, readiness latency and package size. Include existing failing tests
  explicitly rather than claiming a clean baseline.

Acceptance: reproducible baseline, preserved work, agreed branch base and
baseline evidence, plus agreed ownership/identity/migration contracts. No
behavioral change.

### 1. Extract backend assembly with the standalone server as its first caller

Implementation status (2026-10-29): Go commit `adce7605` on
`refactor/embedded-local-backend` adds `internal/backend` for database/handler assembly, API/MCP/auth routes,
metrics and middleware. `cmd/server` is its first consumer. Website assets,
deployment configuration, listeners and signals remain in the command. The
backend runtime closes its instance and event bus and cleans up partial open
failures. Complete worker joining, Local store exclusivity and embedded agent
ownership remain phase 2; no current installation is migrated by this change.
The focused server/backend/MCP tests and a disposable SQLite HTTP smoke test pass.
The wider phase 1 Go run reproduced the known `TestTryPullLocalAgentTasks` cleanup
race. Phase 2 found an asynchronous postback callback reaching SQLite after
close and now joins those callbacks; the full handlers suite passes.

- Move shared API route registration and middleware composition from
  `cmd/server/main.go` into an embeddable backend package.
- Separate explicit configuration from environment/dotenv resolution. Keep
  deployment config, signals, website assets and process exit in the command.
- Introduce constructor/start/stop ownership with cleanup on partial failure.
  Preserve startup order, authentication, metrics, MCP session ownership, DB
  configuration and current standalone-server behavior.
- Keep DB dialect, shared-instance presence/event bus and cluster routing
  available for Fleet. Local uses SQLite and no cross-instance listener/relay;
  retain single-instance identity/recovery services that are actually needed.

Acceptance: the existing server is the consumer of the extracted runtime and
passes unchanged Fleet API/MCP contracts before a Local consumer is added.

### 2. Make backend and execution lifecycles embeddable

Implementation status (2026-10-29): in progress on the Go branch. The backend
now holds a Local OS store lock before `db.Open`, drains queue jobs and callbacks,
joins auto-sync/MCP and agent WebSocket work, and closes SQL before releasing ownership.
The agent has a context-owned `RunEmbedded` entry point with explicit private
paths and an owner restart callback; explicit settings operations and generated
Claude settings stay within the Local root. A restart test confirms its gateway stops
first. The Fleet internal listener also has a stop/join path. The Local
backend now gates work and auto-sync until the expected authenticated agent
WebSocket connects and the embedded agent reports its connection setup. The
gate closes again if that connection disappears; a real embedded-agent test
covers startup without deadlock. Profile sign-in and agent pairing remain
available through the gate for first-run onboarding. The Local composition root
still must own listener drain, process failure and restart, so this is not yet
the phase 2
acceptance result.

- Separate agent flag parsing, signals and process restart from construction and
  execution. Keep `agent.Run` as the Fleet CLI adapter with existing defaults.
- Add bounded, idempotent start/stop/wait behavior for owned queue workers,
  auto-sync, timers, sessions, subscriptions, listeners and agent persistence.
- Keep explicit startup states: ownership acquired, store prepared, backend
  bootstrap available, authenticated internal agent connection established,
  components validated, then ready. Separate liveness from work readiness.
  Bootstrap authentication/WS must be reachable before global readiness, while
  execution, queue consumption and synchronization stay fenced. Test that the
  self-connection cannot deadlock on the readiness gate.
- Make the composition root the sole owner of process signals, fatal component
  failures and restart. Agent-requested restart becomes a request to that owner;
  it must not respawn `os.Executable()` before the backend releases its stores,
  listeners and lifetime lock. Reconstruct configuration deterministically.
- On stop: stop admitting/auto-scheduling work, terminate execution/provider
  children while postback paths remain usable, flush journals and final state,
  close MCP/transport workers and drain DB work before closing the database.
  Verify ordering against cancellation/session hooks rather than assuming it.
- Provide an ownership matrix for queue admission/producers/worker, auto-sync,
  presence/event bus, DB listeners, MCP sessions, HTTP/WS endpoints, persistence,
  provider guards and the lifetime lock. Each entry specifies stop admission,
  cancel, wait/join and deadline failure behavior. Existing `Close`/`Stop` methods
  alone do not prove completion: the queue worker has no explicit stop today,
  postback/wait listeners lack unsubscribe, and MCP stop does not expose a join.
- Test a full queue, concurrent postback, agent-requested restart, startup failure
  after each acquired resource and shutdown deadline exhaustion. A timeout must
  not close SQL under live workers or release ownership while writers survive.

Acceptance: repeated start/stop and failed startup release ports, files and
workers. SIGKILL retains existing tool-termination and journal-recovery guarantees.

### 3. Add the combined Local executable

- Compose the extracted backend and existing agent in `cmd/companion` within
  one OS process, first using fresh disposable stores only. Initially preserve
  authenticated HTTP/WebSocket contracts; leave existing installations untouched.
- Bootstrap/read the installation identity before agent connection using the
  immutable mode configuration, without inheriting deployment credentials.
- Support `mcp`, `provider-guard`, `agent-exec` and any child/restart role used by
  existing execution paths. Auxiliary invocations must not open the database
  or spawn another companion. Share role dispatch instead of duplicating it.
- Preserve the lightweight `cmd/agent` for Fleet and the standalone server for
  existing web/Electron deployments.

Acceptance: Local runs without a `sectile-server` daemon; real MCP tools,
workflows, provider prompts, questions, interruption and recovery still function.

### 4. Add explicit Local/Fleet selection and safe upgrade in Tauri

- Discover runtime capabilities from Go; start only the combined companion in
  Local, or attach/pair a lightweight agent to the selected Fleet backend.
- Persist a versioned runtime manifest and connection-scoped discovery. Adapt
  setup commands that currently assume `installation.url()` is a local server.
- Local onboarding: profile, API-token tracker, project/folder and first prompt.
  Fleet onboarding: backend URL, authentication/pairing, project and workstation
  mapping. Hide unsupported actions without losing existing configuration.
- Introduce a durable, idempotent migration journal with explicit transitions:
  prepared -> old runtime stopped -> backup verified -> candidate data adapted
  -> local validation passed -> committed. Record active/candidate/backup
  generations and schema, manifest, journal and binary versions. Recover safely
  after a crash at every transition, including power loss before/after manifest
  replacement. Never blindly retry an ambiguous step.
- Migrate only when idle and all old producers are stopped and joined. Under the
  coordinator lock, verify service ownership and quiescence, then acquire store
  exclusivity. Older launchers that ignore the new lifetime lock require an
  explicit version/discovery compatibility barrier; do not assume cooperation.
- Back up a single consistent generation: SQLite plus its checkpoint/WAL state,
  encryption key, identity/runtime/connection manifests, agent settings, run
  snapshots and corresponding conversation journals. Validate backup readability
  and encryption bindings without contacting a tracker. Never combine files
  copied at different live sequence numbers.
- Prepare and validate a candidate generation with automatic side effects
  disabled: no queue consumption, execution, sync, tracker writes, OAuth token
  refresh or MCP-triggered work before commit. Even a nominal read can rotate
  a token. Controlled validation is local; read-only remote checks come later.
- Commit the selected generation and compatible binary/manifest set durably,
  then enable normal work. Reuse original identities, folders, history and
  encryption bindings; never auto-replay interrupted tools.
- Before commit, failure can restore the verified old generation and binary set.
  After commit and release of external side effects, do not automatically restore
  a snapshot: it cannot undo a tracker write or OAuth rotation. Stop safely and
  recover forward, with an explicit recovery procedure. An old binary opening a
  newer schema is never the fallback.
- Preserve the user-selected existing-OAuth compatibility path. Any future
  removal or replacement is a separate product decision, not a side effect of
  this process-topology change.

Acceptance: existing history, folders, profiles, trackers and IDs survive;
Electron and Fleet installations remain independent; failed upgrade is recoverable.

### 5. Validate both modes before removing the old Local launcher

- Compatibility matrix: old supported agent/new server, new Fleet agent/existing
  supported server, Electron/new server, fresh Local and migrated Local. Publish
  supported version ranges and refuse incompatible handshakes visibly.
- Contract tests: same REST/MCP calls against standalone Fleet and embedded
  Local, with explicit differences only for mode capabilities and authentication.
- Real isolated tracker fixtures: import/read, workflow transitions, MCP caller
  attribution, cancellation and duplicate-dispatch protection. A fake only proves
  protocol behavior; use a consenting real tracker read separately if needed.
- Real Claude Code and Codex: response, tools, interactive questions, model/mode,
  resume, 90-second delayed-file interruption, SIGKILL and journal preservation.
- Fleet: pairing, reconnect, backend outage, device/project selection, multiple
  agents and shared PostgreSQL/cluster routing where existing suites cover them.
- Coexistence: Local + Electron + Fleet MCP reads resolve to their own identity
  and projects, with no shared-registration rewrite or credential leakage.
- Native: new-install onboarding, empty folder prompt, mode-switch guard,
  interrupted upgrade at every journal transition, occupied port, concurrent
  starts and explicit shutdown. Prove the losing starter cannot alter DB/run
  state and that an old server cannot remain a second writer.
- Provenance: same project IDs across two backends, expired/rotated credentials,
  stale discovery and an ambiguous legacy history must never dispatch work in
  another connection. Test failure instead of fallback to Electron discovery.
- Verify no external calls/jobs occur before migration commit, and no automatic
  snapshot restore occurs after remote writes or token rotation are possible.
- Build boundaries: Fleet agent excludes DB/handlers/SQLite; standalone server
  excludes workstation execution; companion excludes website assets. Preserve
  Windows hidden child processes and platform builds.
- Compare process count, idle memory, startup and bundle size against phase 0.
  One process is the topology goal; performance gains must be measured.

Acceptance: recorded results for both modes, documented limits and supported
rollback. Only then remove two-service Local startup/resources, update README,
changelog, release packaging and supersede ADR 0003 with a reviewed ADR.

## Confirmed decisions and next gates

1. Keep a separate combined Local executable so Fleet retains its current lean
   dependency boundary.
2. Accept existing loopback HTTP/WebSocket transport inside that process for the
   first delivery; defer direct dispatch/single-listener optimization.
3. Treat Local/Fleet as separate connections and stores; exclude live switching
   and automatic data transfer from this migration.
4. Keep API-token onboarding as the initial fresh Local path, independently of
   runtime topology, and preserve the current OAuth installation with its existing
   reconnect/refresh behavior. The user confirmed this compatibility choice.
5. Accept automatic snapshot rollback only before migration commit; after work
   resumes, use a defined forward-recovery procedure.

## Out of scope

Rust rewrite, a generic plugin system, a public OAuth relay, cloud account
creation, automatic Local-to-Fleet synchronization and rewriting tracker/skill
logic. The planned extraction shares the existing Go engine.

The future product boundary is defined in the V1 migration plan: Tauri owns the
primary agentic board; the headless backend retains API/MCP/auth and required
business logic. Roadmapping separation and web-client deprecations remain
independent product lots, not part of phases 0-5.

### Phase 3 qualification (2026-10-29)

The Go branch now builds `cmd/companion`, composing backend and embedded agent
in one process against an explicit desktop installation manifest. Profile and
pairing bootstrap, authenticated readiness, SIGTERM shutdown and relaunch passed
on disposable data. The Tauri launcher has since switched to the companion
through the managed, journaled migration described below.

Combined-process provider qualification (2026-10-29): real Claude Code (`haiku`)
and Codex (`gpt-6-luna`) free conversations passed execution/tool trace,
interruption, SIGKILL during a tool and explicit resumption on disposable data.
History prefixes, context, model and mode survived; both 90-second delayed tools
remained stopped beyond their deadlines and their counters showed no replay.
The Go repository records details in `docs/companion-provider-qualification.md`.
Real provider MCP calls from native Tauri are now qualified. Full tracker-ticket
workflow and Fleet/Electron coexistence matrix remain pending.

Combined MCP workflow qualification (2026-10-29): a fixture-scoped Claude pickup
workflow completed clarification, OpenSpec specification, implementation and
review using real `sectile_companion` MCP calls, Git commits/push, unittest and
MR evidence against a disposable GitLab service. An initial forge validation
failure remained recorded; resumption reused the accepted stages and MR and
finished successfully. Details and scope limits are in the Go repository's
`docs/companion-mcp-workflow-qualification.md`.

### Managed Local migration and launcher (2026-10-29)

The migration journals `prepared`, `source_stopped`, `backup_verified`,
`validated` and `committed` stages. It preserves the existing root as a verified
backup, adapts a separate candidate generation, and atomically selects the
candidate. Tests inject a failure at every durable boundary, retry, and verify
the user/device identity, encrypted credential key and legacy database fence.
The old two-process installation's server and agent are stopped only after
authenticated ownership checks; active runs and provider turns refuse migration.
After commit the old launcher cannot reopen the source database. A corrupt or
unknown selection fails closed.

The actual macOS Tauri installation migrated successfully. Ten original projects,
two trackers, the user identity and all 25 prior conversation histories were
retained. Jira account verification returned HTTP 200. Native folder-only Claude
Code and Codex prompts completed with visible tool events; their conversations
then each made a successful real `get_project_context` call over MCP. The app
starts one idle Go companion. The pre-migration files and binaries remain under
the private data root for recovery. Electron/Fleet migration and behavior were
not changed by this operation. Live source-process shutdown has only been
qualified on macOS; Linux and Windows require the old services to be stopped.

Startup serializes migration and service start. It reads only the generation
named by the committed selection and never falls back to the legacy root.
Readiness is awaited before API requests and folder prompts; failed requests
invalidate readiness so the next request can recover the companion. A new
companion MCP environment passes the embedded daemon's explicit credential to
provider children, fixing free-conversation tool calls without changing
standalone Fleet credential lookup.

### Compatibility and resource qualification (2026-10-29)

The first cross-surface check slice passes:

| Surface | Evidence | Result and limit |
| --- | --- | --- |
| Go Fleet/server boundaries | `go test ./cmd/agent ./cmd/server` | Pass; both independent entry points still build and server tests pass. This is not the full old/new client-version matrix. |
| Go companion and shared runtime | `go test -race ./cmd/companion ./internal/companion ./internal/agent ./internal/backend`; 2026-10-29 cross-builds of `cmd/companion`, `cmd/agent` and `cmd/server` for `linux/amd64` and `windows/amd64` | Pass on macOS host; all six outputs have the expected ELF/PE x86-64 formats. This proves Go compilation only; Linux/Windows Tauri packaging, startup, migration, execution and process-window behavior remain unqualified. |
| Go child-process credentials | `go test ./internal/runner ./internal/agent ./cmd/agent ./cmd/companion`; Windows builds of `cmd/agent` and `cmd/companion` | Pass; the local desktop gateway credential is removed from spawned CLI and shell environments. This is a compile/unit check, not Windows process-window verification. |
| Electron renderer | `desktop/npm test` | 315 tests pass after installing the lockfile dependencies. No Electron UI or live shared-MCP registration was changed in this work. |
| Tauri renderer/native layer | `npm run check`, `npm test`, `npm run test:ui`, `npm run test:rust`, `npm run build` | Pass on the current branch: 9 domain tests, 87 UI tests and 35 Rust tests. The Rust tests cover explicit Local/Fleet file routing, private credential storage, Fleet URL policy, pairing HTTP contract handling, account identity binding, selected-runtime replacement rejection, runtime-mode status and durable connection UUID migration. |
| Tauri Fleet | Pairing contract fixture plus an isolated macOS bundle pointed at the existing Fleet workstation agent | The contract fixture covers pairing request/response and errors. The live Tauri dashboard loaded 19 projects from the v0.7.6 agent and displayed the Fleet backend identity without writing remotely. One-time-code pairing, the native attach button, Fleet profile/API setup and OAuth on the live backend remain unqualified. Conversation identity and explicit legacy association are covered by local Go/Rust/UI tests, not a live Fleet session. |
| Existing native Local install | Managed macOS migration verification and real Jira/Claude/Codex/MCP requests | Pass; details above. |
| Existing Electron/Go Fleet connection | Read-only calls to the existing workstation agent's authenticated `/desktop/status`, `/desktop/version` and `/desktop/projects` routes | The agent reports v0.7.6, connected, no contract error, and 19 projects (2026-10-29). This verifies the existing connection only; no Tauri mode or server data was changed. |
| macOS release bundle | `npm run tauri -- build --bundles app --no-sign` | Pass on the current commit (2026-10-29); the unsigned release `.app` is 101.72 MiB versus the recorded 133.1 MiB debug app. It contains about 95 MiB of Go binaries: companion (40 MiB), legacy server (36 MiB) and legacy agent (20 MiB) for migration/recovery. This is an unsigned macOS app bundle, not a release artifact or a cross-platform size comparison. |
| macOS startup | Isolated unsigned optimized app under a separate bundle ID; first native window timed independently from UI readiness | On arm64 macOS 27.0.1, the initial launch showed its first native window at 1.09 s. Three subsequent process relaunches with the same unconfigured profile and warm filesystem cache took 207–245 ms to first window. The setup screen was manually accessible by about 3.5 s on the initial launch (upper bound from a two-second observation interval); this was not automated UI-ready timing. At 33 s, initial-launch app RSS was 94.7 MiB and no companion was running. These samples do not establish a startup SLA or true repeated cold-boot performance. |

The idle Go service process count is now one. At the earlier captured idle sample,
`sectile-companion` used about 40.7 MiB RSS, compared with the phase 0 server and
agent medians totaling 55.7 MiB. The Tauri desktop process used another 62.2 MiB
and was excluded from the Go-service comparison; this does not establish a total
desktop memory saving. A later one-shot process snapshot measured the active Tauri
debug process at 55.3 MiB RSS and the companion at 40.2 MiB RSS; the conditions
were not controlled, so this is a point-in-time observation, not a baseline.
The rebuilt debug `.app` occupies 133.1 MiB versus the 92.9 MiB phase 0 debug
app, an increase of about 40.2 MiB. It bundles the companion plus the legacy
server and agent migration executables. A signed release artifact and repeated
true cold-start measurements are still required before claiming a
smaller or faster delivery; the process-relaunch samples above use a warm
filesystem cache.

The isolated native service test was attempted after preparing fresh Go binaries,
but its fixed port reservation failed because port 52963 was occupied. The
existing service was left running. The test must be repeated after that service
stops.

Both Go binary handshake directions now pass using the reproducible harness
`python3 scripts/compatibility/test-v076-server-current-agent.py` from the Go
repository: the released v0.7.6 agent connects to the current handler, and the
current agent connects to a server built from the v0.7.6 tag. The harness also
starts both server binaries against isolated temporary databases with a seeded
project/task, then exercises each server with the other version's MCP SDK
client. Both clients list the 21-tool catalog, call `list_projects`, create a
GitHub issue through a mock API using the personal tracker token, transition
that remote task to `clarified`, verify its tracker sync, start and finish a run
on a local task, and read back the persisted task and run statuses. The mock
observed one authenticated issue creation and one authenticated stage update
from each server version. This proves MCP session/catalog compatibility,
caller-token attribution, one tracker stage update and a workflow transition in
both directions. The harness also calls the authenticated project
`skills-status` route with each opposite-version agent connected. The operation
crosses the server-agent WebSocket and returns the expected temporary Git
workspace in both directions. The harness then removes that temporary
repository's `.git` metadata, invokes the native `init-git` API through each
server/agent pairing and verifies Git initialized the expected path; it restores
the original temporary metadata afterward. This qualifies one mutating agent
operation. Task worktree operations, cancellation, provider execution, live
GitHub/Jira/GitLab behavior and Electron registration remain unqualified. Stage 5 remains
open for a live Fleet pairing/setup session, an Electron live MCP call after the
Tauri migration, signed-package qualification and repeated true cold-start
measurements. The qualification host currently has zero valid macOS
code-signing identities. Do not infer these from unit tests or
from the migrated Local installation.

Existing workstation connection qualification (2026-10-29): the saved local
agent connection was used for read-only authenticated calls. The v0.7.6 agent
returned `connected: true`, an empty `contractError`, and 19 projects. This is
not evidence of Tauri one-time-code pairing, project creation through the new
API route, or a live MCP call after Local migration.

Fleet pairing contract qualification (2026-10-29): the Tauri native exchange
was extracted behind a small HTTP boundary and verified against a loopback
fixture. The tests assert the POST path, one-time-code payload, device/user/API
credential decoding, expired-code error, incomplete success payload and backend
error. They do not redeem a real Fleet code, validate remote TLS deployment, or
prove a paired workstation's reconnection behavior.

Release bundle measurement (2026-10-29): a clean optimized Tauri build completed
with signing disabled and produced a 101.72 MiB macOS `.app`. The bundle still
contains all three Go executables, including the previous two-service pair, so
the measurement quantifies the current migration-capable package and does not
claim the final Fleet or V1 distribution size. The active desktop remained open;
no release app was launched over its data directory. The isolated cold-start
probe used a separate app identifier and fresh profile; the first native window
appeared at 1.09 seconds, and the setup UI was accessible by about 3.5 seconds
(one manual upper-bound observation). At 33 seconds, process RSS was 94.7 MiB;
no companion was started. Three subsequent relaunches produced first-window
times of 207–245 ms with the filesystem cache warm. They do not replace repeated
true cold-start measurements; a signed artifact also remains open. Source-level
protocol fixtures pass in both directions: the current server parses the pinned
v0.7.6 agent connection query, and a parser copied from the v0.7.6 server
source accepts the current agent announcement. Separately, a real v0.7.6 agent
binary connected to the current server WebSocket handler in an isolated test
with a temporary database and HOME; the server accepted its version and complete
operation list. The reproducible Go harness also starts a v0.7.6 server from an
isolated SQLite database and accepts a current `dev` agent with all 27 operations
and `outdated=false`. Both results qualify handshake compatibility only; tracker,
MCP and workflow API parity remain open. Windows cross-compilation also passed
for the Go companion and handler test binary; Windows runtime and Linux native
execution remain untested.
