# ADR 0003: Keep Go for the standalone desktop backend

- Date: 2026-10-28
- Status: Accepted; initial runtime and onboarding implemented

## Context

The desktop must eventually bootstrap a profile, tracker connections and projects
without a separately operated Sectile website. It must support project-scoped
prompts, tracker-backed work, orchestration and progress monitoring. The user
prefers maintaining Go to a Rust backend rewrite.

The current agent executes work but delegates identity, encrypted tracker
credentials, synchronization, project configuration and workflow state to the Go
server. Replacing only the agent would not remove that dependency.

## Decision

Keep both Go services. Tauri provides the desktop UI and a narrow native boundary;
the Go server owns local configuration and tracker integration, and the Go agent
owns execution. Build native onboarding screens against explicit native commands
that call the existing Go APIs. Do not embed the website as the setup experience.

Target flow:

```text
Tauri native screens → native command boundary → local Go server
                                            → local Go agent → provider runtimes
local Go server → remote trackers (authoritative ticket state)
```

Rust is responsible for native integration and eventual service lifecycle, not a
second implementation of Sectile's business rules. MCP remains an agent-facing
tool interface; it is not a desktop application control protocol.

## Isolation contract

- The server listens on loopback. Its database, encryption key and environment
  file live in the companion's private application data directory.
- The agent uses a separate settings directory and discovery file. The desktop
  does not take over the existing installation's global provider MCP entries.
- Clear inherited deployment credentials and database settings when launching
  managed services. Never inherit a production database URL accidentally.
- Credentials stay behind native commands. Tracker secrets are encrypted by the
  existing Go store and never persisted in web storage.
- Native setup uses personal tracker credentials and joins registered trackers;
  it does not create an independent local ticket board.
- Startup must detect failures and version mismatches. Shutdown must have an
  explicit policy for active runs; navigating away from a run must never stop a run.

## Implementation status

The single-view UI, themes, shared monitor and existing-agent execution controls
are implemented. Native Settings now activate an independent installation, create
or rename its profile, store personal tracker access, register trackers, create
projects and map local repositories. OAuth now uses the browser handoff in ADR 0004; advanced configuration
editing remains outside this first setup flow.

A compatible Sectile build must include `SECTILE_SERVER_HOST`,
`SECTILE_SERVER_DATA_DIR`, `SECTILE_DISABLE_MCP_REFRESH`, and the protocol 1
`/api/local-runtime` discovery endpoint. The endpoint accepts an installation
nonce held by native code, refuses browser origins and is not a user credential.
The desktop refuses to attach to a different listener occupying its saved port.

Activation persists a private installation manifest, then starts the loopback
server. Local sign-in yields a short-lived web session used to pair a workstation;
only its device credential is persisted. Profile and tracker API calls use that
credential. The renderer receives setup metadata, never the service credentials.
Startup and pairing are serialized with an OS file lock as well as the in-process
mutex to avoid duplicate servers or pairing races across windows.

The app starts missing services at launch, and reconnects to running ones.
Services intentionally survive closing the desktop. They are not installed as OS
login services, automatically upgraded, or continuously restarted by a watchdog.
This preserves ongoing runs. Settings offers an explicit startup retry, and logs
stay in the private runtime directory. Background commands hide Windows consoles.

Build service resources with `npm run prepare:services` from a compatible Go
checkout. They remain untracked external artifacts. The integration test uses
an isolated temporary database, a mock tracker HTTP service, a temporary Git
repository and disposable identity; it covers onboarding and server restart
without using real provider credentials.

Remaining work includes free-form workspace conversations, self-hosted forge OAuth,
advanced project/profile configuration and production service upgrade handling.

## Consequences

This preserves existing orchestration, tracker behavior and Go expertise. It
adds two local processes and requires explicit lifecycle handling. A Rust rewrite
can be considered later for a measured requirement, with contract tests and an
incremental migration; it is not a prerequisite for a standalone desktop.

The new repository is MIT. The Go checkout currently has no license file, and
the desktop build bundles binaries built from that source. The Tauri MIT license
does not grant rights to the Go services. Public binary distribution therefore
requires an explicit license for the Go source and any corresponding third-party
notices. Do not infer relicensing or redistribution rights from this decision.
