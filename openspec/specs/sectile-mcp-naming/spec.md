# sectile-mcp-naming Specification

## Purpose
TBD - created by archiving change 64-sectile-mcp-tool-names. Update Purpose after archive.
## Requirements
### Requirement: Canonical service and tool discovery
Both supported MCP transports SHALL identify their server as `sectile` and expose exactly these eight tools: `get_task`, `transition_stage`, `add_comment`, `list_tasks`, `get_project_context`, `list_projects`, `start_run`, and `finish_run`. First-party bridge and desktop MCP client identities SHALL use Sectile names.

#### Scenario: Discover either supported transport
- **GIVEN** an upgraded server and workstation agent
- **WHEN** a client initializes and lists tools through Streamable HTTP or stdio
- **THEN** the server name is `sectile` and the catalog contains exactly the eight canonical names
- **AND** tool descriptions refer to the canonical names when referencing other tools.

### Requirement: Renaming preserves existing tool contracts
Each canonical tool SHALL preserve its former argument and response contracts, validation, authentication, effects and workflow ownership rules.

#### Scenario: Use canonical calls
- **GIVEN** valid inputs and authorized access for each of the eight tools
- **WHEN** the corresponding canonical tool is called through either transport
- **THEN** it returns the same response shape and performs the same operation as its previous prefixed counterpart.

#### Scenario: Preserve rejected requests and run ownership
- **GIVEN** an invalid argument, unauthorized connection, prohibited managed-run transition, or a run owned by another task
- **WHEN** the corresponding canonical request is attempted
- **THEN** the existing rejection and state-preservation behavior remains in effect
- **AND** reading a task or finishing a run does not implicitly advance its workflow stage.

### Requirement: Legacy tool names are unsupported
Neither transport SHALL advertise or execute any of the eight `sectile_` tool names, and first-party callers SHALL NOT retry using a legacy name. No `sectile_` tool aliases SHALL be introduced.

#### Scenario: Reject every former name
- **GIVEN** an upgraded deployment and valid arguments for any former tool
- **WHEN** a client calls that tool using its `sectile_` name through HTTP or stdio
- **THEN** the call fails as an unknown tool and produces no task, comment, run or tracker mutation.

### Requirement: Managed registrations converge on Sectile
Normal bootstrap SHALL create or migrate to exactly one managed `sectile` registration and remove the reserved `sectile` registration for Codex, Claude, Antigravity, Gemini, Cursor and Vibe. It SHALL write that registration to the provider's **user-level** configuration location for every supported provider. It SHALL preserve unrelated settings and registrations and explicit permission restrictions. It SHALL NOT persist bearer credentials, and SHALL NOT write MCP configuration into a repository or worktree.

#### Scenario: Fresh or legacy-only configuration
- **GIVEN** a supported provider with no managed registration or only a legacy `sectile` registration
- **WHEN** bootstrap succeeds
- **THEN** exactly one `sectile` registration uses the current managed connection settings and no `sectile` registration remains
- **AND** unrelated content and the effective explicit restrictions on the corresponding renamed tools are preserved.

#### Scenario: Repeat bootstrap or refresh the gateway
- **GIVEN** an already migrated configuration
- **WHEN** bootstrap runs again, including with a changed executable or gateway
- **THEN** it refreshes managed connection settings without duplicates or permission changes
- **AND** Antigravity retains its shared user-level registration without a process-specific gateway.

#### Scenario: Both registrations already exist
- **GIVEN** existing `sectile` and `sectile` registrations
- **WHEN** bootstrap can reconcile them without discarding Sectile settings or broadening either entry's explicit permissions
- **THEN** it retains the Sectile configuration, refreshes managed connection settings and removes the redundant legacy registration.

#### Scenario: Unsafe or malformed configuration
- **GIVEN** malformed configuration, invalid managed entries, conflicting restrictions, or a policy whose effective permissions cannot safely be preserved
- **WHEN** bootstrap attempts migration
- **THEN** it returns a clear actionable error and leaves the original file unchanged.

#### Scenario: Registration written outside the checkout
- **GIVEN** any supported provider
- **WHEN** bootstrap succeeds
- **THEN** the modified configuration file lies under the user's configuration home and not under the repository or worktree.

### Requirement: First-party instructions and callers use the new contract
First-party server and agent calls, native launch prompts, built-in workflow templates, generated policy text and refreshed managed skills SHALL use canonical tool names and Sectile MCP naming. Existing run IDs and ownership semantics SHALL be retained.

#### Scenario: Launch and finish a native execution
- **GIVEN** a launched execution with a supplied run ID
- **WHEN** the generated instructions start and finish the invocation or the desktop reports process exit
- **THEN** they use `start_run` and `finish_run` as applicable with the supplied ID
- **AND** they do not create a duplicate run or issue a legacy fallback call.

#### Scenario: Preserve customized instructions
- **GIVEN** a stored project-owned skill override or a manually edited managed skill file
- **WHEN** templates are upgraded and managed files are refreshed
- **THEN** the stored custom override is preserved and existing backup behavior protects local edits
- **AND** migration documentation explains how to update custom tool references manually.

### Requirement: Upgrade guidance defines the breaking naming boundary
Current MCP documentation SHALL present Sectile service identities, canonical tools, supported registration locations and executable examples. It SHALL explain coordinated server/agent upgrades, normal registration migration, client reconnection and manual custom-instruction updates, without promising compatibility with legacy calls.

#### Scenario: Follow upgrade guidance
- **GIVEN** an installation with a legacy agent, cached client catalog and custom instructions
- **WHEN** an operator follows the documented upgrade sequence
- **THEN** the operator upgrades both server and agent, reruns registration bootstrap, updates custom references and reconnects clients before relying on the new catalog
- **AND** the guidance identifies unsupported legacy calls and preserved environment variables, paths, endpoint and protocol markers.

#### Scenario: Preserve unrelated compatibility contracts
- **GIVEN** existing environment configuration, data and historical records
- **WHEN** the MCP naming change is installed
- **THEN** existing `SECTILE_*` variables, `.taskflow/` and data paths, `/mcp`, machine markers, module and repository identities continue unchanged
- **AND** historical clarification, specification and decision records are retained.

