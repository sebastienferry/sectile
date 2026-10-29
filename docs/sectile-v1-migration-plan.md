# Sectile V1.0.0 migration plan

Date: 2026-10-29
Status: product direction confirmed; Local companion runtime and managed upgrade
are implemented on dedicated branches. This is not a V1 release: compatibility,
product inventory, UX convergence and release qualification remain open.

## 1. Product destination

Tauri becomes the main interface for Sectile's agentic work: project/workspace
prompts, the operational ticket board, skill/workflow execution, conversations,
interactive decisions and execution supervision.

The Go backend provides a headless application surface: API, MCP and
authentication. Behind those surfaces it retains the domain logic required for
tracker synchronization, workflow rules, execution coordination, authorization,
state and background jobs. "Headless" does not mean removing that logic or
reimplementing it in React.

Broader product planning and roadmapping are candidates for another tool.
V1 does not aim to port every Electron/web feature into Tauri. The exact external
roadmapping tool and its integration contract remain to be selected.

This document is the product-level migration roadmap. The reviewed technical
subplan is [Local/Fleet runtime extraction](local-fleet-runtime-plan.md), with
its [independent review](local-fleet-runtime-review.md). Sectile's canonical
phase 0 engineering contracts are in its Go repository at
`docs/desktop-runtime-contracts.md` and ADR 0066.

## 2. Responsibility boundaries

| Capability | V1 owner/direction | Migration constraint |
| --- | --- | --- |
| Operational agentic board | Tauri | Show remote tracker state and actual workflow/execution state; no independent task truth |
| Prompts, conversations, tools, questions and supervision | Tauri + local execution agent | Preserve model/mode, provider context, journals, interruption and resume |
| Local filesystem, Git, CLIs and process trees | Go execution agent | Keep execution local and retain process guards/Windows behavior |
| Tracker integration and workflow decisions | Shared Go backend library | Reuse existing adapters, identity attribution and business rules |
| Agent-facing tools | Go MCP surface | Same business operations and caller/run binding in Local and Fleet |
| Authentication and credential lifecycle | Go authorization components | Preserve current Jira OAuth; keep runtime topology independent from auth method |
| Product strategy, portfolio and long-term roadmaps | External tool, integration to be defined | Inventory existing consumers/data before any extraction or removal |
| Current web/Electron clients | Compatibility consumers during migration | No silent API deletion, shared MCP takeover or automatic credential migration |

High-level work items, macros and specifications can participate in execution
workflows as well as planning. Do not classify them as removable solely by name.
Trace their use by skills, MCP tools, workflows and clients before assigning
ownership. Preserve referenced IDs and links if another tool later owns planning.

## 3. Two runtime modes, separate authentication choices

### Local

```text
Tauri -> one managed Go companion
           |- embedded backend: API, MCP, trackers, workflow state
           `- execution agent: Git, providers, tools, conversations
```

The companion is a separate executable from the lightweight Fleet agent.
Backend and agent initially retain HTTP/WebSocket exchanges within that one
process. Idle topology is two application processes: desktop and companion.
Provider/guard/STDIO bridge child processes remain normal during execution.

Fresh Local onboarding starts with personal API tokens. The current configured
OAuth installation retains its working login, reconnect and refresh capabilities
through the migration. Local business data remains local.

### Fleet

```text
Tauri -> local Fleet agent -> independently operated Go backend
                                API + MCP + auth + business state
```

The backend can be on the same machine or remote; mode is explicit and is not
inferred from loopback. Keep the agent lightweight and retain supported pairing,
authorization, reconnect, targeting and multi-instance behavior.

### Authentication is orthogonal

- API token: Go validates/stores it privately and calls the tracker.
- Fleet OAuth: the selected backend manages the configured provider integration.
- Future Local OAuth through a remote auth-only service: authorization may depend
  on that service while ticket fetches, workflows, execution and MCP stay local.
  Jira's confidential-client flow also needs the service for token refresh.
  Storage/custody of refresh tokens, device authentication, hosting and revocation
  must be specified before implementing that service.

A remote OAuth-only service is optional future scope, not a prerequisite for the
first Local companion. No shared provider client secret is shipped in a desktop
binary. The already configured OAuth prototype remains supported.

## 4. Ordered migration

| Stage | Deliverable | Exit condition |
| --- | --- | --- |
| 0. Capture and contracts | Prototype commits, dedicated branches, identity/configuration/lifecycle/migration contracts and baseline evidence | Reproducible source baseline; current OAuth/MCP still work; no runtime changes |
| 1. Shared backend assembly | Reusable Go package first consumed by the existing standalone server | Fleet contracts unchanged; website assets remain outside the shared package |
| 2. Lifecycle ownership | Explicit construction, admission, stop/join, restart and store exclusivity | Concurrency, partial startup and crash tests prove stores are not shared unsafely |
| 3. Fresh Local companion | One Go process combining backend and execution on disposable data | Full prompt/workflow/MCP behavior works before existing data is touched |
| 4. Managed upgrade and native modes | Connection-bound Local/Fleet configuration; journaled upgrade of the prototype; native setup in both modes | Histories, folders, identities and current OAuth preserved; precommit rollback tested; Fleet pairing/setup qualified; pairing or attachment cannot silently replace a selected runtime; no implicit transfer |
| 5. Compatibility and measured delivery | Local/Fleet/Electron test matrix, packaged artifacts and performance comparison | Required behavior and supported versions documented before removing old Local startup |
| 6. Product/UI transition | Tauri becomes the primary agentic board; a headless backend distribution is defined | Operational features and existing consumers have an explicit migration path |
| 7. Roadmapping separation | Ownership inventory and an agreed external-tool contract | Data/links and remaining clients preserved; deprecations reviewed before removal |
| 8. V1 release qualification | Agreed support/migration/deprecation boundaries and operational documentation | Release checklist and explicit release approval completed |

Stage 0 is complete. Stages 1-3 are implemented on the dedicated Go branch: the
standalone server consumes the extracted backend package, the companion owns
backend/agent lifecycle, and real Claude Code, Codex and MCP workflows passed on
disposable data. The Local Tauri launcher now migrates its managed two-service
installation into a selected companion generation, preserving the database,
OAuth key, identities, projects, trackers and idle conversation histories. The
migration was exercised against the existing desktop installation on macOS;
Jira verification and native Claude/Codex prompts plus MCP reads succeeded after
the switch. Stage 4 has explicit Local/Fleet selection; Tauri can pair Fleet with
a one-time code or privately attach an existing, already paired agent after
validating its live connection. The shared Electron discovery file is no longer
a silent fallback. Paired Fleet connections now expose native profile, personal
tracker credential, project and OAuth account setup through the selected backend;
OAuth app secrets and shared tracker administration remain server-admin tasks.
Stage 4 remains open for one-time Fleet pairing/setup qualification and
conversation-history identity adoption. A separate-ID Tauri bundle has now
connected read-only to the existing Fleet workstation agent and loaded its 19
projects; new one-time-code pairings also verify the backend user's ID against
the identity recorded at pairing. Runtime switching is deliberately
unsupported: selecting a second mode requires a future explicit migration flow.
Stage 5 remains open: qualify the
Fleet/Electron compatibility matrix,
supported versions, other-platform migration and measured process/memory/startup
and package behavior. An unsigned optimized macOS `.app` has now been measured
at 101.70 MiB on a fresh optimized build of the current Tauri commit; cold startup and signed release artifact measurements remain
open. Do not claim stages 6-8 complete or infer that this branch constitutes a
V1 release.

The first compatibility slice passed: standalone `cmd/agent` and `cmd/server`
tests, race-enabled companion/backend/agent tests, a Windows companion
cross-build, 315 Electron unit tests, and the Tauri typecheck/build plus 86 UI
and 34 Rust tests. Fleet-native setup also passes its Go handler/backend tests;
this is contract coverage, not a live remote Fleet qualification. The migrated macOS install preserved its ten original
projects, trackers, OAuth key and 25 conversation histories; live Jira,
Claude/Codex prompt and embedded MCP calls succeeded. One idle companion used
about 40.7 MiB RSS against 55.7 MiB combined server/agent phase 0 medians. The
Tauri desktop process is excluded, so this is not a total application memory
claim. The debug `.app` measured 133.1 MiB versus 92.9 MiB at phase 0, mainly
because the upgrade build still bundles both legacy migration executables. An
unsigned release app measures 101.70 MiB and includes the companion and both
legacy binaries. Cold native startup, signed release artifact size, explicit
old/new Fleet protocol fixtures, a remote Fleet session, and a post-migration
Electron live MCP call remain unqualified. See
[the runtime plan](local-fleet-runtime-plan.md) for the scope of this evidence.

### Stage 5 compatibility evidence

| Combination | Evidence now available | Qualification |
| --- | --- | --- |
| Tauri + Local companion + embedded backend from this branch | Existing macOS managed-install migration; disposable Claude Code, Codex, MCP and companion lifecycle checks | Verified on macOS for the recorded prototype; repeat against the current commit before V1 |
| Tauri + bundled Fleet agent + current Go backend | Pairing HTTP contract fixture, TLS/credential checks, API authorization tests and UI/native setup tests | Contract coverage only; a real remote one-time-code pairing and native setup are still required |
| Tauri + legacy attached Fleet agent | Isolated macOS Tauri bundle loaded 19 projects from the existing v0.7.6 Fleet agent and showed `Agent local · backend Fleet` (2026-10-29), with no remote writes | Live dashboard and project discovery verified; the attach button itself, one-time pairing and native API setup remain unqualified |
| Existing Electron/Go workstation + live backend | The existing v0.7.6 agent returned connected status, no contract error and 19 discovered projects in a read-only live check (2026-10-29); 315 Electron unit tests pass | This qualifies the existing connection only; post-migration MCP and current-branch Electron/server compatibility remain unqualified |
| Mixed Go agent/server versions | Server/agent protocol V1 documents additive fields and unknown-field handling; dedicated old/new binary fixtures are absent | Unsupported as a V1 compatibility claim until the fixtures pass |
| Native platforms | macOS live migration and unsigned bundle build; Windows companion cross-build | Windows runtime/migration and Linux native builds remain unqualified |

Until these gates close, compatibility claims are limited to the tested source
revision and platform evidence above. The versioned server/agent contract is
additive within V1, but that rule alone does not prove any particular mixed
binary pair. V1 must publish a tested server/agent version matrix and desktop
platform support table before release.

Stages 6 and 7 are product-direction work, not authorization to remove existing
web pages, API routes or domain data during stages 0-5. A headless distribution
can omit website assets without immediately deleting the current web server.

## 5. Non-regression and upgrade gates

- A lifetime store lock precedes DB open/recovery; duplicate starters cannot
  alter an active run before discovering a port conflict.
- Conversations and discovery are bound to stable connection/backend/user
  identity. No fallback to Electron or another connection on failure.
- The composed runtime owns immutable configuration, restart and shutdown;
  auxiliary CLI roles must never start a backend accidentally.
- Migration uses a durable journal and coherent data/key/settings/journal
  generations. Automatic external work and OAuth refresh are disabled before
  commit. Automatic snapshot rollback stops at commit; later recovery proceeds
  forward to avoid replaying tracker writes or refresh rotation.
- Existing OAuth records, callback, credential kind and encryption bindings are
  preserved. A process-topology change must not force API-token replacement.
- Verify Claude Code/Codex response, tool tracking, questions, model/mode,
  resume and abrupt-stop behavior; verify supported Fleet/Electron combinations.
- Measure process count, memory, startup and package size. Do not infer a memory
  saving solely from merging processes.
- Separate any roadmap deprecation from runtime extraction and preserve tracker
  source-of-truth and caller attribution throughout.

## 6. Repository and branch strategy

- Sectile Go: `refactor/embedded-local-backend`, from preserved prototype commit
  `4576ca30ea4969bcec9be1a424e96ad677107cf7` on `feat/desktop-local-runtime`.
- Tauri: `feat/local-fleet-modes`, from initial prototype commit
  `afa466c801f146a00d7ef745907b5fc95c59be4c` on `feat/tauri-bootstrap`.
- Preserve separate repository history and licensing. No fork or relicensing is
  required for this branch-based implementation; independent maintenance is a
  later governance decision. Reuse the existing Go worktree; isolate subsequent
  parallel implementation work if needed.
- Use reviewable increments with both repositories' compatibility/version
  expectations recorded. Do not publish a partially migrated binary combination.

## 7. Open product decisions

1. Exact operational feature inventory required before Tauri is the default board.
2. External roadmapping tool, data ownership and linking/synchronization contract.
3. Deprecation/support window for current web/Electron experiences and consumers.
4. Whether/when to operate an OAuth-only service for public desktop distribution.
5. Supported platforms, backend versions and installation/update model for V1.

The separate companion, temporary internal HTTP/WebSocket transport and continued
support for current OAuth are confirmed decisions, not open questions.

## 8. Release meaning

V1.0.0 names the product destination and its eventual compatibility commitment.
This plan does not bump manifests, create a tag or imply that V1 is the next
release. Cut a release only through the Go repository's documented tag/changelog
procedure after the V1 scope and readiness criteria are met and approved.
