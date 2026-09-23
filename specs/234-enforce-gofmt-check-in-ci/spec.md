# #234 — Enforce the gofmt check server-side in CI

Ticket: https://github.com/sebastienferry/sectile/issues/234
Branch: `feat/234`.
Clarification: [`docs/clarifications/234.md`](../../docs/clarifications/234.md) (rounds 1 and 2, settled).

## Context

#163 made the repository `gofmt`-clean and added `make fmt-check`, which `make test`
depends on. The check only runs for a contributor who runs `make test` locally, so an
unformatted commit can be pushed and merged. The repository now has a CI convention:
the GitHub repository is pull-mirrored into GitLab, and `.gitlab-ci.yml` runs a pipeline
on every mirrored branch and tag. The owner settled that the gate goes into that pipeline
(clarification round 2).

Out of scope: the formatter itself (`gofmt -s`, `gofumpt`), the four known `test:go`
failures and the wider `make test` gate, fork pull requests (the pull mirror never
copies them), GitHub Actions, and the image, binary and promotion jobs.

## User stories

### US1 — An unformatted Go file fails the pipeline (P1)

As a maintainer, I want a pipeline to fail when a branch carries a Go file that is not
`gofmt`-clean, so that the failure is visible before the branch is merged.

- **Given** a branch whose Go sources are all `gofmt`-clean, **when** its pipeline runs,
  **then** the formatting job passes.
- **Given** a branch that adds or changes a Go file so that `gofmt -l` lists it, **when**
  its pipeline runs, **then** the formatting job fails, names the file, and the pipeline
  status is failed.
- **Given** a branch with a Go file `gofmt` cannot parse, **when** its pipeline runs,
  **then** the formatting job fails rather than passing on an empty listing.
- **Given** the dependencies and the Go toolchain the runner downloads or restores from its
  cache, **when** the formatting job runs, **then** only the repository's own sources are
  checked, never third-party or toolchain sources.

### US2 — CI formats with the toolchain `go.mod` asks for (P1)

As a contributor, I want CI to judge formatting with the `gofmt` of the Go version
`go.mod` declares, so that a file my toolchain accepts is not rejected by another.

- **Given** `go.mod` declares `go 1.26.6` and the runner image ships another 1.26 patch,
  **when** the formatting job runs, **then** the `gofmt` it executes is the one from Go
  1.26.6.

### US3 — The failure reaches the GitHub pull request and can block the merge (P2)

As a maintainer, I want the pipeline result to show on the GitHub pull request and to be
able to require it on `main`.

- **Given** the GitLab project's GitHub integration posts commit statuses, **when** a
  mirrored branch's pipeline fails on formatting, **then** the pull request's head commit
  shows a failing status.
- **Given** that status is required in the branch protection of `main`, **when** it is
  failing, **then** GitHub refuses the merge.

US3 depends on two settings outside the repository (see Open requirements). This change
only delivers what the repository controls.

### US4 — The failure can be reproduced locally (P2)

- **Given** the formatting job failed, **when** a contributor reads the README, **then**
  it names the job, says that `make fmt-check` reproduces it, and that `gofmt -w .` fixes it.

## Functional requirements

- **FR1** The pipeline has a `lint:gofmt` job in the `test` stage that runs on every
  pipeline the `workflow` rules create, with no dependency on another job.
- **FR2** The job runs `make fmt-check`, the same command as the local reproduction.
- **FR3** The job is not allowed to fail: its failure fails the pipeline.
- **FR4** The job checks only the repository's sources. No module cache or downloaded
  toolchain may sit inside the checkout while it runs.
- **FR5** The `gofmt` the job runs is the one of the toolchain `go.mod` selects.
- **FR6** The publish and promote jobs keep their current `needs`.
- **FR7** The README's formatting paragraph names the job, keeps `make fmt-check` as the
  local reproduction, and says that making it required is a branch-protection setting.

## Non-functional requirements

- **NFR1** The job needs no secret and no service, and it downloads no module.
- **NFR2** No change to `Makefile`, `go.mod` or any Go source.

## Open requirements

These are not product decisions. They are admin actions outside the repository, and
they block US3 only:

1. **GitLab → GitHub commit statuses are off.** Verified on 2026-09-23: the GitLab
   project 86589838 has no GitHub integration (`GET /projects/86589838/integrations/github`
   returns 404), and the head commits of pull requests #365, #366 and #367 carry no status on
   GitHub even though their mirror pipelines succeeded. Until a maintainer enables it
   (GitLab → Settings → Integrations → GitHub), the job gates the mirror pipeline but
   nothing shows on the GitHub pull request, and "Done when" is not met.
2. **Required check on `main`.** Once statuses arrive, a repository admin adds the status
   context GitLab posts (the whole pipeline, not the individual job) to the required checks
   of `main`.
