# ADR 0067: Build a headless API server without web assets

- Date: 2026-10-29
- Status: Accepted

## Context

The V1 product direction makes Sectile Desktop the primary interface for
agentic work while retaining the Go backend's API, MCP, authentication,
tracker adapters and workflow rules. The standalone Fleet server currently
embeds and serves the web application. This couples API/MCP deployment to the
web bundle even when a deployment intentionally has no browser interface.

The Local companion already consumes the shared backend without website
assets. A separately built headless server is useful for Fleet deployments and
provides a clear server artifact for the desktop-first product direction. It
must not change the existing Fleet server binary or remove web pages, routes,
or data.

## Decision

Add an opt-in Go build variant named `sectile_headless` for `cmd/server`. The
variant omits the embedded web assets and returns `404` for browser paths while
retaining API, MCP, authentication, tracker, workflow and agent relay routes.
Expose it through a separate `make build-server-headless` target that does not
require Node or build the web application.

The default server build continues embedding and serving the web application.
The headless variant is a separate artifact, not a runtime switch, so an
existing Fleet deployment cannot silently lose its UI through configuration
drift. Neither artifact is selected or distributed by default by this ADR.

## Consequences

- Desktop-first and API/MCP-only Fleet deployments can build the server without
  web dependencies or embedded web assets.
- The normal server build, existing browser deployments and their APIs remain
  unchanged.
- Both artifacts must keep the same API/MCP contracts and version reporting.
- Packaging and redistributing Go services remains subject to the existing
  licensing and provenance gate; this ADR does not resolve it.
- A V1 package may choose the headless artifact only after compatibility,
  upgrade and deployment documentation gates pass.
