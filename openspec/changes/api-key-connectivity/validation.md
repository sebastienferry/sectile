# Validation

## Automated
- `go build ./...`, `go vet ./...`, `go test ./...`, `npm test --prefix web`, `npm test --prefix desktop`.
- `internal/db`: migration on a database holding paired workstations keeps them resolving;
  an expired row resolves to nobody with the expiry error; renewal moves `expires_at` only.
- `internal/handlers`: the same key passes the WebSocket handshake, `/api/v1/agent/*`,
  `/mcp`; an expired key is refused everywhere with `API key expired`; the shared server
  token still resolves and logs its deprecation.
- `internal/agent`: the gateway accepts the key and refuses anything else; `pair` writes a
  0600 settings file the daemon starts from; consoles receive the key.
- `internal/agentconfig`: Claude Code gets an HTTP registration against the server; a
  provider without HTTP keeps the bridge with the key in its environment.

## Manual
1. Upgrade a server with a paired desktop: the desktop reconnects without any action, and the
   profile lists the workstation as "no expiry".
2. Create a key from the profile, paste it into `~/.claude.json` as an HTTP MCP server with
   the agent stopped: `list_tasks` answers, `transition_stage` reports the missing agent.
3. Start the agent with that key, then stop and restart it: the Claude Code session keeps
   its tools.
4. Generate a pairing code, run `sectile-agent pair`, start the agent with no `TOKEN` set.
5. Revoke the key: the agent is dropped, the MCP call answers 401, the desktop names it.
