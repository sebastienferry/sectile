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
- Fleet mode is entered only after the user selects **Associer l’agent Fleet
  existant**. The desktop reads the shared legacy connection file only in that
  command, validates a loopback agent, verifies `/desktop/status` reports an
  authenticated connection, and copies the credential to a private Tauri-owned
  Fleet file.
- Ordinary requests use only the selected mode's private connection file. An
  absent or failed connection is an error; it never triggers a fallback.
- The native Local profile, tracker, OAuth and project bootstrap screens remain
  disabled in Fleet mode because those operations are owned by the selected
  Fleet backend and are not implemented in this desktop setup flow yet.

## Consequences

This closes silent Electron/Tauri connection reuse and provides a migration
path for a workstation that already has a connected Fleet agent. It does not
yet implement first-time Fleet pairing from Tauri, Fleet backend URL setup,
active-work-safe mode switching, or a full Fleet setup UI. Those remain stage 4
work under the [runtime plan](../local-fleet-runtime-plan.md). Local and Fleet
connections are not merged, and the Go/Electron behavior remains unchanged.

## Security

The renderer cannot provide arbitrary file paths, URLs or commands. Native code
uses the one known shared connection path only on explicit user action, checks
that the agent is loopback-bound and authenticated, then writes the connection
under Tauri's private application data directory. The shared file is not
modified.
