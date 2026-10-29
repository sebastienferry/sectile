# Sectile V1.0.0 migration plan

Date: 2026-10-29
Status: product direction and V1 ownership are confirmed; Local companion runtime
and managed upgrade are implemented on dedicated branches. macOS is the V1
target, Linux is experimental and Windows is deferred. Stage 5 compatibility
qualification remains underway. Web planning remains the V1 planning owner and
is unchanged in this release. Remaining compatibility, signing and UX
convergence gates are documented below. This is not a V1 release.

## 1. Product destination

Tauri becomes the main interface for Sectile's agentic work: project/workspace
prompts, the operational ticket board, skill/workflow execution, conversations,
interactive decisions and execution supervision.

The Go backend provides a headless application surface: API, MCP and
authentication. Behind those surfaces it retains the domain logic required for
tracker synchronization, workflow rules, execution coordination, authorization,
state and background jobs. "Headless" does not mean removing that logic or
reimplementing it in React.

Broader product planning and roadmapping remain in the existing web application
for V1. V1 does not aim to port every Electron/web feature into Tauri. The web
planning surfaces and data remain unchanged, and there is no web deprecation in
this release.

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
| Product strategy, portfolio and long-term roadmaps | Existing Sectile web application for V1 | Keep its planning features and data unchanged; no deprecation or extraction in this release |
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
| 7. Roadmapping ownership | Confirmed V1 owner and coexistence contract for web planning and the operational desktop | Web planning remains V1-owned and unchanged; no data move or deprecation in this release |
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
At Go commit `7dea9140`, the current Tauri agent completed a real temporary
one-time pairing against the live Fleet backend (`660-main`, commit
`b8fabf44`). Its local desktop gateway reported connected, `/api/me` matched the
pairing owner, and `/api/projects` returned successfully. The test revoked the
temporary workstation afterward and verified that no matching device remained.
Stage 4 visual qualification (2026-10-29, isolated Fleet Pair Test bundle)
confirmed that the paired app opens the native project screen, lists the
Fleet-backed projects, and exposes local-folder and tracker association flows.
The tracker screen lists existing remote sources, but OAuth setup reports that
the operation is unavailable on this Fleet backend version; adding a tracker
was not attempted. The profile form rendered with blank email and display-name
fields, so profile persistence was not exercised. No project, tracker or
profile data was changed. The workstation settings page still showed four
registered devices, the same count recorded before this check, and did not
expose a uniquely identifiable test-device record. Because two existing
devices share the same host name, neither was revoked; temporary-pairing
cleanup could not be verified safely. This live UI observation does not qualify
Fleet-native setup as complete. Existing-project folder association now uses the
native directory picker, and UI tests cover successful selection and cancel
without mutation; this does not replace live visual qualification.
Stage 4 visual qualification (2026-10-29, isolated Fleet Current Smoke bundle)
repeated one-time pairing against a disposable Go backend built from commit
`7dea9140`. The UI displayed the paired account's email and display name as soon
as pairing completed, and the email remained read-only. The follow-up fixes the
blank profile observed in the earlier bundle; a UI regression test covers both
pairing and attaching an existing Fleet agent. The isolated backend, paired
agent, app data and bundle were removed after the check. This qualifies profile
hydration on a current backend, while the live Fleet backend's unsupported OAuth
setup operation and safe test-device cleanup remain open.

New conversations persist the selected runtime's private connection UUID in Go's
run journal; a mismatched Tauri connection cannot read or resume them. Legacy
histories remain readable and Tauri requires an explicit association before
resuming an unbound history. Rust, Go and UI fixtures cover UUID migration,
journal persistence, wrong-connection refusal and legacy-client compatibility.
An additional route-level smoke used the current Go branch with a disposable
SQLite database, isolated `HOME` and `XDG_CONFIG_HOME`, and an ephemeral local
account. Authenticated `/api/me` returned that account's email,
`/api/desktop-oauth/apps` returned GitHub, GitLab and Jira without exposing
client secrets, and `/api/projects` returned the initial workspace. OAuth apps
were unconfigured, and no tracker, project or profile writes were made. A
separate-ID Tauri release bundle built successfully, but CUA could not bind its
custom bundle identifier, so this smoke does not verify those screens visually.
An initial server launch omitted the isolated home and made one read-only
`/desktop/runs` request to the already-running local agent; two entries were
copied only into the disposable test database. That server was stopped, the
database removed, and the successful route smoke was repeated with isolated
home/config paths, which did not contact the existing agent.
Onboarding integration re-run (2026-10-29): `npm run test:services` passed
with an isolated temporary Go worktree configured for loopback port 52964,
because the existing Local companion correctly retained production port 52963.
The run used a disposable server database, local account, paired agent, fake
tracker endpoint and temporary home. It verified profile persistence, pairing,
personal tracker credentials, GitHub OAuth app configuration and browser
handoff/cancellation, fake tracker and project creation, project mapping
through the paired agent, server restart and persisted profile/projects. Real
provider tests were not enabled; no live tracker or existing database was used.
The test worktree was removed and the ignored service binaries were restored
from clean Go commit `7dea9140`.
An earlier separate-ID Tauri bundle connected read-only to the existing Fleet
workstation agent and loaded its 19
projects; new one-time-code pairings also verify the backend user's ID against
the identity recorded at pairing. Runtime switching is deliberately
unsupported: selecting a second mode requires a future explicit migration flow.
Stage 5 remains open: qualify the
Fleet/Electron compatibility matrix,
supported versions, other-platform migration and measured process/memory/startup
and package behavior. A fresh Linux ARM64 Debian package was built from Tauri
commit `ab8c7a7` and clean Go commit `7dea9140` in a Debian Bookworm ARM64
container. The 49.11 MiB `.deb` contains an ARM64 Tauri executable and all three
Go services; each embedded service is an ARM64 ELF binary, its SHA-256 matches
the package manifest, and it reports the expected Go commit. The package
installed successfully with `dpkg -i` in Debian Bookworm ARM64 and opened a
visible X11 window under Xvfb/DBus before it was stopped cleanly. This is a
simulated ARM64 startup smoke, not qualification on a real Linux desktop:
interactive UI, migration, provider execution and behavior across distributions
remain unqualified. The
current unsigned optimized macOS `.app` measures
101.72 MiB. One isolated arm64 macOS cold launch showed its first native window
at 1.09 seconds. Three subsequent process relaunches using the same unconfigured
profile and warm filesystem cache showed 207–245 ms to the first window. The
React setup screen was manually accessible by about 3.5 seconds on the initial
launch (an upper bound from a two-second observation interval, not automated UI
readiness). Repeated true cold-start, signed artifact and cross-platform
measurements remain open. Do not claim stages 6-8 complete or
infer that this branch constitutes a V1 release.

The first compatibility slice passed: standalone `cmd/agent` and `cmd/server`
tests, race-enabled companion/backend/agent tests, a Windows companion
cross-build, 315 Electron unit tests, and the Tauri typecheck/build plus 9 domain,
90 UI and 35 Rust tests. Fleet-native setup also passes its Go handler/backend
tests. The migrated macOS install preserved its ten original
projects, trackers, OAuth key and 25 conversation histories; live Jira,
Claude/Codex prompt and embedded MCP calls succeeded. One idle companion used
about 40.7 MiB RSS against 55.7 MiB combined server/agent phase 0 medians. The
Tauri desktop process is excluded, so this is not a total application memory
claim. The debug `.app` measured 133.1 MiB versus 92.9 MiB at phase 0, mainly
because the upgrade build still bundles both legacy migration executables. An
unsigned release app measures 101.72 MiB and includes the companion and both
legacy binaries. One initial launch and three process relaunch samples are
recorded in the runtime plan; repeated true cold-start measurements and a signed
release artifact remain unqualified; this host has zero valid macOS code-signing
identities. Real Fleet one-time pairing and a live Electron MCP list-projects
call have since passed for the tested server/agent combination; native setup
screens remain unqualified. On Go commit `5f809481`, the full Go suite and
Electron unit/UI suites pass. These are source-level regressions checks, not a
live current-branch Electron/server session. Cross-version
tracker/workflow API behavior has passed against an isolated mock; live external
tracker writes remain unqualified.
The Sectile MCP connector was retried read-only on 2026-10-29 and
returned `connection closed: client is closing`; this does not qualify the
post-migration MCP call. See
[the runtime plan](local-fleet-runtime-plan.md) for the scope of this evidence.

Current Tauri verification (2026-10-29, Tauri commit `ab8c7a7`): `npm run
check`, `npm test` (9 tests), `npm run test:ui` (90 tests), `npm run build`, and
`npm run test:rust` (35 tests) all pass. Two isolated macOS release bundles built successfully with unique bundle
identifiers for live Fleet checks; each measured 101.72 MiB. The Fleet Pair Test
bundle contains byte-identical copies of all three `src-tauri/binaries` service
resources; each is a Darwin arm64 `dev` binary. At Tauri commit `2206473` and Go
commit `7dea9140`, `prepare:services` now embeds the same source commit and date
in all three service versions and writes a manifest with SHA-256, size, dirty
state, tag and release eligibility. A clean source build produced three
matching `7dea9140` binaries; an isolated optimized `.app` included the
manifest and all three resources with matching hashes. This verifies
reproducible source identity and packaged provenance on this host, not a tagged
release, signature or other-platform package. The active Local installation
and its companion remained running during those checks, but the debug-bundle
companion was built earlier (2026-10-29 05:06 local time) and its SHA-256 does
not match the current `src-tauri/binaries/sectile-companion` input (rebuilt at
08:39). Its version output is only `dev`, without the embedded source revision.
Do not count that already-running Local installation as execution of the current
Go binary; a fresh isolated current-source Local launch remains a qualification
gate.

Current UX-source bundle check (2026-10-29, Tauri commit `8803eff`): an isolated
unsigned optimized app was built with bundle identifier
`com.sebferry.sectile.tauri.uxaudit` under `/tmp`, measured 101.72 MiB, and
contained arm64 Mach-O `sectile-agent`, `sectile-companion` and `sectile-server`
resources. `npm run check`, the production frontend build and native release
compilation passed. The Mac was locked during this check, so the app was not
launched and no visual or runtime behavior is qualified by this build.

Current-source macOS bundle rebuild (2026-10-29, Tauri commit `15f949f`, clean
Go commit `7dea9140`): `npm run prepare:services` followed by
`npm run tauri -- build --bundles app` produced an unsigned 101.48 MiB `.app`.
The packaged manifest identifies the expected clean Go commit, and the embedded
companion SHA-256 matches the manifest and reports `dev (7dea91406e8c)`. The
bundle was not launched: the active Local companion retained fixed port 52963,
and the existing app was left undisturbed. This qualifies current-source bundle
provenance, not a fresh Tauri-to-companion runtime launch.

Current-source Tauri UI launch (2026-10-29, after Fleet visual qualification):
rebuilt the debug bundle from the current Tauri source and reopened it with the
existing Local workspace. The UI reconnected on the default port, loaded 12
projects, and showed the current sidebar/status and project setup surfaces. All
three Go resource hashes in the rebuilt bundle match the clean `7dea9140`
manifest. The companion process was already running and had been loaded from an
earlier bundle, so it was deliberately left untouched; this verifies current
UI reattachment and packaged provenance, not a cold start of the current Go
binary on port 52963. The separate current-Go Local prompt/MCP smoke remains
qualified only with its isolated test port.

Fresh Local Tauri runtime smoke (2026-10-29, same Tauri source, Go commit
`7dea9140`): an isolated bundle identifier and a temporary Go worktree changing
only the Local port from 52963 to 52964 avoided stopping the user's active
companion. The packaged Tauri app started its current-source companion and
agent, saved an isolated local profile, stored a synthetic GitHub credential
against a loopback mock, created a mock-backed tracker and project, and linked
`/private/tmp/sectile-local-current-smoke-project` through the native directory
picker. The project configuration and both new-prompt and conversation screens
showed that folder as the execution context. Real Codex (`gpt-6-astra`) and
Claude Code conversations each read the temporary README through a tracked tool,
returned the exact expected marker and first line, and reported Sectile MCP as
connected. Neither prompt changed a file. The mock received only a read-only
`GET /user`; the profile, credential, tracker, project and conversations lived
in the isolated app database. Closing the desktop left its companion running as
designed; the test then stopped it and removed the isolated app data, workspace,
bundle and temporary worktree. Generated services were restored from the clean
Go source afterward. This verifies the Tauri Local path with a test-only port
override; it does not replace the clean default-port bundle launch qualification.
During this smoke the conversation detail reported `Prêt`, while the sidebar
labels still said `En cours` after both answers completed. Tauri commit
`620c2d8` resolves this by showing the loaded turn state in project navigation
and labeling an unloaded, long-lived provider process `Session active`. The
domain and workspace UI regressions cover the idle-session case.

### Stage 5 compatibility evidence

| Combination | Evidence now available | Qualification |
| --- | --- | --- |
| Current Go branch and Electron client suites | On Go commit `5f809481`, `go test ./...` passes across all packages, `npm --prefix desktop test` passes the Electron unit suite, and `node --test --test-concurrency=1 --test-reporter=dot desktop/tests/*.ui.cjs` exits successfully on the full Electron UI suite. | Current-source unit and Electron UI regression suites pass. This does not qualify a live Electron session against the current Go server or external tracker mutations. |
| Local companion binary from Go `7dea9140` | Fresh isolated macOS smoke using an ephemeral data root: local profile sign-in, one-time pairing with owner/device identity check, embedded agent readiness, authenticated `/api/projects` and clean SIGTERM shutdown | Pass for the current-source companion binary and backend lifecycle. This does not qualify provider/MCP execution on the default port. |
| Tauri + Local companion + embedded backend from this branch | Existing macOS managed-install migration; fresh isolated Tauri Local launch from Go `7dea9140` source with a test-only alternate port; native profile, fake tracker/project, folder association, Codex and Claude Code prompts, tool events and MCP connection. An isolated bundle from Go `560f1f4a` plus Tauri `bc48a02` then cold-started on 52964, connected a Local profile, created a folder project with the synthetic `local` tracker and displayed its prompt-ready board without remote tracker controls. Current-source Tauri UI also reattached to the existing Local workspace on the default port. | Pass for the isolated macOS Local startup, provider projection and UI path. The user's production companion on 52963 was not restarted, so current-Go process startup on that exact port remains unqualified. The fixed port contract itself remains covered by Go tests. |
| Tauri current agent + live Fleet backend | Agent `dev` from Go commit `7dea9140` paired through a temporary one-time code to live server `660-main` (`b8fabf44aa7a`); the local gateway reported connected, `/api/me` matched the pairing owner, and `/api/projects` succeeded. The visual check confirmed Fleet-backed project listing and local-folder/project-link controls. OAuth setup reports unsupported by this Fleet backend version. The workstation page remained at four devices and did not identify a distinct test record, so no existing device was revoked. | Real pairing and authenticated project access pass. The newer isolated visual pairing test verifies Fleet profile email/name hydration in current Tauri; live-backend OAuth support and safe cleanup of a uniquely identified live test device remain open. No tracker writes were attempted |
| Tauri + legacy attached Fleet agent | An isolated `com.sebferry.sectile.tauri.fleetcheck` macOS bundle used the native “attach existing agent” flow, reported `Agent local · backend Fleet`, loaded 19 live projects, and read 390 tickets from the Sectile project (2026-10-29). It attached to the existing v0.7.6 workstation agent without creating tracker writes or replacing the active Local profile. A read-only `/api/me` request through that agent gateway returned `401`, so this legacy connection cannot supply the authenticated user profile required for native API setup. | Live attachment, backend mode display, project discovery and tracker-backed board reads verified. Native setup through this legacy connection still requires a newly paired desktop credential. |
| Existing Electron/Go workstation + live backend | The installed Electron agent is `v0.7.6` (`ac74004102c0`). Its `sectile-agent mcp` bridge connected to live server `660-main` (`b8fabf44aa7a`), listed 21 tools and completed a read-only `list_projects` call (2026-10-29); 315 Electron unit tests and 137 Electron UI tests pass on Go commit `7dea9140` | Live MCP bridge behavior is verified for this installed client/server pair. Current-branch Electron/server compatibility remains unqualified |
| Source-level v0.7.6 protocol fixtures | The current server parses a pinned v0.7.6 agent connection query; a parser copied from the v0.7.6 server source parses the current agent announcement. Focused agent/handler tests also cover operation dispatch. | Pass on this source revision. These tests do not execute either released binary. |
| Released agent binary to current Go server | The Go repository harness `python3 scripts/compatibility/test-v076-server-current-agent.py` builds the `v0.7.6` agent from its tag and connects it to the current handler with isolated database, HOME and Git repositories. Re-run against Go commit `7dea9140` on 2026-10-29. | Pass for startup, WebSocket handshake, 27-operation status, task lifecycle, mocked tracker API calls, `skills_status`, `init_git` and task-spec worktree creation. |
| Current agent binary to `v0.7.6` server binary | The same harness starts the server built from the Git tag with isolated SQLite state; the current `dev` agent connects and the server reports all 27 operations with `outdated=false`. Re-run against Go commit `7dea9140` on 2026-10-29. | Pass for startup, WebSocket handshake, task lifecycle, mocked tracker API calls, `skills_status`, `init_git` and task-spec worktree creation. |
| Cross-version MCP clients and servers | The harness starts real `v0.7.6` and current server binaries against separate temporary databases. Each server is exercised by the other version's SDK client: list 21 tools, call `list_projects`, create a GitHub issue through a mock API with the personal token, transition the remote task to `clarified`, verify tracker sync, start/finish a run on a local task, read back persisted statuses and prepare a task specification worktree. | Pass for MCP sessions, catalog, remote issue creation, caller-token attribution, one tracker stage update, run lifecycle, workflow transition and task specification worktree preparation in both directions. Live GitHub/Jira/GitLab behavior, execution tools and Electron registration remain unqualified. |
| Cross-version agent operation dispatch | The same harness connects a current agent to the v0.7.6 server and a released v0.7.6 agent to the current server. Each pairing calls authenticated `skills-status` and `init-git` routes. MCP `prepare_task_spec_worktree` creates a registered, distinct worktree on a task branch through the server-agent WebSocket in both pairings; the test verifies its path, branch and Git registration. | Pass for read, mutation and task specification workspace operations over the server-agent WebSocket in both directions. Cancellation, provider execution and Electron registration remain unqualified. |
| Go service cross-builds | On 2026-10-29 at Go commit `3ea74cbb`, built `cmd/companion`, `cmd/agent` and `cmd/server` for both `linux/amd64` and `windows/amd64`; `file` confirmed ELF and PE x86-64 outputs. | Go service compilation passes for both targets. This does not qualify the Tauri package, native startup, migration, process execution or installers on either OS. |
| Optional headless server artifact | At Go commit `7dea9140`, `make build-server-headless` produced `bin/server-headless` without invoking Node. The opt-in `sectile_headless` build retains API/MCP/auth routes and returns 404 for browser paths; isolated runtime smoke verified `/api/health` (200), browser paths (404), and an unauthenticated unknown API route (JSON 401). Both default and tagged `cmd/server`/`internal/webui` tests pass. | Linux/amd64 ELF and Windows/amd64 PE headless binaries cross-compile. This qualifies the optional artifact and its no-web behavior on the current macOS host, not native Linux/Windows runtime or release distribution. |
| Local workspace board metadata | A live Tauri review found that Local projects carry a synthetic `local` tracker row; the older desktop projection dropped provider identity, leaving remote-only ticket filters visible. Go commit `560f1f4a` includes provider per selected tracker; Tauri commit `bc48a02` hides remote controls for Local-only projects and includes a bounded older-agent fallback. An isolated macOS Tauri bundle with a separate app identifier and port started the current Go companion, saved a synthetic Local profile, created a folder-backed project whose DB tracker provider was `local`, and rendered “Votre projet local est prêt” without remote search, grouping or status controls. | End-to-end Local behavior passes in isolation. The production companion and its data were not touched; disposable app data and the 52964 process were removed after the check. |
| Native platforms | User-selected V1 support is macOS-first; Linux is experimental; Windows is deferred. macOS Local/Fleet migration and isolated Local startup/provider paths have live desktop evidence. Linux ARM64 `.deb` was built and installed with a visible X11 window under Xvfb/DBus at Tauri `446285c`/Go `7dea9140`; Windows and Linux Go services cross-build. | macOS runtime behavior is qualified except for a clean default-port cold-start/provider rerun and signed/notarized distribution. Linux remains experimental: real user desktop/provider/migration and distro breadth are unqualified. Windows remains deferred after cross-build only. |
| macOS signing | `security find-identity -v -p codesigning` returned zero valid identities on the qualification host (2026-10-29) | No signed macOS artifact can be qualified on this host; a valid signing identity is required |
| Open-source package provenance | The user selected MIT for both repositories. Tauri has an MIT `LICENSE`; Go commit `5f809481` adds the MIT grant to the services repository and updates its README. Go and Tauri packages now include inventories and license texts for their Go, Rust, and JavaScript dependency sets. The Go notice records the `go.mod` hash; Tauri verifies that hash before bundling the selected Go source. | Repository licensing and package notices are implemented and verified in the Go release archive and macOS app bundle. Keep notice regeneration and dependency-graph validation in the release process; signed macOS distribution remains open. |

Cross-version harness re-run (2026-10-29, Go commit `7dea9140`):
`python3 scripts/compatibility/test-v076-server-current-agent.py` passed. It
connected current and released agents in both server directions, completed
both MCP client/server directions with 21 tools each, exercised task and
workflow state, run lifecycle and separate specification worktrees, and
verified `skills_status` plus `init_git` through each agent. The GitHub issue
creation and stage update used the harness's local fake tracker and a test
personal token; this is protocol and compatibility evidence, not a live
GitHub/Jira/GitLab qualification. The harness cleaned its temporary databases,
homes and repositories and left the Go worktree clean.

Live Local MCP smoke (2026-10-29): the already-running Tauri Local companion
answered an authenticated Streamable HTTP MCP session from its existing local
installation. The client listed 21 tools and successfully called the read-only
`list_projects` tool. The call used the Local companion's private installation
credential in memory; no credential was logged or changed. This verifies the
running Local MCP route, not the isolated release bundle or Electron's separate
MCP registration. A separate read-only smoke then invoked the installed
Electron `sectile-agent mcp` bridge against its configured live backend. It
listed 21 tools and successfully called `list_projects`; its existing bearer
credential was read from the active MCP process in memory and was not logged or
changed. This qualifies the installed Electron bridge against that live backend,
not every Electron/server version pair. The separate Codex app Sectile MCP
connector still returned `connection closed: client is closing`.

Full Go suite re-run (2026-10-29, Go commit `7dea9140`): `go test ./...`
passed across the server, companion, agent, backend, handlers, provider runner,
MCP, tracker and workspace packages after adding the optional headless build.
Electron verification on the same host: `npm test` passed all 315 unit tests.
`npm run test:ui` first ran without the generated `desktop/dist/index.html`, so
Electron opened empty windows: four tests passed, six failed on missing UI, and
66 were cancelled after the appearance test's child process remained open. On
2026-10-29, after `cd desktop && npm run build`, the targeted agent-log UI tests
passed (2/2) and the full `npm run test:ui` suite passed (137/137, no failures or
cancellations). This qualifies the current Electron UI suite on the tested
macOS host and source revision; it does not by itself qualify an Electron/Go
mixed-version pair.

Until these gates close, compatibility claims are limited to the tested source
revision and platform evidence above. The versioned server/agent contract is
additive within V1, but that rule alone does not prove any particular mixed
binary pair. V1 must publish a tested server/agent version matrix and desktop
platform support table before release.

Stages 6 and 7 are product-direction work, not authorization to remove existing
web pages, API routes or domain data during stages 0-5. The user selected the
existing web application as the V1 planning owner, with no web deprecation in
this release. The current client and
feature ownership evidence is recorded in
[the V1 feature ownership inventory](v1-feature-ownership-inventory.md). It
identifies the existing web/Electron planning and operational capabilities and
the current Tauri components, and documents interim transition paths for web,
Electron, Tauri Local/Fleet and MCP clients. It does not select a replacement
roadmapping tool or grant authority to deprecate any client. A headless
distribution can omit website assets without immediately deleting the current
web server.

Stage 6 headless server slice (2026-10-29): Go commit `7dea9140` adds the
`sectile_headless` build tag and `make build-server-headless`. The default Fleet
server build still embeds the web application. Default and headless server/UI
tests pass; the headless binary passed an isolated runtime smoke and Linux and
Windows amd64 cross-builds. A second isolated smoke confirmed `POST /mcp`
remains registered and rejects an unauthenticated initialization with JSON 401.
The inventory now records explicit interim migration paths for existing
consumers. The final Stage 6 distribution decision is to keep the regular Go
server (web, API, MCP and authentication) as the default Fleet server and offer
the optional headless build (API, MCP and authentication) for deployments that
do not need the web application. Tauri Local continues to use the companion,
which embeds the backend with the execution agent. This completes the Stage 6
product-transition definition and headless artifact; Stage 5 still gates
supported macOS packaging and compatibility. Stage 7 now records that web
planning remains the V1 owner, with no web changes or deprecation in this
release.

The post-change default-server smoke also passed with disposable SQLite: the
default binary returned a healthy `/api/health` response and retained its
existing fallback landing page when no web bundle was present. The web-bundled
default path remains unchanged in code; native Fleet pairing and web/Electron
client migration are separate qualifications.

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

## 7. Confirmed product decisions and remaining release gates

- The user selected MIT licensing for both the Tauri and Go repositories.
- V1 targets macOS first; Linux is experimental and Windows is deferred.
- Existing web planning remains the V1 owner and is unchanged in this release;
  no web deprecation, data move or replacement is authorized for V1.
- The separate companion, temporary internal HTTP/WebSocket transport and
  continued support for current OAuth remain confirmed decisions.

Stage 7's ownership decision is complete. Before V1 release, finish the exact
operational feature inventory, publish the tested backend compatibility range,
define packaging/update behavior, qualify macOS signing/notarization, complete
remaining Fleet/Electron compatibility checks and finish the UX convergence
plan. Third-party notices are included in the Go release archive and Tauri
bundle; keep them synchronized with dependency changes. These gates do not
authorize changes to the existing web application.

## 8. Release meaning

V1.0.0 names the product destination and its eventual compatibility commitment.
This plan does not bump manifests, create a tag or imply that V1 is the next
release. Cut a release only through the Go repository's documented tag/changelog
procedure after the V1 scope and readiness criteria are met and approved.
