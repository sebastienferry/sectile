# ADR 0008: Answer provider questions through the existing Go approval contract

- Status: Accepted
- Date: 2026-10-28

## Decision

Render the existing Go `AskUserQuestion` approval shape as a question form for
both Claude Code and Codex. Go already maps Codex request IDs and question IDs to
this representation and validates answers against the currently pending request.
No Go protocol change is required.

A dedicated native `answer` action accepts only the request ID and a string map
keyed by question text. It emits the existing Go approval payload with decision
`answer`. Native validation bounds decoded answers to 16 KiB, 32 questions and
nonempty keys/answers. It does not permit arbitrary permission modes or decisions.

The form supports single-choice, multi-choice, free-text and Codex secret inputs.
Multiple labels and custom text use the existing Go adapter's comma-separated
answer convention. Every question must be answered. Unknown or malformed question
sets can be explicitly skipped, never implicitly allowed as ordinary tool calls.

Drafts remain local to the mounted request and survive snapshot refreshes. A
request or schema change remounts the form. Acknowledged submissions clear drafts
and stay disabled while awaiting removal of the pending request. In-flight sends
are guarded. Failed acknowledgements do not replay automatically; errors are
shown without echoing answers, and secret drafts are cleared on failure. Codex
secret answers remain redacted by the existing Go history adapter.

## Validation

UI cases cover single and multiple selections, custom answers, draft retention,
masked fields, sanitized failures, duplicate-click prevention, malformed question
skip and the complete conversation-to-IPC answer payload. Native tests cover
payload structure, empty/oversized answers and unknown-field rejection. Existing
Go tests cover Claude answer forwarding and Codex native question-ID translation.

Packaged desktop checks succeeded with both real providers: Claude emitted
`Which test scope?`, the desktop submitted Small and Claude continued with
`QUESTION_OK Small`. Codex, running in its existing plan mode, emitted
`Which review scope?`; submitting Large from the same desktop form produced
`QUESTION_CODEX_OK Large`. Neither question was denied or answered as a new prompt.
Final checks: 41 UI tests, four domain tests, 13 Rust tests, targeted Go question
contract tests and the packaged desktop build passed.
