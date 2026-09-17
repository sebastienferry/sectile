# Tasks

## 1. Key model and storage
- [x] 1.1 Add `expires_at` to `device_credentials` with a migration that leaves existing rows NULL.
- [x] 1.2 Mint keys with the `sectile_` prefix; reject expired keys with a distinct error in
      `UserForDeviceToken`.
- [x] 1.3 Add creation with label and expiry choice (90 days default, none), renewal by the
      same period, listing with expiry, and revocation.

## 2. Server surfaces
- [x] 2.1 Route `/api/devices` creation and renewal; keep listing and revocation.
- [x] 2.2 Return `401 API key expired` distinctly on every bearer surface.
- [x] 2.3 Accept `SECTILE_SERVER_TOKEN` with a startup warning and log the deprecation on use.
- [x] 2.4 Have `POST /api/v1/agent/pair` mint a key with the default expiry.

## 3. Web profile
- [x] 3.1 Replace the workstation pairing panel with an API keys panel: create, show once,
      copy, list, renew, revoke, expiry badge, "no expiry" for migrated rows.
- [x] 3.2 Keep the pairing code generator inside that panel as the one-time alternative.

## 4. Agent
- [x] 4.1 Add `sectile-agent pair --url --code`, storing the key in `~/.config/sectile/settings.json`
      (mode 0600); read it when `--token` and `TOKEN` are absent.
- [x] 4.2 Authenticate the gateway with the key; remove the loopback session secret.
- [x] 4.3 Export the key and the server URL to consoles; drop the gateway URL from `SECTILE_AGENT_URL`
      defaults where the provider registers HTTP directly.
- [x] 4.4 Log remaining validity under ten days at connect; print the expiry reason on 401.

## 5. MCP registration
- [x] 5.1 Write a Streamable HTTP registration against the server for providers that support it,
      verifying each provider's current schema.
- [x] 5.2 Keep the stdio bridge for the others, with `--url <server>` and the key in its environment.

## 6. Desktop (closes #201)
- [x] 6.1 Add the pairing code field to the connect screen and call the existing `pair` IPC.
- [x] 6.2 Persist the key without `safeStorage` in the 0600 settings file, so a code is never lost.
- [x] 6.3 Show "API key expired" as its own message.

## 7. Documentation
- [x] 7.1 README: one setup path (create key or pairing code), no `SECTILE_SERVER_TOKEN` in
      the default instructions, direct MCP configuration for Claude Code.
- [x] 7.2 `docs/contracts/server-agent-v1.md`: transport table and authentication paragraphs.
- [x] 7.3 ADR 0011 accepted; ADR 0007 marked superseded in part.

## 8. Tests
- [x] 8.1 Expired key rejected on WebSocket, agent API, `/mcp` and gateway; valid key accepted on all.
- [x] 8.2 Migration leaves existing credentials usable with no expiry.
- [x] 8.3 Renewal keeps the secret; revocation cuts every surface.
- [x] 8.4 `pair` stores the key and the agent starts from it without `TOKEN`.
- [x] 8.5 HTTP registration written for Claude Code; bridge kept for a provider without HTTP.
