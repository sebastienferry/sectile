# Tasks

- [ ] Add the `chosen_name` column to `initIdentitySchema` (`internal/db/identity.go`), next to the existing in-place migrations, with the comment saying why it is not `display_name`.
- [ ] Resolve `User.DisplayName` as `COALESCE(NULLIF(chosen_name, ''), display_name)` in `userColumns` and `scanUser`, so every existing reader picks the chosen name up unchanged.
- [ ] Add `MaxDisplayNameLength`, `ErrDisplayNameTooLong`, `ErrDisplayNameInvalid` and `NormalizeDisplayName` to `internal/db/roles.go` (trim, refuse control characters and line breaks, count runes, empty is valid).
- [ ] Add `SetDisplayName(id, name)` to `internal/db/roles.go`: normalize, update `chosen_name`, return the reloaded user, `sql.ErrNoRows` for an unknown id.
- [ ] Cover the store in `internal/db/roles_test.go`: the chosen name wins, an empty one falls back, and a sign-in after a rename keeps the chosen name.
- [ ] Add the `PATCH` branch to `HandleCurrentUser` (`internal/handlers/auth.go`): refuse an anonymous caller with `401`, decode behind a `MaxBytesReader`, map the two validation errors to `400`, answer the `GET` body.
- [ ] Cover the route in `internal/handlers/auth_test.go`: anonymous `401`, a successful rename, 81 runes, a line break, an empty name, the renamed account in `GET /api/users`, and no route renaming another account.
- [ ] Add `rename(name)` to `web/src/hooks/useCurrentUser.ts`, and seed `settings.userName` with the account name only when it is still empty.
- [ ] Add the display name field, its save button and its status line to the signed-in branch of `web/src/components/SignInStatus.tsx`.
- [ ] Add the label, the placeholder, the hint and the two error messages to both locales of `web/src/locales/translations.ts`.
- [ ] Leave the legacy `settings.userName` block of `ProfileModal.tsx` hidden and unwired.
- [ ] Document `PATCH /api/me` and the `chosen_name` column in `docs/API_AND_DATA_SPEC.md`.
- [ ] Run `go build ./...`, `go test ./...` and the web build, and record the output.
