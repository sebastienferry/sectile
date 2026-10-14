# Design

## Evidence and response contract
Accepted decisions: docs/clarifications/840.md round 3. AppContext.tsx currently uses createdTaskLink only for ordinary creation and imported macro stories; cloneTask and updateTask omit it. createStoryUnderMacro checks task presence rather than local identity. createStoriesFromMacroTodos emits one summary success toast with no links; it is included by the accepted all-creation-flows scope, while partial-batch warnings remain unchanged.

POST /api/projects/{projectId}/macros/{macroKey}/story in internal/handlers/handlers.go returns {task, storyKey, notice}, plus macro/epic for todo creation. Batch /stories returns results with status, storyKey and task (internal/db/macros.go MacroStoryOutcome). internal/db/remotetarget.go createStoryInRoadmapProject retains the provider Task and clears only ID and ProjectID. internal/trackerapi/jira.go CreateIssue returns a fetched task with an external URL, or uses jiraBrowseURL in its fallback after confirmed creation. Thus task.externalUrl is already available for tracker-only stories; no endpoint or backend schema change is required. Verify this chain with a focused regression test, including the post-create fetch fallback. Never derive a URL from a bare key or fabricate a local task.

## Technical decisions
1. Generalize createdTaskLink into an operation-neutral helper accepting the returned Task. A nonempty id selects internal navigation via setSelectedTask(task), plus externalUrl when available. Without id, require the returned externalUrl for an external-only action. Missing identity and URL must yield no inert/fabricated control; supported tracker-only success responses must be covered by contract tests ensuring their URL exists.
2. Extend ToastLink to represent either an internal callback (with optional tracker URL) or an external-only URL, using a discriminated union or equivalent type-safe shape that disallows an action with neither destination. Preserve existing callback-only credential offers. Keep the singular link field supported. Add optional links for a batch's ordered created results; render the same existing action row per issue, without altering the toast shell or adding a dependency. Producers use singular or plural, never both; a shared normalization helper may make renderer/timer interpretation consistent.
3. Wire ordinary create, clone, update and both single-story producers through the helper, keeping all existing side effects and return values. Use Boolean(task.id), not object presence, to distinguish tracker-only results. Epic aliases reuse the same producers. For an all-success batch, attach actions for status=created results only in response order; no extra success toasts and no changes to warning/error batches. Bulk edits continue emitting their existing per-update notifications.
4. Render internal actions as existing buttons that call onOpen and removeToast; render external-only actions as labeled anchors with target=_blank and rel=noopener noreferrer. Preserve the existing separate tracker icon for internal actions with an external URL. External activation retains current non-dismissal behavior. Use the existing openCreated/openInTracker translations, with the issue key in the visible label; preserve each surface's language.
5. Update toastDuration and ToastItem's holdable eligibility to recognize either singular or nonempty plural actions. Preserve explicit duration precedence, 8000ms linked default, 3500ms unlinked default, timer cleanup and independent hover/focus holds. Empty arrays do not extend lifetime. No CSS redesign or new timing policy.

## Target files and validation
- web/src/context/AppContext.tsx: helper and all scoped success producers.
- web/src/types/index.ts: ToastLink variants and optional plural actions.
- web/src/components/ToastContainer.tsx: action rendering and hold eligibility.
- web/src/lib/toastTimer.ts and web/tests/toastTimer.test.mjs: singular/plural/external-only eligibility and unchanged timer behavior.
- Focused web tests: verify returned-data navigation, clone openAfterClone true/false, bulk edits, both macro producers and batch filtering, external anchors, keyboard focus and unchanged credential offers. Use repository browser-test patterns for interaction coverage rather than source-string assertions alone.
- internal/db/remotetarget_test.go or existing roadmap-story tests and internal/trackerapi Jira tests: assert no local identity/persistence and a usable externalUrl reaches single/batch responses, including Jira fetch fallback. Keep production backend unchanged unless these contract checks reveal a genuine missing URL.
- During implementation run npm --prefix web test, npm --prefix web run lint and npm --prefix web run build, focused browser scenarios, and go test ./internal/db ./internal/handlers ./internal/trackerapi for any added Go contract coverage. Add the user-facing Unreleased changelog line.

## Rejected alternatives
- Tracker-only navigation for all tasks contradicts the accepted internal Open action.
- A dummy callback or synthetic task identity opens an invalid detail view.
- Linking only the first story in a batch omits successfully created issues; replacing the batch with per-item toasts changes existing notification grouping.
- Expanding warning/error or move/export notifications exceeds the accepted scope.
- Reconstructing tracker URLs in the browser duplicates provider knowledge already present in the response.

## Open questions
None. Plural action representation is an implementation choice supporting the accepted all-creation scope. No new service, migration or child process is needed.
