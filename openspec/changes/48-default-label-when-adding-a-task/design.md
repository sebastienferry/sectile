# Design — lowercase task creation label

## Context
`internal/db/db.go` applies `SetWorkflowLabel(req.Labels, "New")` in `CreateTask`; `CloneTask` similarly prepares `New` before delegating to creation. The helper already removes workflow labels case-insensitively and preserves custom-label casing and prefix conventions. `web/src/components/QuickAddModal.tsx` resets labels to `['New']` when opened and submits them. QuickAddModal and TaskCard add the visible hash during rendering. ListView already names the new-stage label `#new`.

## Decisions
1. Change the two creation defaults in `internal/db/db.go` to `new` and update the adjacent creation comment in English. Keep `SetWorkflowLabel` unchanged: its caller controls target casing and prefix. The local creation label remains unprefixed `new`.
2. Change the quick-add initialization/reset default to `['new']`. Reuse current rendering and submission so the badge reads `#new` with exactly one hash. Preserve label editing and removal behavior.
3. Preserve the default `to_clarify` status and explicitly supplied statuses in creation and cloning. This correction does not reconcile status and workflow labels or modify clone field selection.
4. Preserve provider-specific translation, including Linear's separate `new` to `New` mapping. No remote taxonomy or historical data migration is part of this change.

## Target Files
- `internal/db/db.go`: creation and cloning defaults and adjacent comment.
- `web/src/components/QuickAddModal.tsx`: initial labels when opening/resetting the form.
- `internal/db/labels_test.go`: focused returned/persisted creation and clone regression coverage using existing isolated database fixtures.
- Existing UI verification mechanism: record browser acceptance for the form, outgoing request, and task display; no new frontend framework is required.

## Rejected Alternatives
- Lowercase every label in the helper or renderer: changes custom labels and other callers beyond the requested scope.
- Change rendering alone: leaves uppercase creation payloads and persisted labels.
- Store `#new` for creation: unnecessarily changes the current unprefixed storage convention.
- Migrate all existing tasks or change provider mappings: exceeds clarified scope.

## Verification
Use local-source test tasks and isolated databases to avoid remote issue creation. Cover empty labels, legacy workflow variants (`New`, `#New`, `#Specified`), mixed-case custom labels (`CustomerCase`, `#TeamTag`), default/explicit status, and clone include-labels enabled/disabled. Assert both returned and reloaded labels, exactly one workflow label, and unchanged clone source. Reuse existing database fixture conventions.

Run focused database coverage, then the repository's `make test` and `make build` checks during implementation. Record browser verification of first open, close/reopen, unchanged-default submission, and the created task label in a view that displays labels. Existing compact cards may hide labels; no display-density change is required. Record any pre-existing check failures separately.

This specification stage runs only OpenSpec validation and diff checks; production tests belong to implementation. No README installation/configuration/major-feature change or architectural decision requires additional documentation. No changelog is present.

## Open Questions
None. Reuse branch `48-default-label-when-adding-a-task`. The local config's stale local-tracker identity is preserved; live TaskFlow task/project context verifies GitHub repository `sebastienferry/taskflow` and OpenSpec for this invocation.
