# Pull request state indicators

Task `prLinks` carry an optional `state`: `open`, `conflicting`, `merged`, or `closed`. An omitted state is unknown. `prUrl` and the last link still identify the current PR; individual history links retain their own state. A task's workflow never implies a merge. Draft requests are open unless the forge reports actual conflicts.

The web and desktop render distinct shapes and colors, with accessible state descriptions. Legacy or unsupported links use a neutral icon until the server observes their state. A failed read preserves the last observed state.

## Refresh cost and ownership

After project synchronization, known links across that project are deduplicated and read in batches of at most 50: GitHub GraphQL PR aliases, or GitLab merge-request lists filtered by IID and grouped by repository. This also catches PR changes when the associated story did not change in the incremental issue window. It does not enqueue single-story synchronization or fetch issue details. Existing discovery of missing links remains separate.

Editing or explicitly resynchronizing a story refreshes only that story's linked PR metadata. Workflow postbacks refresh the affected story after the existing step completes. No new polling loop is introduced. GitLab mergeability can be computed asynchronously; unknown/checking status does not mean conflict, and the next existing refresh observes updates.

The reader accepts only URLs on configured forge origins and sends credentials exclusively to the configured API. It preserves project and acting-user credential resolution, fails on locked personal credentials without fallback, and uses the shared HTTP redirect and response-size guards. GitLab forge reads work independently of the issue tracker (for example, a Jira story linked to a GitLab MR); this does not add a GitLab issue tracker adapter.

States live in the existing `pr_links` JSON field; no database migration is needed. Edits cannot supply authoritative state. A refresh re-reads links under the database write lock and changes only matching states, preserving concurrent detachments, order, branch data and the current URL.
