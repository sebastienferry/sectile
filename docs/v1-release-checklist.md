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

The v0.7.6/current Go binary harness was refreshed on 2026-10-29 against
clean Go `554374b2`. Both agent/server directions passed the 27-operation
agent protocol, 21 MCP tools through direct and STDIO clients, run lifecycle,
task-spec worktree creation and fake GitHub issue/stage writes using a personal
token. The isolated test left the Go checkout clean. Live provider OAuth and
tracker writes remain separate release gates.

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

Electron source regression refresh (2026-10-29): Go branch `fe3a98c8`
remained clean while the Electron source passed its 315 unit tests, 137 UI
tests and production Vite build. The UI tests cover board, console, workflow,
settings and MCP fixtures. This is a source regression check, not a live
Electron session paired with the current Go binary or a remote tracker write.
