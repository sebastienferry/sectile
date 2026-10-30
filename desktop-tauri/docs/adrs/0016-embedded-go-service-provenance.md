# ADR 0016: Embedded Go service provenance

Status: accepted

Date: 2026-10-29

## Context

The Tauri application packages Go services from a separate Sectile checkout.
Without an explicit build identity, the bundled binaries can report `dev` with
no commit, and a packaged application cannot establish which Go source produced
its server, agent and companion. A partial build must also not leave a mixed
set of service revisions in the next application bundle.

## Decision

`scripts/build-services.mjs` reads the selected Go checkout's commit, commit
date, exact tag and dirty state. It embeds the same commit and date in every
service. It builds into a staging directory, checks each binary's version
output, computes SHA-256 hashes and sizes, writes a build manifest, then
replaces the service set with rollback for failed file operations.

The manifest records the source identity and each service's hash and version.
It is generated beside the ignored service binaries and packaged as an app
resource. `SECTILE_SOURCE_COMMIT` pins a build to an expected revision, while
`SECTILE_REQUIRE_CLEAN_SOURCE=1` rejects modifications to the Go checkout. A
package is release-eligible only when the source is clean and HEAD has an exact
Git tag. The script does not grant or change the Go repository's license.

## Consequences

- A desktop package can be traced to one precise Go revision and checked for
  service-file integrity.
- Developer builds remain possible from untagged commits, but identify as
  development builds and are marked ineligible for release.
- Release qualification still requires platform-native packaging, signing,
  compatibility tests and a license that permits distribution of the Go code.
- The Go repository remains independently versioned and maintained.
