# #348 — Implementation checklist

Ordered so the server answers the right value before anything front-end stops guarding
against the wrong one. References: [`spec.md`](./spec.md), [`plan.md`](./plan.md).

## 1. Server — the projection on read (D1, D2, US1, US2)

- [ ] T1.1 `internal/handlers/handlers.go`, `composedSettings`: inside the existing
      `if user, err := h.db.GetUser(userID); err == nil && user != nil { ... }` block, add
      `composed.UserName = user.Name()`. Restructure the condition so the e-mail keeps its
      `user.Email != ""` guard while the name is assigned unconditionally (D2).
- [ ] T1.2 Update the doc comment above `composedSettings` so it names both projections
      instead of only `userEmail`.

## 2. Server — the write is ignored (D3, US3)

- [ ] T2.1 `internal/handlers/handlers.go`, `HandleSettings`: add
      `delete(sent, "userName")` immediately after `delete(sent, "userEmail")`, before
      `memberSettingsViolations` runs, and extend the comment above the pair to cover both.
- [ ] T2.2 Confirm `personalSettingsKeys` (`internal/handlers/authz.go:147`) is untouched and
      that `grep -n "userName" internal/handlers/authz.go` still shows it in the personal
      half.

## 3. Server tests (D5, US1, US2, US3)

- [ ] T3.1 `internal/handlers/authz_test.go`: after the `userEmail` assertion (~line 276),
      add the symmetric block — member `POST /api/settings` with
      `{"userName":"someone else"}` answers `200`, and the following `GET` contains the
      account's own resolved name, not `someone else`.
- [ ] T3.2 Same test: assert a payload carrying `userName` alongside a preference the member
      owns still saves that preference (US3, third criterion).
- [ ] T3.3 `internal/db/identity_test.go`: add `TestUserNameFallsBackToEmailThenID` — a table
      over `User.Name()` with the four rungs of the chain plus a whitespace-only display name.
      Create the file if the package has no identity test yet.
- [ ] T3.4 `go test ./internal/...` passes.

## 4. Front end — drop the placeholders (D4, US4)

- [ ] T4.1 `web/src/context/AppContext.tsx:368`: `userName: 'Developer'` becomes
      `userName: ''`.
- [ ] T4.2 `web/src/components/Sidebar.tsx:955`: drop the `: 'SF'` arm of the initials
      ternary, keeping the truthiness guard so an empty name renders an empty circle.
- [ ] T4.3 `web/src/components/Sidebar.tsx:960`: drop `|| 'Sylvain Ferry'`.
- [ ] T4.4 `web/src/components/Sidebar.tsx:963`: drop `|| 'Paramètres & Profil'`.
- [ ] T4.5 `web/src/components/StatusBar.tsx:43`: drop `|| 'Profile'`.
- [ ] T4.6 `web/src/components/ProfileModal.tsx:153`: remove the `userName: settings.userName`
      line from the `updateSettings` payload in `handleSave`, leaving `userEmail` as it is.
- [ ] T4.7 `grep -rn "Sylvain Ferry\|'SF'\|Paramètres & Profil\|'Developer'" web/src` returns
      nothing.

## 5. Front-end test (D5, US4)

- [ ] T5.1 Add `web/tests/identityProjection.test.mjs` following the existing text-assertion
      convention: the four literals are absent from their files, and `handleSave` in
      `ProfileModal.tsx` no longer contains `userName:`.
- [ ] T5.2 `cd web && npm test` passes.

## 6. Record and announce (D6, US5)

- [ ] T6.1 `docs/adrs/0015-sign-in-is-mandatory-and-settings-are-personal.md`: extend the
      *"`userEmail` is a projection"* paragraph to name `userName`, and mark both as
      projections in the personal-half list so neither reads as a field anyone types.
- [ ] T6.2 `CHANGELOG.md`: one line under `## [Unreleased]` → `### Fixed`, written for
      whoever uses Sectile — the sidebar and status bar now show the name set in
      Settings → Account instead of `Developer`. Reference `(#348)`.

## 7. Verify the whole (US1-US5)

- [ ] T7.1 `make fmt-check` is clean.
- [ ] T7.2 `make test` passes — Go suites and web suites both.
- [ ] T7.3 `cd web && npm run build` and `npm run lint` are clean.
- [ ] T7.4 Re-read the diff as a reviewer: no schema change, no migration, `authz.go`
      untouched, and nothing outside the scope table in `spec.md`.
