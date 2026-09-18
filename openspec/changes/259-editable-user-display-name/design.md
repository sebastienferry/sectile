# Design

## Decisions

### The chosen name is a column of its own, not a rewrite of `display_name`
`users.display_name` is the name the provider — or the local sign-in — last supplied, and
`UpsertUser` rewrites it at every sign-in. A chosen name written into that same column would
be erased at the next visit, which the second requirement forbids. A new column carries the
choice:

```sql
ALTER TABLE users ADD COLUMN chosen_name TEXT NOT NULL DEFAULT '';
```

added next to the existing in-place `ALTER TABLE` statements of `initIdentitySchema`, which
is how `role` and `last_sign_in` were introduced. `User.DisplayName` — the field every
reader already uses — becomes `COALESCE(NULLIF(chosen_name, ''), display_name)`, resolved in
`userColumns` and `scanUser`. Nothing downstream changes: `/api/me`, `/api/users`,
`ownerDisplayName` and the users panel keep reading `DisplayName` and simply start seeing
the chosen value.

### The provider keeps the e-mail, the person keeps the name
`UpsertUser` is unchanged: it keeps writing `email` and `display_name` from the provider on
every sign-in. Because the chosen name lives in another column and wins the `COALESCE`, the
claim is recorded and simply not shown. A person who clears their chosen name falls straight
back to the provider's spelling.

### One route, the caller's own account
`PATCH /api/me`, served by `HandleCurrentUser` alongside its `GET`. It is not added to
`publicPath()` — `/api/me` is public for `GET` only, so the handler refuses a `PATCH`
without a session. It takes `{"displayName": "…"}` and nothing else, so no other account
attribute can be written through it, and no route renames another account.

### Validation lives in the store
`NormalizeDisplayName(name) (string, error)` next to `NormalizeRole` and
`NormalizeLocalEmail` in `internal/db/roles.go`: trims, refuses a control character or a
line break, refuses more than `MaxDisplayNameLength = 80` runes. The empty string is valid
and means "no choice". The handler maps the two errors to `400`.

### The interface edits the account, not the legacy settings
The account section (`SignInStatus.tsx`) gains the field, because that is where the
signed-in identity already is. The legacy `settings.userName` block stays hidden behind
`hasAccount` and is untouched; it is only seeded so the sidebar's "My tasks" filter is not
left empty for a new account.

## Rejected alternatives

- **Write the chosen name into `display_name` and make `UpsertUser` fill it only when
  empty.** Works for the local sign-in, and quietly freezes the provider's name for
  everyone else, so a change at the provider would stop propagating.
- **A `user_settings.display_name` personal setting.** The name is an attribute of the
  account, read by the users panel and the activity log, both of which join on `users`.
  Putting it in the settings store would force a second join into three queries in `db.go`
  for no gain.
- **Extend `POST /api/settings`.** That endpoint routes keys between two *settings* stores;
  adding a `users` write into it would make the routing table lie about what it owns.

## Data contracts

| Column | Written by | Read as |
| --- | --- | --- |
| `users.display_name` | `UpsertUser`, at every sign-in | fallback of `User.DisplayName` |
| `users.chosen_name` | `SetDisplayName` only | `User.DisplayName` when not empty |

```
PATCH /api/me
  body: {"displayName": "Sébastien Ferry"}
  200:  the same body as GET /api/me, with the new displayName
  400:  {"error": "A display name is at most 80 characters"}
        {"error": "A display name cannot contain line breaks"}
  401:  {"error": "Sign in to use this interface"}
  405:  any other method
```

`GET /api/me`, `GET /api/users` and `PUT /api/users/{id}` keep their current shape; only the
value of `displayName` can differ. `CurrentUser` in `web/src/lib/session.ts` is unchanged —
it already carries `displayName`; `useCurrentUser` gains `rename(name)`.
