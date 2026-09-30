# Tasks #654 - Connect Jira through Atlassian OAuth instead of a pasted API token

Ordered checklist. Each group is one commit and leaves the tree buildable and
the tests green. References are to `spec.md` (FR, US, AC) and `plan.md`
(sections).

## 1. Atlassian OAuth client (plan 1)

- [ ] T1.1 `internal/atlassian/oauth.go`: `App`, `Endpoints`,
  `DefaultEndpoints`, `Tokens`, `Site`, `Scopes` (FR16), `ErrInvalidGrant`,
  `AuthorizeURL`, `Exchange`, `Refresh`, `Sites`, `JiraAPIBase`.
- [ ] T1.2 `internal/atlassian/atlassiantest`: the fake Atlassian of the plan's
  test strategy (authorise, rotating token endpoint that invalidates a used
  refresh token at once, `invalid_grant` and 503 switches, accessible
  resources, Bearer-checked `/ex/jira/{cloudId}` gateway recording the account
  of each write and rejecting a scope the grant lacks).
- [ ] T1.3 Tests: authorise URL parameters (audience, scopes, `prompt=consent`,
  redirect, state); exchange and refresh parse expiry; `invalid_grant` maps to
  `ErrInvalidGrant`; a 5xx is a plain error; **no error text contains the
  secret, the code or a token** (FR2, AC9).

## 2. Scopes check (FR16, AC8)

- [ ] T2.1 Check every row of the FR16 table against the "OAuth scopes
  required" of Atlassian's Jira platform and Jira Software REST references;
  correct `spec.md` and `atlassian.Scopes` together where they differ (board
  configuration and backlog endpoints first).
- [ ] T2.2 Test: a list of every `/rest/api/3` and `/rest/agile/1.0` path the
  `trackerapi` package calls (grep-based, like the endpoint table) maps to a
  scope in `Scopes`; a new path without a mapping fails the test.

## 3. Sealing and storage (plan 2, 3; FR10, FR11)

- [ ] T3.1 `secrets.Binding.Kind` (unchanged bytes for `""` / `api_token`,
  suffix for `oauth`) and `secrets.OAuthAppBinding`. Tests: an existing record
  still opens; an oauth record does not open as api_token, as another user, as
  a server credential or as an app secret.
- [ ] T3.2 Migrations 38 (`user_tracker_credentials` `kind`, `version`,
  `disconnected_at`), 39 (`jira_oauth_flows`), 40 (`tracker_oauth_apps`) in
  `internal/db/migrations.go` only; next free numbers if taken. New tables in
  `migrationTables`.
- [ ] T3.3 Rewind and replay fixtures of plan 3 drop the new columns and
  tables; `dropCredentialAccountColumn` comment range updated.
- [ ] T3.4 `ensureKeyOpensUserCredentials` passes the row's kind.
- [ ] T3.5 `SetUserTrackerCredential` writes `kind = 'api_token'`, clears
  `disconnected_at`, bumps `version`; lock, unlock and presence code skip
  oauth rows.
- [ ] T3.6 `UserCredential.Kind`, `.Disconnected`, `.GrantedSites`, filled by
  `UserTrackerCredentials`.
- [ ] T3.7 Tests (SQLite and PostgreSQL): an upgraded database reads every
  existing row as `api_token` and a sealed token still unlocks (AC10);
  migration replay from the baseline passes.

## 4. OAuth app configuration (plan 2; FR1 to FR3, US6)

- [ ] T4.1 `internal/db/oauthapps.go`: `JiraOAuthApp`, `SaveJiraOAuthApp`,
  `ClearJiraOAuthApp`, environment fallback with ADR 0028 precedence.
- [ ] T4.2 `internal/handlers/jiraoauthapp.go`: `GET/PUT/DELETE
  /api/admin/jira-oauth`, admin only, secret write-only, redirect URL
  validation; routes in `cmd/server/main.go`.
- [ ] T4.3 Tests: stored wins over environment as a whole; empty secret on
  save keeps the stored one; first save without a secret is 400; a member is
  refused 403; no response body contains the secret (AC9); unreadable row
  fails and does not fall back.

## 5. Authorisation flow (plan 4; FR4 to FR9, US1, US4.3, US5.1, US7)

- [ ] T5.1 `StartJiraOAuthFlow` / `ConsumeJiraOAuthFlow` (single-statement
  consume) and the cleanup of expired or consumed flows.
- [ ] T5.2 `db.ConfiguredJiraSites()` (global plus every Jira project's site,
  normalised, deduplicated).
- [ ] T5.3 `db.SaveJiraGrant` (replaces any row and its unlock, one
  transaction).
- [ ] T5.4 `POST /api/me/tracker-credentials/jira/connect` (409 when not
  configured, 403 for an agent bearer) and `GET /auth/jira/callback` with the
  five outcomes, redirecting to `/?trackerCredentials=jira&jiraOAuth=<outcome>`.
- [ ] T5.5 `GET /api/me/tracker-credentials` adds `jiraOAuth {configured,
  sites}`; unchanged otherwise.
- [ ] T5.6 Tests against the fake Atlassian: connect then callback stores an
  oauth row with the account and both granted sites (US1.3, US4.1); callback
  with no state, a reused state, an expired state (clock), another session,
  another user → nothing stored (AC6); consumption on a second `DB` sharing
  the database succeeds once only (US7.2); `access_denied` → `cancelled`,
  nothing stored; a grant covering no configured site → `no_site`, the
  existing API token untouched (AC7); connecting over a sealed, locked API
  token replaces it and its unlock without a passphrase (AC2); the handler's
  logs contain neither code, token nor secret (AC9).

## 6. Refresh and transport (plan 5, 6; FR12 to FR15, FR21, US2 to US4)

- [ ] T6.1 `trackerapi.Client.JiraAPIBase` / `JiraBearer`, `jiraEndpoint()`,
  Bearer header, `jiraConfigured()`; `CheckJiraBearer`; the bearer 401
  message.
- [ ] T6.2 `PersonalCredential` and the new `ResolveUser` signature;
  `ForActingUser` passes the effective site and sets bearer fields; every
  caller and test double of `ResolveUser` updated.
- [ ] T6.3 `MissingPersonalCredentialError.Reason` / `.Site` with the two new
  texts; today's text unchanged for an empty reason.
- [ ] T6.4 `db.jiraGrantAccess` with the compare-and-set refresh of plan 5,
  dispatched from `UserTrackerCredentialsFor` by kind; check that no caller of
  `ForWrite` / `ForActingUser` holds `d.mu`.
- [ ] T6.5 `ConfirmUserTrackerCredential` and `CheckTrackerCredentials` skip
  oauth rows.
- [ ] T6.6 Tests, `trackerapi`: a Bearer variant of `newJiraSite`; with bearer
  fields every call goes to the API base with `Authorization: Bearer`, links
  and caches keep the site URL; without them, calls are byte-for-byte as today
  (AC10).
- [ ] T6.7 Tests, `db` against the fake Atlassian:
  - transition, comment and assignment by a connected person are recorded
    under their account by the fake gateway (AC1);
  - access token expired: a queued write (tracker op activity) and a managed
    run's stage transition succeed after one refresh, the row's version moved
    by one (AC3);
  - two `DB` instances on one database refresh the same expired grant
    concurrently against the strict rotating fake: both calls succeed, the row
    is not disconnected, and a later refresh succeeds (AC4); repeated under
    `-race`;
  - `invalid_grant` marks the row disconnected; the write fails with
    `MissingPersonalCredentialError{Reason: disconnected}`, carries the
    `tracker_credential_missing` code in the handler's 403, and the server
    credential of the fake is never used (AC5);
  - a 503 on refresh fails the call and leaves the row connected (US2.4);
  - a project whose site the grant lacks fails with `Reason:
    site-not-granted` naming the site (AC7);
  - reads for a disconnected grant fall back to the project client, as reads
    do today.

## 7. Web (plan 7; FR17 to FR20)

- [ ] T7.1 `web/src/lib/jiraOAuth.ts` (`jiraEntryState`,
  `oauthOutcomeFromSearch`); `trackers.ts` ignores oauth rows in the
  passphrase counts.
- [ ] T7.2 `AppContext.tsx`: `connectJira`, the start-up outcome handling
  (open the profile on Jira, notify, strip the parameters).
- [ ] T7.3 `TrackerCredentialForm.tsx` (or `JiraConnectPanel.tsx`): the five
  states of FR18, "Use an API token instead", *Disconnect* with the Atlassian
  note, *Reconnect Jira*.
- [ ] T7.4 `JiraOAuthAppPanel.tsx` and `lib/jiraOAuthApp.ts` in the
  Administration page.
- [ ] T7.5 French and English strings in `locales/translations.ts`.
- [ ] T7.6 Tests `web/tests/jiraOAuth.test.mjs`: every `jiraEntryState`
  case, including `'form'` whenever OAuth is not configured (AC10); every
  outcome parsed, unknown ones ignored; passphrase counts ignore oauth rows;
  translation completeness covers the new keys (extend
  `trackerSetup.test.mjs:171`); `tsc` passes.
- [ ] T7.7 The web never holds the secret beyond the admin's own input: the
  admin GET type has no secret field, the panel clears the secret input after
  a save, and no module under `web/src` reads a `SECTILE_JIRA_OAUTH_*`
  variable (a grep test in `jiraOAuth.test.mjs`). With T4.3, this covers AC9
  for the bundle.

## 8. Documentation (FR22, AC12)

- [ ] T8.1 `CHANGELOG.md` `Added` line under `[Unreleased]`.
- [ ] T8.2 `docs/USER_GUIDE.md` Jira access section, with the admin
  registration steps and the scopes.
- [ ] T8.3 `docs/API_AND_DATA_SPEC.md` § 2.3.1 and the admin route.
- [ ] T8.4 `.env.sample`: the three variables.
- [ ] T8.5 ADR 0044: title, status Accepted, the two amendments and the
  scope list; ADR 0014 status line.

## 9. Verification

- [ ] T9.1 `go test ./...` on SQLite, and `internal/db`, `internal/handlers`
  on a throwaway PostgreSQL database (AC11).
- [ ] T9.2 `node --test web/tests/*.test.mjs` and `tsc` (AC11).
- [ ] T9.3 Manual pass with OAuth unconfigured: profile and a Jira write
  identical to `main` (AC10).
- [ ] T9.4 Pull request states that the end-to-end check against Atlassian
  (AC1) waits on the prerequisites, unless they exist by then.
