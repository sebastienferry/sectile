# ADR 0014: Migrate managed Local installations to the Go companion

Date: 2026-10-29
Status: accepted

## Context

The Tauri Local prototype used a Go server and a separate Go agent. The target
Local topology is one Go companion process containing both components. Existing
users already have SQLite state, encrypted tracker credentials, OAuth grants,
agent settings and resumable conversation journals. Starting the new process
against those files before validating their ownership and compatibility could
corrupt data or allow two writers.

## Decision

Tauri invokes a Go-owned upgrade command while holding its startup lock. The
upgrade authenticates the legacy loopback server, verifies managed listener
executables, refuses active backend or provider work, stops the source services,
and prepares a private hashed backup. It validates a separate candidate
generation, including its existing encryption key and SQLite integrity, before
atomically writing `runtime-selection.json` as the commit point.

The selected generation is the sole Local data/configuration root. The native
launcher fails closed on an unknown selection and never falls back to source
paths. Before commit, an incomplete candidate is recreated from the verified
backup. After commit, recovery proceeds forward; snapshots are never restored
automatically after work can resume. The prior source files and compatible
service binaries remain private for explicit recovery.

Local starts one companion. Fleet remains a separately configured topology and
continues to use the standalone agent/backend contracts. This decision does not
remove Electron support or alter shared provider MCP registrations.

## Consequences

- Existing user, project, tracker, conversation and OAuth encryption identities
  remain intact during the managed upgrade.
- The first upgrade requires old tasks and provider turns to be idle.
- macOS verifies and stops the managed legacy process trees. On platforms without
  a qualified process ownership adapter, users must stop the old services before
  retrying migration.
- The migration is recoverable across each durable journal boundary, but it is
  not a general backup/restore or Local-to-Fleet transfer facility.
- Fleet/Electron compatibility and platform packaging remain separate release
  qualification gates.

## Verification

Automated recovery tests fail and retry each migration checkpoint, compare
source/backup contents and verify the selection and legacy fence. The existing
macOS installation migrated with its projects, trackers, conversation history,
Jira credentials and OAuth encryption key. Native Claude Code and Codex prompts,
tool tracking and real companion MCP reads succeeded after migration.
