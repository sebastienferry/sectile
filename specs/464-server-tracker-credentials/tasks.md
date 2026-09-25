# Tasks #464 - One server tracker credential per provider

Ordered checklist. Each group is one commit (Conventional Commits). Before
group 2, check the next free migration number against `origin/main` (17 at the
time of writing) and renumber if main landed one.

## 1. Server binding in `internal/secrets` (FR-4)

- [x] T1.1 `Binding.Server bool`, `ServerBinding(tracker)`, `bytes()` with the
  `sectile:v1:server:tracker:` prefix; `Seal` accepts an empty user when
  `Server` is set.
- [x] T1.2 Tests: a server record opens with `ServerBinding` of its tracker
  only; it fails with `ServerBinding` of another tracker and with a personal
  binding of any user; an existing personal record (golden bytes from a
  fixture sealed before the change) still opens.

## 2. Schema (FR-9, US4)

- [x] T2.1 Migration 17 `server_tracker_credentials` + drop
  `projects.github_token`, `projects.gitlab_token` (one statement per entry).
  Never touch the baseline `CREATE TABLE`.
- [x] T2.2 Remove the project token fields from `models.Project`, the update
  request, the project `SELECT` / `INSERT` / `UPDATE` in `internal/db/db.go`,
  and `withoutProjectTokens`.
- [x] T2.3 Migration tests on SQLite and PostgreSQL: table present, project
  columns gone, a project row survives the migration.

## 3. Store: resolve, save, clear, states (FR-2, FR-5, US2, US7)

- [x] T3.1 `internal/db/servercredentials.go`: `serverTrackerCredential`,
  `SaveServerTrackerCredential`, `ClearServerTrackerCredential`,
  `ServerTrackerCredentialStates`, `ErrServerCredentialUnreadable`,
  `recordServerCredentialCheck(tracker, account)`.
- [x] T3.2 `trackerCredentials` fills tokens and Jira e-mail from the new store
  as a whole per provider; project overlay limited to URLs and GitLab project;
  unreadable credential surfaced so the call fails (plan §3).
- [x] T3.3 `withoutTrackerTokens` computes `…Set` / `…FromEnv` from the states;
  `jiraEmail` blanked for a member answer (handler side passes the role).
- [x] T3.4 `UpdateSettings` and `SaveTrackerCredentials` stop writing the token
  and Jira e-mail columns.
- [x] T3.5 Tests: database wins over environment; Jira row never mixed with
  env e-mail; clear falls back to env, then none; unreadable (record sealed
  with another key) fails and does not use env; a save on one `DB` handle is
  seen by a second handle on the same database (multi-replica stand-in).

## 4. Environment resolution in `trackerapi` (FR-3, FR-12, US5)

- [x] T4.1 `NewClient` reads only `SECTILE_GITHUB_TOKEN`,
  `SECTILE_GITLAB_TOKEN`, `SECTILE_JIRA_EMAIL`, `SECTILE_JIRA_TOKEN`; remove
  `trackerToken` and `genericTokenVar`; add `EnvCredential(tracker)` and
  `RemovedTokenVariables` (name → replacement).
- [x] T4.2 Rewrite `client_test.go` / `jira_test.go` cases that relied on
  `SECTILE_TRACKER_TOKEN` or `GH_TOKEN`: with only removed variables set,
  every provider resolves empty; each provider reads its own variable only.
- [x] T4.3 Startup warning in `cmd/server/main.go` from
  `RemovedTokenVariables`; test the formatting function (one line per set
  variable, none when unset). Fix the comment at `cmd/server/main.go:39`.

## 5. Sync without acting user (FR-1, FR-10, FR-11, US1)

- [x] T5.1 `SkillJob.RequestedBy`; `EnqueueSyncWith` fills it and leaves
  `ActingUser` empty for sync jobs; activity `UserID` unchanged.
- [x] T5.2 `processSyncJob` drops `tracker.WithActingUser`; any display use of
  `job.ActingUser` moves to `RequestedBy`.
- [x] T5.3 `autosync.go` enqueues with `""`; remove the owner comment.
- [x] T5.4 `missingServerCredential` and the split at the call sites
  (`client.go`, `github.go`, `jira_client.go` incl. the missing e-mail case);
  unreadable-credential message.
- [x] T5.5 Tests (`internal/db`, fake tracker HTTP server recording the
  `Authorization` header):
  - timer sync of a GitHub project whose owner has a personal token → server
    token sent;
  - manual sync by a user with a personal token → server token sent, activity
    `userId` is that user;
  - Jira sync with owner's credential sealed and locked → succeeds with the
    server pair;
  - no credential → error text contains the variable name and
    "Administration", not "Profile";
  - personal write (comment) by a user with a personal token → personal token
    sent; Jira write without personal token → refused (unchanged);
  - GitHub server credential is never sent to the Jira fake and conversely.

## 6. Upgrade adoption (FR-13, US6)

- [x] T6.1 `adoptLegacyServerTrackerTokens`, called after migrations in the
  open path; transaction per provider; `ON CONFLICT (tracker) DO NOTHING`.
- [x] T6.2 `internal/db/migrate.go`: copy `server_tracker_credentials`;
  `ensureKeyOpensCredentials` also tries one server record.
- [x] T6.3 Tests: clear-text GitHub and Jira (with e-mail) in `settings` →
  sealed rows, settings columns blank, account empty; second open does
  nothing; clear text + no key (`serverKeyErr` forced) → open fails naming
  `SECTILE_SECRET_KEY`, settings untouched; no clear text + no key → open
  succeeds; row present + clear text → column blanked, row unchanged; SQLite
  → PostgreSQL copy carries the table and refuses a wrong key.

## 7. Admin API (FR-6, FR-7, FR-8, US2, US3)

- [x] T7.1 `internal/handlers/servercredentials.go`: `GET`, `PUT`, `DELETE`,
  `POST …/check` under `/api/admin/tracker-credentials`; `requireAdmin` first;
  `CheckServerTrackerCredentials` with no acting user; Jira requires e-mail
  and token; unknown tracker 404. Register in `cmd/server/main.go`.
- [x] T7.2 `trackerSettingsKeys` without the four credential keys; settings
  update ignores them for everyone.
- [x] T7.3 `HandleTrackerSetup`: token or e-mail from a member → 403 with a
  message pointing to an admin; from an admin → check then
  `SaveServerTrackerCredential`; URL / project / repository still saved for a
  member.
- [x] T7.4 Tests: anonymous 401 and member 403 on each admin route; member
  settings PUT with `githubToken` stores nothing; member tracker setup with a
  token 403, without a token 200; admin PUT with a failing check stores
  nothing; admin PUT success stores and returns the account; no response body
  of any of these routes contains the token (grep the recorded body); `GET
  /api/settings` as member has flags and no `jiraEmail`.

## 8. Web (US2, US3, US4)

- [x] T8.1 `web/src/lib/serverCredentials.ts` (API calls, types,
  `serverCredentialLabel`) + `web/tests/serverCredentials.test.mjs` for the
  label (stored, stored unreadable, environment, none).
- [x] T8.2 `ServerTrackerCredentialsPanel.tsx` in `AdminView.tsx`; save enabled
  after a successful check; clear asks for confirmation; strings in
  `web/src/locales/translations.ts` (French and English).
- [x] T8.3 `ProjectModal.tsx`: GitHub token field and payload key removed;
  `types/index.ts` project token fields removed; `AppContext.tsx`
  `saveTrackerCredentials` removed; `trackers.ts` member wording
  ("configured by an admin").
- [x] T8.4 `npx tsc --noEmit`, oxlint and `npm test` in `web/` (symlink the
  main checkout's `node_modules` if the worktree has none, remove it after).

## 9. Documentation

- [x] T9.1 `docs/adrs/0028-tracker-sync-uses-a-server-credential-per-provider.md`
  (check the next free number on `origin/main`); "Superseded in part by ADR
  0028" line in ADR 0014.
- [x] T9.2 `README.md` (tracker connection section and env table),
  `.env.sample` (server tracker section rewritten, `SECTILE_TRACKER_TOKEN`
  line removed, `gh auth token` tip moved to `SECTILE_GITHUB_TOKEN`),
  `scripts/env-profiles.conf.sample`.
- [x] T9.3 `CHANGELOG.md` `[Unreleased]`: `Changed` (syncs use the provider's
  server credential, set by an admin, encrypted; Jira projects that relied on
  the owner's personal token need a server Jira credential) and `Removed`
  (the five variables, per-project GitHub/GitLab tokens), referencing #464.

## Implementation notes

Three deviations from the plan, acceptance criteria unchanged:

- T5.1: no `SkillJob.RequestedBy`. The activity row already records who asked
  (`task_activities.user_id`), so `EnqueueSyncWith` keeps writing it there and
  simply leaves the job's `ActingUser` empty; `processSyncJob` no longer puts
  one on the context either, so a job naming somebody still reads as nobody.
- T7.2: a member sending a credential key to `PUT /api/settings` is refused
  with 403 naming the keys (the existing guard for non-member keys), rather
  than having them ignored. An admin's is accepted and still stores nothing.
- T8.3: no member wording change in `web/src/lib/trackers.ts`. No member
  screen offers a server token field: the personal credential form already
  replaces the server flags with the member's own credential state.

Follow-up: drop the blank `settings.github_token`, `gitlab_token`,
`jira_api_token` and `jira_email` columns in a later release.

## Test plan

- `go test ./...`, including the PostgreSQL suite against a throwaway server
  (`SECTILE_TEST_POSTGRES_DSN` truncates what it names: never dev). Rerun the
  handlers keepalive test alone if it flakes under full-suite load.
- `node --test` in `web/`; `tsc` and oxlint.
- Manual, on a **copy** of the dev database, started **without** any real
  tracker token in the environment (a branch server writes to the real
  tracker otherwise), against a local fake or a throwaway repository:
  1. start with only `SECTILE_TRACKER_TOKEN` set → one warning in the log; a
     sync fails with the message naming `SECTILE_GITHUB_TOKEN`;
  2. as admin, Administration → GitHub: wrong token → refused, nothing saved;
     good token → saved, account shown; the sync now succeeds;
  3. as a member: the section is absent, project settings have no token field,
     `curl` on the admin routes answers 403;
  4. clear the stored GitHub credential with `SECTILE_GITHUB_TOKEN` set → state
     "provided by the environment";
  5. copy of a database holding a clear-text settings token → after start,
     the token is sealed and the settings column is empty
     (`sqlite3 … "select github_token from settings"`).
