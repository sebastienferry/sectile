# ADR 0007: Server-owned MCP sessions, client-owned runs

Status: Accepted. Refines the run reporting introduced by ADR 0001.

## Context

Run reporting is entirely client-driven. `start_run` and `finish_run` are ordinary
tools, and the only thing that calls them is prompt text carried by the managed
skills. Two failures follow from that. A client that crashes, is killed or is
closed mid-execution never calls `finish_run`, and its run stays active on the
board forever. A client that receives no managed skills reports nothing at all:
skill installation targets Claude Code, Codex and Antigravity, so a session opened
from Claude Desktop is invisible even though it holds a valid MCP connection and
mutates tasks.

The server cannot compensate today. Its MCP endpoint is stateless
(`StreamableHTTPOptions{Stateless: true}` in `internal/handlers/agent_api.go`):
every request builds a temporary session that is discarded when the request ends,
no `Mcp-Session-Id` is read or emitted, and GET and DELETE return 405. There is
therefore no connection to observe, no disconnect to react to, and no way to
correlate two calls from the same client. Identity is equally absent:
`resolveAgentUser` maps any valid bearer to the single `"default"` user, so two
windows of the same client are indistinguishable.

The agent gateway does not help either. Its `/mcp` route is a plain reverse proxy
that only attaches the daemon bearer; it never inspects the protocol. The one
infrastructure-owned closure that exists today — the daemon finishing a run when
its console process exits — works precisely because the daemon owns that process
lifecycle, which no component owns for an external MCP client.

A naive fix, opening a run whenever a tool is called, is wrong on its own terms:
`list_projects` and `list_tasks` carry no task, and reading a task is explicitly
not work. It would fill the board with runs nobody started.

## Decision

Separate the two facts that are currently conflated.

A **session** is an infrastructure fact and belongs to the server. Serve the MCP
endpoint statefully so the connection itself becomes observable. The stdio bridge
already provides a clean boundary: it opens one upstream session at startup and
holds it for the lifetime of the process the client supervises, so the session
begins when the client connects and ends when the client goes away.
`ServerOptions.InitializedHandler` marks the start; a client `DELETE` or a
`KeepAlive` failure marks the end. The bridge carries a client identity beyond the
shared bearer so concurrent sessions of one credential stay distinct.

A **run** is a domain fact and stays with the client. Only the client knows it is
executing a named skill against a task rather than browsing the board, so
`start_run` keeps its explicit contract and its result contract.

The two are joined by **adoption**: a run started inside a session is bound to it,
and the end of the session closes every run it still owns, with a status recording
that the client disappeared rather than reported. `finish_run` remains the way to
close a run early and precisely; it stops being the only thing standing between a
crash and a permanently active task.

A restart is the one ending a session cannot report, because it destroys the
registry along with every session in it. Startup therefore closes the runs those
sessions owned. Telling them apart needs an owner marker that survives the
process, and one already exists: a run's action distinguishes an agent-dispatched
execution from one a client created. Agent-dispatched runs keep the preservation
ADR 0006 gives them, since their supervisor reconnects and reports the real
process exit; a client's run has nothing left that could ever close it.

## Consequences

Runs can no longer outlive their client, not even a restart, and the failure mode
becomes an accurate "client disconnected" instead of a stale active indicator. Sessions from clients
without managed skills, Claude Desktop in particular, appear as connections
without inventing runs for them.

Leaving stateless mode is the substantive cost. Sessions must be tracked, and a
zombie session must not hold a run indefinitely when a disconnect is never
observed. The
`Mcp-Session-Id` header crosses the agent gateway unchanged, so the loopback proxy
keeps its single responsibility and gains no protocol awareness.

Server and clients continue to upgrade together; the bridge's catalog check
already enforces that lockstep. Authentication stays single-user: the client
identity introduced here distinguishes sessions, it does not introduce accounts
and does not replace the deployment's access controls.

See [ADR 0001](0001-mcp-and-agent-configuration.md) for the tool catalog and
transport, and [the interface contract](../contracts/server-agent-v1.md) for the
gateway boundaries.

## Amendment (2026-09-21, #307)

Silence is no longer read as proof of a dead client. The original consequence
expired a session that had said nothing for the idle bound, which closed the runs
it owned; with the stdio bridge and its keepalive gone, a stage that compiles,
tests or waits for its owner crosses that bound while perfectly alive, and lost
its run for it.

The transport is now given no session timeout: only an explicit termination, a
broken connection or the restart sweep ends a session, which is what this ADR
always meant by a client that went away. Sectile keeps a bound of its own,
`SECTILE_MCP_SESSION_TIMEOUT`, defaulting to four hours, but crossing it only
appends one sentence to the runs the session owns — the observation an operator
needs, without the verdict. A run a disconnection did cancel stays recoverable by
its owner through `finish_run`, so an accident no longer costs a chain.

The decision itself — a run belongs to the session that started it, and a session
ending closes it — is unchanged.

## Second amendment (2026-09-24, #319)

The first amendment left one case open on purpose: a client that died without
closing its connection kept its run `running` for as long as the server lived,
holding the board and the chain behind it. That consequence is now bounded, with
two bounds rather than one.

- **The silence bound still only observes.** `SECTILE_MCP_SESSION_TIMEOUT`, four
  hours, appends its sentence, and the board shows the run as *silent*, read off
  that sentence. No column is added; the state lasts until the run ends, even if
  the client speaks again, which is the price of needing no migration.
- **The abandon bound decides.** Past `SECTILE_MCP_SESSION_ABANDON_AFTER`, eight
  hours by default and never less than the silence bound, the session is taken
  for dead: the registry closes it, which cancels its runs with the disconnect
  note, and closes the transport's session so a client that comes back is told
  so. Eight hours was preferred to the 24 first proposed: a run waiting on its
  owner through a working day survives it, and a dead client no longer holds the
  board overnight.
- **The verdict stays reversible.** The owner may still report the real outcome
  through `finish_run`, as for any disconnection. The rewrite now matches the
  disconnect note anywhere in the summary: matched as a prefix, it missed every
  run that had been silenced first, which is exactly the run this bound closes.
- **A human may close one sooner.** A client-created run can be closed from the
  board by its owner or an admin, the way a disconnection would close it, and the
  activities view's cancel goes through the same ownership and hand-back path.

Agent-dispatched runs are out of this: their supervisor reports the real process
exit, as ADR 0006 established.
