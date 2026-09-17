# Design

## Context
Four secrets exist because three concerns were solved one at a time.

`SECTILE_SERVER_TOKEN` (`internal/handlers/agent_api.go`, `validAgentToken`) predates
identity: any holder is the implicit user. ADR 0007 added `device_credentials`, minted by
redeeming a `pairing_codes` row (`internal/db/identity.go`), so an agent resolves to a
person and a workstation can be revoked alone. The same ADR decided that credential must
never leave the agent process, because the agent exports its environment to every console
it opens. Hence the loopback secret: `rand.Text()` at each start (`internal/agent/agent.go`,
`loopback.token`), exported as `SECTILE_AGENT_TOKEN`, checked by `validLoopbackRequest` on
the gateway, which then attaches the device credential upstream. The desktop app adds
`SECTILE_DESKTOP_TOKEN` for `/desktop/*`.

What that isolation costs in practice: the CLI registration written by
`agentconfig.BootstrapMCP` is a stdio bridge pointed at the gateway, so it needs a running
agent for a read-only `list_tasks`; the bridge process does not survive an agent restart;
and a person who wants to call `/mcp` from anywhere else has no credential to give it, since
the device credential is hidden inside the desktop's encrypted settings. The pairing code,
the one thing the interface tells the user to use, has no screen to receive it (#201).

## Decisions

### The device credential becomes the API key, with an expiry
No new table. `device_credentials` gains `expires_at DATETIME` (NULL means none). The key
stays 256 bits of randomness, stored as a SHA-256 hash, and gains the prefix `sectile_` so
a pasted value can be told from a short pairing code and so a leaked one is recognisable in
a scanner. `UserForDeviceToken` rejects a row whose `expires_at` has passed, with a distinct
error so the caller can say "expired" rather than "invalid".

**Default 90 days**, chosen per user on creation among 90 days and none. Renewal extends
`expires_at` by the same period without changing the secret, so nothing on the workstation
has to be reconfigured. Rotation is revoke-and-create. The web profile lists keys with
label, creation, last use, expiry, and the three actions.

**Rejected:** a separate `api_keys` table beside `device_credentials`. Two tables for one
concept, and the migration would have had to copy rows the agent is presenting right now.

### One key on every machine surface
`resolveAgentUser` is already the single resolver for the WebSocket handshake, the agent
API and `/mcp`. The gateway stops checking `loopback.token` and runs the same bearer check,
comparing against the key the agent itself holds, so a console launched by the agent and a
CLI configured by hand present the same thing. `/desktop/*` keeps its own token: it is an
inter-process contract between the desktop app and the agent it spawned, and the user never
sees it.

**Rejected:** keeping the loopback secret and merely documenting it. The secret is the
reason the agent is mandatory for MCP and the reason the bridge cannot be reconfigured by
hand. Its threat model, a script in an agent console reading the identity credential, is
the same as `gh` or `git` credentials in that console's home directory, and the key now
carries the two mitigations ADR 0007 asked for: revocation per workstation and expiry.

**Rejected:** scoped keys. Nothing today distinguishes what an agent may do from what an
MCP client may do; adding a scope column with a single value would be ceremony. The column
can be added when a second scope exists.

### Pairing is a one-time exchange, from any client
`POST /api/v1/agent/pair` keeps its contract and mints a key with the default expiry.
`sectile-agent pair --url <server> --code <code>` calls it and stores the key in
`~/.config/sectile/settings.json` under `secret`, mode 0600, the file the agent already
reads. The agent then starts without `TOKEN`: the flag and the variable stay as overrides.
The desktop connect screen gets the code field it lacks, calling the `pair` IPC that already
exists, and persists the key even where `safeStorage` is unavailable, in the same 0600 file,
so a single-use code is never spent for nothing.

### MCP registrations address the server, over HTTP
`BootstrapMCP` writes, for providers that support it (Claude Code first), a Streamable HTTP
entry: `{"type":"http","url":"<server>/mcp","headers":{"Authorization":"Bearer <key>"}}`.
The server URL is stable and independent of the agent, so Claude Code keeps its tools when
the agent is stopped or restarted; only agent-routed tools (`transition_stage` with local
evidence, runs dispatched to a workstation) report the absence of an agent, which the server
already does. Providers without HTTP transport keep the stdio bridge, `--url <server>`, with
`SECTILE_AGENT_TOKEN` set to the key. The `sectile-mcp-naming` requirement that forbids
persisting bearer credentials in managed registrations is amended accordingly: the key is
written with the user's file permissions, is revocable and expires.

### Deprecation and migration
On first start after upgrade, the schema migration adds `expires_at` with NULL for every
existing row: every paired workstation becomes a key without expiry and nothing breaks. The
profile shows those as "no expiry" and offers to set one. `SECTILE_SERVER_TOKEN` is still
resolved to the implicit user for one release, with a warning at startup and a line in the
profile inviting the user to create a key; the next release drops `validAgentToken`.

### Telling the user before it expires
The agent logs the remaining validity at connect when under ten days, and the web profile
flags such keys. A rejected key returns `401 {"error":"API key expired"}` distinct from an
invalid one, and the agent and desktop print that reason instead of "check your token".

## Risks
A key in `~/.claude.json` is readable by any process running as the user; that is the
accepted trade-off, bounded by expiry and revocation. The HTTP registration format differs
per provider and must be verified against each one's current schema before it is written.
