# #234 — Technical plan

Behaviour and acceptance criteria live in [`spec.md`](./spec.md). This file records the
implementation choices only.

## Stack and constraints

- GitLab CI on the mirror project, including `smartadserver/templates/jobs/go@v5.0.0`.
  That template sets `GOMODCACHE: $CI_PROJECT_DIR/.go-mod-cache`, and its `default:` block
  restores that cache into the checkout for every job that does not override `cache:`.
- Runner image: the template's `golang:1.26-trixie`, which ships `make`. The pipeline sets
  `GOTOOLCHAIN=auto` globally.
- `make fmt-check` runs `gofmt -l .` from the repository root and fails on any listed file
  or on a non-zero `gofmt` exit.

## Target files

```
.gitlab-ci.yml   new lint:gofmt job under "Tests"
README.md        "Formatting and checks" paragraph
```

## Decisions

### D1 — A standalone job, not `extends: .go-test`

`.go-test` brings a `gotestsum` script and junit artifacts the job does not produce. The
job only needs the default image, so it declares its own `script`.

### D2 — Keep every cache out of the checkout (FR4)

`gofmt -l .` descends into dot-directories. Checked with Go 1.27.1: a file under
`./.hidden/` is listed. With the template's cache restored, `./.go-mod-cache` holds every
dependency's sources. Because `test:go` runs with `GOTOOLCHAIN=auto`, it also holds the full
Go toolchain source tree, whose testdata is deliberately unformatted. The job would fail on
every pipeline.

So the job sets `cache: []` and points `GOMODCACHE` back to the image's default,
`/go/pkg/mod`, outside the checkout. It needs no module: `gofmt` reads source files only.

Rejected: excluding directories in `fmt-check`, because it changes the local contract and
the Makefile, which the clarification kept out of scope. Also rejected: running `gofmt -l`
over `git ls-files '*.go'` in CI only, because CI would no longer run the command the README
tells contributors to run.

### D3 — Run the `go.mod` toolchain's `gofmt` (FR5)

`GOTOOLCHAIN=auto` switches the `go` command, not the separate `gofmt` binary: the one on
`PATH` is the image's. The clarification assumed otherwise. The job therefore prepends
`$(go env GOROOT)/bin` to `PATH`. `go env` runs under `GOTOOLCHAIN=auto`, so it fetches the
toolchain `go.mod` selects (into `/go/pkg/mod`, per D2) and reports its root. The job then
prints `gofmt`'s location and `go version` so the log shows which one ran.

### D4 — Gating, and outside the publish path (FR3, FR6)

No `allow_failure`, following the `test:postgres` precedent. It is not added to the
`needs` of `publish:*`: the clarification kept the publishing path unchanged. A formatting
failure still fails the pipeline, and the pipeline status is what GitHub would receive.

## Verification

- `glab ci lint` on the mirror project validates the file with its includes.
- A WSL replay of the job script (`GOMODCACHE` outside the tree, `GOTOOLCHAIN=auto`)
  on the clean tree, and on a copy with an unformatted file and with an unparseable one.
- The mirror pipeline of `feat/234` runs the job for real.
