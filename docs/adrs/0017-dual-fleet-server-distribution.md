# ADR 0017: Keep the web server and offer an optional headless Fleet backend

Status: accepted

Date: 2026-10-29

## Context

Tauri is the intended primary desktop for agentic work. Fleet deployments still
need the shared Go API, MCP, authentication, tracker integration and workflow
logic. The existing Go server also hosts web planning features and serves
current web and Electron consumers. Removing those surfaces before choosing a
roadmapping destination or agreeing support windows would strand existing
users.

## Decision

Keep the regular Go server as the default Fleet distribution, including its web
application, API, MCP and authentication. Offer the separately built
`sectile_headless` server as an optional Fleet artifact for deployments that
only need API, MCP and authentication. Tauri Local uses the companion, which
embeds the backend and execution agent in one local process.

Tauri is the destination interface for agentic board and execution work. Web and
Electron remain supported compatibility clients during migration. This
decision does not remove web features, select an external roadmapping tool,
change API or MCP contracts, or set deprecation dates.

## Consequences

- Existing Fleet installations keep the current full server behavior by
  default.
- Operators can choose a headless Fleet artifact without changing the Go
  backend contracts.
- The desktop Local mode does not need a separately launched server process.
- Platform packaging, tested version support and public redistribution remain
  governed by the V1 qualification gates.
- Roadmapping ownership and web/Electron deprecation remain open decisions.
