# Sectile Tauri V1 release checklist

This checklist qualifies a future V1 release. It does not authorize publishing
or create a Git tag.

## Confirmed release scope

The [candidate support matrix](v1-support-matrix.md) lists the exact behavior
demonstrated so far and the combinations that remain unqualified.
The [Local runtime measurements](v1-runtime-measurements.md) record an isolated
service-only comparison; release-binary desktop performance remains open.

- Tauri is the V1 desktop for operational boards, prompts and agentic execution.
- The existing web application remains the V1 owner for product planning and is
  unchanged. Electron remains a compatibility client until its support window
  is separately decided.
- The desktop and Go service repositories use MIT licensing. Include their
  dependency notices with every distributed package.
- macOS is the V1 platform target. Linux remains experimental; Windows is
  deferred.
- V1 desktop updates are manual downloads of a signed and notarized DMG. There
  is no in-app or automatic updater. Keep the bundle identifier and user-data
  layout stable across releases.

## Source and compatibility

On 2026-10-30 the current/v0.7.6 binary harness passed again with clean Go
`84e19b1b`: both agent/server directions, 27 operations, 21 MCP tools,
direct/STDIO clients, run lifecycle, task-spec worktrees and fake tracker writes.
The newer interactive Tauri tracker conversation path remains limited to
servers exposing the explicit launch `runId`; protocol compatibility alone
does not establish that native feature on v0.7.6.

The v0.7.6/current Go binary harness was refreshed on 2026-10-29 against
clean Go `554374b2`. Both agent/server directions passed the 27-operation
agent protocol, 21 MCP tools through direct and STDIO clients, run lifecycle,
task-spec worktree creation and fake GitHub issue/stage writes using a personal
token. The isolated test left the Go checkout clean. Later isolated Go API tests
created, updated, read back and deleted real GitHub and GitLab issues. Fresh
provider OAuth consent and native tracker writes remain separate release gates.

An isolated native Local retake with Tauri `317151c` and Go `5500840d`
verified that Claude Code and Codex conversations show French Sectile-authored
start and ready notices while returning their expected provider responses.
The test used a disposable companion port and data directory; it did not
exercise OAuth or tracker writes.

- [ ] Select and record the exact Go source commit used to build the companion.
- [ ] Build only from clean, reviewable source commits. Confirm the packaged
  `build-manifest.json` names the selected Go commit, reports `dirty: false`,
  and lists hashes for all three service binaries.
- [ ] Confirm the Tauri notice generator accepts the Go dependency graph hash;
  inspect the Go, Rust and JavaScript notice inventories and included license
  texts.
- [ ] Record the Tauri release commit, Go release commit, app version, package
  architecture and supported server/agent combinations in the release notes.
- [ ] List only exact Tauri/server/agent combinations qualified for the
  features enabled in that release. Do not promote the v0.7.6/current protocol
  tests into a general Fleet setup support claim.
- [ ] Run the Go suite, Tauri TypeScript/domain/UI/Rust suites, Electron unit/UI
  suites and the cross-version compatibility harness. Keep the tested pairings
  and their limitations in the [migration plan](sectile-v1-migration-plan.md).
- [ ] Verify current Local and Fleet setup, project/folder setup, OAuth
  callbacks, MCP access, Claude Code and Codex responses, tool events,
  interruption and conversation resume on the release binaries.

## macOS package and upgrade

- [ ] Build the arm64 app and DMG from the selected release commits with
  `npm run package:macos`; confirm the post-build manifest/hash verifier passes.
  Add an x86_64 artifact only after native/package qualification for that
  architecture.
- [ ] Sign the Go executables and the complete `.app` with a valid Developer ID
  identity. Verify the signature and Gatekeeper assessment.
- [ ] Notarize the app/DMG, staple the notarization ticket and verify it on a
  clean macOS account.
- [ ] Verify the DMG filesystem image and publish a SHA-256 checksum.
- [ ] Install the DMG in a disposable macOS user profile, launch the app, and
  exercise Local setup and a provider prompt.
- [ ] Replace an earlier Tauri app bundle with the release app. Verify that the
  Local database, credentials, OAuth records, projects, folders and conversation
  history remain readable and that managed-runtime migration recovers safely.
  An isolated macOS bundle replacement smoke on 2026-10-29 verified a saved
  Local profile, folder-backed project and folder association across bundles
  with the same identifier and data directory. Credentials, OAuth records,
  conversation upgrade from a released older version and managed-runtime
  recovery inside a release bundle remain unverified; see the migration plan.

  Go commit `b907c42f` now checks migration recovery at every durable boundary
  with a personal tracker token, encrypted Jira OAuth app, project, ticket and
  run-store snapshot/journal. This is source-level retention evidence; it does
  not replace the release-bundle upgrade check or provider conversation resume.
  A separate unsigned arm64 bundle replacement on 2026-10-29 used one disposable
  app identity and data directory with test versions 0.1.0 and 0.1.1. A real
  Codex conversation created in the first bundle returned `FIRST_TURN_OK`; the
  replacement restored its folder, transcript, `gpt-6-astra` model and
  execution mode. A second turn recalled the first turn's marker. This tests
  same-source bundle replacement with Go `b907c42f`, not a released-version
  upgrade, credential/OAuth retention in the native app, or signed installation.
  Packaged recovery check (2026-10-30): the candidate's exact Go `84e19b1b`
  companion executed `--upgrade --data-dir` against disposable installations
  prepared at all six durable interruption boundaries. The opt-in harness at Go
  `6603b78b` verified identity/API token, encryption key, personal tracker token,
  encrypted Jira OAuth app, unexpired synthetic Jira/GitHub grants, projects,
  ticket, snapshot/journal bytes, backup and the legacy-database fence. All six
  packaged recoveries and the normal source companion suite passed. No listener
  was started and the regular companion remained active. This establishes
  packaged recovery from source-prepared checkpoints; it does not test a native
  installer, process-kill timing inside that binary, real grant refresh rotation
  or provider-session resumption through a bundle replacement.
- [ ] Confirm manual update instructions and release notes identify required
  migration behavior and any incompatible Fleet server/agent combinations.

## Platform and release gates

- [ ] Publish the macOS support statement and explicitly label Linux
  experimental. Do not publish a Windows support claim.
- [ ] Re-run the current-source Local companion cold start on its fixed default
  port only when it can be done without interrupting a user's existing
  companion. Verify the release binary hash and process cleanup.
- [ ] Close the remaining Fleet/Electron compatibility and live-provider gates
  listed in the migration plan, or state their exact unsupported combinations.
- [ ] Confirm all current and newly generated release assets are present and
  their checksums match the published manifest.
- [ ] Obtain explicit owner approval of the exact version, changelog and
  artifacts before creating or pushing the release tag.

## Current qualification status

Live tracker writes (2026-10-29): `npm run test:live-trackers` used a disposable
Go `45940914` server on port 52965 with personal tokens and the user-approved
`sebastienferry/sectile` GitHub repository and
`smartadserver/private/shared-platform/taskativ` GitLab project. Sectile
created GitHub issue #898 and GitLab issue !112, queued a title change for
each, and a direct provider read confirmed the changed titles. The test
deleted both issues, and independent provider reads confirmed their absence.
The private server and database were removed. This qualifies these Go
personal-token write paths, not fresh OAuth consent, the native Tauri form,
Jira writes or a deployed Fleet installation.

The existing-project tracker association was exercised on 2026-10-29 against
Go `45940914` through an isolated server and agent on port 52965. A folder-only
project kept its agent folder mapping after joining a registered GitLab fixture
tracker and after the test server restarted; its tracker selection and default
also persisted. This qualifies the Go API contract used by the native action,
not a live tracker import or a packaged Tauri UI interaction.

An isolated unsigned Tauri app at `5e44fde` paired with a disposable Fleet
backend built from Go `45940914`. In the native project configuration it added
the registered GitLab fixture tracker to an existing folder-only project. The
screen showed the linked tracker, and a backend read confirmed its selection
and default without creating a second project. This checks the native action
against a current Go backend; it does not cover a real import, a folder picker
in that same test or the post-capture layout correction.
The same isolated application was rebuilt at `2a1e59e`, reopened against the
same temporary backend, and showed the selected tracker with a completed-state
message and no empty selector or native fieldset border. The app, its agent,
backend, private data and test bundle were stopped or removed afterward; the
regular Local companion remained on port 52963.

Native Local folder-to-tracker qualification (2026-10-29): an unsigned Tauri
`d0ecceb` app used a unique bundle ID and a disposable Go `45940914` companion
whose only source change was test port 55466. From the native window it started
Local, saved a test profile, opened an empty folder as a prompt-ready project,
registered a GitHub fixture source, and attached that source from the project
configuration. An authenticated API read showed one selected tracker, the same
project ID and folder mapping. After quitting the desktop and stopping the
companion, reopening the app restarted the companion and restored the same
project ID, folder and tracker selection. A free Codex prompt in that project
returned `LOCAL_TRACKER_OK` with the correct working directory and MCP shown
connected. The fixture tracker was intentionally not given a credential or
synced, so this is not evidence of ticket import or tracker writes. The test
app, companion, data and worktrees were removed; port 52963 was untouched.

The binary compatibility harness was re-run on 2026-10-29 against Go commit
`c2f38222` and the released v0.7.6 server/agent pair. Both directions passed
21-tool MCP, STDIO bridge, mocked GitHub create/stage update, task/run lifecycle,
`skills_status`, `init_git` and task-spec worktree checks. This closes that
specific protocol slice; it does not close full Electron UI compatibility,
live tracker/provider behavior or Fleet reconnection.

Current source test refresh (2026-10-29): Go commit `22c33ec1` passes `go test
./...`. Electron passes 315 unit tests and 137 UI tests. Tauri commit `f02e511`
passes `npm test` (14 tests), `npm run test:ui` (101 tests), `npm run check`,
`npm run test:rust` (35 tests) and `npm run build`. The compatibility harness
and live service/provider qualifications are recorded in the migration plan;
they cover only the named source pairings and operations, not every Fleet,
Electron, OAuth or external tracker combination.

The 2026-10-29 source refresh at Go `45940914` and Tauri `c08db18` passes
`go test ./...`, Electron build, 315 unit tests and 137 UI tests, and Tauri
TypeScript check, production build, 15 domain tests, 127 UI tests and 36 Rust
tests. The v0.7.6/current binary harness also passes in both directions:
27 agent operations, 21 MCP tools, STDIO bridge reads, task/run lifecycle,
task-spec worktrees and mocked personal-token tracker creation and transition.
The Electron renderer build is 1,392.14 kB JavaScript (428.06 kB gzip) and
retains its chunk-size warning. These are source and fixture checks; the
release-binary, external OAuth, live native tracker write and signing gates
remain open.

An isolated clean worktree at Tauri `4bcb4e2` bundled clean Go `45940914`
without replacing the user's running desktop. `npm run package:macos` and the
resource verifier passed for the 103.31 MiB arm64 app and 56,331,700-byte DMG.
Both bundles contain the fixed port 52963 companion and 52 license files;
`hdiutil verify` passed. DMG SHA-256:
`02f31a53ac8f23adb3d5e6491c52c011111af054619266fd618d701df89b665a`.
The manifest reports `dirty: false` and `releaseEligible: false` because this
is an untagged development build. Signing, notarization, clean-user installation
and release-binary provider execution remain open. `codesign -dv` reports an
ad hoc linker signature with no Team ID; `spctl --assess` rejects the app.

Current-source Local provider qualification (2026-10-29): Tauri commit
`ecac630` ran `npm run test:services` with all three services built from Go
commit `5274a160` in a disposable worktree whose only source difference was a
test port of 52965. The test verified Local startup, profile and pairing, fake
GitHub OAuth browser handoff, fake GitLab credentials/tracker/project setup,
folder-only project mapping, real Claude Code and Codex replies/tool events,
interruption with each delayed marker absent after 95 seconds, same-conversation
resume, and persistence across a server restart. No external OAuth exchange or
remote tracker write was made; the Tauri GUI was not part of this service test.
Temporary data and services were removed. This closes that current-source Local
service/provider slice, not release-binary qualification.

Electron Codex Stop qualification (2026-10-29): Electron `0.7.5` connected to a
disposable server and agent built from Go `22c33ec1` on test port 52965. A
project prompt launched real Codex `0.165.0`; its shell created the start
marker, Electron Stop marked the run canceled, and the delayed marker remained
absent 104.6 seconds after the start. This closes the built-in Electron
project-prompt cancellation slice; tracked-ticket and Fleet execution remain
open.

Go PTY credential qualification (2026-10-29): commit `f5984122` keeps the
temporary run-control token out of the console command and provider child
environment. A real wrapped PTY test checks both surfaces, and `go test ./...`
passes. An updated arm64 app/DMG bundles this Go commit, as recorded below.

Current-source package verification (2026-10-29): clean Go commit
`22c33ec19707c400612715e233e4554b77bfb0f2` and Tauri commit `f54571c`
produced a 102.28 MiB arm64 app and 51.69 MiB DMG.
The manifest names the clean Go source, all three service hashes, fixed port
52963, 52 license files and `releaseEligible: false`. The verifier and
`hdiutil verify` passed. DMG SHA-256:
`7a68498e44235017b36ab7a3380e2f0a488b8ea2108c636886c8b6a52b0603d0`. The
manifest reports `releaseEligible: false`; `security find-identity -v -p
codesigning` found zero valid identities. The production companion owns the
fixed port, so a clean default-port launch was not attempted. Signing,
notarization, Gatekeeper, clean-user installation, complete upgrade retention,
full Fleet/Electron workflows and live tracker/provider qualification remain
open. Linux remains experimental and Windows is deferred; neither is a V1
qualification gate. No V1 release is ready to publish.

Updated package verification (2026-10-29): Tauri source after `fecaa3b` and
clean Go commit `f598412218d4127823d976eacce551faa907c758` produced a
102.28 MiB arm64 app and 51.70 MiB DMG. The post-build verifier and
`hdiutil verify` passed; the manifest reports default port 52963, 52 license
files, `goDirty: false` and `releaseEligible: false`. DMG SHA-256:
`c770aa7e2a7e050aee433fed7718e4b2af58de57bed650b876ee7b0ff99c6c24`.
Signing, notarization and clean-user installation remain open.

Tracked-tool cancellation and package refresh (2026-10-29): Go commit
`25a8e77d25c172cb22cfe7a348fca2279af9fe01` passed `go test ./...` and a
real Electron tracked-ticket Stop check with signed-in Codex `0.165.0`. Its
shell tool wrote a start marker, then was stopped from the Electron window;
the file scheduled after 90 seconds was absent after 170 seconds. Clean Go
services from that commit were bundled into the arm64 Tauri app and DMG.
`npm run package:macos`, the resource verifier and `hdiutil verify` passed;
the DMG is 54,210,359 bytes with SHA-256
`40cfe98a4477ae40dbdae420aca63fa6ce279e694659f47447f363a372ca4717`.
The manifest still reports `releaseEligible: false`, and signing,
notarization, clean-user installation and a provider run from this package
remain open.

Source and compatibility refresh (2026-10-29): Go `25a8e77d` passed the full
Go suite; Tauri source at `2bb47fb` passed TypeScript, 14 domain tests, 101 UI
tests and 35 Rust tests; Electron passed 315 unit tests and 137 UI tests. The
current/v0.7.6 binary compatibility harness passed both directions with 27
agent operations, 21 MCP tools, STDIO bridges, run lifecycle and fake tracker
creation/stage writes. These checks use isolated data. No Developer ID
Application identity is installed on the test Mac, so signing and notarization
cannot be qualified until one is provisioned.

Branded package check (2026-10-29): clean Tauri `041300f8` and clean Go
`25a8e77d` produced an arm64 app and DMG. The macOS
bundle's `icon.icns` has the same SHA-256 as the Electron Sectile icon;
`npm run package:macos`, the app/DMG resource verifier and `hdiutil verify`
passed. The DMG is 56,309,471 bytes with SHA-256
`d84bd9c7705022aac6c5378d56c75ac8be6e584d6ae662d5fe9479b620e6f3ca`.
Its manifest reports `releaseEligible: false`;
signing, notarization and clean-user installation remain open.

Desktop OAuth contract check (2026-10-29): focused Go handler tests passed
complete fake-provider GitHub, GitLab and Jira callback exchanges and the fixed
Local Jira callback contract. Real provider consent and tracker reads from a
signed desktop package remain open.

Default-port bootstrap check (2026-10-29): the packaged companion built from
clean Go `fe3a98c8` accepted an authenticated Local runtime probe on 52963 in
174 ms and released the port after shutdown. It used disposable data and no
Tauri window or paired execution agent. Full release-binary cold start,
provider execution and clean-user installation remain open.

Live Local Jira check (2026-10-29): the same Go companion started against the
existing Local installation and completed an authenticated PE Jira sync with
1,988 issues. A read-only database query confirmed the same number attached
to the tracker. The companion was stopped and port 52963 released. This
qualifies the existing OAuth grant's Local read path with Go `fe3a98c8`, but
not a fresh consent, the native Tauri interaction, Fleet or signed packaging.

Current-source unsigned macOS package (2026-10-29): clean Tauri `f479b86` and
clean Go `fe3a98c8` produced an arm64 app of 103.30 MiB and a 56,315,588-byte
DMG. The post-build app/DMG resource verifier and `hdiutil verify` passed;
the manifest names fixed port 52963, 52 license files, `goDirty: false` and
`releaseEligible: false`. DMG SHA-256:
`5961bb4d12a27dbd323b99fd9d2d9b2bfaf3851bcab29c1cf7ebb65943450031`.
Signing, notarization, Gatekeeper, clean-user installation and native UI
qualification of the real Jira result remain open.

Updated unsigned package check (2026-10-29): clean Tauri `109c18e` and Go
`b907c42f` produced a 103.30 MiB arm64 app and a 56,316,296-byte DMG.
Resource verification and `hdiutil verify` passed with fixed port 52963,
52 license files, `goDirty: false` and `releaseEligible: false`. DMG SHA-256:
`fa8c249635387a7c6143180c1d5796fb0985aa3741f1666c6d9ad9db9f56c452`.
Go `554374b2` subsequently fixed a misleading idle-restart notice; this
package does not include that later source change. A refreshed package with
that Go commit is recorded below.

Latest unsigned package check (2026-10-29): the Tauri application code from
`109c18e` and clean Go `554374b2` produced a 103.30 MiB arm64 app and a
56,317,277-byte DMG. Resource verification and `hdiutil verify` passed with
fixed port 52963, 52 license files, `goDirty: false` and
`releaseEligible: false`. DMG SHA-256:
`25c65d90105d064b114daa16877647017ad2e694a0135d62f74173014481809b`.
An isolated native app using the same Go commit completed an idle Codex turn,
then restored its project folder, answer, `gpt-6-astra` model and Execution
mode after the desktop and companion were stopped. No false pause notice
appeared in the restored transcript. This checks the restart UX fix; it is
not a signed installation or an upgrade from a released version. The
transcript correction also passes the full Go agent and companion package
suites.

Native Local Jira sync check (2026-10-29): the current unsigned macOS app,
bundling Go `554374b2`, used the existing Jira OAuth grant to request a PE
sync from Accounts and trackers. The tracker row showed the in-progress state
and then “1988 tickets synchronisés.” The PE board displayed 1,988 tickets
in Jira status columns. The app and its package-owned companion were stopped
and port 52963 released. This verifies the current native sync/result/board
path with an existing Local grant; fresh OAuth consent, Fleet and signed
distribution remain open.

Native Claude Code replacement check (2026-10-29): an isolated app built from
clean Tauri `3ae07ab` and Go `554374b2` created a folder-backed Local project
and completed a Claude Code turn with `claude-sonnet-5` in Execution mode.
Rebuilding the same test bundle identifier from version 0.1.0 to 0.1.1, with
both app and companion stopped between launches, retained the project folder,
history, model and mode. A resumed turn recalled a marker supplied only in the
first prompt. Test app/data/folder were removed and port 52963 released. This
qualifies the same-source Claude conversation replacement slice, alongside the
earlier Codex slice; released-version migration, credentials and signed DMG
installation remain open.

Electron source regression refresh (2026-10-29): Go branch `fe3a98c8`
remained clean while the Electron source passed its 315 unit tests, 137 UI
tests and production Vite build. The UI tests cover board, console, workflow,
settings and MCP fixtures. This is a source regression check, not a live
Electron session paired with the current Go binary or a remote tracker write.

Live tracker qualification (2026-10-29): an isolated current Go API server
created and updated GitHub `sebastienferry/sectile` issue `#896` and GitLab
`smartadserver/private/shared-platform/taskativ` issue `!110`. It waited for
the queued tracker activities, and direct forge reads confirmed the titles.
Both issues were deleted. A second run against Go `e1b80fe8` saved the GitLab
website address without `/api/v4`, then created and updated GitLab issue `!111`
and GitHub issue `#897`; both were deleted. The regular companion on 52963 was
untouched. The native UI, fresh OAuth consent, Jira writes, deployed Fleet
backend and signed package are not qualified by these API tests.

Native GitLab setup and read (2026-10-29): an isolated unsigned Tauri app from
`a792fdb` and Go `45940914` used a unique bundle ID, disposable data and
test-only port 55466. Its native window created a Local profile, GitLab source
from the website URL, and tracker-backed project. Settings showed a completed
sync of 106 live `taskativ` tickets, and the project board displayed them in
remote status columns. The source stored the normalized `/api/v4` URL, and
the folderless project's prompt offered a native folder picker. The personal
GitLab token was injected through the isolated backend API, so native account
connection, OAuth consent and tracker writes remain open. The test app and
companion were removed without touching the regular 52963 companion.

Native GitHub clarification (2026-10-30): Tauri `0711eb8` with Go `920e313f`
services and a test-only port `55466` launched Codex from a real fixture ticket.
The desktop accepted MCP approvals, showed tool activity, sent a confirmation
follow-up and displayed a server-confirmed clarified result. Direct GitHub
readback confirmed the `#clarified` label. No repository file changed or PR was
created. The full specification/implementation/review chain remains open; a
refused attempt to restart the completed first round is recorded in the UX
audit. This is an unsigned isolated test, not release-binary qualification.

Standard native chain (2026-10-30): Tauri `51e93f2` and Go `84e19b1b` passed a
Claude Code/Sonnet standard pickup chain through clarification, OpenSpec,
implementation and review against a fresh loopback GitLab/Git HTTP fixture.
The successful retry preserved accepted commits after fixture CLI/auth repairs.
Server completion, `#reviewed` backend/remote labels, a passing Python unittest,
one ready unmerged MR, matching pushed HEAD and a clean worktree were verified.
The native review displayed all seven changed files. This closes that controlled
native chain slice; real GitLab, Codex chain and signed-release checks remain
separate. See the UX audit for exact run/commit identities and retry limitations.

Clean-source candidate refresh (2026-10-30): Tauri `4063b47` with clean Go
`84e19b1b` passed TypeScript/build, 16 domain tests, 135 UI tests, 38 Rust tests,
the complete Go suite and the current/v0.7.6 binary compatibility harness.
An isolated Cargo target produced the arm64 app and DMG without replacing the
regular running bundle. Package verification confirmed matching Go service
hashes in app and mounted DMG, fixed port 52963, arm64 executables and 52
license files. `hdiutil verify` passed. The DMG is 56,346,015 bytes with SHA-256
`0af7754b9fd2c3e7b946f06eb289dd85a657cd2ce1c83d117084a15f9445a9a5`.
The retained local artifact directory is
`src-tauri/target/candidates/2026-10-30-4063b47/`, containing the app, DMG,
`candidate.json`, `SHA256SUMS` and testing notes. Its signature is ad hoc, with
no Developer ID or notarization; `releaseEligible` is false. The app was not
launched against the user's existing installation. A running older companion
can remain attached, so opening the new UI alone must not be counted as a
runtime upgrade test. This is a local testing candidate, not a V1 release or
a closure of the remaining runtime/migration/integration gates.


Packaged Fleet crash recovery (2026-10-30): `npm run test:fleet-crash` passed
with both signed-in providers using the candidate's exact Go `84e19b1b`
server and agent executables. The isolated Fleet server used loopback port
61848; Local discovery correctly returned 404. Codex (`gpt-6-astra`, execution)
and Claude Code (`sonnet`, execution) each answered a first turn and started a
90-second delayed shell write. SIGKILL of the fixture agent stopped the tool:
the delayed file was absent after 95 seconds. Restart restored the history,
model and mode; a follow-up recalled the context and checked the file without
replaying the original tool. Both provider checks passed and the fixture
cleaned up its processes and data. The regular companion on 52963 (PID 89986)
remained running. The harness now allocates a Fleet port without rebuilding or
changing the shipped Local callback port. This qualifies packaged Fleet
conversation recovery through the agent API; it does not qualify native Fleet
onboarding, remote tracker writes, OAuth consent or a signed installation.


### Native Fleet MCP isolation correction (2026-10-30)

Preparing a native Codex chain exposed a gap outside the previously qualified
Fleet setup slice: managed Fleet launch did not supply a process-scoped MCP
endpoint. Provider sessions could read the shared Electron registration, while
explicit skill/MCP preparation could bootstrap it. Tauri now passes its selected
origin through `SECTILE_DESKTOP_MCP_URL` and disables shared refresh. Go
`9b0b68f6` accepts HTTPS origins or HTTP loopback for this explicit opt-in,
preserves the older Local-only endpoint contract, suppresses shared refresh and
uses the daemon's paired credential privately. External attached agents retain
their own configuration.

`npm run test:fleet-mcp` passed for real Codex (`gpt-6-astra`) and Claude Code
(`sonnet`) against packaged server `84e19b1b` and a development build of the Go
fix on isolated port 64223. Each called `get_project_context` through the scoped
server, received the actual fixture project and used it in its response. Shared
Claude and Codex MCP tables remained unchanged. The first harness attempt
compared the entire Codex preferences file and failed after the tool/response
checks; the corrected harness parses all MCP tables independently of unrelated
preferences, using Python TOML parsing. No MCP fields are excluded. Fixtures
were removed; the regular companion on 52963 remained running.

Argument/environment, no-shared-write and endpoint-validation regressions pass;
Tauri TypeScript, 16 domain tests, 135 UI tests, production build and 39 Rust
tests pass. This is real provider/API evidence and native-launch source evidence,
not a native Fleet full-chain completion. The retained `4063b47`/`84e19b1b`
candidate predates this fix; a rebuilt native package must be tested before
claiming it includes the correction. Existing managed agents need a restart to
acquire the new configuration; external agents are never silently restarted.


Native managed Fleet and replacement verification (2026-10-30): clean Tauri
`8d8dd70` with packaged Go `9b0b68f6` used a unique bundle identifier and a
disposable backend on loopback port 49663. Native pairing automatically started
its private agent. Selecting a folder through the native picker created the
folder-only project and opened its prompt. Real Codex (`gpt-6-astra`,
workspace-write) completed a scoped MCP project-context read after one-time
native approval. Real Claude Code (`sonnet`, default permission mode, displayed
as execution) completed the same read after tool discovery. Both returned the
actual project name and showed completed tool activity and connected MCP state.
Codex survived actual desktop quit/reopen and recalled the name in a follow-up;
its one tool call was not replayed. Replacing only this test bundle with Tauri
`d25915f`, keeping the agent/backend running, preserved Claude's exact history
prefix, folder, model, mode and context. Its native follow-up recalled the
project without additional tool calls. The rebuilt Codex history visibly showed
“Authorization: request allowed once” in French instead of the raw `allow`
value. Packaged service hashes matched the clean Go manifest. The complete Go
suite and Tauri TypeScript, 16 domain tests, 136 UI tests, production build and
39 Rust tests passed. These results qualify this isolated native Fleet
conversation/bundle-replacement slice, not a released-version Local migration,
remote TLS deployment, OAuth consent, tracker chain or signed installation.

The disposable native app, managed agent, backend, private state and bundle were
removed after verification. The regular Local companion (PID 89986, port 52963)
remained running throughout. The retained candidate DMG still names the older
`4063b47`/`84e19b1b` pairing; it has not been silently replaced with this test app.


## Autonomous Codex chain and crash-safety correction (2026-10-30)

A controlled service-level check used real Codex CLI 0.165.0 / `gpt-6-astra`,
installed standard skills, the scoped Fleet MCP bridge, a loopback GitLab test
double and a real Git HTTP origin. No project skill override or real forge was
used. Initial setup required the Fleet server's synthetic read credential and
private glab configuration; personal write credentials alone do not authorize
Fleet server-wide synchronization.

The first run exposed a product defect: plain `codex exec` inherited read-only
permissions and could not answer MCP approvals, then exited zero at New. Go
`88154064` changes the built-in autonomous launch to `--approve-for-me --json`,
retaining Codex's workspace sandbox and automatic review, and normalizes Codex
JSONL into the existing local execution trace. Configured templates and
interactive conversations keep their own settings. Go `72119df5` additionally
preserves top-level Codex error messages before decoding Claude's different
message shape. See Go ADR 0069 and the official
[non-interactive documentation](https://learn.chatgpt.com/docs/non-interactive-mode).

The corrected implementation run `5e218b39-271f-40fd-a7ad-1e34a382b28d`
accepted clarification, specification and implementation. Review correctly
stopped on malformed fixture feedback responses. A continuation preserved those
commits and completed the review checklist, then refused to claim Reviewed when
the fixture ignored MR-description updates. After repairing only those fixture
HTTP contracts, run `fc4dcc3a-ea06-4476-93ad-ede44f171baf` completed Reviewed
through MCP. No stage was advanced through a direct database or fixture edit.

Independent checks confirmed three passing Python unittests, an additional
`greet("Ada") == "Hello, Ada!"` assertion, one ready open MR, an open issue,
`#reviewed` in both backend and tracker, a clean worktree and matching local,
pushed and MR HEAD `ab7668688248f7d60f51ad7e9563825ea86735de`. The implementation
commit remains an ancestor; the only extra commit completes the review checklist.
The default branch was unchanged. A real WebSocket subscriber received 33 live
trace frames, including 29 tool-progress lines and four stage-report lines.
Shared Claude/Codex MCP tables remained unchanged. This is a standard Codex
runtime chain with fixture-repair retries, not an uninterrupted first pass or a
native-button launch qualification. The owner deferred the locked-Mac visual
check; do not restart the unlock request.

A separate regression exposed autonomous process ownership: `StartDetached`
stopped explicit cancellations but allowed a delayed tool write after owner
SIGKILL. Go `85da2f24` applies the conversation lifetime guard to autonomous runs
and checkpoints their complete identity before provider startup. A fast Claude
run had also disappeared before the five-second checkpoint; a configured store
failing its initial write now refuses execution. Ordinary autonomous trace tails
remain periodic; this is not conversation-style per-event journaling.

Real custom autonomous tasks then ran a one-shot shell command with a ready file,
90-second sleep and delayed write. SIGKILL of the test agent prevented every
delayed-file variant after 95 seconds for Codex (`gpt-6-astra`, run
`91c51d3f-092f-486a-8615-49b351459357`) and Claude Code (`sonnet`, run
`5025ce23-df93-420f-acc6-0fc90bba17a7`). Restart restored each run as terminal,
with provider/model retained, unchanged ready-file timestamp, no tool replay and
the completed workflow history still present. Shared MCP tables remained intact.
These were development binaries of the committed fixes against packaged Go
`9b0b68f6` server services; the newly packaged source pairing requires its own
artifact verification. All Go packages passed, including startup-write ordering,
storage-failure refusal, owner-SIGKILL, explicit cancellation and trace routing.


## Refreshed unsigned candidate (2026-10-30)

Tauri `c4258210c209bc58be02cf7aaef69c382043931c` and clean Go
`85da2f243a9792bb76660b47e68b37a788e201d6` were packaged in an isolated detached
worktree, without replacing the regular app or companion. The arm64 app/DMG
verifier passed for embedded service hashes, architecture, fixed port 52963 and
52 license files; `hdiutil verify` passed. The DMG is 56,356,912 bytes with SHA-256
`b9d3bd3e1bef85e84adcecd357da94cbc1e54ac411ea8f9c3e12039cd629d0d0`.
The retained artifact directory is `src-tauri/target/candidates/2026-10-30-c425821/`
and contains the app, DMG, manifest, checksums and scope notes. It has no Developer
ID signature or notarization and is not release-eligible. No release/tag/version
bump was performed.

The exact packaged Go `85da2f24` companion also recovered the six source-prepared
durable migration boundaries, preserving installation identity, API credential,
encryption key, personal token, encrypted OAuth app/grants, projects, ticket,
snapshot/journal bytes, backup and the legacy DB fence. This remains packaged
checkpoint recovery, not a native installer, real grant refresh or arbitrary
process-kill timing inside migration.

The current/v0.7.6 compatibility harness passed again against source `85da2f24`:
27 operations, 21 MCP tools, both direct/STDIO directions, run lifecycle,
task-spec worktrees and fake tracker writes with personal credentials. It does
not qualify older servers for the connection-bound native tracker conversation
path requiring explicit `runId`, nor general Fleet OAuth/setup support.

Native inspection and install/service-replacement checks were explicitly
postponed by the owner. An isolated app with a distinct identity and the same Go
service hashes was prepared without launching it. Test backend/forge processes
were stopped; private fixture state is retained for that deferred check. The
temporary package worktree and stopped failed setup fixtures were removed. The
regular companion on port 52963 (PID 89986) remained untouched. Opening the new
UI while an older companion is running must not be counted as a service upgrade.


### Manual Local update preparation (2026-10-30)

Source inspection confirmed that `ensure_server` accepts a ready authenticated
protocol-1 companion without comparing its binary fingerprint to the bundle.
Closing Tauri keeps that process alive. The new source-checkout `stop:local`
command requests an authenticated idle shutdown and waits for connection refusal
from both Local transports. Nine HTTP fixture tests cover graceful closure,
busy refusal, Fleet exclusion, unknown generation, remote discovery, non-companion
or mismatched service, redirects, incomplete shutdown and explicit directory
selection. The existing agent tests cover shutdown/restart refusal semantics.
The README now explains the required order: finish runs, quit all desktop
windows, stop the idle Local services, replace the bundle and reopen.

This does not close the native update gate: no regular companion was stopped,
and native preparation, full process exit, clean-user installation and bundle
replacement remain unqualified for the refreshed candidate. The source command
is not included in the standalone app and is not a substitute for its V1 UX.


### Native Local preparation implementation (2026-10-30)

The configured Local Services action now invokes the bundled Go preparation
role, verifies store ownership release as well as both transport closures, and
quits Tauri on success. Automatic startup is fenced during preparation and
uncertain failure; Settings keeps explicit retry and restart controls after a
remount. A real ephemeral-port companion test verifies graceful return, SQLite
reopen and retained project/installation identity. The initial unpaired state,
multiple desktop instances, native exit/rendering and replacement of the
refreshed app remain unqualified. This source change is newer than candidate
`2026-10-30-c425821`; that retained candidate does not contain the new action.

Validation for the native preparation source lot: Go `ba4c512e` passes
`go test ./...`; the real companion and HTTP update tests also pass with
`-race`. Tauri passes 140 UI tests, 41 Rust tests, TypeScript checking and the
production frontend build. The source-checkout Node suite passes 25 tests.
The regular Local companion remained running throughout; no user installation
was selected for shutdown or replacement. These are source/runtime proofs,
not evidence of native button execution or an updated retained DMG.
