# Tasks #517 - MCP sessions kept alive, orphans released

Ordered checklist. One commit, leaving the tree buildable.

## 1. Registry keepalive (FR1-FR4)

- [x] T1.1 Exported defaults `DefaultKeepaliveInterval = 25s`, `DefaultKeepaliveFailures = 3` (the handlers' readers return them); `pingFailures` on `liveSession`.
- [x] T1.2 `SetKeepalive(interval, failures)`, starting the loop once; the loop pings and applies results (plan decisions 1, 2).
- [x] T1.3 `Touch` resets `pingFailures`.

## 2. Settings (FR5)

- [x] T2.1 `mcpKeepaliveInterval()`, `mcpKeepaliveFailures()` in `internal/handlers`.
- [x] T2.2 `handlers.go`: call `SetKeepalive` after building the registry.

## 3. Tests

- [x] T3.1 US1: real client over `httptest`, 50 ms interval, silent for several intervals: one session, same id.
- [x] T3.2 US2: session whose client stops answering (transport closed without DELETE): closed after the threshold; registry empty; goroutine count back to baseline.
- [x] T3.3 US2 bis: pings failing but `Touch` in between: kept.
- [x] T3.4 US3: session with an adopted run and failing pings: kept, run not closed.
- [x] T3.5 US4: pings answered, `lastSeen` unchanged.
- [x] T3.6 US5: settings parsing.

## 4. Documentation (FR6, FR7)

- [x] T4.1 `README.md` MCP session settings, `.env.sample`, `docs/contracts/server-agent-v1.md`, an amendment bullet in ADR 0007 and the SDK note in `.agents/MEMORY.md`.
- [x] T4.2 `CHANGELOG.md` `Fixed` line.

## 5. Verification

- [ ] T5.1 `go test ./internal/taskmcp/ ./internal/handlers/ -count=1`, `go vet ./...`.
