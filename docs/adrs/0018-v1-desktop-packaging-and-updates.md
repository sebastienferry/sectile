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
replacing the application with the downloaded version, and reopening it. Do
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
