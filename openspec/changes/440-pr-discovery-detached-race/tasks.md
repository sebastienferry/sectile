# Tasks: PR Discovery Re-attaches Links a User Detached While It Was Running (#440)

- [x] 1. Re-read `pr_links_detached` inside `applyDiscoveredPullRequests`'s transaction when `!force` and thread `force` through all call sites.
- [x] 2. Add a test in `internal/db/prdiscovery_test.go` detaching links between discovery and write and verifying automatic vs forced behavior.
- [x] 3. Update `docs/db-concurrency-audit.md` and `CHANGELOG.md`.
