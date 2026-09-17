# Design

## Where the gate lives
`make test` is this repository's single aggregated quality gate: it already runs
`go test ./...`, the web tests, `tsc --noEmit` and `oxlint`. It is what the
workflow skills and contributors invoke. Adding the formatting check there makes
it run wherever the existing checks run, with no new tooling.

Rejected alternatives:
- **A GitHub Actions workflow.** There is no `.github/` directory and no CI
  convention to follow; creating one is an infrastructure decision with its own
  trade-offs (runner cost, secrets, required-check configuration) that this
  ticket did not settle. It remains the natural follow-up once the baseline is
  clean, because only a server-side gate blocks a push.
- **A local git hook.** Not shared through the checkout by default, and a hook
  installed per workstation reintroduces the "every contributor has to know"
  problem the ticket is trying to remove.

## Scope of the check
`gofmt -l .` over the whole module, not the `./cmd ./internal` pair the ticket's
wording used. The wider scope is already clean, so it costs nothing today, and it
cannot be escaped by adding a Go package outside those two trees later.

## Formatter strictness
Plain `gofmt`. Not `gofmt -s` (it rewrites code, not whitespace, so the
"formatting-only commit" claim would stop holding) and not `gofumpt` (a new
tool dependency and a much larger reformat). Adopting a stricter formatter is a
separate decision.

## Failure mode of the target
`gofmt -l` exits 0 whether or not it lists files, so the target has to test the
output itself and print the offending paths before failing — a bare
`gofmt -l .` in a recipe would silently pass. The message names the fix
(`gofmt -w`) so the reader does not have to look it up.
