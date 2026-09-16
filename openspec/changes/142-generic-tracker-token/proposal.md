# The server tracker credential gets a tracker-agnostic name

## Why
Sectile drives GitHub and Linear projects, yet the credential an operator is told to
export is `SECTILE_GITHUB_TOKEN`, and every credential-missing error names it —
including on a Linear project, where setting that variable changes nothing. The
name and the diagnostics both claim the product is GitHub-only.

## What Changes
Introduce `SECTILE_TRACKER_TOKEN` as the canonical, tracker-agnostic server
credential. Each provider resolves its token in order: its provider-specific
variable, then `SECTILE_TRACKER_TOKEN`, then its environment-only convention
fallback. Provider-specific variables keep priority so a deployment serving two
trackers cannot leak a GitHub credential to Linear. Credential-missing errors stop
naming GitHub. Documentation presents the generic name and records the
provider-specific ones as overrides.

## Capabilities
### Added Capabilities
- `tracker-credentials`: server-side tracker credential resolution and diagnostics.

## Impact
`internal/trackerapi/client.go`, README credential documentation, tracker
regression tests. No schema change, no migration: credentials are read from the
server process environment at startup. Existing deployments keep working
unchanged.
