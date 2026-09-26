# ADR 0037: Explicit public hosts for MCP behind a loopback ingress

Status: Accepted

## Context

The MCP Go SDK rejects a non-loopback Host when the request's local socket
address is loopback. A hosting ingress can legitimately preserve the public
Host while connecting to the server on `127.0.0.1` or `::1`. This made Sectile's
MCP initialization fail with `403 Forbidden: invalid Host header` even though
the agent's project discovery and WebSocket connection worked.

An unauthenticated probe is misleading: Sectile's bearer middleware returns
401 before invoking the SDK. The response's `Server: envoy` header also does
not identify the source of the error; the SDK generates the rejection inside
the application.

## Decision

The server reads `SECTILE_MCP_ALLOWED_HOSTS`, a comma-separated list of exact
Host authorities, when it constructs its shared MCP transport. Matching ignores
case but preserves ports. Entries contain neither schemes nor paths, and no
wildcards are expanded. Each server replica must receive the setting.

Sectile replaces the SDK's fixed loopback check with the same check extended by
this explicit list. With an empty list, behavior is unchanged. A loopback
connection accepts a loopback Host or a listed authority; other hosts receive
the existing 403. Connections arriving on non-loopback interfaces keep their
existing behavior. The policy does not resolve DNS or trust Forwarded headers.

The single transport still owns all sessions and is shared by the public and
internal MCP routes. Both routes retain bearer authentication and Origin
rejection; the internal route also retains its cluster credential. The
workstation proxy's own Host and Origin guards remain unchanged.

## Consequences

Deployments with a loopback ingress must explicitly name their public hosts
and restart the server when that list changes. No provider configuration or
workstation agent update is needed. Deployment validation must initialize MCP
with valid authentication and list tools, not stop at an unauthenticated 401.

Blindly disabling the SDK check would remove a protection from ordinary local
servers. Inferring trust from request headers would let the caller choose its
own exception. Rewriting the outbound public Host to localhost in the agent
would break hosting ingress routing. An explicit server-side list avoids those
trade-offs, and regression tests cover the proxy path and rejected requests.
