# Specification #517 - MCP sessions kept alive, orphans released

Task: `gh-11a4f59c-b6bd-4747-8ea5-0a10d5b78da4-517`
Clarification: `docs/clarifications/517.md`
Branch: `fix/batch-411-499-517`

## Summary

A proxy cuts the silent `GET /mcp` stream after 50s, and the client then opens a
new session every ~152s, leaving the old one registered for 8 hours. The
server keeps each stream busy with a ping, and releases a session without
adopted runs once its client stops answering.

## Scope

In scope: server-side keepalive in the MCP session registry, its two settings
and their documentation. Out of scope: the ingress timeout (deployment
repository), `/api/events`, `/ws/agent-connect`, and the 4h/8h silence and
abandon rules, which keep their meaning.

## Definitions

- **Adopted run**: a run a session created itself with `start_run` without a runId.
- **Failed ping**: a ping that returns an error or times out.
- **Client message**: any request or notification the client sends (what `Touch` records).

## User stories (prioritised)

### US1 - One session per client across silences (P1)

- **Given** a connected client with its standalone stream open and a keepalive interval shorter than the proxy's idle timeout,
  **when** the client says nothing for longer than that idle timeout,
  **then** it keeps the same session, and the server lists one session for it.

### US2 - An orphaned session is released (P1)

- **Given** a session with no adopted run whose client no longer answers pings and sends nothing,
  **when** the configured number of consecutive pings fail,
  **then** the session is closed and removed from the registry, and its goroutines are released.
- **Given** a session whose pings fail but whose client keeps sending messages,
  **then** it is not closed: a client message resets the failure count.

### US3 - A session holding a run is left to the existing rules (P1)

- **Given** a session with an adopted run whose client stops answering,
  **when** pings keep failing,
  **then** the keepalive does not close it and its run stays open. The silence notice (4h) and the abandon (8h) apply as before.

### US4 - A ping reply is not the client speaking (P2)

- **Given** a session answering pings and sending nothing else,
  **then** its last activity is not updated by the replies, and the silence notice is posted at the usual bound.

### US5 - Configurable (P3)

- **Given** `SECTILE_MCP_KEEPALIVE_INTERVAL` and `SECTILE_MCP_KEEPALIVE_FAILURES` set to valid values,
  **then** the registry uses them. An invalid or non-positive value keeps the default (25s, 3).

## Functional requirements

| ID | Requirement |
| --- | --- |
| FR1 | The registry pings every live session with a transport every interval (default 25s), with a timeout of half the interval. |
| FR2 | A session with no adopted run is closed after N consecutive failed pings (default 3) with no client message in between. Closing releases the transport session. |
| FR3 | A session with an adopted run is never closed by the keepalive. |
| FR4 | Ping replies do not update the session's last activity. |
| FR5 | `SECTILE_MCP_KEEPALIVE_INTERVAL` (duration) and `SECTILE_MCP_KEEPALIVE_FAILURES` (positive integer) override the defaults; invalid values keep them. |
| FR6 | The settings are documented in `README.md` next to the other MCP session settings. |
| FR7 | `CHANGELOG.md` gets a `Fixed` line under `[Unreleased]`. |

## Success criteria

- Tests for US1 to US5, with short intervals, over a real streamable HTTP transport.
- After deployment (owner): `go_goroutines` flat over 24h and about one session per connected client in `GET /api/mcp/sessions`.

## Open questions

None. The last success criterion can only be checked after deployment.
