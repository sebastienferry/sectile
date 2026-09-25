# ADR 0027: Prometheus metrics at /metrics, and "active" read from the sessions

Status: Accepted

## Context

The server had no metrics. Operating it meant reading logs to answer questions
a dashboard should answer: how many requests each controller serves, how fast,
how many fail, how many people use the board, how many runs are in flight.
The same questions, for people rather than for machines, had no home in the
interface either: the admin dialog listed the accounts and nothing else, while
the status bar carried an indicator of connected MCP clients that few people
needed to see every day.

Three things had to be decided: where the metrics are served, what "an active
user" means when several replicas share one database, and how a request is
attributed to a controller without one series per task id.

## Decision

**The metrics are served at `/metrics` on the server's own port**, next to the
interface, not on a listener of their own. A second port is one more thing
every deployment has to declare, route around and keep closed, for a single
read-only route. The path is `/metrics` rather than `/api/v1/metrics` because
it is not part of the REST API: it speaks Prometheus's exposition format, and
`/metrics` is where every scraper looks by default.

Being outside `/api/`, the path is outside the session guard, which a scraper
could not pass anyway. `SECTILE_METRICS_TOKEN`, when set, makes it require that
bearer token, compared in constant time. When it is unset the route is open:
the metrics grant nothing and name nobody, but they do describe how the board
is used, so a deployment reachable from outside either sets the token or keeps
the path off its public ingress. This is not the open mode ADR 0019 removed:
that one handed out an identity, this one hands out counts.

**An active user is an account with a valid browser session seen within the
last five minutes.** `web_sessions` gains `last_seen_at` (migration 15), written
when a session is resolved, at most once a minute per session, and kept fresh
by the event stream of an open tab. The count is a query on the shared
database, so every replica, and the admin page whichever replica answers it,
gives the same figure. An in-memory count per process would have been cheaper
and wrong: a person whose requests land on two replicas would count twice, and
one who is idle on one replica would count as gone.

**Two kinds of series, read two ways.** The HTTP series (requests, errors,
latency) are each process's own and are summed across replicas. The board
series (`sectile_active_users`, `sectile_active_runs`) are read from the
database at scrape time and are the same on every replica: dashboards take
`max()` of them. A failed read leaves the series out of that scrape instead of
reporting zero, so an unavailable database reads as a gap rather than as an
empty board.

**The controller label is the mux pattern**, obtained from
`ServeMux.Handler`, never the path: `/api/tasks/` is one series whatever the
task id. The method is bounded too. Event streams and WebSockets are counted
but kept out of the latency histogram, since their duration is the client's
session, not the controller's work.

The client library is `github.com/prometheus/client_golang`, the reference
implementation. It brings the Go runtime and process collectors, and the
exposition format stays its problem rather than ours.

## Consequences

- A deployment that wants the metrics adds a scrape target on the server's
  port (a `ServiceMonitor` or a pod annotation), with the token when one is set.
  No new port is declared.
- The scrapes themselves appear under `handler="/metrics"` in the HTTP series.
- Every request that resolves a session may write once a minute per session.
  On SQLite that is one small write per open tab per minute.
- The status bar no longer shows the MCP clients nor the active executions
  counter; the Administration page carries both figures, and
  `GET /api/mcp/sessions` remains for whoever needs the list of clients.
