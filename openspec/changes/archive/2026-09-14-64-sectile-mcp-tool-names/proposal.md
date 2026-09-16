# Sectile MCP identity and generic tool names

## Why
The product is named Sectile, but MCP discovery and managed client registrations still expose Sectile. GitHub #64 requests a consistent Sectile service identity and generic tool names. The accepted clarification explicitly removes legacy tool support on both agent and server.

## What Changes
- Identify HTTP and stdio MCP servers and managed registrations as `sectile`.
- Remove `sectile_` from all eight tool names; reject old calls without aliases or fallback dispatch.
- Update first-party callers, tool descriptions, launch prompts, built-in/generated instructions and current MCP documentation.
- Migrate the reserved legacy registration for all six supported providers, preserving unrelated configuration and explicit permission restrictions, with idempotent bootstrap and non-destructive error handling.
- Document coordinated upgrades, client reconnection and manual updates to user-owned instructions.

## Capabilities
### New Capabilities
- `sectile-mcp-naming`: canonical MCP identities, tool names, registration migration and caller consistency.

### Modified Capabilities
None. The repository has no baseline capability specification for this contract.

## Impact
Breaking MCP naming change across the Go server, workstation agent, stdio bridge and client configuration bootstrap. Tool schemas, business behavior, authentication and workflow ownership remain unchanged. No new dependency or database migration is required.

## Out of Scope
Environment variable namespaces, storage and configuration paths, database schema, `/mcp`, machine markers, repository/module identity and historical records. User-owned stored instructions must not be rewritten automatically. No production implementation is part of this specification stage.

## Decision Source
`docs/clarifications/64.md` and the accepted clarification on https://github.com/sebastienferry/sectile/issues/64. No open product questions remain. The MCP naming decision supersedes the compatibility approach of change #47 only for MCP tool names and managed service registration.
