# ADR 0001: MCP tools and versioned agent configuration

Status: Accepted

## Context

The central Sectile service owns task state and tracker synchronization, while
workstation agents own code, worktrees and AI subprocesses. Prompt-generated curl
calls and server filesystem paths crossed that boundary unreliably.

## Decision

Use the official MCP Go SDK for typed tools over Streamable HTTP. A database-free
stdio process discovers and forwards those tools through the local gateway or an
explicit central URL. All tools delegate to existing workflow services, preserving
managed-run guards and queued tracker writes.

Publish a versioned, explicit execution DTO rather than serializing the settings
or database models. The DTO excludes server paths and tracker credentials. Refresh
it on agent connection and task dispatch, reject unknown versions, and apply local
overrides only on the workstation. Install skills with a content-hash manifest to
track managed paths; back up manual changes before refreshing server-owned skills. Resolve worktrees locally and reject mismatched branches.

## Consequences

Clients get schema validation and protocol error handling from a maintained SDK.
Both transports expose the same tool catalog. The SDK adds Go dependencies.
Execution requires a reachable configuration API; offline job execution and
legacy SSE are not part of this change. Remote tracker synchronization remains
asynchronous and its queued state is explicit in tool results. Authentication
continues to be single-user and can pin a shared bearer credential; this change
does not introduce accounts or replace the deployment's web access controls.
