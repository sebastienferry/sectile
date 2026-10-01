# Plan #654 - Connect Jira through Atlassian OAuth instead of a pasted API token

## Stack

Go server (`internal/atlassian` new, `internal/secrets`, `internal/db`,
`internal/trackerapi`, `internal/handlers`, `cmd/server`), SQLite and
PostgreSQL through numbered migrations, React 19 + TypeScript web app
(`web/`), `node --test` unit tests and Go tests. No agent, desktop, MCP or
GitHub/GitLab adapter change.

## Design

### 1. Atlassian OAuth client (`internal/atlassian`, new package)

A small, dependency-free package owning every call to Atlassian's identity
endpoints, so the rest of the code never builds an OAuth request.

```go
type App struct{ ClientID, ClientSecret, RedirectURL string }

// Endpoints default to Atlassian's; tests point them at an httptest server.
type Endpoints struct{ Authorize, Token, Resources, API string }
var DefaultEndpoints = Endpoints{
    Authorize: "https://auth.atlassian.com/authorize",
    Token:     "https://auth.atlassian.com/oauth/token",
    Resources: "https://api.atlassian.com/oauth/token/accessible-resources",
    API:       "https://api.atlassian.com",
}

type Tokens struct{ AccessToken, RefreshToken string; ExpiresAt time.Time; Scope string }
type Site struct{ CloudID, URL, Name string }

var Scopes = []string{ /* spec FR16, in order */ }
var ErrInvalidGrant = errors.New("atlassian: invalid_grant")

func (e Endpoints) AuthorizeURL(app App, state string) string
func (e Endpoints) Exchange(ctx, hc *http.Client, app App, code string) (Tokens, error)
func (e Endpoints) Refresh(ctx, hc *http.Client, app App, refreshToken string) (Tokens, error)
func (e Endpoints) Sites(ctx, hc *http.Client, accessToken string) ([]Site, error)
func (e Endpoints) JiraAPIBase(cloudID string) string // API + "/ex/jira/" + cloudID
```

- `AuthorizeURL`: `audience=api.atlassian.com`, `client_id`, `scope` (space
  separated), `redirect_uri`, `state`, `response_type=code`, `prompt=consent`.
- `Exchange` / `Refresh`: `POST` JSON to `Token` with
  `grant_type=authorization_code|refresh_token`. `ExpiresAt = now +
  expires_in`. A 400/401 whose body `error` is `invalid_grant` returns
  `ErrInvalidGrant`; every other failure returns an error carrying the status
  and Atlassian's `error` code only. **Errors never include the request body,
  the client secret or a token** (spec FR2); a test asserts it.
- `Sites`: `GET Resources` with Bearer; keeps entries whose `scopes` include a
  Jira scope.
- Sites match on a normalised URL: `trackerapi.JiraSite` (lower case, no
  trailing slash, scheme kept), the helper the Jira client already uses.

### 2. Configuration of the OAuth app (spec FR1 to FR3, US6)

- Environment: `SECTILE_JIRA_OAUTH_CLIENT_ID`,
  `SECTILE_JIRA_OAUTH_CLIENT_SECRET`, `SECTILE_JIRA_OAUTH_REDIRECT_URL`, read
  with `strings.TrimSpace(os.Getenv(...))` like `SECTILE_OIDC_REDIRECT_URL`
  (`internal/auth/oidc.go:74`).
- **Migration 40** `tracker_oauth_apps`:

  ```sql
  CREATE TABLE tracker_oauth_apps (
      tracker TEXT PRIMARY KEY,
      client_id TEXT NOT NULL,
      record BLOB NOT NULL,          -- client secret, sealed
      redirect_url TEXT NOT NULL,
      updated_at TEXT NOT NULL,
      updated_by TEXT NOT NULL DEFAULT ''
  );
  ```

  PostgreSQL variant with `BYTEA`, as migration 22 (`server_tracker_credentials`)
  does. Added to `migrationTables` (`internal/db/migrate.go:24`).
- `secrets.OAuthAppBinding(tracker)`: a third binding prefix,
  `"sectile:v1:oauth-app:tracker:%d:%s"`, so the secret never opens as a
  server or personal credential.
- `internal/db/oauthapps.go`: `JiraOAuthApp() (atlassian.App, source string,
  err error)` (source `database` / `environment` / `none`; a stored row wins as
  a whole, an unreadable row fails and never falls back, as
  `ServerTrackerCredentialState` does), `SaveJiraOAuthApp(clientID, secret,
  redirectURL, adminID)` (empty secret keeps the stored one; the first save
  requires it), `ClearJiraOAuthApp()`. Configured means all three non-empty.
  The app is read at each use, never cached (ADR 0028).
- Admin API `internal/handlers/jiraoauthapp.go`, `/api/admin/jira-oauth`,
  behind `requireAdmin` / `adminOnlyRoute`, `MaxBytesReader` 16 KB:
  - `GET` → `{configured, clientId, secretSet, redirectUrl, source}`;
  - `PUT {clientId, clientSecret?, redirectUrl}` → same shape; `redirectUrl`
    must be an absolute `https` URL (`http` allowed for `localhost` /
    `127.0.0.1`), else 400;
  - `DELETE` → 204.
  Registered in `cmd/server/main.go` next to the admin tracker credentials
  (:329).

### 3. Personal credential rows (spec FR10, FR11)

- **Migration 38** `user_tracker_credentials.oauth`, one statement each:

  ```sql
  ALTER TABLE user_tracker_credentials ADD COLUMN kind TEXT NOT NULL DEFAULT 'api_token';
  ALTER TABLE user_tracker_credentials ADD COLUMN version INTEGER NOT NULL DEFAULT 0;
  ALTER TABLE user_tracker_credentials ADD COLUMN disconnected_at TEXT NOT NULL DEFAULT '';
  ```

  Every existing row reads as an API token (FR11). Never in the frozen
  baseline (`testdata/baseline_schema.txt` untouched).
- `secrets.Binding` gains `Kind string`. `bytes()` is unchanged when `Kind` is
  `""` or `"api_token"`, so existing records keep opening; `"oauth"` appends
  `":kind:%d:%s"`. `ensureKeyOpensUserCredentials` (`migrate.go:151`) passes
  the row's kind.
- The grant record, sealed with `Binding{UserID, "jira", Kind: "oauth"}`, is
  JSON:

  ```json
  {"refreshToken": "...", "accessToken": "...", "expiresAt": "RFC3339",
   "scope": "...", "sites": [{"cloudId": "...", "url": "https://x.atlassian.net", "name": "x"}]}
  ```

  `site_url` and `email` are empty, `sealed = 0`, `salt` NULL, `account` the
  confirmed display name.
- `internal/db/jiragrants.go`:
  - `SaveJiraGrant(userID string, t atlassian.Tokens, sites []atlassian.Site,
    account string) error`: one transaction, upsert the row with `kind =
    'oauth'`, `version = version + 1` (0 on insert), `disconnected_at = ''`,
    and delete the row of `user_credential_unlocks` (US5.1).
  - `jiraGrantAccess(ctx, userID, site string) (apiBase, accessToken string, err error)`,
    the refresh algorithm of section 5.
  - `UserCredential` (`usercredentials.go:31`) gains `Kind` (`kind`),
    `Disconnected` (`disconnected`) and `GrantedSites []string`
    (`grantedSites`, the site URLs, read by opening the record with the server
    key). `UserTrackerCredentials` fills them.
  - `UnlockUserTrackerCredential`, `LockUserTrackerCredential` and the unlock
    presence code skip `kind = 'oauth'` rows (FR19); `SetUserTrackerCredential`
    writes `kind = 'api_token'`, `disconnected_at = ''` and bumps `version`
    (US5.3). `ClearUserTrackerCredential` is unchanged (it deletes any kind).
- Rewind and replay fixtures drop the new columns and tables wherever they
  rewind below 38: `migrations_test.go` (`forgetSchemaVersion`,
  `dropRepositoryColumns`, `dropCredentialAccountColumn` and its "24 to 37"
  comment, `undoWorkstationMigrations`, the inline lists of
  `TestMigrationThirtyOne…` and `TestMigrationSixteen…`),
  `activerun_test.go:136-171`, `specartifacts_test.go:109-132`,
  `branchformat_test.go:93-122`, `stagecommits_test.go:64-92`. Each list
  already ends with the `readiness` and `credential_missing` drops; add
  `kind`, `version`, `disconnected_at` and the two new tables beside them.
  If `main` has taken 38 to 40 when this lands, take the next free numbers
  and recreate the PostgreSQL test database before trusting a "column does
  not exist".

### 4. Authorisation flow (spec FR4 to FR9, US1, US7)

- **Migration 39** `jira_oauth_flows`, modelled on `login_flows`
  (`internal/db/sessions.go`):

  ```sql
  CREATE TABLE jira_oauth_flows (
      state_hash TEXT PRIMARY KEY,
      user_id TEXT NOT NULL,
      session_hash TEXT NOT NULL,
      created_at TEXT NOT NULL,
      expires_at TEXT NOT NULL,
      consumed_at TEXT
  );
  ```

  Added to `migrationTables`. `StartJiraOAuthFlow(userID, sessionToken)
  (state string, err error)` stores `hashSecret(state)` and
  `hashSecret(sessionToken)`, TTL 10 minutes (`jiraOAuthFlowTTL`).
  `ConsumeJiraOAuthFlow(state, userID, sessionToken) error` consumes in one
  statement, so exactly one caller wins on any replica:

  ```sql
  UPDATE jira_oauth_flows SET consumed_at = ?
   WHERE state_hash = ? AND user_id = ? AND session_hash = ?
     AND consumed_at IS NULL AND expires_at > ?
  ```

  One row affected is success; zero is `ErrJiraOAuthFlow` whatever the cause
  (missing, reused, expired, foreign session or person, US7.1). The periodic
  cleanup that purges `login_flows` purges this table too.
- `POST /api/me/tracker-credentials/jira/connect` (in
  `internal/handlers/usercredentials.go`, same routes as today):
  - OAuth not configured → 409 `{code: "jira_oauth_not_configured"}`;
  - the caller must hold a web session cookie (`sectile_session`): an agent
    bearer is refused 403 (FR5);
  - answers `{authorizeUrl}`; the web sets `window.location` to it.
- `GET /auth/jira/callback` in `internal/handlers/jiraoauth.go`. `/auth/` is
  already a public prefix (`auth.go:283-300`), so the handler reads the
  session itself; the session cookie is `SameSite=Lax` (`auth.go:53`), so it
  is sent on Atlassian's top-level redirect. Steps:
  1. `error=access_denied` → outcome `cancelled` (the flow is consumed when
     the state is valid, ignored otherwise).
  2. No session, or `ConsumeJiraOAuthFlow` fails → `invalid`.
  3. `Exchange` → `Sites` → keep every site; compute the configured Jira
     sites (`db.ConfiguredJiraSites()`: `settings.JiraUrl` plus the
     `TrackerUrl` of every Jira project, normalised, deduplicated); no
     intersection → `no_site`, nothing stored (FR7).
  4. `CheckJiraBearer` (section 6) on the first covered configured site →
     account name; failure → `unreachable`.
  5. `SaveJiraGrant` → `connected`.
  Atlassian unreachable or a non-2xx at steps 3-4 → `unreachable`. The
  handler logs the outcome and the user id, never the code, a token or the
  secret.
  Every outcome redirects (302) to
  `/?trackerCredentials=jira&jiraOAuth=<outcome>`.
- `GET /api/me/tracker-credentials` adds
  `jiraOAuth: {configured: bool, sites: string[]}` (the configured Jira
  sites, for the `no_site` message and the profile). Nothing else in the
  answer changes when OAuth is not configured (FR17).

### 5. Refresh, serialised by compare-and-set (spec FR14, FR15, US2, US3)

`jiraGrantAccess(ctx, userID, site)`:

1. Read `kind, version, record, disconnected_at` and open the record. The
   read takes the store's read lock only for the query; **no lock of `d.mu`
   is held across an HTTP call**, and the implementation checks that no
   caller of `ForWrite` / `ForActingUser` holds `d.mu` (the RWMutex is not
   re-entrant; `userTrackerCredential` already documents one such deadlock).
2. `disconnected_at != ''` → `&MissingPersonalCredentialError{Tracker:
   "jira", Reason: ReasonDisconnected}`.
3. Find the site matching `site`; none →
   `&MissingPersonalCredentialError{Tracker: "jira", Reason:
   ReasonSiteNotGranted, Site: site}`.
4. `ExpiresAt` more than one minute ahead → return
   `JiraAPIBase(cloudID)` and the access token.
5. Otherwise `Refresh` with the app read at this moment:
   - success → seal the new record and
     `UPDATE ... SET record = ?, version = version + 1, updated_at = ?
      WHERE user_id = ? AND tracker = 'jira' AND version = ?`.
     One row → use the new token. Zero rows → another replica won: go to 1
     and use its token (at most three rounds, then fail).
   - `ErrInvalidGrant` → read the row again first. If its `version` moved,
     another replica rotated the refresh token: go to 1. Otherwise
     `UPDATE ... SET disconnected_at = ? WHERE ... AND version = ?`, and
     answer `ReasonDisconnected`. The version check is what keeps a loser of a
     concurrent refresh from disconnecting a live grant (AC4).
   - any other error → return it wrapped, row untouched (US2.4).
   - OAuth no longer configured → fail with an error saying the Jira OAuth
     app is not configured; the row is untouched.

A per-process `singleflight` keyed on the user id may merge concurrent
refreshes on one replica; it is an optimisation, the compare-and-set is the
guarantee.

### 6. Jira transport (spec FR12, FR13)

- `trackerapi.Client` gains `JiraAPIBase, JiraBearer string`. `jira()`
  (`jira_client.go:80`) builds `endpoint := c.jiraEndpoint() + path`, where
  `jiraEndpoint()` is `JiraAPIBase` when set, `JiraURL` otherwise, and sends
  `"Bearer " + JiraBearer` when set, `jiraBasicAuth(...)` otherwise.
  `jiraConfigured()` accepts a bearer without an e-mail. Pagination helpers,
  links and the priority cache keep reading `JiraURL` (the site), unchanged.
- `ResolveUser` changes shape to receive the project's site and answer a
  struct, so the grant can pick its `cloudId`:

  ```go
  type PersonalCredential struct {
      SiteURL, Email, Token string // API token path, as today
      APIBase, Bearer       string // OAuth grant
  }
  ResolveUser func(userID, tracker, site string) (PersonalCredential, error)
  ```

  `ForActingUser` passes `resolved.JiraURL` as `site`; for a bearer it sets
  `JiraAPIBase` and `JiraBearer`, leaves `JiraURL` (the project's effective
  site) and clears `JiraToken` / `JiraEmail`. `db.UserTrackerCredentialsFor`
  (wired at `db.go:247`) dispatches on `kind`: API tokens as today, grants
  through `jiraGrantAccess` with a context bounded to 15 s.
- An error from the resolver already makes `ForWrite` fail (writes) and
  `trackerAs` fall back to the project client (reads), which is the wanted
  behaviour for both new reasons.
- `MissingPersonalCredentialError` gains `Reason` (`""`,
  `ReasonSiteNotGranted`, `ReasonDisconnected`) and `Site`. `Error()` is
  unchanged for `""` (FR21). The two new texts:
  - site: `no personal Jira grant covers <site> for this user: reconnect Jira in Profile → Tracker credentials and pick that site on the Atlassian consent screen, or the work would be attributed to the server account`;
  - disconnected: `the Jira connection of this user was revoked or has expired: reconnect Jira in Profile → Tracker credentials, or the work would be attributed to the server account`.
  `MissingCredentialTracker` and the `tracker_credential_missing` code are
  unchanged, so #645's offer appears with no web change on that path.
- `CheckJiraBearer(ctx, apiBase, accessToken) (string, error)`: `GET
  /rest/api/3/myself`, same answer as `CheckJira`.
- A 401 through a bearer (grant revoked within the access token's hour) is
  reported by `jiraError` with a French message telling the person to
  reconnect Jira in *Profil → Identifiants du tracker*, instead of the API
  token message. The row is not disconnected on a 401: only `invalid_grant`
  does that.
- `ConfirmUserTrackerCredential` and `CheckTrackerCredentials` skip OAuth
  rows (the grant is confirmed at the callback).

### 7. Web (spec FR17 to FR20)

- `web/src/lib/jiraOAuth.ts` (pure, unit tested):
  - `jiraEntryState(credential, oauth)` → `'form' | 'connect' |
    'token-and-connect' | 'connected' | 'disconnected'` (FR18; `'form'`
    whenever OAuth is not configured);
  - `oauthOutcomeFromSearch(search)` → `{outcome} | null`, and the list of
    outcomes `connected | cancelled | invalid | no_site | unreachable`.
- `web/src/lib/trackers.ts`: `credentialState`, `sealingConsequence` and the
  passphrase counts ignore `kind === 'oauth'` (FR19).
- `AppContext.tsx`: `connectJira()` (POST connect, then
  `window.location.assign(authorizeUrl)`); on start-up, when
  `oauthOutcomeFromSearch(location.search)` answers, call
  `openTrackerCredentials('jira')`, refresh the credentials, show a
  notification for the outcome (success or error, the `no_site` one naming
  `jiraOAuth.sites`), and remove the two parameters with
  `history.replaceState`.
- `TrackerCredentialForm.tsx`: for Jira, render by `jiraEntryState`:
  *Connect Jira* / "Use an API token instead" revealing today's form;
  connected (account, sites, *Disconnect* calling `clearUserCredential`, the
  Atlassian connected-apps note); disconnected (warning, *Reconnect Jira*).
  Other trackers unchanged. Extract a `JiraConnectPanel.tsx` if the form
  grows past readability.
- Administration: `web/src/components/JiraOAuthAppPanel.tsx` beside
  `ServerTrackerCredentialsPanel` in `AdminView.tsx:118`, with
  `web/src/lib/jiraOAuthApp.ts` (path, state type, source label). The secret
  field is a password input left empty on load, "set" shown from `secretSet`.
- Strings in `web/src/locales/translations.ts` under `trackerCredentials.oauth`
  and `admin.jiraOAuth`, French and English.

### 8. Documentation

- `CHANGELOG.md`, `[Unreleased]` → `### Added`: one line, for example
  "**Connect Jira with one consent screen.** When an admin configured an
  Atlassian OAuth app, *Profile → Tracker credentials* offers **Connect
  Jira** instead of pasting an API token: your writes keep going out under
  your Jira account, refreshed in the background, and **Reconnect Jira**
  appears if the connection is lost. API tokens keep working. (#654)".
- `docs/USER_GUIDE.md` § "Set up a personal Jira token" (:25): *Connect
  Jira* first, the API token as the alternative, *Reconnect Jira*, removing
  the app from Atlassian's connected apps; an admin subsection on registering
  the app (distribution, callback URL, scopes of spec FR16).
- `docs/API_AND_DATA_SPEC.md` § 2.3.1 (:276): the connect and callback
  routes, the `kind` / `disconnected` / `grantedSites` fields, `jiraOAuth`,
  the admin route, the grant record and its binding.
- `.env.sample`: the three variables, commented.
- ADR 0044: title "ADR 0044", status Accepted, decision amended (every granted
  site kept, refusal only when none is configured; explicit
  `SECTILE_JIRA_OAUTH_REDIRECT_URL` instead of a derived URL; the scope list
  of spec FR16). ADR 0014 status line: "Amended by ADR 0044 for Jira".

## Target files

| Area | Files |
| --- | --- |
| OAuth client | `internal/atlassian/oauth.go`, `oauth_test.go` (new) |
| Sealing | `internal/secrets/secrets.go`, `secrets_test.go` |
| Store | `internal/db/migrations.go`, `migrate.go`, `usercredentials.go`, `jiragrants.go` (new), `oauthapps.go` (new), `jiraoauthflows.go` (new), `trackercredentials.go`, `db.go`, rewind fixtures listed in section 3 |
| Tracker client | `internal/trackerapi/client.go`, `jira_client.go`, `jira_test.go` (Bearer variant of `newJiraSite`) |
| Handlers | `internal/handlers/usercredentials.go`, `jiraoauth.go` (new), `jiraoauthapp.go` (new), `cmd/server/main.go` |
| Web | `web/src/lib/jiraOAuth.ts`, `jiraOAuthApp.ts` (new), `trackers.ts`, `context/AppContext.tsx`, `components/TrackerCredentialForm.tsx`, `TrackerCredentialsTab.tsx`, `JiraOAuthAppPanel.tsx` (new), `AdminView.tsx`, `locales/translations.ts`, `web/tests/jiraOAuth.test.mjs` (new) |
| Docs | `CHANGELOG.md`, `docs/USER_GUIDE.md`, `docs/API_AND_DATA_SPEC.md`, `.env.sample`, ADR 0044, ADR 0014 |

## Test strategy

- A fake Atlassian (`httptest`) in `internal/atlassian` test helpers,
  exported for `db` and `handlers` tests through a small `atlassiantest`
  package: authorise (records the query), token endpoint (issues codes,
  rotates refresh tokens, **invalidates a used refresh token at once**, can be
  told to answer `invalid_grant` or 503), accessible-resources (configurable
  sites), and `/ex/jira/{cloudId}/rest/...` serving the Jira paths with Bearer
  checking, recording the account each write was made with.
- Go: SQLite and PostgreSQL (`SECTILE_TEST_POSTGRES_DSN`, a throwaway
  database only). Web: `node --test web/tests/*.test.mjs` and `tsc`.
- End to end against Atlassian: manual, once the prerequisites exist (spec
  AC1); the pull request says whether it was done.

## Risks

- `d.mu` held by a caller of `ForWrite` while the refresh needs it: a
  deadlock. Checked while implementing (section 5, step 1).
- Granular Jira Software scopes named wrongly: the consent fails or an agile
  call answers 401. The scopes are checked against Atlassian's reference and
  the fake gateway rejects a call whose scope the grant lacks.
- Atlassian's rate limit is per app and site: only person-caused calls go
  through grants; synchronisation keeps the server credential.

## Implementation notes

What the implementation changed from this plan, and why.

- **Migration numbers.** `main` took 38 (`projects.roadmap_axis_writes`) and
  39 (`projects.epic_axis_prefixes`), so the three migrations are 40
  (`user_tracker_credentials.oauth`), 41 (`jira_oauth_flows`) and 42
  (`tracker_oauth_apps`). Timestamps are
  `DATETIME`, like `login_flows`, rather than `TEXT`.
- **The refresh is claimed before Atlassian is called.** Section 5 compared
  the version only after an `invalid_grant`. The concurrency test against the
  strict rotating fake showed the hole: an instance reading the row after
  another's refresh but before its write spent the rotated refresh token and
  disconnected a live grant. Migration 40 therefore adds
  `refresh_claimed_at`; a claim is a compare-and-set on `version` that sets
  it, waiters poll until the version moves, a claim older than the refresh
  wait (60 s, four times the 15 s call timeout, so a replica whose clock is
  ahead does not take over a refresh still in flight) is taken over, and a
  claim whose refresh failed without spending the token is released at once.
  With the claim held, `invalid_grant` is trusted and disconnects. The refresh
  runs detached from the caller's deadline, so a cancelled request never drops
  a rotated refresh token, and a claimant's write also requires the row to be
  an OAuth grant still claimed, since a deleted and recreated row restarts its
  version.
- **Scopes.** Checked against Atlassian's OpenAPI documents (see spec FR16):
  `read:board-scope.admin:jira-software` added for the board configuration,
  `read:issue:jira-software` and `write:issue:jira-software` dropped, and the
  priority search skipped for a client calling through a grant, since it needs
  `manage:jira-configuration`.
- **`unauthorized_client` on a refresh** whose description names the refresh
  token is read as `invalid_grant`, which is how Atlassian answers some expired
  refresh tokens; the same code without it (a wrong client id) disconnects
  nobody.
- **Checks of a grant.** `CheckTrackerCredentials` checks a grant through
  itself (`CheckJiraBearer`) when no token is typed, instead of falling back to
  the server token; `ConfirmUserTrackerCredential` skips grants.
- **The flow logic lives in the store** (`db.JiraOAuthAuthorizeURL`,
  `db.CompleteJiraOAuth`), so the handlers stay thin and the store tests cover
  the five outcomes; the handler tests cover the routes and the logs.
- **A grant keeps its panel when OAuth is no longer configured**
  (`jiraEntryState`): the token form would claim a token is stored.
