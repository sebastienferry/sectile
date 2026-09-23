# #406: Implementation checklist

References: [`spec.md`](spec.md), [`plan.md`](plan.md).

## 1. Store (FR1, FR3, FR5, FR7, FR8)

- [x] T1.1 Migration 7; `forgetSchemaVersion` undoes it.
- [x] T1.2 `presence.go`: address, `Shared`, `InternalToken`, presence methods.
- [x] T1.3 Reaper deletes presence of vanished instances.
- [x] T1.4 Presence tests on SQLite and PostgreSQL.

## 2. Dispatcher (FR2 to FR5)

- [x] T2.1 `AgentDirectory`, `SetCluster`, presence on register/unregister, remote close.
- [x] T2.2 Resolution local then remote, with the grace.
- [x] T2.3 Forwarder and internal handler; local-only variants.
- [x] T2.4 `CallOperation`, `Dispatch`, `DispatchAndWait`, `PullTasks` forward; `Reachable`.
- [x] T2.5 Handler call sites and `HandleAgentStatus`.
- [x] T2.6 Forwarding tests.

## 3. Server and web (FR6, FR7, FR9)

- [x] T3.1 Internal listener, address, token in `cmd/server/main.go`.
- [x] T3.2 `useAgentStatus` named events.

## 4. Documentation

- [x] T4.1 README (internal port and URL, deployment note); CHANGELOG.

## Test plan

`go build ./...`, `go vet ./...`, `go test ./...`, PostgreSQL subset, `-race` on the new
tests, `npm test` in `web`.
