# ADR 0018: Use manual desktop updates for V1

Status: accepted

Date: 2026-10-29

## Context

The desktop bundles the Go companion and its services, so updating the app also
updates the local execution/runtime contract. The current Tauri app has no
updater plugin or update service. A production macOS release must also be
signed and notarized, but the qualification host has no valid signing identity.
An automatic updater would add release metadata, signing-key custody, staged
rollout and rollback behavior before the first supported release is qualified.

## Decision

For V1, distribute the macOS app as a signed and notarized DMG through the
project's release page. Users install an update manually by quitting Sectile,
gracefully stopping the idle managed Local companion, replacing the application
with the downloaded version, and reopening it. Quitting the desktop alone
leaves the companion running and is insufficient for a service update. Do
not add an in-app update checker or automatic updater for V1.

Keep the Tauri bundle identifier and user-data layout stable across V1 updates.
Application replacement must preserve the user's Local data; schema and
managed-runtime migrations remain governed by the existing journaled upgrade
contract. The DMG must be verified and its checksum published with each release.

Linux remains experimental and Windows is deferred. Neither platform has a V1
update guarantee. Reconsider an updater after signed macOS releases establish a
stable artifact and rollback policy.

## Consequences

- V1 does not depend on an update server or updater signing key.
- Users initiate installation, and release notes explain the changes.
- A release process must sign and notarize the app, verify the DMG, publish a
  checksum, and test that replacing the bundle retains existing Local data.
- This decision does not authorize a release or close the signing, notarization,
  upgrade-installation, or release-automation qualification gates.


## Implementation qualification (2026-10-30)

The launcher currently reuses an authenticated protocol-compatible companion
without comparing the running binary fingerprint with the packaged service.
The source-checkout command `npm run stop:local -- --data-dir <absolute-root>`
provides an explicit graceful stop before manual replacement. It follows only
the known selected generation, rejects Fleet and remote discovery, authenticates
the Local server and agent relationship, honors the agent's busy refusal, and
waits for both loopback connections to refuse connections. Mock HTTP regression
tests cover the success and refusal paths. It does not send signals or change
the regular installation during tests.

This command is not a native update flow or proof of process/database exit.
Before the V1 update gate can close, the desktop must offer an equivalent native
preparation action, prevent startup races while preparing, verify full service
exit and qualify replacement on a disposable installation. Do not report a new
UI attached to an older running companion as a completed backend update.


## Native preparation implementation (2026-10-30)

Configured Local Services now exposes an explicit stop-and-quit action. Tauri
holds the setup/startup locks and installation lock, suspends automatic startup,
and invokes the bundled Go `--prepare-update` role. The helper authenticates the
companion/agent relationship, honors the existing busy refusal, and requires
both transport closure and store lifetime ownership release. This strengthens
the earlier Node maintenance command, which checks transports only. On helper
failure, Tauri retains the startup fence; Settings remounts still expose retry
and the existing explicit service-start action. Successful preparation exits
Tauri before releasing its installation/startup locks.

The action does not download an update, stop Fleet, modify shared Electron
configuration or interrupt active tasks. Other desktop instances must be closed
first. The initial unpaired/bootstrap state remains unsupported by the native
preparation action. Go integration tests use a real companion on an ephemeral
port and verify retained data after SQL reopen; UI tests cover acknowledgement,
busy failure, remount recovery and Fleet exclusion. Native button/exit and DMG
replacement qualification remain open while the owner defers visual testing.
