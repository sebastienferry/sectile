# ADR 0002: Execution control through the existing local agent

- Date: 2026-10-28
- Status: Accepted

## Decision

Use fixed native commands for the Go agent's task launch, status, run list,
stop and named orchestration endpoints. The renderer cannot select an arbitrary
route, HTTP method, bearer token, shell command or filesystem path. Typed launch
payloads reject unknown fields, invalid modes and oversized requests. No `force`
option is exposed. Keep provider selection, workflow admission, worktrees,
authorization and project capacity in the existing Go implementation.

Poll status and runs sequentially every three seconds, never overlapping polls.
Keep the last snapshot visible but mark it stale after a failure or ten seconds
without an update. Scope visible runs to their recorded project. Track mutations
separately from polling, prevent duplicate submission, and refresh after success
or failure. A POST timeout has an uncertain outcome and must never auto-retry.
Only the Go agent's stop acknowledgement confirms a stop.

Read console bytes from the authenticated local `/desktop/terminal` WebSocket
through a Rust-owned subscription and Tauri channel. Render with xterm's parser
and bounded scrollback rather than injecting output as HTML. No terminal input,
resize, approval or shell execution messages are sent. Closing a view cancels
its subscription; it does not stop the run. A new view receives the agent's replay.
Provider approvals remain in existing clients. The UI explains this limitation.

Orchestration runs a saved named profile once. Profile configuration remains in
Sectile Desktop. The Go server validates guidelines and enforces shared capacity;
this client does not infer remaining capacity from this workstation's run count.
The response lists launched and skipped tickets, including refusal reasons.

## Validation

HTTP fixture tests exercise encoded identifiers, authenticated POST payloads,
204 stop acknowledgements, error detail/redaction and redirect refusal. UI tests
exercise launch de-duplication, stop failure, orchestration reports, stale polls,
and polling cleanup. Domain tests preserve project scoping and pickup semantics.
These tests do not launch real provider tasks or change tracker data.
