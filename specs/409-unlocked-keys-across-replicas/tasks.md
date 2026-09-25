# #409: Implementation checklist

References: [`spec.md`](spec.md), [`plan.md`](plan.md).

## 1. Key wrapping (FR6)

- [x] T1.1 `secrets.WrapKey` / `secrets.UnwrapKey` with their own associated data.
- [x] T1.2 Tests: round trip, wrong key, other owner, tampered, not openable as a record.

## 2. Lock generation (FR3, FR4)

- [x] T2.1 Migration 25 `user_tracker_credentials.unlock_generation`.
- [x] T2.2 Held keys carry their generation; the resolver and the list check it.
- [x] T2.3 Lock bumps the generation and returns an error; store bumps it on conflict.
- [x] T2.4 Handler: a failed lock answers 500.
- [x] T2.5 Replay and rewind fixtures extended for the new column.
- [x] T2.6 Tests (SQLite): stale generation reads as locked; lock and store bump it.

## 3. Sharing in the store (FR1, FR2, FR5, FR7)

- [x] T3.1 `UnlockedKeyRelay`, `SetUnlockedKeyRelay`, calls from unlock, lock, store, clear.
- [x] T3.2 `HeldKeys`, `AdoptSharedKey`, `ForgetSharedKey`.
- [x] T3.3 Tests: adopt refuses stale, foreign, wrong; accepts current; no key bytes in
      the database file.

## 4. Internal channel (FR1, FR2, FR6, FR8)

- [x] T4.1 `credentialCluster`: push to live peers in parallel, bounded, logged failures.
- [x] T4.2 `/internal/credentials/keys` GET and POST behind the internal bearer.
- [x] T4.3 `PullUnlockedKeys` at start; wiring in `EnableAgentCluster` and
      `cmd/server/internal.go`.
- [x] T4.4 Tests: two nodes on one SQLite file (unlock, late start, lock with a peer down,
      store again, delete, bad bearer, no key in logs); PostgreSQL two-instance test.

## 5. Documentation

- [x] T5.1 README: sealed credentials under several replicas.
- [x] T5.2 CHANGELOG line (shared with #410's multi-replica entry).

## Test plan

- `go vet ./...`, `gofmt -l`.
- `go test ./internal/secrets/ ./internal/db/ ./internal/handlers/ ./cmd/server/`.
- `SECTILE_TEST_POSTGRES_DSN=... go test -p 1 ./internal/db/ ./internal/handlers/ -run 'TestPostgres|TestMigrate'`.
