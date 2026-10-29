# ADR 0005: Evaluate CopilotKit headless for project conversations

- Status: Superseded by ADR 0006 (2026-10-28)
- Date: 2026-10-28

This ADR records the historical experiment. CopilotKit, AG-UI and RxJS have since
been removed; current conversations use the React controller described in ADR 0006.

## Context

The desktop needs a fast free-prompt experience scoped to a mapped project.
The existing Go agent already owns conversations, provider processes, permission
requests, interruption and history. Introducing another execution runtime would
duplicate that responsibility and complicate desktop packaging.

## Decision

Use CopilotKit 1.83.0 headless React hooks with an AG-UI `AbstractAgent` adapter.
The adapter sends only the newest message through narrow Tauri IPC commands and
translates Go conversation snapshots into AG-UI state events. Go retains provider
context and execution authority. The transcript and tool details use plain React
text, never raw HTML. Tool execution stays entirely in Go; there are no frontend
tools, generated interfaces or autonomous approval decisions.

The prototype intentionally uses `agents__unsafe_dev_only`. The installed SDK
labels direct registration development-only and requires CopilotRuntime for its
supported production path. This is not a claim of a supported runtime-free
production integration. `selfManagedAgents` carries a separate Enterprise
Intelligence licensing notice; we do not use it. No CopilotKit key, runtime URL,
Node sidecar, Intelligence service or cloud transport is configured. The existing
native CSP continues to permit only IPC connections from the renderer.

The free-prompt module is lazy-loaded. Existing ticket launch remains available
from board cards. Navigation never interrupts execution. Reopening a conversation
reads Go history without resending any prompt. A connection failure requires
explicit observation recovery; it never retries a mutation automatically.

## Measured cost

Vite production build, same workstation and configuration:

| Asset | Before | Prototype |
| --- | ---: | ---: |
| Main JavaScript | 258.83 kB / 80.46 kB gzip | 259.29 kB / 80.59 kB gzip |
| Lazy project prompt | none | 663.66 kB / 164.99 kB gzip |
| Console JavaScript | 333.67 kB / 84.12 kB gzip | unchanged |

These are bundle sizes, not measured startup latency or memory consumption.
The dependency installation grew from 101 to 1052 audited packages; dependencies
are not bundled wholesale into the desktop. `npm audit` currently reports 18
transitive findings (12 high, 6 low), originating in Zod and KaTeX. The headless
path does not render KaTeX; Zod is in the imported agent stack. Do not force major
transitive upgrades without upstream compatibility checks. This unresolved audit
is another reason to keep this evaluation out of production releases.

## Limits and evaluation

- Polling every 750 ms while a turn runs; no new Go streaming endpoint.
- Full bounded Go history is reread; large transcripts can be expensive.
- Go snapshots drive the UI. This does not yet use CopilotKit chat components,
  AG-UI text deltas or tool renderers: those require additional event mapping.
- The UI supports allow-once and deny. Provider questionnaires and persistent
  permission rules are not implemented here; deny a questionnaire and clarify
  in the next message. No implicit escalation of permissions.
- Restored or stopped conversations are read-only according to Go.
- No app-side transcript persistence. CLI authentication and a mapped local folder
  are prerequisites, separate from tracker OAuth.
- Automated tests exercise the real CopilotKit core against mocked Go IPC.
  A real provider session still needs a manual end-to-end test.

Initial assessment: the dependency cost is substantial for a state subscription
hook. Compare the actual prompt experience before adopting the framework. A
small native React adapter using the same Go routes remains a viable alternative.

## References

- https://docs.copilotkit.ai/backend/copilot-runtime
- https://github.com/CopilotKit/CopilotKit
- Installed `@copilotkit/core` configuration declarations and React provider
  implementation (1.83.0) for direct-agent restrictions.
