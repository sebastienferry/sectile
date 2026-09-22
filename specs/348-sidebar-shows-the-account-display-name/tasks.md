# #348 — Implementation checklist

Ordered so the server answers the right value before anything front-end stops guarding
against the wrong one. References: [`spec.md`](./spec.md), [`plan.md`](./plan.md).

## 1. Server — the projection on read (D1, D2, US1, US2)

- [x] T1.1 `internal/handlers/handlers.go`, `composedSettings`: inside the existing
      `if user, err := h.db.GetUser(userID); err == nil && user != nil { ... }` block, add
      `composed.UserName = user.Name()`. Restructure the condition so the e-mail keeps its
      `user.Email != ""` guard while the name is assigned unconditionally (D2).
- [x] T1.2 Update the doc comment above `composedSettings` so it names both projections
      instead of only `userEmail`.

## 2. Server — the write is ignored (D3, US3)

- [x] T2.1 `internal/handlers/handlers.go`, `HandleSettings`: add
      `delete(sent, "userName")` immediately after `delete(sent, "userEmail")`, before
      `memberSettingsViolations` runs, and extend the comment above the pair to cover both.
- [x] T2.2 Confirm `personalSettingsKeys` (`internal/handlers/authz.go:147`) is untouched and
      that `grep -n "userName" internal/handlers/authz.go` still shows it in the personal
      half.

## 3. Server tests (D5, US1, US2, US3)

- [x] T3.1 `internal/handlers/authz_test.go`: after the `userEmail` assertion (~line 276),
      add the symmetric block — member `POST /api/settings` with
      `{"userName":"someone else"}` answers `200`, and the following `GET` contains the
      account's own resolved name, not `someone else`.
- [x] T3.2 Same test: assert a payload carrying `userName` alongside a preference the member
      owns still saves that preference (US3, third criterion).
- [x] T3.3 `internal/db/identity_test.go`: add `TestUserNameFallsBackToEmailThenID` — a table
      over `User.Name()` with the four rungs of the chain plus a whitespace-only display name.
      Create the file if the package has no identity test yet.
- [x] T3.4 `go test ./internal/...` passes.

## 4. Front end — drop the placeholders (D4, US4)

- [x] T4.1 `web/src/context/AppContext.tsx:368`: `userName: 'Developer'` becomes
      `userName: ''`.
- [x] T4.2 `web/src/components/Sidebar.tsx:955`: drop the `: 'SF'` arm of the initials
      ternary, keeping the truthiness guard so an empty name renders an empty circle.
- [x] T4.3 `web/src/components/Sidebar.tsx:960`: drop `|| 'Sylvain Ferry'`.
- [x] T4.4 `web/src/components/Sidebar.tsx:963`: drop `|| 'Paramètres & Profil'`.
- [x] T4.5 `web/src/components/StatusBar.tsx:43`: drop `|| 'Profile'`.
- [x] T4.6 `web/src/components/ProfileModal.tsx:153`: remove the `userName: settings.userName`
      line from the `updateSettings` payload in `handleSave`, leaving `userEmail` as it is.
- [x] T4.7 `grep -rn "Sylvain Ferry\|'SF'\|Paramètres & Profil\|'Developer'" web/src` returns
      nothing.

## 5. Front-end test (D5, US4)

- [x] T5.1 Add `web/tests/identityProjection.test.mjs` following the existing text-assertion
      convention: the four literals are absent from their files, and `handleSave` in
      `ProfileModal.tsx` no longer contains `userName:`.
- [x] T5.2 `cd web && npm test` passes.

## 6. Record and announce (D6, US5)

- [x] T6.1 `docs/adrs/0015-sign-in-is-mandatory-and-settings-are-personal.md`: extend the
      *"`userEmail` is a projection"* paragraph to name `userName`, and mark both as
      projections in the personal-half list so neither reads as a field anyone types.
- [x] T6.2 `CHANGELOG.md`: one line under `## [Unreleased]` → `### Fixed`, written for
      whoever uses Sectile — the sidebar and status bar now show the name set in
      Settings → Account instead of `Developer`. Reference `(#348)`.

## 7. Verify the whole (US1-US5)

- [x] T7.1 `make fmt-check` is clean.
- [x] T7.2 `make test` passes — Go suites and web suites both.
- [x] T7.3 `cd web && npm run build` and `npm run lint` are clean.
- [x] T7.4 Re-read the diff as a reviewer: no schema change, no migration, `authz.go`
      untouched, and nothing outside the scope table in `spec.md`.

## Notes on what was done differently

- **T4.7 needed one file the scope table did not name.** The same `'SF'` literal also sat
  in `web/src/components/SignInStatus.tsx:45`, as the last rung of that component's own
  `displayName || email || userId || 'SF'` chain. It became `''`, which keeps the guard
  against an all-empty user while removing the maintainer's initials. Same defect, same
  ticket, one token.
- **One fallback was deliberately left in place.** `Sidebar.tsx:637` builds the "My Tasks"
  tooltip from `settings.userName || 'nom non renseigné dans le profil'`. It belongs to the
  "My Tasks" control, which the clarification put out of scope, so it is untouched and the
  web suite reads the account button rather than the whole file. The projection makes that
  fallback unreachable in practice; removing it belongs to the separate assignee-matching
  issue.
- **`AppContext`'s `userEmail: 'dev@example.com'` was left as it is.** It is the address's
  pre-fetch placeholder, not the name's, and the clarification named only `userName`.
- **Review (adjust): a rename did not reach the chrome until a reload.** `SignInStatus`
  renamed through `PATCH /api/me` and refreshed only its own `useCurrentUser` state, so the
  sidebar and the status bar kept the old `settings.userName` until the page was reloaded.
  `AppContext` now exposes `reloadSettings` (its existing `fetchSettings`) and `saveName`
  calls it after a successful rename, which is the "settings are re-read" step US1 assumes.
