# Tasks #501 - An unlocked sealed credential stays unlocked while its owner is connected

Ordered checklist. Each group is one commit (Conventional Commits). Before
starting, `git fetch origin` and integrate `origin/main` (merge, the branch is
pushed); check `internal/db/migrations.go` for a version 26 landed meanwhile
and renumber (plan §1).

## 1. Wrapped unlock binding (FR-8) - `feat(secrets)`

- [ ] T1.1 `Binding.Unlock` with its own serialisation prefix and
  `UnlockBinding(userID, tracker)` (plan §2).
- [ ] T1.2 Tests: an unlock record opens only with its own binding, owner,
  tracker and server key; a credential record never opens as an unlock.

## 2. Shared unlock storage (FR-1, FR-6, FR-7, FR-9, FR-10) - `fix(db)`

- [ ] T2.1 Migration 26 `user_credential_unlocks` (plan §1); teach the rewind
  fixtures to drop the table (`forgetSchemaVersion`, the `version >= N`
  fixtures in `migrations_test.go` and `activerun_test.go`).
- [ ] T2.2 Remove `unlockedKeys` and `DB.unlocked`; rewrite unlock, lock,
  store, clear, listing and `userTrackerCredential` against the table
  (plan §3 table). `userTrackerCredential` still takes no lock.
- [ ] T2.3 `ErrServerKeyUnavailable` on unlock without a server key.
- [ ] T2.4 Adapt the existing `usercredentials_test.go` tests; add: survives a
  reopen, visible through a second `DB` handle, lock through one handle seen by
  the other, unwrap failure locks and deletes, no clear-text secret in the row.

## 3. Presence-bound expiry (FR-2, FR-3, FR-4, FR-11) - `fix(db)`

- [ ] T3.1 `internal/db/credentialunlocks.go`: `unlockIdleWindow`,
  `unlockSweepEvery`, `ForgetIdleUnlocks`, `ForgetUnlocksIfAbsent`,
  `ForgetUserUnlocks` (plan §4), both dialects.
- [ ] T3.2 Sweep once in `StartInstance` before its goroutine, then on the
  `unlockSweep` ticker.
- [ ] T3.3 Table tests of plan §6 (sessions seen 29/31 min, revoked, expired,
  recent unlock, live agent, disconnected agent, dead instance), clock passed
  as `now`.

## 4. Sign-out and block (FR-5, FR-6) - `fix(auth)`

- [ ] T4.1 `WebSessionOwner(token)` in `sessions.go` (no touch).
- [ ] T4.2 `HandleLogout`: resolve owner, revoke, `ForgetUnlocksIfAbsent`;
  a failure is logged, the sign-out still succeeds.
- [ ] T4.3 Block in `roles.go`: `ForgetUserUnlocks` after `RevokeUserSessions`.
- [ ] T4.4 Lock route reports the new error of `LockUserTrackerCredential`;
  unlock route maps `ErrServerKeyUnavailable` to 503.
- [ ] T4.5 Handler tests: last-session sign-out locks; sign-out with a second
  live session keeps; sign-out with a connected agent keeps; unlock without a
  server key answers 503. Test for the block.

## 5. Documentation (US7) - `docs`

- [ ] T5.1 ADR 0031 (plan §7); ADR 0014 status line "Amended by ADR 0031".
- [ ] T5.2 `README.md` § personal credentials and § sign-in.
- [ ] T5.3 `CHANGELOG.md` `[Unreleased]` → `### Fixed`:
  "**Sealed tracker tokens no longer lock themselves when the server
  restarts.** Once unlocked, a token sealed with a passphrase stays unlocked on
  every server instance while you are connected, from a browser tab or a
  running local agent, and locks itself 30 minutes after you leave, or at once
  when you sign out with nothing else of yours connected. (#501)"

## 6. Verification

- [ ] T6.1 `gofmt`, `go vet ./...`, `go test ./...` (outside the sandbox for
  httptest; `GOCACHE` under `$TMPDIR`).
- [ ] T6.2 `go test ./internal/db/` against a throwaway PostgreSQL database
  (`SECTILE_TEST_POSTGRES_DSN`, never the dev database).
- [ ] T6.3 Manual check on a **copy** of the dev database with no tracker token
  in the environment: unlock, restart the branch server, the profile still shows
  the credential unlocked; sign out, it shows locked.

## Test plan (acceptance → test)

| Acceptance | Test |
| --- | --- |
| US1 restart keeps unlock | T2.4 reopen |
| US1 stale before restart locks | T3.2 sweep at start, T3.3 |
| US2 replicas | T2.4 second handle |
| US3 idle 30 min | T3.3 |
| US3 recent unlock | T3.3 |
| US4 agent keeps / disconnect / dead instance | T3.3 |
| US5 sign-out variants | T4.5, T3.3 (`ForgetUnlocksIfAbsent`) |
| US6 lock, re-store, delete, block | T2.4, T4.5 |
| FR-8 no secret in clear | T1.2, T2.4 |
| FR-9 no server key | T2.4, T4.5 |
