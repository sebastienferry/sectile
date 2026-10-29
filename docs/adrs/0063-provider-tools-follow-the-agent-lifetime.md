# ADR 0063: Provider tools follow the agent lifetime

## Status

Accepted.

## Context

A real SIGKILL test showed that a Claude tool could outlive the Go agent and
perform its delayed write after conversation recovery. An explicit interruption
can terminate a provider tree, but a killed daemon cannot execute its cleanup.
Stopping the provider's process group alone also misses tools in other sessions.

## Decision

Use `agentexec.StartOwned` for Claude Code conversation turns and Codex
conversation app-servers. Keep terminal and ordinary detached process behavior
unchanged: closing the desktop still leaves its Go services running.

On Unix, the same agent executable has a private `provider-guard` role. The guard
starts the provider, forwards its stdio, and watches a pipe whose write end is
held only by the owning Go daemon. Provider descendants do not inherit the
lifetime descriptor. A daemon crash, including SIGKILL, closes that pipe without
requiring a defer, a heartbeat or a later restart. The independent guard freezes
and terminates the provider tree through `StopTree`, including descendants in
separate sessions. The guard also handles ordinary termination signals.

An explicit input-close notification travels on the lifetime pipe. This separates
normal provider stdin EOF from daemon death: the guard holds the provider's stdin
open on owner death until it has inspected the tree, avoiding early provider exit
and orphaning. `OwnedStdinPipe` preserves `exec.Cmd`'s original file ownership;
wrapping its read-side file would let `exec.Start` close it before forwarding.

On Windows, a conversation provider starts suspended and hidden. Assign it to a
Job Object with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` before resuming its primary
thread. Fail startup if job creation, assignment or resumption fails. The job
handle is private to Go, so Windows terminates its descendants when Go dies.
Suspended startup closes the child-spawn race in the older detached-job helper.

## Consequences

Each active Unix provider has one extra small supervisor process and an ownership
pipe. No credentials, prompts or provider protocol frames are interpreted or
persisted by the guard. Stdio, exit status, CLI session identity and subsequent
conversation resumption retain their existing contracts.

This controls the subprocess tree owned by the local provider. It does not undo
writes completed before the crash, cancel remote work already submitted to an
external service, or provide durability for uncheckpointed streaming text.

## Validation

Automated subprocess tests kill a simulated agent without running its defers,
while a provider tool in a separate session waits to write a delayed marker. The
marker must stay absent after its deadline. Additional tests cover normal stdin
EOF, literal protocol text, stderr, exit status and explicit lifetime release.
Windows tests assert suspended, detached and no-console-window creation flags;
the Windows process-management package is cross-compiled on macOS.

## Real-provider crash verification (2026-10-28)

The native macOS desktop used newly built Go companions. Claude `haiku` in
`acceptEdits` and Codex `gpt-6-luna` in `workspace-write` each memorized a synthetic
nonce and fictional filename, then started one shell tool in a disposable local
folder. The tool appended one `START` line, slept 90 seconds and would append a
`DONE` marker. SIGKILL targeted only the owned Go listener while both tools were
active and their start markers existed.

A pre-crash process-tree snapshot recorded 21 descendants. Process inspection
30 seconds after the crash found none of those descendants still present. After
native Settings restarted Go, both original conversation IDs were writable and
retained their complete checkpointed transcript prefixes, chosen model and mode.
An explicit new message forbidding tools produced `CRASH_RESUME_OK` with the
remembered nonce, filename and interrupted tool intent from each provider.

At 125 seconds after SIGKILL, neither delayed marker existed, including alternate
filenames matching the actual command prefix. Each start counter remained exactly
one line and neither resumed transcript contained a new tool call. This fixes the
previous observed Claude orphan-tool write while preserving session recovery.

Go's complete agent and process-management packages passed. Targeted provider
regressions and guardian tests passed with the race detector, including a transient
process-inspection failure followed by retry. The desktop passed TypeScript,
45 UI tests, four domain tests and 15 Rust tests, and the debug app bundle built.
Windows process-management code and tests were cross-compiled; the real provider
SIGKILL scenario was executed on macOS, not on Windows.
