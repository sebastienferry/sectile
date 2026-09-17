## ADDED Requirements

### Requirement: Gofmt-clean Go sources
Every Go source file tracked in the repository SHALL be formatted as `gofmt`
produces it. `gofmt -l` run from the repository root SHALL produce no output.

#### Scenario: Clean checkout
- **GIVEN** a checkout of the default branch
- **WHEN** `gofmt -l .` is run from the repository root
- **THEN** it prints nothing

#### Scenario: Formatting-only change
- **GIVEN** a Go file that `gofmt` would reformat
- **WHEN** it is brought into compliance
- **THEN** the change alters whitespace only and no package behaviour changes

### Requirement: Enforced formatting check
The repository SHALL expose a `fmt-check` make target that fails when any Go file
is unformatted, and the aggregated `test` target SHALL run it. The target SHALL
name the offending files so the reader does not have to re-run the formatter to
find them.

#### Scenario: Compliant tree
- **GIVEN** a tree where every Go file is formatted
- **WHEN** `make fmt-check` is run
- **THEN** it exits zero

#### Scenario: Unformatted file
- **GIVEN** a tree containing at least one unformatted Go file
- **WHEN** `make fmt-check` is run
- **THEN** it exits non-zero and prints the path of each unformatted file

#### Scenario: Aggregated gate
- **GIVEN** a tree containing an unformatted Go file
- **WHEN** `make test` is run
- **THEN** it fails on the formatting check

#### Scenario: Whole-module scope
- **GIVEN** an unformatted Go file outside `cmd/` and `internal/`
- **WHEN** `make fmt-check` is run
- **THEN** it fails, because the check covers the whole module

### Requirement: Documented formatting convention
The repository documentation SHALL state that Go sources are kept
`gofmt`-formatted, that `make test` enforces it, and how to fix a violation.

#### Scenario: Contributor lookup
- **GIVEN** a contributor whose build fails on the formatting check
- **WHEN** they read `README.md`
- **THEN** they find the convention and the `gofmt -w` remedy without reading the `Makefile`
