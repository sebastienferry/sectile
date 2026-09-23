# #405: Technical plan

References: [`spec.md`](spec.md), [`docs/clarifications/405.md`](../../docs/clarifications/405.md).

## `internal/db/bus.go` (new)

```go
type BusMessage struct {
    Origin     string `json:"o"`
    Kind       string `json:"k"`           // "event" or "cancel"
    Type       string `json:"t,omitempty"` // SSE event type
    TaskID     string `json:"task,omitempty"`
    ActivityID string `json:"act,omitempty"`
    Error      string `json:"err,omitempty"` // at most 1000 characters
}
```

- `const busChannel = "sectile_events"`.
- `PublishEvent(eventType, taskID, activityID, errText string)`: kind `event`.
- `OnRelayedEvent(fn func(BusMessage))`: registered by the handler.
- `publish(msg)`: no-op when `dialect.ServesOneProcess()`; otherwise
  `SELECT pg_notify(?, ?)` through the pool; failures are logged, never returned.
- `handleBusMessage(payload string)`: ignores unparsable payloads and `Origin == instanceID`;
  `cancel` calls `cancelLocal(activityID)`; `event` calls the registered callbacks.
- `StartEventBus() (stop func(), err error)`: no-op on SQLite. On PostgreSQL a goroutine
  opens a dedicated `pgx.Conn` (config from `dialect`), `LISTEN sectile_events`, loops on
  `WaitForNotification`, and reconnects after 1 s, doubling to 30 s, on any error until
  stopped.

## Dialect

`postgresDialect.connConfig(cfg)` extracted from `Open` (ParseConfig + UTC timezone) and
reused by the listener, so both see the same server.

## Cancel

`CancelActivity`: local cancel (extracted as `cancelLocal`), row update as today, then
`publish(cancel)`.

## Status guard

`AND status != 'canceled'` on: job start (`processSkillJob`), panic handler, agent-launch
end, sync end, tracker-update not-found and end, tracker-op end (`trackerops.go`).

## Handler

- `BroadcastEvent(e)`: local fan-out (unchanged, extracted as `broadcastLocal`), then
  `db.PublishEvent` unless `e.Type == "agent_pty_output"` or the handler has no store.
- `NewHandler`: `db.OnRelayedEvent` builds an `Event` (task and activity reloaded by id,
  absent ones left nil) and calls `broadcastLocal`.

## Server

`cmd/server/main.go`: `database.StartEventBus()` after `StartInstance`; failure is fatal.

## Tests

- `internal/db/bus_test.go` (SQLite): `handleBusMessage` ignores own origin and garbage,
  routes `cancel` to the local cancel function, `event` to callbacks; publish on SQLite is
  a no-op; error truncation.
- `internal/db/canceled_guard_test.go` (SQLite): a canceled activity stays canceled when
  the job starts late and when a sync, tracker-op or agent-launch job ends.
- `internal/db/postgres_bus_test.go`: two stores; event relayed and not self-delivered;
  cancel from one calls the other's cancel function.
- `internal/handlers`: a relayed message becomes a local `task_updated` with the task
  loaded; `agent_pty_output` is not published.

## Target files

`internal/db/bus.go`, `internal/db/db.go`, `internal/db/trackerops.go`,
`internal/db/dialect_postgres.go`, `internal/handlers/handlers.go`, `cmd/server/main.go`,
tests above, `CHANGELOG.md`, `README.md` (multi-server notes).
