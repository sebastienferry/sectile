# Sectile V1.0.0 migration plan

Date: 2026-10-29
Status: product direction and V1 ownership are confirmed. Stages 0-4 are
delivered on dedicated branches, including current-source Fleet setup and
managed-migration recovery evidence. Stages 5 and 8 remain open. Stages 6-7
are complete as product decisions and artifacts. macOS is the V1 target, Linux
is experimental and Windows is deferred. Web planning remains the V1 owner
and is unchanged in this release. The scoped UX convergence audit is complete;
current qualification gaps are listed below. This is not a V1 release.
The [candidate support matrix](v1-support-matrix.md) summarizes the exact
client/backend and platform combinations demonstrated by these checks.

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
| 4. Managed upgrade and native modes | Connection-bound Local/Fleet configuration; journaled upgrade of the prototype; native setup in both modes | Histories, folders, identities and current OAuth preserved; pre-commit source preservation and candidate retry tested at durable boundaries, post-commit recovery selects the committed generation; Fleet pairing/setup qualified; pairing or attachment cannot silently replace a selected runtime; no implicit transfer |
| 5. Compatibility and measured delivery | Local/Fleet/Electron test matrix, packaged artifacts and performance comparison | Required behavior and supported versions documented before removing old Local startup |
| 6. Product/UI transition | Tauri becomes the primary agentic board; a headless backend distribution is defined | Operational features and existing consumers have an explicit migration path |
| 7. Roadmapping ownership | Confirmed V1 owner and coexistence contract for web planning and the operational desktop | Web planning remains V1-owned and unchanged; no data move or deprecation in this release |
| 8. V1 release qualification | Agreed support/migration/deprecation boundaries and operational documentation | Release checklist and explicit release approval completed |

The operational checklist is [V1 release qualification](v1-release-checklist.md).
It records proven checks separately from open signing, installation,
compatibility and owner-approval gates; it does not authorize publishing.

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

Current-source Fleet native setup qualification (2026-10-29): an isolated
Tauri app built from the current application source paired through the native
screen to a disposable Go backend from `df1d846c`. The UI saved a Fleet profile,
validated and stored a fake GitLab credential against a loopback endpoint,
created a tracker and linked project, then reconnected after the backend
restarted with the profile, tracker and projects retained. No external service
or production account was used. This closes the native Fleet setup slice for
that exact source pair; OAuth app setup and live-provider qualification remain
open.

Managed-migration recovery qualification (Go `2cb7f75a`, 2026-10-29):
`TestUpgradeRecoversEveryDurableBoundaryAndPreservesIdentity` now asserts that
the source database and installation manifest remain byte-identical and no
runtime selection is committed after injected failures at each pre-commit
boundary. At post-commit boundaries it verifies the selected generation is
available. The test then recovers forward and verifies the original identity,
credential, encryption key and database remain usable. The full Go suite
passes. This exercises the accepted retry/forward-recovery contract; it does
not implement automatic snapshot rollback.

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

Local service/onboarding rerun (2026-10-29, Go `df1d846c`): the Tauri service
harness passed with an isolated companion on port 52965, disposable HOME/data,
and a fake tracker endpoint. It verified authenticated discovery, profile,
pairing, personal tracker credentials, tracker/project creation, folder-only
project mapping, companion restart and persisted profile/projects. Real-provider
flags were disabled. The test binaries were replaced afterward by the clean
Go `df1d846c` default-port build; the manifest reports `dirty: false`, the
existing companion remained on port 52963, and port 52965 was free.
Stage 5 compatibility is qualified in bounded slices, but the full matrix is
not closed. On 2026-10-29, the exact current Go commit `df1d846c` passed
`go test ./...`, the v0.7.6/current binary compatibility harness in both
directions, and the Electron production build plus 315 unit and 137 UI tests.
The harness passed the 21-tool MCP catalog, both STDIO bridges, run lifecycle,
workflow operations and fake tracker create/stage updates. A separate live
Electron UI smoke against this Go commit loaded projects and tickets read-only
from isolated data. A disposable current-source Go backend paired with the
current Tauri Fleet build; the app loaded its profile and default project,
closed, and reopened with the saved pairing, profile and project restored.
This verified Fleet reconnect behavior only, with a local fake account and no
external tracker. Current Local Tauri onboarding, Codex/Claude responses, tool
tracking, interruption/resume and MCP behavior have passed on isolated data
with the test-only port override; the current-source package and DMG integrity
verifier also pass. Exact-source evidence and limitations are in the Stage 5
table below.

The remaining Stage 5 gaps are end-to-end live OAuth/tracker/provider workflows
on the release binary and tracked-task tests with remote tracker mutations.
The isolated Electron tracked-ticket run below now confirms visible skill tool
activity and Stop against the current Go agent. A later tracked-ticket run
verified that its Codex tool cannot write a delayed file after Stop. The
bounded project-prompt test below confirmed that
Electron displays Codex's read-only tool activity. The repeat test against Go
commit `5274a160` confirmed that Electron Stop changes the run to canceled while
Codex's detached shell survives and writes its marker after 90 seconds. The Go
PTY descendant-stop test did not cover that Codex-owned process. The later
Electron rerun against Go `22c33ec1` stopped the built-in Codex PTY tool and
left its delayed marker absent after 104.6 seconds. A tracked-ticket rerun
against Go `eb485f64` exposed a separate race: the Ctrl-C grace period let
Codex exit and orphan its shell tool, which wrote the delayed marker. Go
`25a8e77d` closes the process tree before the provider or PTY exits; the
Electron rerun left the marker absent after 170 seconds. The structured Codex
conversation path used by Tauri has also passed interruption and delayed-marker
checks. The installed live
Fleet backend `660-main` is older than the current Go branch: pairing and
authenticated project reads work, but its OAuth administration endpoint is
unavailable. Do not infer support for that server from the current-source
protocol harness. The current release package
has no valid Developer ID signature, so Gatekeeper, notarization and clean-user
installation remain Stage 8 gates. A clean default-port cold start is deferred
while the user's existing companion owns port 52963. Linux is experimental and
Windows is deferred, so migration qualification for those platforms is outside
the V1 release gate. A fresh Linux ARM64 Debian package was built from Tauri
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
measurements remain open. Stages 6 and 7 are complete as product-transition
decisions and artifacts; Stage 8 remains open. Do not infer that this branch
constitutes a V1 release.

The first compatibility slice passed: standalone `cmd/agent` and `cmd/server`
tests, race-enabled companion/backend/agent tests, a Windows companion
cross-build, 315 Electron unit tests and 137 Electron UI tests, and the current
Tauri typecheck/build plus 14 domain, 101 UI and 35 Rust tests. Fleet-native setup also passes its Go handler/backend
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
call have since passed for the tested server/agent combination. A current-source
Tauri Fleet UI qualification is now recorded in the Stage 5 table below. On Go
commit `5f809481`, the full Go suite and
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

The [Local runtime measurements](v1-runtime-measurements.md) compare startup,
process count, memory and binary size for the embedded companion and separate
Go services on disposable data. They do not include the desktop renderer.

| Combination | Evidence now available | Qualification |
| --- | --- | --- |
| Current-source regression suites | On Go commit `22c33ec1`, `go test ./...` passes, including the PTY stop regression. Electron `npm test` passes all 315 tests and `npm run test:ui` passes all 137 tests. On Tauri commit `f02e511`, `npm test` passes 14 tests, `npm run test:ui` passes 101 tests, `npm run check`, `npm run test:rust` (35 tests) and `npm run build` pass. Verified on 2026-10-29. | Pass for the source-level Go, Electron and Tauri suites at these revisions. These suites do not replace live tracker/OAuth/provider workflows, tracked-ticket Electron execution or package signing and clean installation gates. |
| Current Go branch and Electron client suites | On Go commit `df1d846c`, `go test ./...` passes across all packages; `npm --prefix desktop run build`, `npm --prefix desktop test` (315 tests) and `npm --prefix desktop run test:ui` (137 tests) pass on 2026-10-29. The Electron renderer build reports 1.39 MB uncompressed JavaScript (427.72 kB gzip) and a Vite chunk-size warning. | Current-source Go and Electron regression suites pass. Electron UI tests use isolated fixtures and do not qualify a live Electron session against the current Go server or external tracker mutations. The bundle warning is a follow-up optimization candidate, not a release blocker by itself. |
| Electron 0.7.5 UI to current Go server and agent | On 2026-10-29, the current Electron app was launched with a separate `SECTILE_DESKTOP_DATA_DIR` and a current Go server/agent from commit `df1d846c`. The temporary SQLite database contained one local project/task; the Electron board loaded the task as `ELECTR-1` in New. All processes, credentials and data were isolated and removed after the check. | Pass for Electron connection, project discovery and read-only ticket-board loading against the current Go API. This does not qualify Fleet server setup/reconnect, tracker adapters, execution or full workflow behavior. |
| Electron project prompt, tool activity and interruption | The 2026-10-29 Electron `0.7.5` smoke against Go `5274a160` displayed the Codex response and `1 background terminal running`, but Stop marked the run canceled while its shell continued and wrote the delayed marker. Go commit `22c33ec1` changes built-in interactive Codex PTY commands to `--no-daemon` and sends Ctrl-C before closing the supervised PTY tree. `SECTILE_TEST_CODEX_PTY=1 go test ./internal/agent -run '^TestCodexPTYStopPreventsDelayedMarker$' -count=1 -v` exercised the actual signed-in Codex `0.165.0` CLI through Go's `/desktop/stop` endpoint in a disposable workspace and `CODEX_HOME`; the marker scheduled after 90 seconds was absent after 95 seconds. The complete Go suite passes on this commit. | Partial: built-in Codex interruption now passes through the real Go endpoint and process tree. The live Electron UI rerun is recorded in the next row; tracked-ticket tool activity, tracker mutations and live provider/tracker workflows remain unqualified. Custom Codex command templates remain responsible for their own process ownership. |
| Electron project-prompt Stop against current Go | On 2026-10-29, Electron `0.7.5` connected to a disposable Go server and agent built from `22c33ec1` with only the Local test port changed to 52965. The UI opened a folder-only project prompt with Codex `0.165.0` in a temporary Git repository and isolated `CODEX_HOME`. Codex started `printf STARTED > started-marker.txt; sleep 90; printf STOPPED > stop-marker.txt`; the first marker appeared. Clicking Electron's Stop changed the run to Stopped/canceled, and the delayed marker remained absent 104.6 seconds after the first marker's creation. | Pass for Electron project-prompt Codex PTY tool cancellation against this exact Go source. Tracked-ticket tool activity, tracker mutations, Fleet variants and external OAuth/provider workflows remain unqualified. |
| Electron tracked-ticket tool activity and Stop | On 2026-10-29, Electron `0.7.5` used a disposable server/agent from Go `eb485f64`, with only the Local test port changed to 52965. A synthetic `FIXTUR-1` task in an initialized temporary Git repository used the installed `clarify-issue` skill and signed-in Codex `0.165.0` through an isolated `CODEX_HOME`. The Electron console showed Codex reading `SKILL.md`; Stop changed the UI to Stopped/canceled, SQLite recorded `remote_run canceled`, and no fixture `agent-exec` remained. A separate read-only Codex launch failed as expected and also left no supervisor after the fix. The complete `go test ./...` suite passes on `eb485f64`. | Pass for the tracked-ticket launch, visible tool activity, run cancellation and process cleanup on this isolated source pair. No delayed 90-second marker was scheduled in the tracked task, and no real tracker mutation or external OAuth flow was tested. The production companion on 52963 was not touched. |
| Electron tracked-ticket delayed-tool Stop | On 2026-10-29, an isolated Electron `0.7.5` tracked task on Go `eb485f64` started a signed-in Codex `0.165.0` shell tool that wrote `STARTED`, slept 90 seconds and then wrote `LATE`. Electron Stop marked the task canceled, but the shell was reparented to `launchd` and wrote `LATE` after 90 seconds. With only the test port changed to 52965, the disposable agent was rebuilt from Go `25a8e77d` and the same tracked task was launched through Electron against the disposable server. `STARTED` appeared, Electron Stop showed Stopped/canceled, no tool process remained, and `LATE` was absent after 170 seconds. The focused detached-child regression and `go test ./...` pass on `25a8e77d`. The production companion on 52963 was untouched. | Pass for Electron tracked-task Codex cancellation of a delegated shell tool on this macOS host. The fixture uses a synthetic local task and tracker; remote tracker mutation, real OAuth and packaged-app behavior remain separate gates. |
| PTY run-control token privacy | Go commit `f5984122` removes the temporary run-control token from the shell command injected into a desktop PTY. The token is supplied to `agent-exec` through the PTY session environment and removed before the provider child starts. The focused test launches a real wrapped PTY command and checks its history and provider environment; the complete `go test ./...` suite passes on this commit. | Pass for source-level token isolation, including consoles that pre-create their PTY session. The Electron visual Stop check above used the prior Go commit; a current-source packaged desktop smoke remains open. |
| Local companion binary from Go `df1d846c` | On 2026-10-29, `npm run test:services` built all three services from a disposable worktree with only the Local runtime port changed to 52965 because 52963 and 52964 were occupied. Isolated startup/profile/pairing/credential/tracker/project/mapping/restart checks used a fake tracker endpoint and temporary data. The Tauri binaries were then restored from clean Go commit `df1d846c`, with the default 52963 contract and `dirty: false` manifest. | Pass for current-source Local onboarding/service contracts on isolated data. This is not live OAuth/tracker/provider execution or native Tauri screen qualification. The existing companion on 52963 remained active; the test port was released. |
| Current Tauri Fleet UI and current Go backend | Tauri source commit `a75b140` (application code unchanged from `239d7c1`) was packaged with a unique bundle ID and temporary Cargo target. Go commit `df1d846c` ran on port 55460 with a disposable database/account; a loopback fake GitLab endpoint returned a fixture identity. Through native Tauri screens, the test paired Fleet, saved a profile name and fake GitLab token, created a GitLab tracker and linked project, then restarted the Go server. The app reconnected and retained both projects and the tracker. The test app, agent, fake endpoint, server, app data and build target were removed. | Pass for native Fleet pairing, profile and personal credential setup, tracker/project creation and reconnect persistence for this exact source pair on macOS. OAuth was not configured; no real tracker, provider or production backend was contacted. |
| Claude Code and Codex Local conversations | Go commit `408cd909` in the isolated 52964 test worktree ran real signed-in Claude Code and Codex CLIs in a disposable folder. Both returned responses with tracked tool calls. Each ran a command scheduled to write a marker after 90 seconds; stop was acknowledged, the marker remained absent after 95 seconds, and the same conversation resumed with `RESUME_OK`. The onboarding/restart checks then completed and all test services and data were cleaned up. | Pass for current-source Local provider responses, tool tracking, interruption, surviving-child prevention, same-conversation resume and state retention on this macOS host. This does not exercise a Tauri GUI window or remote tracker writes. |
| Current-source Local onboarding and provider workflows | On Tauri commit `ecac630`, all three services were built from Go commit `5274a160` in a disposable worktree with only the test port changed to 52965; 52963 remained the production companion and 52964 belonged to the upgrade qualification process. `npm run test:services` exercised authenticated Local startup, profile/pairing, fake GitHub OAuth browser handoff, fake GitLab credential/tracker/project setup, folder-only project mapping, real Codex and Claude Code responses/tool events, interruption, absent 90-second markers, same-conversation `RESUME_OK`, server restart and profile/project persistence. The temporary services, data and workspace were removed. | Pass for the current Go/Tauri source pair and real signed-in provider CLIs on isolated data. External OAuth exchange, remote tracker writes and a native Tauri GUI cold start were not exercised. The service binaries used a test-only port and were replaced with clean default-port binaries afterward. |
| Tauri + Local companion + embedded backend from this branch | Existing macOS managed-install migration; fresh isolated Tauri Local launch from Go `7dea9140` source with a test-only alternate port; native profile, fake tracker/project, folder association, Codex and Claude Code prompts, tool events and MCP connection. An isolated bundle from Go `560f1f4a` plus Tauri `bc48a02` then cold-started on 52964, connected a Local profile, created a folder project with the synthetic `local` tracker and displayed its prompt-ready board without remote tracker controls. Current-source Tauri UI also reattached to the existing Local workspace on the default port. On 2026-10-29, release-bundle inspection found Go resources from dirty test build `c2f38222` configured for 52964 while saved Local installation state expected 52963. The app rejected the mismatch. Restoring all three packaged services and manifest from clean Go commit `df1d846c` made the open UI report the Local companion and execution ready, then reload 12 projects; all service hashes matched. Neither running companion was restarted. | Isolated macOS startup and provider projection pass; default-port app reattachment and artifact mismatch recovery pass. This does not verify a clean default-port cold start or provider execution. The fixed port contract is also covered by Go tests. |
| Tauri current agent + live Fleet backend | Agent `dev` from Go commit `7dea9140` paired through a temporary one-time code to live server `660-main` (`b8fabf44aa7a`); the local gateway reported connected, `/api/me` matched the pairing owner, and `/api/projects` succeeded. The visual check confirmed Fleet-backed project listing and local-folder/project-link controls. OAuth setup reports unsupported by this Fleet backend version. The workstation page remained at four devices and did not identify a distinct test record, so no existing device was revoked. | Real pairing and authenticated project access pass. The newer isolated visual pairing test verifies Fleet profile email/name hydration in current Tauri; live-backend OAuth support and safe cleanup of a uniquely identified live test device remain open. No tracker writes were attempted |
| Tauri + legacy attached Fleet agent | An isolated `com.sebferry.sectile.tauri.fleetcheck` macOS bundle used the native “attach existing agent” flow, reported `Agent local · backend Fleet`, loaded 19 live projects, and read 390 tickets from the Sectile project (2026-10-29). It attached to the existing v0.7.6 workstation agent without creating tracker writes or replacing the active Local profile. A read-only `/api/me` request through that agent gateway returned `401`, so this legacy connection cannot supply the authenticated user profile required for native API setup. | Live attachment, backend mode display, project discovery and tracker-backed board reads verified. Native setup through this legacy connection still requires a newly paired desktop credential. |
| Existing Electron/Go workstation + live backend | The installed Electron agent is `v0.7.6` (`ac74004102c0`). Its `sectile-agent mcp` bridge connected to live server `660-main` (`b8fabf44aa7a`), listed 21 tools and completed a read-only `list_projects` call (2026-10-29); 315 Electron unit tests and 137 Electron UI tests pass on Go commit `7dea9140` | Live MCP bridge behavior is verified for this installed client/server pair. Current-branch Electron/server compatibility remains unqualified |
| Source-level v0.7.6 protocol fixtures | The current server parses a pinned v0.7.6 agent connection query; a parser copied from the v0.7.6 server source parses the current agent announcement. Focused agent/handler tests also cover operation dispatch. | Pass on this source revision. These tests do not execute either released binary. |
| Released agent binary to current Go server | On 2026-10-29, `python3 scripts/compatibility/test-v076-server-current-agent.py` built the v0.7.6 agent from its tag and connected it to the Go `dev` server at commit `c2f38222`, using isolated homes, SQLite and Git repositories. | Pass for startup, WebSocket handshake, all 27 operations, 21-tool MCP catalog/client calls, STDIO bridge, task/run lifecycle, mocked tracker create/stage update, `skills_status`, `init_git` and task-spec worktree creation. |
| Current agent binary to `v0.7.6` server binary | The same run built the v0.7.6 server from its tag and connected the current agent; the released server accepted all 27 operations with `outdated=false`. | Pass for startup, WebSocket handshake, 21-tool MCP catalog/client calls, STDIO bridge, task/run lifecycle, mocked tracker create/stage update, `skills_status`, `init_git` and task-spec worktree creation. |
| Cross-version MCP clients and servers | The harness starts real `v0.7.6` and current server binaries against separate temporary databases. Each server is exercised by the other version's SDK client: list 21 tools, call `list_projects`, create a GitHub issue through a mock API with the personal token, transition the remote task to `clarified`, verify tracker sync, start/finish a run on a local task, read back persisted statuses and prepare a task specification worktree. | Pass for MCP sessions, catalog, remote issue creation, caller-token attribution, one tracker stage update, run lifecycle, workflow transition and task specification worktree preparation in both directions. Live GitHub/Jira/GitLab behavior, execution tools and Electron registration remain unqualified. |
| Electron/Codex stdio MCP bridge | On Go compatibility harness commit `c2f38222`, `sectile-agent mcp` was launched as a real stdio child through the MCP SDK's command transport against both server binaries. The current agent to v0.7.6 server and v0.7.6 agent to current server each listed all 21 tools and called `list_projects` successfully using temporary credentials. The spawned bridge uses `agentexec.Hidden` for Windows. | Pass for the agent bridge transport and read-only tool call in both tested version pairings. This is the process Electron/Codex register, but does not qualify provider-specific registration, a live Electron UI session or mutations through those clients. |
| Cross-version agent operation dispatch | The same harness connects a current agent to the v0.7.6 server and a released v0.7.6 agent to the current server. Each pairing calls authenticated `skills-status` and `init-git` routes. MCP `prepare_task_spec_worktree` creates a registered, distinct worktree on a task branch through the server-agent WebSocket in both pairings; the test verifies its path, branch and Git registration. | Pass for read, mutation and task specification workspace operations over the server-agent WebSocket in both directions. Cancellation, provider execution and Electron registration remain unqualified. |
| macOS app and DMG packaging | `npm run package:macos` on clean Tauri commit `cc378ac` built a 102.28 MiB arm64 app and 51.68 MiB `Sectile Tauri_0.1.0_aarch64.dmg` with clean Go commit `df1d846c`. The post-build verifier passed on both app and mounted DMG: bundle ID `com.sebferry.sectile.tauri`, version `0.1.0`, all three Go service hashes, companion port 52963, four dependency notices and 52 license files. `hdiutil verify` passed. DMG SHA-256: `eeb502f5ffb33e51c9a755c1d8430749c809d00e603ffa99cdf20cc1078f7873`. | Current macOS package assembly and archive integrity pass. The manifest marks this untagged, unsigned artifact `releaseEligible: false`; signing, notarization, Gatekeeper and installation/update qualification remain open. |
| Current-source arm64 macOS app and DMG | On Tauri commit `f54571c` and clean Go commit `22c33ec19707c400612715e233e4554b77bfb0f2`, `npm run prepare:services` and `npm run package:macos` built a 102.28 MiB arm64 app and 51.69 MiB DMG. The verifier accepted all three service hashes, default port 52963, 52 license files and `goDirty: false`; `hdiutil verify` passed. DMG SHA-256: `7a68498e44235017b36ab7a3380e2f0a488b8ea2108c636886c8b6a52b0603d0`. The manifest reports `releaseEligible: false`. | Current-source app/DMG assembly, resource provenance and archive integrity pass. The artifact is unsigned and unnotarized, has not been installed on a clean user account, and was not launched because the user's companion owns 52963. |
| Updated arm64 macOS app and DMG | On 2026-10-29, Tauri source after `fecaa3b` and clean Go commit `f598412218d4127823d976eacce551faa907c758` produced a 102.28 MiB arm64 app and 51.70 MiB DMG. `npm run prepare:services` and `npm run package:macos` passed; the post-build verifier confirmed all three service hashes, port 52963, 52 license files and `goDirty: false`. `hdiutil verify` passed. DMG SHA-256: `c770aa7e2a7e050aee433fed7718e4b2af58de57bed650b876ee7b0ff99c6c24`. | Current Go token-privacy source is present in the package. The manifest remains `releaseEligible: false`: signing, notarization, Gatekeeper and clean-user installation remain open. The user's companion still owns the fixed port, so the bundle was not launched. |
| PTY supervisor fix in arm64 macOS package | On 2026-10-29, Tauri `ebe0052` rebuilt all three services from clean Go commit `eb485f64ca9d1cb1244100c925cd2f8df83b7f65` and ran `npm run package:macos`. The post-build verifier accepted the app and mounted DMG with matching service hashes, Local port 52963 and 52 license files; `hdiutil verify` passed. The arm64 app is 102.28 MiB, the DMG is 51.70 MiB (54,210,684 bytes), and its SHA-256 is `07ce61b46ceab1abd0f9175912f18a02ae8b6cf100d959c8338bca25df848093`. | Current Go process supervision is present in the package. The untagged manifest remains `releaseEligible: false`; the artifact is unsigned and unnotarized. A clean-user installation and provider run from the package remain open. The existing companion still owns port 52963 and was not restarted. |
| Tracked-tool stop fix in arm64 macOS package | On 2026-10-29, Tauri source at `82006a3` rebuilt all three services from clean Go commit `25a8e77d25c172cb22cfe7a348fca2279af9fe01` and ran `npm run package:macos`. The post-build verifier accepted the app and mounted DMG with matching service hashes, Local port 52963 and 52 license files; `hdiutil verify` passed. The arm64 app is 102.28 MiB, the DMG is 51.70 MiB (54,210,359 bytes), and its SHA-256 is `40cfe98a4477ae40dbdae420aca63fa6ce279e694659f47447f363a372ca4717`. | The Codex delegated-tool stop fix is present in the package, but the packaged app was not launched because the existing companion still owns 52963. The untagged manifest remains `releaseEligible: false`; signing, notarization and clean-user installation remain open. |
| Isolated Local bundle replacement | On 2026-10-29, an app built from Tauri commit `6539dbf` used unique bundle identifier `com.sebferry.sectile.tauri.upgradetest`, a disposable app-data directory and a companion built from Go commit `c2f38222` with only the Local port changed to 52964. The 0.1.0 app created a synthetic Local profile and a folder-backed project in `/tmp/sectile-v1-upgrade-qualification`. After closing it, Tauri rebuilt the app as 0.1.1 at the same bundle path with the same identifier and data directory. The replacement UI showed the fixture project still linked to its temporary folder and the profile screen retained the fixture email and display name. The companion stayed up across replacement; the production companion on 52963 was left running throughout. | Pass for basic app-bundle replacement and Local profile/project/folder retention on this macOS host. This uses the same source on both sides with an app-version change, not a released prior version. It does not cover tracker credentials, OAuth records, conversations, managed-runtime recovery, signed DMG installation or a clean macOS user. Disposable app data, folder and test companion were removed; packaged services were restored from clean Go commit `c2f38222`. |
| V1 desktop update policy | [ADR 0018](adrs/0018-v1-desktop-packaging-and-updates.md) selects signed/notarized DMG releases with user-initiated replacement updates and no in-app updater. An isolated macOS replacement-bundle smoke used the same unique bundle identifier and data directory across Tauri app versions 0.1.0 and 0.1.1. After replacing the first bundle, the second displayed the saved synthetic Local profile, project and associated folder. Its Go companion and SQLite database remained in the isolated data directory during replacement. | The basic profile/project/folder retention path passes on this host. Credentials, OAuth records, conversation history, managed-runtime recovery, installation from a signed DMG and upgrade from a released older version remain open. Signing/notarization and automated release publishing also remain open. |
| Go service cross-builds | On 2026-10-29 at Go commit `3ea74cbb`, built `cmd/companion`, `cmd/agent` and `cmd/server` for both `linux/amd64` and `windows/amd64`; `file` confirmed ELF and PE x86-64 outputs. | Go service compilation passes for both targets. This does not qualify the Tauri package, native startup, migration, process execution or installers on either OS. |
| Optional headless server artifact | At Go commit `7dea9140`, `make build-server-headless` produced `bin/server-headless` without invoking Node. The opt-in `sectile_headless` build retains API/MCP/auth routes and returns 404 for browser paths; isolated runtime smoke verified `/api/health` (200), browser paths (404), and an unauthenticated unknown API route (JSON 401). Both default and tagged `cmd/server`/`internal/webui` tests pass. | Linux/amd64 ELF and Windows/amd64 PE headless binaries cross-compile. This qualifies the optional artifact and its no-web behavior on the current macOS host, not native Linux/Windows runtime or release distribution. |
| Local workspace board metadata | A live Tauri review found that Local projects carry a synthetic `local` tracker row; the older desktop projection dropped provider identity, leaving remote-only ticket filters visible. Go commit `560f1f4a` includes provider per selected tracker; Tauri commit `bc48a02` hides remote controls for Local-only projects and includes a bounded older-agent fallback. An isolated macOS Tauri bundle with a separate app identifier and port started the current Go companion, saved a synthetic Local profile, created a folder-backed project whose DB tracker provider was `local`, and rendered “Votre projet local est prêt” without remote search, grouping or status controls. | End-to-end Local behavior passes in isolation. The existing companion and its data were not touched; disposable app data and the 52964 test process were removed after the check. |
| Native platforms | User-selected V1 support is macOS-first; Linux is experimental; Windows is deferred. macOS Local/Fleet migration and isolated Local startup/provider paths have live desktop evidence. Linux ARM64 `.deb` was built and installed with a visible X11 window under Xvfb/DBus at Tauri `446285c`/Go `7dea9140`; Windows and Linux Go services cross-build. | macOS runtime behavior is qualified except for a clean default-port cold-start/provider rerun and signed/notarized distribution. Linux remains experimental: real user desktop/provider/migration and distro breadth are unqualified. Windows remains deferred after cross-build only. |
| macOS signing | `security find-identity -v -p codesigning` returned zero valid identities on the qualification host (2026-10-29) | No signed macOS artifact can be qualified on this host; a valid signing identity is required |
| Open-source package provenance | The user selected MIT for both repositories. Tauri has an MIT `LICENSE`; Go commit `5f809481` adds the MIT grant to the services repository and updates its README. Go and Tauri packages now include inventories and license texts for their Go, Rust, and JavaScript dependency sets. The Go notice records the `go.mod` hash; Tauri verifies that hash before bundling the selected Go source. | Repository licensing and package notices are implemented and verified in the Go release archive and macOS app bundle. Keep notice regeneration and dependency-graph validation in the release process; signed macOS distribution remains open. |

Current compatibility refresh (2026-10-29, Go commit `df1d846c`): `go test
./...`, the v0.7.6/current server-agent harness, Electron's production renderer
build, all 315 Electron unit tests and all 137 Electron UI tests passed. The
compatibility harness connected both agent/server directions, listed all 21
MCP tools through both stdio bridges, completed task/run/workflow checks and
created/updated fake tracker issues. The Electron UI run used isolated fixtures.
The live Electron-to-current-Go board smoke remains read-only and isolated.
These results qualify the named current-source pair and the tested v0.7.6
mixed pair for those operations; they do not establish a general Fleet version
range or qualify live external tracker writes.

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

Current compatibility refresh (2026-10-29, Go application source commit
`1ee27b4b`, harness commit `c2f38222`):
`go test ./...` passed across all Go packages; the Electron source suite passed
315 unit and 137 UI tests. `python3
scripts/compatibility/test-v076-server-current-agent.py` also passed against
this commit in both directions: the current agent connected to the released
v0.7.6 server, and the released v0.7.6 agent connected to the current server.
The harness exercised the 21-tool MCP catalog, project reads, fake GitHub issue
creation and stage synchronization with a test personal token, run lifecycle,
task status, task-spec worktree creation, and authenticated `skills_status` /
`init_git` agent operations. The actual `sectile-agent mcp` stdio bridge also
listed all 21 tools and called `list_projects` successfully in both pairings.
All databases, homes, tracker endpoints and Git
repositories were isolated and removed by the harness. This is stronger mixed-
binary evidence for the named pair, but does not qualify live external tracker
writes, provider execution, provider-specific Electron/Codex configuration, the
Electron window against this server, or any other released version pair.

Current-source Local services smoke (2026-10-29, Tauri commit `347ca5b`, Go
commit `1ee27b4b`): `npm run prepare:services` built and identified all three
services from the clean Go commit. `npm run test:services` then ran with an
isolated source worktree whose only source difference was the fixed local port
set to 52964. It passed local sign-in, pairing, profile updates, disposable
tracker credentials, OAuth browser handoff and cancellation, fake tracker and
project setup, agent project mapping, companion/server readiness, server
restart and persistence checks. The test's database, app home, repository and
HTTP tracker fixture were disposable; no live provider credentials were used.
The generated services were restored from the clean Go commit afterward. The
existing companion continued listening on 52963 throughout. This qualifies the
current Local backend setup flow in isolation, not a fresh default-port launch
or live provider authorization.

Current-source provider supervision smoke (2026-10-29, Go commit `408cd909`):
the same disposable 52964 test worktree enabled the already signed-in Codex and
Claude Code CLIs. Each answered a prompt and emitted a tracked tool call. The
interrupt test started a shell command with a 90-second delayed file write,
interrupted the run, waited 95 seconds, and verified the file did not exist.
Both original conversations resumed and confirmed the missing file with
`RESUME_OK`. The isolated onboarding/restart checks then completed; the test
worktree, profile, database, repository and services were cleaned up, and the
Tauri service binaries were rebuilt from clean Go commit `408cd909`. This
qualifies Go-side provider supervision and persistence, not the Tauri GUI
rendering or a production-port process restart.

Until these gates close, compatibility claims are limited to the tested source
revision and platform evidence above. The versioned server/agent contract is
additive within V1, but that rule alone does not prove any particular mixed
binary pair. V1 must publish a tested server/agent version matrix and desktop
platform support table before release.

### V1 compatibility claim rule

V1 support is declared per exact Tauri, Go server and Go agent release
combination; do not infer support for an untested version range from additive
protocol rules. Local mode is paired to the exact Go companion source commit in
the Tauri package manifest. Fleet mode requires an explicitly listed server
and agent pair for the features that release exposes. A pair that passes MCP or
agent protocol checks alone is compatibility-tested for those operations, not
qualified for all native Fleet setup.

The Go `df1d846c`/v0.7.6 mixed pair passes the common 21-tool MCP catalog, selected
workflow operations, agent dispatch and the real stdio MCP bridge in both
directions. That evidence does not qualify the older server's OAuth
administration or the complete native Tauri setup surface. Until a tagged Go
backend and its Tauri Fleet setup are qualified together, no Go version range
is declared as generally supported by a V1 Fleet desktop.

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

Stage 7's ownership decision and the UX convergence plan are complete. Before
V1 release, publish the exact supported Fleet pair (or list Fleet setup as
unsupported for unqualified server versions), qualify macOS signing and
notarization, complete clean-user and full-state upgrade installation, and
close the remaining Fleet/Electron/live-provider checks. Third-party notices
are included in the Go release archive and Tauri bundle; keep them synchronized
with dependency changes. These gates do not authorize changes to the existing
web application.

## 8. Release meaning

V1.0.0 names the product destination and its eventual compatibility commitment.
This plan does not bump manifests, create a tag or imply that V1 is the next
release. Cut a release only through the Go repository's documented tag/changelog
procedure after the V1 scope and readiness criteria are met and approved.
