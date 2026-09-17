# Tasks

## 1. Clear the baseline
- [ ] 1.1 Run `gofmt -w internal/terminal/run_test.go` and confirm the diff is
      whitespace only (`git diff --stat`, then read the hunk).
- [ ] 1.2 Confirm `gofmt -l .` is empty at the repository root.

## 2. Enforce it
- [ ] 2.1 Add a `fmt-check` target to the `Makefile`, declared in `.PHONY`, with
      a `##` help line so it appears in `make help`. It runs `gofmt -l .`,
      prints the offending files and exits non-zero when the list is non-empty.
- [ ] 2.2 Make `test` depend on `fmt-check`.

## 3. Document it
- [ ] 3.1 Add a short paragraph to `README.md`, near the build and check
      instructions, stating that Go sources are `gofmt`-clean, that `make test`
      enforces it through `fmt-check`, and that `gofmt -w .` is the fix.

## 4. Verification
- [ ] 4.1 `make fmt-check` succeeds on the branch.
- [ ] 4.2 Introduce a deliberate formatting defect in a scratch file, confirm
      `make fmt-check` fails and names it, then revert it.
- [ ] 4.3 `make test` passes end to end; record the real output.
- [ ] 4.4 `openspec validate 163-gofmt-baseline --strict`.
