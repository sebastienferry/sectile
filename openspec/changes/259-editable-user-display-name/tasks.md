# Tasks

- [x] Add the `chosen_name` column to `initIdentitySchema` (`internal/db/identity.go`), next to the existing in-place migrations, with the comment saying why it is not `display_name`.
- [x] Resolve `User.DisplayName` as `COALESCE(NULLIF(chosen_name, ''), display_name)` in `userColumns` and `scanUser`, so every existing reader picks the chosen name up unchanged.
- [x] Add `MaxDisplayNameLength`, `ErrDisplayNameTooLong`, `ErrDisplayNameInvalid` and `NormalizeDisplayName` to `internal/db/roles.go` (trim, refuse control characters and line breaks, count runes, empty is valid).
- [x] Add `SetDisplayName(id, name)` to `internal/db/roles.go`: normalize, update `chosen_name`, return the reloaded user, `sql.ErrNoRows` for an unknown id.
- [x] Cover the store in `internal/db/roles_test.go`: the chosen name wins, an empty one falls back, and a sign-in after a rename keeps the chosen name.
- [x] Add the `PATCH` branch to `HandleCurrentUser` (`internal/handlers/auth.go`): refuse an anonymous caller with `401`, decode behind a `MaxBytesReader`, map the two validation errors to `400`, answer the `GET` body.
- [x] Cover the route in `internal/handlers/auth_test.go`: anonymous `401`, a successful rename, 81 runes, a line break, an empty name, the renamed account in `GET /api/users`, and no route renaming another account.
- [x] Add `rename(name)` to `web/src/hooks/useCurrentUser.ts`. The seeding of `settings.userName` was dropped: it defaults to `Developer` in `AppContext`, so "still empty" never holds, and coupling the two settings is exactly what OQ1 leaves open.
- [x] Add the display name field, its save button and its status line to the signed-in branch of `web/src/components/SignInStatus.tsx`.
- [x] Label and hint written as English literals in `SignInStatus.tsx` rather than added to `web/src/locales/translations.ts`: that component carries no translation at all today (`Account`, `Signed in as`, `Sign out`), and wiring `useApp()` into it for two strings would make it the only translated part of an untranslated section. The two error messages come from the server, which answers in English like every other API message.
- [x] Leave the legacy `settings.userName` block of `ProfileModal.tsx` hidden and unwired.
- [x] Document `PATCH /api/me` and the `chosen_name` column in `docs/API_AND_DATA_SPEC.md`.
- [x] Run `go build ./...`, `go test ./...` and the web build, and record the output.
