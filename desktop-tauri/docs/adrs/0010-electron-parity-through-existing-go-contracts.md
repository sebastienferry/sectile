# ADR 0010: Restore desktop journeys through existing Go contracts

- Status: Accepted
- Date: 2026-10-28

## Context

The first Tauri implementation reduced workspace setup to one folder and ticket
execution to a generic launch form. Electron also provides project attachments,
repository/specification roles, guided workflow steps, server-confirmed skill
results, code review and saved orchestration policies. An independent UX review
identified missing provider preparation and misleading message-private attachment
semantics before implementation began.

## Decision

Reuse existing Go endpoints instead of rewriting the executor or inventing local
workflow state. Add a bounded Rust project view that omits embedded prompts and
credentials, native-selected folder operations, typed policy/orchestration writes
and run-bound review/editor operations. Keep free project conversations separate
from tracker ticket workflows and keep one remote ticket collection with alternative
grouping. Resolve displayed stages from labels, tracker mappings and statuses;
Go/server remain authoritative for all transitions and launch admission.

Folder additions from a composer persist at project scope. The UI names that
scope and their next-message effect. Existing conversations show their own recorded
directory, even after the project directory changes. Detachment removes an
association only. Native settings expose mapped repositories and separate Macro/
Issue specification roots without making them prerequisites for a simple prompt.

Skills, MCP and permissions are separate from CLI sign-in. Provider setup is
explicit and does not dispatch work. Existing MCP entries require explicit
replacement consent when they do not match this companion. Configuration status
is not presented as proof of a successful provider-session connection. Keep the
retired Electron setup wizard retired; expose direct controls in engine settings.

Next-step launches reread project/task/run data and block concurrent or changed
work. An accepted request stays pending until its execution appears; observation
never resubmits it. Skill verdicts use the matched server activity and workflow
stage, not the terminal exit alone. Pickup displays its configured stopping point.
Manual review declarations are separate explicit server writes with a justification.

Named orchestration definitions and Markdown previews stay behind existing Go
validation. Saving never evaluates the backlog. Disabling future launches does
not stop current tasks; individual cancellation remains explicit. Code review
reads only folders belonging to a run and renders patches as text.

## Consequences

No new runtime dependency or browser credential store is introduced. Heavy terminal
and workflow views load lazily. Tests cover fresh-state launch guards, PR recovery
policy, result validation, attachment scope, setup consent and configuration-only
operations. Real provider checks use disposable folders; workflow transition tests
use controlled fixtures rather than writing to a real tracker.
