# Technical design

## Context and decisions
The product clarification for ticket #47 authorizes renaming Sectile to **Sectile** (slug: `sectile`) across public branding, build/distribution targets, package metadata, and documentation while retaining backwards compatibility with existing environments and configurations.

Retaining existing configuration paths and environment namespaces ensures zero disruption for existing users, pipelines, and local environments: existing databases and configurations remain valid without requiring data migrations or dual-store complexity.

## Identity inventory

| Surface | Decision / target |
| --- | --- |
| UI and browser | Replace product display title with "Sectile" in `web/index.html`, `web/src/locales/translations.ts`, and visible components. Preserve existing icons, styles, layout, and language switching behavior. |
| CLI & server text | Update CLI help examples, server startup log banners, API landing page info, and terminal prompt banners in `cmd/server/main.go` and `internal/runner/runner.go`. Subcommands, flags, and arguments remain unchanged. |
| Build and release | `Makefile`: target `build` produces `bin/sectile`; target `run` executes `./bin/sectile`. Target `release` outputs `dist/sectile-$$os-$$arch` (with `.exe` for Windows). The platform matrix (`darwin-arm64`, `darwin-amd64`, `linux-amd64`, `linux-arm64`, `windows-amd64`) and web asset embedding remain identical. |
| Package metadata | `web/package.json` updates package name to `sectile-web`. Lockfile root metadata aligned without altering dependency versions. |
| Instructions & templates | Update default product prose in `internal/db/projectcontext.go`, `internal/db/skilltemplates.go`, `internal/runner/runner.go`, and generated workflow skill templates. Delimiters (`sectile:project-context`) remain unchanged. |
| Documentation | Update `README.md` and current guides under `docs/` to present Sectile, document upgrade steps, and list retained compatibility contracts. Historical clarification reports and past OpenSpec change records are preserved. |

## Retained compatibility contracts

| Contract | Exact policy |
| --- | --- |
| Database path resolution | Retain existing database search order: explicit `DB_PATH` > working-directory `./tasks.db` > user-config `$APP_DIR/sectile/tasks.db` > user-config `$APP_DIR/taskacao/tasks.db` > fallback default `$APP_DIR/sectile/tasks.db`. Retain SQLite WAL/SHM file structure and schema. No database file relocation or schema migration. |
| Environment files | Retain existing precedence: process environment > `.env` > `.env.local` > `$APP_DIR/sectile/.env`. No breaking discovery path changes. |
| Environment variables | Retain existing `SECTILE_*` and `TASKACAO_*` environment variable support and precedence. Child processes and custom terminal commands continue receiving `SECTILE_*` variables. |
| Project files | Retain `.taskflow/` configuration directory, config JSON schema, `.tasks/` worktree paths, and `sectile:project-context` delimiters in instruction files. |
| Browser state | Retain `sectile_*` localStorage keys and existing `taskacao_*` read fallbacks. Do not introduce broken or fragmented keys. |
| Health and machine protocols | Retain health check service identifier (`sectile-api`), API route paths, machine-facing terminal markers (`__SECTILE_*`), and task result contracts. Update only human-readable presentation. |
| Repository and module | Retain Go module identity (`tasks`) and GitHub repository coordinates (`sebastienferry/sectile`). Working branch remains `47-rename-sectile-to-something-else`. |
| Custom scripts | Retain stored custom commands verbatim. Document how users can point custom scripts to `sectile` or create a local symlink/alias if desired. |

## Implementation approach
1. Inventory occurrences using `rg -i 'sectile'` to classify occurrences between user-facing presentation (to rename to Sectile) and machine/protocol contracts (to retain for compatibility).
2. Update frontend metadata and presentation strings in `web/index.html` and `web/src/locales/translations.ts`.
3. Update backend entry points in `cmd/server/main.go`, `internal/runner/runner.go`, and template generators in `internal/db/projectcontext.go` and `internal/db/skilltemplates.go`.
4. Update `Makefile` targets (`build`, `run`, `release`) to output `bin/sectile` and `dist/sectile-*`.
5. Update `README.md` and relevant technical documentation under `docs/`.
6. Add unit and regression tests verifying database discovery precedence, environment variable compatibility, and template generation idempotence.

## Alternatives rejected
- **Renaming the Go module or Git remote**: Rejected as out of scope; would cause unnecessary import churn and breaking repository redirects without product value.
- **Introducing dual writable storage namespaces (`sectile/` alongside `sectile/`)**: Rejected because dual stores risk split-brain state, synchronization bugs, and migration complexity.
- **Breaking legacy environment variables immediately**: Rejected to prevent breaking existing automation, CI pipelines, and developer environments.

## Validation and risks
- **Risks**:
  - Unintentionally modifying machine markers or API service identifiers, breaking protocol communication.
  - Overwriting user-owned configuration outside managed instruction blocks.
- **Verification**:
  - Run `go test ./...` and `make test` to ensure all tests pass.
  - Run `make build` and verify that `bin/sectile` is produced and executes cleanly.
  - Run `make release` and confirm that all five cross-platform binaries (`dist/sectile-*`) are generated.
  - Verify that `openspec validate 47-rename-sectile-to-sectile --strict` passes.
  - Run `git diff --check` to ensure no whitespace or formatting anomalies.
