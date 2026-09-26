# Design

## Context

The provider-scoped setup migration retired `.taskflow/config.json`. A coding
agent instead receives project context in dispatch context and can obtain the
current project context through the authenticated Sectile MCP
`get_project_context` operation. The remaining references in current
documentation are stale; archived OpenSpec changes and clarification reports
are historical evidence and accurately describe the retired behavior.

## Decisions

### Correct all current-document references together

`AGENTS.md`, the architecture guide, and the server-agent contract describe
the same operational model. Updating all of them keeps instructions and
published technical documentation consistent, without broadening into a source
or protocol change.

### Preserve active neighboring configuration paths

The legacy `.taskflow/agent.json` workstation fallback and the diagnostic
`.taskflow/remote-config.json` snapshot are separate, active behaviors. Their
documentation stays intact so that retiring one file does not imply retiring
the whole `.taskflow/` directory.

### Preserve historical references

Archived specifications and clarification reports remain unchanged because
they record why the retired file existed and how it was removed.

## Rejected Alternatives

- **Restore or migrate `.taskflow/config.json`.** No production component
  consumes it, and the live MCP context flow is the supported replacement.
- **Update only `AGENTS.md`.** This leaves architecture and contract documents
  internally inconsistent.
- **Rewrite all historical references.** That would erase useful migration
  history without changing current behavior.
