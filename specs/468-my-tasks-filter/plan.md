# Plan: My Tasks finds the tickets assigned to me

Behaviour is in `spec.md`. This file says how, and where.

## Stack

- Go server: `internal/db` (shared SQL, SQLite and PostgreSQL), `internal/handlers`,
  routes in `cmd/server/main.go`.
- React/TypeScript web app: `web/src`, unit tests with `node --test` in
  `web/tests/*.test.mjs`, browser tests in `web/tests/*.browser.mjs`.
- Only the web app has the button; the desktop app embeds the same web UI.

## Root cause (three defects)

1. `Sidebar.tsx:642` sends `assignee=<settings.userName>`, the account display name
   since #348, and `GetTasksInScope` (`internal/db/db.go`, the `assignee = ?` condition, line 1781 on
   `origin/main`) matches
   `assignee = ?` exactly. `tasks.assignee` holds the GitHub login
   (`trackerapi/mapping.go`) or the Jira displayName (`trackerapi/jira_mapping.go`):
   practically never equal.
2. The facet guard in `AppContext.tsx:1576-1588` drops any `assigneeFilter` that is not
   one of the assignees in scope. The display name is almost never one, so the filter
   is cleared right after the click: the button looks dead.
3. `isMyTasksActive` (`Sidebar.tsx:271`) compares the stored name with the current one,
   so a rename leaves a stale, invisible filter.

## Implementation notes

Two refinements made while implementing, neither changing the specification:

- **Migration 24, not 22.** `#469` (#464, server tracker credentials) landed migration 22
  and #475 (the waiting session) migration 23 on `main` before this branch merged them,
  so the column is added by migration 24.
- **Case folding is `lowerASCII` on both sides**, the store's existing pair
  (`d.lowerASCII` in SQL, `asciiLower` in Go), not `LOWER()` against `strings.ToLower`.
  SQLite's `LOWER` folds A-Z only: with `strings.ToLower` on the Go side, an assignee
  `Élodie` stored and typed identically would have stopped matching on SQLite. With the
  same ASCII fold on both sides the two engines agree, identical text always matches,
  and only a case difference on an accented capital (`ÉLODIE` against `élodie`) is not
  folded, on either engine.
- **Saving probes through `ConfirmUserTrackerCredential`**, which opens the credential
  just stored and asks its own site with its own token, rather than calling
  `CheckTrackerCredentials` with the request's fields: that one falls back to the server
  token for GitLab and would have confirmed the wrong account.
- **"Its own site" compares the address the check actually used** with the one the
  stored credential would use (defaults applied, `JiraSite` for Jira), because the
  profile's *Vérifier* button sends the pre-filled server site when the personal
  credential names none.

## Architecture

"Mine" becomes a flag resolved on the server. The client never sends a name for it.

### 1. Migration 22: the confirmed account of a personal credential

`internal/db/migrations.go`, next free version. `origin/main` stops at 21
(`task_activities.waiting_reason`, landed with #456 and #470); `feat/468` must merge
`origin/main` first, and renumber if `main` lands another one before this merges:

```go
{
    // The account a personal tracker credential belongs to, as the tracker
    // reported it when the credential was confirmed (#468). My Tasks matches
    // tickets on it. Empty until the credential is saved or verified again.
    version: 22,
    name:    "user_tracker_credentials.account",
    statements: []string{
        "ALTER TABLE user_tracker_credentials ADD COLUMN account TEXT NOT NULL DEFAULT '';",
    },
},
```

Never in `ensureUserCredentialsTable` (the baseline definition is frozen).
The account is a tracker login or display name, not a secret: it is stored in clear,
next to the sealed record, which is why a locked credential keeps it (spec FR 5).
This refines answer 2 of the clarification, which fell back on a locked credential
because it assumed a probe at read time; the owner accepted the refinement on
2026-09-25 (`docs/clarifications/468.md`, "Specification note").

### 2. Storage (`internal/db/usercredentials.go`)

- `UserCredential` gains `Account string \`json:"account,omitempty"\``, read by
  `UserTrackerCredentials`.
- `SetUserTrackerCredential` resets `account` to `''` whenever it writes the row: a new
  token or site is unconfirmed until probed.
- New `SetUserTrackerCredentialAccount(userID, tracker, account string) error`: updates
  the one column; `ErrNoUserCredential` when the row is missing.
- `ClearUserTrackerCredential` already deletes the row, which forgets the account.
- New `TrackerAccounts(userID string) (map[string]string, error)`: tracker → account for
  the rows whose account is not empty. No decryption, so it works on sealed rows.

### 3. Learning the account (spec FR 4)

- **On save** (`handlers/usercredentials.go`, PUT): after `SetUserTrackerCredential`
  succeeds, and for `github`, `gitlab`, `jira`, call
  `h.db.CheckTrackerCredentials(h.actingContext(r), tracker, req.SiteURL, req.Email, req.Token)`
  (5 s timeout already). Success → `SetUserTrackerCredentialAccount`. Failure → log it,
  leave the account empty, still answer 200: the save itself keeps today's contract
  (the form already refuses to save before a successful check). An empty `req.Token`
  falls back to the stored token, as `checkTrackerCredentials` already does for GitHub
  and Jira.
- **On verify** (`db/trackercredentials.go`, `checkTrackerCredentials`): in the `github`
  and `jira` branches, when the probe used the caller's own stored token (`token == ""`
  and `own != ""`) and the site is the stored one (`apiURL == "" || apiURL == site`),
  record the returned account for the acting user. A check with a typed token, or one
  answered by the server-wide token, records nothing: it is not proof of the caller's
  own account (the admin setup screen goes through the same endpoint).
- Nothing else probes. `GET /api/tasks` never reaches a tracker.

### 4. Resolving "me" (`internal/db`, new file `mytasks.go`)

```go
// MyTasks is who "me" is for the My Tasks filter: one identity per tracker
// whose personal credential was confirmed, and the account's name and e-mail
// for every other source, local tickets included.
type MyTasks struct {
    ByTracker map[string]string // "github" → "sebastienferry"
    Fallback  []string          // name, e-mail
}
```

- `TaskScope` gains `Mine *MyTasks`: nil means no My Tasks filter. Kept on the scope
  rather than as a thirteenth positional parameter of `GetTasksInScope` (3 callers).
- Condition appended by `GetTasksInScope` when `scope.Mine != nil`, values trimmed,
  lowercased in Go (`strings.ToLower`), deduplicated, empties dropped, trackers sorted
  for a stable query:

```sql
(   (source = 'github' AND LOWER(TRIM(assignee)) IN (?))
 OR (source = 'jira'   AND LOWER(TRIM(assignee)) IN (?))
 OR (source NOT IN ('github','jira') AND LOWER(TRIM(assignee)) IN (?, ?)) )
```

  With no tracker identity the last branch has no `NOT IN`. With no identity at all
  the condition is `1 = 0`. `mine` and `assignee` both present combine with `AND`.
- Engines: PostgreSQL's `LOWER` is Unicode-aware; SQLite's folds A-Z only, so `ÉRIC`
  does not match `éric` on SQLite. Accepted, as in #447 (SQLite is being retired).
- New `TaskSourcesInScope(scope TaskScope) ([]string, error)`: `SELECT DISTINCT source`
  over the scope conditions only (`taskScopeUnsafe`), for the tooltip.

### 5. HTTP

- `GET /api/tasks`: `mine=1` (or `true`) → `scope.Mine = h.myTasks(r)`.
- `h.myTasks(r)` (`handlers`, new file `mytasks.go`): `userID := h.webSessionUser(r)`;
  `settings := h.composedSettings(userID)` gives `UserName` (the account `Name()`, or
  the local profile when signed out) and `UserEmail`; `ByTracker` from
  `h.db.TrackerAccounts(userID)` when `userID != ""`.
- New `GET /api/me/assignee-identities?projectId=|viewId=` (register in
  `cmd/server/main.go`), same scope rules as `/api/tasks` (404 on a foreign view):

```json
{
  "signedIn": true,
  "fallback": ["Sébastien F.", "sferry@example.com"],
  "trackers": [
    {"tracker": "github", "identity": "sebastienferry", "known": true},
    {"tracker": "jira", "known": false}
  ]
}
```

  `trackers` lists the non-local sources in scope. Not under
  `HandleUserTrackerCredentials`, which answers 401 to a signed-out caller.
- `docs/API_AND_DATA_SPEC.md`: add `mine=1` to the `/api/tasks` row, the new route, and
  `account` to the tracker-credentials GET description.

### 6. Web client

- `AppContext.tsx`:
  - new state `myTasksOnly: boolean` and `setMyTasksOnly(value)`, persisted as
    `{ mine: value ? '1' : null }` with the other per-scope filters; turning it on also
    clears the person filter (state and storage). `setAssigneeFilter(non-null)` turns it
    off.
  - restore effect: `setMyTasksOnlyState(stored.mine === '1')`; `stored.assignee` keeps
    restoring the person filter (spec FR 11).
  - `buildTaskQuery`: `if (myTasksOnly) params.append('mine', '1')`, in the dependency
    list.
  - the facet guard is untouched: it only looks at `assigneeFilter`, which My Tasks no
    longer sets (spec FR 10).
  - `myTasksIdentities`: fetched from `/api/me/assignee-identities` with the scope
    params when the project or view changes, and again after the personal credentials
    are saved, deleted or verified (the existing refresh around `AppContext.tsx:1336`).
- `web/src/lib/myTasks.ts` (new, pure): `myTasksTooltip(identities, t)` returns the
  button's title: the label alone, or the label plus the fallback sentence naming the
  unknown trackers, or the signed-out sentence.
- `Sidebar.tsx`: `isMyTasksActive = myTasksOnly`, `onClick={() => setMyTasksOnly(!myTasksOnly)}`,
  `title={myTasksTooltip(...)}`; the status shortcuts (line 906-915) add
  `&& !myTasksOnly` to `isActive` and `setMyTasksOnly(false)` to `onClick`, next to
  `setAssigneeFilter(null)` (spec FR 8).
- `Header.tsx`: `hasActiveFilters` includes `myTasksOnly`; a chip labelled
  `t.nav.myTasks` whose × calls `setMyTasksOnly(false)`, next to the person chip
  (line 183). There is no clear-all action: each chip clears its own filter.
- `RoadmapView.tsx`: same chip in `activeFilterChips`.
- `locales/translations.ts`: `nav.myTasksFallback` ("Sur {trackers}, votre nom et votre
  e-mail sont utilisés : enregistrez ou vérifiez un accès personnel dans votre profil."
  / "On {trackers}, your name and e-mail are used: save or verify a personal credential
  in your profile.") and `nav.myTasksSignedOut` (FR 13), French and English.
  The old `'nom non renseigné dans le profil'` fallback is removed.

## Tests

- `internal/db/mytasks_test.go` (SQLite, and PostgreSQL when
  `SECTILE_TEST_POSTGRES_DSN` is set): GitHub login match; Jira display name match and
  account name non-match; local name and e-mail; trim and case; fallback without a
  tracker identity; mixed "All projects"; a saved view; `mine` AND another filter;
  no identity → no ticket; `TaskSourcesInScope`.
- `internal/db/usercredentials_test.go`: save resets the account; set account; sealed
  and locked row still reported by `TrackerAccounts`; clear forgets it; migration 22
  on an existing database. The new column must also be dropped by the rewind-test
  helpers (`dropRepositoryColumns`-style), or the db rewind tests fail.
- `internal/handlers/mytasks_test.go` with an `httptest` fake tracker: PUT records the
  account from `/user` and `/myself`; a failing probe still saves with no account;
  `/api/setup/tracker/check` with the stored token records, with a typed token does
  not; `GET /api/tasks?mine=1` makes zero requests to the fake; the identities route,
  signed in and signed out, and 404 on a foreign view.
- `web/tests/myTasks.test.mjs`: `myTasksTooltip` in its three cases.
- `web/tests/board-views.browser.mjs`: S8 and S12 assert `mine=1` and
  `localStorage…mine === '1'` instead of `assignee=Alice`; stub
  `/api/me/assignee-identities`; a view whose storage holds `assignee: 'Alice'` still
  sends `assignee=Alice` and the button is not active; a status shortcut clicked
  with My Tasks on drops `mine=1` from the next request.

## Changelog

`CHANGELOG.md`, `## [Unreleased]` → `### Fixed`: "My Tasks now finds the tickets
assigned to you on GitHub and Jira (#468)."

## Risks

- Credentials saved before this change stay on the fallback until verified or saved
  again; the tooltip says so. A background probe at startup was rejected: it would
  reach the trackers without anyone acting, and cannot open sealed rows.
- Jira homonyms are both "me" (display name, not accountId), out of scope.
