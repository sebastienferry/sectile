# Implementation tasks

- [x] 1. Add focused contract coverage showing tracker-only story creation retains externalUrl while leaving id/projectId empty and persisting no local task. Cover single and batch response propagation and the Jira post-create fetch fallback; reuse existing provider fixtures.
- [x] 2. Extend ToastLink for external-only destinations and optional plural actions while keeping callback-only credential offers valid. Add a shared issue action helper using the returned task identity, key and URL.
- [x] 3. Render internal, internal-plus-tracker and external-only action rows with existing styles, safe anchors and accessible localized labels. Support all created actions in a successful batch without changing its summary or emitting extra toasts.
- [x] 4. Recognize singular/plural actions for the existing linked-toast lifetime and pause behavior. Test explicit duration precedence, empty lists, hover plus focus holds, focus moving inside and timer cleanup.
- [x] 5. Attach issue actions to ordinary creation, cloning, updateTask, both macro story producers and successful macro batches; retain epic aliases, bulk-edit cardinality, clone auto-opening and returned data. Leave warnings, credential offers, failures and excluded operations unchanged.
- [x] 6. Exercise local/imported/tracker-only tasks, missing optional tracker URLs on imported tasks, clone auto-opening enabled/disabled, bulk edits targeting distinct issues, macro title/todo creation, parent warnings, all-success mixed-target batches, skipped entries and partial failures. Verify actual navigation/dismissal and new-tab anchors through focused browser tests.
- [x] 7. Add an English user-facing Fixed entry under CHANGELOG.md Unreleased describing links from issue creation/edit notifications.
- [x] 8. Run web tests, lint and build, focused browser interaction checks and affected Go contract tests as listed in design.md. Run openspec validate 840-issue-toast-links --strict and report results before implementation completion. Keep the specification PR draft until the later workflow requires readiness; do not mark reviewed here.

## Implementation verification (2026-10-14)

- Web unit suite: 751 passed, 0 failed.
- Web lint: exit 0; existing repository warnings remain.
- Web build: TypeScript and Vite pass; the existing large-bundle advisory remains.
- Browser regression: real AppProvider producers, returned-data navigation, internal dismissal, optional tracker URLs, clone auto-opening enabled/disabled, independent bulk updates, both macro creation producers, parent warnings, mixed successful batches, skipped/partial/empty results, safe new-tab activation and keyboard focus holds pass.
- Existing credential-offer browser regression is run separately to retain callback-only actions.
- Go contract suites: internal/db, internal/handlers and internal/trackerapi pass; focused Jira URL assertions pass after final additions.
- OpenSpec strict validation and git diff --check pass.
- No backend production changes, migration or dependency added. Accepted clarification and specification retained. PR #845 stays draft; this stage does not mark reviewed.
