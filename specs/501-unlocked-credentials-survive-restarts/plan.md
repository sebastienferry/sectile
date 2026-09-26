# Plan #501 - An unlocked sealed credential stays unlocked while its owner is connected

Behaviour: `spec.md`. Decisions: `docs/clarifications/501.md`. Line numbers are
those of `9ba14877` and only locate the code; re-find them before editing.

## Stack and surfaces touched

- Go server only: `internal/secrets`, `internal/db`, `internal/handlers`,
  `cmd/server`.
- One numbered migration (26) in `internal/db/migrations.go`, never the frozen
  baseline.
- No web or desktop change: the sign-in screen and the profile already unlock,
  lock and show `unlocked` (`web/src/lib/session.ts`,
  `web/src/components/SignInScreen.tsx`, `AppContext.lockAllUserCredentials`).
- Docs: new ADR 0031, ADR 0014 status line, `README.md` (§ personal
  credentials around line 319, § sign-in around line 743), `CHANGELOG.md`.

## Design choice: the unlock belongs to the person, not to one session row

The clarification says the key is "attached to the person's server-side web
session". Taken literally (a foreign key to one `web_sessions` row), it
contradicts two of its own decisions: an agent keeps the unlock alive with no
tab open (decision 3), and signing out of one of several sessions keeps it
(decision 4). The unlock is therefore stored per `(user_id, tracker)`, like the
credential it opens, and its life is bounded by the person's presence, which
the web sessions and `agent_presence` rows provide. Revoking or expiring the
sessions does end it, through that presence. Recorded in ADR 0031.

## 1. Storage: `user_credential_unlocks` (migration 26)

```sql
CREATE TABLE IF NOT EXISTS user_credential_unlocks (
    user_id     TEXT NOT NULL,
    tracker     TEXT NOT NULL,
    wrapped_key BLOB NOT NULL,
    unlocked_at DATETIME NOT NULL,
    PRIMARY KEY (user_id, tracker),
    FOREIGN KEY (user_id, tracker) REFERENCES user_tracker_credentials(user_id, tracker) ON DELETE CASCADE
);
```

- One statement per migration entry (the file's convention): the table only.
  `BLOB` and `DATETIME` are rebound for PostgreSQL by `rebind.go`.
- If the composite foreign key is awkward under either dialect, drop it: every
  path that deletes a credential deletes its unlock explicitly anyway (§3).
- Version 26, name `user_credential_unlocks`. `main` had 25 at `9ba14877`; if a
  parallel branch lands 26 first, renumber.
- Test fixtures that rewind `schema_migrations` replay it and fail with "table
  already exists": drop the table in `forgetSchemaVersion` and in the
  `version >= N` fixtures (`migrations_test.go`, `activerun_test.go`), as #464
  did with `undoServerCredentialsMigration`. Run the PostgreSQL suite too.

## 2. Wrapping the derived key: `internal/secrets`

- Add to `Binding` a field `Unlock bool`, serialised under its own prefix so an
  unlock record never opens as a credential and the other way round:
  `sectile:v1:unlock:user:%d:%s:tracker:%d:%s`.
- Helper `UnlockBinding(userID, tracker string) Binding`.
- The wrapped key is `secrets.Seal(serverKey, UnlockBinding(u, t), hex(derived))`;
  `Open` gives the hex back, decoded into a `Key` (32 bytes; reject another
  length as `ErrWrongKey`).
- Tests: an unlock record does not open with the credential binding, nor under
  another user or tracker, nor with another server key.

## 3. `internal/db/usercredentials.go`

Remove `unlockedKeys`, its helpers, `unlockKey` and the `unlocked` field of `DB`
(`db.go:138`). Every former use reads or writes the table.

| Today | After |
| --- | --- |
| `UnlockUserTrackerCredential` → `unlocked.set` | verify the passphrase as today, then refuse with `ErrServerKeyUnavailable` (wraps `d.serverKeyErr`) if the server key is missing (FR-9); else upsert `(user_id, tracker, wrapped_key, unlocked_at = now)` under `d.mu.Lock` |
| `LockUserTrackerCredential` → `unlocked.clear` | `DELETE FROM user_credential_unlocks WHERE user_id = ? AND tracker = ?`; signature gains an `error` return, the handler reports it |
| `SetUserTrackerCredential` clear + set | in the same transaction as the upsert of the credential: delete the unlock, then insert a new one if sealed |
| `ClearUserTrackerCredential` clear | delete the unlock in the same transaction as the credential |
| `UserTrackerCredentials` → `unlocked.get` | `LEFT JOIN user_credential_unlocks` in the listing query; `Unlocked = sealed == 0 OR unlock row present` |
| `userTrackerCredential` → `unlocked.get` | `LEFT JOIN` in its single `SELECT`, reading `wrapped_key`; absent → `ErrCredentialLocked`; present → unwrap with `d.serverKey`; an unwrap failure deletes the row (best effort, no lock taken) and answers `ErrCredentialLocked` |

- `userTrackerCredential` keeps taking no lock of its own (the re-entrancy
  comment at :351 stays true): one query, no extra round trip.
- Add `ErrServerKeyUnavailable` in `usercredentials.go`; the unlock route maps
  it to 503 with the existing French message style ("la clé de chiffrement du
  serveur est indisponible").
- Expiry is enforced by the sweep (§4), not at read time: a key past its window
  can be used for at most one sweep period, which FR-3 allows.

## 4. Presence and the sweep: new `internal/db/credentialunlocks.go`

```go
// unlockIdleWindow is how long an unlock outlives its owner's last presence.
const unlockIdleWindow = 30 * time.Minute

// unlockSweepEvery is how often each instance forgets idle unlocks. A package
// var so tests can shorten it, like instanceHeartbeatEvery.
var unlockSweepEvery = time.Minute

// ForgetIdleUnlocks deletes every unlock whose owner's last presence is older
// than the idle window at now. It returns how many were forgotten.
func (d *DB) ForgetIdleUnlocks(now time.Time) (int, error)

// ForgetUnlocksIfAbsent deletes one person's unlocks when they are not
// connected at now, ignoring the unlock time (FR-5).
func (d *DB) ForgetUnlocksIfAbsent(userID string, now time.Time) error

// ForgetUserUnlocks deletes one person's unlocks unconditionally.
func (d *DB) ForgetUserUnlocks(userID string) error
```

Last presence of a user, in SQL, both dialects (`?` placeholders rebound):

- browser: `MAX(COALESCE(last_seen_at, created_at))` over `web_sessions` with
  `revoked_at IS NULL AND expires_at > now`;
- agent: over `agent_presence p JOIN server_instances i ON i.id = p.instance_id`
  (LEFT JOIN, a vanished instance row counts as dead):
  - `disconnected_at IS NULL` and `i.last_seen >= now - instanceDeadAfter` →
    `now`;
  - `disconnected_at IS NULL` and the instance dead → `i.last_seen` (or
    `connected_at` if the row is gone);
  - otherwise `disconnected_at`;
- the unlock: `unlocked_at` (not for `ForgetUnlocksIfAbsent`).

`ForgetIdleUnlocks` is one `DELETE ... WHERE unlocked_at < cutoff AND NOT EXISTS
(session seen >= cutoff) AND NOT EXISTS (agent presence >= cutoff)`, with
`cutoff = now - unlockIdleWindow`: no per-user loop, idempotent when several
replicas run it at once. Reuse `instanceDeadAfter` from `instances.go`; do not
copy its value.

`ForgetUnlocksIfAbsent` uses the same two `NOT EXISTS` for one user, without the
`unlocked_at` clause.

Loop: add a third ticker `unlockSweep` to the goroutine of `StartInstance`
(`instances.go:176`), and call `ForgetIdleUnlocks(now)` once synchronously in
`StartInstance` before the goroutine starts, so a server that was down longer
than the window forgets stale unlocks before `cmd/server/main.go` serves
(FR-3). Log a failure with `⚠️` and a count when non-zero, as the reclaim
branch does (French log lines, like the neighbours).

## 5. Triggers

- **Sign-out**, `HandleLogout` (`internal/handlers/auth.go:186`): resolve the
  user of the cookie with a non-touching lookup before `RevokeWebSession`, then
  call `ForgetUnlocksIfAbsent(userID, now)` after it. Add
  `WebSessionOwner(token string) string` to `sessions.go` (the `SELECT user_id`
  of `UserForWebSession`, no `last_seen_at` write, revoked or not). A failure is
  logged and does not fail the sign-out.
- **Block**, `roles.go:229`: after `RevokeUserSessions`, call
  `ForgetUserUnlocks(id)` (unconditional: a blocked person's agents are refused
  anyway).
- **Lock / Lock all**, `internal/handlers/usercredentials.go:154`: unchanged
  routes; report the new `error` of `LockUserTrackerCredential` as 500.

## 6. Tests

`internal/db/credentialunlocks_test.go` (clock passed as `now`, no sleeps):

- unlock survives a reopen of the same database file (restart, US1);
- two `DB` handles on the same file (two replicas, US2): unlock on A, resolve on
  B; lock on A, B answers `ErrCredentialLocked`;
- sweep: session seen 29 min ago keeps it; 31 min ago forgets it; revoked or
  expired session does not count; unlock 10 min ago with a stale session keeps
  it; connected agent on a live instance keeps it for 2 h; agent disconnected
  31 min ago forgets it; agent on an instance dead for 31 min forgets it;
- `ForgetUnlocksIfAbsent`: sole session revoked → forgotten despite a fresh
  unlock; another session seen 5 min ago → kept; agent connected → kept; other
  session seen 40 min ago → forgotten;
- `ForgetUserUnlocks` after a block;
- unlock without a server key → `ErrServerKeyUnavailable`, nothing stored;
- a wrapped key that the server key cannot open → locked and the row deleted;
- the stored row contains neither the passphrase, the derived key nor the token
  in clear (scan the bytes, as `TestSessionCookiesAreNotStoredInPlaintext`).

Adapt `usercredentials_test.go` (`TestASealedTokenNeedsItsPassphrase`,
`TestReplacingACredentialRetiresTheKeyItWasSealedWith`,
`TestClearingRemovesTheCredentialAndItsKey`,
`TestAMissingServerKeyBlocksOnlyWhatNeedsIt`) to the table; their behaviour is
unchanged except FR-9.

`internal/handlers`: sign-out of the last session locks, sign-out with a second
live session does not; the unlock route answers 503 without a server key.

`internal/secrets`: the binding tests of §2.

Run `go test ./...`, then `internal/db` under PostgreSQL (see the memory note on
`SECTILE_TEST_POSTGRES_DSN`, on a throwaway database).

## 7. Docs

- `docs/adrs/0031-unlocked-sealed-credentials-live-with-their-owners-presence.md`:
  context (restarts, replicas, #501), decision (§1, §4, §5), consequences (a
  database copy plus the server key opens a sealed token during the window;
  root on the server could already read it from memory; the database alone
  still opens nothing; unlock now needs the server key), alternatives rejected
  (keep it in memory; the browser holds the key; a foreign key to one session
  row; a setting for the delay).
- ADR 0014: status line "Amended by ADR 0031 for the lifetime of an unlock";
  leave its body as written.
- `README.md` § personal credentials: replace the implicit "until restart" with
  the new lifetime; § sign-in: a passphrase given at sign-in keeps the tokens
  unlocked while connected, then 30 minutes.
- `CHANGELOG.md`, `[Unreleased]` → `### Fixed`: one line (see `tasks.md`).

## Rejected alternatives (implementation)

- **Check expiry at read time in `userTrackerCredential`**: exact to the second
  but adds the presence subqueries to every credential resolution, including
  from paths that hold `d.mu`. The one-minute sweep meets FR-3.
- **An `expires_at` pushed forward on each presence**: writes on every session
  touch and agent heartbeat, in several places; computing from existing presence
  rows needs no new write path.
- **A cache of unwrapped keys per instance**: brings back per-replica state and
  a second place to forget on lock.
