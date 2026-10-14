# Proposal: PR Discovery Re-attaches Links a User Detached While It Was Running (#440)

## Why
`applyDiscoveredPullRequests` (`internal/db/prdiscovery.go`) locks the task row with `FOR UPDATE` and re-reads `pr_links` and `branch_name`, but does not re-read `pr_links_detached`. If a user detaches a task's pull request links while an automatic PR discovery is waiting on the forge, the discovery overwrites the user's detachment by re-attaching the discovered links and resetting `pr_links_detached = 0`.

## What Changes
- Update `applyDiscoveredPullRequests` to take `force bool` and re-read `pr_links_detached` inside the locked transaction when `!force`, skipping the write when `pr_links_detached != 0`.
- Pass `force` from `rediscoverPullRequests`, `true` from `discoverAndApply`, and `false` from `discoverSecondaryPullRequests`.
- Add a test in `internal/db/prdiscovery_test.go` that detaches a task's links between discovery and write and verifies nothing is re-attached on the automatic path while the forced path still attaches.
- Update the `applyDiscoveredPullRequests` row in `docs/db-concurrency-audit.md`.

## Impact
- `internal/db/prdiscovery.go`
- `internal/db/prdiscovery_test.go`
- `internal/db/postgres_writepaths_test.go`
- `internal/db/postgres_concurrency_test.go`
- `internal/db/recordpr_test.go`
- `docs/db-concurrency-audit.md`
