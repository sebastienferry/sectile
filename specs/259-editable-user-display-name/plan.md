# #259 — Implementation plan

## Stack

Unchanged: Go server (`internal/handlers`, `internal/db`, SQLite through `d.conn`),
React + TypeScript interface (`web/src`), Electron desktop untouched.

## Architecture decisions

### D1 — The chosen name is a column of its own, not a rewrite of `display_name`

`users.display_name` is the name the *provider* (or the local sign-in) last gave. It is
rewritten at every sign-in by `UpsertUser`. A chosen name written into the same column
would be erased on the next visit, which US2 forbids.

A new column carries the choice:

```sql
ALTER TABLE users ADD COLUMN chosen_name TEXT NOT NULL DEFAULT '';
```

added next to the existing in-place `ALTER TABLE` statements of `initIdentitySchema`
(`internal/db/identity.go`), which is how `role` and `last_sign_in` were introduced.

`User.DisplayName` — the field every reader already uses — becomes
`COALESCE(NULLIF(chosen_name, ''), display_name)`, resolved in `userColumns` /
`scanUser`. Nothing downstream changes: `/api/me`, `/api/users`, `ownerDisplayName` and the
users panel keep reading `DisplayName` and simply start seeing the chosen value.

Rejected alternative: making `UpsertUser` write `display_name` only when it is empty. It
would work for the local sign-in and quietly freeze the provider's name for everyone else,
so an e-mail change at the provider would stop propagating.

Rejected alternative: a `user_settings.display_name` personal setting. The name is an
attribute of the account, read by the users panel and the activity log, both of which join
on `users`; putting it in the settings store would force a second join into three queries
in `db.go` for no gain.

### D2 — The provider keeps the e-mail, the person keeps the name

`UpsertUser` is unchanged: it keeps writing `email` and `display_name` from the provider on
every sign-in. Because the chosen name lives in another column and wins the `COALESCE`, the
claim is recorded and simply not shown (OQ3). A person who clears their chosen name falls
straight back to the provider's spelling.

### D3 — One route, the caller's own account

`PATCH /api/me` (new), served by `HandleCurrentUser` in `internal/handlers/auth.go`
alongside its `GET`:

- it is **not** added to `publicPath()`; `/api/me` is public only for `GET`, so the handler
  refuses a `PATCH` without a session with `401` and the message
  `Sign in to use this interface`, the wording ADR 0015 already uses;
- it takes `{"displayName": "…"}` and nothing else, so no other account attribute can ever
  be written through it;
- there is no route that renames another account (US5, OQ2).

Rejected alternative: extending `POST /api/settings`. The settings endpoint routes keys
between two *settings* stores (#256, D2); adding a `users` write into it would make the
routing table lie about what it owns.

### D4 — Validation lives in the store, not in the handler

`db.NormalizeDisplayName(name) (string, error)` next to `NormalizeRole` and
`NormalizeLocalEmail` in `internal/db/roles.go`: trims the value, refuses a control
character or a line break (`ErrDisplayNameInvalid`), refuses more than
`MaxDisplayNameLength = 80` runes (`ErrDisplayNameTooLong`). Counted in runes, so an
accented name is not shorter than an unaccented one. The empty string is valid and means
"no choice" (US3). The handler maps the two errors to `400`.

### D5 — The interface edits the account, not the legacy settings

The account section (`web/src/components/SignInStatus.tsx`) gains the field, because that
is where the signed-in identity already is and where the person looks. The legacy
`settings.userName` block stays hidden behind `hasAccount` and is untouched (OQ1); `T9`
only seeds it so the "My tasks" filter is not left empty for a new account.

`useCurrentUser` gains a `rename(name)` that `PATCH`es and refreshes its state, so the
account section updates without a reload (US1).

## Data contracts

### Storage

| Column | Written by | Read as |
| --- | --- | --- |
| `users.display_name` | `UpsertUser`, at every sign-in | fallback of `User.DisplayName` |
| `users.chosen_name` | `SetDisplayName` only | `User.DisplayName` when not empty |

New store function, `internal/db/roles.go`:

```go
// SetDisplayName records the name its owner chose. An empty value clears the
// choice and hands the account back to the provider's spelling.
func (d *DB) SetDisplayName(id, name string) (*User, error)
```

`sql.ErrNoRows` for an unknown id.

### HTTP

```
PATCH /api/me
  body:     {"displayName": "Sébastien Ferry"}
  200:      the same body as GET /api/me, with the new displayName
  400:      {"error": "A display name is at most 80 characters"}
            {"error": "A display name cannot contain line breaks"}
  401:      {"error": "Sign in to use this interface"}
  405:      any other method
```

`GET /api/me`, `GET /api/users` and `PUT /api/users/{id}` keep their current shape; only the
value of `displayName` can differ.

### Web

`CurrentUser` in `web/src/lib/session.ts` is unchanged (it already carries `displayName`).
`useCurrentUser` returns `rename: (name: string) => Promise<void>` in addition to `reload`.

## Target files

| File | Change |
| --- | --- |
| `internal/db/identity.go` | `chosen_name` column, `userColumns` / `scanUser` COALESCE |
| `internal/db/roles.go` | `NormalizeDisplayName`, `MaxDisplayNameLength`, the two errors, `SetDisplayName` |
| `internal/db/roles_test.go` | store tests |
| `internal/handlers/auth.go` | `PATCH` branch of `HandleCurrentUser` |
| `internal/handlers/auth_test.go` | route tests |
| `web/src/hooks/useCurrentUser.ts` | `rename` |
| `web/src/components/SignInStatus.tsx` | the field, its save button and its status line |
| `web/src/locales/translations.ts` | the labels, both locales |
| `docs/adrs/` | no ADR: this follows ADR 0015 rather than amending it |
