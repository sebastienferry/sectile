# Implementation tasks

- [x] 1. Add shared report metadata and receipts to standalone MCP, stage operation and managed result/forwarding contracts; validate explicit metadata and retain legacy stage note support. Test unmarked discussion, invalid input, stage mismatch and managed ownership guards.
- [x] 2. Add durable task report schema and storage for exact body, identity, actor, stage/round, attachment and publication state on SQLite/PostgreSQL. Test pre-upload persistence, identity conflicts, chronological intermediate rounds and storage failures.
- [x] 3. Add Jira multipart attachment transport through the existing acting-user write client. Test file part, safe filename, exact Unicode/whitespace/line-ending bytes, CSRF header, token/OAuth site routing and response parsing. Cover permission, disabled/oversized upload, timeouts and malformed responses.
- [x] 4. Add direct plain-text ADF summary construction and shared report publisher. Test legacy generic summaries, three clarification decisions plus open questions, specification goals, outcome PR links, literal Markdown and preservation of non-Jira/ordinary comment publication.
- [x] 5. Wire add_comment reports, transition notes and validated managed result notes to the publisher. Return attachment warnings without failed stage/tool status; keep comment errors visible. Test every entry path and upload-failure fallback with full local retention.
- [x] 6. Persist attachment/comment receipts and implement retry serialization and uncertain-outcome reconciliation. Test upload-success/comment-failure retry after restart, concurrent workers, repeated completed publication, distinct same-stage publications and conflicting reused IDs. Preserve attained stage and remote history.
- [x] 7. Expose authorized retained-report reads and task history UI actions for full Markdown and Jira attachment links. Test cross-task authorization, chronological rounds, tracker read failures and attachment failures; verify the task view manually.
- [x] 8. Update clarify/specify/implement/adjust/handoff/pickup shared and source skill fragments, result contract instructions and API documentation; run make render-skills and generated skill tests. Verify final rounds publish once and only Jira splitting guidance changes.
- [x] 9. Add an English user-facing Fixed/Changed entry under CHANGELOG.md Unreleased describing reliable Jira reports and access to full history.
- [x] 10. Run focused Go transport, DB, MCP, agent and skill tests, then repository make fmt-check, make test and required builds; run web tests, lint and build for UI changes. Run openspec validate 803-jira-report-attachments --strict. Record evidence and any transport ambiguity limits in the implementation report; publish through the authorized workflow only.

## Verification notes

All current publication paths are implemented and verified. The retired managed
result-file publisher is handled as documented in design.md's implementation
alignment, with the legacy ownership guard tested rather than a new completion
endpoint. PostgreSQL report tests are included and use the project's existing
optional SECTILE_TEST_POSTGRES_DSN convention; no live PostgreSQL service was
configured for this run. See implementation.md for evidence and transport limits.
