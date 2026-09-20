## MODIFIED Requirements

### Requirement: Canonical service and tool discovery
Both supported MCP transports SHALL identify their server as `sectile` and expose exactly these ten tools: `get_task`, `transition_stage`, `add_comment`, `list_tasks`, `get_project_context`, `list_projects`, `start_run`, `finish_run`, `create_task`, and `update_task`. First-party bridge and desktop MCP client identities SHALL use Sectile names.

#### Scenario: Discover either supported transport
- **GIVEN** an upgraded server and workstation agent
- **WHEN** a client initializes and lists tools through Streamable HTTP or stdio
- **THEN** the server name is `sectile` and the catalog contains exactly the ten canonical names
- **AND** tool descriptions refer to the canonical names when referencing other tools.

#### Scenario: Relay a catalog that does not match
- **GIVEN** a stdio relay whose server advertises a catalog that is not exactly the ten canonical names
- **WHEN** the relay enumerates that catalog
- **THEN** it refuses to serve and reports that the server and agent must be upgraded together
- **AND** it does not expose a partial catalog.

### Requirement: Legacy tool names are unsupported
Neither transport SHALL advertise or execute any `sectile_`-prefixed name for a canonical tool, and first-party callers SHALL NOT retry using a legacy name. No `sectile_` tool aliases SHALL be introduced.

#### Scenario: Reject every former name
- **GIVEN** an upgraded deployment and valid arguments for any canonical tool
- **WHEN** a client calls that tool using its `sectile_` name through HTTP or stdio
- **THEN** the call fails as an unknown tool and produces no task, comment, run or tracker mutation.
