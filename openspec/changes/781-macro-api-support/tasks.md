## 1. Confirmed Product Scope

- [x] 1.1 Obtain owner answers to the three questions in `docs/clarifications/781.md`; define the metadata contract from the existing editor.
- [x] 1.2 Record the answers in a new clarification round and finalize this proposal and specification.
- [x] 1.3 Complete clarification through the applicable Sectile workflow contract before recording specification completion.

## 2. Implement Shared Metadata Behavior

- [ ] 2.1 Add a shared macro metadata operation covering validation, known-resource checks, existing updates, axis persistence, queued writes, and computed flags without accepting todo replacement.
- [ ] 2.2 Test omitted versus empty or false values, blank titles, invalid axes/horizons, missing resources, and unchanged unrelated fields.
- [ ] 2.3 Route HTTP metadata editing through the shared operation and verify existing response shapes, aliases, tracker restrictions, and queue notes remain compatible.

## 3. Add MCP Resource Tools

- [ ] 3.1 Add typed `list_macros`, `create_macro`, and `update_macro` tools with project scoping and the proposed result envelopes; retain `get_macro` and `update_macro_todos` compatibility.
- [ ] 3.2 Require authenticated caller identity for writes and preserve it through tracker operations and jobs.
- [ ] 3.3 Preserve a saved macro in an error-marked partial-success response; add a focused MCP serialization/client test.
- [ ] 3.4 Test empty and unknown projects, same-key project isolation, anonymous refusal, create defaults, metadata edits, and preservation of story-linked todos.

## 4. Add HTTP Single-Macro Reading

- [ ] 4.1 Add the exact-depth single-macro GET with encoded-key handling and missing-resource errors.
- [ ] 4.2 Test one-resource responses, project isolation, collection compatibility, unknown suffixes, and existing macro subroutes.

## 5. Verify and Document

- [ ] 5.1 Exercise local-project behavior and mocked tracker refusals, pending copy status, and roadmap write restrictions without contacting real trackers.
- [ ] 5.2 Run focused database tests and `go test ./internal/taskmcp ./internal/handlers`; run `git diff --check`.
- [ ] 5.3 Document the tools, HTTP read contract, partial-success semantics, and supported metadata; add one user-visible Unreleased changelog entry.
- [x] 5.4 Re-run `openspec validate 781-macro-api-support --strict` after settled-scope edits.

Implementation tasks are not executed during specify-issue. The owner confirmed the operation set and agent permissions; no product decisions remain open. Publish the specification-stage PR required by live project settings after validation.
