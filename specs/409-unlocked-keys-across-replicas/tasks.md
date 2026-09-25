# #409: Implementation checklist

References: [`spec.md`](spec.md), [`plan.md`](plan.md).

## 1. Key wrapping (FR6)

- [ ] T1.1 `secrets.WrapKey` / `secrets.UnwrapKey` with their own associated data.
- [ ] T1.2 Tests: round trip, wrong key, other owner, tampered, not openable as a record.

## 2. Lock generation (FR3, FR4)

- [ ] T2.1 Migration 25 `user_tracker_credentials.unlock_generation`.
- [ ] T2.2 Held keys carry their generation; the resolver and the list check it.
- [ ] T2.3 Lock bumps the generation and returns an error; store bumps it on conflict.
- [ ] T2.4 Handler: a failed lock answers 500.
- [ ] T2.5 Replay and rewind fixtures extended for the new column.
- [ ] T2.6 Tests (SQLite): stale generation reads as locked; lock and store bump it.

## 3. Sharing in the store (FR1, FR2, FR5, FR7)

- [ ] T3.1 `UnlockedKeyRelay`, `SetUnlockedKeyRelay`, calls from unlock, lock, store, clear.
- [ ] T3.2 `HeldKeys`, `AdoptSharedKey`, `ForgetSharedKey`.
- [ ] T3.3 Tests: adopt refuses stale, foreign, wrong; accepts current; no key bytes in
      the database file.

## 4. Internal channel (FR1, FR2, FR6, FR8)

- [ ] T4.1 `credentialCluster`: push to live peers in parallel, bounded, logged failures.
- [ ] T4.2 `/internal/credentials/keys` GET and POST behind the internal bearer.
- [ ] T4.3 `PullUnlockedKeys` at start; wiring in `EnableAgentCluster` and
      `cmd/server/internal.go`.
- [ ] T4.4 Tests: two nodes on one SQLite file (unlock, late start, lock with a peer down,
      store again, delete, bad bearer, no key in logs); PostgreSQL two-instance test.

## 5. Documentation

- [ ] T5.1 README: sealed credentials under several replicas.
- [ ] T5.2 CHANGELOG line (shared with #410's multi-replica entry).

## Test plan

- `go vet ./...`, `gofmt -l`.
- `go test ./internal/secrets/ ./internal/db/ ./internal/handlers/ ./cmd/server/`.
- `SECTILE_TEST_POSTGRES_DSN=... go test -p 1 ./internal/db/ ./internal/handlers/ -run 'TestPostgres|TestMigrate'`.
