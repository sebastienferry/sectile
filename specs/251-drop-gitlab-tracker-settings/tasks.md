# #251 — Implementation checklist

Ordered so every step leaves the tree building: consumers stop using a field
before the field is removed. References: [`spec.md`](./spec.md),
[`plan.md`](./plan.md).

## 1. Server: tracker client and store (D1, D2, D8, FR1–FR3)

- [x] T1.1 `internal/db/trackercredentials.go`: drop GitLab from resolution,
      redaction (`withoutTrackerTokens`, `withoutProjectTokens`), the check and
      the save.
- [x] T1.2 `internal/db/db.go`: drop the `gitlab_*` columns from the settings and
      project `SELECT`/`Scan`, the settings upsert (column list, values,
      `ON CONFLICT`), project `INSERT`/`UPDATE` and `UpdateProject`; drop
      `trackerDisplayName("gitlab")` and `proj.GitlabProject` in the log target.
      Keep the `CREATE TABLE` columns and the `ALTER` loop.
- [x] T1.3 `internal/trackerapi/client.go`: remove `DefaultGitlabURL`, the
      GitLab fields, the env reads, the `For` / `ForActingUser` branches and
      `CheckGitlab`; `jira_client.go` comment.
- [x] T1.4 `internal/models/models.go`: remove the GitLab fields from
      `Settings`, `Project`, `CreateProjectRequest`, `ProjectUpdate`,
      `TrackerSettings`.

## 2. Server: migration (D3, D4, FR4)

- [x] T2.1 `internal/db/migrations.go`: migration 2
      `user_tracker_credentials.drop_gitlab`; update the "list is empty" comment.
- [x] T2.2 `internal/db/migrations_test.go`: synthetic migrations use
      `latestVersion()+1`, counts follow `len(migrations)`.
- [x] T2.3 Test: a database with `gitlab` and `github` personal rows at the
      baseline, migrated, keeps `github` and loses `gitlab`.

## 3. Server: API (D5, D6, FR5, FR6)

- [x] T3.1 `HandleTrackerSetup`: `400` for any tracker other than `github` /
      `jira`, check or save; comment updated.
- [x] T3.2 `authz.go`: GitLab keys out of `trackerSettingsKeys`; add
      `retiredSettingsKeys` skipped by `memberSettingsViolations` and never
      merged by `settingsOverlay`.
- [x] T3.3 Tests: setup check/save with `gitlab` → `400`, settings unchanged;
      a member posting a stale row carrying `gitlab*` keys is not refused.
- [x] T3.4 `trackercredentials_test.go`: drop GitLab expectations, replace
      `TestStoredGitlabParametersDoNotRegisterATracker` with checks that
      `CheckTrackerCredentials` / `SaveTrackerCredentials` refuse `gitlab`.

## 4. Web (D7, FR7, FR8)

- [x] T4.1 `web/src/lib/trackers.ts`: GitLab out of `TRACKER_CONFIGS`; remove
      `hasAdapter`, `personalTrackers`, `PERSONAL_TRACKERS`; `storedFor` and
      `StoredSettings` lose GitLab; comments in English where touched.
- [x] T4.2 `TrackerCredentialsTab.tsx`: use `getTrackers(t)`.
- [x] T4.3 `web/src/types/index.ts`: GitLab fields out of `Project` and
      `UserSettings`; `TrackerCredentials['tracker']` =
      `Exclude<IssueTracker, 'local'>`; comments.
- [x] T4.4 `translations.ts`: remove the `gitlab` schema entry and both fr/en
      values.
- [x] T4.5 `web/tests/trackerSetup.test.mjs`: trackers are `['jira','github']`,
      no GitLab; every project tracker is offered by the shared list.

## 5. Documentation (FR9)

- [x] T5.1 README: drop the `SECTILE_GITLAB_*` lines, the GitLab mentions in the
      setup text, the "no GitLab adapter" paragraph; keep the CI / mirror
      sections.
- [x] T5.2 `docs/API_AND_DATA_SPEC.md`: mark the `gitlab_*` columns unused.
- [x] T5.3 `CHANGELOG.md` `[Unreleased]` → `Removed`: GitLab tracker settings.

## 6. Verify (NFR1, US5)

- [x] T6.1 `grep -rni gitlab internal web/src` shows forge uses only (runner,
      MR badges, MR URL tests) plus the inert DDL.
- [x] T6.2 `go build ./...`, `go vet ./...`, `go test ./...` (WSL, per the
      Windows caveat), `cd web && npm run lint && npm test && npm run build`.
