## MODIFIED Requirements

### Requirement: Managed registrations converge on Sectile
Normal bootstrap SHALL create or migrate to exactly one managed `sectile` registration and remove the reserved `sectile` registration for Codex, Claude, Antigravity, Gemini, Cursor and Vibe. It SHALL write that registration to the provider's **user-level** configuration location for every supported provider. For a provider that supports a Streamable HTTP transport the registration SHALL address the server's `/mcp` directly with the workstation's API key as bearer; otherwise it SHALL register the stdio bridge against the server URL with the key in its environment. It SHALL preserve unrelated settings and registrations and explicit permission restrictions. It SHALL write the key only into user-level files with owner-only permissions, and SHALL NOT write MCP configuration into a repository or worktree.

#### Scenario: Provider with HTTP transport
- **GIVEN** a workstation with a key and Claude Code selected
- **WHEN** the agent bootstraps the registration
- **THEN** the user-level configuration holds one `sectile` HTTP entry addressing the server
- **AND** the entry keeps working while the agent is stopped

#### Scenario: Provider without HTTP transport
- **GIVEN** a provider whose configuration cannot express an HTTP MCP server
- **WHEN** the agent bootstraps the registration
- **THEN** the `sectile` entry runs the stdio bridge with `--url <server>` and the key in its environment
