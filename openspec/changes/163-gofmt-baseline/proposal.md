# Clear the gofmt baseline and enforce it

## Why
`gofmt -l` has never been empty on `main`, so it cannot be used as a check: a
contributor running it sees a standing list of expected noise and cannot tell a
new defect from the baseline. An editor that formats on save turns any diff
touching one of those files into unrelated churn.

The baseline has shrunk on its own since the ticket was filed. As of
2026-09-17 (`origin/main` 487b857) `gofmt -l .` reports a single file,
`internal/terminal/run_test.go`, and the defect there is map-literal key
alignment — pure whitespace. Clearing it costs one commit, so the ticket's
"document an exemption" branch is not worth taking.

## What Changes
- Reformat `internal/terminal/run_test.go` with `gofmt -w`. No behavioural change.
- Add a `fmt-check` target to the `Makefile` that runs `gofmt -l .` and fails
  when the output is non-empty, and make `test` depend on it.
- Document the convention in `README.md` so a contributor finds it without
  reading the `Makefile`.

## Impact
`internal/terminal/run_test.go` (whitespace only), `Makefile`, `README.md`.
No Go package API changes, no schema change, no contract version change. The
repository has no CI runner; enforcement is the `make test` gate that agents and
contributors already run. Introducing a server-side gate (GitHub Actions) is a
separate infrastructure decision and stays out of this change.
