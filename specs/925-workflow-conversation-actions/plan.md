# Technical plan — #925

## Stack and boundaries

Use existing React/TypeScript Tauri renderer, CSS and Vitest/Testing Library tests. No dependency, Rust/Go API, persistence schema, backend endpoint or migration change is required. Existing `specs/` is the initialized Spec Kit directory. Behavior is defined separately in `spec.md`.

## Existing contracts to preserve

- `desktop-tauri/src/domain/workflow.ts`: `nextSkill(task, projectContext)` resolves stages and PR recovery, returns an optional `skillId`, label and reason. Keep it as the single resolver; do not adopt Electron's post-success offer rules.
- `Conversation` in `desktop-tauri/src/components/ProjectPrompt.tsx` computes `step` and `anyTaskRunActive` from `taskRuns` and the selected run using `isActive`. The header currently requires `run.taskId`, `step.skillId`, `onLaunchSkill`, and no active task run; `!agentAvailable` disables it. Add `snapshot.readOnly` only for the contextual version.
- Both next-step controls call `onLaunchSkill(step.skillId)` through existing `App.tsx` wiring and `startSkill`; do not create a new launch API or directly transition the ticket. Preserve existing launch error handling and lifecycle.
- `stopConversation` guards `stopping`, missing run and inactive run; it calls `stopRun(id)`, refreshes the controller and calls `onStopped`. Preserve its error text, cancellation guards and `finally` handling.
- Stop rendering currently depends on selected-run activity, with disabling for unavailable agent, `stopping`, or `run.cancelRequested`. It does not depend on `snapshot.busy`; retain this distinction for idle sessions.
- `snapshot.readOnly` selects history versus writable composer layouts; both render `ConversationStatus`. The controlled draft and folder/context state remain owned by the existing components/controller.

## Component design and target files

1. `desktop-tauri/src/components/ProjectPrompt.tsx`: derive shared next-step rendering/props once in `Conversation` so both locations use identical eligibility, tooltip, accessible name, disabled state and callback. A small local helper or shared element is sufficient; no new workflow abstraction is needed.
2. Extend `ConversationStatus` with optional action content (for example a `ReactNode` slot) rendered immediately after its Workflow toggle. Pass the contextual next-step only for `snapshot.readOnly` and existing eligibility. Pass the existing Stop control for the selected active run in both layouts. Render the slot even without a task chip so free conversations retain Stop access.
3. Remove active Stop from the header, preserving its inactive `onArchive` branch and Start over. Retain header next-step and workflow menu. Keep Stop and next-step as separate semantic `type="button"` controls, outside the Workflow toggle.
4. `desktop-tauri/src/App.css`: reuse `icon-button`, the existing next-step compact treatment and stop styling. Add only placement/spacing rules required by the strip. Verify wrapping, focus visibility and narrow widths; avoid changing unrelated header styling.
5. `desktop-tauri/tests/prompt.ui.test.tsx`: extend current header/Stop/read-only tests, using `within` to distinguish identical next-step names in header and status strip. Reuse mock API/controller setup and assert `onLaunchSkill` arguments and Stop requests.
6. `desktop-tauri/src/App.tsx`, `desktop-tauri/src/lib/conversation-controller.ts` and workflow domain are reference contracts; change them only if a concrete integration defect requires it, not for placement.
7. Update `CHANGELOG.md`, `desktop-tauri/CHANGELOG.md`, `README.md` and `desktop-tauri/README.md` during implementation with user-facing placement and lifecycle guidance. Preserve runtime wording; repository prose stays English.

## Validation matrix

Test read-only/writable selection; inactive/active selected run; another active task run; idle active session; archived history; unavailable agent; stopping/cancel requested; missing task/callback/configuration/skill; finished/unknown stages; PR recovery. Assert status-strip adjacency and header retention/removal independently. Cover callback-once behavior, stop failure/refresh, Workflow toggle independence, keyboard activation and preserved drafts/context. Include free-conversation Stop regression.

For implementation, run from `desktop-tauri`: `npm run check`, `npm test`, `npm run test:ui`, `npm run build`, and `npm run test:rust`, as required by its AGENTS.md. Inspect narrow/wide layouts and disabled/focus states visually. This specification-only invocation validates document consistency and whitespace; it does not claim production checks have run.

## Tradeoffs and risks

A status action slot keeps workflow decisions and stop state in their current owner and prevents duplicate resolution in the strip. Two next-step buttons deliberately share an accessible name; tests must query by region. Read-only status alone cannot authorize launch: task-wide run activity must remain part of eligibility. Archived status is not an exclusion rule. An idle active provider must retain Stop and must not become eligible for next-step merely because it is not busy. A significant architecture decision is not introduced, so no ADR is needed.

## Delivery

Reuse assigned branch `feat/925` and preserve accepted clarification commits. Check ignore status before staging; ignored artifacts stay local and are summarized in the transition note. Commit only non-ignored specification artifacts. Project setting `pushStageCommits=false` means no specification push. PR creation is deferred to the configured implemented stage; no PR or reviewed transition belongs to this invocation.
