## Context
`desktop/src/main.js` implements the project ticket list (Backlog) and `openBoard`/`renderBoard`. Single-ticket launches call `launchServerTask`; Electron `launchRequest` and the local agent `desktopTasks` currently omit batchTaskIds. Server `batchLaunchMembers` already verifies distinct members, explicit project membership, the first-member lead and no forced launch; run-skill checks every member for busy execution before recording a batch.

## Decisions
1. Keep transient ordered selection by full task ID for the active project in the renderer. Board and Backlog consume the same selection. Preserve it across filtering and rerendering; project change or closing the ticket workspace clears it. An explicit clear action is available. Use selection insertion order as the dialog's initial order. Do not persist batches to storage.
2. Use a renderer preparation dialog with key/title rows, keyboard-operable move-up/down controls, editable worktree name, cancel and confirm. Reordering changes only launch order, never ticket stage or backlog position. Cancel keeps selection. Capture a preparation snapshot so refresh cannot silently replace its members; unresolved members block confirmation with a correction message.
3. Port or extract pure equivalents of `web/src/lib/batchPickup.ts` into a Desktop-compatible module. Trim the name and enforce `^[a-zA-Z0-9][a-zA-Z0-9_-]{0,79}$`; generate `batch-` plus sanitized keys, truncate at 80 and remove trailing separators. Build the existing `/pickup-issues` prompt with ordered full IDs, `.tasks/worktrees/<name>`, preservation of existing work, sequential execution and one combined PR. Do not interpolate a shell command.
4. Extend the existing launch call with an optional batchTaskIds array through `desktop/electron/preload.cjs`, `main.cjs`, `run-project.cjs`, and `internal/agent/agent_desktop.go`. Keep existing positional arguments and single-ticket serialization compatible. The local POST remains `/desktop/tasks?projectId=<id>` with taskID equal to the first ordered member, skillID `pickup_issues`, prompt, and batchTaskIds. Forward to `/api/tasks/<lead-full-id>/run-skill` using skillId, projectId, batchTaskIds, prompt and the existing targetDeviceId. Use autonomous mode as the existing pickup batch does; preserve engine resolution and workstation console settings. Never offer force for a batch.
5. Validate basic selection and name before submission, but retain server authority for eligibility and busy races. Verify membership through ProjectIDs semantics, including tickets belonging to several projects. Do not dispatch one request per member. Guard confirm while pending, close/clear only on accepted response, and retain order/name/selection with an accessible error after refusal. A transport failure must not automatically replay a possibly accepted launch; allow explicit retry through existing busy protection.
6. New runtime copy follows each touched surface's established language; do not translate existing strings in passing. Checkboxes have ticket-specific accessible names and do not activate card navigation, stage dragging or row menus. Normal single-ticket actions remain available.

## Target Files and Tests
- Renderer: `desktop/src/main.js`, `desktop/src/style.css` (confirm stylesheet location during implementation), a small pure batch helper module if useful.
- Bridge: `desktop/electron/preload.cjs`, `main.cjs`, `run-project.cjs`; extend request serialization tests.
- Agent: `internal/agent/agent_desktop.go` and Desktop request relay tests.
- Regression coverage: Desktop UI tests for both surfaces and preparation, pure helper tests for validation/prompt, agent forwarding tests; retain server batch launch tests in `internal/handlers/batch_launch_test.go`.
- Documentation: `CHANGELOG.md` Added entry when behavior is implemented.

## Rejected Alternatives
- Launching tickets independently loses shared worktree, membership and combined PR semantics.
- Reimplementing server busy rules is race-prone and duplicates authority.
- A saved batch entity or new scheduling endpoint exceeds settled scope.
- Table-only or sidebar-only selection contradicts the accepted clarification.

## Risks and Validation
Older agents can silently discard unknown fields: expose a batch-launch capability in agent status and disable batch confirmation with an update message when absent, rather than falling back to single-ticket execution. Cover this compatibility gate. Preserve existing single launches. Test response refusal and duplicate submission behavior. No new child process is needed; any introduced internal process must use agentexec.Hidden under AGENTS.md.
