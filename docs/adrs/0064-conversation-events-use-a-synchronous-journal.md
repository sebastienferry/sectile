# ADR 0064: Conversation events use a synchronous journal

## Status

Accepted.

## Context

The run store checkpointed active runs every five seconds and discarded streamed
assistant drafts. Crash-safe provider ownership stopped orphan tools, but SIGKILL
could still remove the most recent visible conversation output. Rewriting the
entire transcript for each streamed token would increase disk work with history.

## Decision

Free Claude Code and Codex conversations have a private append-only JSON-lines
journal beside their existing run snapshot. Each normalized message, tool call,
tool result and streamed text update is written and synced before publication to
the desktop. Session identity and model/mode metadata are committed before the
corresponding next turn is dispatched. No new storage dependency is introduced.

Stream updates append deltas when possible; a new draft replaces its predecessor.
Complete assistant events clear their saved draft in the same journal record.
Claude block-stop frames do not discard text while the complete assistant message
is still pending. Raw provider frames, environments, credentials and pending
approval answers are not journaled. Existing answer redaction remains in place.

On restart, replay only the complete ordered journal prefix. Repair an incomplete
or invalid trailing record before appending again. Promote an unfinished draft to
one assistant history event marked `interrupted`, then clear its saved draft in
the same committed operation. The desktop labels it as an interrupted reply and
keeps its Markdown. Provider work is not replayed; resumption requires a new user
message. A missing provider identity still permits read-only draft recovery.

The existing periodic loop now compacts conversations rather than defining their
recovery window. Save the snapshot with its journal sequence, then truncate the
journal while holding its writer lock. Recovery skips records already represented
in the snapshot, including a crash between replacement and truncation. A four-MiB
journal also triggers compaction. Trace and streaming size bounds remain in force.
Clear and retention remove both files and prevent later callbacks recreating them.

A write failure is surfaced in the desktop. Do not dispatch a new prompt whose
local journal write failed. Keep subsequent output in memory with an explicit
persistence warning, then repair through a full checkpoint when storage recovers.
The journal file is mode 0600 in the existing mode-0700 run directory.

## Consequences

A process crash no longer loses already committed, displayed conversation events
because of the five-second timer. Output not yet received/processed by the agent,
provider-side persistence and storage-device failures remain distinct concerns.
The journal does not turn drafts into prompts, restore pending approvals or undo
completed tool side effects. Ordinary terminal run persistence is unchanged.

Synchronous commits add disk latency. A local 100-update sample on macOS arm64
measured about 3.1 ms per synced draft update; this is a small environment-specific
sample, not a cross-platform latency guarantee. Incremental records avoid copying
and rewriting the complete transcript for every delta.

## Validation

Regression tests recover messages, calls, results and a partial reply without
running any periodic flush; restore twice without duplicate fragments; replace a
draft atomically with its complete answer; repair a torn tail; survive the
snapshot-replacement/truncation crash window; preserve read-only drafts without a
provider identity; and prevent cleared histories from reappearing. Write-failure
tests assert no prompt dispatch and a visible persistence warning. Targeted tests
pass with Go's race detector. A frontend test covers the interrupted reply label.

A real native desktop test ran Claude `haiku`/`acceptEdits` and Codex
`gpt-6-luna`/`workspace-write` in a temporary folder. Each completed one shell tool
and began streaming a long synthetic reply. SIGKILL immediately followed the live
snapshot checks, without waiting for a periodic checkpoint. The base snapshots
still contained zero draft characters. The desktop had observed 784 Claude draft
characters and 153 Codex characters; journal recovery retained 811 and 221,
respectively, including additional chunks received before SIGKILL landed.

All complete event prefixes were retained, and both restored drafts were labeled
interrupted. Both conversations resumed with `JOURNAL_RESUME_OK`, recalled the
original nonce and fictional filename without those values being repeated, and
kept their model and mode. Neither provider repeated a tool; each disposable
counter retained exactly one expected marker.

The complete Go agent/process-management suite passed, as did targeted journal
tests with the race detector, TypeScript checks, 47 UI tests, four domain tests
and 15 Rust tests. The desktop bundle built successfully. The Go agent also
cross-compiled for Windows; real provider/crash validation ran on macOS.
