# ADR 0027: Prometheus metrics on their own listener, and "active" read from the sessions

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

**The metrics are served on a listener of their own**, `SECTILE_METRICS_ADDR`
(`:8093` by default, `off` to disable), at `/metrics`, with no authentication.
It follows the internal port's rule: declared on the container, never routed by
the ingress. Serving them on the interface's port would have put them behind
the session guard, which a scraper cannot pass, or in front of it, which
publishes who uses the board to anyone who can reach the ingress.

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

- A deployment that wants the metrics declares port 8093 on the container and a
  scrape target (a `ServiceMonitor` or a pod annotation); nothing changes for
  one that does not.
- A desktop or local run opens port 8093 as well; `SECTILE_METRICS_ADDR=off`
  closes it, and a port already taken only costs the metrics.
- Every request that resolves a session may write once a minute per session.
  On SQLite that is one small write per open tab per minute.
- The status bar no longer shows the MCP clients; `GET /api/mcp/sessions`
  remains for whoever needs the list.
