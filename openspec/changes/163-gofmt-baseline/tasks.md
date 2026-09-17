# Tasks

## 1. Clear the baseline
- [x] 1.1 Run `gofmt -w internal/terminal/run_test.go` and confirm the diff is
      whitespace only (`git diff --stat`, then read the hunk).
- [x] 1.2 Confirm `gofmt -l .` is empty at the repository root.

## 2. Enforce it
- [x] 2.1 Add a `fmt-check` target to the `Makefile`, declared in `.PHONY`, with
      a `##` help line so it appears in `make help`. It runs `gofmt -l .`,
      prints the offending files and exits non-zero when the list is non-empty.
- [x] 2.2 Make `test` depend on `fmt-check`.

## 3. Document it
- [x] 3.1 Add a short paragraph to `README.md`, near the build and check
      instructions, stating that Go sources are `gofmt`-clean, that `make test`
      enforces it through `fmt-check`, and that `gofmt -w .` is the fix.

## 4. Verification
- [x] 4.1 `make fmt-check` succeeds on the branch.
- [x] 4.2 Introduce a deliberate formatting defect in a scratch file, confirm
      `make fmt-check` fails and names it, then revert it.
- [x] 4.3 Introduce a deliberate syntax error in a scratch file, confirm
      `make fmt-check` propagates the non-zero `gofmt` status, then revert it.
- [x] 4.4 `make test` passes end to end; record the real output.
- [x] 4.5 `openspec validate 163-gofmt-baseline --strict`.
