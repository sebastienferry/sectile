# #406: Technical plan

References: [`spec.md`](spec.md), [`docs/clarifications/406.md`](../../docs/clarifications/406.md).

## Schema: migration 7 `agent_presence`

```sql
ALTER TABLE server_instances ADD COLUMN address TEXT NOT NULL DEFAULT '';
CREATE TABLE agent_presence (
    user_id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    instance_id TEXT NOT NULL,
    device_id TEXT NOT NULL DEFAULT '',
    connected_at DATETIME NOT NULL,
    disconnected_at DATETIME,
    PRIMARY KEY (user_id, project_id)
);
```

## `internal/db/presence.go` (new)

- `AgentLocation{UserID, ProjectID, InstanceID, Address, DeviceID, ConnectedAt}`.
- `SetInstanceAddress(addr)`: kept on the store, written by `registerInstance` (so a
  re-registration keeps it).
- `Shared() bool`: `!dialect.ServesOneProcess()`, for the server wiring.
- `InternalToken() (string, error)`: hex HMAC-SHA256 of `sectile-internal-v1` under the
  server key; error when the key is unavailable.
- `AgentConnected(user, project, device) (previous AgentLocation, hadPrevious bool, err)`:
  reads the live previous holder, then upserts this instance with `disconnected_at NULL`.
- `AgentDisconnected(user, project, device)`: sets `disconnected_at` where the row is this
  instance's and device's and still connected.
- `AgentOwner(user, project) (AgentLocation, bool)`: connected rows of live instances, for
  the project then `default`, `all`, empty, in that order.
- `AgentRecentlyConnected(user, project, since) bool`: same fallbacks, connected or
  `connected_at`/`disconnected_at` after `since`.
- `ConnectedAgentLocations() []AgentLocation`: connected rows of live instances.
- `reclaimDeadInstances` also deletes presence rows whose instance no longer exists.

## `internal/handlers`

- `agent_cluster.go` (as built, one file for the directory, the routing, the forwarder and the internal handler): `AgentDirectory` interface (the five presence methods plus
  `InstanceID`), implemented by `*db.DB`.
- `AgentDispatcher.SetCluster(dir AgentDirectory, token string, tokenErr error)`.
- `Register`: after the local bind, `AgentConnected`; a previous holder on another
  instance is asked to close (best effort, in a goroutine). `Unregister`: `AgentDisconnected`.
- Resolution: `waitForRoute(ctx, user, project, grace) *AgentRoute`:
  local map, then owner (ignoring this instance), polling both during the grace when the
  slot was recently held anywhere.
- `CallOperation`, `Dispatch`, `DispatchAndWait`, `PullTasks` fall back to the forwarder
  when the agent is remote. A local-only variant of each serves the internal endpoints,
  so a request is forwarded at most once.
- `Route(user, project) *AgentRoute` (user, project and device of the slot, local or remote) replaces the handlers'
  `Lookup(...) != nil` checks before launches.
- Forwarder: HTTP client, one POST per verb to `<address>/internal/agent/<verb>`
  with the bearer; transport failures become "the server instance holding the local agent
  (<id>) did not answer: …".
- `InternalHandler()` with `operation`, `dispatch`,
  `dispatch-and-wait`, `pull`, `close`; constant-time bearer check (401 otherwise); an answer coded `no_agent` when the
  agent is not held here, so the caller's `ErrNoAgentConnected` is preserved.
- `HandleAgentStatus`: `ConnectedAgentLocations` when a directory is set.

## Server

`cmd/server/main.go`, when `database.Shared()`: internal port from `SECTILE_INTERNAL_PORT`
(8092), address from `SECTILE_INTERNAL_URL` or detected IPv4, `SetInstanceAddress` before
`StartInstance`, `SetCluster`, internal listener in a goroutine. A missing server key logs
a warning; forwarding and the endpoints then refuse.

## Web

`useAgentStatus.ts`: listen to `agent_connected` and `agent_disconnected` named events.

## Tests

- `internal/db/presence_test.go` (SQLite) and `postgres_presence_test.go`.
- `internal/handlers/agent_cluster_test.go`: two dispatchers, shared in-memory directory,
  internal servers via `httptest`; operation, dispatch, dispatch-and-wait, pull, rebind,
  dead owner, bad bearer, no key.
- `web`: existing unit tests; the hook change is covered by review (no hook test harness).

## Target files

`internal/db/{migrations.go,instances.go,presence.go}`, `internal/handlers/{agent_dispatcher.go,
agent_operations.go,agent_cluster.go,handlers.go}`, `cmd/server/{main.go,internal.go}`, `web/src/hooks/useAgentStatus.ts`, tests, README, CHANGELOG.
