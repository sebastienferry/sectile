# ADR 0009: Apply model and mode selections with the next message

- Status: Accepted
- Date: 2026-10-28

## Decision

Expose model and Plan/execution selectors in both the first-prompt composer and
existing conversations. Selections are local draft parameters until a message is
sent. Go remains authoritative for accepted model and mode state. A preserve
option omits the field entirely, including when the current mode is read-only or
a different configured permission mode. Read-only histories and active turns do
not permit changing these controls.

Model choices merge the workstation provider catalogue from `/desktop/engines`
with the conversation's live model list (Codex), retaining its current identifier.
A custom identifier supports CLI aliases and catalogues not available locally.
The UI does not claim a model is entitled for an account; the provider validates
availability. Switching the provider before creating a conversation clears draft
choices. In-flight catalogue reads cannot update an unmounted/provider-switched
component.

The native message action accepts optional model and typed `plan`/`execute`
parameters. Model identifiers follow the existing Go identifier syntax. A native
read of the target conversation determines the provider; the renderer cannot
supply an arbitrary permission mode. Plan maps to `plan` for both providers.
Execution maps to Claude `default` or Codex `workspace-write`. Existing Go sandbox,
network and approval rules apply. No global workstation preference is changed.

## Validation

UI tests cover initial and subsequent messages, merged catalogues, current-model
retention, custom IDs, catalogue failure, read-only history and preserving draft
choices across snapshot refreshes. Native tests cover provider-specific mode
translation, unsupported modes/providers and model argument-injection rejection.
Existing Go mode/model conversation tests validate forwarding to provider CLIs.

Packaged desktop checks succeeded with real providers. The first Claude prompt
selected the `sonnet` CLI alias and Plan in the new-prompt form; Go reported both
choices and Claude replied `CLAUDE_MODEL_PLAN_OK`. An existing Codex conversation
selected `gpt-6.1-sol` from its live catalogue and Plan before its next message;
it replied `CODEX_MODEL_PLAN_OK`. Execution-mode translation was covered by native
and Go tests without running file-changing actions during the UI smoke checks.
Final checks passed: 45 UI tests, four domain tests, 15 Rust tests, targeted Go
mode/model tests, TypeScript validation and the packaged desktop build.

## Plan-to-execution context verification (2026-10-28)

Both providers were tested through the packaged desktop selectors in a new
non-Git temporary folder, with one independent conversation per provider.
The Plan message supplied a unique nonce and output filename and explicitly
forbade tool use or file creation. The execution message did not repeat either
value, requiring the provider to recover them from its existing context.

| Provider | Selected model retained | Plan mode | Execution mode | Result |
| --- | --- | --- | --- | --- |
| Claude Code | `sonnet` | `plan` | `default` | Wrote and reread the remembered nonce in `claude-transition.txt` |
| Codex | `gpt-6.1-sol` | `plan` | `workspace-write` | Wrote and reread the remembered nonce in `codex-transition.txt` |

The planned output was absent before each transition. Claude requested an explicit
one-time Write approval, which was granted for that exact fixture file. Both
providers then reported `EXECUTION_OK`. Read-only checks of the agent snapshots
confirmed unchanged conversation IDs and model selections, the expected mode
transition, and exactly two user turns per conversation. Direct filesystem checks
confirmed byte-for-byte nonce contents and only the two planned files in the test
folder. No runtime change was needed for this verification.

## In-conversation model-switch verification (2026-10-28)

The same two desktop conversations were reused after the Plan-to-execution test.
Claude switched from `sonnet` to `haiku`; Codex switched from `gpt-6.1-sol` to
`gpt-6-luna`, selected from its live catalogue. The mode selector was left on
preserve, so the model switch did not resend or change the execution mode.

Each new prompt deliberately omitted the earlier nonce and filename and forbade
tool use. Both providers returned `MODEL_SWITCH_OK`, the exact remembered nonce
and filename, and an accurate summary of the earlier Plan and execution turns.
The desktop displayed the previous transcript and the new answer together.

Read-only snapshot assertions verified the changed model, unchanged conversation
ID, unchanged mode (`default` for Claude, `workspace-write` for Codex), byte-for-byte
retention of the previous event prefix, exactly one additional user turn and zero
new tool calls. Both fixture files retained their exact nonce contents, and no
other files appeared. No runtime correction was required.

## Desktop quit/reopen verification (2026-10-28)

The packaged desktop was fully quit through its application menu, and a process
check confirmed its executable had exited. The existing Go services stayed
reachable while the desktop was closed; both conversations were still writable
and their full event arrays matched the captured baseline.

After reopening the desktop, both conversations were selected from the project
sidebar. Their transcripts, current models (`haiku`, `gpt-6-luna`) and execution
modes were displayed correctly. Each accepted a new message and returned
`DESKTOP_RESUME_OK` with its original nonce and output filename, without those
values being repeated in the new prompt and without using tools.

Snapshot assertions confirmed unchanged conversation IDs, model and mode,
byte-for-byte preservation of the pre-quit event prefix and exactly one new user
turn per conversation. Neither history became read-only. Fixture contents stayed
unchanged. This verifies desktop-only quit/reopen with the Go services continuing;
it does not test restarting the agent or the workstation. No correction was needed.

## Initial agent restart verification: resumption unsupported (2026-10-28)

An actual graceful restart of the isolated Go companion (old and new listener
PIDs verified) restored both test transcripts with their previous event prefixes
intact. Both existing conversation IDs were retained, but the conversations became
`readOnly: true`. The desktop disabled the composer and selectors. Explicit
continuation attempts were rejected with HTTP 409, `Conversation is unavailable`.

The saved run metadata retained `haiku` and `gpt-6-luna`, but the conversation read
endpoint no longer exposed model or mode because no providerConversation runtime
was reconstructed. At that point, `storedRun` persisted the desktop run and trace, not the
provider session/thread ID or conversation mode. `restoreRuns` intentionally
constructed an exited, restored history entry without a provider conversation.

At that point desktop-only quit/reopen was validated, but resumption after agent
restart was not implemented for either provider. Provider CLI context may remain
in its own storage; this test could not resume it through Sectile. A future change
must persist resumable provider identity and turn settings, distinguish resumable
idle conversations from intentionally stopped histories, and explicitly rebuild
the provider runtime on continuation without replaying previous prompts or tools.

## Restart recovery implementation (2026-10-28)

The Go run store now checkpoints provider session identity and turn settings for
free Claude/Codex conversations. It reconstructs idle writable conversations and
resumes only on an explicit new message. The initial limitation above remains
applicable to legacy records that have no provider identity. See Go ADR 0062.

Real-provider verification used the native desktop and its isolated companion in
a temporary folder. Claude `haiku` in Plan and Codex `gpt-6-luna` in execution each
memorized a synthetic nonce and filename before graceful SIGTERM of the owned
agent. After starting a new listener from native Settings, both original run IDs
were writable, their transcript prefixes matched exactly and their model/mode
were unchanged. Both replied `AGENT_RESUME_OK` with the original values when the
new prompt intentionally omitted them. No tools or errors appeared in either
conversation. The legacy read-only histories remained intact.

The same conversations also passed a second agent restart: both retained their
entire previous transcript and recalled the initial values and first restart
verification with `SECOND_RESTART_OK`. Model and mode remained unchanged.

## Initial abrupt agent crash during active tools (2026-10-28)

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

## Crash-safe provider ownership (2026-10-28)

Go ADR 0063 adds an independent Unix provider guard with an owner-only lifetime
pipe, and suspended assignment to a Windows kill-on-close job. Claude turns and
Codex app-servers use this ownership path. No renderer API or frontend control
changes are required. This supersedes the surviving-tool limitation observed in
the initial crash test above; checkpoint and partial-stream durability limits
remain unchanged.

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

## Incremental conversation journal (2026-10-28)

Go ADR 0064 supersedes the five-second conversation-loss window and unsaved-draft
limitations recorded in the earlier tests above. Normalized events and streamed
text deltas sync before publication. Recovery marks saved partial replies as
interrupted history, without creating a new provider prompt or replaying tools.
The desktop displays a persistence warning when local commits fail.

Real SIGKILL validation ran both providers during long assistant streams after
one completed shell tool per conversation. No checkpoint wait was inserted. Both
base snapshots still held zero partial characters. Observed Claude/Codex partials
were 784/153 characters; recovery retained 811/221 characters, including additional
chunks received before the kill landed. All observed message/tool/result event
prefixes survived. Native screens showed the interrupted label and kept Markdown.
Both sessions resumed with JOURNAL_RESUME_OK, their original nonce, fictional
filename and tool result; model/mode stayed unchanged and neither counter changed.
