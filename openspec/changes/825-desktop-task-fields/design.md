# Design

## Context and evidence
Accepted source: docs/clarifications/825.md round 2. `desktop/src/task-page.mjs` shares create/edit UI; `task-form.mjs` computes changed fields. `desktop/electron/main.cjs` and `preload.cjs` expose agent-backed IPC. `internal/agent/agent_desktop.go:desktopCreateTask` does not currently forward type/priority; `agent_desktop_task.go` has an explicit edit allowlist. `models.CreateTaskRequest` and `UpdateTaskRequest` already define `issueType` and `priority` (pointer update semantics). `internal/db/board.go:ListTrackerIssueTypesAs` uses provider capability checks and bounded discovery. The existing `/api/admin/trackers/{id}/issue-types` route is administrative; it is not suitable for the Desktop member flow.

## Decisions
1. Keep behavior in the shared page and pure form helpers. Use native labeled selects and existing styles; remove page assignee UI/search state and assignment diff writes, retaining APIs used by other surfaces.
2. Add project-scoped read-only discovery, proposed `GET /api/projects/{projectId}/trackers/{trackerId}/issue-types`. Authenticate with existing paired-user/session mechanisms, authorize project access and verify tracker membership in that specific project before calling existing provider discovery. Do not relax admin configuration routes or rely solely on global TrackerHasProjects. Reuse existing acting context and provider capability limits; do not change tracker mappings or introduce a migration.
3. Discovery response contract: `{supported: true, issueTypes: [string]}` for successful (possibly empty) discovery; `{supported: false, issueTypes: []}` for the typed unsupported-capability outcome. Other errors retain non-success HTTP status and existing error handling. Arrays are never null. Remove blank/duplicate options without altering type values. No tracker credentials/configuration are returned.
4. Bridge with a read-only agent route, proposed `/desktop/tasks/issue-types?projectId=...&trackerId=...`, and a narrow preload method `taskIssueTypes(projectId, trackerId)` backed by an IPC handler. Validate string arguments, project configuration and selected tracker membership; server rechecks authorization. Existing task detail checks establish the editing task's membership; resolve discovery against its tracker ID rather than project default.
5. Model options with effective project/tracker identity plus a monotonically increasing request generation; invalidate on switch, retry and page destruction. Default/loaded options are always available. For creation, preserve the previous selection while resolving a switch only if it is confirmed in the new successful list; otherwise use Default. Unsupported/empty new trackers yield Default. A failed discovery does not reset edits but cannot make old tracker options appear current; retain the selected value as pending rather than fabricate options. Editing preserves the loaded type throughout. Discovery updates alone do not mark dirty; intentional selection and effective tracker changes follow existing creation dirty-state rules.
6. Extend initial creation forwarding with `issueType` and `priority`; extend the edit allowlist with pointer fields so explicit empty type reaches the server. Remove assignment fields from this form's payloads, not server assignment support. Keep workflow/project allowlist protection. Initialize the editing baseline from the actual returned task after creation/save. Preserve existing follow-up PR saves and edit mode after partial success.
7. Advertise a new local agent capability, e.g. `task-fields`, for write support and discovery. Electron must check it before sending new field writes: the existing `task-page` capability alone cannot prove an older agent forwards them. Surface update/restart guidance, preserving pending inputs; no silent downgrade of a requested write. Unsupported provider discovery is different from missing agent support.

## Target files and checks
- Renderer: desktop/src/task-page.mjs, task-form.mjs, style.css only as needed.
- IPC: desktop/electron/main.cjs, preload.cjs; capabilities in the agent desktop status.
- Agent: internal/agent/agent_desktop.go, agent_desktop_task.go and focused tests.
- Server: internal/handlers/trackers.go or the project route dispatch in handlers.go, authorization helpers as needed; internal/db/board.go only for safe reusable discovery adaptations; handler tests.
- Tests: desktop/tests/task-form.test.mjs, task-page.ui.cjs and applicable IPC tests; internal/agent/agent_desktop_task_test.go and creation forwarding tests; internal/handlers/trackers_test.go for authorization/provider outcomes.
- CHANGELOG.md during implementation.

## Rejected alternatives
- Fixed web type lists or tracker import filters contradict the accepted clarification.
- Calling admin discovery would prevent ordinary paired members from using the form; broadening admin access would expose unrelated configuration.
- Renderer-only changes would silently drop values in the agent.
- Reusing task-page capability would allow old agents to claim unsupported writes succeeded.
- Clearing assignment or editing tracker mappings extends the agreed scope.

## Risks and verification
Verify provider unsupported errors by their typed capability outcome, not by message text. Test foreign tracker/project requests before provider calls, default tracker resolution, task tracker differing from project default, stale/reversed requests, missing loaded types, explicit empty updates, creation followed by failed PR save, and old-agent rejection. Keep French server messages and English Desktop wording in their existing surfaces. No child process is needed; any introduced command must follow AGENTS.md agentexec rules.

## Open questions
None. Route names are implementation choices; the authorization and response semantics above are the required contracts.
