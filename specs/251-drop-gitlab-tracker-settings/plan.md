# #251 — Plan

Spec: [`spec.md`](./spec.md). Stack: Go (`internal/`), SQLite and PostgreSQL
through `internal/db`, React + TypeScript web client (`web/`), tests with
`go test` and `node --test`.

## Decisions

- **D1 — Remove GitLab from the tracker client, not just hide it.**
  `trackerapi.Credentials` and `Client` lose `GitlabURL`, `GitlabProject`,
  `GitlabToken`; `DefaultGitlabURL`, the `SECTILE_GITLAB_*` / `GITLAB_TOKEN`
  reads, the `gitlab` branches of `For` / `ForActingUser` and `CheckGitlab` go.
  With nothing left to read these fields, a GitLab personal credential falls to
  the `default` branch of `ForActingUser` and is never used.
- **D2 — Inert columns.** The `gitlab_*` columns stay in the `CREATE TABLE`
  statements and in the additive `ALTER` loop of `applyLegacyMigrations`. The
  baseline is frozen (ADR 0021), and keeping them means fresh and upgraded
  databases stay identical on both engines (`schemaparity_test.go`). Only the
  `SELECT`, `Scan`, `INSERT` and `UPDATE` lists stop naming them. The settings
  upsert leaves them out of both the column list and the `ON CONFLICT` set, so
  existing values are neither read nor overwritten.
- **D3 — Migration 2 deletes the personal rows.**
  `{version: 2, name: "user_tracker_credentials.drop_gitlab", statements:
  {"DELETE FROM user_tracker_credentials WHERE tracker = 'gitlab';"}}`. The same
  SQL works on both engines, and it runs exactly once through the numbered
  scheme. The table exists at the baseline (`applyBaseline` calls
  `ensureUserCredentialsTable`). Stored tokens are lowercase, as
  `SetUserTrackerCredential` lowercases the tracker name.
- **D4 — Synthetic-migration tests follow the latest version.** In
  `migrations_test.go`, `TestAMigrationIsAppliedOnceAndRecorded` and
  `TestAFailedMigrationLeavesThePreviousVersion` number their fake migration
  `baselineVersion + 1` and assume the database is at the baseline. With a real
  migration 2 in the list, a new database is already at 2, so they must use
  `latestVersion() + 1` / `latestVersion()`. The "exactly the baseline and one
  migration" count also becomes `len(migrations) + 2`.
- **D5 — The handler refuses explicitly.** `HandleTrackerSetup` gets one
  `supported` check (`github`, `jira`). Any other value returns `400
  "tracker %q non pris en charge"` before any check or save, whether or not the
  request is check-only. Today an unknown name falls through to "return the
  settings". The spec only requires the refusal for `gitlab`, but refusing
  every unknown name is the same line of code and avoids a silent success.
  `SaveTrackerCredentials` and `checkTrackerCredentials` already reject unknown
  names in their `default` branch; they just lose the `gitlab` case.
- **D6 — Models lose the fields.** The Go JSON decoder ignores unknown fields,
  so older clients that still send `gitlab*` keep working and their values are
  dropped (spec US2). The GitLab keys leave `trackerSettingsKeys`. A plain
  removal breaks one path, though: `memberSettingsViolations` compares each key
  a member sends with the stored row. A browser tab opened before the upgrade
  still holds `gitlabUrl: ""` / `gitlabTokenSet: false`, which no longer match
  the absent stored value, so the member's save would answer `403` on a key
  that no longer exists. A small `retiredSettingsKeys` set (the five GitLab
  keys) is therefore skipped by the violation check, and `settingsOverlay`
  never merges it either. The model no longer has the field, so decoding drops
  it anyway.
- **D7 — Web: one list.** `TRACKER_CONFIGS` = Jira, GitHub. `hasAdapter`,
  `personalTrackers()` and `PERSONAL_TRACKERS` go. `TrackerCredentialsTab` uses
  `getTrackers(t)`, like `TrackerSetup`. `storedFor` loses its GitLab case,
  `TrackerCredentials['tracker']` becomes `Exclude<IssueTracker, 'local'>`, and the fr/en `trackers.gitlab` entries go.
- **D8 — Small leftovers.** `trackerDisplayName` loses its `gitlab` case. No
  adapter reports that name, and the generic capitalisation covers anything
  else. The project target in the activity log (`firstNonEmpty(...,
  proj.GitlabProject)`) loses its GitLab argument. Comments naming GitLab as a
  tracker (`jira_client.go`, `handlers.go`, `client.go`, `trackers.ts`,
  `types/index.ts`) are rewritten in English (AGENTS.md). The generic
  `tracker/ticketing.go` interface comment stays: it lists examples of
  trackers, not a promise.

## Rejected alternatives

- **Dropping the columns** — the owner rejected it (Q2). It would be
  destructive DDL on two engines, and it would contradict the frozen baseline.
- **Keeping `hasAdapter` for a future adapter** — dead flexibility. Any future
  adapter adds its entry back.
- **Deleting GitLab personal rows in `ensureUserCredentialsTable`** — that code
  is part of the frozen baseline. Numbered migrations exist to replace
  `IF NOT EXISTS` hedges like this one.

## Target files

| File | Change |
| --- | --- |
| `internal/trackerapi/client.go` | D1 |
| `internal/trackerapi/jira_client.go` | comment (D8) |
| `internal/models/models.go` | D6 |
| `internal/db/db.go` | D2, D8 |
| `internal/db/trackercredentials.go` | D1/D2: resolution, redaction, check, save |
| `internal/db/migrations.go` | D3 |
| `internal/db/migrations_test.go` | D4, and a test for migration 2 |
| `internal/db/trackercredentials_test.go` | remove GitLab cases; assert refusal |
| `internal/handlers/handlers.go` | D5 |
| `internal/handlers/authz.go` | D6 |
| handler test (existing setup-tracker test file, or new) | 400 on `gitlab` |
| `web/src/lib/trackers.ts` | D7 |
| `web/src/components/TrackerCredentialsTab.tsx` | D7 |
| `web/src/types/index.ts` | D7 |
| `web/src/locales/translations.ts` | D7 |
| `web/tests/trackerSetup.test.mjs` | D7 |
| `README.md`, `docs/API_AND_DATA_SPEC.md` | FR9 |
| `CHANGELOG.md` | `Removed` entry under `[Unreleased]` |

## Data contracts

- `GET/PUT /api/settings`: `gitlabUrl`, `gitlabProject`, `gitlabToken`,
  `gitlabTokenSet`, `gitlabTokenFromEnv` disappear from responses and are
  ignored on input.
- Project read/create/update: `gitlabUrl`, `gitlabProject`, `gitlabToken`,
  `gitlabTokenSet` likewise.
- `POST /api/setup/tracker[/check]`: `tracker` ∈ {`github`, `jira`}; anything
  else → `400`.
