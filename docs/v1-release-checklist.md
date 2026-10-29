# Sectile Tauri V1 release checklist

This checklist qualifies a future V1 release. It does not authorize publishing
or create a Git tag.

## Confirmed release scope

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
  conversation history and managed-runtime recovery remain unverified; see the
  migration plan.
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
