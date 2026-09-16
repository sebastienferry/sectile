# ADR 0007: User identity and agent binding

Status: Accepted

## Context

The server authenticates agents with a single shared bearer token. Any holder of
that token is the same anonymous caller: `resolveAgentUser` returns a constant
`default`, and `validAgentToken` only rejects an empty string. This is sufficient
for one person running one workstation against one server, which is what the
product has been until now.

The product is moving to several people sharing one server, each running their
own local agent. The task board stays shared: everybody sees and edits the same
tasks, as on any tracker board. Identity is therefore not needed to segment data.
It is needed for three other things. Attributing who transitioned, commented or
started a run; routing a run to the right person's machine, clone and worktree;
and revoking one workstation without disturbing the others.

A shared secret cannot express any of that. Worse, the agent currently exports
its server credential into the environment of every console and every session it
opens, so that agent-launched tooling can reach the MCP endpoint. Once that
credential carries an identity, any script, skill or dependency running in those
consoles can read it and act as that person, with no expiry and no revocation.
The local gateway was built to hold the credential, since it attaches the bearer
itself and refuses browser origins, but exporting the same secret beside it
defeats the purpose.

## Decision

Separate the credential that proves *who someone is* from the credential that
proves *a process belongs to this agent session*.

Identity lives on the server. People sign in to the web interface through the
organization's identity provider. From the profile page they obtain a short
lived, single-use pairing code. The desktop app exchanges that code once, at
first launch, for a device credential that binds one workstation to one user.
The agent presents the device credential; the server resolves it to a user and
attributes every action to them. `resolveAgentUser` becomes that lookup, and the
identity guard already enforced on agent dispatch extends to every agent API.

The local relay carries no identity. The agent's HTTP gateway keeps proxying
`/mcp` and `/api/`, attaching the device credential on the way out, and local MCP
clients address the gateway rather than the server. They authenticate to it with
a loopback secret that is generated at each agent start, written to a private
file and never persisted, the mechanism the desktop connection file already
uses. The loopback secret says "this process belongs to this agent session"; it
says nothing about who the user is, and it is worthless anywhere else.

Consequently the device credential never leaves the agent process. Consoles and
sessions receive the gateway URL and the loopback secret, never the credential
that carries the identity.

## Consequences

A stolen loopback secret is bounded by the lifetime of an agent session and by
having local access to the machine already. A stolen device credential is
revocable per workstation, without rotating anything for other users.

The local gateway stops being decorative and becomes the security boundary it
was drafted as. It cannot be removed without either distributing identity
credentials to every MCP client configuration file, or exposing the server
directly to clients that cannot present one.

The MCP tool surface gains the caller's identity for attribution. It gains no
filter: tasks stay shared, and a per-user view would be a product decision, not a
consequence of this one.

Existing single-user installations keep working through one shared device
credential until they pair. The agent no longer accepts an identity credential
from an environment variable alone.

## Naming

The product is Sectile. Environment variables use the `SECTILE_` prefix and the
binaries are `sectile-server` and `sectile-agent`.

Three things deliberately keep the former name, because they address data that
already exists on disk: the agent's local connection file `~/.taskflow/agent-connection.json`
and the repository-level `.taskflow/` files it writes beside a clone, the legacy
MCP registration name that `agentconfig` migrates away from, and the
`<!-- taskflow:project-context -->` markers written into repository files.
Renaming any of them would strand existing installations rather than rename
them. Each needs a migration of its own to move.

The workstation and server data directories are not among them: they moved with
the product. `agentconfig.SettingsPath` returns `~/.config/sectile/settings.json`,
the agent manifest sits beside it, and the server data directory is
`<user config dir>/sectile`, whose only legacy fallback is a `taskacao/tasks.db`
predating the former name.

That move shipped without its migration, and this ADR recorded the intent rather
than the result. Nothing reads the former `~/.config/taskflow/settings.json`, so
a workstation that upgrades starts from empty settings: it loses its server URL,
its device credential and its project mappings, and has to pair again without
being told why. A read-only fallback would not be enough either, since
`WriteSettings` preserves unknown keys — `server`, `secret`, `repo` — only from
the file found at the new path; the first save after such a fallback would drop
them. The fix is a one-shot copy of the legacy file, and of the manifest beside
it, on first read. It is an outstanding gap, not a decision.

See [ADR 0006](0006-independent-server-agent-runtimes.md) for the runtime split
this builds on, and [the interface contract](../contracts/server-agent-v1.md).
