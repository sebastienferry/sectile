# #410: Implementation checklist

References: [`spec.md`](spec.md), [`plan.md`](plan.md).

## 1. Readiness (FR1)

- [x] T1.1 `DB.Ping`, `DB.InstanceRegistered`.
- [x] T1.2 `ReadyPath`, `HandleReady`, public route, `SetInternalServing`, `BeginDrain`.
- [x] T1.3 Unit tests: ready, draining, store closed, shared without row or listener.

## 2. Drain (FR2)

- [x] T2.1 `CloseAgentConnections` on the dispatcher.
- [x] T2.2 `http.Server`, signal handling, grace, instance stop, shutdown in `main.go`.
- [x] T2.3 `SECTILE_SHUTDOWN_GRACE` parsing with a unit test.

## 3. Test bounds (FR3)

- [x] T3.1 `db.SetInstanceTiming` and the three environment variables, with a unit test.

## 4. Harness (FR4)

- [x] T4.1 Replica process helper, balancer, scripted agent.
- [x] T4.2 Scenario steps 1 to 6.
- [x] T4.3 `test:postgres` includes `./cmd/server/`.

## 5. Documentation (FR5)

- [x] T5.1 ADR 0030 and the ADR 0016 amendment.
- [x] T5.2 README "Several replicas" section.
- [x] T5.3 CHANGELOG line.

## Test plan

- `go vet ./...`, `gofmt -l .`.
- `go test ./cmd/server/ ./internal/handlers/ ./internal/db/`.
- `SECTILE_TEST_POSTGRES_DSN=... go test -p 1 ./internal/db/ ./internal/handlers/ ./cmd/server/ -run 'TestPostgres|TestMigrate' -v`.
