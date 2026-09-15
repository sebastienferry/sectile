# ADR 0005: Sectile MCP naming and conservative registration migration

Status: Accepted for implementation of #64

## Context

The earlier product rename preserved machine contracts. The accepted clarification
for #64 explicitly requires removal of legacy MCP tool names on server and agent.
Existing client permissions can refer to both the service namespace and tool names;
retaining an obsolete deny rule would silently weaken its protection.

## Decision

Keep the shared typed MCP catalog and HTTP/stdio architecture from ADR 0001.
Identify both servers as `sectile` and remove `sectile_` from the eight tools.
Do not register aliases, translate calls or retry using old names.

Migrate the reserved registration during normal bootstrap, preserving unrelated
settings and explicit restrictions. Normalize exact references in recognized
server-scoped policy lists. Keep current Sectile non-transport settings; reject
conflicting policies, ambiguous patterns or unknown legacy references before
writing. Missing policy values on either side of a collision also require manual
reconciliation, since their provider defaults may differ from explicit values.

Keep configuration writes atomic within the existing file. Separate policy files
are read for known legacy references and never automatically rewritten; report
manual reconciliation rather than attempt a multi-file transaction. Enterprise
and custom policy sources are the operator's responsibility. Preserve stored
custom skills and the existing backup mechanism for refreshed managed files.

## Consequences

Server and agent upgrades and client reconnection must be coordinated. Old calls
fail visibly. Conservative errors may require manual configuration changes, but
bootstrap never silently combines incompatible approval or filtering policies.
Environment namespaces, storage paths, DTOs, `/mcp` and workflow services remain
unchanged. Historical decisions are retained; this decision supersedes #47 only
for MCP tool and managed service naming.
