# Design

## Baseline and target files
Go server and tracker adapters, SQLite/PostgreSQL persistence, React/TypeScript task view, generated Markdown skills. Reuse internal/trackerapi/client.go credential and HTTP routing, internal/trackerapi/jira.go, internal/tracker/ticketing.go, internal/db/comments.go, internal/db/trackerops.go, internal/db/skillresult.go, internal/taskmcp/server.go, internal/models, internal/agent result forwarding, the existing task HTTP routes/client types, web/src/components/TaskComments.tsx and task detail integration, internal/skills/fragments and skill rendering tests.

## Publication contract
Add a shared optional report object: publicationId (stable opaque caller ID), stage (clarified/specified/implemented/reviewed/finished), round (positive integer for numbered clarification rounds), summary (plain text). The existing body/note remains the unchanged full report; never read a workstation file on the server. Marked add_comment requires a publicationId, stage and summary; unmarked add_comment remains ordinary. Stage and managed notes are implicitly reports; report metadata is optional for legacy callers. Updated callers provide it, with round for clarification and summary tailored to the accepted decisions. Reject invalid stage, conflicting outer stage, invalid round, blank explicit summary and identity/content conflicts before remote writes. A clarification final report with a round uses that round's filename, not an unnumbered duplicate.

Stage operations use their durable operation ID when no explicit publication ID exists. Managed results derive the fallback identity from run ID and stage. Replaying an existing operation/result must use its original identity. Distinct authorized stage submissions are separate publications, even at the same stage. Do not deduplicate unrelated reports by content alone. Preserve execution guards and result ownership; carry metadata through all existing forwarding layers and schemas.

## Durable publication ledger
Add a task report table through supported SQLite/PostgreSQL schema paths. Store report ID, task/actor/tracker identity, stage/round, exact body, summary, PR URLs, creation time, attachment ID/name/content URL, upload and comment state, errors and remote comment identity if available. Scope identity uniqueness to task and tracker; reject changed immutable payload on reuse. Retain original body independently of legacy stage comment headers. Persist before upload; stop on storage error. Do not mirror all tracker discussion comments into this table.

Persist each successful remote step before attempting the next. Serialize concurrent publication of one ledger entry using the existing durable operation execution mechanism or a ledger claim, so two workers cannot both upload a known pending publication. A known attachment survives restart and comment failure. Reconcile an uncertain upload response by querying issue attachment metadata for the publication's unique filename before another upload. Use a stable short publication suffix when necessary to distinguish multiple reports at the same stage; preserve the task/stage/round prefix. Jira lacks an atomic cross-system transaction: document unresolved transport outcomes as uncertain, avoid blind automatic duplicate uploads, and return recoverable publication status rather than claim exactly-once transport. Never delete older attachments. A repeated completed publication returns its saved receipt without posting a second summary.

## Jira transport and summary
Introduce an optional report/attachment adapter interface rather than requiring non-Jira providers to implement uploads. Upload via the resolved write client using multipart/form-data with a file part, X-Atlassian-Token: no-check and application/json response acceptance. The UTF-8 body bytes are exact; set a safe basename and Markdown MIME type. Retain the returned attachment ID and content URL. Reuse bounded request timeouts and existing safe error handling; no credential logging or alternate account fallback.

Build minimal ADF directly: doc version 1, paragraph nodes, plain text nodes, optional standalone link marks for attachment and PR URLs. Never parse summary Markdown; unusual characters remain literal. Add attachment reference after the supplied summary; on upload failure add the existing surface's French failure notice and local report availability. Do not include the full report in ADF. Preserve supplied report language; new product labels and generic summaries use the existing French surface language.

Official transport reference: https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-attachments/ . Verify OAuth upload scope routing and configured consent against the attachment endpoint; fixtures must cover token and OAuth clients. Upload errors (including permissions/settings/size/network/invalid response) become structured warnings. Credential refusal still follows existing acting-user rules, and a refused summary write remains a publication failure.

## Result and retry semantics
Expose reportId, attachment reference, upload status and warning details in tool receipts, managed activity output and persisted tracker-operation steps. Attachment-only warning must not set the tool isError flag or fail an otherwise successful stage activity. Comment errors remain errors; do not pretend the already-recorded local stage was rolled back. Retry failed remote publication through its saved operation/report, preserving accepted artifacts and stage. A failed upload followed by successful fallback comment completes that publication with a warning; a repeat returns the receipt. A genuinely new report gets a new ID. This change must not bypass managed validation or advance a task merely because a draft PR exists.

## Task view
Provide authorized report-list and full-report read access through the existing task API conventions. Show chronological reports in the task detail discussion/history surface with stage/round, summary, an action to read the complete retained Markdown and an external attachment link when present. Fetch report history independently of tracker comments so a tracker read failure cannot hide retained reports. Render with existing safe Markdown components; avoid duplicating the full report inline alongside every summary comment. Existing task access checks apply to listing and reading, including intermediate reports.

## Skills and compatibility
Update source fragments and shared publication/result guidance, then regenerate checked-in skill outputs with the project's render target. Jira uses the full section plus metadata and summary, not 30,000-character splitting; GitHub/GitLab keep current rules. Final clarification uses its transition/result note once. Legacy stage notes remain valid with deterministic generic summary; unmarked legacy discussion comments deliberately remain discussion. Do not migrate historic notes.

## Rejected alternatives
General Markdown parser repair leaves future formatting failures possible. Guessing reports from body text misclassifies discussion. Replacing a cumulative attachment destroys historic references. Server semantic summarization changes accepted decisions. Returning upload warnings as fatal errors encourages duplicate transitions. Tracker comments alone lose intermediate full reports when summaries replace their bodies.

## Validation
HTTP fixtures prove exact uploaded bytes, headers, routing and plain ADF; database/workflow integration proves pre-upload retention, intermediate rounds, standalone and managed contracts, warnings, comment failure, restart/concurrency retry and access control on SQLite and PostgreSQL-supported paths. UI checks prove accessible full reports under tracker and attachment failures. Regenerated skill tests prove provider-specific guidance. No live Jira credentials are required. No unresolved product decisions remain.

## Implementation alignment (2026-10-14)

ADR 0006 retires the server-local result-file worker. The current worker in
internal/db/db.go dispatches native runs and states that workflow results are
reported through MCP; internal/db/skillresult.go retains ownership guards but
contains no managed-result publication contract. Implement the current MCP and
HTTP stage paths, forward report metadata through Desktop and interactive
confirmation, and preserve/test the legacy managed-stage guard. No replacement
managed completion endpoint is introduced. Generated guidance requires an
external host's supplied completion contract and reports unsupported report
metadata instead of bypassing validation. This applies the acceptance criteria
to every report publication path present in this checkout.

Uncertain uploads are reconciled by a unique task/stage/round/publication filename.
If metadata reads cannot confirm the outcome, retries avoid another upload and
retain a warning; summary fallback remains available. Jira comment properties
reconcile uncertain comment creation. Report metadata and exact bodies are stored
in a versioned, shared SQLite/PostgreSQL migration; claims are durable and remote
work is bounded to ninety seconds, below the five-minute claim lease.
