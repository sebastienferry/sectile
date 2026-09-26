# Tasks #501 - An unlocked sealed credential stays unlocked while its owner is connected

Ordered checklist. Each group is one commit (Conventional Commits). Before
starting, `git fetch origin` and integrate `origin/main` (merge, the branch is
pushed); check `internal/db/migrations.go` for a version 26 landed meanwhile
and renumber (plan §1).

## 1. Wrapped unlock binding (FR-8) - `feat(secrets)`

- [x] T1.1 `Binding.Unlock` with its own serialisation prefix and
  `UnlockBinding(userID, tracker)` (plan §2).
- [x] T1.2 Tests: an unlock record opens only with its own binding, owner,
  tracker and server key; a credential record never opens as an unlock.

## 2. Shared unlock storage (FR-1, FR-6, FR-7, FR-9, FR-10) - `fix(db)`

- [x] T2.1 Migration 29 `user_credential_unlocks` (plan §1); teach the rewind
  fixtures to drop the table (`forgetSchemaVersion`, the `version >= N`
  fixtures in `migrations_test.go` and `activerun_test.go`).
- [x] T2.2 Remove `unlockedKeys` and `DB.unlocked`; rewrite unlock, lock,
  store, clear, listing and `userTrackerCredential` against the table
  (plan §3 table). `userTrackerCredential` still takes no lock.
- [x] T2.3 `ErrServerKeyUnavailable` on unlock without a server key.
- [x] T2.4 Adapt the existing `usercredentials_test.go` tests; add: survives a
  reopen, visible through a second `DB` handle, lock through one handle seen by
  the other, unwrap failure locks and deletes, no clear-text secret in the row.

## 3. Presence-bound expiry (FR-2, FR-3, FR-4, FR-11) - `fix(db)`

- [x] T3.1 `internal/db/credentialunlocks.go`: `unlockIdleWindow`,
  `unlockSweepEvery`, `ForgetIdleUnlocks`, `ForgetUnlocksIfAbsent`,
  `ForgetUserUnlocks` (plan §4), both dialects.
- [x] T3.2 Sweep once in `StartInstance` before its goroutine, then on the
  `unlockSweep` ticker.
- [x] T3.3 Table tests of plan §6 (sessions seen 29/31 min, revoked, expired,
  recent unlock, live agent, disconnected agent, dead instance), clock passed
  as `now`.

## 4. Sign-out and block (FR-5, FR-6) - `fix(auth)`

- [x] T4.1 `WebSessionOwner(token)` in `sessions.go` (no touch).
- [x] T4.2 `HandleLogout`: resolve owner, revoke, `ForgetUnlocksIfAbsent`;
  a failure is logged, the sign-out still succeeds.
- [x] T4.3 Block in `roles.go`: `ForgetUserUnlocks` after `RevokeUserSessions`.
- [x] T4.4 Lock route reports the new error of `LockUserTrackerCredential`;
  unlock route maps `ErrServerKeyUnavailable` to 503.
- [x] T4.5 Handler tests: last-session sign-out locks; sign-out with a second
  live session keeps; sign-out with a connected agent keeps; unlock without a
  server key answers 503. Test for the block.

## 5. Documentation (US7) - `docs`

- [x] T5.1 ADR 0032 (plan §7); ADR 0014 status line "Amended by ADR 0032".
- [x] T5.2 `README.md` § personal credentials and § sign-in.
- [x] T5.3 `CHANGELOG.md` `[Unreleased]` → `### Fixed`:
  "**Sealed tracker tokens no longer lock themselves when the server
  restarts.** Once unlocked, a token sealed with a passphrase stays unlocked on
  every server instance while you are connected, from a browser tab or a
  running local agent, and locks itself 30 minutes after you leave, or at once
  when you sign out with nothing else of yours connected. (#501)"

## 5b. Merge with #409 (#506) - `merge`

#409, merged into `main` meanwhile, kept the derived key in memory and relayed
it between instances, and rejected persisting it. The owner chose #501's
database store over it (answer A on the ticket, 2026-09-26).

- [x] T5b.1 Migration renumbered 26 → 29 after main's 26-28; ADR 0031 → 0032.
- [x] T5b.2 `secrets.WrapKey`/`UnwrapKey` from main kept (own associated-data
  prefix); `secrets.UnlockBinding` removed.
- [x] T5b.3 Relay removed: `internal/db/unlockedkeys.go`,
  `internal/handlers/credential_cluster.go`, the `/internal/credentials/keys`
  route, the key pull at start, and their tests. `unlock_generation`
  (migration 28) left unused.
- [x] T5b.4 ADR 0030 amended (row, rejected alternative); README multi-replica
  paragraph and the #409 changelog line rewritten.

## 6. Verification

- [x] T6.1 `gofmt`, `go vet ./...`, `go test ./...` (outside the sandbox for
  httptest; `GOCACHE` under `$TMPDIR`).
- [x] T6.2 `go test ./internal/db/` against a throwaway PostgreSQL database
  (`SECTILE_TEST_POSTGRES_DSN`, never the dev database).
- [ ] T6.3 Manual check on a **copy** of the dev database with no tracker token
  in the environment: unlock, restart the branch server, the profile still shows
  the credential unlocked; sign out, it shows locked.
  Not run: `TestAnUnlockSurvivesARestart`, `TestEveryReplicaSeesTheSameUnlock`
  and the sign-out handler tests cover it against real database files, and a
  branch server on a copy of the dev database is left to the reviewer.

## Implementation notes

- An agent's `agent_presence` row is dropped when its instance is reclaimed,
  and every row is dropped when a single-process server restarts. Read from
  those rows alone, an unlock kept alive by an agent would be forgotten at that
  moment instead of 30 minutes after the agent was last seen (US4, and US1 for
  a person whose only presence is an agent). Migration 29 therefore carries a
  nullable `agent_seen_at`, filled from the dropped rows just before they go
  (`keepAgentPresenceInUnlocks`), and the sweep counts it. ADR 0032 records it.
- The composite foreign key of plan §1 was dropped: SQLite does not enforce it
  here, and every path that deletes a credential, or an account, deletes its
  unlock explicitly.
- A sealed credential saved without a server key is stored locked, since its
  unlock has nowhere to live; unlocking it answers the FR-9 error.

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
