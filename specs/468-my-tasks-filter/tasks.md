# Tasks: My Tasks finds the tickets assigned to me

Ordered checklist. Each task names what proves it done. Requirement numbers refer to
`spec.md`, design to `plan.md`.

## 0. Before starting

- [x] `git fetch`, then merge `origin/main` into `feat/468` (no rebase: the branch is
      pushed). Migration 22 must still be the next free version; check nobody changed
      `GetTasksInScope` or the sidebar button.
- [x] Symlink `web/node_modules` from the main checkout if the worktree has none
      (remove it before committing). Not needed: this worktree has its own.

## 1. Tests first (red)

- [x] `internal/db/mytasks_test.go`: the resolution cases of `plan.md` (fail: `Mine`
      does not exist).
- [x] `internal/db/usercredentials_test.go`: account reset, set, sealed, clear.
- [x] `web/tests/myTasks.test.mjs`: tooltip helper (fails: module missing).

## 2. Migration and storage (FR 4, 5)

- [x] Migration 24 (22 and 23 were taken by #469 and #475) `user_tracker_credentials.account`, both engines; the rewind-test
      helpers drop the new column.
- [x] `UserCredential.Account`, reset in `SetUserTrackerCredential`,
      `SetUserTrackerCredentialAccount`, `TrackerAccounts`.
- [x] Check: `go test ./internal/db/` on SQLite; PostgreSQL with a throwaway DSN.

## 3. Learning the account (FR 4)

- [x] PUT `/api/me/tracker-credentials` probes after saving and records the account;
      a failed probe keeps the save.
- [x] `checkTrackerCredentials` records the account when it used the caller's own
      stored token on the stored site, and only then.
- [x] Handler tests with an `httptest` fake tracker, both paths and the negative ones.

## 4. Server filter (FR 1, 2, 3, 6)

- [x] `MyTasks`, `TaskScope.Mine`, the condition in `GetTasksInScope`.
- [x] `TaskSourcesInScope`.
- [x] `GET /api/tasks?mine=1`; `h.myTasks(r)` from `composedSettings` and
      `TrackerAccounts`.
- [x] `GET /api/me/assignee-identities`, registered in `cmd/server/main.go`.
- [x] Check: `mytasks_test.go` green; the fake tracker saw zero requests during
      `GET /api/tasks?mine=1`.

## 5. Web client (FR 7-13)

- [x] `AppContext`: `myTasksOnly` state, persistence as `mine`, restore, exclusivity
      with the person filter, `mine=1` in `buildTaskQuery`, identities fetch and
      refresh after credential changes.
- [x] `lib/myTasks.ts` and the French and English strings.
- [x] `Sidebar`: button state, toggle, tooltip; status shortcuts turn My Tasks off and
      are not active while it is on (FR 8).
- [x] `Header` and `RoadmapView` chips.
- [x] `board-views.browser.mjs` S8/S12 on `mine=1`, identities stub, legacy
      `assignee: 'Alice'` case, status shortcut dropping `mine=1`.
- [x] Check: `node --test web/tests/*.test.mjs`, `npx tsc --noEmit`, `npx oxlint`, the
      browser test (build first: `npx vite build`, then restore `webui/.gitkeep`).

## 6. Documentation

- [x] `docs/API_AND_DATA_SPEC.md`: `mine=1`, the identities route, `account`.
- [x] `CHANGELOG.md`: the `Fixed` line under `[Unreleased]` (FR 14).

## 7. Manual check

- [ ] On a copy of the dev database, server started without tracker tokens: a GitHub
      project with a personal credential verified in the profile, My Tasks on, only
      the login's tickets remain; hover shows no fallback; delete the credential,
      hover names GitHub.
