# ADR 0006: Observe Go conversations with a lightweight React controller

- Status: Accepted
- Date: 2026-10-28
- Supersedes: ADR 0005

## Context

The CopilotKit experiment used only headless state subscription and lifecycle
handling. Our React components already rendered the composer, transcript, tools
and approvals. Go already owned provider execution, history, context, permissions,
interruption and thread resumption. Translating Go snapshots into AG-UI state
snapshots added an extra state model with little benefit to this desktop.

## Decision

Remove CopilotKit, AG-UI and RxJS. A conversation controller subscribes React
through `useSyncExternalStore` and calls the existing narrow native IPC bridge.
No Go or Rust execution API changes are needed.

The controller maintains one snapshot per conversation, polls non-overlapping
reads every 750 ms while Go reports a busy turn and stops on idle or error.
It prevents duplicate sends, sends only the latest prompt and never retries an
ambiguous mutation. Explicit observation refresh recovers from errors. A revision
counter discards reads started before mutations. Refreshes requested during an
in-flight read or send are queued, so post-mutation state is read afterward.

Navigation detaches React listeners without interrupting Go. Active observers
continue until idle; the cache evicts idle, unobserved entries when it reaches
20 entries. Go remains the source of history when a view is reopened. Tool
approvals require explicit user action. Read-only histories remain read-only.

Keep the existing interface and lazy-loading boundary. Replace the framework
badge with a plain local-agent label; do not add new product flows in this change.

## Validation and cost

The existing conversation tests cover creation, observation, latest-message-only
sending, read-only history, approvals, interruption and recovery. Additional cases
cover duplicate sends, stale reads crossing mutations, lost acknowledgements and
observation recovery while the last known Go state is busy.

The pre-change production conversation module was 663.85 kB (165.06 kB gzip).
The final React-only build produced 6.61 kB (2.43 kB gzip), roughly a 99% reduction
in that lazy module. The console module was unchanged. These are build sizes,
not measured startup latency or resident memory. Removing the framework removed
951 npm packages; the resulting 101-package audit reported zero vulnerabilities
at the time of migration. This is an audit result, not a security guarantee.

A paired measurement using Node zlib gzip for both builds recorded:

| JavaScript | Before (bytes) | After (bytes) |
| --- | ---: | ---: |
| Lazy project prompt | 663,851 | 6,611 |
| All JavaScript modules | 1,260,109 | 602,718 |
| All modules gzip | 327,718 | 166,549 |

The total raw reduction is 657,391 bytes (52.2%). Gzip implementations differ
slightly from Vite's printed estimates; paired values above use the same encoder.

Validation passed: 28 UI tests, four domain tests, 11 Rust tests and the disposable
Go-service integration scenario. The packaged desktop restored an existing real
Claude transcript with tool details through the React controller. A new real
Claude turn read the fixture file, displayed its Bash call and result, answered
`REACT_CONTROLLER_OK` and returned idle in the packaged desktop.

## Consequences

We own a small conversation state controller and its lifecycle tests. Future
Go event streaming can replace polling behind the same controller. Richer event
rendering or an interoperable agent protocol can be evaluated when required;
there is no framework runtime or AG-UI translation layer in the current renderer.
