# Technical design

## Context
The accepted clarification is the behavior authority. `internal/taskmcp/server.go` registers eight typed tools; `cmd/server/mcp.go` forwards the upstream catalog through a database-free stdio server. `internal/agentconfig/mcp.go` bootstraps six providers. The desktop completion path calls a hard-coded tool. This is an intentional breaking naming change, not a protocol or workflow redesign.

## Identity and tool mapping
HTTP and stdio server implementation names become `sectile`; bridge client identity becomes `sectile-stdio`; desktop client identity becomes `sectile-desktop-agent`. Keep current version fields.

| Former tool | Canonical tool |
| --- | --- |
| `taskflow_get_task` | `get_task` |
| `taskflow_transition_stage` | `transition_stage` |
| `taskflow_add_comment` | `add_comment` |
| `taskflow_list_tasks` | `list_tasks` |
| `taskflow_get_project_context` | `get_project_context` |
| `taskflow_list_projects` | `list_projects` |
| `taskflow_start_run` | `start_run` |
| `taskflow_finish_run` | `finish_run` |

Change registration names and cross-references without changing handler bodies, schemas or delegation to database workflow services. Validate that the upstream catalog contains exactly the eight canonical tools before serving stdio clients, then retain transparent forwarding and upstream errors. Do not introduce a name translation or compatibility layer at runtime.

## Registration migration
Retain the existing parser, rooted filesystem access and temporary-file rename pattern in `internal/agentconfig/mcp.go`. Validate the whole prospective migration before writing. Reuse current transport-field replacement rules, credential exclusion, absolute-executable and loopback-gateway checks.

| Provider | Existing location | Registration shape |
| --- | --- | --- |
| Codex | `.codex/config.toml` | `mcp_servers` map |
| Claude | `.mcp.json` | `mcpServers` map |
| Antigravity (`agy`) | user `~/.gemini/config/mcp_config.json` | `mcpServers` map |
| Gemini | `.gemini/settings.json` | `mcpServers` map |
| Cursor | `.cursor/mcp.json` | `mcpServers` map |
| Vibe | `.vibe/config.toml` | `mcp_servers` array, entry `name` |

Construct a single `sectile` entry from the existing canonical entry, otherwise the legacy entry, otherwise defaults. Preserve non-transport fields for Vibe as well as map-based providers. Antigravity keeps `mcp` arguments without a process-specific URL; others refresh `mcp --url <gateway>` as today. Delete the legacy entry only after reconciliation succeeds. Preserve all unrelated array entries and map keys. Reject malformed managed entries and duplicate managed array entries that cannot be reconciled safely.

Permission preservation is semantic: leaving a deny rule pointing at a nonexistent old tool would silently remove its protection. For recognized provider policy fields, map exact references to the eight old tool names and the reserved service namespace to their canonical equivalents while preserving allow/deny and approval behavior. Do not perform global text replacement. Keep unrelated tool references intact. Preserve restrictions outside the managed entry; if preserving their effect would require an unsupported rewrite, fail with an actionable manual-migration error before writing.

For both-name collisions, retain existing Sectile non-transport values. Equal normalized restrictions can be deduplicated; missing legacy restrictions may be carried over only when their semantics remain intact. For conflicting or unrecognized policy combinations, return an error identifying the configuration and conflicting fields, without logging secrets or modifying bytes. This conservative error path is the chosen implementation of the clarification's unsafe-reconciliation rule; do not invent a permissive merge of policy values. Repeated bootstrap must be semantically idempotent.

## Callers, instructions and documentation
Update `cmd/server/agent.go` launch text, `cmd/server/agent_desktop.go` client identity/completion call, `internal/db/skilltemplates.go` and `internal/db/projectskills.go`, plus any remaining first-party producers discovered by a targeted search. Preserve `TASKFLOW_RUN_ID` and all other environment contracts. Retain `internal/agentconfig/local.go` manifest and backup behavior. Inspect template refresh logic so built-in defaults move forward without overwriting stored custom overrides. Test generated outputs; do not blindly replace user-owned skills or pre-existing worktree edits.

Update `README.md`, `docs/ARCHITECTURE.md` and `docs/contracts/server-agent-v1.md` for current names, all eight tools and migration steps. Correct current registration examples consistently, including Antigravity's user-level registry. Preserve historical files and markers. Add an ADR documenting the deliberate break with MCP naming compatibility and conservative permission migration, referencing ADR 0001 and change #47; no new runtime architecture is introduced. Update a changelog if one exists at implementation time.

## Rollout and failure handling
Coordinate server and agent deployment, then run normal bootstrap, manually update customized instructions/unsupported policies, and reconnect native clients to discard cached catalogs. Mixed versions and existing client sessions are not supported by aliases; unknown-tool failures must surface. A rollback requires coordinated binary and registration/instruction restoration rather than old-name fallback calls. No data migration is involved.

## Validation plan
- `internal/handlers/mcp_test.go`: HTTP initialization identity, exact eight-tool set, schemas, valid calls and all eight legacy unknown-tool failures with no mutation; retain authentication and workflow guard tests.
- `cmd/server/agent_config_test.go` and adjacent desktop/launch tests: stdio identity/catalog parity, canonical round trips, every legacy rejection, desktop completion with the original run ID, and canonical launch text.
- `internal/agentconfig/mcp_test.go`: table-driven six-provider coverage for fresh, legacy-only, canonical-only and both-name inputs; gateway refresh and repeated bootstrap; preserved unrelated settings, Vibe policies, explicit tool restrictions and namespace references; invalid shapes, duplicate/conflicting entries, unsupported policies, byte-identical failure and absence of persisted credentials. Keep provider-reader smoke tests conditional on executable availability.
- `internal/db/skills_test.go` and local skill refresh tests: canonical built-in/project policy output, preserved custom overrides and backup behavior for modified managed files.
- Run `openspec validate 64-sectile-mcp-tool-names --strict`, `git diff --check`, `go test ./...`, `go vet ./...`, `make test` and `make binary-build` during implementation. These use the repository's Go/frontend checks and build without requiring desktop packaging for an MCP-only change. Run the focused MCP tests again against the built bridge if the existing integration harness requires an executable.
- Search active first-party MCP sources/docs for old names, classifying retained migration inputs, negative tests and historical references rather than asserting that every `taskflow` string must disappear.

## Rejected alternatives
- Prefixing tools with `sectile_`: contradicts generic names.
- Keeping old aliases or caller fallbacks: contradicts the explicit user decision.
- Renaming only metadata: leaves provider namespaces and callers stale.
- Blind config replacement or permissive collision merging: loses settings or weakens restrictions.
- Broad replacement across storage, environment and custom instructions: exceeds the accepted scope.

## Open questions
None. Unrecognized/conflicting configuration policies fail non-destructively and require an operator correction; they do not authorize broader permissions.

## Implementation notes

The implementation automatically maps exact tool references in Codex
`enabled_tools`/`disabled_tools`, Gemini `includeTools`/`excludeTools`, and
Antigravity `disabledTools`. Other fields retain their values; unsupported legacy
references require manual reconciliation. Prefixed globs and regular expressions
are not rewritten. Both-name collisions with missing explicit policy values also
fail conservatively because provider defaults can differ.

Known separate provider settings are parsed and checked before registration is
written, including Claude local settings and Cursor permissions files. Vibe root
policies and enterprise/custom policy sources require manual updates as documented.
The web copy-command menu was included in the first-party caller inventory.
Checked-in managed instructions receive only name substitutions; unrelated local
workflow edits remain uncommitted and the preservation stash retains their originals.
