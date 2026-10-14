# Implementation checklist

- [x] 1. Add authorized project/tracker discovery with the supported/empty/unsupported/error response contract. Reuse provider discovery and paired acting context; reject foreign scope before provider calls. Add handler coverage for member/admin, unauthenticated, foreign tracker, empty, unsupported and provider failure outcomes.
- [x] 2. Add the read-only local agent route, preload/IPC bridge and task-fields capability. Validate inputs and membership, retain task-page protection and paired credentials. Test forwarding and outdated-agent rejection without exposing secrets.
- [x] 3. Extend agent create/update forwarding for issueType and priority. Test initial values, explicit empty type, changed-only updates and continued rejection of workflow/project fields. Verify form requests omit all assignment fields.
- [x] 4. Update pure form state/diff helpers for defaults, loaded values, type/priority dirty state and tracker option normalization. Add unit tests for custom loaded types, Default resets, unchanged values and assignment preservation.
- [x] 5. Update the shared page: required field order, remove Assignee/search state, add native selects, discovery loading/empty/unsupported/failure/retry states and generation guards. Preserve description and PR interactions. Test project/tracker switch, task tracker differing from default, reversed async responses and page destruction.
- [x] 6. Preserve save baselines and pending values across failures, successful creation followed by failed PR save, and old-agent errors. Add focused UI coverage for retry without duplicate creation, dirty/discard behavior, exact visual/focus order, default and stored priority, keyboard selects and narrow layout.
- [x] 7. Add the English Unreleased changelog entry. Review scope against accepted round 2; retain assignment elsewhere, web forms, priority mappings and workflow behavior.
- [x] 8. Run desktop unit and UI suites (`npm --prefix desktop test`, `npm --prefix desktop run test:ui`), desktop build (`npm --prefix desktop run build`), relevant Go agent/handler/db tests, and repository formatting/build/test checks required for implementation. Record actual results and any environment limitations; do not claim provider integration from mocks alone.
- [x] 9. Revalidate `openspec validate 825-desktop-task-fields --strict`; commit implementation and update the same draft PR. Transition only to implemented after implementation verification; readiness/review belongs to adjustment.

## Implementation report

The accepted clarification and specification are retained. No product decision changed.

| Files | Change and purpose |
| --- | --- |
| `desktop/src/task-page.mjs` | Shared field order, native type/priority selects, scoped discovery and retry, stale-request guards, dirty state and persisted save baselines; removed form assignment state. |
| `desktop/src/task-form.mjs` | Changed-only type/priority diffs and provider option normalization; assignment fields are never emitted by the form. |
| `desktop/electron/main.cjs` | Validated discovery IPC and capability checks preventing old agents from dropping field writes. |
| `desktop/electron/preload.cjs` | Narrow discovery bridge for the renderer. |
| `internal/agent/agent_desktop.go` | Discovery route registration, task-fields capability and initial creation forwarding. |
| `internal/agent/agent_desktop_task.go` | Project/tracker membership validation for read-only discovery and pointer-based type/priority updates. |
| `internal/agentconfig/config.go`, `internal/db/agentconfig.go` | Export the project's actual default tracker for creation discovery. |
| `internal/handlers/handlers.go`, `internal/handlers/trackers.go` | Signed-in project-scoped discovery using existing acting context and typed provider capability outcomes. |
| `desktop/tests/task-form.test.mjs`, `desktop/tests/task-page.ui.cjs` | Defaults, custom types, changed-only writes, assignment preservation, failures, retry, stale responses, dirty state, keyboard order and narrow layout. |
| `desktop/tests/console.ui.cjs` | Updated the mock creation agent's capability to match the new write contract. |
| `internal/agent/agent_desktop_task_test.go`, `internal/agent/agent_desktop_test.go` | Paired forwarding, scope refusals, capabilities, initial values and explicit empty updates. |
| `internal/handlers/trackers_test.go` | Member/admin authorization, acting user, foreign scope rejection before provider access and distinct discovery outcomes. |
| `internal/db/agentconfig_test.go` | Verify the published default tracker belongs to the project configuration. |
| `CHANGELOG.md` | User-facing English Unreleased entry for the shared Desktop form improvement. |

### Verification

- `make test` exited 0: all Go packages passed; web output was `# tests 737`, `# pass 737`, `# fail 0`; formatting, TypeScript and oxlint completed successfully. Existing web lint warnings remain.
- `npm --prefix desktop test` exited 0: `# tests 282`, `# pass 282`, `# fail 0`.
- `go build ./...`, `npm --prefix desktop run build` and `npm --prefix web run build` exited 0. Both Vite builds report large-chunk warnings.
- Strict OpenSpec validation reported `Change '825-desktop-task-fields' is valid`; JavaScript syntax checks and implementation whitespace checks passed.
- The focused Electron run passed four tests, including discovery recovery and the unrelated Codex conversation test. The narrow-window screenshot was inspected without overlap or horizontal overflow.
- Initial sandbox-only runs could not bind loopback mock servers or write the default Go cache. Successful runs used the permitted test-server access and a temporary Go cache. The remote launch's `SECTILE_*` settings were removed from test subprocess environments so mocks controlled their own connections. The outdated console mock capability was fixed rather than bypassing the old-agent protection.
- Discovery behavior is verified with test doubles, not live provider integration. No assignment surface outside the form, web form, tracker priority mapping or workflow behavior was changed.

- Full `npm --prefix desktop run test:ui` exited 0: `# tests 132`, `# pass 132`, `# fail 0`. The latest task-page tests include actual default tracker selection and type/priority-only dirty state.
- Publication reuses draft PR #828 on `feat/825`, with current `origin/main` merged without rewriting shared history. The workflow result is submitted only as implemented; readiness and review remain for adjustment.
