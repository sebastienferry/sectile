# Retire managed AGENTS.md project context

## Why

The provider-scoped setup no longer writes a managed project-context block, but checkout cleanup recognizes only legacy `taskflow` markers. Existing `sectile` blocks therefore remain, including one tracked in this repository.

## What Changes

- Remove complete managed project-context blocks with either `sectile` or `taskflow` markers during normal scaffold cleanup.
- Preserve all instructions outside managed blocks and retain an error for incomplete markers.
- Remove this repository's tracked managed block from `AGENTS.md`.

## Scope

Project identity and workflow context continue to come from dispatch and authenticated MCP. This change does not alter those sources or unrelated `AGENTS.md` instructions.
