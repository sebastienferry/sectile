# #256 — Implementation plan

## Stack

Unchanged: Go 1.x server (`internal/handlers`, `internal/db`, SQLite through `d.conn`),
React + TypeScript interface (`web/src`), Electron desktop untouched.

## Architecture decisions

### D1 — The implicit mode is removed, not disabled

`modeImplicit` and the two fallbacks that depend on it go together, otherwise a half
removal leaves a path that still resolves an anonymous request to an admin.

- `internal/handlers/authz.go`: delete `modeImplicit`, `signInMode()` returns
  `auth.ModeOIDC` or `auth.ModeLocal`; `principalFor` loses the `p.UserID == ImplicitUser`
  admin shortcut, so `default` resolves through `GetUser` like any account and keeps its
  stored role.
- `internal/handlers/pairing.go`: `webSessionUser()` returns `""` after the cookie and the
  API key both fail.
- `internal/handlers/auth.go`: `RequireSession` drops `|| h.signInMode() == modeImplicit`.
  `publicPath()` is unchanged — it already names exactly the list the ticket asks for.
- `ImplicitUser` is kept **only** as the owner of the requestless entry points
  (`LaunchTaskExternalTerminal`, `agent_operations.go:23`, `agent_api.go:138`) and renamed
  in place is not required; what matters is that no HTTP path reaches it. `db.ImplicitUserID`
  stays: the `default` row must remain creatable and readable.

### D2 — Two stores, one API shape

The interface posts and reads the whole `models.Settings` object in a dozen components.
Changing that shape would spread this ticket across the whole front end for no behavioural
gain. So:

- storage is split in two: the existing `settings` row keeps the deployment columns, a new
  `user_settings` table holds the personal ones;
- `GET /api/settings` composes the answer: deployment columns from the global row, personal
  columns from the caller's row, `userEmail` from `users.email`;
- `POST/PUT /api/settings` routes each key: a personal key is written to the caller's row,
  a deployment key to the global row after the admin check that already exists.

`memberSettingsKeys` in `authz.go` already enumerates exactly the personal set minus the
two workstation commands; it becomes the single source of truth for the routing and gains
`editorCommand` and `externalTerminalCommand`.

Rejected alternative: a `user_id` column on `settings` with one row per user. It would
duplicate the twenty deployment columns per account and leave two writable copies of the
tracker configuration, which is the bug this ticket exists to remove.

### D3 — Seeding, not migrating

`user_settings` is created empty. `UserSettings(userID)` returns the stored row if there is
one, otherwise the personal projection of the global row, and writes it on first save. No
data migration runs at startup: an existing deployment therefore keeps showing exactly what
it showed before, per account, from the first render.

### D4 — `userEmail` is a projection

`GetUserSettings` overwrites `UserEmail` with `users.email` for the account. On a write the
incoming `userEmail` is ignored. The column stays in `user_settings` only to avoid a
special case in the JSON round-trip; nothing reads it.

### D5 — Workstation commands follow the execution's owner

`launchTaskExternalTerminal(ctx, userID, …)` and `HandleOpenEditor` read
`h.db.UserSettings(userID)` for `ExternalTerminalCommand` / `EditorCommand` instead of
`GetSettings()`. The project-level override keeps priority, then the owner's value, then
the deployment default. The AI configuration is untouched and keeps reading `GetSettings()`
(`applySkillCommandOverride`, `applyProjectSettings`, `internal/db/agentconfig.go`).

### D6 — The passphrase is a client-side loop

`POST /api/me/tracker-credentials/unlock` already answers the full credential list, so the
sign-in screen calls it once per sealed tracker and reports a single aggregated message. No
server change, no new endpoint, and the wrong-passphrase answer (`403`) is already the
uniform one ADR 0014 wants. The sign-in itself is never failed by it.

## Data contracts

### New table

```sql
CREATE TABLE IF NOT EXISTS user_settings (
  user_id TEXT PRIMARY KEY,
  theme TEXT NOT NULL DEFAULT 'dark',
  accent_color TEXT NOT NULL DEFAULT 'indigo',
  language TEXT NOT NULL DEFAULT 'fr',
  density TEXT NOT NULL DEFAULT 'standard',
  default_view TEXT NOT NULL DEFAULT 'board',
  detail_mode TEXT NOT NULL DEFAULT 'panel',
  ui_scale INTEGER NOT NULL DEFAULT 100,
  user_name TEXT NOT NULL DEFAULT '',
  user_email TEXT NOT NULL DEFAULT '',
  user_avatar TEXT NOT NULL DEFAULT '',
  editor_command TEXT NOT NULL DEFAULT '',
  external_terminal_command TEXT NOT NULL DEFAULT '',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

The personal columns are **left in place** in `settings`: they are the seed of D3 and
dropping them would need a table rebuild for no gain. They stop being written by the API.

### Store API (`internal/db/usersettings.go`, new file)

- `UserSettings(userID string) (*models.Settings, error)` — personal fields, seeded from
  the global row when the account has no row yet.
- `UpdateUserSettings(userID string, s models.Settings) (*models.Settings, error)` —
  upsert of the personal columns only.

### HTTP

No route is added or removed. `/api/settings` keeps its payload shape; the difference is
where each key lands. `/api/me` is unchanged except that `mode` can no longer be
`implicit`.

### Web

`SignInMode` in `web/src/lib/session.ts` loses `'implicit'`; `needsSignIn(user)` becomes
`!!user && !user.signedIn`. `describeSignInMode` loses its implicit branch. `App.tsx` is
unchanged in shape — it already refuses to mount the board when `needsSignIn` is true.
`SignInScreen.tsx` gains an optional passphrase field and posts the e-mail as it does
today, then loops the unlock calls before redirecting.

## Target files

| File | Change |
| --- | --- |
| `internal/handlers/authz.go` | remove `modeImplicit`, the implicit branch of `signInMode`/`principalFor`, extend `memberSettingsKeys` |
| `internal/handlers/auth.go` | `RequireSession` without the implicit bypass |
| `internal/handlers/pairing.go` | `webSessionUser` returns `""` |
| `internal/handlers/handlers.go` | `HandleSettings` composes/routes; terminal and editor launch read the owner's settings |
| `internal/db/usersettings.go` | new store |
| `internal/db/db.go` | `user_settings` DDL in the schema list |
| `internal/models/models.go` | comment on `UserEmail` as a projection (no shape change) |
| `web/src/lib/session.ts` | `SignInMode` without `implicit`, `needsSignIn` |
| `web/src/components/SignInScreen.tsx` | optional passphrase field, unlock loop, wording |
| `docs/adrs/0015-*.md` | new ADR superseding the two ADR 0013 decisions |

## Test plan

- Go unit tests next to the code they cover (`internal/handlers/*_test.go`,
  `internal/db/usersettings_test.go`): anonymous `401` on an interface route, public paths
  served, API key still resolving its user, `default` keeping its stored role, two accounts
  reading different personal settings, seeding from the global row, member refused on a
  deployment key and allowed on a personal one, `userEmail` not writable, owner's terminal
  command used at launch.
- Web tests in `web/tests/*.test.mjs` (`node --test`, the suite `npm test` runs) for
  `needsSignIn` and the sign-in redirect rules.
- `go build ./... && go test ./...`, and in `web/`: `npm run lint`, `npm run build`,
  `npm test`. A test that loads `dist/index.html` needs the build to have run first.
