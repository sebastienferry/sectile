# V1 desktop support matrix (candidate)

This matrix records what the migration branch has demonstrated as of
2026-10-29. It is not a release announcement. A V1 release must name its exact
Tauri and Go commits, publish signed macOS artifacts, and repeat the release
binary checks in the [release checklist](v1-release-checklist.md).

## Platform and delivery

| Target | V1 position | Current evidence and limit |
| --- | --- | --- |
| macOS arm64 | First release target | A 102.28 MiB app and 51.70 MiB DMG bundle clean Go commit `25a8e77d` with the fixed companion port 52963. The resource verifier and DMG checksum pass. The artifact is unsigned and unnotarized, so installation, Gatekeeper and clean-user launch are not qualified. |
| macOS x86_64 | Not promised for the first release | No native package or runtime qualification is recorded. |
| Linux | Experimental | A Debian ARM64 package opened a visible window under Xvfb/DBus. Real user desktop, provider and migration workflows across distributions remain unqualified. |
| Windows | Deferred | Go services cross-build, but native desktop startup, migration, process execution and installer behavior remain unqualified. |

## Runtime combinations

| Client and backend | Demonstrated behavior | Boundary for V1 |
| --- | --- | --- |
| Tauri Local with the embedded Go companion | Native Local project/profile screens, folder mapping and the Local board were exercised in isolated macOS bundles. A disposable Local service harness with real Claude Code and Codex confirmed responses, tool activity, interruption, resume and persistence. | The latest `25a8e77d` package has not been launched on the fixed port or installed for a clean user. External OAuth and tracker operations are not part of Local qualification. |
| Tauri Fleet with current Go server and agent | Native pairing, profile update, fake GitLab credential/tracker/project setup, reconnect and persistence passed against a disposable current-Go backend. | A release may claim this pairing only after its exact Tauri/server/agent revisions and real tracker/OAuth workflows are qualified. The isolated fixture does not establish general Fleet support. |
| Tauri Fleet with the existing `660-main` backend | Pairing and authenticated project reads passed. | OAuth setup is unsupported by that backend version. Do not list it as a fully supported V1 Fleet backend. |
| Electron 0.7.5 with current Go server and agent | An isolated window loaded projects and a task board. A tracked task against Go `25a8e77d` displayed real Codex activity; Electron Stop canceled its delegated shell tool and the 90-second marker remained absent after 170 seconds. | Remote tracker mutations and the Electron window against a signed release package remain unqualified. Electron remains a compatibility client; no retirement date is set. |
| Existing MCP clients with the tested current/v0.7.6 server-agent pairings | The binary harness passed its 21-tool contract and read-only STDIO bridge call in both directions. | This does not qualify every MCP client registration, provider-specific consent or remote mutation. |

## Product ownership and updates

Tauri is the intended V1 desktop for operational boards, prompts and agentic
execution. The current web app remains the V1 owner of product planning and is
unchanged. Electron stays available as a compatibility client while its
support window is decided separately. Desktop updates use a user-initiated
replacement of a signed and notarized DMG; no in-app updater is planned.

The [migration evidence](sectile-v1-migration-plan.md#stage-5-compatibility-evidence)
contains the source revisions and exact checks behind each row. The release
checklist remains open until the release binaries and external integrations are
qualified.
