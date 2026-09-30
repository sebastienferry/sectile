# ADR 0042: Experimental Claude conversations use process pipes

Status: Proposed (test branch only)

## Context

Desktop currently presents provider CLIs through PTY consoles. Autonomous Claude
runs already parse JSON events, but render them into terminal lines. A native
conversation needs structured messages and a way to send follow-up prompts.

## Decision

Trial an opt-in Claude conversation alongside existing consoles. Admit it as a
local free console in an existing execution's directory. Launch one hidden,
detached Claude process per message, read JSON from stdout, keep stderr separate,
and resume the session ID for later messages. Send prompts on stdin rather than
through shell command interpolation.

The agent owns process cancellation and message admission. Keep rendered event
objects as JSON in the existing bounded trace, with the existing local run store
providing read-only recovery. Serve snapshots through the authenticated Desktop
HTTP API and route them through Electron IPC. The renderer uses DOM text nodes
for all engine output.

## Consequences

The prototype needs no SDK runtime or server schema changes and leaves existing
workflow launches unchanged. It proves the local conversation UI and session
continuation, but does not provide interactive approvals, token deltas, workflow
integration or live restoration. `acceptEdits` permits file edits; unresolved
permission prompts are denied. Those limits are visible in the composer.

A persistent streaming-input process or an Agent SDK adapter may replace the
per-turn process before production adoption. That decision must include the
approval and question protocol, cancellation semantics and crash recovery.
