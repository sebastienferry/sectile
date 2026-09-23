# #251 — Drop the GitLab tracker settings

Ticket: https://github.com/sebastienferry/sectile/issues/251.
Clarification: [`docs/clarifications/251.md`](../../docs/clarifications/251.md)
(two rounds, owner confirmed on 2026-09-23).

## Problem

GitLab can be configured as an issue tracker: settings, project overrides, a
personal credential, three environment variables and a connection check. But
there is no GitLab adapter in the tracker registry, so no project can use GitLab
Issues and a GitLab token is never used. The first-start setup dialog still
offers GitLab, checks the token and saves it. Nothing ever reads it back.

The owner chose the removal outcome: nothing in Sectile should imply a GitLab
tracker exists.

## Scope

In scope: every **tracker** use of GitLab — configuration, API fields,
environment variables, client fields, the connection check, the setup dialog,
personal credentials, translations and documentation.

Out of scope (unchanged): GitLab as a **forge** — `glab` merge-request
discovery and evidence (`internal/runner`), the "GitLab MR" badges on task
cards, `prUrl` handling, the GitLab CI pipeline and mirror documentation. Also
out of scope: a GitLab Issues adapter (a future ticket), dropping the inert
columns, and restricting `SetUserTrackerCredential` to registered trackers in
general.

## User stories

### US1 (P1) — The setup dialog offers only trackers that exist

As somebody setting up Sectile, I am offered Jira and GitHub only, so I never
configure a tracker that does nothing.

- **Given** the first-start tracker setup dialog, **when** it opens, **then** it
  lists Jira and GitHub and no GitLab.
- **Given** Profile > Trackers, **when** it opens, **then** it lists the same
  trackers as the setup dialog.
- **Given** the project tracker list, **then** every non-local tracker in it is
  one the setup dialog and Profile > Trackers offer.

### US2 (P1) — The API refuses GitLab as a tracker

As an API client, I get a clear refusal instead of a silent save when I send
GitLab tracker parameters.

- **Given** `POST /api/setup/tracker/check` or `POST /api/setup/tracker` with
  `tracker: "gitlab"`, **then** the answer is `400` naming the unsupported
  tracker and nothing is stored.
- **Given** a settings or project update carrying `gitlabUrl`,
  `gitlabProject` or `gitlabToken`, **then** those fields are ignored: nothing
  is written.
- **Given** a settings or project read, **then** the response contains no
  `gitlab*` field.

### US3 (P1) — No GitLab token lingers or is used

As an operator, I know that no GitLab token is still used by the server.

- **Given** a server started with `SECTILE_GITLAB_TOKEN`, `GITLAB_TOKEN`,
  `SECTILE_GITLAB_API_URL` or `SECTILE_GITLAB_PROJECT` set, **then** none of them
  is read.
- **Given** a database holding personal credentials with `tracker = 'gitlab'`,
  **when** the upgraded server starts, **then** those rows are deleted once and
  the credentials of other trackers are untouched.
- **Given** a fresh database and an upgraded one, on SQLite and PostgreSQL,
  **then** both have the same schema (the `gitlab_*` columns stay on `settings`
  and `projects`, unused).

### US4 (P2) — The documentation matches

- **Given** the README and `docs/API_AND_DATA_SPEC.md`, **then** they no longer
  document GitLab tracker settings or variables; the `gitlab_*` columns are
  documented as unused legacy columns; the GitLab CI / mirror sections are
  unchanged.

### US5 (P1) — The forge side is unaffected

- **Given** a repository on GitLab, **then** MR discovery through `glab`, MR
  evidence, the "GitLab MR" badge and `prUrl` behave as before.

## Functional requirements

- **FR1** The tracker client has no GitLab URL, project or token, reads no
  `SECTILE_GITLAB_*` / `GITLAB_TOKEN` variable, has no GitLab connection check,
  and has no GitLab branch in personal credential substitution.
- **FR2** Settings, project, project update, project creation and tracker
  settings models have no GitLab fields; the store neither reads nor writes the
  `gitlab_url`, `gitlab_project`, `gitlab_token` columns.
- **FR3** The columns stay in the baseline schema and its additive migration,
  unchanged.
- **FR4** A numbered schema migration deletes `user_tracker_credentials` rows
  whose `tracker` is `gitlab`, on both engines, exactly once.
- **FR5** The tracker check and save refuse `gitlab` with `400` (unsupported
  tracker), without storing anything.
- **FR6** The admin-only settings allow-list has no GitLab keys.
- **FR7** The web tracker list (`TRACKER_CONFIGS`) is Jira and GitHub; the
  setup dialog, Profile > Trackers and the project tracker checks share it;
  the adapter-availability flag and the separate personal list are removed.
- **FR8** Web types and fr/en translations have no GitLab tracker fields.
- **FR9** README and `docs/API_AND_DATA_SPEC.md` are updated as in US4.

## Non-functional requirements

- **NFR1** `go build ./...`, `go vet ./...`, `go test ./...`, and in `web/`
  `npm run lint`, `npm test`, `npm run build` pass.
- **NFR2** No destructive DDL: no `DROP COLUMN`.

## Open requirements

None. The clarification left no product question open.
