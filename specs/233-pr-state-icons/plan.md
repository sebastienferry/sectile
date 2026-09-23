# Design and validation

Persist optional `state` on each existing JSON `TaskPullRequest` link. Values: open, conflicting, merged, closed; omitted means unknown. No schema migration is required. Preserve state during normalization and edits; forge observations remain authoritative.

Add a forge-specific reader to `internal/trackerapi`: GitHub GraphQL aliases in chunks of at most 50 linked PRs, and GitLab project merge-request list reads filtered by IID in chunks of at most 50. Resolve the configured API origin, never use an arbitrary link as a credential destination. Keep acting-user/project credentials, redirect restrictions, response limits, cancellation and errors. GitLab forge support is independent of the issue tracker adapter (a Jira story may link a GitLab MR).

Use explicit conflict evidence only: GitHub mergeable CONFLICTING; GitLab has_conflicts or detailed_merge_status conflict. Terminal states take precedence. GitLab list results may reflect asynchronously computed mergeability; refresh on the next existing trigger rather than polling or forcing expensive checks.

The database refresh collects known links across the project independently of incremental issue changes, deduplicates requests, performs grouped forge reads, and updates only still-attached matching URLs under the write lock. It never rewrites links from a stale task snapshot. Targeted story writes/resyncs and workflow operations use the same refresh with one task. Failures preserve state and are surfaced as warnings. Existing discovery is retained for missing links; state refresh does not call SyncSingleTask or enqueue sync_task.

Web: shared state-aware icon component and current-link selector, used by cards, menus, lists and detail/history links. Desktop: shared metadata/icon helper for task rows, execution rows and selected execution header. Accessible state labels accompany distinct shapes and colors; unknown stays neutral.

Tests: forge mappings, host validation, chunking/deduplication, missing/error responses; database persistence/order/concurrent detach and request counts; current/historical link selection and all UI surfaces. Run Go tests/vet, web tests/build/lint, desktop tests/build and Go builds. Review the complete diff against origin/main.

References: https://docs.github.com/en/graphql/reference/pulls and https://docs.gitlab.com/api/merge_requests/.
