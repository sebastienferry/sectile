# #348 — Technical plan

Behaviour and acceptance criteria live in [`spec.md`](./spec.md). This file records the
implementation choices only.

## Stack and constraints

- Go server under `internal/`, `gofmt`-clean (`make fmt-check` gates it), tested with
  `go test ./...`.
- React 19 + TypeScript under `web/src`, built by `tsc -b && vite build`, linted by
  `oxlint`, tested by `node --test tests/*.test.mjs`. The `web/tests` suites read source
  files as text and assert on them — no build step, no DOM.
- `make test` runs `fmt-check`, the Go suites and the web suites; it is the gate.
- No database migration and no schema change: `users.chosen_name` and
  `user_settings.user_name` both already exist.
- The HTTP shape does not change (ADR 0015): `GET /api/settings` keeps answering one object
  with `userName` in it, and a `POST` keeps accepting the whole object.

## Shape of the change

```
internal/db/identity.go          [A] User.Name() — already the chain, read only
                                  |     DisplayName -> Email -> ID
                                  |     (DisplayName is COALESCE(NULLIF(chosen_name,''), display_name))
                                  v
internal/handlers/handlers.go    [B] composedSettings: inside the existing GetUser block,
                                  |     composed.UserName = user.Name()
                                  [C] HandleSettings: delete(sent, "userName")
                                  |     beside delete(sent, "userEmail")
                                  v
internal/handlers/authz_test.go  [D] the symmetric write-is-ignored assertion
internal/db/identity_test.go     [E] the fallback chain, incl. both-names-empty
                                  v
web/src/context/AppContext.tsx   [F] userName: '' instead of 'Developer'
web/src/components/Sidebar.tsx   [G] drop 'SF', 'Sylvain Ferry', 'Paramètres & Profil'
web/src/components/StatusBar.tsx [H] drop 'Profile'
web/src/components/ProfileModal.tsx [I] stop sending userName
web/tests/identityProjection.test.mjs [J] pins F..I as absences
                                  v
docs/adrs/0015-...md             [K] the projection paragraph covers userName
CHANGELOG.md                     [L] one Fixed line under [Unreleased]
```

## Decisions

### D1 — The projection is set on the server, in `composedSettings`

One assignment next to the `userEmail` one, inside the `if user, err := h.db.GetUser(userID)`
block that already runs. The alternative — having `Sidebar` and `StatusBar` read
`user.displayName` — costs either two more `/api/me` fetches or plumbing the current user
through `AppContext`, which does not carry it today (`App.tsx:124` is the single
`useCurrentUser()` mount). One server line against a front-end refactor.

Rejected: resolving in `db.UserSettings`. That function is the storage read, shared with the
write path's merge; making it answer a value nothing stored would make a save round-trip
write the projected name back into the column.

### D2 — The chain is `User.Name()`, not a new helper

`internal/db/identity.go:420` already reads:

```go
func (u User) Name() string {
    switch {
    case strings.TrimSpace(u.DisplayName) != "":
        return u.DisplayName
    case strings.TrimSpace(u.Email) != "":
        return u.Email
    }
    return u.ID
}
```

`DisplayName` is already `COALESCE(NULLIF(chosen_name, ''), display_name)` by `userColumns`,
so `user.Name()` *is* the settled chain `chosen_name -> display_name -> email -> userId`,
trimmed at each step. Writing a second copy in the handler would be the same chain in two
places, free to diverge.

The asymmetry with the e-mail line is deliberate: `composed.UserEmail` is guarded by
`user.Email != ""` because an account without an address must keep whatever the personal row
holds, whereas `user.Name()` never returns empty for a real account, so `composed.UserName`
is assigned unconditionally inside the same block.

### D3 — The write is dropped from the raw payload, not from the key map

`delete(sent, "userName")` goes immediately after `delete(sent, "userEmail")`, i.e. **before**
`memberSettingsViolations` runs. `sent` is the raw `map[string]json.RawMessage`, and both the
authorization check and `memberSettingsPayload` read it, so deleting there makes the field
invisible to the write without touching either.

Rejected: removing `userName` from `personalSettingsKeys` (`authz.go:147`). That map doubles
as the member-authorization list, so a member posting the whole settings row — which the
interface does — would get a `403` naming `userName` instead of a silent ignore. US3 forbids
that.

The typed `req models.Settings` is left alone: nothing downstream of the raw-map routing reads
`req.UserName`, and `PersonalSettings` keeps the column in its projection so the stored value
is preserved rather than blanked.

### D4 — The front end keeps reading `settings.userName`

No component changes what it reads. `Sidebar` and `StatusBar` keep rendering
`settings.userName`; only their `||` fallbacks go, because the server now guarantees a
non-empty value for a signed-in caller and the fallbacks were a maintainer's own name
(`'Sylvain Ferry'`, `'SF'`) and two UI strings (`'Paramètres & Profil'`, `'Profile'`) shipped
as defaults.

The initials keep `settings.userName.substring(0, 2).toUpperCase()` behind the existing
truthiness guard so an empty pre-fetch render produces an empty circle rather than a crash.

`AppContext`'s default becomes `userName: ''` — it is a pre-fetch placeholder, and
`'Developer'` is the exact string the ticket reports.

`ProfileModal.handleSave` stops sending `userName`. It keeps sending `userEmail` exactly as it
does today, which the server already ignores; changing that line is not this ticket's.

### D5 — Two Go tests and one web test

- `internal/handlers/authz_test.go`, beside the existing `userEmail` block: a member posts
  `{"userName":"someone else"}`, gets `200`, and the following `GET` answers the account's own
  name. Same shape as the assertion one block above it.
- `internal/db/identity_test.go`: a table over `User.Name()` covering the chosen name, the
  provider name, both-empty falling to the e-mail (the `default` account's shape) and
  everything-empty falling to the id, with whitespace-only values treated as empty.
  `SignInLocal` writes the address into `display_name`, so the e-mail rung is reached through
  a `User` value rather than through a signed-in fixture.
- `web/tests/identityProjection.test.mjs`: reads `Sidebar.tsx`, `StatusBar.tsx`,
  `AppContext.tsx` and `ProfileModal.tsx` as text and asserts the four literals are absent and
  that `handleSave` no longer sends `userName`. That is the convention every existing suite in
  `web/tests` follows.

### D6 — ADR 0015 is amended in place

Its *"`userEmail` is a projection"* paragraph is extended to name `userName`, and the
personal-half list marks the two as projections rather than typed fields. The record gains a
sentence saying this completes the ADR 0013 retirement it already invoked; nothing is
reversed, so no superseding ADR is written.

## Risks

- **An agent that sets `userName` through `/api/settings` or MCP silently stops having an
  effect.** That is the intended contract change; it is announced in `CHANGELOG.md` and
  recorded in the ADR. The call keeps answering `200`, so nothing breaks loudly.
- **The "My Tasks" filter's match changes** when `userName` starts resolving to the account
  name. It is already unreliable (tracker login vs display name) and is explicitly out of
  scope; the separate issue carries it.
