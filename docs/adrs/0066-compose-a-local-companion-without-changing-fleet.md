# ADR 0066: Compose a Local companion without changing Fleet boundaries

- Date: 2026-10-29
- Status: Accepted and implemented; platform and release qualification in progress

## Context

The standalone desktop currently runs a backend and execution agent as separate
Go processes. Their REST, MCP and WebSocket contracts already carry identity,
workflow state, progress, cancellation and postbacks. Fleet also depends on these
contracts and deliberately keeps SQLite/backend handlers outside its agent binary.

## Decision

Extract backend assembly into a reusable internal package, preserving the
standalone server as its first consumer. Make agent and backend lifecycles
explicit. A separate Local companion command will compose both in one process.
Keep internal HTTP/WebSocket contracts for the first delivery; direct dispatch
and listener unification are deferred until behavior and measurements justify them.

The Fleet agent remains lightweight and the Fleet server remains independent of
workstation execution packages. The server alone includes website assets.
The composition root owns process signals, exclusive data ownership and restart.

Runtime topology is separate from tracker authentication. Preserve the current
working OAuth installation throughout the migration, including token refresh and
reconnect; API tokens are the initial fresh Local onboarding path. No hosted
OAuth relay or shared vendor secret distribution is part of this extraction.

The [phase 0 contracts](../desktop-runtime-contracts.md) define identity binding,
immutable configuration, lifetime locks, shutdown and journaled migration.
Rollback is automatic only before migration commit; later failures require safe
forward recovery so external effects or refresh rotation cannot be replayed.

## Consequences

One long-lived Local Go companion replaces the former server and agent pair.
Provider CLI, guard and MCP bridge children remain normal execution processes.
The companion binary includes backend dependencies; Fleet's agent does not.
Tauri owns the companion lifecycle and performs a journaled upgrade of the
prototype installation. A separate artifact adds packaging/version
compatibility obligations.

The shared runtime extraction changes no tracker source of truth and does not
merge Local and Fleet data. Phase 1 added `internal/backend`; phase 2 added an
OS-owned Local store lock, context-owned shutdown paths and an authenticated-
agent startup gate. The Go repository is MIT licensed. macOS migration and
Local/Fleet runtime paths have isolated qualification evidence in the Tauri
repository's V1 migration plan. Signing, notarization, clean-user installation
and the remaining compatibility matrix are release gates; this ADR does not
claim that the desktop is ready for V1 release.
