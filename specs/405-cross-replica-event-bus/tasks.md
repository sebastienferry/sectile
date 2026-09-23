# #405: Implementation checklist

References: [`spec.md`](spec.md), [`plan.md`](plan.md).

## 1. Status guard (FR5)

- [ ] T1.1 `AND status != 'canceled'` on the seven job updates listed in the plan.
- [ ] T1.2 Tests: canceled stays canceled on late start and on each job end.

## 2. Bus (FR1 to FR4, FR6, FR7)

- [ ] T2.1 `postgresDialect.connConfig` extracted from `Open`.
- [ ] T2.2 `bus.go`: message, publish, handle, callbacks, `StartEventBus` with reconnect.
- [ ] T2.3 `CancelActivity` publishes `cancel`; `cancelLocal` extracted.
- [ ] T2.4 SQLite tests for `handleBusMessage` and publish no-op.
- [ ] T2.5 PostgreSQL test with two stores (event, no self-delivery, cancel).

## 3. Handler and server (FR1, FR2)

- [ ] T3.1 `broadcastLocal` + publish in `BroadcastEvent`; `agent_pty_output` kept local.
- [ ] T3.2 Relayed messages rebuilt into `Event` and delivered locally.
- [ ] T3.3 Handler test.
- [ ] T3.4 `StartEventBus` in `cmd/server/main.go`.

## 4. Documentation

- [ ] T4.1 README multi-server note; `CHANGELOG.md` `Fixed`.

## Test plan

`go build ./...`, `go vet ./...`, `go test ./...`, PostgreSQL subset with a throwaway DSN,
`-race` on the bus tests.
