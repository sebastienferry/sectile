# #406: Implementation checklist

References: [`spec.md`](spec.md), [`plan.md`](plan.md).

## 1. Store (FR1, FR3, FR5, FR7, FR8)

- [ ] T1.1 Migration 7; `forgetSchemaVersion` undoes it.
- [ ] T1.2 `presence.go`: address, `Shared`, `InternalToken`, presence methods.
- [ ] T1.3 Reaper deletes presence of vanished instances.
- [ ] T1.4 Presence tests on SQLite and PostgreSQL.

## 2. Dispatcher (FR2 to FR5)

- [ ] T2.1 `AgentDirectory`, `SetCluster`, presence on register/unregister, remote close.
- [ ] T2.2 Resolution local then remote, with the grace.
- [ ] T2.3 Forwarder and internal handler; local-only variants.
- [ ] T2.4 `CallOperation`, `Dispatch`, `DispatchAndWait`, `PullTasks` forward; `Reachable`.
- [ ] T2.5 Handler call sites and `HandleAgentStatus`.
- [ ] T2.6 Forwarding tests.

## 3. Server and web (FR6, FR7, FR9)

- [ ] T3.1 Internal listener, address, token in `cmd/server/main.go`.
- [ ] T3.2 `useAgentStatus` named events.

## 4. Documentation

- [ ] T4.1 README (internal port and URL, deployment note); CHANGELOG.

## Test plan

`go build ./...`, `go vet ./...`, `go test ./...`, PostgreSQL subset, `-race` on the new
tests, `npm test` in `web`.
