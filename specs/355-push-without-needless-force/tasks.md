# #355 — Implementation checklist

References: [`spec.md`](./spec.md), [`plan.md`](./plan.md).

- [x] T1 `create_pr/steps.md`: drop the force-with-lease clause from step 2; put the push rule, the refused-push handling and the `--force` ban in step 3 (D1, D2, FR1–FR3).
- [x] T2 `adjust/steps.md`: replace the step 6 force sentence with the same wording (D1, FR1–FR4).
- [x] T3 `catalog_test.go`: assert the missing-branch `git push -u origin`, the ancestor check, the resync-and-retry and the `--force` ban in `create_pr`, `adjust`, `pickup` and `pickup_issues`.
- [x] T4 Regenerate goldens: `UPDATE_GOLDEN=1 go test ./internal/skills/...`; review the golden diff.

## Test plan

- `go build ./...`, `go vet ./...`, `go test ./...`.
