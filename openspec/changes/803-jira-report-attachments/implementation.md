# Implementation report — #803

## Outcome and preserved work

Implemented on the assigned feat/803 worktree. Accepted clarification rounds,
proposal and requirements remain preserved. The design records an implementation
alignment with ADR 0006: the old managed result-file worker is retired, and native
runs publish through MCP. No replacement completion endpoint or guard bypass was
introduced. Draft PR #834 is reused; this invocation records implemented only,
never reviewed.

## Changes and reasons

- internal/models/report.go defines explicit immutable publication metadata,
  retained-report receipts and Jira attachment references, accepting Jira's
  numeric or string attachment IDs.
- internal/tracker/ticketing.go adds an optional report adapter interface;
  internal/trackerapi/jira_reports.go uploads exact UTF-8 Markdown bytes as a
  multipart file, uses the existing acting-user client and OAuth gateway, builds
  plain-text ADF without Markdown parsing, and reconciles attachments/comments
  using unique filenames and comment properties. The guarded HTTP transport in
  internal/trackerapi/client.go is shared with multipart requests. Scope coverage
  in jira_scopes_test.go includes the existing write:jira-work consent.
- internal/db/migrations.go adds shared migration 55 for durable task_reports;
  migrate.go includes the table in supported database transfer. reports.go
  persists exact bodies before remote work, rejects identity conflicts, records
  receipts, serializes concurrent publication and reuses attachments after
  restart. Upload and reconciliation failures warn and still attempt summaries;
  summary failures remain errors. stage.go commits retention and the stage
  atomically; trackerops.go publishes reports and records warnings in activities.
- internal/taskmcp/server.go accepts marked add_comment reports and transition
  metadata, while get_task returns retained reports independently of tracker
  comments. The existing HTTP stage and interactive confirmation paths in
  handlers.go, interactive.go and agent_desktop.go carry metadata without
  weakening execution ownership guards.
- The authorized task reports API and TaskReports.tsx expose chronological
  summaries, attachment links and expandable complete Markdown in TaskComments.
  Reads load independently of tracker discussion. Failed history reads can be
  retried, and switching tasks does not display another task's reports.
- Skill fragments and regenerated golden/plugin outputs explain explicit report
  metadata, tailored plain-text summaries, immutable retry IDs and Jira-specific
  attachment publication. GitHub/GitLab splitting and single final clarification
  publication remain intact. The API contract documents receipts and retry limits.
- CHANGELOG.md adds an English user-visible entry. Migration rewind fixtures and
  the Atlassian gateway fixture reflect the new schema and actual Jira responses.
- A single trailing blank line in main's new agent_agy_conversation_test.go was
  removed to restore make fmt-check. Baseline evidence from origin/main confirmed
  this formatting issue predates the implementation; no test logic was changed.

## Validation evidence

After merging the latest origin/main commits into the shared task branch, all
required checks passed:

- make fmt-check: exit 0, silent.
- make test: exit 0, including go test ./..., TypeScript checking and oxlint src.
  Representative actual output:

  ```text
  ok   tasks/internal/agent 79.388s
  ok   tasks/internal/db 23.762s
  ok   tasks/internal/handlers 23.167s
  ok   tasks/internal/skills (cached)
  ok   tasks/internal/taskmcp 6.204s
  ok   tasks/internal/trackerapi 2.035s
  # pass 737
  # fail 0
  ```

- make build-server build-agent: exit 0; Vite built the embedded UI, and both Go
  binaries built. The existing large-bundle advisory remains nonfatal.
- go vet ./...: exit 0, silent.
- make render-skills ARGS='-framework openspec -out /tmp/issue803-rendered-skills':
  `Rendered 11 file(s) (openspec framework, format: skill)`.
- Regenerated skill parity and plugin checks pass with UPDATE_GOLDEN=1 and -update,
  followed by the ordinary skill suite in make test.
- Browser regression with the real TaskComments/TaskReports and safe Markdown
  renderer, headless Chrome and isolated fixtures:
  `Report history survives tracker failure, retains rounds, switches tasks safely,
  and retries history reads.` Screenshot /tmp/issue803-report-history.png was
  visually inspected: chronological rounds, readable full Markdown/table,
  attachment link and fallback notice are present alongside unavailable tracker
  comments.
- openspec validate 803-jira-report-attachments --strict:
  `Change '803-jira-report-attachments' is valid`.
- Implementation whitespace checks pass. Whole-branch git diff --check still
  reports the accepted clarification's pre-existing trailing blank line at
  docs/clarifications/803.md:137; that accepted artifact is intentionally preserved.

New tests cover byte/line-ending preservation, multipart headers, token and OAuth
routing, numeric IDs, invalid attachment responses, unsafe filenames, literal
summary Markdown, receipt retention before upload, upload-warning fallback,
comment failure/retry, database restart, concurrency claims, uncertain-outcome
reconciliation/read failures, identity conflicts, chronological rounds, storage
failures, task/session authorization, unmarked discussion and stage mismatch,
managed-stage guards and atomic stage retention.

## Deliberate scope and limits

No historic report migration, general Markdown-to-ADF repair, or GitHub/GitLab
attachment rollout is included. No live Jira credentials are required or used.
PostgreSQL report tests are present and skipped under the project's normal
convention because SECTILE_TEST_POSTGRES_DSN was not configured; shared migration
and SQL paths are included, but no live PostgreSQL runtime result is claimed.
Existing lint warnings in unrelated components remain nonfatal; the new report
component introduces none.

Jira and Sectile cannot commit one atomic remote transaction. Known uploaded
attachments are reused; an interrupted upload is reconciled by the publication's
unique filename. If the outcome cannot be confirmed, the publisher avoids blind
re-upload and retains a warning while attempting the fallback summary. Comment
properties reconcile a lost comment response, but delayed remote visibility can
still require inspection. No exactly-once transport guarantee is claimed.

The work remains available on feat/803 for review and retry. No pull-request merge, approval,
ready-state change or workspace cleanup is performed.
