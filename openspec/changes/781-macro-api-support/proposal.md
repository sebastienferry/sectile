## Why

Issue #781 requests MCP/API support for macros. HTTP already exposes macro listing and mutations, while MCP exposes individual reads and ordered todo replacement. Agents cannot discover project macros or create and edit their metadata through equivalent MCP tools.

## Status

Scope confirmed by the owner on 2026-10-07 and recorded in Round 2 of `docs/clarifications/781.md`. This change specifies the approved behavior; production implementation is a subsequent stage.

Task: `gh-8b726529-6cf4-48cb-91e1-cdb93ccf491c-781`.
Tracker: https://github.com/sebastienferry/sectile/issues/781.
Branch: `feat/781`.

## What Changes

Confirmed operation set:

- Add MCP macro listing, creation, and metadata editing alongside the existing single-macro read.
- Add a distinct HTTP single-macro read while retaining the collection response.
- Require an identified caller for MCP writes and preserve existing tracker ownership, write restrictions, and partial-success reporting.
- Preserve existing todo editing and macro execution contracts.

## Capabilities

### New Capabilities

- `macro-resource-access`: consistent project-scoped macro discovery, reads, creation, and metadata edits through MCP and HTTP.

### Modified Capabilities

None proposed beyond the interfaces described here.

## Non-goals

New deletion tools, slicing, story generation, arbitrary label management, run launching, tracker copy repair, worktree tools, UI changes, new tracker capabilities, and database schema changes. Existing interfaces for those operations remain available.

## Settled Decisions

1. Provide MCP list/get/create/update and HTTP get-one, with no new deletion tool.
2. Leave advanced macro actions outside this ticket.
3. Permit macro creation and metadata editing by identified agents under existing tracker rules.

No product questions remain open. The metadata contract includes title, description, framing, horizon, closed state, priority, quarter, and readiness; it does not accept todos, arbitrary labels, or server-derived fields.

## Impact

Go MCP registration and schemas, HTTP macro routing, shared macro mutation orchestration, focused regression tests, API documentation, and one user-visible changelog entry at implementation time. The attached infrastructure repository requires no changes. The live project configuration selects OpenSpec and a draft PR after successful specification.
