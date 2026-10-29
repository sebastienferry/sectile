# ADR 0062: Resume local provider conversations after agent restart

## Status

Accepted.

## Context

The run store retained a transcript but discarded Claude session IDs and Codex
thread IDs. A restarted agent could only present read-only histories, even when
the provider still held the conversation context on the same workstation.

## Decision

Extend the existing private, atomic run records with optional conversation
metadata: provider session identity, permission mode, reasoning effort and extra
working directories. The model remains in desktop run metadata. Store no CLI
credentials, environment maps, pending messages, approvals or partial answers.

Only free Claude Code and Codex conversations with a recorded provider identity
are eligible. Ticket discussions with injected environments retain the historical
read-only behavior. Existing records without the new metadata stay readable and
read-only; inferring a provider session from transcript text is unsafe.

Restore an idle conversation with an open trace and no provider process. Reading
its Codex snapshot does not start the app-server. A new explicit message invokes
Claude with `--resume` or Codex with `thread/resume`. Resume failures are surfaced;
no fresh conversation is silently substituted. Interrupted work and pending
messages are never replayed by Sectile.

Daemon cleanup records whether the conversation was still open before it canceled
its processes. Explicitly stopped conversations remain closed. Save completed
turns promptly and retain the existing periodic crash checkpoint and final flush.
Model/mode changes participate in dirty checking, even without new trace lines.

## Consequences

Provider context is owned by the provider's local session store. Deleting that
store or moving the application to another workstation cannot be repaired from
the displayed transcript. A crash may lose up to the existing five-second
checkpoint window for a turn in progress. A new message is required after restart.

## Validation

Regression tests cover graceful shutdown, crash checkpoints, repeated restarts,
explicit stops, unchanged-transcript settings updates, and actual provider resume
requests through fake CLI subprocesses. Targeted tests also pass with Go's race
detector. The complete agent and process-management test packages pass.

A real native desktop test restarted its isolated Go companion after idle Claude
`haiku`/Plan and Codex `gpt-6-luna`/execution turns in a temporary folder. Both
retained their run IDs and transcript prefixes, remained writable, and recalled
synthetic nonces and filenames without those values being repeated. No tools were
used to recover this context.

The same conversations also passed a second agent restart: both retained their
entire previous transcript and recalled the initial values and first restart
verification with `SECOND_RESTART_OK`. Model and mode remained unchanged.

## Abrupt agent crash during active tools (2026-10-28)

A real test launched Claude `haiku` in `acceptEdits` and Codex `gpt-6-luna` in
`workspace-write` in a disposable local folder. Each conversation first memorized
a synthetic nonce and filename. Its next turn invoked one shell tool that appended
`START` to a provider-specific counter, slept 90 seconds and attempted a delayed
`DONE` write. The owned Go listener received SIGKILL while both counters existed,
both conversations were busy and neither delayed write had completed.

After native Settings started a new agent, the same conversation IDs were idle
and writable. Both saved transcript prefixes exactly matched the complete events
captured immediately before the crash, and both model/mode selections matched.
An explicit new message forbidding tool use elicited `CRASH_RESUME_OK`, the original
nonce and filename, and the interrupted tool's intent from both providers. The
native UI displayed the unmatched tool as having no received result.

At 108 seconds after SIGKILL, each counter still had exactly one `START` line.
Neither resumed conversation emitted another tool call: no replay occurred.
However, the original Claude tool survived the agent and performed its delayed
write. The Codex delayed write did not appear (including the alternate filename
in the actual emitted command). Process inspection also observed the surviving
Claude test shell after the crash.

This validates recovery and absence of replay for this test, not child-process
termination on a daemon crash. An agent killed with SIGKILL cannot run its cleanup.
A provider-lifetime supervisor or equivalent ownership mechanism would be needed
to guarantee termination of already running tools. Do not present an idle restored
conversation as proof that its old subprocesses have stopped. The existing
five-second checkpoint window and intentionally unsaved partial streaming output
also prevent a universal zero-loss guarantee for arbitrary crash timing; the
complete event prefixes happened to be fully checkpointed in this observed test.

The surviving-tool limitation in the initial crash test above is addressed by
[ADR 0063](0063-provider-tools-follow-the-agent-lifetime.md). A guarded real test
SIGKILLed Go during both providers' 90-second tools; neither delayed file existed
at 125 seconds, no tool was replayed and both conversations resumed with their
checkpointed history and turn settings intact.
