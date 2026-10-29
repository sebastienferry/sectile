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

- [ ] Build the arm64 app and DMG from the selected release commits. Add an
  x86_64 artifact only after native/package qualification for that architecture.
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

The migration plan records a valid unsigned arm64 DMG, current-source Local
onboarding and provider-supervision tests, Go/Electron regression suites, and
current/v0.7.6 agent/server protocol tests. The latest app bundle reports
`releaseEligible: false`. Signing/notarization, clean-user installation and
upgrade retention, default-port cold start with the current binary, and the
remaining live Fleet/Electron checks are still open. No V1 release is ready to
publish.
