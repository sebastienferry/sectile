# ADR 0011: One API key for the agent, the desktop app and MCP clients

Status: Accepted

## Context

ADR 0007 split the credential that proves who someone is from the credential that
proves a process belongs to an agent session. The first is the device credential
obtained by redeeming a pairing code; the second is a loopback secret generated at
each agent start, exported to every console as `SECTILE_AGENT_TOKEN`, and checked by
the agent gateway before it attaches the device credential upstream. The shared
`SECTILE_SERVER_TOKEN` from before identity existed was kept beside them, and the
desktop app talks to the agent with a fourth token.

Living with that for two weeks showed the cost. The pairing code the interface asks
for has no screen to receive it (#201). The CLI registration is a stdio bridge to
the agent gateway, so a read-only MCP call needs a running agent, and the bridge
dies with the agent for the rest of the Claude Code session. Nobody can call `/mcp`
on the server by hand, because the only credential that would work is hidden in the
desktop's encrypted settings. Four secrets for one person and one machine is a
setup no one can hold in their head.

## Decision

A workstation is authenticated by one API key, and that key is the device
credential of ADR 0007 with three additions: an expiry, a `sectile_` prefix, and a
visible creation path in the web profile where it is shown once, listed, renewed
and revoked. The default expiry is 90 days; the user may choose none. Renewal
extends the date without changing the secret.

The same key is the bearer credential on every machine surface: the agent
WebSocket, `/api/v1/agent/*`, `/mcp` on the server, and the agent gateway. The
loopback session secret is removed. The pairing code remains as a one-time exchange
for a key, from the agent CLI and the desktop, and is optional.

Managed CLI registrations address the server's `/mcp` over Streamable HTTP with the
key as bearer, so the local agent is not required for MCP. The stdio bridge remains
only for providers that cannot register an HTTP transport, and receives the key.

Existing device credentials become keys without expiry on upgrade, with no action
required. `SECTILE_SERVER_TOKEN` is accepted for one more release with a warning,
then removed. One universal key is chosen; scopes are deferred until a second scope
exists.

## Consequences

The isolation ADR 0007 bought is given up knowingly: the identity credential now
sits in the CLI's user-level configuration and in the environment of agent
consoles, readable by any process running as the user. That is the threat model of
every developer credential on the machine already, `gh`, `git` and cloud CLIs
included, and the key carries the two controls ADR 0007 asked for in return:
revocation per workstation and expiry. The desktop-to-agent token stays, because
it is an inter-process contract the user never sees.

The agent gateway stops being a security boundary and becomes a convenience proxy;
it is a candidate for removal once consoles address the server directly. Setup
collapses to one step: create a key or redeem a pairing code, then paste the same
value wherever a Sectile client asks for one.

ADR 0007 is superseded in part: its identity half, one credential per workstation
bound to one user and revocable alone, stands. Its loopback-secret half is
replaced by this decision. See the `api-key-connectivity` change for the delta
specifications and the migration.
