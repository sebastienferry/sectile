# Tasks: My Tasks finds the tickets assigned to me

Ordered checklist. Each task names what proves it done. Requirement numbers refer to
`spec.md`, design to `plan.md`.

## 0. Before starting

- [ ] `git fetch`, then merge `origin/main` into `feat/468` (no rebase: the branch is
      pushed). Migration 22 must still be the next free version; check nobody changed
      `GetTasksInScope` or the sidebar button.
- [ ] Symlink `web/node_modules` from the main checkout if the worktree has none
      (remove it before committing).

## 1. Tests first (red)

- [ ] `internal/db/mytasks_test.go`: the resolution cases of `plan.md` (fail: `Mine`
      does not exist).
- [ ] `internal/db/usercredentials_test.go`: account reset, set, sealed, clear.
- [ ] `web/tests/myTasks.test.mjs`: tooltip helper (fails: module missing).

## 2. Migration and storage (FR 4, 5)

- [ ] Migration 22 `user_tracker_credentials.account`, both engines; the rewind-test
      helpers drop the new column.
- [ ] `UserCredential.Account`, reset in `SetUserTrackerCredential`,
      `SetUserTrackerCredentialAccount`, `TrackerAccounts`.
- [ ] Check: `go test ./internal/db/` on SQLite; PostgreSQL with a throwaway DSN.

## 3. Learning the account (FR 4)

- [ ] PUT `/api/me/tracker-credentials` probes after saving and records the account;
      a failed probe keeps the save.
- [ ] `checkTrackerCredentials` records the account when it used the caller's own
      stored token on the stored site, and only then.
- [ ] Handler tests with an `httptest` fake tracker, both paths and the negative ones.

## 4. Server filter (FR 1, 2, 3, 6)

- [ ] `MyTasks`, `TaskScope.Mine`, the condition in `GetTasksInScope`.
- [ ] `TaskSourcesInScope`.
- [ ] `GET /api/tasks?mine=1`; `h.myTasks(r)` from `composedSettings` and
      `TrackerAccounts`.
- [ ] `GET /api/me/assignee-identities`, registered in `cmd/server/main.go`.
- [ ] Check: `mytasks_test.go` green; the fake tracker saw zero requests during
      `GET /api/tasks?mine=1`.

## 5. Web client (FR 7-13)

- [ ] `AppContext`: `myTasksOnly` state, persistence as `mine`, restore, exclusivity
      with the person filter, `mine=1` in `buildTaskQuery`, identities fetch and
      refresh after credential changes.
- [ ] `lib/myTasks.ts` and the French and English strings.
- [ ] `Sidebar`: button state, toggle, tooltip; status shortcuts turn My Tasks off and
      are not active while it is on (FR 8).
- [ ] `Header` and `RoadmapView` chips.
- [ ] `board-views.browser.mjs` S8/S12 on `mine=1`, identities stub, legacy
      `assignee: 'Alice'` case, status shortcut dropping `mine=1`.
- [ ] Check: `node --test web/tests/*.test.mjs`, `npx tsc --noEmit`, `npx oxlint`, the
      browser test (build first: `npx vite build`, then restore `webui/.gitkeep`).

## 6. Documentation

- [ ] `docs/API_AND_DATA_SPEC.md`: `mine=1`, the identities route, `account`.
- [ ] `CHANGELOG.md`: the `Fixed` line under `[Unreleased]` (FR 14).

## 7. Manual check

- [ ] On a copy of the dev database, server started without tracker tokens: a GitHub
      project with a personal credential verified in the profile, My Tasks on, only
      the login's tickets remain; hover shows no fallback; delete the credential,
      hover names GitHub.
