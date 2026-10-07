## Status and Context

Technical plan for the owner-confirmed scope in `proposal.md`. Production implementation follows in the implementation stage.

The live project configuration selects OpenSpec despite the installed skill's static Spec Kit default. `openspec/config.yaml` already initializes the spec-driven schema. The assigned worktree and branch are reused.

## Evidence Read

- `docs/clarifications/781.md`: original questions and the owner's Round 2 confirmation of the operation set and agent permissions.
- `.agents/MEMORY.md`: tracker access through HTTP adapters and propagation of acting-user identity.
- `internal/taskmcp/server.go`: existing `get_macro`, `update_macro_todos`, caller resolution, and MCP response envelopes.
- `internal/taskmcp/macros_test.go`: reads, copy status, anonymous-write refusal, and preservation of story-linked todos.
- `internal/handlers/handlers.go`: collection GET, creation, metadata edits, and existing macro subactions.
- `internal/handlers/macroaxes.go`: queued axis writes and roadmap restrictions.
- `internal/db/macros.go`: listing, metadata persistence, create operations, and tracker side effects.
- `internal/db/macrotodosmirror.go`: single-macro lookup and protected todo replacement.
- `internal/models/models.go`: macro fields and computed write capabilities.
- `openspec/config.yaml` and `openspec/changes/459-clarification-report/{proposal.md,specs/clarification-report/spec.md}`: repository specification conventions.

## Proposed Interface Contracts

| Interface | Input | Successful response |
| --- | --- | --- |
| MCP `list_macros` | Required `projectId` | `{ "macros": [...] }`, with an empty array for a known empty project |
| MCP `get_macro` | Existing `projectId`, `macroKey` | Existing `{ "macro": ... }` |
| MCP `create_macro` | Required `projectId`, `title`; optional `horizon` | `{ "macro": ... }` |
| MCP `update_macro` | Required `projectId`, `macroKey`; optional metadata fields | `{ "macro": ..., "labelNote": ... }` |
| HTTP `GET /api/projects/{id}/macros/{key}` | Encoded project and macro identity | `{ "macro": ... }`; 404 for missing resources |

Metadata fields: `title`, `description`, `framingComment`, `horizon`, `closed`, `priority`, `quarter`, and `readiness`. Use pointers for optional fields so omitted values differ from empty strings and false. Do not expose todos, arbitrary labels, tracker status, computed flags, or caller identity as editable metadata. A request with no metadata fields should fail with a useful validation error. Validate supported horizons and axes before writing; reject a blank supplied title.

Keep the collection HTTP GET response as an array. Match single-resource GET by exact path depth after the existing subaction dispatch so requests for runs and other existing subroutes retain their behavior. Reject unknown extra path components rather than treating them as a collection request. Preserve the existing `/epics` alias if it shares this branch.

## Shared Operations and Identity

Register tools alongside the current macro tools in `internal/taskmcp/server.go` using typed inputs and existing caller helpers. Require `requireCaller` for writes and pass `tracker.WithActingUser(ctx, caller.UserID)` through synchronous operations and queued jobs. Resolve the project before listing and resolve an existing macro before editing: the underlying metadata save can create a record and must not silently turn an update into a create.

Reuse `GetProjectMacros`, `GetMacro`, and `CreateMacro`. Metadata orchestration needs more than a direct `UpdateMacro` call: the HTTP handler separately normalizes axes, queues horizon and axis operations, saves axes, and fills computed flags. Extract the minimum shared application operation into `internal/db` for the HTTP handler and MCP caller, preserving the existing HTTP contract and tracker restrictions. Preserve the HTTP editor's existing todo handling separately from the MCP metadata contract.

Reads reuse existing listing behavior. `GetMacro` currently calls `GetProjectMacros`, which may refresh local tracker metadata; this proposal does not promise side-effect-free local caching or add new tracker writes on reads.

## Partial Success

`CreateMacro` and `UpdateMacro` can return a saved macro together with an error. New MCP wrappers must preserve that macro and the error in an error-marked tool result with structured content such as `{ "macro": ..., "localSaved": true, "trackerError": ... }`. Do not return a generic error that loses the saved identity, or a success result that loses the refusal. Verify the MCP SDK's structured-error result behavior in a focused tool test before relying on it.

Preserve existing queue notes and copy status. Current database code also ignores some tracker-client failures; broader tracker error normalization is not silently included here. The proposed guarantee covers errors and queue outcomes actually reported by the reused operation. No retry or rollback contract is added.

## Dependencies and Alternatives

No migration, infrastructure change, new dependency, or new tracker capability is proposed. Local projects, GitHub milestones, and Jira roadmap epics retain current behavior, including creation limitations. Do not claim that generic macro creation adds Jira epic creation.

HTTP calls from MCP would duplicate authentication and add an unnecessary server round trip. Copying HTTP mutation orchestration into MCP would allow validation and queue behavior to diverge. Accepting raw todo lists in MCP metadata editing would bypass existing linked-story protections. The owner excluded full parity with all macro subactions.

## Validation and Delivery

Extend `internal/taskmcp/macros_test.go`, add focused handler coverage, and test any extracted mutation helper. Use local SQLite fixtures and fake tracker clients; no live tracker mutation is needed. Verify caller propagation, project isolation, optional fields, invalid-input atomicity, local partial success, protected todos, and collection/subroute compatibility.

Run `go test ./internal/taskmcp ./internal/handlers` and the focused `internal/db` tests covering extracted behavior during implementation. Add race checks for any new asynchronous test fixture. Update the relevant API documentation and one Unreleased changelog entry when implementation is delivered.

Run `openspec validate 781-macro-api-support --strict` and `git diff --check` for the specification. Commit the artifacts, publish the assigned branch, and create or reuse its draft specification PR as required by live project settings. Report the verified clarification and specification stages through Sectile; do not begin implementation.
