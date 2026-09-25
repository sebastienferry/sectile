# Plan #464 - One server tracker credential per provider

Behaviour: `spec.md`. This file says how.

## Stack

Go server (`internal/db`, `internal/trackerapi`, `internal/secrets`,
`internal/handlers`, `cmd/server`), SQLite and PostgreSQL through the numbered
migrations of ADR 0021, React web client (`web/src`). The desktop app has no
tracker token screen and needs no change.

## Current state (what the change starts from)

- Server tokens live in clear text in the shared `settings` row
  (`github_token`, `gitlab_token`, `jira_email`, `jira_api_token`), overridden
  by `projects.github_token` / `projects.gitlab_token`, and completed field by
  field from the environment in `trackerapi.NewClient` (`trackerToken`, with
  `SECTILE_TRACKER_TOKEN` and the provider conventions as fallbacks).
- `trackerCredentials` (`internal/db/trackercredentials.go`) merges settings
  and project overrides; `Client.For` overlays them on the environment values.
- Members may write the token keys (`trackerSettingsKeys`,
  `internal/handlers/authz.go`) and `POST /api/setup/tracker` has no admin
  check.
- The autosync enqueues with `proj.OwnerUserID` (`internal/db/autosync.go`),
  a manual sync with the clicking user (`EnqueueSyncAs`, `handlers.go:473,497,524`),
  and `processSyncJob` puts `job.ActingUser` into the tracker context, which
  makes `ForActingUser` substitute a personal token.

## Architecture

### 1. Storage: `server_tracker_credentials`

Migration 17 (`internal/db/migrations.go`), shared statements:

```sql
CREATE TABLE server_tracker_credentials (
    tracker     TEXT PRIMARY KEY,          -- 'github' | 'jira' | 'gitlab'
    email       TEXT NOT NULL DEFAULT '',  -- Jira only
    record      BLOB NOT NULL,             -- secrets.Seal output
    account     TEXT NOT NULL DEFAULT '',  -- account the last successful check named
    checked_at  DATETIME,
    updated_at  DATETIME NOT NULL,
    updated_by  TEXT NOT NULL DEFAULT ''   -- admin user id
);
ALTER TABLE projects DROP COLUMN github_token;
ALTER TABLE projects DROP COLUMN gitlab_token;
```

`BLOB` and `DATETIME` go through `RewriteDDL` as the other migrations do. One
statement per entry (pgx refuses multi-statement `Exec`).

The `settings` columns `github_token`, `gitlab_token`, `jira_api_token` stay in
the schema for this release: they are the input of the one-off adoption (§5)
and are blanked by it. No code reads them after that step. Dropping them is a
follow-up migration in a later release (see *Follow-ups*). `jira_email` stays:
it is blanked with the token by the adoption, same reason.

### 2. Encryption: a server binding

`internal/secrets`:

- Extend `Binding` with `Server bool`. `bytes()` returns
  `sectile:v1:server:tracker:<len>:<name>` when `Server` is set, and the
  current `sectile:v1:user:…` string otherwise, so every existing personal
  record keeps opening and a server record can never open as a personal one
  (or the reverse).
- `Seal` accepts an empty `UserID` when `Server` is set; it still requires the
  tracker.
- Add `ServerBinding(tracker string) Binding` for readability.

The key is the existing server key (`DB.serverKey`, `SECTILE_SECRET_KEY` or
`secret.key`). No passphrase path exists for server credentials.

### 3. Resolution

New file `internal/db/servercredentials.go`:

```go
type ServerCredentialSource string // "database" | "environment" | "none"

type ServerCredentialState struct {
    Tracker   string                 `json:"tracker"`
    Source    ServerCredentialSource `json:"source"`
    Unreadable bool                  `json:"unreadable,omitempty"` // stored but the key does not open it
    Email     string                 `json:"email,omitempty"`      // Jira; admin view only
    Account   string                 `json:"account,omitempty"`    // admin view only
    CheckedAt *time.Time             `json:"checkedAt,omitempty"`
    UpdatedAt *time.Time             `json:"updatedAt,omitempty"`
}

func (d *DB) serverTrackerCredential(tracker string) (email, token string, err error)
func (d *DB) ServerTrackerCredentialStates() ([]ServerCredentialState, error)
func (d *DB) SaveServerTrackerCredential(tracker, email, token, account, adminID string) error
func (d *DB) ClearServerTrackerCredential(tracker string) error
```

`serverTrackerCredential`: row present → `secrets.Open` with
`ServerBinding(tracker)`; an open failure (or `serverKeyErr`) returns
`ErrServerCredentialUnreadable`, never the environment value (spec US7). No
row → environment pair from `trackerapi.EnvCredential(tracker)`. Neither →
empty, no error. It reads without taking `d.mu`, like `trackerCredentials`,
because it is called under paths that already hold it.

`trackerapi.Client`:

- `NewClient` reads only `SECTILE_GITHUB_TOKEN`, `SECTILE_GITLAB_TOKEN`,
  `SECTILE_JIRA_EMAIL`, `SECTILE_JIRA_TOKEN`. `trackerToken` and
  `genericTokenVar` go. The fields keep holding the environment pair, now used
  only through `EnvCredential`.
- `Credentials` loses nothing, but `DB.trackerCredentials` now fills the three
  tokens and the Jira e-mail from `serverTrackerCredential`, as a whole per
  provider (FR-2): when the database holds a Jira row, both e-mail and token
  come from it. The project overlay keeps URLs and the GitLab project only.
- `Client.For` keeps overlaying non-empty values. An unreadable stored
  credential is surfaced by a new `Credentials.Err map[string]error` (or an
  equivalent per-provider error field) so the call fails with that error
  instead of silently using the environment token held by the client. The
  implementer picks the smallest shape that keeps `For` pure; the test in T5
  is the contract.

### 4. Sync uses no acting user

- `autosync.go`: `EnqueueSyncWith("", …)`; the owner comment goes.
- `SkillJob` gains `RequestedBy string`. `EnqueueSyncWith` records the user id
  there (the activity keeps `UserID` for display, spec US1 "still records that
  they asked"), and leaves `ActingUser` empty for sync jobs.
- `processSyncJob` no longer calls `tracker.WithActingUser`. It keeps
  whatever else reads `job.ActingUser` for display, switched to `RequestedBy`.
- Personal writes (`processTrackerUpdateJob`, `trackerops.go`, `comments.go`,
  `prstates.go` for a named user) are unchanged (FR-10).

### 5. Adoption of the clear-text tokens (upgrade)

`adoptLegacyServerTrackerTokens` in `servercredentials.go`, called once from
the open path right after the migrations have run (next to where the schema
version is checked in `db.go`). For each provider:

1. read the clear-text value from `settings` (`github_token`, `gitlab_token`,
   `jira_api_token` + `jira_email`);
2. empty → nothing to do;
3. a row already exists in `server_tracker_credentials` → blank the settings
   column only (an earlier run inserted it and was interrupted before
   blanking);
4. otherwise, `serverKeyErr != nil` → return an error naming
   `secrets.KeyEnvVar`; the open fails and the server refuses to start
   (spec US6). Nothing is written;
5. otherwise seal, insert (`account` empty, `updated_by` = `'upgrade'`), then
   blank the column, in one transaction.

Idempotent by construction (steps 2 and 3), so it needs no version number and
is safe under several replicas starting together: the insert uses
`ON CONFLICT (tracker) DO NOTHING`, and step 3 covers the loser.

The project token columns are dropped by migration 17 without adoption: the
owner chose to discard them (clarification Q2).

### 6. HTTP API

All in a new `internal/handlers/servercredentials.go`, registered in
`cmd/server/main.go` next to `AdminStatsPath`. Every handler starts with
`requireAdmin` (401 anonymous, 403 member).

| Method and path | Body | Answer |
| --- | --- | --- |
| `GET /api/admin/tracker-credentials` | (none) | `200 [ServerCredentialState…]` for github, jira, gitlab |
| `POST /api/admin/tracker-credentials/{tracker}/check` | `{email?, token?}` | `200 {ok, account}`; empty token checks the resolved credential; `400 {error}` on refusal |
| `PUT /api/admin/tracker-credentials/{tracker}` | `{email?, token}` | checks first (`CheckTrackerCredentials` with the server URL), then seals and stores; `200 ServerCredentialState`; `400` on failed check, nothing stored |
| `DELETE /api/admin/tracker-credentials/{tracker}` | (none) | `200 ServerCredentialState` (now environment or none) |

- The check of a server credential must not fall back to the admin's own
  personal token, which `checkTrackerCredentials` does today for GitHub and
  Jira: add a `CheckServerTrackerCredentials` that passes no acting user.
- A successful check of a stored credential updates `account` and
  `checked_at`.
- For Jira, `PUT` requires both `email` and `token`; a missing one is `400`.
- Unknown tracker → `404`.

Existing endpoints:

- `trackerSettingsKeys`: remove `githubToken`, `gitlabToken`, `jiraApiToken`,
  `jiraEmail`. The settings update path ignores these four keys for everyone,
  admin included (the admin endpoint is the one write path). `UpdateSettings`
  and `SaveTrackerCredentials` stop writing the token columns.
- `HandleTrackerSetup`: a request carrying `token` or `email` for GitHub,
  GitLab or Jira → `403` for a member, and for an admin it is delegated to
  `SaveServerTrackerCredential` after the check, so older clients keep
  working. URL, project and repository still save for a member.
- `GET /api/settings`: `withoutTrackerTokens` computes the `…TokenSet` /
  `…TokenFromEnv` flags from `ServerTrackerCredentialStates` (member-safe
  projection, FR-6). `jiraEmail` is blanked in the answer to a member.
- Project payloads: `models.Project.GithubToken`, `GitlabToken`,
  `GithubTokenSet`, `GitlabTokenSet` and the update request pointers go;
  `withoutProjectTokens` goes.

### 7. Error messages

`trackerapi`:

- `missingCredential(tracker)` stays for personal writes.
- `missingServerCredential(tracker)` for a call with no acting user. The
  `trackerapi` messages are in English today (`missingCredential`,
  `jiraConfigured`, the Jira refusal); keep that language, as translating them
  is a decision of its own (AGENTS.md). For example
  `no GitHub server credential: set SECTILE_GITHUB_TOKEN or save one in Administration`
  (Jira names `SECTILE_JIRA_EMAIL` and `SECTILE_JIRA_TOKEN`).
- `jiraConfigured` gets the same split for its missing e-mail case, which is
  part of the Jira server credential.
- The call sites (`client.go` x3, `github.go`, `jira_client.go`) choose by
  whether the client was resolved for an acting user. Where no context
  reaches the check, carry the flag on the resolved `Client` rather than
  threading a context through.
- Unreadable stored credential, English as well:
  `the stored Jira server credential cannot be decrypted with the server key: save it again in Administration`.

### 8. Startup warning

`cmd/server/main.go`, after env loading: for each removed variable that is set,
`log.Printf("⚠️  %s n'est plus lu comme accès tracker : utilisez %s", name, replacement)`
with `SECTILE_TRACKER_TOKEN → SECTILE_GITHUB_TOKEN, SECTILE_JIRA_TOKEN ou SECTILE_GITLAB_TOKEN`,
`GH_TOKEN`/`GITHUB_TOKEN → SECTILE_GITHUB_TOKEN`, `JIRA_API_TOKEN → SECTILE_JIRA_TOKEN`,
`GITLAB_TOKEN → SECTILE_GITLAB_TOKEN`. Put the table in `trackerapi` as
`RemovedTokenVariables` so the test and the log share it. The variables are not
unset: child processes (`internal/runner`) keep inheriting them.

### 9. SQLite → PostgreSQL copy

`internal/db/migrate.go` copies tables explicitly: add
`server_tracker_credentials`, and extend `ensureKeyOpensCredentials` to try one
server record with `ServerBinding` when the table is not empty.

### 10. Web

- `web/src/lib/serverCredentials.ts`: types of `ServerCredentialState`,
  `fetchServerCredentials`, `saveServerCredential`, `checkServerCredential`,
  `clearServerCredential`, and a pure `serverCredentialLabel(state)` for the
  state line (tested).
- `web/src/components/ServerTrackerCredentialsPanel.tsx`: one card per
  provider in `AdminView.tsx`, after the users section. State line, account
  and dates, token input (and e-mail for Jira), *Vérifier*, *Enregistrer*
  (enabled after a successful check, same rule as `TrackerCredentialForm`),
  *Effacer* with confirmation. Never displays a token.
- `web/src/components/ProjectModal.tsx`: remove the GitHub token field and
  `githubToken` from the payload.
- `web/src/context/AppContext.tsx`: remove `saveTrackerCredentials` (no
  caller); keep `checkTrackerCredentials` (personal flow).
- `web/src/types/index.ts`: drop the project token fields.
- `web/src/locales/translations.ts`: French and English strings for the panel
  (the interface stays in the language it speaks).
- `web/src/lib/trackers.ts`: the `tokenIsSet` / `tokenFromEnv` projection keeps
  working from the settings flags; wording for a member becomes "configured by
  an admin" rather than an invitation to type a server token.

### 11. Documentation

- `docs/adrs/0028-tracker-sync-uses-a-server-credential-per-provider.md`:
  context (owner borrowing, cross-provider leak, locked tokens), decision
  (this plan's rules), consequences (breaking env change, one instance per
  provider, attribution to the service account, key loss means re-entry),
  supersedes the unattended-work paragraph of ADR 0014; ADR 0014 gets a
  "Superseded in part by ADR 0028" line.
- `README.md` §Tracker connection parameters and the env table; `.env.sample`
  server tracker section; `scripts/env-profiles.conf.sample`; the comment in
  `cmd/server/main.go:39`.
- `CHANGELOG.md` `[Unreleased]`: `Changed` and `Removed` lines of the spec.

## Data contracts

`ServerCredentialState` JSON (admin):

```json
{"tracker":"jira","source":"database","email":"sync@acme.com",
 "account":"Sectile Sync","checkedAt":"2026-09-25T08:00:00Z","updatedAt":"2026-09-25T08:00:00Z"}
```

`{"tracker":"github","source":"environment"}`,
`{"tracker":"gitlab","source":"none"}`,
`{"tracker":"github","source":"database","unreadable":true,"updatedAt":"…"}`.

Settings flags for everybody: `githubTokenSet`, `githubTokenFromEnv`,
`gitlabTokenSet`, `gitlabTokenFromEnv`, `jiraApiTokenSet`, `jiraApiTokenFromEnv`
(`Set` = stored, `FromEnv` = environment and nothing stored).

## Target files

| File | Change |
| --- | --- |
| `internal/secrets/secrets.go` (+ test) | server binding |
| `internal/db/migrations.go` (+ test) | migration 17 |
| `internal/db/servercredentials.go` (new, + test) | resolution, save, clear, states, adoption |
| `internal/db/trackercredentials.go` | resolution from the new store, no project tokens, flags |
| `internal/db/db.go` | adoption call, sync job `RequestedBy`, `processSyncJob`, settings SQL without token writes, project SQL without token columns |
| `internal/db/autosync.go` | no owner |
| `internal/db/migrate.go` | copy the table, key check |
| `internal/models/models.go` | project token fields out |
| `internal/trackerapi/client.go`, `jira_client.go`, `github.go` (+ tests) | env resolution, messages, removed-variable table |
| `internal/handlers/servercredentials.go` (new, + test) | admin endpoints |
| `internal/handlers/authz.go`, `handlers.go` | member keys, tracker setup |
| `cmd/server/main.go` | routes, startup warning |
| `web/src/…` (see §10) | panel, project modal, types, strings |
| `README.md`, `.env.sample`, `scripts/env-profiles.conf.sample`, `CHANGELOG.md`, `docs/adrs/0028-…`, `docs/adrs/0014-…` | docs |

## Rejected alternatives

- **Seal in place in the `settings` row.** Keeps one table fewer, but the row
  is written by the general settings path, which members reach; a separate
  table gives the admin-only rule one place to live and a natural key per
  provider.
- **Adoption as a Go step inside the migration runner.** The runner is
  SQL-only by design (ADR 0021); adding a Go hook for a one-off is a bigger
  change than an idempotent startup step.
- **Dropping the clear-text columns in migration 17.** SQL migrations run
  before any Go code, so the values would be gone before they could be sealed.
- **Discarding clear-text tokens when no key is available.** Silent loss of
  the only working credential; refusing to start with a named fix is
  recoverable. Only PostgreSQL without `SECTILE_SECRET_KEY` can hit it, since
  SQLite generates its key file.
- **Falling back to the environment when a stored credential is unreadable.**
  The admin believes the stored one is in use; a silent switch to another
  account would misattribute writes.
- **Caching the decrypted credential in memory.** Breaks FR-5 across replicas;
  a row read plus one AES-GCM open per call is what the settings row already
  costs.

## Follow-ups (not in this change)

- Drop `settings.github_token`, `settings.gitlab_token`,
  `settings.jira_api_token` and `settings.jira_email` in a later release's
  migration, once every deployment has run the adoption.

## Risks

- **Breaking environment change** for deployments on `SECTILE_TRACKER_TOKEN`:
  mitigated by the startup warning and the explicit sync error. Pre-1.0, MINOR
  bump at release.
- **Jira projects with personal-only access** (no server Jira credential)
  stop syncing on the timer, where they used to borrow the owner's token. The
  error says what to set; the CHANGELOG line must say it too.
- **Tests that set `SECTILE_TRACKER_TOKEN` or `GH_TOKEN`**
  (`internal/trackerapi/client_test.go`, `jira_test.go`,
  `internal/db/headless_test.go`, `trackercredentials_test.go`) must be
  rewritten, not deleted: they carry the provider-isolation contract.
