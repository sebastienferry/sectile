# One API key for the agent, the desktop app and MCP clients

## Why
Connecting the three parties today takes four different secrets, and the person setting
them up meets all four. The server is started with a shared `SECTILE_SERVER_TOKEN`. The
desktop app exchanges a pairing code for a device credential, which it stores encrypted and
never shows again. The agent generates a loopback session secret at every start and injects
it into the consoles it opens as `SECTILE_AGENT_TOKEN`, so that a `sectile-agent mcp` stdio
bridge registered in the CLI's configuration can reach the agent gateway on `:8091`, which
in turn attaches the device credential upstream. The desktop app talks to the agent with a
fourth token of its own.

The user-facing result, observed on 2026-09-17: a pairing code that the desktop app has no
screen to enter (#201), a bridge that dies with the agent and never reconnects for the rest
of a Claude Code session, a token in `~/.claude.json` that is neither the pairing secret
nor the device credential, and no way to call `/mcp` on the server directly without first
reverse-engineering which of the four secrets it wants. The ephemeral loopback secret was
introduced by ADR 0007 so the identity credential would never leave the agent process. It
buys that isolation at the price of making the agent mandatory for every MCP call and of a
credential story nobody can hold in their head.

## What Changes
- A workstation is authenticated by **one API key**, created from the web profile, shown
  once, hashed at rest, labelled, revocable, with a **90-day** default expiry that can be
  extended and can be disabled. It is the existing device credential with an expiry, a
  recognisable prefix and a visible creation path.
- The same key is the bearer credential on **every machine surface**: the agent WebSocket,
  `/api/v1/agent/*`, `/mcp` on the server, and the agent gateway on `:8091`, which stops
  demanding a session secret of its own.
- The pairing code stays as a one-time exchange for a key, now usable from the agent CLI
  (`sectile-agent pair`) and from the desktop connect screen, and it is optional: pasting a
  key works everywhere a code does.
- The **local agent is optional** for MCP. Managed CLI registrations address the server's
  `/mcp` over Streamable HTTP with the key as bearer; the stdio bridge remains only for
  providers that cannot register an HTTP transport, and is given the key, not a session
  secret.
- The loopback session secret is removed. `SECTILE_SERVER_TOKEN` is accepted for one more
  release with a startup warning, then removed.
- **Migration is automatic**: existing device credentials become keys without expiry and
  keep working; the desktop's stored credential is unchanged.

## Non-goals
Scoped keys (an "MCP only" key distinct from an "agent" key) are deferred: one universal
key was chosen for now. The desktop-to-agent token on `/desktop/*` is internal to the two
processes, invisible to the user, and stays as it is. Web sign-in (ADR 0008) is untouched.

## Impact
`internal/db/identity.go` (expiry column, migration), `internal/handlers/pairing.go` and
`agent_api.go` (key creation, listing, renewal, expiry checks, server-token deprecation),
`internal/agent` (gateway auth, `pair` subcommand, key storage in
`~/.config/sectile/settings.json`, console environment), `internal/agentconfig/mcp.go`
(HTTP registration against the server), `web/src/components/WorkstationPairing.tsx` and
`ProfileModal.tsx` (API keys panel), `desktop/src/main.js` and `desktop/electron`
(connect screen, #201), `README.md`, `docs/contracts/server-agent-v1.md`, ADR 0011.
Supersedes the loopback-secret half of ADR 0007; its identity half stands.
