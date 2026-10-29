# ADR 0015: Explicit Local and Fleet selection

Date: 2026-10-29
Status: accepted

## Context

The Tauri desktop previously read its managed Local companion discovery file
when available, then silently fell back to `~/.taskflow/agent-connection.json`.
That file is shared with the Electron/Go agent setup. The fallback made the
active backend implicit and allowed histories or actions to cross runtime
boundaries without a user-selected connection.

The Go agent exposes a workstation-local authenticated gateway for its selected
Fleet backend. This ADR uses that existing contract and does not change Electron
or Go pairing behavior.

## Decision

- Persist a native `runtime-mode.json` in Tauri's private application data.
- Local mode uses only the managed companion's discovery file. An existing
  managed installation without a mode record is treated as Local because its
  installation manifest is unambiguous.
- Fleet mode is entered only through an explicit native onboarding action.
  Selecting **Associer l’agent Fleet existant** reads the shared legacy
  connection file only in that command, validates a loopback agent, verifies
  `/desktop/status` reports an authenticated connection, and copies the
  credential to a private Tauri-owned Fleet file.
- Fleet mode can also pair this workstation directly through the backend's
  one-time `/api/v1/agent/pair` code. Native code stores the returned API key
  in the Go agent's private settings file, starts the bundled lightweight Go
  agent, and keeps its loopback desktop credential in a separate private
  runtime file and the discovery file. No durable API key reaches the renderer,
  process arguments or child-process environment.
- Ordinary requests use only the selected mode's private connection file. An
  absent or failed connection is an error; it never triggers a fallback.
- Native profile, personal tracker credential, OAuth account and project
  bootstrap screens use the selected Fleet backend after pairing. OAuth app
  secrets and shared tracker administration remain server-admin operations.
- Once a runtime mode is selected, pairing or attaching another runtime is
  rejected. Local/Fleet data is never transferred implicitly; live switching
  requires a separately designed migration flow.

## Consequences

This closes silent Electron/Tauri connection reuse and supports both first-time
Fleet pairing and attachment of an already paired workstation. Native Fleet
setup uses the selected backend without exposing its API credential to the
renderer. Existing modes cannot be replaced by pairing or attachment; runtime
switching and data transfer remain unsupported until an explicit migration is
designed. Local and Fleet connections are not merged, and the Go/Electron
behavior remains unchanged. Conversation identity adoption remains a separate
qualification gate under the [runtime plan](../local-fleet-runtime-plan.md).

## Security

The renderer cannot provide arbitrary file paths, URLs or commands. Native code
uses the one known shared connection path only on explicit user action, checks
that the agent is loopback-bound and authenticated, then writes the connection
under Tauri's private application data directory. The shared file is not
modified.
