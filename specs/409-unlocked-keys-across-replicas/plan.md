# #409: Technical plan

References: [`spec.md`](spec.md), [`docs/clarifications/409.md`](../../docs/clarifications/409.md).

## Schema: migration 25 `user_tracker_credentials.unlock_generation`

```sql
ALTER TABLE user_tracker_credentials ADD COLUMN unlock_generation INTEGER NOT NULL DEFAULT 0;
```

Only in `migrations.go` (the baseline is frozen). The generation is not secret: it only
says which unlock a held key belongs to.

- Lock: `UPDATE ... SET unlock_generation = unlock_generation + 1`.
- Store (upsert): the conflict branch sets `unlock_generation = unlock_generation + 1`
  as well, so a key held for the previous record is refused before it is even tried.
- Delete: the row is gone; nothing can be opened.

## `internal/secrets`

- `WrapKey(wrapping Key, owner Binding, key Key) ([]byte, error)` and
  `UnwrapKey(wrapping Key, owner Binding, wrapped []byte) (Key, error)`: AES-GCM with the
  associated data `sectile:v1:unlocked-key:` + the owner binding, so a wrapped key never
  opens as a credential record, nor under another owner.

## `internal/db/usercredentials.go`

- `unlockedKeys` entries become `heldKey{key secrets.Key, generation int64}`.
- `UnlockUserTrackerCredential` reads `record, sealed, salt, unlock_generation`, keeps
  the key with that generation, then tells the relay.
- `LockUserTrackerCredential` bumps the generation in the database (under `d.mu`), forgets
  the key, tells the relay; its signature gains an `error` return, and the handler
  answers 500 when the bump fails (the lock would otherwise be local only).
- `SetUserTrackerCredential` (sealed) keeps the new key at the generation it wrote; it
  reads the generation back inside the same lock; the relay is told of the new key, or
  of the forgotten one when unsealed.
- `ClearUserTrackerCredential` tells the relay the key is forgotten.
- `userTrackerCredential` selects `unlock_generation` and uses a held key only if its
  generation matches; a mismatch forgets it and answers `ErrCredentialLocked`.
- `UserTrackerCredentials` reports `Unlocked` with the same generation check.

## `internal/db/unlockedkeys.go` (new)

- `UnlockedKeyRelay` interface: `KeyHeld(SharedKey)`, `KeyForgotten(userID, tracker string)`.
  `SetUnlockedKeyRelay(relay)`. The relay is called after the local change, never
  under `d.mu` for longer than the call, and must not block (the handler side pushes in
  goroutines).
- `SharedKey{UserID, Tracker, Generation, Wrapped []byte}`: what crosses the channel.
- `HeldKeys() ([]SharedKey, error)`: every held key, wrapped under the server key, for a
  peer's start-up pull. Refused without a server key.
- `AdoptSharedKey(SharedKey) error`: unwraps, reads the current row, keeps the key only
  when the row is sealed, the generation matches, and the key opens the record.
  Otherwise returns why (no credential, stale generation, wrong key), keeping nothing.
- `ForgetSharedKey(userID, tracker)`: drops the held key, no database write.

## `internal/handlers/credential_cluster.go` (new)

- `credentialCluster{directory MCPDirectory, token, tokenErr, client}`; nil when the
  store is not shared, every method then a no-op.
- Implements `db.UnlockedKeyRelay`: `KeyHeld` wraps nothing (the db already wrapped)
  and POSTs `{"op":"held","key":SharedKey}` to every live peer, in parallel, 3 s each;
  `KeyForgotten` POSTs `{"op":"forgotten","userId","tracker"}`. Failures are logged
  with the peer id and the owner/tracker only.
- `InternalCredentialsHandler()`: `/internal/credentials/keys`, `Authorization: Bearer`
  internal credential (constant-time, `internalBearerMatches`). `POST` applies a held or
  forgotten key; `GET` returns `HeldKeys()`.
- `PullUnlockedKeys(ctx)`: asks every live peer for its held keys and adopts each; an
  unreachable peer or a refused key is logged, never fatal.
- `EnableAgentCluster` also installs the credential cluster and the relay.

## Server

`cmd/server/internal.go`: mount `/internal/credentials/keys`; once the internal listener
serves, pull the peers' keys in a goroutine (the instance row is already registered, so
a key unlocked from then on is pushed here, and one unlocked before is pulled).

## Tests

- `internal/secrets`: wrap and unwrap round trip; wrong wrapping key, other owner and
  tampered bytes refused; a wrapped key does not open as a record.
- `internal/db/unlockedkeys_test.go` (SQLite): generation bumped by lock and by store;
  a held key with a stale generation reads as locked; `AdoptSharedKey` refuses a stale,
  foreign or wrong key and accepts the current one; `HeldKeys` round trip; nothing of
  the key in the database file after unlock, adopt and lock.
- `internal/handlers/credential_cluster_test.go`: two stores on one SQLite file, each
  behind its own internal listener, with an in-memory instance directory: unlock on A
  opens on B; a node started later pulls it; lock on A with B unreachable leaves B
  unable to open; store again and delete; bad bearer refused; no key in the captured
  log output.
- `internal/handlers/postgres_credential_cluster_test.go`: the unlock-on-A, open-on-B
  criterion on two instances sharing PostgreSQL (runs in `test:postgres`).
- Replay fixtures: `forgetSchemaVersion`, `TestAStampedDatabaseStillGainsALaterColumn`
  and the rewind helpers learn about the new column.

## Target files

`internal/db/{migrations.go,usercredentials.go,unlockedkeys.go}`,
`internal/secrets/secrets.go`, `internal/handlers/{credential_cluster.go,usercredentials.go,handlers.go}`,
`cmd/server/internal.go`, tests, README, CHANGELOG.
