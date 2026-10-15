## Why

Sectile Desktop displays installed versions and bundled release notes but cannot discover a newer published Desktop release. Owners must currently find releases themselves. The accepted clarification in `docs/clarifications/839.md` (Round 2) authorizes detection and a release/download link.

## What Changes

- Detect newer stable tagged GitHub releases at launch, every 24 hours while open, and on demand in Changelog settings.
- Show persistent update availability and notify at most once per newly detected version.
- Offer the matching Desktop archive through the user's browser, without downloading or installing it automatically.
- Distinguish unsuccessful checks from successful checks with no update; retain installed versions and embedded notes offline.

## Capabilities

### New Capabilities
- `desktop-updates`: stable Desktop release discovery, check status, availability, and release navigation.

### Modified Capabilities
None.

## Impact

Electron main/preload boundary, Desktop settings and availability presentation, local settings persistence, and focused Desktop tests. Implementation adds an English Unreleased changelog entry. No server/agent update detection, automatic installation, rolling GitLab builds, release pipeline changes, credentials, or data migration. Existing bundled-agent mismatch/restart behavior remains separate. No unresolved product requirements.
