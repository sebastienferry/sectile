# Design: PR Discovery Re-attaches Links a User Detached While It Was Running (#440)

## Context
Since `#407`, `applyDiscoveredPullRequests` executes inside `d.conn.WithTx` after locking the task row via `d.lockTaskUnsafe(tx, task.ID)`. However, `pr_links_detached` was only checked before the forge call in `pullRequestDiscoveryGate`, leaving a window during the forge call where a concurrent user detachment is lost.

## Decisions
1. **Re-read `pr_links_detached` on the Locked Task Row**:
   - Add `force bool` to `applyDiscoveredPullRequests(task *models.Task, found []models.TaskPullRequest, force bool)`.
   - Inside the transaction, immediately after `d.lockTaskUnsafe(tx, task.ID)`, when `!force`, query `SELECT pr_links_detached FROM tasks WHERE id = ?` on `tx`. If `detached != 0`, return `nil` without modifying `links`, `attached`, `warnings`, or the task row.
   - When `force` is `true` (`discoverAndApply` or `rediscoverPullRequests(..., true)`), bypass the `pr_links_detached` check and attach the discovered links as before, resetting `pr_links_detached = 0`.
2. **Audit Table Update**:
   - Update `docs/db-concurrency-audit.md` to record that `pr_links_detached` is re-read inside the locked transaction of `applyDiscoveredPullRequests`.
