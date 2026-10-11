# Workstation selection and run recovery

## Why
Web users cannot choose among eligible connected workstations at launch or easily identify where a run executes. A queued, pending or running execution can keep a task busy after disconnection. Recovery must clear genuine stale records without falsely claiming that an unreachable local process stopped.

## What Changes
- Apply a shared workstation picker to all Web task skill launches, card shortcuts, activity retries and compatible task batches. Select the sole eligible workstation automatically; require a choice among several; explain and block agent launches when none qualify.
- Display readable workstation attribution on task cards, task details and activity summaries, including historical runs and reconnects.
- Expose explicit cancellation for queued, pending and running executions. Use confirmed process stopping, proven-orphan recovery or an explicitly confirmed force close; retain disconnected runs whose termination is unknown.
- Preserve owner/admin permissions, active-run uniqueness, completion validation and workflow handback.

## Scope
Accepted baseline: docs/clarifications/816.md, Round 2, confirmed by the owner on 2026-10-11. The ticket's conceptual targetDeviceId is implemented with the existing deviceId field. No unconfirmed execution expires merely because a connection timeout elapsed.

Out of scope: macro launches, Desktop launch UX changes, remembered workstation preferences, automatic rerouting or rescheduling, new execution engines, and timeout-based expiry of unconfirmed processes. Server-only skills retain their existing launch behavior and need no workstation.

## Impact
Go handler/distribution and activity persistence contracts, React launch/cancellation flows, workstation labels from existing paired-device metadata, localized Web strings, server and browser tests, and implementation-time English changelog entries. No production code is changed at the specification stage. No product questions remain open.
