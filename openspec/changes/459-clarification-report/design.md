## Decisions
Use a numbered integer default-zero database migration and a boolean JSON field named pushStageCommits. Follow fullChainStopStage through project requests, database reads and writes, agent configuration and MCP context. Add the control to the workflow tab. Update canonical skill fragments and regenerate goldens, rather than editing installed skill copies.

Every clarify round publishes its new section, regardless of execution mode. Standalone intermediate rounds use add_comment; the final round uses transition_stage only. Managed runs put the section in their result note and call neither tool. Split oversized sections into ordered numbered parts, preserving the final transition contract. Plain pushes use -u on first publication, never force, and failures are reported without blocking the stage. Ignored artifacts are never committed or force-added.

## Alternatives
Always pushing would violate the default-off choice. Linking local reports alone leaves the ticket unreadable. Changing other stage comments is outside the settled scope.
