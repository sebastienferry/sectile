# #259 — Implementation checklist

Ordered so the column exists before anything resolves it, the server is coherent before
the interface offers the field, and the tests of each layer land with that layer.

## 1. Storage

- [ ] **T1** `internal/db/identity.go`: add
      `ALTER TABLE users ADD COLUMN chosen_name TEXT NOT NULL DEFAULT '';` next to the
      existing in-place migrations of `initIdentitySchema`, with the comment saying why it
      is not `display_name` (D1).
- [ ] **T2** Resolve `User.DisplayName` as `COALESCE(NULLIF(chosen_name, ''), display_name)`
      in `userColumns` and `scanUser`, so every existing reader picks the chosen name up
      without changing.
- [ ] **T3** `internal/db/roles.go`: `MaxDisplayNameLength = 80`, `ErrDisplayNameTooLong`,
      `ErrDisplayNameInvalid` and `NormalizeDisplayName` (trim, refuse control characters
      and line breaks, count runes, empty is valid) (D4).
- [ ] **T4** `internal/db/roles.go`: `SetDisplayName(id, name)` — normalize, update
      `chosen_name`, return the reloaded `*User`, `sql.ErrNoRows` for an unknown id.
- [ ] **T5** `internal/db/roles_test.go`: a chosen name wins over `display_name`; an empty
      chosen name falls back; `SignInLocal` then `SetDisplayName` then `SignInLocal` again
      keeps the chosen name (US2); `ListUsers` still orders admins first.

## 2. Server

- [ ] **T6** `internal/handlers/auth.go`: `HandleCurrentUser` gains a `PATCH` branch —
      `webPrincipal`, refuse an anonymous caller with `401`
      `Sign in to use this interface`, decode `{"displayName"}` behind a
      `http.MaxBytesReader` of 4096 as the neighbouring handlers do, map
      `ErrDisplayNameTooLong` / `ErrDisplayNameInvalid` to `400` with the messages of the
      spec, and answer the same body as the `GET` (D3).
- [ ] **T7** Keep `publicPath()` as it is and make sure the `PATCH` cannot ride on the
      public `GET`: the refusal is in the handler, and a test proves it.
- [ ] **T8** `internal/handlers/auth_test.go`: anonymous `PATCH` → `401`; signed-in rename →
      `200` and the new name in the body and in a following `GET /api/me`; a name of 81
      runes → `400`; a name with `\n` → `400`; an empty name → `200` and `displayName`
      back to the e-mail; the renamed account shown with its new name by
      `GET /api/users` (US6); no route renames another account (US5).

## 3. Interface

- [ ] **T9** `web/src/hooks/useCurrentUser.ts`: add `rename(name)` — `PATCH /api/me`, set
      the returned user on success, surface the server message on `400`. Seed
      `settings.userName` with the account name when it is still empty, and do not touch it
      otherwise (D5, OQ1).
- [ ] **T10** `web/src/components/SignInStatus.tsx`: in the signed-in branch, a
      **Display name** input filled from `user.displayName`, a save button disabled while
      the value is unchanged or while the call is in flight, and the existing `role="status"`
      line reporting the outcome. Leave the sign-out button, the role line and the local
      sign-in warning as they are.
- [ ] **T11** `web/src/locales/translations.ts`: the label, the placeholder, the hint
      (`Your name on this board. Clear it to go back to your e-mail address.`) and the two
      error messages, in both locales (NFR3).
- [ ] **T12** Leave the legacy `settings.userName` block of `ProfileModal.tsx` hidden and
      unchanged; do not wire it to the new field (OQ1).

## 4. Documentation

- [ ] **T13** `docs/API_AND_DATA_SPEC.md`: add `PATCH /api/me` and the `chosen_name` column.
- [ ] **T14** `CHANGELOG.md` if the repository carries one: one line under Added.

## Test plan

| Level | What it proves | Where |
| --- | --- | --- |
| Store | the fallback chain, the survival of a sign-in, the validation rules | `internal/db/roles_test.go` (T5) |
| Handler | the four status codes, the payload shape, the absence of a cross-account route | `internal/handlers/auth_test.go` (T8) |
| Interface | manual: rename, reopen the profile, sign out and back in | — |

Gates before the pull request: `go build ./...`, `go test ./...`, and the web build
(`npm run build` in `web/`) since `SignInStatus.tsx` and the locales are typed.

## Blocked by an open question

Nothing in this checklist is blocked. OQ2 (an admin renaming someone else) would add a
branch to `PUT /api/users/{id}` and a control to `UsersPanel.tsx`; OQ1 (converging with
`settings.userName`) would rewrite T9 and T12. Both are deliberately out of this list.
