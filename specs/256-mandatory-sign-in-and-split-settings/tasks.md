# #256 — Implementation checklist

Ordered so the server is coherent before the interface stops offering the implicit mode,
and so the settings split lands before the code that reads the owner's commands.

## 1. Storage — personal settings

- [x] **T1** Add the `user_settings` DDL of `plan.md` to the schema list in
      `internal/db/db.go` (next to the `settings` table).
- [x] **T2** Create `internal/db/usersettings.go` with `UserSettings(userID)` and
      `UpdateUserSettings(userID, s)`: read returns the stored row, or the personal
      projection of the global row when the account has none (D3); write upserts the
      personal columns only.
- [x] **T3** `internal/db/usersettings_test.go`: seeding from the global row for a new
      account, isolation between two accounts, an omitted key changing nothing.

## 2. Server — the settings split

- [x] **T4** In `internal/handlers/authz.go`, add `editorCommand` and
      `externalTerminalCommand` to `memberSettingsKeys` and document it as the routing
      table between the two stores.
- [x] **T5** In `HandleSettings` (`internal/handlers/handlers.go`), compose the `GET`
      answer (deployment row + caller's personal row + `users.email` for `userEmail`) and
      route the `POST/PUT` per key, keeping the existing member refusal on deployment keys.
- [x] **T6** Ignore an incoming `userEmail` on write (D4) and note it on the field in
      `internal/models/models.go`.
- [x] **T7** Make the external terminal and the editor read the execution owner's settings:
      `launchTaskExternalTerminal` and `HandleOpenEditor`, project override first, then the
      owner, then the deployment default (D5). Leave every AI read on `GetSettings()`.
- [x] **T8** Handler tests: member refused on a deployment key with the key named, member
      allowed on a personal key, two accounts reading their own values, `userEmail` not
      writable, owner's terminal command used at launch.

## 3. Server — mandatory sign-in

- [x] **T9** Remove `modeImplicit` and its branch in `signInMode()`; drop the
      `ImplicitUser` admin shortcut in `principalFor` so `default` resolves through
      `GetUser` and keeps its stored role.
- [x] **T10** `webSessionUser()` returns `""` when neither the cookie nor the workstation
      API key identifies anyone (`internal/handlers/pairing.go`), and its comment is
      rewritten accordingly.
- [x] **T11** `RequireSession` drops the implicit bypass (`internal/handlers/auth.go`);
      `publicPath()` stays as it is.
- [x] **T12** Keep `ImplicitUser` only on the requestless entry points
      (`LaunchTaskExternalTerminal`, `agent_operations.go`, `agent_api.go`) and check no
      HTTP path reaches it any more.
- [x] **T13** Tests: anonymous `401` on an interface route, each public path still served,
      a valid API key still resolving its user, an existing `default` admin still admin,
      the first account on an empty deployment becoming admin.

## 4. Web interface

- [x] **T14** `web/src/lib/session.ts`: `SignInMode` without `'implicit'`, `needsSignIn` is
      `!!user && !user.signedIn`, `describeSignInMode` loses its implicit branch.
- [x] **T15** `web/src/components/SignInScreen.tsx`: required e-mail, optional sealing
      passphrase, no login password; wording updated to say what the passphrase is for.
- [x] **T16** After a successful sign-in, when a passphrase was supplied, call
      `POST /api/me/tracker-credentials/unlock` for each sealed tracker and report one
      aggregated message; a failure never blocks the redirect (US7).
- [x] **T17** Check that nothing else in the front end still branches on the implicit mode
      (`grep -rn implicit web/src`), and keep the sign-out button of `SignInStatus.tsx` as
      it is.
- [x] **T18** `web/tests/session.test.mjs`: `needsSignIn` for a signed-out user with no
      implicit escape hatch, and the existing redirect rules still passing.

## 5. Documentation

- [x] **T19** Write `docs/adrs/0015-sign-in-is-mandatory-and-settings-are-personal.md`,
      superseding the two ADR 0013 decisions ("the implicit user lasts until the first
      account" and the shared settings row), and reference it from ADR 0013.
- [x] **T20** Update `README.md` where it describes a deployment opening without sign-in,
      and `CHANGELOG.md` if the repository keeps one.

## 6. Gates

- [x] **T21** `go build ./... && go test ./...`.
- [x] **T22** In `web/`: `npm run lint`, `npm run build`, then `npm test`.
- [x] **T23** Walk the acceptance criteria of `spec.md` one by one and record the result.
